package mastodon

import (
	"net/http"
)

// StreamingHandler handles Mastodon streaming API endpoints.
type StreamingHandler struct{}

// NewStreamingHandler returns a new StreamingHandler.
func NewStreamingHandler() *StreamingHandler {
	return &StreamingHandler{}
}

// GETHealth handles GET /api/v1/streaming/health. Returns 200 with body "OK" (plain text).
func (h *StreamingHandler) GETHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}
