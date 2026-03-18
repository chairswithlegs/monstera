package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaxBodySize(t *testing.T) {
	t.Parallel()

	t.Run("request under limit succeeds", func(t *testing.T) {
		t.Parallel()
		var readBody []byte
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			readBody = b
			w.WriteHeader(http.StatusOK)
		})
		handler := MaxBodySize(1024)(next)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader("hello"))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "hello", string(readBody))
	})

	t.Run("request at exact limit succeeds", func(t *testing.T) {
		t.Parallel()
		body := strings.Repeat("x", 64)
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			assert.Len(t, b, 64)
			w.WriteHeader(http.StatusOK)
		})
		handler := MaxBodySize(64)(next)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(body))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("request over limit returns MaxBytesError", func(t *testing.T) {
		t.Parallel()
		var readErr error
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, readErr = io.ReadAll(r.Body)
			if readErr != nil {
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				return
			}
			w.WriteHeader(http.StatusOK)
		})
		handler := MaxBodySize(10)(next)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(strings.Repeat("x", 100)))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		require.Error(t, readErr)
		var mbe *http.MaxBytesError
		assert.ErrorAs(t, readErr, &mbe)
	})

	t.Run("nil body is not wrapped", func(t *testing.T) {
		t.Parallel()
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Nil(t, r.Body)
			w.WriteHeader(http.StatusOK)
		})
		handler := MaxBodySize(1024)(next)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Body = nil
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}
