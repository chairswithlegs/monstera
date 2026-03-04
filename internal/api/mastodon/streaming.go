package mastodon

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/events/sse"
)

const (
	sseKeepaliveInterval = 30 * time.Second
	initialComment       = ":)\n\n"
	keepaliveComment     = ":keepalive\n\n"
)

// StreamingHandler handles Mastodon streaming API endpoints.
type StreamingHandler struct {
	hub *sse.Hub
}

// NewStreamingHandler returns a new StreamingHandler.
func NewStreamingHandler(hub *sse.Hub) *StreamingHandler {
	return &StreamingHandler{hub: hub}
}

// GETHealth handles GET /api/v1/streaming/health. Returns 200 with body "OK" (plain text).
func (h *StreamingHandler) GETHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// GETUser handles GET /api/v1/streaming/user. Requires auth; stream key user:{accountID}.
func (h *StreamingHandler) GETUser(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	streamKey := sse.StreamUserPrefix + account.ID
	h.serveSSE(w, r, streamKey)
}

// GETPublic handles GET /api/v1/streaming/public. Optional auth; ?local=true -> public:local.
func (h *StreamingHandler) GETPublic(w http.ResponseWriter, r *http.Request) {
	streamKey := sse.StreamPublic
	if strings.EqualFold(r.URL.Query().Get("local"), "true") {
		streamKey = sse.StreamPublicLocal
	}
	h.serveSSE(w, r, streamKey)
}

// GETPublicLocal handles GET /api/v1/streaming/public/local. Optional auth.
func (h *StreamingHandler) GETPublicLocal(w http.ResponseWriter, r *http.Request) {
	h.serveSSE(w, r, sse.StreamPublicLocal)
}

// GETHashtag handles GET /api/v1/streaming/hashtag. Optional auth; ?tag=foo required.
func (h *StreamingHandler) GETHashtag(w http.ResponseWriter, r *http.Request) {
	tag := strings.TrimSpace(r.URL.Query().Get("tag"))
	if tag == "" {
		api.HandleError(w, r, api.NewBadRequestError("tag is required"))
		return
	}
	tag = strings.TrimPrefix(strings.ToLower(tag), "#")
	if tag == "" {
		api.HandleError(w, r, api.NewBadRequestError("tag is required"))
		return
	}
	streamKey := sse.StreamHashtagPrefix + tag
	h.serveSSE(w, r, streamKey)
}

func (h *StreamingHandler) serveSSE(w http.ResponseWriter, r *http.Request, streamKey string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("streaming not supported"))
		return
	}

	rc := http.NewResponseController(w)
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		slog.WarnContext(r.Context(), "streaming: set write deadline", slog.Any("error", err))
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write([]byte(initialComment)); err != nil {
		return
	}
	flusher.Flush()

	eventCh, cancel := h.hub.Subscribe(streamKey)
	defer cancel()

	keepalive := time.NewTicker(sseKeepaliveInterval)
	defer keepalive.Stop()

	for {
		select {
		case ev, ok := <-eventCh:
			if !ok {
				return
			}
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Event, ev.Data); err != nil {
				return
			}
			flusher.Flush()
		case <-keepalive.C:
			if _, err := w.Write([]byte(keepaliveComment)); err != nil {
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
