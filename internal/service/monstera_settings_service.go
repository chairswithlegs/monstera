package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
)

var defaultMonsteraSettings = domain.MonsteraSettings{
	RegistrationMode: domain.MonsteraRegistrationModeClosed,
}

// MonsteraSettingsService provides read/write access to server-wide Monstera settings.
type MonsteraSettingsService interface {
	Get(ctx context.Context) (domain.MonsteraSettings, error)
	Update(ctx context.Context, in domain.MonsteraSettings) error
}

type monsteraSettingsService struct {
	store store.Store
}

// NewMonsteraSettingsService returns a MonsteraSettingsService that uses the given store.
func NewMonsteraSettingsService(s store.Store) MonsteraSettingsService {
	return &monsteraSettingsService{store: s}
}

// Get returns the current Monstera settings. If none exist, returns a default (open registration).
func (svc *monsteraSettingsService) Get(ctx context.Context) (domain.MonsteraSettings, error) {
	settings, err := svc.store.GetMonsteraSettings(ctx)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return defaultMonsteraSettings, fmt.Errorf("GetMonsteraSettings: %w", err)
	}
	if settings == nil {
		return defaultMonsteraSettings, nil
	}
	return *settings, nil
}

// Update persists the given settings. RegistrationMode must be one of open, approval, invite, closed.
func (svc *monsteraSettingsService) Update(ctx context.Context, in domain.MonsteraSettings) error {
	switch in.RegistrationMode {
	case domain.MonsteraRegistrationModeOpen, domain.MonsteraRegistrationModeApproval,
		domain.MonsteraRegistrationModeInvite, domain.MonsteraRegistrationModeClosed:
	default:
		return fmt.Errorf("invalid registration_mode %q: %w", in.RegistrationMode, domain.ErrValidation)
	}
	if err := svc.store.UpdateMonsteraSettings(ctx, &in); err != nil {
		return fmt.Errorf("UpdateMonsteraSettings: %w", err)
	}
	return nil
}
