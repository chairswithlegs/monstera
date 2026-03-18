package service

import (
	"context"
	"fmt"
	"net/url"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// PushSubscriptionService manages Web Push subscriptions.
type PushSubscriptionService interface {
	Create(ctx context.Context, accessTokenID, accountID, endpoint, p256dh, auth string, alerts domain.PushAlerts, policy string) (*domain.PushSubscription, error)
	Get(ctx context.Context, accessTokenID string) (*domain.PushSubscription, error)
	Update(ctx context.Context, accessTokenID string, alerts domain.PushAlerts, policy string) (*domain.PushSubscription, error)
	Delete(ctx context.Context, accessTokenID string) error
	ListByAccountID(ctx context.Context, accountID string) ([]domain.PushSubscription, error)
}

type pushSubscriptionService struct {
	store store.Store
}

func NewPushSubscriptionService(s store.Store) PushSubscriptionService {
	return &pushSubscriptionService{store: s}
}

func validatePushEndpoint(endpoint string) error {
	if endpoint == "" {
		return fmt.Errorf("endpoint is required: %w", domain.ErrValidation)
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("endpoint must be a valid HTTPS URL: %w", domain.ErrValidation)
	}
	return nil
}

func (svc *pushSubscriptionService) Create(ctx context.Context, accessTokenID, accountID, endpoint, p256dh, auth string, alerts domain.PushAlerts, policy string) (*domain.PushSubscription, error) {
	if err := validatePushEndpoint(endpoint); err != nil {
		return nil, err
	}
	if p256dh == "" {
		return nil, fmt.Errorf("p256dh key is required: %w", domain.ErrValidation)
	}
	if auth == "" {
		return nil, fmt.Errorf("auth key is required: %w", domain.ErrValidation)
	}
	if policy == "" {
		policy = "all"
	}
	ps, err := svc.store.CreatePushSubscription(ctx, store.CreatePushSubscriptionInput{
		ID:            uid.New(),
		AccessTokenID: accessTokenID,
		AccountID:     accountID,
		Endpoint:      endpoint,
		KeyP256DH:     p256dh,
		KeyAuth:       auth,
		Alerts:        alerts,
		Policy:        policy,
	})
	if err != nil {
		return nil, fmt.Errorf("CreatePushSubscription: %w", err)
	}
	return ps, nil
}

func (svc *pushSubscriptionService) Get(ctx context.Context, accessTokenID string) (*domain.PushSubscription, error) {
	ps, err := svc.store.GetPushSubscription(ctx, accessTokenID)
	if err != nil {
		return nil, fmt.Errorf("GetPushSubscription: %w", err)
	}
	return ps, nil
}

func (svc *pushSubscriptionService) Update(ctx context.Context, accessTokenID string, alerts domain.PushAlerts, policy string) (*domain.PushSubscription, error) {
	ps, err := svc.store.UpdatePushSubscription(ctx, accessTokenID, alerts, policy)
	if err != nil {
		return nil, fmt.Errorf("UpdatePushSubscription: %w", err)
	}
	return ps, nil
}

func (svc *pushSubscriptionService) Delete(ctx context.Context, accessTokenID string) error {
	if err := svc.store.DeletePushSubscription(ctx, accessTokenID); err != nil {
		return fmt.Errorf("DeletePushSubscription: %w", err)
	}
	return nil
}

func (svc *pushSubscriptionService) ListByAccountID(ctx context.Context, accountID string) ([]domain.PushSubscription, error) {
	subs, err := svc.store.ListPushSubscriptionsByAccountID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("ListPushSubscriptionsByAccountID: %w", err)
	}
	return subs, nil
}
