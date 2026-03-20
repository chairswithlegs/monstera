package service

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
)

// SearchType restricts which dimensions are searched.
type SearchType string

const (
	SearchTypeAccounts SearchType = "accounts"
	SearchTypeStatuses SearchType = "statuses"
	SearchTypeHashtags SearchType = "hashtags"
	SearchTypeAll      SearchType = "all"
)

// SearchResult holds the result of a search (accounts, statuses, hashtags).
// Phase 1: Statuses is always empty.
type SearchResult struct {
	Accounts []*domain.Account
	Statuses []*domain.Status
	Hashtags []domain.Hashtag
}

// WebFingerResolver resolves a remote account by acct (user@domain).
// Implemented by activitypub.RemoteAccountResolver.
type WebFingerResolver interface {
	ResolveRemoteAccount(ctx context.Context, acct string) (*domain.Account, error)
}

// SearchService orchestrates account search, hashtag search, and optional remote account resolution.
type SearchService interface {
	Search(ctx context.Context, viewer *domain.Account, q string, filter SearchType, resolve bool, limit int) (*SearchResult, error)
}

type searchService struct {
	store    store.Store
	resolver WebFingerResolver
	backfill BackfillService
}

func NewSearchService(s store.Store, resolver WebFingerResolver, backfill BackfillService) SearchService {
	return &searchService{store: s, resolver: resolver, backfill: backfill}
}

// acctPattern matches user@domain (username and domain non-empty).
// used to determine if the account is remote.
var acctPattern = regexp.MustCompile(`^[a-zA-Z0-9_]+@[a-zA-Z0-9][a-zA-Z0-9.-]*[a-zA-Z0-9]$`)

// Search runs account and/or hashtag search and optionally resolves a remote account by acct.
// viewer may be nil (unauthenticated). limit is clamped by the handler; Phase 1 statuses are always empty.
func (svc *searchService) Search(ctx context.Context, viewer *domain.Account, q string, filter SearchType, resolve bool, limit int) (*SearchResult, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return &SearchResult{
			Accounts: []*domain.Account{},
			Statuses: []*domain.Status{},
			Hashtags: []domain.Hashtag{},
		}, nil
	}

	limit = ClampLimit(limit, 5, 40)
	out := &SearchResult{
		Accounts: []*domain.Account{},
		Statuses: []*domain.Status{}, // Phase 1: always empty
		Hashtags: []domain.Hashtag{},
	}

	wantAccounts := filter == SearchTypeAccounts || filter == SearchTypeAll
	wantHashtags := filter == SearchTypeHashtags || filter == SearchTypeAll

	if wantAccounts {
		accounts, err := svc.searchAccounts(ctx, q, limit)
		if err != nil {
			return nil, err
		}
		out.Accounts = accounts

		if resolve && acctPattern.MatchString(q) {
			out.Accounts = svc.resolveAndMergeAccount(ctx, q, out.Accounts, limit)
		}
	}

	if wantHashtags {
		hashtags, err := svc.searchHashtags(ctx, q, limit)
		if err != nil {
			return nil, err
		}
		out.Hashtags = hashtags
	}

	return out, nil
}

// searchAccounts queries the store for accounts matching q.
func (svc *searchService) searchAccounts(ctx context.Context, q string, limit int) ([]*domain.Account, error) {
	accounts, err := svc.store.SearchAccounts(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("SearchAccounts: %w", err)
	}
	return accounts, nil
}

// resolveAndMergeAccount resolves a remote account by acct URI, triggers a backfill,
// deduplicates against existing accounts, and trims to limit.
func (svc *searchService) resolveAndMergeAccount(ctx context.Context, acct string, accounts []*domain.Account, limit int) []*domain.Account {
	if svc.resolver == nil {
		return accounts
	}

	remote, err := svc.resolver.ResolveRemoteAccount(ctx, acct)
	if err != nil {
		slog.DebugContext(ctx, "search resolve failed", slog.String("acct", acct), slog.Any("error", err))
		return accounts
	}

	// Always request a backfill when we resolve a remote account, even if it's
	// already in the result set. The account may not have been backfilled yet, and
	// the worker's cooldown check prevents redundant execution.
	if svc.backfill != nil {
		if bfErr := svc.backfill.RequestBackfill(ctx, remote.ID); bfErr != nil {
			slog.WarnContext(ctx, "backfill request failed", slog.String("account_id", remote.ID), slog.Any("error", bfErr))
		}
	}

	// Already in the result set — no need to append.
	for _, a := range accounts {
		if a.ID == remote.ID {
			return accounts
		}
	}

	accounts = append(accounts, remote)
	if len(accounts) > limit {
		accounts = accounts[:limit]
	}
	return accounts
}

// searchHashtags queries the store for hashtags matching q (with optional # prefix stripped).
func (svc *searchService) searchHashtags(ctx context.Context, q string, limit int) ([]domain.Hashtag, error) {
	prefix := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(q), "#"))
	if prefix == "" {
		return nil, nil
	}

	hashtags, err := svc.store.SearchHashtagsByPrefix(ctx, prefix, limit)
	if err != nil {
		return nil, fmt.Errorf("SearchHashtagsByPrefix: %w", err)
	}
	return hashtags, nil
}
