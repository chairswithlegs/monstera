package service

import (
	"context"
	"fmt"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/events"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

const (
	defaultNotificationLimit = 20
	maxNotificationLimit     = 40
)

// NotificationService handles the creation and management of notifications.
type NotificationService interface {
	Create(ctx context.Context, accountID, fromID, notifType string, statusID *string) error
	CreateAndEmit(ctx context.Context, recipientID string, fromAccount *domain.Account, notifType string, statusID *string) error
	List(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Notification, error)
	Get(ctx context.Context, id, accountID string) (*domain.Notification, error)
	Clear(ctx context.Context, accountID string) error
	Dismiss(ctx context.Context, id, accountID string) error
}

type notificationService struct {
	store store.Store
}

// NewNotificationService returns a NotificationService.
func NewNotificationService(s store.Store) NotificationService {
	return &notificationService{store: s}
}

// Create creates a single notification (e.g. for inbox follow, mention, favourite, reblog).
func (svc *notificationService) Create(ctx context.Context, accountID, fromID, notifType string, statusID *string) error {
	_, err := svc.store.CreateNotification(ctx, store.CreateNotificationInput{
		ID:        uid.New(),
		AccountID: accountID,
		FromID:    fromID,
		Type:      notifType,
		StatusID:  statusID,
	})
	if err != nil {
		return fmt.Errorf("CreateNotification: %w", err)
	}
	return nil
}

// CreateAndEmit creates a notification and emits a notification.created domain event.
func (svc *notificationService) CreateAndEmit(ctx context.Context, recipientID string, fromAccount *domain.Account, notifType string, statusID *string) error {
	notif, err := svc.store.CreateNotification(ctx, store.CreateNotificationInput{
		ID:        uid.New(),
		AccountID: recipientID,
		FromID:    fromAccount.ID,
		Type:      notifType,
		StatusID:  statusID,
	})
	if err != nil {
		return fmt.Errorf("CreateNotification: %w", err)
	}
	if err := events.EmitEvent(ctx, svc.store, domain.EventNotificationCreated, "notification", notif.ID, domain.NotificationCreatedPayload{
		RecipientAccountID: recipientID,
		Notification:       notif,
		FromAccount:        fromAccount,
		StatusID:           statusID,
	}); err != nil {
		return fmt.Errorf("emit notification event: %w", err)
	}
	return nil
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
