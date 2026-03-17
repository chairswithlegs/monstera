package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

const (
	defaultNotificationLimit = 20
	maxNotificationLimit     = 40
)

// NotificationService handles read and management of notifications.
// Notification creation is handled by the NotificationSubscriber
// (internal/events/notification_subscriber.go) which reacts to domain events.
type NotificationService interface {
	List(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Notification, error)
	Get(ctx context.Context, id, accountID string) (*domain.Notification, error)
	Clear(ctx context.Context, accountID string) error
	Dismiss(ctx context.Context, id, accountID string) error
	CreateAndEmit(ctx context.Context, recipientID, fromAccountID, notifType string, statusID *string) error
}

type notificationService struct {
	store store.Store
}

// NewNotificationService returns a NotificationService.
func NewNotificationService(s store.Store) NotificationService {
	return &notificationService{store: s}
}

// List returns notifications for the account with cursor pagination.
// limit is clamped to [1, maxNotificationLimit]; default defaultNotificationLimit if <= 0.
func (svc *notificationService) List(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Notification, error) {
	l := limit
	if l <= 0 {
		l = defaultNotificationLimit
	}
	if l > maxNotificationLimit {
		l = maxNotificationLimit
	}
	rows, err := svc.store.ListNotifications(ctx, accountID, maxID, l)
	if err != nil {
		return nil, fmt.Errorf("ListNotifications: %w", err)
	}
	return rows, nil
}

// Get returns a single notification by ID. Caller must ensure accountID is the owner.
func (svc *notificationService) Get(ctx context.Context, id, accountID string) (*domain.Notification, error) {
	n, err := svc.store.GetNotification(ctx, id, accountID)
	if err != nil {
		return nil, fmt.Errorf("GetNotification: %w", err)
	}
	return n, nil
}

// Clear removes all notifications for the account.
func (svc *notificationService) Clear(ctx context.Context, accountID string) error {
	if err := svc.store.ClearNotifications(ctx, accountID); err != nil {
		return fmt.Errorf("ClearNotifications: %w", err)
	}
	return nil
}

// Dismiss removes a single notification by ID. Caller must ensure accountID is the owner.
func (svc *notificationService) Dismiss(ctx context.Context, id, accountID string) error {
	if err := svc.store.DismissNotification(ctx, id, accountID); err != nil {
		return fmt.Errorf("DismissNotification: %w", err)
	}
	return nil
}

// CreateAndEmit atomically creates a notification and emits a notification.created
// event within a single transaction.
func (svc *notificationService) CreateAndEmit(ctx context.Context, recipientID, fromAccountID, notifType string, statusID *string) error {
	if err := svc.store.WithTx(ctx, func(tx store.Store) error {
		notif, err := tx.CreateNotification(ctx, store.CreateNotificationInput{
			ID:        uid.New(),
			AccountID: recipientID,
			FromID:    fromAccountID,
			Type:      notifType,
			StatusID:  statusID,
		})
		if err != nil {
			return fmt.Errorf("CreateNotification: %w", err)
		}
		fromAccount, err := tx.GetAccountByID(ctx, fromAccountID)
		if err != nil {
			return fmt.Errorf("GetAccountByID(from): %w", err)
		}
		payload := domain.NotificationCreatedPayload{
			RecipientAccountID: recipientID,
			Notification:       notif,
			FromAccount:        fromAccount,
			StatusID:           statusID,
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal notification.created payload: %w", err)
		}
		return tx.InsertOutboxEvent(ctx, store.InsertOutboxEventInput{
			ID:            uid.New(),
			EventType:     domain.EventNotificationCreated,
			AggregateType: "notification",
			AggregateID:   notif.ID,
			Payload:       raw,
		})
	}); err != nil {
		return fmt.Errorf("CreateAndEmit: %w", err)
	}
	return nil
}
