package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogger(t *testing.T) {
	t.Parallel()

	tests := []struct {
		env   string
		level string
	}{
		{"development", "info"},
		{"development", "debug"},
		{"production", "info"},
		{"production", "error"},
	}

	for _, tt := range tests {
		t.Run(tt.env+"/"+tt.level, func(t *testing.T) {
			t.Parallel()
			logger := NewLogger(tt.env, tt.level)
			require.NotNil(t, logger)
		})
	}
}

func TestWithAccountID_roundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	assert.Empty(t, AccountIDFromContext(ctx))

	ctx = WithAccountID(ctx, "01ARZ3NDEKTSV4RRFFQ69G5FAV")
	assert.Equal(t, "01ARZ3NDEKTSV4RRFFQ69G5FAV", AccountIDFromContext(ctx))
}

func TestRequestIDMiddlware_GeneratesRequestID(t *testing.T) {
	t.Parallel()

	var ctx context.Context
	handler := RequestIDMiddlware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		ctx = r.Context()
	}))
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, RequestIDFromContext(ctx))
}

func TestRequestLoggerMiddleware(t *testing.T) {
	t.Parallel()

	handler := RequestLoggerMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusCreated, rec.Code)
}
