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
	searchAccounts := filter == SearchTypeAccounts || filter == SearchTypeAll
	searchHashtags := filter == SearchTypeHashtags || filter == SearchTypeAll
	if searchAccounts {
		accounts, err := svc.store.SearchAccounts(ctx, q, limit)
		if err != nil {
			return nil, fmt.Errorf("SearchAccounts: %w", err)
		}
		out.Accounts = accounts
		// If the account is remote, resolve it (only when resolver is configured).
		if resolve && svc.resolver != nil && acctPattern.MatchString(q) {
			remote, err := svc.resolver.ResolveRemoteAccount(ctx, q)
			if err != nil {
				slog.DebugContext(ctx, "search resolve failed", slog.String("acct", q), slog.Any("error", err))
			} else {
				if svc.backfill != nil {
					if bfErr := svc.backfill.RequestBackfill(ctx, remote.ID); bfErr != nil {
						slog.WarnContext(ctx, "backfill request failed", slog.String("account_id", remote.ID), slog.Any("error", bfErr))
					}
				}
				seen := make(map[string]bool)
				for _, a := range out.Accounts {
					seen[a.ID] = true
				}
				if !seen[remote.ID] {
					out.Accounts = append(out.Accounts, remote)
					if len(out.Accounts) > limit {
						out.Accounts = out.Accounts[:limit]
					}
				}
			}
		}
	}
	if searchHashtags {
		prefix := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(q), "#"))
		if prefix != "" {
			hashtags, err := svc.store.SearchHashtagsByPrefix(ctx, prefix, limit)
			if err != nil {
				return nil, fmt.Errorf("SearchHashtagsByPrefix: %w", err)
			}
			out.Hashtags = hashtags
		}
	}
	return out, nil
}
