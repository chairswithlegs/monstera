package service

import (
	"context"
	"fmt"

	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	"github.com/chairswithlegs/monstera-fed/internal/uid"
)

// ServerFilterService provides server-side content filter CRUD for admin.
type ServerFilterService interface {
	ListServerFilters(ctx context.Context) ([]domain.ServerFilter, error)
	CreateServerFilter(ctx context.Context, phrase, scope, action string, wholeWord bool) (*domain.ServerFilter, error)
	UpdateServerFilter(ctx context.Context, id, phrase, scope, action string, wholeWord bool) (*domain.ServerFilter, error)
	DeleteServerFilter(ctx context.Context, id string) error
}

type serverFilterService struct {
	store store.Store
}

// NewServerFilterService returns a ServerFilterService that uses the given store.
func NewServerFilterService(s store.Store) ServerFilterService {
	return &serverFilterService{store: s}
}

func (svc *serverFilterService) ListServerFilters(ctx context.Context) ([]domain.ServerFilter, error) {
	filters, err := svc.store.ListServerFilters(ctx)
	if err != nil {
		return nil, fmt.Errorf("ListServerFilters: %w", err)
	}
	return filters, nil
}

func (svc *serverFilterService) CreateServerFilter(ctx context.Context, phrase, scope, action string, wholeWord bool) (*domain.ServerFilter, error) {
	if scope == "" {
		scope = domain.ServerFilterScopeAll
	}
	if action == "" {
		action = domain.ServerFilterActionHide
	}
	filter, err := svc.store.CreateServerFilter(ctx, store.CreateServerFilterInput{
		ID:        uid.New(),
		Phrase:    phrase,
		Scope:     scope,
		Action:    action,
		WholeWord: wholeWord,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateServerFilter: %w", err)
	}
	return filter, nil
}

func (svc *serverFilterService) UpdateServerFilter(ctx context.Context, id, phrase, scope, action string, wholeWord bool) (*domain.ServerFilter, error) {
	filter, err := svc.store.UpdateServerFilter(ctx, store.UpdateServerFilterInput{
		ID:        id,
		Phrase:    phrase,
		Scope:     scope,
		Action:    action,
		WholeWord: wholeWord,
	})
	if err != nil {
		return nil, fmt.Errorf("UpdateServerFilter(%s): %w", id, err)
	}
	return filter, nil
}

func (svc *serverFilterService) DeleteServerFilter(ctx context.Context, id string) error {
	if err := svc.store.DeleteServerFilter(ctx, id); err != nil {
		return fmt.Errorf("DeleteServerFilter(%s): %w", id, err)
	}
	return nil
}
