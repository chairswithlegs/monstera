package service

import (
	"context"
	"fmt"

	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/store"
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
type TimelineService struct {
	store store.Store
}

// NewTimelineService returns a TimelineService that uses the given store.
func NewTimelineService(s store.Store) *TimelineService {
	return &TimelineService{store: s}
}

// Home returns the home timeline for the given account (self + accepted follows), ordered by id desc.
// maxID is optional (cursor); limit defaults to defaultTimelineLimit if <= 0, capped at maxTimelineLimit.
// TODO: confirm that this function is needed
func (svc *TimelineService) Home(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, error) {
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
func (svc *TimelineService) HomeEnriched(ctx context.Context, accountID string, maxID *string, limit int) ([]EnrichedStatus, error) {
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

// PublicLocal returns the public timeline. localOnly true restricts to local statuses.
// maxID is optional; limit defaults to defaultTimelineLimit if <= 0, capped at maxTimelineLimit.
func (svc *TimelineService) PublicLocal(ctx context.Context, localOnly bool, maxID *string, limit int) ([]domain.Status, error) {
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
func (svc *TimelineService) PublicLocalEnriched(ctx context.Context, localOnly bool, maxID *string, limit int) ([]EnrichedStatus, error) {
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

// GetAccountPublicStatuses returns public statuses for an account (for outbox). maxID is optional cursor; limit is clamped.
func (svc *TimelineService) GetAccountPublicStatuses(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, error) {
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
func (svc *TimelineService) CountAccountPublicStatuses(ctx context.Context, accountID string) (int64, error) {
	n, err := svc.store.CountAccountPublicStatuses(ctx, accountID)
	if err != nil {
		return 0, fmt.Errorf("CountAccountPublicStatuses(%s): %w", accountID, err)
	}
	return n, nil
}
