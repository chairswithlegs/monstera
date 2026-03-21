package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
)

const (
	defaultTimelineLimit = 20
	maxTimelineLimit     = 40
)

// EnrichedStatus is a status with its author, mentions, tags, and media loaded.
// Used by timeline endpoints to return Mastodon API response shape.
// Favourited, Reblogged, Bookmarked, Pinned, and Muted are viewer-relative and set when viewerAccountID is provided.
type EnrichedStatus struct {
	Status     *domain.Status
	Author     *domain.Account
	Mentions   []*domain.Account
	Tags       []domain.Hashtag
	Media      []domain.MediaAttachment
	Poll       *EnrichedPoll   // optional; set when status has an attached poll
	Favourited bool            // viewer has favourited this status
	Reblogged  bool            // viewer has reblogged this status
	Bookmarked bool            // viewer has bookmarked this status
	Pinned     bool            // author has pinned this status (only meaningful when viewer is author)
	Muted      bool            // viewer has muted this status's conversation
	Card       *domain.Card    // nil if not yet fetched or no URL in status
	ReblogOf   *EnrichedStatus // populated when Status.ReblogOfID != nil
}

// EnrichedPoll is a poll with options (and vote counts), plus viewer-relative voted/own_votes.
type EnrichedPoll struct {
	Poll     domain.Poll
	Options  []PollOptionWithCount // ordered by position
	Voted    bool                  // has the viewer voted (only when viewerAccountID was provided)
	OwnVotes []int                 // 0-based option indices the viewer selected
}

// PollOptionWithCount is a poll option with its vote count for API response.
type PollOptionWithCount struct {
	Title      string
	VotesCount int
}

// TimelineService handles timeline queries (home, public).
type TimelineService interface {
	HomeEnriched(ctx context.Context, accountID string, maxID *string, limit int) ([]EnrichedStatus, error)
	PublicLocalEnriched(ctx context.Context, localOnly bool, viewerAccountID *string, maxID *string, limit int) ([]EnrichedStatus, error)
	AccountStatusesEnriched(ctx context.Context, accountID string, viewerAccountID *string, maxID *string, limit int) ([]EnrichedStatus, error)
	GetAccountPublicStatuses(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, error)
	CountAccountPublicStatuses(ctx context.Context, accountID string) (int64, error)
	HashtagTimelineEnriched(ctx context.Context, tagName string, viewerAccountID *string, maxID *string, limit int) ([]EnrichedStatus, error)
	FavouritesEnriched(ctx context.Context, accountID string, maxID *string, limit int) ([]EnrichedStatus, *string, error)
	BookmarksEnriched(ctx context.Context, accountID string, maxID *string, limit int) ([]EnrichedStatus, *string, error)
	ListTimelineEnriched(ctx context.Context, accountID, listID string, maxID *string, limit int) ([]EnrichedStatus, error)
}

type timelineService struct {
	store      store.Store
	accountSvc AccountService
	statusSvc  StatusService
}

// NewTimelineService returns a TimelineService that uses the given store and status service.
func NewTimelineService(s store.Store, accountSvc AccountService, statusSvc StatusService) TimelineService {
	return &timelineService{store: s, accountSvc: accountSvc, statusSvc: statusSvc}
}

func (svc *timelineService) enrichStatuses(ctx context.Context, statuses []domain.Status, viewerAccountID *string) ([]EnrichedStatus, error) {
	ptrs := make([]*domain.Status, len(statuses))
	for i := range statuses {
		ptrs[i] = &statuses[i]
	}
	enriched, err := svc.statusSvc.EnrichStatuses(ctx, ptrs, EnrichOpts{IncludeCard: true, ViewerID: viewerAccountID})
	if err != nil {
		return nil, fmt.Errorf("enrichStatuses: %w", err)
	}
	return enriched, nil
}

// filterBlockedStatuses removes statuses where the viewer and author have a block
// relationship in either direction, matching Mastodon's behavior on public timelines.
func (svc *timelineService) filterBlockedStatuses(ctx context.Context, statuses []EnrichedStatus, viewerAccountID string) ([]EnrichedStatus, error) {
	filtered := make([]EnrichedStatus, 0, len(statuses))
	for i := range statuses {
		if statuses[i].Status == nil {
			continue
		}
		blocked, err := svc.store.IsBlockedEitherDirection(ctx, viewerAccountID, statuses[i].Status.AccountID)
		if err != nil {
			return nil, fmt.Errorf("IsBlockedEitherDirection: %w", err)
		}
		if !blocked {
			filtered = append(filtered, statuses[i])
		}
	}
	return filtered, nil
}

// HomeEnriched returns the home timeline with author, mentions, tags, and media loaded for each status.
// maxID is optional (cursor); limit is clamped to [1, maxTimelineLimit], default defaultTimelineLimit.
func (svc *timelineService) HomeEnriched(ctx context.Context, accountID string, maxID *string, limit int) ([]EnrichedStatus, error) {
	l := ClampLimit(limit, defaultTimelineLimit, maxTimelineLimit)
	statuses, err := svc.store.GetHomeTimeline(ctx, accountID, maxID, l)
	if err != nil {
		return nil, fmt.Errorf("GetHomeTimeline: %w", err)
	}
	viewerID := &accountID
	out, err := svc.enrichStatuses(ctx, statuses, viewerID)
	if err != nil {
		return nil, err
	}
	filtered := make([]EnrichedStatus, 0, len(out))
	for i := range out {
		ok, err := svc.statusSvc.CanViewStatus(ctx, out[i].Status, viewerID)
		if err != nil {
			return nil, fmt.Errorf("CanViewStatus: %w", err)
		}
		if ok {
			filtered = append(filtered, out[i])
		}
	}
	out = filtered
	filters, err := svc.store.GetActiveUserFiltersByContext(ctx, accountID, domain.FilterContextHome)
	if err != nil {
		slog.WarnContext(ctx, "failed to load user filters, returning unfiltered timeline", slog.Any("error", err))
	} else {
		out = ApplyUserFiltersToEnriched(out, filters)
	}
	return out, nil
}

// FavouritesEnriched returns the favourites timeline with author, mentions, tags, and media.
// nextCursor is the favourite ID to use as max_id for the next page, or nil if there are no more.
func (svc *timelineService) FavouritesEnriched(ctx context.Context, accountID string, maxID *string, limit int) ([]EnrichedStatus, *string, error) {
	l := ClampLimit(limit, defaultTimelineLimit, maxTimelineLimit)
	statuses, nextCursor, err := svc.store.GetFavouritesTimeline(ctx, accountID, maxID, l)
	if err != nil {
		return nil, nil, fmt.Errorf("GetFavouritesTimeline: %w", err)
	}
	out, err := svc.enrichStatuses(ctx, statuses, &accountID)
	if err != nil {
		return nil, nil, err
	}
	return out, nextCursor, nil
}

// BookmarksEnriched returns the bookmarks timeline with author, mentions, tags, and media.
func (svc *timelineService) BookmarksEnriched(ctx context.Context, accountID string, maxID *string, limit int) ([]EnrichedStatus, *string, error) {
	l := ClampLimit(limit, defaultTimelineLimit, maxTimelineLimit)
	statuses, nextCursor, err := svc.store.GetBookmarks(ctx, accountID, maxID, l)
	if err != nil {
		return nil, nil, fmt.Errorf("GetBookmarks: %w", err)
	}
	out, err := svc.enrichStatuses(ctx, statuses, &accountID)
	if err != nil {
		return nil, nil, err
	}
	return out, nextCursor, nil
}

// ListTimelineEnriched returns the list timeline with author, mentions, tags, and media.
// The accountID must own the list.
func (svc *timelineService) ListTimelineEnriched(ctx context.Context, accountID, listID string, maxID *string, limit int) ([]EnrichedStatus, error) {
	list, err := svc.store.GetListByID(ctx, listID)
	if err != nil {
		return nil, fmt.Errorf("GetListByID: %w", err)
	}
	if list.AccountID != accountID {
		return nil, fmt.Errorf("ListTimelineEnriched: %w", domain.ErrForbidden)
	}
	l := ClampLimit(limit, defaultTimelineLimit, maxTimelineLimit)
	statuses, err := svc.store.GetListTimeline(ctx, listID, maxID, l)
	if err != nil {
		return nil, fmt.Errorf("GetListTimeline: %w", err)
	}
	out, err := svc.enrichStatuses(ctx, statuses, &accountID)
	if err != nil {
		return nil, err
	}
	viewerID := &accountID
	filtered := make([]EnrichedStatus, 0, len(out))
	for i := range out {
		ok, err := svc.statusSvc.CanViewStatus(ctx, out[i].Status, viewerID)
		if err != nil {
			return nil, fmt.Errorf("CanViewStatus: %w", err)
		}
		if ok {
			filtered = append(filtered, out[i])
		}
	}
	return filtered, nil
}

// PublicLocalEnriched returns the public timeline with author, mentions, tags, and media loaded for each status.
// When viewerAccountID is non-nil, statuses from blocked/blocking accounts are filtered out.
func (svc *timelineService) PublicLocalEnriched(ctx context.Context, localOnly bool, viewerAccountID *string, maxID *string, limit int) ([]EnrichedStatus, error) {
	l := ClampLimit(limit, defaultTimelineLimit, maxTimelineLimit)
	statuses, err := svc.store.GetPublicTimeline(ctx, localOnly, maxID, l)
	if err != nil {
		return nil, fmt.Errorf("GetPublicTimeline: %w", err)
	}
	out, err := svc.enrichStatuses(ctx, statuses, viewerAccountID)
	if err != nil {
		return nil, err
	}
	if viewerAccountID != nil {
		out, err = svc.filterBlockedStatuses(ctx, out, *viewerAccountID)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// AccountStatusesEnriched returns statuses for an account (for GET /accounts/:id/statuses). When viewerAccountID is nil or != accountID, only public statuses are returned.
func (svc *timelineService) AccountStatusesEnriched(ctx context.Context, accountID string, viewerAccountID *string, maxID *string, limit int) ([]EnrichedStatus, error) {
	l := ClampLimit(limit, defaultTimelineLimit, maxTimelineLimit)
	var statuses []domain.Status
	var err error
	if viewerAccountID != nil && *viewerAccountID == accountID {
		statuses, err = svc.store.GetAccountStatuses(ctx, accountID, maxID, l)
	} else {
		statuses, err = svc.store.GetAccountPublicStatuses(ctx, accountID, maxID, l)
	}
	if err != nil {
		return nil, fmt.Errorf("GetAccountStatuses/GetAccountPublicStatuses: %w", err)
	}
	out, err := svc.enrichStatuses(ctx, statuses, viewerAccountID)
	if err != nil {
		return nil, err
	}
	if viewerAccountID != nil && *viewerAccountID != accountID {
		out, err = svc.filterBlockedStatuses(ctx, out, *viewerAccountID)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// GetAccountPublicStatuses returns public statuses for an account (for outbox). maxID is optional cursor; limit is clamped.
func (svc *timelineService) GetAccountPublicStatuses(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, error) {
	l := ClampLimit(limit, defaultTimelineLimit, maxTimelineLimit)
	rows, err := svc.store.GetAccountPublicStatuses(ctx, accountID, maxID, l)
	if err != nil {
		return nil, fmt.Errorf("GetAccountPublicStatuses: %w", err)
	}
	return rows, nil
}

// CountAccountPublicStatuses returns the count of public statuses for an account (for outbox totalItems).
func (svc *timelineService) CountAccountPublicStatuses(ctx context.Context, accountID string) (int64, error) {
	n, err := svc.store.CountAccountPublicStatuses(ctx, accountID)
	if err != nil {
		return 0, fmt.Errorf("CountAccountPublicStatuses(%s): %w", accountID, err)
	}
	return n, nil
}

// HashtagTimeline returns statuses for the given hashtag (public/unlisted only). Tag name is normalized to lowercase.
func (svc *timelineService) HashtagTimeline(ctx context.Context, tagName string, maxID *string, limit int) ([]domain.Status, error) {
	l := ClampLimit(limit, defaultTimelineLimit, maxTimelineLimit)
	statuses, err := svc.store.GetHashtagTimeline(ctx, strings.ToLower(tagName), maxID, l)
	if err != nil {
		return nil, fmt.Errorf("GetHashtagTimeline: %w", err)
	}
	return statuses, nil
}

// HashtagTimelineEnriched returns the hashtag timeline with author, mentions, tags, and media for each status.
// When viewerAccountID is non-nil, statuses from blocked/blocking accounts are filtered out.
func (svc *timelineService) HashtagTimelineEnriched(ctx context.Context, tagName string, viewerAccountID *string, maxID *string, limit int) ([]EnrichedStatus, error) {
	statuses, err := svc.HashtagTimeline(ctx, tagName, maxID, limit)
	if err != nil {
		return nil, err
	}
	out, err := svc.enrichStatuses(ctx, statuses, viewerAccountID)
	if err != nil {
		return nil, err
	}
	if viewerAccountID != nil {
		out, err = svc.filterBlockedStatuses(ctx, out, *viewerAccountID)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
