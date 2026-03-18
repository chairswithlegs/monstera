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
	"github.com/chairswithlegs/monstera/internal/webpush"
)

// PushSubscriptionLister lists push subscriptions for an account.
type PushSubscriptionLister interface {
	ListByAccountID(ctx context.Context, accountID string) ([]domain.PushSubscription, error)
}

// PushSubscriptionDeleter deletes a push subscription.
type PushSubscriptionDeleter interface {
	Delete(ctx context.Context, accessTokenID string) error
}

// PushDeliveryDeps groups the dependencies for the push delivery subscriber.
type PushDeliveryDeps struct {
	PushSubs PushSubscriptionLister
	Deleter  PushSubscriptionDeleter
	Sender   webpush.Sender
}

// PushDeliverySubscriber consumes notification.created events and delivers Web Push notifications.
type PushDeliverySubscriber struct {
	js   jetstream.JetStream
	deps PushDeliveryDeps
}

func NewPushDeliverySubscriber(js jetstream.JetStream, deps PushDeliveryDeps) *PushDeliverySubscriber {
	return &PushDeliverySubscriber{js: js, deps: deps}
}

func (p *PushDeliverySubscriber) Start(ctx context.Context) error {
	if err := natsutil.RunConsumer(ctx, p.js, StreamDomainEvents, ConsumerPushDelivery,
		func(msg jetstream.Msg) { go p.processMessage(ctx, msg) },
		natsutil.WithLabel("push delivery subscriber"),
	); err != nil {
		return fmt.Errorf("push delivery subscriber: %w", err)
	}
	return nil
}

func (p *PushDeliverySubscriber) processMessage(ctx context.Context, msg jetstream.Msg) {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "push delivery: panic", slog.Any("panic", r), slog.String("subject", msg.Subject()))
			_ = msg.Nak()
		}
	}()
	var event domain.DomainEvent
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		slog.WarnContext(ctx, "push delivery: invalid event", slog.Any("error", err))
		_ = msg.Ack()
		return
	}
	if event.EventType != domain.EventNotificationCreated {
		_ = msg.Ack()
		return
	}
	var payload domain.NotificationCreatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		slog.WarnContext(ctx, "push delivery: bad notification payload", slog.Any("error", err))
		_ = msg.Ack()
		return
	}
	p.deliver(ctx, payload)
	_ = msg.Ack()
}

func (p *PushDeliverySubscriber) deliver(ctx context.Context, payload domain.NotificationCreatedPayload) {
	subs, err := p.deps.PushSubs.ListByAccountID(ctx, payload.RecipientAccountID)
	if err != nil {
		slog.ErrorContext(ctx, "push delivery: list subscriptions", slog.Any("error", err))
		return
	}
	if len(subs) == 0 {
		return
	}
	notifType := ""
	if payload.Notification != nil {
		notifType = payload.Notification.Type
	}
	pushPayload := buildPushPayload(payload, notifType)
	for i := range subs {
		sub := &subs[i]
		if !alertEnabled(sub.Alerts, notifType) {
			continue
		}
		if err := p.deps.Sender.Send(ctx, sub, pushPayload); err != nil {
			if errors.Is(err, webpush.ErrSubscriptionGone) {
				slog.InfoContext(ctx, "push delivery: subscription gone, deleting",
					slog.String("endpoint", sub.Endpoint))
				_ = p.deps.Deleter.Delete(ctx, sub.AccessTokenID)
				continue
			}
			slog.WarnContext(ctx, "push delivery: send failed",
				slog.Any("error", err),
				slog.String("endpoint", sub.Endpoint),
			)
		}
	}
}

func alertEnabled(alerts domain.PushAlerts, notifType string) bool {
	switch notifType {
	case domain.NotificationTypeFollow:
		return alerts.Follow
	case domain.NotificationTypeFavourite:
		return alerts.Favourite
	case domain.NotificationTypeReblog:
		return alerts.Reblog
	case domain.NotificationTypeMention:
		return alerts.Mention
	case domain.NotificationTypeFollowRequest:
		return alerts.FollowRequest
	default:
		return false
	}
}

type pushNotificationPayload struct {
	NotificationID   string `json:"notification_id"`
	NotificationType string `json:"notification_type"`
	Title            string `json:"title"`
	Body             string `json:"body"`
}

func buildPushPayload(payload domain.NotificationCreatedPayload, notifType string) []byte {
	title := notifType
	body := ""
	if payload.FromAccount != nil {
		displayName := payload.FromAccount.Username
		if payload.FromAccount.DisplayName != nil && *payload.FromAccount.DisplayName != "" {
			displayName = *payload.FromAccount.DisplayName
		}
		switch notifType {
		case domain.NotificationTypeFollow:
			body = displayName + " followed you"
		case domain.NotificationTypeFavourite:
			body = displayName + " favourited your post"
		case domain.NotificationTypeReblog:
			body = displayName + " boosted your post"
		case domain.NotificationTypeMention:
			body = displayName + " mentioned you"
		case domain.NotificationTypeFollowRequest:
			body = displayName + " requested to follow you"
		default:
			body = displayName
		}
	}
	notifID := ""
	if payload.Notification != nil {
		notifID = payload.Notification.ID
	}
	out, _ := json.Marshal(pushNotificationPayload{
		NotificationID:   notifID,
		NotificationType: notifType,
		Title:            title,
		Body:             body,
	})
	return out
}
