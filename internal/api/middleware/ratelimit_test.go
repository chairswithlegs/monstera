package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/ratelimit"
)

type stubLimiter struct {
	result ratelimit.Result
	err    error
	calls  int
}

func (s *stubLimiter) Allow(_ context.Context, _ string, _ int, _ time.Duration) (ratelimit.Result, error) {
	s.calls++
	return s.result, s.err
}

func TestRateLimit_AllowedRequest(t *testing.T) {
	t.Parallel()
	stub := &stubLimiter{result: ratelimit.Result{
		Allowed:   true,
		Limit:     300,
		Remaining: 299,
		ResetAt:   time.Now().Add(5 * time.Minute),
	}}

	handler := RateLimit(stub, 300, 5*time.Minute, func(r *http.Request) string {
		return "test-key"
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "300", rec.Header().Get("X-RateLimit-Limit"))
	assert.Equal(t, "299", rec.Header().Get("X-RateLimit-Remaining"))
	assert.NotEmpty(t, rec.Header().Get("X-RateLimit-Reset"))
	assert.Equal(t, 1, stub.calls)
}

func TestRateLimit_DeniedRequest(t *testing.T) {
	t.Parallel()
	resetAt := time.Now().Add(3 * time.Minute)
	stub := &stubLimiter{result: ratelimit.Result{
		Allowed:   false,
		Limit:     300,
		Remaining: 0,
		ResetAt:   resetAt,
	}}

	handler := RateLimit(stub, 300, 5*time.Minute, func(r *http.Request) string {
		return "test-key"
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("handler should not be called when rate limited")
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))
	assert.Equal(t, "0", rec.Header().Get("X-RateLimit-Remaining"))
}

func TestRateLimit_EmptyKeySkipsLimiting(t *testing.T) {
	t.Parallel()
	stub := &stubLimiter{}

	handler := RateLimit(stub, 300, 5*time.Minute, func(r *http.Request) string {
		return ""
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 0, stub.calls)
}

func TestRateLimitByAccount_ExtractsAccountID(t *testing.T) {
	t.Parallel()
	stub := &stubLimiter{result: ratelimit.Result{
		Allowed:   true,
		Limit:     300,
		Remaining: 299,
		ResetAt:   time.Now().Add(5 * time.Minute),
	}}

	var called bool
	handler := RateLimitByAccount(stub, 300, 5*time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := WithAccount(req.Context(), &domain.Account{ID: "acct-123"})
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.True(t, called)
	assert.Equal(t, 1, stub.calls)
}

func TestRateLimitByAccount_SkipsWhenNoAccount(t *testing.T) {
	t.Parallel()
	stub := &stubLimiter{}

	var called bool
	handler := RateLimitByAccount(stub, 300, 5*time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	require.True(t, called)
	assert.Equal(t, 0, stub.calls)
}

func TestRateLimitByIP_ExtractsIP(t *testing.T) {
	t.Parallel()
	stub := &stubLimiter{result: ratelimit.Result{
		Allowed:   true,
		Limit:     100,
		Remaining: 99,
		ResetAt:   time.Now().Add(5 * time.Minute),
	}}

	var called bool
	handler := RateLimitByIP(stub, 100, 5*time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:54321"

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.True(t, called)
	assert.Equal(t, 1, stub.calls)
}

func TestRateLimit_ErrorAllowsRequest(t *testing.T) {
	t.Parallel()
	stub := &stubLimiter{err: context.DeadlineExceeded}

	var called bool
	handler := RateLimit(stub, 300, 5*time.Minute, func(r *http.Request) string {
		return "key"
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	require.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}
