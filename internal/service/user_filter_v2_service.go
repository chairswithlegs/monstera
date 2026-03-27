package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// UserFilterV2Service manages per-account v2 content filters (multi-keyword, filter_action).
type UserFilterV2Service interface {
	// Filter CRUD
	CreateFilter(ctx context.Context, accountID, title string, context []string, expiresAt *string, filterAction string) (*domain.UserFilterV2, error)
	GetFilter(ctx context.Context, accountID, filterID string) (*domain.UserFilterV2, error)
	ListFilters(ctx context.Context, accountID string) ([]domain.UserFilterV2, error)
	UpdateFilter(ctx context.Context, accountID, filterID, title string, context []string, expiresAt *string, filterAction string) (*domain.UserFilterV2, error)
	DeleteFilter(ctx context.Context, accountID, filterID string) error

	// Keyword CRUD
	ListKeywords(ctx context.Context, accountID, filterID string) ([]domain.FilterKeyword, error)
	AddKeyword(ctx context.Context, accountID, filterID, keyword string, wholeWord bool) (*domain.FilterKeyword, error)
	GetKeyword(ctx context.Context, accountID, keywordID string) (*domain.FilterKeyword, error)
	UpdateKeyword(ctx context.Context, accountID, keywordID, keyword string, wholeWord bool) (*domain.FilterKeyword, error)
	DeleteKeyword(ctx context.Context, accountID, keywordID string) error

	// FilterStatus CRUD
	ListFilterStatuses(ctx context.Context, accountID, filterID string) ([]domain.FilterStatus, error)
	AddFilterStatus(ctx context.Context, accountID, filterID, statusID string) (*domain.FilterStatus, error)
	GetFilterStatus(ctx context.Context, accountID, filterStatusID string) (*domain.FilterStatus, error)
	DeleteFilterStatus(ctx context.Context, accountID, filterStatusID string) error

	// Enrichment helper: computes filter results for a status.
	ComputeFilterResults(ctx context.Context, accountID, statusID string, content, cw string) ([]domain.FilterResult, error)
}

type userFilterV2Service struct {
	store store.Store
}

// NewUserFilterV2Service returns a UserFilterV2Service backed by the given store.
func NewUserFilterV2Service(s store.Store) UserFilterV2Service {
	return &userFilterV2Service{store: s}
}

func (svc *userFilterV2Service) parseExpiresAt(expiresAt *string) (*time.Time, error) {
	if expiresAt == nil || *expiresAt == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, *expiresAt)
	if err != nil {
		return nil, fmt.Errorf("expires_at: %w", err)
	}
	return &t, nil
}

func (svc *userFilterV2Service) CreateFilter(ctx context.Context, accountID, title string, context []string, expiresAt *string, filterAction string) (*domain.UserFilterV2, error) {
	if title == "" {
		return nil, fmt.Errorf("CreateFilter: %w", domain.ErrValidation)
	}
	if len(context) == 0 {
		context = []string{domain.FilterContextHome}
	}
	if filterAction == "" {
		filterAction = "warn"
	}
	exp, err := svc.parseExpiresAt(expiresAt)
	if err != nil {
		return nil, fmt.Errorf("CreateFilter: %w", err)
	}
	f, err := svc.store.CreateUserFilterV2(ctx, store.CreateUserFilterV2Input{
		ID:           uid.New(),
		AccountID:    accountID,
		Title:        title,
		Context:      context,
		ExpiresAt:    exp,
		FilterAction: filterAction,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateFilter: %w", err)
	}
	return f, nil
}

func (svc *userFilterV2Service) GetFilter(ctx context.Context, accountID, filterID string) (*domain.UserFilterV2, error) {
	f, err := svc.store.GetUserFilterV2ByID(ctx, filterID)
	if err != nil {
		return nil, fmt.Errorf("GetFilter: %w", err)
	}
	if f.AccountID != accountID {
		return nil, fmt.Errorf("GetFilter: %w", domain.ErrForbidden)
	}
	return f, nil
}

func (svc *userFilterV2Service) ListFilters(ctx context.Context, accountID string) ([]domain.UserFilterV2, error) {
	list, err := svc.store.ListUserFiltersV2(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("ListFilters: %w", err)
	}
	return list, nil
}

func (svc *userFilterV2Service) UpdateFilter(ctx context.Context, accountID, filterID, title string, context []string, expiresAt *string, filterAction string) (*domain.UserFilterV2, error) {
	existing, err := svc.store.GetUserFilterV2ByID(ctx, filterID)
	if err != nil {
		return nil, fmt.Errorf("UpdateFilter: %w", err)
	}
	if existing.AccountID != accountID {
		return nil, fmt.Errorf("UpdateFilter: %w", domain.ErrForbidden)
	}
	if title == "" {
		title = existing.Title
	}
	if len(context) == 0 {
		context = existing.Context
	}
	if filterAction == "" {
		filterAction = existing.FilterAction
	}
	exp, err := svc.parseExpiresAt(expiresAt)
	if err != nil {
		return nil, fmt.Errorf("UpdateFilter: %w", err)
	}
	if expiresAt == nil {
		exp = existing.ExpiresAt
	}
	f, err := svc.store.UpdateUserFilterV2(ctx, store.UpdateUserFilterV2Input{
		ID:           filterID,
		Title:        title,
		Context:      context,
		ExpiresAt:    exp,
		FilterAction: filterAction,
	})
	if err != nil {
		return nil, fmt.Errorf("UpdateFilter: %w", err)
	}
	return f, nil
}

func (svc *userFilterV2Service) DeleteFilter(ctx context.Context, accountID, filterID string) error {
	existing, err := svc.store.GetUserFilterV2ByID(ctx, filterID)
	if err != nil {
		return fmt.Errorf("DeleteFilter: %w", err)
	}
	if existing.AccountID != accountID {
		return fmt.Errorf("DeleteFilter: %w", domain.ErrForbidden)
	}
	if err := svc.store.DeleteUserFilterV2(ctx, filterID); err != nil {
		return fmt.Errorf("DeleteFilter: %w", err)
	}
	return nil
}

func (svc *userFilterV2Service) ListKeywords(ctx context.Context, accountID, filterID string) ([]domain.FilterKeyword, error) {
	if _, err := svc.GetFilter(ctx, accountID, filterID); err != nil {
		return nil, err
	}
	kws, err := svc.store.ListFilterKeywords(ctx, filterID)
	if err != nil {
		return nil, fmt.Errorf("ListKeywords: %w", err)
	}
	return kws, nil
}

func (svc *userFilterV2Service) AddKeyword(ctx context.Context, accountID, filterID, keyword string, wholeWord bool) (*domain.FilterKeyword, error) {
	if _, err := svc.GetFilter(ctx, accountID, filterID); err != nil {
		return nil, err
	}
	kw, err := svc.store.AddFilterKeyword(ctx, filterID, uid.New(), keyword, wholeWord)
	if err != nil {
		return nil, fmt.Errorf("AddKeyword: %w", err)
	}
	return kw, nil
}

func (svc *userFilterV2Service) GetKeyword(ctx context.Context, accountID, keywordID string) (*domain.FilterKeyword, error) {
	kw, err := svc.store.GetFilterKeywordByID(ctx, keywordID)
	if err != nil {
		return nil, fmt.Errorf("GetKeyword: %w", err)
	}
	// Verify ownership via parent filter.
	if _, err := svc.GetFilter(ctx, accountID, kw.FilterID); err != nil {
		return nil, err
	}
	return kw, nil
}

func (svc *userFilterV2Service) UpdateKeyword(ctx context.Context, accountID, keywordID, keyword string, wholeWord bool) (*domain.FilterKeyword, error) {
	kw, err := svc.store.GetFilterKeywordByID(ctx, keywordID)
	if err != nil {
		return nil, fmt.Errorf("UpdateKeyword: %w", err)
	}
	if _, err := svc.GetFilter(ctx, accountID, kw.FilterID); err != nil {
		return nil, err
	}
	updated, err := svc.store.UpdateFilterKeyword(ctx, keywordID, keyword, wholeWord)
	if err != nil {
		return nil, fmt.Errorf("UpdateKeyword: %w", err)
	}
	return updated, nil
}

func (svc *userFilterV2Service) DeleteKeyword(ctx context.Context, accountID, keywordID string) error {
	kw, err := svc.store.GetFilterKeywordByID(ctx, keywordID)
	if err != nil {
		return fmt.Errorf("DeleteKeyword: %w", err)
	}
	if _, err := svc.GetFilter(ctx, accountID, kw.FilterID); err != nil {
		return err
	}
	if err := svc.store.DeleteFilterKeyword(ctx, keywordID); err != nil {
		return fmt.Errorf("DeleteKeyword: %w", err)
	}
	return nil
}

func (svc *userFilterV2Service) ListFilterStatuses(ctx context.Context, accountID, filterID string) ([]domain.FilterStatus, error) {
	if _, err := svc.GetFilter(ctx, accountID, filterID); err != nil {
		return nil, err
	}
	fsts, err := svc.store.ListFilterStatuses(ctx, filterID)
	if err != nil {
		return nil, fmt.Errorf("ListFilterStatuses: %w", err)
	}
	return fsts, nil
}

func (svc *userFilterV2Service) AddFilterStatus(ctx context.Context, accountID, filterID, statusID string) (*domain.FilterStatus, error) {
	if _, err := svc.GetFilter(ctx, accountID, filterID); err != nil {
		return nil, err
	}
	fs, err := svc.store.AddFilterStatus(ctx, uid.New(), filterID, statusID)
	if err != nil {
		return nil, fmt.Errorf("AddFilterStatus: %w", err)
	}
	return fs, nil
}

func (svc *userFilterV2Service) GetFilterStatus(ctx context.Context, accountID, filterStatusID string) (*domain.FilterStatus, error) {
	fs, err := svc.store.GetFilterStatusByID(ctx, filterStatusID)
	if err != nil {
		return nil, fmt.Errorf("GetFilterStatus: %w", err)
	}
	if _, err := svc.GetFilter(ctx, accountID, fs.FilterID); err != nil {
		return nil, err
	}
	return fs, nil
}

func (svc *userFilterV2Service) DeleteFilterStatus(ctx context.Context, accountID, filterStatusID string) error {
	fs, err := svc.store.GetFilterStatusByID(ctx, filterStatusID)
	if err != nil {
		return fmt.Errorf("DeleteFilterStatus: %w", err)
	}
	if _, err := svc.GetFilter(ctx, accountID, fs.FilterID); err != nil {
		return err
	}
	if err := svc.store.DeleteFilterStatus(ctx, filterStatusID); err != nil {
		return fmt.Errorf("DeleteFilterStatus: %w", err)
	}
	return nil
}

// ComputeFilterResults returns the list of FilterResult for a status given the viewer's active filters.
// content and cw are the plain-text content and content warning of the status.
func (svc *userFilterV2Service) ComputeFilterResults(ctx context.Context, accountID, statusID string, content, cw string) ([]domain.FilterResult, error) {
	filters, err := svc.store.GetActiveUserFiltersV2(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("ComputeFilterResults: %w", err)
	}
	return computeFilterResults(filters, statusID, content, cw), nil
}

// computeFilterResults matches a set of active filters against a status and returns
// the list of FilterResult entries. It is a pure function for testability.
func computeFilterResults(filters []domain.UserFilterV2, statusID, content, cw string) []domain.FilterResult {
	text := cw + " " + content
	var out []domain.FilterResult
	for _, f := range filters {
		var kwMatches []string
		for _, kw := range f.Keywords {
			if matchesKeyword(text, kw.Keyword, kw.WholeWord) {
				kwMatches = append(kwMatches, kw.Keyword)
			}
		}
		var stMatches []string
		for _, fs := range f.Statuses {
			if fs.StatusID == statusID {
				stMatches = append(stMatches, statusID)
			}
		}
		if len(kwMatches) > 0 || len(stMatches) > 0 {
			out = append(out, domain.FilterResult{
				Filter:         f,
				KeywordMatches: kwMatches,
				StatusMatches:  stMatches,
			})
		}
	}
	return out
}

func matchesKeyword(text, keyword string, wholeWord bool) bool {
	if wholeWord {
		re, err := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(keyword) + `\b`)
		if err != nil {
			return false
		}
		return re.MatchString(text)
	}
	return strings.Contains(strings.ToLower(text), strings.ToLower(keyword))
}
