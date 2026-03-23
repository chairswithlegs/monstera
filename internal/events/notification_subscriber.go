package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/natsutil"
)

// NotificationDeps groups the service dependencies needed by the notification subscriber.
type NotificationDeps struct {
	Notifications NotificationCreator
	Accounts      AccountLookup
	Conversations ConversationMuteChecker
}

// NotificationCreator is the service interface for creating notifications with events.
type NotificationCreator interface {
	CreateAndEmit(ctx context.Context, recipientID, fromAccountID, notifType string, statusID *string) error
}

// AccountLookup is the service interface for looking up accounts by ID.
type AccountLookup interface {
	GetByID(ctx context.Context, id string) (*domain.Account, error)
}

// ConversationMuteChecker checks if a viewer has muted the conversation containing a status.
type ConversationMuteChecker interface {
	IsConversationMutedForViewer(ctx context.Context, viewerAccountID, statusID string) (bool, error)
}

// NotificationSubscriber consumes domain events from DOMAIN_EVENTS and creates
// notifications reactively. This centralizes all notification creation logic,
// removing it from the inbox and inline service code.
type NotificationSubscriber struct {
	js   jetstream.JetStream
	deps NotificationDeps
}

// NewNotificationSubscriber creates a notification subscriber.
func NewNotificationSubscriber(js jetstream.JetStream, deps NotificationDeps) *NotificationSubscriber {
	return &NotificationSubscriber{js: js, deps: deps}
}

// Start subscribes to the domain-events-notifications consumer and processes
// messages until ctx is cancelled.
func (n *NotificationSubscriber) Start(ctx context.Context) error {
	if err := natsutil.RunConsumer(ctx, n.js, StreamDomainEvents, ConsumerNotifications,
		func(msg jetstream.Msg) { go n.processMessage(ctx, msg) },
		natsutil.WithLabel("notification subscriber"),
	); err != nil {
		return fmt.Errorf("notification subscriber: %w", err)
	}
	return nil
}

func (n *NotificationSubscriber) processMessage(ctx context.Context, msg jetstream.Msg) {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "notification subscriber: panic in processMessage", slog.Any("panic", r), slog.String("subject", msg.Subject()))
			_ = msg.Nak()
		}
	}()
	var event domain.DomainEvent
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		slog.WarnContext(ctx, "notification subscriber: invalid event payload", slog.Any("error", err))
		_ = msg.Ack()
		return
	}

	switch event.EventType {
	case domain.EventFollowCreated:
		n.handleFollowCreated(ctx, event)
	case domain.EventFollowRequested:
		n.handleFollowRequested(ctx, event)
	case domain.EventFavouriteCreated:
		n.handleFavouriteCreated(ctx, event)
	case domain.EventReblogCreated:
		n.handleReblogCreated(ctx, event)
	case domain.EventStatusCreated, domain.EventStatusCreatedRemote:
		n.handleStatusCreatedMentions(ctx, event)
	}
	_ = msg.Ack()
}

func (n *NotificationSubscriber) handleFollowCreated(ctx context.Context, event domain.DomainEvent) {
	var payload domain.FollowCreatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		slog.ErrorContext(ctx, "notification subscriber: unmarshal follow.created", slog.Any("error", err))
		return
	}
	if payload.Target == nil || payload.Target.IsRemote() {
		return
	}
	if payload.Actor == nil {
		return
	}
	n.createNotification(ctx, payload.Target.ID, payload.Actor.ID, domain.NotificationTypeFollow, nil)
}

func (n *NotificationSubscriber) handleFollowRequested(ctx context.Context, event domain.DomainEvent) {
	var payload domain.FollowRequestedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		slog.ErrorContext(ctx, "notification subscriber: unmarshal follow.requested", slog.Any("error", err))
		return
	}
	if payload.Target == nil || payload.Target.IsRemote() {
		return
	}
	if payload.Actor == nil {
		return
	}
	n.createNotification(ctx, payload.Target.ID, payload.Actor.ID, domain.NotificationTypeFollowRequest, nil)
}

func (n *NotificationSubscriber) handleFavouriteCreated(ctx context.Context, event domain.DomainEvent) {
	var payload domain.FavouriteCreatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		slog.ErrorContext(ctx, "notification subscriber: unmarshal favourite.created", slog.Any("error", err))
		return
	}
	author, err := n.deps.Accounts.GetByID(ctx, payload.StatusAuthorID)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			slog.WarnContext(ctx, "notification subscriber: GetByID for favourite author",
				slog.String("account_id", payload.StatusAuthorID), slog.Any("error", err))
		}
		return
	}
	if author.IsRemote() {
		return
	}
	if payload.AccountID == payload.StatusAuthorID {
		return
	}
	statusID := payload.StatusID
	n.createNotification(ctx, payload.StatusAuthorID, payload.AccountID, domain.NotificationTypeFavourite, &statusID)
}

func (n *NotificationSubscriber) handleReblogCreated(ctx context.Context, event domain.DomainEvent) {
	var payload domain.ReblogCreatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		slog.ErrorContext(ctx, "notification subscriber: unmarshal reblog.created", slog.Any("error", err))
		return
	}
	author, err := n.deps.Accounts.GetByID(ctx, payload.OriginalAuthorID)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			slog.WarnContext(ctx, "notification subscriber: GetByID for reblog author",
				slog.String("account_id", payload.OriginalAuthorID), slog.Any("error", err))
		}
		return
	}
	if author.IsRemote() {
		return
	}
	if payload.AccountID == payload.OriginalAuthorID {
		return
	}
	statusID := payload.OriginalStatusID
	n.createNotification(ctx, payload.OriginalAuthorID, payload.AccountID, domain.NotificationTypeReblog, &statusID)
}

func (n *NotificationSubscriber) handleStatusCreatedMentions(ctx context.Context, event domain.DomainEvent) {
	var payload domain.StatusCreatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		slog.ErrorContext(ctx, "notification subscriber: unmarshal status.created (mentions)", slog.Any("error", err))
		return
	}
	if payload.Status == nil || payload.Author == nil {
		return
	}
	statusID := payload.Status.ID
	for _, mentioned := range payload.Mentions {
		if mentioned == nil || mentioned.IsRemote() {
			continue
		}
		if mentioned.ID == payload.Author.ID {
			continue
		}
		muted, err := n.deps.Conversations.IsConversationMutedForViewer(ctx, mentioned.ID, statusID)
		if err != nil {
			slog.WarnContext(ctx, "notification subscriber: IsConversationMutedForViewer", slog.Any("error", err))
			continue
		}
		if muted {
			continue
		}
		n.createNotification(ctx, mentioned.ID, payload.Author.ID, domain.NotificationTypeMention, &statusID)
	}
}

func (n *NotificationSubscriber) createNotification(ctx context.Context, recipientID, fromAccountID, notifType string, statusID *string) {
	if err := n.deps.Notifications.CreateAndEmit(ctx, recipientID, fromAccountID, notifType, statusID); err != nil {
		slog.WarnContext(ctx, "notification subscriber: create notification failed",
			slog.String("type", notifType),
			slog.String("recipient", recipientID),
			slog.Any("error", err),
		)
	}
}
