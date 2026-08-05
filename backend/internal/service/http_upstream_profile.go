package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptrace"
	"sync/atomic"
)

// HTTPUpstreamProfile marks HTTP upstream requests that need provider-specific
// transport policy.
type HTTPUpstreamProfile string

const (
	HTTPUpstreamProfileDefault HTTPUpstreamProfile = ""
	HTTPUpstreamProfileOpenAI  HTTPUpstreamProfile = "openai"
)

type httpUpstreamProfileContextKey struct{}
type httpUpstreamDisableRedirectsContextKey struct{}

// UpstreamRequestWriteState records whether an upstream HTTP request may have
// reached the provider. Unknown is intentionally not treated as safe to replay.
type UpstreamRequestWriteState uint8

const (
	UpstreamRequestWriteUnknown UpstreamRequestWriteState = iota
	UpstreamRequestNotWritten
	UpstreamRequestWritten
)

// HTTPUpstreamRequestError carries request-write evidence across the repository
// boundary while preserving errors.Is/errors.As behavior for the original error.
type HTTPUpstreamRequestError struct {
	Err               error
	RequestWriteState UpstreamRequestWriteState
}

func (e *HTTPUpstreamRequestError) Error() string {
	if e == nil || e.Err == nil {
		return "upstream request failed"
	}
	return e.Err.Error()
}

func (e *HTTPUpstreamRequestError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// HTTPUpstreamRequestWriteTracker uses net/http's WroteRequest callback. The
// callback is also invoked when a request write fails partway through, which is
// conservatively classified as written because the provider may have received
// a partial or complete request.
type HTTPUpstreamRequestWriteTracker struct {
	wrote atomic.Bool
}

func TrackHTTPUpstreamRequestWrite(req *http.Request) (*http.Request, *HTTPUpstreamRequestWriteTracker) {
	if req == nil {
		return nil, nil
	}
	tracker := &HTTPUpstreamRequestWriteTracker{}
	trace := &httptrace.ClientTrace{
		WroteRequest: func(httptrace.WroteRequestInfo) {
			tracker.wrote.Store(true)
		},
	}
	return req.WithContext(httptrace.WithClientTrace(req.Context(), trace)), tracker
}

func (t *HTTPUpstreamRequestWriteTracker) State() UpstreamRequestWriteState {
	if t == nil {
		return UpstreamRequestWriteUnknown
	}
	if t.wrote.Load() {
		return UpstreamRequestWritten
	}
	return UpstreamRequestNotWritten
}

func WrapHTTPUpstreamRequestError(err error, tracker *HTTPUpstreamRequestWriteTracker) error {
	if err == nil {
		return nil
	}
	if tracker == nil {
		return err
	}
	return &HTTPUpstreamRequestError{Err: err, RequestWriteState: tracker.State()}
}

func HTTPUpstreamRequestWriteStateFromError(err error) UpstreamRequestWriteState {
	var requestErr *HTTPUpstreamRequestError
	if !errors.As(err, &requestErr) || requestErr == nil {
		return UpstreamRequestWriteUnknown
	}
	return requestErr.RequestWriteState
}

// WithHTTPUpstreamProfile injects an upstream transport profile into ctx.
func WithHTTPUpstreamProfile(ctx context.Context, profile HTTPUpstreamProfile) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if profile == HTTPUpstreamProfileDefault {
		return ctx
	}
	return context.WithValue(ctx, httpUpstreamProfileContextKey{}, profile)
}

// HTTPUpstreamProfileFromContext resolves the upstream transport profile from ctx.
func HTTPUpstreamProfileFromContext(ctx context.Context) HTTPUpstreamProfile {
	if ctx == nil {
		return HTTPUpstreamProfileDefault
	}
	profile, ok := ctx.Value(httpUpstreamProfileContextKey{}).(HTTPUpstreamProfile)
	if !ok {
		return HTTPUpstreamProfileDefault
	}
	switch profile {
	case HTTPUpstreamProfileOpenAI:
		return profile
	default:
		return HTTPUpstreamProfileDefault
	}
}

// WithHTTPUpstreamRedirectsDisabled prevents credential-bearing probes from
// following redirects through the shared upstream client.
func WithHTTPUpstreamRedirectsDisabled(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, httpUpstreamDisableRedirectsContextKey{}, true)
}

func HTTPUpstreamRedirectsDisabled(ctx context.Context) bool {
	return ctx != nil && ctx.Value(httpUpstreamDisableRedirectsContextKey{}) == true
}
