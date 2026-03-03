package sse

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/nats-io/nats.go"

	"github.com/chairswithlegs/monstera-fed/internal/observability"
)

const (
	subscriberBufferSize = 16
)

// natsConn is the subset of *nats.Conn used by the Hub (for testing).
type natsConn interface {
	Subscribe(subject string, handler nats.MsgHandler) (interface{ Unsubscribe() error }, error)
}

type subscriber struct {
	ch chan SSEEvent
}

type managedSub struct {
	sub      interface{ Unsubscribe() error }
	refCount int
}

// Hub fans out NATS core pub/sub messages to connected SSE clients.
type Hub struct {
	nc      natsConn
	metrics *observability.Metrics

	mu          sync.RWMutex
	subscribers map[string][]*subscriber
	onDemand    map[string]*managedSub
	alwaysOn    []interface{ Unsubscribe() error }
	done        chan struct{}
}

// NewHub returns a new Hub. Call Start to begin receiving from NATS.
func NewHub(nc natsConn, metrics *observability.Metrics) *Hub {
	return &Hub{
		nc:          nc,
		metrics:     metrics,
		subscribers: make(map[string][]*subscriber),
		onDemand:    make(map[string]*managedSub),
		done:        make(chan struct{}),
	}
}

// Start subscribes to always-on subjects and blocks until ctx is cancelled.
// On cancellation, unsubscribes all NATS subscriptions and closes subscriber channels.
func (h *Hub) Start(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	handler := func(streamKey string) nats.MsgHandler {
		return func(msg *nats.Msg) {
			ev, err := DecodeSSEEvent(msg.Data)
			if err != nil {
				slog.Error("hub: decode event", slog.Any("error", err), slog.String("subject", msg.Subject))
				return
			}
			h.fanOut(streamKey, ev)
		}
	}

	subPublic, err := h.nc.Subscribe(SubjectPrefixPublic, handler(StreamPublic))
	if err != nil {
		return fmt.Errorf("hub: subscribe %s: %w", SubjectPrefixPublic, err)
	}
	h.alwaysOn = append(h.alwaysOn, subPublic)

	subLocal, err := h.nc.Subscribe(SubjectPrefixPublicLocal, handler(StreamPublicLocal))
	if err != nil {
		for _, s := range h.alwaysOn {
			_ = s.Unsubscribe()
		}
		return fmt.Errorf("hub: subscribe %s: %w", SubjectPrefixPublicLocal, err)
	}
	h.alwaysOn = append(h.alwaysOn, subLocal)

	h.mu.Unlock()
	<-ctx.Done()
	h.mu.Lock()

	for _, sub := range h.alwaysOn {
		_ = sub.Unsubscribe()
	}
	h.alwaysOn = nil

	for streamKey, managed := range h.onDemand {
		_ = managed.sub.Unsubscribe()
		delete(h.onDemand, streamKey)
	}

	for streamKey, subs := range h.subscribers {
		for _, s := range subs {
			close(s.ch)
		}
		delete(h.subscribers, streamKey)
	}

	close(h.done)
	return nil
}

// Subscribe adds a new SSE client for the given stream key. Returns the event channel
// and a cancel function to call when the client disconnects.
func (h *Hub) Subscribe(streamKey string) (<-chan SSEEvent, func()) {
	ch := make(chan SSEEvent, subscriberBufferSize)
	sub := &subscriber{ch: ch}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.subscribers[streamKey] = append(h.subscribers[streamKey], sub)

	needNatsSub := false
	if managed, ok := h.onDemand[streamKey]; ok {
		managed.refCount++
	} else {
		needNatsSub = true
	}

	if needNatsSub {
		natsSub, err := h.nc.Subscribe(StreamKeyToSubject(streamKey), func(msg *nats.Msg) {
			ev, err := DecodeSSEEvent(msg.Data)
			if err != nil {
				slog.Error("hub: decode event", slog.Any("error", err), slog.String("subject", msg.Subject))
				return
			}
			h.fanOut(streamKey, ev)
		})
		if err != nil {
			h.subscribers[streamKey] = h.subscribers[streamKey][:len(h.subscribers[streamKey])-1]
			h.mu.Unlock()
			slog.Error("hub: subscribe", slog.Any("error", err), slog.String("stream_key", streamKey))
			h.mu.Lock()
			return nil, func() {}
		}
		h.onDemand[streamKey] = &managedSub{sub: natsSub, refCount: 1}
	}

	label := StreamKeyMetricLabel(streamKey)
	h.metrics.ActiveSSEConnections.WithLabelValues(label).Inc()

	cancel := func() {
		h.mu.Lock()
		defer h.mu.Unlock()

		subs := h.subscribers[streamKey]
		for i, s := range subs {
			if s == sub {
				subs[i] = subs[len(subs)-1]
				h.subscribers[streamKey] = subs[:len(subs)-1]
				if len(h.subscribers[streamKey]) == 0 {
					delete(h.subscribers, streamKey)
				}
				break
			}
		}
		close(sub.ch)

		if managed, ok := h.onDemand[streamKey]; ok {
			managed.refCount--
			if managed.refCount <= 0 {
				_ = managed.sub.Unsubscribe()
				delete(h.onDemand, streamKey)
			}
		}

		h.metrics.ActiveSSEConnections.WithLabelValues(label).Dec()
	}

	return ch, cancel
}

// fanOut sends the event to all subscribers for the stream key. Non-blocking; drops if channel full.
func (h *Hub) fanOut(streamKey string, ev SSEEvent) {
	h.mu.RLock()
	subs := h.subscribers[streamKey]
	if len(subs) == 0 {
		h.mu.RUnlock()
		return
	}
	snapshot := make([]*subscriber, len(subs))
	copy(snapshot, subs)
	h.mu.RUnlock()

	for _, s := range snapshot {
		select {
		case s.ch <- ev:
		default:
			slog.Warn("hub: subscriber channel full, dropping event", slog.String("stream_key", streamKey), slog.String("event", ev.Event))
		}
	}
}
