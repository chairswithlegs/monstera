package service

import (
	"context"
	"fmt"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// UserFilterService manages per-account content filters.
type UserFilterService interface {
	CreateFilter(ctx context.Context, accountID, phrase string, context []string, wholeWord bool, expiresAt *string, irreversible bool) (*domain.UserFilter, error)
	GetFilter(ctx context.Context, accountID, filterID string) (*domain.UserFilter, error)
	ListFilters(ctx context.Context, accountID string) ([]domain.UserFilter, error)
	UpdateFilter(ctx context.Context, accountID, filterID string, phrase string, context []string, wholeWord bool, expiresAt *string, irreversible bool) (*domain.UserFilter, error)
	DeleteFilter(ctx context.Context, accountID, filterID string) error
	GetActiveFiltersByContext(ctx context.Context, accountID, filterContext string) ([]domain.UserFilter, error)
}

type userFilterService struct {
	store store.Store
}

// NewUserFilterService returns a UserFilterService that uses the given store.
func NewUserFilterService(s store.Store) UserFilterService {
	return &userFilterService{store: s}
}

func (svc *userFilterService) CreateFilter(ctx context.Context, accountID, phrase string, context []string, wholeWord bool, expiresAt *string, irreversible bool) (*domain.UserFilter, error) {
	if phrase == "" {
		return nil, fmt.Errorf("CreateFilter: %w", domain.ErrValidation)
	}
	if len(context) == 0 {
		context = []string{domain.FilterContextHome}
	}
	var exp *time.Time
	if expiresAt != nil && *expiresAt != "" {
		t, err := time.Parse(time.RFC3339, *expiresAt)
		if err != nil {
			return nil, fmt.Errorf("CreateFilter expires_at: %w", err)
		}
		exp = &t
	}
	f, err := svc.store.CreateUserFilter(ctx, store.CreateUserFilterInput{
		ID:           uid.New(),
		AccountID:    accountID,
		Phrase:       phrase,
		Context:      context,
		WholeWord:    wholeWord,
		ExpiresAt:    exp,
		Irreversible: irreversible,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateUserFilter: %w", err)
	}
	return f, nil
}

func (svc *userFilterService) GetFilter(ctx context.Context, accountID, filterID string) (*domain.UserFilter, error) {
	f, err := svc.store.GetUserFilterByID(ctx, filterID)
	if err != nil {
		return nil, fmt.Errorf("GetUserFilterByID: %w", err)
	}
	if f.AccountID != accountID {
		return nil, fmt.Errorf("GetFilter: %w", domain.ErrForbidden)
	}
	return f, nil
}

func (svc *userFilterService) ListFilters(ctx context.Context, accountID string) ([]domain.UserFilter, error) {
	list, err := svc.store.ListUserFilters(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("ListUserFilters: %w", err)
	}
	return list, nil
}

func (svc *userFilterService) UpdateFilter(ctx context.Context, accountID, filterID string, phrase string, context []string, wholeWord bool, expiresAt *string, irreversible bool) (*domain.UserFilter, error) {
	f, err := svc.store.GetUserFilterByID(ctx, filterID)
	if err != nil {
		return nil, fmt.Errorf("UpdateFilter GetUserFilterByID: %w", err)
	}
	if f.AccountID != accountID {
		return nil, fmt.Errorf("UpdateFilter: %w", domain.ErrForbidden)
	}
	if phrase == "" {
		phrase = f.Phrase
	}
	if len(context) == 0 {
		context = f.Context
	}
	var exp *time.Time
	if expiresAt != nil {
		if *expiresAt == "" {
			exp = f.ExpiresAt
		} else {
			t, err := time.Parse(time.RFC3339, *expiresAt)
			if err != nil {
				return nil, fmt.Errorf("UpdateFilter expires_at: %w", err)
			}
			exp = &t
		}
	} else {
		exp = f.ExpiresAt
	}
	updated, err := svc.store.UpdateUserFilter(ctx, store.UpdateUserFilterInput{
		ID:           filterID,
		Phrase:       phrase,
		Context:      context,
		WholeWord:    wholeWord,
		ExpiresAt:    exp,
		Irreversible: irreversible,
	})
	if err != nil {
		return nil, fmt.Errorf("UpdateUserFilter: %w", err)
	}
	return updated, nil
}

func (svc *userFilterService) DeleteFilter(ctx context.Context, accountID, filterID string) error {
	f, err := svc.store.GetUserFilterByID(ctx, filterID)
	if err != nil {
		return fmt.Errorf("DeleteFilter GetUserFilterByID: %w", err)
	}
	if f.AccountID != accountID {
		return fmt.Errorf("DeleteFilter: %w", domain.ErrForbidden)
	}
	if err := svc.store.DeleteUserFilter(ctx, filterID); err != nil {
		return fmt.Errorf("DeleteUserFilter: %w", err)
	}
	return nil
}

func (svc *userFilterService) GetActiveFiltersByContext(ctx context.Context, accountID, filterContext string) ([]domain.UserFilter, error) {
	filters, err := svc.store.GetActiveUserFiltersByContext(ctx, accountID, filterContext)
	if err != nil {
		return nil, fmt.Errorf("GetActiveUserFiltersByContext: %w", err)
	}
	return filters, nil
}
