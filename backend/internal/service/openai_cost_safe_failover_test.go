package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewOpenAIHTTPResponseFailoverErrorMarksExplicitHTTPResponse(t *testing.T) {
	for _, status := range []int{http.StatusBadGateway, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			err := newOpenAIHTTPResponseFailoverError(
				status,
				make(http.Header),
				[]byte(`{"error":{"message":"Upstream service temporarily unavailable"}}`),
				"Upstream service temporarily unavailable",
				false,
			)

			require.True(t, err.ExplicitUpstreamHTTPResponse)
			require.True(t, err.CostSafeToFailover())
		})
	}
}

func TestNewOpenAIUpstreamFailoverErrorDoesNotAssumeHTTPResponse(t *testing.T) {
	err := newOpenAIUpstreamFailoverError(
		http.StatusBadGateway,
		make(http.Header),
		[]byte(`{"type":"error","error":{"message":"Upstream service temporarily unavailable"}}`),
		"Upstream service temporarily unavailable",
		false,
	)

	require.False(t, err.ExplicitUpstreamHTTPResponse)
	require.False(t, err.CostSafeToFailover())
}

func TestNewOpenAIStreamFailoverErrorDoesNotMarkExplicitHTTPResponse(t *testing.T) {
	payload := []byte(`{
		"type":"response.failed",
		"response":{"error":{"type":"server_error","message":"Upstream service temporarily unavailable"}}
	}`)
	err := (&OpenAIGatewayService{}).newOpenAIStreamFailoverError(
		nil,
		nil,
		true,
		"",
		payload,
		"Upstream service temporarily unavailable",
	)

	require.Equal(t, http.StatusBadGateway, err.StatusCode)
	require.False(t, err.ExplicitUpstreamHTTPResponse)
	require.False(t, err.RequestScopedTransient)
	require.False(t, err.CostSafeToFailover())
}

func TestNewOpenAIStreamRateLimitWithUsageRemainsCostUnsafe(t *testing.T) {
	payload := []byte(`{
		"type":"response.failed",
		"response":{
			"error":{"type":"rate_limit_error","code":"rate_limit_exceeded","message":"Concurrency limit exceeded"},
			"usage":{"input_tokens":17,"output_tokens":0,"total_tokens":17}
		}
	}`)
	err := (&OpenAIGatewayService{}).newOpenAIStreamFailoverError(
		nil,
		nil,
		true,
		"",
		payload,
		"Concurrency limit exceeded",
	)

	require.Equal(t, http.StatusTooManyRequests, err.StatusCode)
	require.False(t, err.ExplicitUpstreamHTTPResponse)
	require.False(t, err.RequestScopedTransient)
	require.False(t, err.CostSafeToFailover())
}

func TestNewOpenAIStreamCapacityShedRemainsCostSafe(t *testing.T) {
	payload := []byte(`{
		"type":"response.failed",
		"response":{"error":{"type":"server_error","code":"server_is_overloaded","message":"Server is overloaded"}}
	}`)
	err := (&OpenAIGatewayService{}).newOpenAIStreamFailoverError(
		nil,
		nil,
		true,
		"",
		payload,
		"Server is overloaded",
	)

	require.False(t, err.ExplicitUpstreamHTTPResponse)
	require.True(t, err.RequestScopedTransient)
	require.True(t, err.CostSafeToFailover())
}
