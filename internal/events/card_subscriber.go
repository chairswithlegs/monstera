package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/natsutil"
)

// CardProcessor is the service interface for fetching and storing link preview cards.
type CardProcessor interface {
	FetchAndStoreCard(ctx context.Context, statusID string) error
}

// CardSubscriber consumes EventStatusCreated events and triggers link preview
// card fetching for local statuses.
type CardSubscriber struct {
	js  jetstream.JetStream
	svc CardProcessor
}

// NewCardSubscriber creates a CardSubscriber.
func NewCardSubscriber(js jetstream.JetStream, svc CardProcessor) *CardSubscriber {
	return &CardSubscriber{js: js, svc: svc}
}

// Start subscribes to the domain-events-cards consumer and processes messages
// until ctx is cancelled.
func (c *CardSubscriber) Start(ctx context.Context) error {
	if err := natsutil.RunConsumer(ctx, c.js, StreamDomainEvents, ConsumerCards,
		func(msg jetstream.Msg) { go c.processMessage(ctx, msg) },
		natsutil.WithLabel("card subscriber"),
	); err != nil {
		return fmt.Errorf("card subscriber: %w", err)
	}
	return nil
}

func (c *CardSubscriber) processMessage(ctx context.Context, msg jetstream.Msg) {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "card subscriber: panic in processMessage", slog.Any("panic", r), slog.String("subject", msg.Subject()))
			_ = msg.Nak()
		}
	}()
	var event domain.DomainEvent
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		slog.WarnContext(ctx, "card subscriber: invalid event payload", slog.Any("error", err))
		_ = msg.Ack()
		return
	}
	// Defensive check: the consumer filter already restricts to status.created
	// and status.created.remote, but guard here in case a message arrives unexpectedly.
	if event.EventType != domain.EventStatusCreated && event.EventType != domain.EventStatusCreatedRemote {
		_ = msg.Ack()
		return
	}
	c.handleStatusCreated(ctx, event)
	_ = msg.Ack()
}

func (c *CardSubscriber) handleStatusCreated(ctx context.Context, event domain.DomainEvent) {
	var payload domain.StatusCreatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		slog.ErrorContext(ctx, "card subscriber: unmarshal status.created", slog.Any("error", err))
		return
	}
	if payload.Status == nil {
		return
	}
	// Errors are warn-logged and the message is still Acked; card fetch failures
	// are non-critical and there is no retry path. Transient store failures will
	// leave the status without a card permanently.
	if err := c.svc.FetchAndStoreCard(ctx, payload.Status.ID); err != nil {
		slog.WarnContext(ctx, "card subscriber: FetchAndStoreCard failed",
			slog.String("status_id", payload.Status.ID),
			slog.Any("error", err),
		)
	}
}
