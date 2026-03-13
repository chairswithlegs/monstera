package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamingTokenFromQuery(t *testing.T) {
	t.Parallel()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "" {
			w.Header().Set("X-Auth-Set", auth)
		}
		w.WriteHeader(http.StatusOK)
	})
	handler := StreamingTokenFromQuery(next)

	t.Run("no access_token leaves Authorization empty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/streaming/user", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, req.Header.Get("Authorization"))
	})

	t.Run("access_token in query with empty Authorization sets Bearer header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/streaming/user?access_token=secret-token", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "Bearer secret-token", req.Header.Get("Authorization"))
	})

	t.Run("existing Authorization is not overwritten", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/streaming/user?access_token=query-token", nil)
		req.Header.Set("Authorization", "Bearer existing-token")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "Bearer existing-token", req.Header.Get("Authorization"))
	})

	t.Run("next handler is always called", func(t *testing.T) {
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		})
		handler := StreamingTokenFromQuery(next)
		req := httptest.NewRequest(http.MethodGet, "/?access_token=tk", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		require.True(t, nextCalled)
		assert.Equal(t, "Bearer tk", req.Header.Get("Authorization"))
	})
}
