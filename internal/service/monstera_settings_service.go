package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
)

const defaultServerName = "Monstera"

var defaultMonsteraSettings = domain.MonsteraSettings{
	RegistrationMode: domain.MonsteraRegistrationModeClosed,
}

// applyServerNameDefault fills in the default server name when none has been configured.
func applyServerNameDefault(s domain.MonsteraSettings) domain.MonsteraSettings {
	if s.ServerName == nil {
		name := defaultServerName
		s.ServerName = &name
	}
	return s
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

// Get returns the current Monstera settings. If none exist, returns a default.
// ServerName is always non-nil; if unset in the store it defaults to "Monstera".
func (svc *monsteraSettingsService) Get(ctx context.Context) (domain.MonsteraSettings, error) {
	settings, err := svc.store.GetMonsteraSettings(ctx)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return applyServerNameDefault(defaultMonsteraSettings), fmt.Errorf("GetMonsteraSettings: %w", err)
	}
	if settings == nil {
		return applyServerNameDefault(defaultMonsteraSettings), nil
	}
	return applyServerNameDefault(*settings), nil
}

// Update validates and persists the given settings.
func (svc *monsteraSettingsService) Update(ctx context.Context, in domain.MonsteraSettings) error {
	if err := validateMonsteraSettings(in); err != nil {
		return err
	}
	if in.TrendingLinksScope == "" {
		in.TrendingLinksScope = domain.MonsteraTrendingLinksScopeDisabled
	}
	if err := svc.store.UpdateMonsteraSettings(ctx, &in); err != nil {
		return fmt.Errorf("UpdateMonsteraSettings: %w", err)
	}
	return nil
}

func validateMonsteraSettings(s domain.MonsteraSettings) error {
	switch s.RegistrationMode {
	case domain.MonsteraRegistrationModeOpen, domain.MonsteraRegistrationModeApproval,
		domain.MonsteraRegistrationModeInvite, domain.MonsteraRegistrationModeClosed:
	default:
		return fmt.Errorf("invalid registration_mode %q: %w", s.RegistrationMode, domain.ErrValidation)
	}
	if s.ServerName != nil {
		if *s.ServerName == "" {
			return fmt.Errorf("server_name must not be empty: %w", domain.ErrValidation)
		}
		if len(*s.ServerName) > 24 {
			return fmt.Errorf("server_name must be 24 characters or fewer: %w", domain.ErrValidation)
		}
	}
	switch s.TrendingLinksScope {
	case domain.MonsteraTrendingLinksScopeDisabled, domain.MonsteraTrendingLinksScopeUsers,
		domain.MonsteraTrendingLinksScopeAll:
	case "":
		// Empty is allowed; treated as disabled.
	default:
		return fmt.Errorf("invalid trending_links_scope %q: %w", s.TrendingLinksScope, domain.ErrValidation)
	}
	return nil
}
