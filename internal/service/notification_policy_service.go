package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

const (
	defaultNotificationRequestLimit = 20
	maxNotificationRequestLimit     = 40
)

// NotificationPolicyService manages notification filtering policy and requests.
type NotificationPolicyService interface {
	// GetOrCreatePolicy returns the account's notification policy, creating one with defaults if absent.
	GetOrCreatePolicy(ctx context.Context, accountID string) (*domain.NotificationPolicy, error)
	// UpdatePolicy applies partial filter-setting updates to the account's policy.
	// Nil pointer fields are left unchanged.
	UpdatePolicy(ctx context.Context, in UpdateNotificationPolicyInput) (*domain.NotificationPolicy, error)
	// PolicySummary returns pending request and notification counts for the account.
	PolicySummary(ctx context.Context, accountID string) (pendingRequests, pendingNotifications int64, err error)
	// ListRequests returns paginated notification requests for the account.
	ListRequests(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.NotificationRequest, error)
	// GetRequest returns a single notification request.
	GetRequest(ctx context.Context, id, accountID string) (*domain.NotificationRequest, error)
	// AcceptRequest removes the notification request (allows future notifications through).
	AcceptRequest(ctx context.Context, id, accountID string) error
	// DismissRequest removes the notification request.
	DismissRequest(ctx context.Context, id, accountID string) error
	// AcceptRequestsByIDs removes multiple notification requests.
	AcceptRequestsByIDs(ctx context.Context, accountID string, ids []string) error
	// DismissRequestsByIDs removes multiple notification requests.
	DismissRequestsByIDs(ctx context.Context, accountID string, ids []string) error
	// UpsertNotificationRequest creates or increments a notification request for the given sender.
	// Called by the notification subscriber when a notification is filtered by policy.
	UpsertNotificationRequest(ctx context.Context, accountID, fromAccountID string, lastStatusID *string) error
}

// UpdateNotificationPolicyInput holds partial filter-setting updates for a policy.
// Nil pointer fields are left unchanged (partial-update semantics).
type UpdateNotificationPolicyInput struct {
	AccountID             string
	FilterNotFollowing    *domain.NotificationFilterPolicy
	FilterNotFollowers    *domain.NotificationFilterPolicy
	FilterNewAccounts     *domain.NotificationFilterPolicy
	FilterPrivateMentions *domain.NotificationFilterPolicy
	ForLimitedAccounts    *domain.NotificationFilterPolicy
}

type notificationPolicyService struct {
	store store.Store
}

// NewNotificationPolicyService returns a NotificationPolicyService.
func NewNotificationPolicyService(s store.Store) NotificationPolicyService {
	return &notificationPolicyService{store: s}
}

func (svc *notificationPolicyService) GetOrCreatePolicy(ctx context.Context, accountID string) (*domain.NotificationPolicy, error) {
	p, err := svc.store.GetNotificationPolicyByAccountID(ctx, accountID)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("GetOrCreatePolicy: %w", err)
		}
		// Create with defaults.
		p, err = svc.store.UpsertNotificationPolicy(ctx, accountID)
		if err != nil {
			return nil, fmt.Errorf("GetOrCreatePolicy upsert: %w", err)
		}
	}
	return p, nil
}

func (svc *notificationPolicyService) UpdatePolicy(ctx context.Context, in UpdateNotificationPolicyInput) (*domain.NotificationPolicy, error) {
	// Get or create the current policy to apply partial updates against.
	current, err := svc.GetOrCreatePolicy(ctx, in.AccountID)
	if err != nil {
		return nil, fmt.Errorf("UpdatePolicy get: %w", err)
	}
	// Merge non-nil fields.
	if in.FilterNotFollowing != nil {
		current.FilterNotFollowing = *in.FilterNotFollowing
	}
	if in.FilterNotFollowers != nil {
		current.FilterNotFollowers = *in.FilterNotFollowers
	}
	if in.FilterNewAccounts != nil {
		current.FilterNewAccounts = *in.FilterNewAccounts
	}
	if in.FilterPrivateMentions != nil {
		current.FilterPrivateMentions = *in.FilterPrivateMentions
	}
	if in.ForLimitedAccounts != nil {
		current.ForLimitedAccounts = *in.ForLimitedAccounts
	}
	p, err := svc.store.UpdateNotificationPolicy(ctx, store.UpdateNotificationPolicyInput{
		AccountID:             in.AccountID,
		FilterNotFollowing:    current.FilterNotFollowing,
		FilterNotFollowers:    current.FilterNotFollowers,
		FilterNewAccounts:     current.FilterNewAccounts,
		FilterPrivateMentions: current.FilterPrivateMentions,
		ForLimitedAccounts:    current.ForLimitedAccounts,
	})
	if err != nil {
		return nil, fmt.Errorf("UpdatePolicy: %w", err)
	}
	return p, nil
}

func (svc *notificationPolicyService) PolicySummary(ctx context.Context, accountID string) (int64, int64, error) {
	reqCount, err := svc.store.CountPendingNotificationRequests(ctx, accountID)
	if err != nil {
		return 0, 0, fmt.Errorf("PolicySummary requests: %w", err)
	}
	notifCount, err := svc.store.CountPendingNotifications(ctx, accountID)
	if err != nil {
		return 0, 0, fmt.Errorf("PolicySummary notifications: %w", err)
	}
	return reqCount, notifCount, nil
}

func (svc *notificationPolicyService) ListRequests(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.NotificationRequest, error) {
	l := limit
	if l <= 0 {
		l = defaultNotificationRequestLimit
	}
	if l > maxNotificationRequestLimit {
		l = maxNotificationRequestLimit
	}
	rows, err := svc.store.ListNotificationRequests(ctx, accountID, maxID, l)
	if err != nil {
		return nil, fmt.Errorf("ListRequests: %w", err)
	}
	return rows, nil
}

func (svc *notificationPolicyService) GetRequest(ctx context.Context, id, accountID string) (*domain.NotificationRequest, error) {
	r, err := svc.store.GetNotificationRequestByID(ctx, id, accountID)
	if err != nil {
		return nil, fmt.Errorf("GetRequest: %w", err)
	}
	return r, nil
}

func (svc *notificationPolicyService) AcceptRequest(ctx context.Context, id, accountID string) error {
	if err := svc.store.DeleteNotificationRequest(ctx, id, accountID); err != nil {
		return fmt.Errorf("AcceptRequest: %w", err)
	}
	return nil
}

func (svc *notificationPolicyService) DismissRequest(ctx context.Context, id, accountID string) error {
	if err := svc.store.DeleteNotificationRequest(ctx, id, accountID); err != nil {
		return fmt.Errorf("DismissRequest: %w", err)
	}
	return nil
}

func (svc *notificationPolicyService) AcceptRequestsByIDs(ctx context.Context, accountID string, ids []string) error {
	if err := svc.store.DeleteNotificationRequestsByIDs(ctx, accountID, ids); err != nil {
		return fmt.Errorf("AcceptRequestsByIDs: %w", err)
	}
	return nil
}

func (svc *notificationPolicyService) DismissRequestsByIDs(ctx context.Context, accountID string, ids []string) error {
	if err := svc.store.DeleteNotificationRequestsByIDs(ctx, accountID, ids); err != nil {
		return fmt.Errorf("DismissRequestsByIDs: %w", err)
	}
	return nil
}

func (svc *notificationPolicyService) UpsertNotificationRequest(ctx context.Context, accountID, fromAccountID string, lastStatusID *string) error {
	_, err := svc.store.UpsertNotificationRequest(ctx, store.UpsertNotificationRequestInput{
		ID:            uid.New(),
		AccountID:     accountID,
		FromAccountID: fromAccountID,
		LastStatusID:  lastStatusID,
	})
	if err != nil {
		return fmt.Errorf("UpsertNotificationRequest: %w", err)
	}
	return nil
}
