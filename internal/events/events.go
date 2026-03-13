// Package events defines the application event bus contract for pub-sub style events.
package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// EmitEvent writes a domain event to the transactional outbox within the
// current transaction. The event is published to NATS by the outbox poller.
func EmitEvent(ctx context.Context, tx store.Store, eventType, aggregateType, aggregateID string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal %s event payload: %w", eventType, err)
	}
	if err := tx.InsertOutboxEvent(ctx, store.InsertOutboxEventInput{
		ID:            uid.New(),
		EventType:     eventType,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		Payload:       raw,
	}); err != nil {
		return fmt.Errorf("emit %s event: %w", eventType, err)
	}
	return nil
}

// StatusCreatedEvent carries the data needed to publish a status create event.
type StatusCreatedEvent struct {
	Status              *domain.Status
	Author              *domain.Account
	Mentions            []*domain.Account
	Tags                []domain.Hashtag
	Media               []domain.MediaAttachment
	MentionedAccountIDs []string // for direct visibility: local account IDs to notify
}

// StatusDeletedEvent carries the data needed to publish a status delete event.
type StatusDeletedEvent struct {
	StatusID            string
	AccountID           string
	Visibility          string
	Local               bool
	HashtagNames        []string
	MentionedAccountIDs []string
}

// NotificationCreatedEvent carries the data needed to publish a notification event.
type NotificationCreatedEvent struct {
	RecipientAccountID string
	Notification       *domain.Notification
	FromAccount        *domain.Account
	StatusID           *string
}

// EventBus publishes real-time events (e.g. SSE) for status and notification changes.
// Methods are fire-and-forget: they do not return errors; failures are logged by the implementation.
type EventBus interface {
	PublishStatusCreated(ctx context.Context, data StatusCreatedEvent)
	PublishStatusDeleted(ctx context.Context, data StatusDeletedEvent)
	PublishNotificationCreated(ctx context.Context, data NotificationCreatedEvent)
}

// NoopEventBus is an EventBus that does nothing.
var NoopEventBus EventBus = (*noopEventBus)(nil)

type noopEventBus struct{}

func (*noopEventBus) PublishStatusCreated(context.Context, StatusCreatedEvent)             {}
func (*noopEventBus) PublishStatusDeleted(context.Context, StatusDeletedEvent)             {}
func (*noopEventBus) PublishNotificationCreated(context.Context, NotificationCreatedEvent) {}
