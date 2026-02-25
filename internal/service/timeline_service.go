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
