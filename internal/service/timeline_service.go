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
// Used by timeline endpoints to return Mastodon API response shape without handler store access.
type EnrichedStatus struct {
	Status   *domain.Status
	Author   *domain.Account
	Mentions []*domain.Account
	Tags     []domain.Hashtag
	Media    []domain.MediaAttachment
}

// TimelineService handles timeline queries (home, public).
type TimelineService interface {
	Home(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, error)
	HomeEnriched(ctx context.Context, accountID string, maxID *string, limit int) ([]EnrichedStatus, error)
	PublicLocal(ctx context.Context, localOnly bool, maxID *string, limit int) ([]domain.Status, error)
	PublicLocalEnriched(ctx context.Context, localOnly bool, maxID *string, limit int) ([]EnrichedStatus, error)
	AccountStatusesEnriched(ctx context.Context, accountID string, viewerAccountID *string, maxID *string, limit int) ([]EnrichedStatus, error)
	GetAccountPublicStatuses(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, error)
	CountAccountPublicStatuses(ctx context.Context, accountID string) (int64, error)
	HashtagTimeline(ctx context.Context, tagName string, maxID *string, limit int) ([]domain.Status, error)
	HashtagTimelineEnriched(ctx context.Context, tagName string, maxID *string, limit int) ([]EnrichedStatus, error)
	FavouritesEnriched(ctx context.Context, accountID string, maxID *string, limit int) ([]EnrichedStatus, *string, error)
	BookmarksEnriched(ctx context.Context, accountID string, maxID *string, limit int) ([]EnrichedStatus, *string, error)
	ListTimelineEnriched(ctx context.Context, accountID, listID string, maxID *string, limit int) ([]EnrichedStatus, error)
}

type timelineService struct {
	store store.Store
}

// NewTimelineService returns a TimelineService that uses the given store.
func NewTimelineService(s store.Store) TimelineService {
	return &timelineService{store: s}
}

// enrichStatuses loads author, mentions, tags, and media for each status.
func (svc *timelineService) enrichStatuses(ctx context.Context, statuses []domain.Status) ([]EnrichedStatus, error) {
	out := make([]EnrichedStatus, 0, len(statuses))
	for i := range statuses {
		s := &statuses[i]
		author, err := svc.store.GetAccountByID(ctx, s.AccountID)
		if err != nil {
			return nil, fmt.Errorf("GetAccountByID(%s): %w", s.AccountID, err)
		}
		mentions, err := svc.store.GetStatusMentions(ctx, s.ID)
		if err != nil {
			return nil, fmt.Errorf("GetStatusMentions(%s): %w", s.ID, err)
		}
		tags, err := svc.store.GetStatusHashtags(ctx, s.ID)
		if err != nil {
			return nil, fmt.Errorf("GetStatusHashtags(%s): %w", s.ID, err)
		}
		media, err := svc.store.GetStatusAttachments(ctx, s.ID)
		if err != nil {
			return nil, fmt.Errorf("GetStatusAttachments(%s): %w", s.ID, err)
		}
		out = append(out, EnrichedStatus{
			Status:   s,
			Author:   author,
			Mentions: mentions,
			Tags:     tags,
			Media:    media,
		})
	}
	return out, nil
}

// Home returns the home timeline for the given account (self + accepted follows), ordered by id desc.
// maxID is optional (cursor); limit defaults to defaultTimelineLimit if <= 0, capped at maxTimelineLimit.
// TODO: confirm that this function is needed
func (svc *timelineService) Home(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, error) {
	l := limit
	if l <= 0 {
		l = defaultTimelineLimit
	}
	if l > maxTimelineLimit {
		l = maxTimelineLimit
	}
	rows, err := svc.store.GetHomeTimeline(ctx, accountID, maxID, l)
	if err != nil {
		return nil, fmt.Errorf("GetHomeTimeline: %w", err)
	}
	return rows, nil
}

// HomeEnriched returns the home timeline with author, mentions, tags, and media loaded for each status.
// maxID is optional (cursor); limit is clamped to [1, maxTimelineLimit], default defaultTimelineLimit.
func (svc *timelineService) HomeEnriched(ctx context.Context, accountID string, maxID *string, limit int) ([]EnrichedStatus, error) {
	l := limit
	if l <= 0 {
		l = defaultTimelineLimit
	}
	if l > maxTimelineLimit {
		l = maxTimelineLimit
	}
	statuses, err := svc.store.GetHomeTimeline(ctx, accountID, maxID, l)
	if err != nil {
		return nil, fmt.Errorf("GetHomeTimeline: %w", err)
	}
	out, err := svc.enrichStatuses(ctx, statuses)
	if err != nil {
		return nil, err
	}
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
	l := limit
	if l <= 0 {
		l = defaultTimelineLimit
	}
	if l > maxTimelineLimit {
		l = maxTimelineLimit
	}
	statuses, nextCursor, err := svc.store.GetFavouritesTimeline(ctx, accountID, maxID, l)
	if err != nil {
		return nil, nil, fmt.Errorf("GetFavouritesTimeline: %w", err)
	}
	out, err := svc.enrichStatuses(ctx, statuses)
	if err != nil {
		return nil, nil, err
	}
	return out, nextCursor, nil
}

// BookmarksEnriched returns the bookmarks timeline with author, mentions, tags, and media.
func (svc *timelineService) BookmarksEnriched(ctx context.Context, accountID string, maxID *string, limit int) ([]EnrichedStatus, *string, error) {
	l := limit
	if l <= 0 {
		l = defaultTimelineLimit
	}
	if l > maxTimelineLimit {
		l = maxTimelineLimit
	}
	statuses, nextCursor, err := svc.store.GetBookmarks(ctx, accountID, maxID, l)
	if err != nil {
		return nil, nil, fmt.Errorf("GetBookmarks: %w", err)
	}
	out, err := svc.enrichStatuses(ctx, statuses)
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
	l := limit
	if l <= 0 {
		l = defaultTimelineLimit
	}
	if l > maxTimelineLimit {
		l = maxTimelineLimit
	}
	statuses, err := svc.store.GetListTimeline(ctx, listID, maxID, l)
	if err != nil {
		return nil, fmt.Errorf("GetListTimeline: %w", err)
	}
	out, err := svc.enrichStatuses(ctx, statuses)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// PublicLocal returns the public timeline. localOnly true restricts to local statuses.
// maxID is optional; limit defaults to defaultTimelineLimit if <= 0, capped at maxTimelineLimit.
func (svc *timelineService) PublicLocal(ctx context.Context, localOnly bool, maxID *string, limit int) ([]domain.Status, error) {
	l := limit
	if l <= 0 {
		l = defaultTimelineLimit
	}
	if l > maxTimelineLimit {
		l = maxTimelineLimit
	}
	rows, err := svc.store.GetPublicTimeline(ctx, localOnly, maxID, l)
	if err != nil {
		return nil, fmt.Errorf("GetPublicTimeline: %w", err)
	}
	return rows, nil
}

// PublicLocalEnriched returns the public timeline with author, mentions, tags, and media loaded for each status.
func (svc *timelineService) PublicLocalEnriched(ctx context.Context, localOnly bool, maxID *string, limit int) ([]EnrichedStatus, error) {
	l := limit
	if l <= 0 {
		l = defaultTimelineLimit
	}
	if l > maxTimelineLimit {
		l = maxTimelineLimit
	}
	statuses, err := svc.store.GetPublicTimeline(ctx, localOnly, maxID, l)
	if err != nil {
		return nil, fmt.Errorf("GetPublicTimeline: %w", err)
	}
	out, err := svc.enrichStatuses(ctx, statuses)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// AccountStatusesEnriched returns statuses for an account (for GET /accounts/:id/statuses). When viewerAccountID is nil or != accountID, only public statuses are returned.
func (svc *timelineService) AccountStatusesEnriched(ctx context.Context, accountID string, viewerAccountID *string, maxID *string, limit int) ([]EnrichedStatus, error) {
	l := limit
	if l <= 0 {
		l = defaultTimelineLimit
	}
	if l > maxTimelineLimit {
		l = maxTimelineLimit
	}
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
	out, err := svc.enrichStatuses(ctx, statuses)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GetAccountPublicStatuses returns public statuses for an account (for outbox). maxID is optional cursor; limit is clamped.
func (svc *timelineService) GetAccountPublicStatuses(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, error) {
	l := limit
	if l <= 0 {
		l = defaultTimelineLimit
	}
	if l > maxTimelineLimit {
		l = maxTimelineLimit
	}
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
	l := limit
	if l <= 0 {
		l = defaultTimelineLimit
	}
	if l > maxTimelineLimit {
		l = maxTimelineLimit
	}
	statuses, err := svc.store.GetHashtagTimeline(ctx, strings.ToLower(tagName), maxID, l)
	if err != nil {
		return nil, fmt.Errorf("GetHashtagTimeline: %w", err)
	}
	return statuses, nil
}

// HashtagTimelineEnriched returns the hashtag timeline with author, mentions, tags, and media for each status.
func (svc *timelineService) HashtagTimelineEnriched(ctx context.Context, tagName string, maxID *string, limit int) ([]EnrichedStatus, error) {
	statuses, err := svc.HashtagTimeline(ctx, tagName, maxID, limit)
	if err != nil {
		return nil, err
	}
	return svc.enrichStatuses(ctx, statuses)
}
