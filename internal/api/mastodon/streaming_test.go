package mastodon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/events/sse"
	"github.com/chairswithlegs/monstera/internal/observability"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/testutil"
)

func TestStreamingHandler_GETHealth(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)
	hub := sse.NewHub(newMockHubConn(), metrics)
	h := NewStreamingHandler(hub)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/streaming/health", nil)
	rec := httptest.NewRecorder()
	h.GETHealth(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Equal(t, "OK", rec.Body.String())
}

func TestStreamingHandler_GETUser_Unauthenticated_Returns401(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)
	hub := sse.NewHub(newMockHubConn(), metrics)
	h := NewStreamingHandler(hub)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/streaming/user", nil)
	rec := httptest.NewRecorder()
	h.GETUser(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestStreamingHandler_GETHashtag_MissingTag_Returns400(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)
	hub := sse.NewHub(newMockHubConn(), metrics)
	h := NewStreamingHandler(hub)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/streaming/hashtag", nil)
	rec := httptest.NewRecorder()
	h.GETHashtag(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Contains(t, body["error"], "tag")
}

func TestStreamingHandler_GETHashtag_EmptyTagAfterTrim_Returns400(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)
	hub := sse.NewHub(newMockHubConn(), metrics)
	h := NewStreamingHandler(hub)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/streaming/hashtag?tag=%23", nil)
	rec := httptest.NewRecorder()
	h.GETHashtag(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// flusherRecorder wraps ResponseRecorder to implement http.Flusher for SSE tests.
type flusherRecorder struct {
	*httptest.ResponseRecorder
}

func (f *flusherRecorder) Flush() {}

func TestStreamingHandler_GETUser_HappyPath_returns200AndSSEHeaders(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)
	hub := sse.NewHub(newMockHubConn(), metrics)
	h := NewStreamingHandler(hub)
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	acc, err := accountSvc.Register(context.Background(), service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/streaming/user", nil).WithContext(ctx)
	req = req.WithContext(middleware.WithAccount(req.Context(), acc))
	rec := &flusherRecorder{ResponseRecorder: httptest.NewRecorder()}
	done := make(chan struct{})
	go func() {
		h.GETUser(rec, req)
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Body.String(), ":)")
}

func TestStreamingHandler_GETPublic_returns200AndSSEHeaders(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)
	hub := sse.NewHub(newMockHubConn(), metrics)
	h := NewStreamingHandler(hub)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/streaming/public", nil).WithContext(ctx)
	rec := &flusherRecorder{ResponseRecorder: httptest.NewRecorder()}
	done := make(chan struct{})
	go func() {
		h.GETPublic(rec, req)
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
}

func TestStreamingHandler_GETHashtag_validTag_returns200AndSSEHeaders(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)
	hub := sse.NewHub(newMockHubConn(), metrics)
	h := NewStreamingHandler(hub)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/streaming/hashtag?tag=go", nil).WithContext(ctx)
	rec := &flusherRecorder{ResponseRecorder: httptest.NewRecorder()}
	done := make(chan struct{})
	go func() {
		h.GETHashtag(rec, req)
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
}

// mockHubConn is a minimal natsConn for tests that only need a non-nil Hub (e.g. GETHealth, error paths).
type mockHubConn struct{}

func newMockHubConn() *mockHubConn {
	return &mockHubConn{}
}

func (m *mockHubConn) Subscribe(subject string, handler nats.MsgHandler) (interface{ Unsubscribe() error }, error) {
	return &mockHubSub{}, nil
}

type mockHubSub struct{}

func (m *mockHubSub) Unsubscribe() error { return nil }
