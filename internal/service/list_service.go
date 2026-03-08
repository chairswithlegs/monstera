package service

import (
	"context"
	"fmt"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// ListService manages user lists and list membership.
type ListService interface {
	CreateList(ctx context.Context, accountID string, title, repliesPolicy string, exclusive bool) (*domain.List, error)
	GetList(ctx context.Context, accountID, listID string) (*domain.List, error)
	ListLists(ctx context.Context, accountID string) ([]domain.List, error)
	UpdateList(ctx context.Context, accountID, listID string, title, repliesPolicy string, exclusive bool) (*domain.List, error)
	DeleteList(ctx context.Context, accountID, listID string) error
	ListListAccountIDs(ctx context.Context, listID string) ([]string, error)
	GetListAccounts(ctx context.Context, ownerAccountID, listID string) ([]domain.Account, error)
	AddAccountsToList(ctx context.Context, accountID, listID string, accountIDs []string) error
	RemoveAccountsFromList(ctx context.Context, accountID, listID string, accountIDs []string) error
}

type listService struct {
	store store.Store
}

// NewListService returns a ListService that uses the given store.
func NewListService(s store.Store) ListService {
	return &listService{store: s}
}

func (svc *listService) CreateList(ctx context.Context, accountID string, title, repliesPolicy string, exclusive bool) (*domain.List, error) {
	if title == "" {
		return nil, fmt.Errorf("CreateList: %w", domain.ErrValidation)
	}
	rp := repliesPolicy
	if rp == "" {
		rp = domain.ListRepliesPolicyList
	}
	switch rp {
	case domain.ListRepliesPolicyFollowed, domain.ListRepliesPolicyList, domain.ListRepliesPolicyNone:
	default:
		rp = domain.ListRepliesPolicyList
	}
	l, err := svc.store.CreateList(ctx, store.CreateListInput{
		ID:            uid.New(),
		AccountID:     accountID,
		Title:         title,
		RepliesPolicy: rp,
		Exclusive:     exclusive,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateList: %w", err)
	}
	return l, nil
}

// GetList returns the list for the given account. Returns ErrForbidden if the list is not owned by the account.
func (svc *listService) GetList(ctx context.Context, accountID, listID string) (*domain.List, error) {
	l, err := svc.store.GetListByID(ctx, listID)
	if err != nil {
		return nil, fmt.Errorf("GetList GetListByID: %w", err)
	}
	if l.AccountID != accountID {
		return nil, fmt.Errorf("GetList: %w", domain.ErrForbidden)
	}
	return l, nil
}

func (svc *listService) ListLists(ctx context.Context, accountID string) ([]domain.List, error) {
	lists, err := svc.store.ListLists(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("ListLists: %w", err)
	}
	return lists, nil
}

func (svc *listService) UpdateList(ctx context.Context, accountID, listID string, title, repliesPolicy string, exclusive bool) (*domain.List, error) {
	l, err := svc.store.GetListByID(ctx, listID)
	if err != nil {
		return nil, fmt.Errorf("UpdateList GetListByID: %w", err)
	}
	if l.AccountID != accountID {
		return nil, fmt.Errorf("UpdateList: %w", domain.ErrForbidden)
	}
	rp := repliesPolicy
	if rp == "" {
		rp = l.RepliesPolicy
	}
	switch rp {
	case domain.ListRepliesPolicyFollowed, domain.ListRepliesPolicyList, domain.ListRepliesPolicyNone:
	default:
		rp = l.RepliesPolicy
	}
	if title == "" {
		title = l.Title
	}
	updated, err := svc.store.UpdateList(ctx, store.UpdateListInput{
		ID:            listID,
		Title:         title,
		RepliesPolicy: rp,
		Exclusive:     exclusive,
	})
	if err != nil {
		return nil, fmt.Errorf("UpdateList: %w", err)
	}
	return updated, nil
}

func (svc *listService) DeleteList(ctx context.Context, accountID, listID string) error {
	l, err := svc.store.GetListByID(ctx, listID)
	if err != nil {
		return fmt.Errorf("DeleteList GetListByID: %w", err)
	}
	if l.AccountID != accountID {
		return fmt.Errorf("DeleteList: %w", domain.ErrForbidden)
	}
	if err := svc.store.DeleteList(ctx, listID); err != nil {
		return fmt.Errorf("DeleteList: %w", err)
	}
	return nil
}

func (svc *listService) ListListAccountIDs(ctx context.Context, listID string) ([]string, error) {
	ids, err := svc.store.ListListAccountIDs(ctx, listID)
	if err != nil {
		return nil, fmt.Errorf("ListListAccountIDs: %w", err)
	}
	return ids, nil
}

// GetListAccounts returns accounts in the list for the owner. Returns ErrForbidden if the caller is not the list owner.
// Suspended accounts are omitted from the result.
func (svc *listService) GetListAccounts(ctx context.Context, ownerAccountID, listID string) ([]domain.Account, error) {
	l, err := svc.store.GetListByID(ctx, listID)
	if err != nil {
		return nil, fmt.Errorf("GetListAccounts GetListByID: %w", err)
	}
	if l.AccountID != ownerAccountID {
		return nil, fmt.Errorf("GetListAccounts: %w", domain.ErrForbidden)
	}
	accountIDs, err := svc.store.ListListAccountIDs(ctx, listID)
	if err != nil {
		return nil, fmt.Errorf("GetListAccounts ListListAccountIDs: %w", err)
	}
	if len(accountIDs) == 0 {
		return nil, nil
	}
	accounts, err := svc.store.GetAccountsByIDs(ctx, accountIDs)
	if err != nil {
		return nil, fmt.Errorf("GetListAccounts GetAccountsByIDs: %w", err)
	}
	byID := make(map[string]*domain.Account, len(accounts))
	for _, acc := range accounts {
		byID[acc.ID] = acc
	}
	out := make([]domain.Account, 0, len(accountIDs))
	for _, aid := range accountIDs {
		acc := byID[aid]
		if acc == nil || acc.Suspended {
			continue
		}
		out = append(out, *acc)
	}
	return out, nil
}

func (svc *listService) AddAccountsToList(ctx context.Context, accountID, listID string, accountIDs []string) error {
	l, err := svc.store.GetListByID(ctx, listID)
	if err != nil {
		return fmt.Errorf("AddAccountsToList GetListByID: %w", err)
	}
	if l.AccountID != accountID {
		return fmt.Errorf("AddAccountsToList: %w", domain.ErrForbidden)
	}
	for _, id := range accountIDs {
		if err := svc.store.AddAccountToList(ctx, listID, id); err != nil {
			return fmt.Errorf("AddAccountToList: %w", err)
		}
	}
	return nil
}

func (svc *listService) RemoveAccountsFromList(ctx context.Context, accountID, listID string, accountIDs []string) error {
	l, err := svc.store.GetListByID(ctx, listID)
	if err != nil {
		return fmt.Errorf("RemoveAccountsFromList GetListByID: %w", err)
	}
	if l.AccountID != accountID {
		return fmt.Errorf("RemoveAccountsFromList: %w", domain.ErrForbidden)
	}
	for _, id := range accountIDs {
		if err := svc.store.RemoveAccountFromList(ctx, listID, id); err != nil {
			return fmt.Errorf("RemoveAccountFromList: %w", err)
		}
	}
	return nil
}
