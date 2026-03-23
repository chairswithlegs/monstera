package mastodon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/mastodon/sse"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
)

const (
	wsPingInterval     = 30 * time.Second
	wsWriteTimeout     = 10 * time.Second
	wsMaxSubscriptions = 10
)

// WSClientMessageType is the type of a client-to-server WebSocket message.
type WSClientMessageType string

const (
	wsClientMsgSubscribe   WSClientMessageType = "subscribe"
	wsClientMsgUnsubscribe WSClientMessageType = "unsubscribe"
)

// wsClientMsg is a message sent from the WebSocket client to the server.
type wsClientMsg struct {
	Type   WSClientMessageType `json:"type"`           // "subscribe" | "unsubscribe"
	Stream string              `json:"stream"`         // e.g. "user", "public", "hashtag", "list", "public:local", "direct"
	Tag    string              `json:"tag,omitempty"`  // required when Stream == "hashtag"
	List   string              `json:"list,omitempty"` // required when Stream == "list"
}

// wsServerMsg is the Mastodon wire format for WebSocket server-to-client messages.
// Stream is always a JSON array per the Mastodon spec.
type wsServerMsg struct {
	Stream  []string `json:"stream"`
	Event   string   `json:"event"`
	Payload string   `json:"payload"`
}

// GETStreamingWS handles GET /api/v1/streaming with an Upgrade: websocket header.
// It upgrades the connection and runs the Mastodon multiplexed streaming protocol.
func (h *StreamingHandler) GETStreamingWS(w http.ResponseWriter, r *http.Request) {
	// InsecureSkipVerify disables the same-origin check that coder/websocket
	// applies to browser requests. Mastodon native clients (Ivory, Tusky, etc.)
	// do not send an Origin header, and the Mastodon reference implementation
	// also skips this check, so it is expected to be disabled.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		// websocket.Accept writes the error response itself.
		slog.WarnContext(r.Context(), "ws: upgrade failed", slog.Any("error", err))
		return
	}
	defer func() { _ = conn.CloseNow() }()

	account := middleware.AccountFromContext(r.Context())

	// Handle optional initial ?stream= subscription.
	var initial []wsClientMsg
	if streamParam := r.URL.Query().Get("stream"); streamParam != "" {
		initial = append(initial, wsClientMsg{
			Type:   wsClientMsgSubscribe,
			Stream: streamParam,
			Tag:    r.URL.Query().Get("tag"),
			List:   r.URL.Query().Get("list"),
		})
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	h.serveWS(ctx, conn, account, cancel, initial)
}

// serveWS manages the WebSocket connection lifecycle:
//   - one write goroutine (sole writer to conn)
//   - one ping goroutine
//   - one fan-out goroutine per active subscription (hub channel → outbox)
//   - read loop on calling goroutine (owns the subscriptions map)
func (h *StreamingHandler) serveWS(
	ctx context.Context,
	conn *websocket.Conn,
	account *domain.Account,
	cancel context.CancelFunc,
	initial []wsClientMsg,
) {
	outbox := make(chan wsServerMsg, 64)
	subscriptions := make(map[string]func()) // streamKey → cancel func
	defer func() {
		for _, cancelSub := range subscriptions {
			cancelSub()
		}
	}()

	go h.wsWriteLoop(ctx, conn, outbox)
	go h.wsPingLoop(ctx, conn, cancel)

	// Process any initial subscriptions before entering the read loop.
	for _, msg := range initial {
		h.wsHandleSubscribe(ctx, msg, account, subscriptions, outbox)
	}

	h.wsReadLoop(ctx, conn, account, subscriptions, outbox)
}

// wsReadLoop reads client messages and dispatches subscribe/unsubscribe commands.
// It is the sole owner of the subscriptions map and runs until the connection closes.
func (h *StreamingHandler) wsReadLoop(
	ctx context.Context,
	conn *websocket.Conn,
	account *domain.Account,
	subscriptions map[string]func(),
	outbox chan<- wsServerMsg,
) {
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}

		var msg wsClientMsg
		if err := json.Unmarshal(data, &msg); err != nil {
			slog.DebugContext(ctx, "ws: invalid client message", slog.Any("error", err))
			continue
		}

		switch msg.Type {
		case wsClientMsgSubscribe:
			h.wsHandleSubscribe(ctx, msg, account, subscriptions, outbox)
		case wsClientMsgUnsubscribe:
			streamKey, _, err := h.resolveWSStreamKey(ctx, msg, account)
			if err != nil {
				slog.DebugContext(ctx, "ws: invalid unsubscribe", slog.String("stream", msg.Stream), slog.Any("error", err))
				continue
			}
			if cancelSub, ok := subscriptions[streamKey]; ok {
				cancelSub()
				delete(subscriptions, streamKey)
			}
		}
	}
}

// wsHandleSubscribe validates a subscribe request and starts a fan-out goroutine.
func (h *StreamingHandler) wsHandleSubscribe(
	ctx context.Context,
	msg wsClientMsg,
	account *domain.Account,
	subscriptions map[string]func(),
	outbox chan<- wsServerMsg,
) {
	if len(subscriptions) >= wsMaxSubscriptions {
		slog.WarnContext(ctx, "ws: max subscriptions reached")
		return
	}

	streamKey, streamLabel, err := h.resolveWSStreamKey(ctx, msg, account)
	if err != nil {
		if errors.Is(err, api.ErrUnauthorized) {
			slog.DebugContext(ctx, "ws: auth required for stream", slog.String("stream", msg.Stream))
		} else {
			slog.DebugContext(ctx, "ws: invalid subscribe", slog.String("stream", msg.Stream), slog.Any("error", err))
		}
		return
	}

	if _, already := subscriptions[streamKey]; already {
		return
	}

	eventCh, cancelSub := h.hub.Subscribe(streamKey)
	if eventCh == nil {
		slog.WarnContext(ctx, "ws: hub returned nil channel", slog.String("stream_key", streamKey))
		cancelSub()
		return
	}

	subscriptions[streamKey] = cancelSub
	go h.wsFanOutLoop(ctx, eventCh, streamLabel, outbox)
}

// wsFanOutLoop reads events from a hub channel and forwards them to the outbox.
// It exits when the hub channel closes or ctx is cancelled.
func (h *StreamingHandler) wsFanOutLoop(
	ctx context.Context,
	eventCh <-chan sse.SSEEvent,
	streamLabel []string,
	outbox chan<- wsServerMsg,
) {
	for {
		select {
		case ev, ok := <-eventCh:
			if !ok {
				return
			}
			msg := wsServerMsg{
				Stream:  streamLabel,
				Event:   ev.Event,
				Payload: ev.Data,
			}
			select {
			case outbox <- msg:
			case <-ctx.Done():
				return
			default:
				slog.WarnContext(ctx, "ws: outbox full, dropping event",
					slog.String("stream", streamLabel[0]),
					slog.String("event", ev.Event),
				)
			}
		case <-ctx.Done():
			return
		}
	}
}

// wsWriteLoop is the sole writer to the WebSocket connection.
// It exits when ctx is cancelled.
func (h *StreamingHandler) wsWriteLoop(ctx context.Context, conn *websocket.Conn, outbox <-chan wsServerMsg) {
	for {
		select {
		case msg, ok := <-outbox:
			if !ok {
				return
			}
			data, err := json.Marshal(msg)
			if err != nil {
				slog.ErrorContext(ctx, "ws: marshal message", slog.Any("error", err))
				continue
			}
			writeCtx, writeCancel := context.WithTimeout(ctx, wsWriteTimeout)
			err = conn.Write(writeCtx, websocket.MessageText, data)
			writeCancel()
			if err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// wsPingLoop sends periodic pings to detect dead connections.
// On ping failure it calls cancel to tear down the connection.
func (h *StreamingHandler) wsPingLoop(ctx context.Context, conn *websocket.Conn, cancel context.CancelFunc) {
	ticker := time.NewTicker(wsPingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			pingCtx, pingCancel := context.WithTimeout(ctx, wsWriteTimeout)
			err := conn.Ping(pingCtx)
			pingCancel()
			if err != nil {
				slog.DebugContext(ctx, "ws: ping failed", slog.Any("error", err))
				cancel()
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// resolveWSStreamKey maps a client subscribe/unsubscribe message to the internal hub stream key
// and the Mastodon wire-format stream label array (always []string).
func (h *StreamingHandler) resolveWSStreamKey(
	ctx context.Context,
	msg wsClientMsg,
	account *domain.Account,
) (streamKey string, streamLabel []string, err error) {
	switch msg.Stream {
	case "user":
		if account == nil {
			return "", nil, api.ErrUnauthorized
		}
		return sse.StreamUserPrefix + account.ID, []string{"user"}, nil

	case "direct":
		if account == nil {
			return "", nil, api.ErrUnauthorized
		}
		return sse.StreamDirectPrefix + account.ID, []string{"direct"}, nil

	case "list":
		if account == nil {
			return "", nil, api.ErrUnauthorized
		}
		listID := strings.TrimSpace(msg.List)
		if listID == "" {
			return "", nil, fmt.Errorf("%w: list is required", api.ErrBadRequest)
		}
		if _, err := h.lists.GetList(ctx, account.ID, listID); err != nil {
			return "", nil, fmt.Errorf("GetList: %w", err)
		}
		return sse.StreamListPrefix + listID, []string{"list", listID}, nil

	case "public":
		return sse.StreamPublic, []string{"public"}, nil

	case "public:local":
		return sse.StreamPublicLocal, []string{"public:local"}, nil

	case "hashtag":
		tag := strings.TrimSpace(msg.Tag)
		tag = strings.TrimPrefix(strings.ToLower(tag), "#")
		if tag == "" {
			return "", nil, fmt.Errorf("%w: tag is required", api.ErrBadRequest)
		}
		return sse.StreamHashtagPrefix + tag, []string{"hashtag", tag}, nil

	default:
		return "", nil, fmt.Errorf("%w: unknown stream %s", api.ErrBadRequest, msg.Stream)
	}
}
