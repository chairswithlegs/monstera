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
