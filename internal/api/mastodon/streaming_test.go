package mastodon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera-fed/internal/events/sse"
	"github.com/chairswithlegs/monstera-fed/internal/observability"
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
