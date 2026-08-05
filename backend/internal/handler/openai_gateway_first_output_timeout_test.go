package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIForwardMayFailoverOnlyAfterNonSemanticWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	before := service.OpenAICompactKeepaliveAdjustedWrittenSize(c)

	_, err := fmt.Fprint(c.Writer, ":\n\n")
	require.NoError(t, err)
	c.Writer.Flush()

	require.True(t, openAIForwardMayFailover(c, before, &service.UpstreamFailoverError{
		SafeToFailoverAfterWrite: true,
	}))
	require.False(t, openAIForwardMayFailover(c, before, &service.UpstreamFailoverError{}))
}

func TestOpenAIForwardMayFailoverWithCostSafety(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	before := service.OpenAICompactKeepaliveAdjustedWrittenSize(c)

	tests := []struct {
		name        string
		strict      bool
		failoverErr *service.UpstreamFailoverError
		want        bool
	}{
		{
			name:        "legacy mode preserves ambiguous 502 failover",
			failoverErr: &service.UpstreamFailoverError{StatusCode: http.StatusBadGateway},
			want:        true,
		},
		{
			name:        "strict mode allows explicit 403 rejection",
			strict:      true,
			failoverErr: &service.UpstreamFailoverError{StatusCode: http.StatusForbidden},
			want:        true,
		},
		{
			name:        "strict mode allows explicit 429 rejection",
			strict:      true,
			failoverErr: &service.UpstreamFailoverError{StatusCode: http.StatusTooManyRequests},
			want:        true,
		},
		{
			name:   "strict mode allows transport failure before request write",
			strict: true,
			failoverErr: &service.UpstreamFailoverError{
				StatusCode:        http.StatusBadGateway,
				RequestWriteState: service.UpstreamRequestNotWritten,
			},
			want: true,
		},
		{
			name:   "strict mode blocks timeout after request write",
			strict: true,
			failoverErr: &service.UpstreamFailoverError{
				StatusCode:        http.StatusBadGateway,
				RequestWriteState: service.UpstreamRequestWritten,
			},
			want: false,
		},
		{
			name:        "strict mode blocks ambiguous transport failure",
			strict:      true,
			failoverErr: &service.UpstreamFailoverError{StatusCode: http.StatusBadGateway},
			want:        false,
		},
		{
			name:   "strict mode blocks first output timeout",
			strict: true,
			failoverErr: &service.UpstreamFailoverError{
				StatusCode:               http.StatusGatewayTimeout,
				SafeToFailoverAfterWrite: true,
				RequestWriteState:        service.UpstreamRequestWritten,
			},
			want: false,
		},
		{
			name:   "strict mode allows explicit capacity rejection",
			strict: true,
			failoverErr: &service.UpstreamFailoverError{
				StatusCode:             http.StatusServiceUnavailable,
				RequestScopedTransient: true,
			},
			want: true,
		},
		{
			name:   "strict mode allows account credential failure",
			strict: true,
			failoverErr: &service.UpstreamFailoverError{
				StatusCode: http.StatusBadGateway,
				Stage:      service.GatewayFailureStageAccountAuth,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, openAIForwardMayFailoverWithCostSafety(c, before, tt.failoverErr, tt.strict))
		})
	}
}

func TestOpenAIFirstOutputFailoverStopsAfterOneAccountSwitch(t *testing.T) {
	failoverErr := &service.UpstreamFailoverError{SafeToFailoverAfterWrite: true}
	count := 0

	require.False(t, openAIFirstOutputFailoverExhausted(failoverErr, &count))
	require.Equal(t, 1, count)
	require.True(t, openAIFirstOutputFailoverExhausted(failoverErr, &count))
	require.Equal(t, 1, count)
}

func TestOpenAIRequestAllowsFailoverReplayStopsCanceledClient(t *testing.T) {
	require.False(t, openAIRequestAllowsFailoverReplay(nil))

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	requestCtx, cancel := context.WithCancel(context.Background())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil).WithContext(requestCtx)

	require.True(t, openAIRequestAllowsFailoverReplay(c))
	cancel()
	require.False(t, openAIRequestAllowsFailoverReplay(c))
}
