//go:build unit

package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const (
	costSafeFirstAccountID  int64 = 9920
	costSafeSecondAccountID int64 = 9921
)

type openAICostSafeFailoverUpstream struct {
	service.HTTPUpstream

	mu          sync.Mutex
	accountIDs  []int64
	firstStatus int
	firstErr    error
	streamError bool
}

func (u *openAICostSafeFailoverUpstream) Do(_ *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	u.mu.Lock()
	u.accountIDs = append(u.accountIDs, accountID)
	u.mu.Unlock()

	if accountID == costSafeFirstAccountID {
		if u.firstErr != nil {
			return nil, u.firstErr
		}
		if u.streamError {
			body := strings.Join([]string{
				`data: {"type":"response.failed","response":{"id":"resp_cost_safe_failed","status":"failed","error":{"type":"rate_limit_error","code":"rate_limit_exceeded","message":"Concurrency limit exceeded"},"usage":{"input_tokens":17,"output_tokens":0,"total_tokens":17}}}`,
				"",
			}, "\n")
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}
		return &http.Response{
			StatusCode: u.firstStatus,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"upstream_error","message":"Upstream service temporarily unavailable"}}`)),
		}, nil
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"id":"resp_cost_safe_ok","object":"response","model":"gpt-5.6-sol","status":"completed","output":[],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`,
		)),
	}, nil
}

func (u *openAICostSafeFailoverUpstream) calls() []int64 {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]int64(nil), u.accountIDs...)
}

type openAICostSafeAccountRepo struct {
	service.AccountRepository
	accounts []service.Account
}

func (r *openAICostSafeAccountRepo) ListSchedulableByPlatform(_ context.Context, platform string) ([]service.Account, error) {
	accounts := make([]service.Account, 0, len(r.accounts))
	for _, account := range r.accounts {
		if account.Platform == platform && account.IsSchedulable() {
			accounts = append(accounts, account)
		}
	}
	return accounts, nil
}

func (r *openAICostSafeAccountRepo) ListSchedulableByGroupIDAndPlatform(ctx context.Context, _ int64, platform string) ([]service.Account, error) {
	return r.ListSchedulableByPlatform(ctx, platform)
}

func (r *openAICostSafeAccountRepo) ListSchedulableUngroupedByPlatform(ctx context.Context, platform string) ([]service.Account, error) {
	return r.ListSchedulableByPlatform(ctx, platform)
}

func (r *openAICostSafeAccountRepo) GetByID(_ context.Context, id int64) (*service.Account, error) {
	for _, account := range r.accounts {
		if account.ID == id {
			copy := account
			return &copy, nil
		}
	}
	return nil, nil
}

func newOpenAICostSafeFailoverTestHandler(t *testing.T, upstream service.HTTPUpstream) (*OpenAIGatewayHandler, int64) {
	t.Helper()

	groupID := int64(4205)
	repo := &openAICostSafeAccountRepo{accounts: []service.Account{
		{
			ID: costSafeFirstAccountID, Name: "cost-safe-first", Platform: service.PlatformOpenAI,
			Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Priority: 1,
			Credentials: map[string]any{"api_key": "sk-first", "base_url": "https://first.example.test"},
			Extra:       map[string]any{"openai_passthrough": true},
		},
		{
			ID: costSafeSecondAccountID, Name: "cost-safe-second", Platform: service.PlatformOpenAI,
			Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Priority: 2,
			Credentials: map[string]any{"api_key": "sk-second", "base_url": "https://second.example.test"},
			Extra:       map[string]any{"openai_passthrough": true},
		},
	}}

	cfg := &config.Config{RunMode: config.RunModeSimple}
	cfg.Default.RateMultiplier = 1
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Gateway.CostSafeFailover = true
	cfg.Gateway.MaxAccountSwitches = 3

	billingCacheService := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billingCacheService.Stop)
	gatewayService := service.NewOpenAIGatewayService(
		repo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		cfg,
		nil,
		nil,
		service.NewBillingService(cfg, nil),
		nil,
		billingCacheService,
		upstream,
		&service.DeferredService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	handler := NewOpenAIGatewayHandler(
		gatewayService,
		service.NewConcurrencyService(nil),
		billingCacheService,
		service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg),
		nil,
		nil,
		nil,
		nil,
		cfg,
	)
	return handler, groupID
}

func runOpenAICostSafeResponsesRequest(t *testing.T, handler *OpenAIGatewayHandler, groupID int64, stream bool) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := `{"model":"gpt-5.6-sol","input":"hello","stream":false}`
	if stream {
		body = `{"model":"gpt-5.6-sol","input":"hello","stream":true}`
	}
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{
		ID: 1805, GroupID: &groupID,
		User:  &service.User{ID: 1705, Status: service.StatusActive},
		Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI, Status: service.StatusActive},
	})
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1705, Concurrency: 0})

	handler.Responses(c)
	return recorder
}

func TestOpenAIResponsesCostSafeFailoverSelectsSecondAccountAfterExplicitHTTPFailure(t *testing.T) {
	for _, status := range []int{http.StatusBadGateway, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			upstream := &openAICostSafeFailoverUpstream{firstStatus: status}
			handler, groupID := newOpenAICostSafeFailoverTestHandler(t, upstream)

			recorder := runOpenAICostSafeResponsesRequest(t, handler, groupID, false)

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.Equal(t, "resp_cost_safe_ok", gjson.GetBytes(recorder.Body.Bytes(), "id").String())
			require.Equal(t, []int64{costSafeFirstAccountID, costSafeSecondAccountID}, upstream.calls())
		})
	}
}

func TestOpenAIResponsesCostSafeFailoverBlocksTransportFailureAfterRequestWrite(t *testing.T) {
	upstream := &openAICostSafeFailoverUpstream{firstErr: &service.HTTPUpstreamRequestError{
		Err:               io.ErrUnexpectedEOF,
		RequestWriteState: service.UpstreamRequestWritten,
	}}
	handler, groupID := newOpenAICostSafeFailoverTestHandler(t, upstream)

	recorder := runOpenAICostSafeResponsesRequest(t, handler, groupID, false)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, []int64{costSafeFirstAccountID}, upstream.calls())
	require.Contains(t, recorder.Body.String(), `"type":"response.failed"`)
	require.Contains(t, recorder.Body.String(), `"code":"upstream_error"`)
}

func TestOpenAIResponsesCostSafeFailoverBlocksResponseFailedStreamEvent(t *testing.T) {
	upstream := &openAICostSafeFailoverUpstream{streamError: true}
	handler, groupID := newOpenAICostSafeFailoverTestHandler(t, upstream)

	recorder := runOpenAICostSafeResponsesRequest(t, handler, groupID, true)

	require.Equal(t, []int64{costSafeFirstAccountID}, upstream.calls())
	require.NotContains(t, recorder.Body.String(), "resp_cost_safe_ok")
}
