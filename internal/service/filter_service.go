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

var validFilterActions = map[domain.FilterAction]bool{domain.FilterActionWarn: true, domain.FilterActionHide: true}

var validFilterContexts = map[domain.FilterContext]bool{
	domain.FilterContextHome:          true,
	domain.FilterContextNotifications: true,
	domain.FilterContextPublic:        true,
	domain.FilterContextThread:        true,
	domain.FilterContextAccount:       true,
}

// FilterService manages per-account content filters (multi-keyword, filter_action).
type FilterService interface {
	// Filter CRUD
	CreateFilter(ctx context.Context, accountID, title string, context []domain.FilterContext, expiresIn *int, filterAction domain.FilterAction) (*domain.UserFilter, error)
	GetFilter(ctx context.Context, accountID, filterID string) (*domain.UserFilter, error)
	ListFilters(ctx context.Context, accountID string) ([]domain.UserFilter, error)
	UpdateFilter(ctx context.Context, accountID, filterID, title string, context []domain.FilterContext, expiresIn *int, filterAction domain.FilterAction) (*domain.UserFilter, error)
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

type filterService struct {
	store store.Store
}

// NewFilterService returns a FilterService backed by the given store.
func NewFilterService(s store.Store) FilterService {
	return &filterService{store: s}
}

// expiresInToTime converts a seconds-from-now value to a *time.Time.
func expiresInToTime(expiresIn *int) *time.Time {
	if expiresIn == nil {
		return nil
	}
	t := time.Now().Add(time.Duration(*expiresIn) * time.Second)
	return &t
}

// validateFilterAction returns ErrUnprocessable if the value is not "warn" or "hide".
func validateFilterAction(filterAction domain.FilterAction) error {
	if !validFilterActions[filterAction] {
		return fmt.Errorf("filter_action %q: %w", filterAction, domain.ErrUnprocessable)
	}
	return nil
}

// validateFilterContext returns ErrUnprocessable for any unknown context value.
func validateFilterContext(ctx []domain.FilterContext) error {
	for _, c := range ctx {
		if !validFilterContexts[c] {
			return fmt.Errorf("context %q: %w", c, domain.ErrUnprocessable)
		}
	}
	return nil
}

func (svc *filterService) CreateFilter(ctx context.Context, accountID, title string, filterContext []domain.FilterContext, expiresIn *int, filterAction domain.FilterAction) (*domain.UserFilter, error) {
	if title == "" {
		return nil, fmt.Errorf("CreateFilter: %w", domain.ErrValidation)
	}
	if len(filterContext) == 0 {
		filterContext = []domain.FilterContext{domain.FilterContextHome}
	}
	if err := validateFilterContext(filterContext); err != nil {
		return nil, fmt.Errorf("CreateFilter: %w", err)
	}
	if filterAction == "" {
		filterAction = domain.FilterActionWarn
	}
	if err := validateFilterAction(filterAction); err != nil {
		return nil, fmt.Errorf("CreateFilter: %w", err)
	}
	f, err := svc.store.CreateFilter(ctx, store.CreateFilterInput{
		ID:           uid.New(),
		AccountID:    accountID,
		Title:        title,
		Context:      filterContext,
		ExpiresAt:    expiresInToTime(expiresIn),
		FilterAction: filterAction,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateFilter: %w", err)
	}
	return f, nil
}

func (svc *filterService) GetFilter(ctx context.Context, accountID, filterID string) (*domain.UserFilter, error) {
	f, err := svc.store.GetFilterByID(ctx, filterID)
	if err != nil {
		return nil, fmt.Errorf("GetFilter: %w", err)
	}
	if f.AccountID != accountID {
		return nil, fmt.Errorf("GetFilter: %w", domain.ErrForbidden)
	}
	return f, nil
}

func (svc *filterService) ListFilters(ctx context.Context, accountID string) ([]domain.UserFilter, error) {
	list, err := svc.store.ListFilters(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("ListFilters: %w", err)
	}
	return list, nil
}

func (svc *filterService) UpdateFilter(ctx context.Context, accountID, filterID, title string, filterContext []domain.FilterContext, expiresIn *int, filterAction domain.FilterAction) (*domain.UserFilter, error) {
	existing, err := svc.store.GetFilterByID(ctx, filterID)
	if err != nil {
		return nil, fmt.Errorf("UpdateFilter: %w", err)
	}
	if existing.AccountID != accountID {
		return nil, fmt.Errorf("UpdateFilter: %w", domain.ErrForbidden)
	}
	if title == "" {
		title = existing.Title
	}
	if len(filterContext) == 0 {
		filterContext = existing.Context
	} else {
		if err := validateFilterContext(filterContext); err != nil {
			return nil, fmt.Errorf("UpdateFilter: %w", err)
		}
	}
	if filterAction == "" {
		filterAction = existing.FilterAction
	} else {
		if err := validateFilterAction(filterAction); err != nil {
			return nil, fmt.Errorf("UpdateFilter: %w", err)
		}
	}
	exp := existing.ExpiresAt
	if expiresIn != nil {
		exp = expiresInToTime(expiresIn)
	}
	f, err := svc.store.UpdateFilter(ctx, store.UpdateFilterInput{
		ID:           filterID,
		Title:        title,
		Context:      filterContext,
		ExpiresAt:    exp,
		FilterAction: filterAction,
	})
	if err != nil {
		return nil, fmt.Errorf("UpdateFilter: %w", err)
	}
	return f, nil
}

func (svc *filterService) DeleteFilter(ctx context.Context, accountID, filterID string) error {
	existing, err := svc.store.GetFilterByID(ctx, filterID)
	if err != nil {
		return fmt.Errorf("DeleteFilter: %w", err)
	}
	if existing.AccountID != accountID {
		return fmt.Errorf("DeleteFilter: %w", domain.ErrForbidden)
	}
	if err := svc.store.DeleteFilter(ctx, filterID); err != nil {
		return fmt.Errorf("DeleteFilter: %w", err)
	}
	return nil
}

func (svc *filterService) ListKeywords(ctx context.Context, accountID, filterID string) ([]domain.FilterKeyword, error) {
	if _, err := svc.GetFilter(ctx, accountID, filterID); err != nil {
		return nil, err
	}
	kws, err := svc.store.ListFilterKeywords(ctx, filterID)
	if err != nil {
		return nil, fmt.Errorf("ListKeywords: %w", err)
	}
	return kws, nil
}

func (svc *filterService) AddKeyword(ctx context.Context, accountID, filterID, keyword string, wholeWord bool) (*domain.FilterKeyword, error) {
	if _, err := svc.GetFilter(ctx, accountID, filterID); err != nil {
		return nil, err
	}
	kw, err := svc.store.AddFilterKeyword(ctx, filterID, uid.New(), keyword, wholeWord)
	if err != nil {
		return nil, fmt.Errorf("AddKeyword: %w", err)
	}
	return kw, nil
}

func (svc *filterService) GetKeyword(ctx context.Context, accountID, keywordID string) (*domain.FilterKeyword, error) {
	kw, err := svc.store.GetFilterKeywordByID(ctx, keywordID)
	if err != nil {
		return nil, fmt.Errorf("GetKeyword: %w", err)
	}
	if _, err := svc.GetFilter(ctx, accountID, kw.FilterID); err != nil {
		return nil, err
	}
	return kw, nil
}

func (svc *filterService) UpdateKeyword(ctx context.Context, accountID, keywordID, keyword string, wholeWord bool) (*domain.FilterKeyword, error) {
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

func (svc *filterService) DeleteKeyword(ctx context.Context, accountID, keywordID string) error {
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

func (svc *filterService) ListFilterStatuses(ctx context.Context, accountID, filterID string) ([]domain.FilterStatus, error) {
	if _, err := svc.GetFilter(ctx, accountID, filterID); err != nil {
		return nil, err
	}
	fsts, err := svc.store.ListFilterStatuses(ctx, filterID)
	if err != nil {
		return nil, fmt.Errorf("ListFilterStatuses: %w", err)
	}
	return fsts, nil
}

func (svc *filterService) AddFilterStatus(ctx context.Context, accountID, filterID, statusID string) (*domain.FilterStatus, error) {
	if _, err := svc.GetFilter(ctx, accountID, filterID); err != nil {
		return nil, err
	}
	fs, err := svc.store.AddFilterStatus(ctx, uid.New(), filterID, statusID)
	if err != nil {
		return nil, fmt.Errorf("AddFilterStatus: %w", err)
	}
	return fs, nil
}

func (svc *filterService) GetFilterStatus(ctx context.Context, accountID, filterStatusID string) (*domain.FilterStatus, error) {
	fs, err := svc.store.GetFilterStatusByID(ctx, filterStatusID)
	if err != nil {
		return nil, fmt.Errorf("GetFilterStatus: %w", err)
	}
	if _, err := svc.GetFilter(ctx, accountID, fs.FilterID); err != nil {
		return nil, err
	}
	return fs, nil
}

func (svc *filterService) DeleteFilterStatus(ctx context.Context, accountID, filterStatusID string) error {
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
func (svc *filterService) ComputeFilterResults(ctx context.Context, accountID, statusID string, content, cw string) ([]domain.FilterResult, error) {
	filters, err := svc.store.GetActiveFilters(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("ComputeFilterResults: %w", err)
	}
	return computeFilterResults(filters, statusID, content, cw), nil
}

// computeFilterResults is a convenience wrapper for single-status use.
// For batch enrichment, use compileFilters + matchCompiledFilters to avoid re-compiling per status.
func computeFilterResults(filters []domain.UserFilter, statusID, content, cw string) []domain.FilterResult {
	return matchCompiledFilters(compileFilters(filters), statusID, content, cw)
}

// compiledFilter holds a filter with its whole-word keywords pre-compiled into regexes.
type compiledFilter struct {
	filter   domain.UserFilter
	keywords []compiledKeyword
}

type compiledKeyword struct {
	keyword string
	re      *regexp.Regexp // non-nil for whole_word keywords; nil uses case-insensitive substring
}

// compileFilters pre-compiles whole-word keyword regexes for a set of filters.
// Call this once per request batch and reuse across multiple statuses via matchCompiledFilters.
func compileFilters(filters []domain.UserFilter) []compiledFilter {
	out := make([]compiledFilter, 0, len(filters))
	for _, f := range filters {
		cf := compiledFilter{
			filter:   f,
			keywords: make([]compiledKeyword, 0, len(f.Keywords)),
		}
		for _, kw := range f.Keywords {
			ck := compiledKeyword{keyword: kw.Keyword}
			if kw.WholeWord {
				re, err := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(kw.Keyword) + `\b`)
				if err == nil {
					ck.re = re
				}
			}
			cf.keywords = append(cf.keywords, ck)
		}
		out = append(out, cf)
	}
	return out
}

// matchCompiledFilters checks a status against a pre-compiled filter set and returns
// the list of FilterResult entries for any matching filters.
func matchCompiledFilters(compiled []compiledFilter, statusID, content, cw string) []domain.FilterResult {
	text := cw + " " + content
	lowerText := strings.ToLower(text)
	var out []domain.FilterResult
	for _, cf := range compiled {
		var kwMatches []string
		for _, ck := range cf.keywords {
			var matched bool
			if ck.re != nil {
				matched = ck.re.MatchString(text)
			} else {
				matched = strings.Contains(lowerText, strings.ToLower(ck.keyword))
			}
			if matched {
				kwMatches = append(kwMatches, ck.keyword)
			}
		}
		var stMatches []string
		for _, fs := range cf.filter.Statuses {
			if fs.StatusID == statusID {
				stMatches = append(stMatches, statusID)
			}
		}
		if len(kwMatches) > 0 || len(stMatches) > 0 {
			out = append(out, domain.FilterResult{
				Filter:         cf.filter,
				KeywordMatches: kwMatches,
				StatusMatches:  stMatches,
			})
		}
	}
	return out
}
