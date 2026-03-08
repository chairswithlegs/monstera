package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCORS(t *testing.T) {
	t.Parallel()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := CORS(next)

	t.Run("sets CORS headers on any request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/timelines/public", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "Link", rec.Header().Get("Access-Control-Expose-Headers"))
	})

	t.Run("OPTIONS returns 204 with Allow-Methods and Allow-Headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/api/v1/statuses", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
		assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "GET, POST, PUT, DELETE, PATCH, OPTIONS", rec.Header().Get("Access-Control-Allow-Methods"))
		assert.Equal(t, "Accept, Authorization, Content-Type, X-CSRF-Token", rec.Header().Get("Access-Control-Allow-Headers"))
		assert.Equal(t, "300", rec.Header().Get("Access-Control-Max-Age"))
	})

	t.Run("OPTIONS does not call next handler", func(t *testing.T) {
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		})
		handler := CORS(next)
		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.False(t, nextCalled, "OPTIONS should be handled by middleware without calling next")
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("GET passes to next", func(t *testing.T) {
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		})
		handler := CORS(next)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		require.True(t, nextCalled)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}
