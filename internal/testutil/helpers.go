package testutil

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// StrPtr returns a pointer to s. Useful in tests when building structs with optional *string fields.
func StrPtr(s string) *string {
	return &s
}

// AddChiURLParam sets a chi URL param on the request for testing. Use when calling handlers
// directly without the router so that chi.URLParam(r, key) returns value.
func AddChiURLParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// AddChiURLParams sets multiple chi URL params on the request for testing.
func AddChiURLParams(r *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// NoopBlocklistRefresher is a no-op implementation of service.BlocklistRefresher for tests.
type NoopBlocklistRefresher struct{}

// Refresh is a no-op.
func (NoopBlocklistRefresher) Refresh(_ context.Context) error { return nil }
