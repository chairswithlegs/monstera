// Package events defines the application event bus contract for pub-sub style events.
package events

import (
	"context"

	"github.com/chairswithlegs/monstera-fed/internal/domain"
)

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
