package service

import (
	"context"
	"fmt"
	"slices"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
)

// Allowed marker timelines (Mastodon spec).
var allowedTimelines = []string{"home", "notifications"}

// MarkerService provides read-position markers for timelines.
type MarkerService interface {
	GetMarkers(ctx context.Context, accountID string, timelines []string) (map[string]domain.Marker, error)
	SetMarker(ctx context.Context, accountID, timeline, lastReadID string) error
}

type markerService struct {
	store store.Store
}

// NewMarkerService returns a MarkerService using the given store.
func NewMarkerService(s store.Store) MarkerService {
	return &markerService{store: s}
}

// GetMarkers returns markers for the given account and timelines (only home/notifications).
func (svc *markerService) GetMarkers(ctx context.Context, accountID string, timelines []string) (map[string]domain.Marker, error) {
	allowed := filterAllowedTimelines(timelines)
	if len(allowed) == 0 {
		return map[string]domain.Marker{}, nil
	}
	m, err := svc.store.GetMarkers(ctx, accountID, allowed)
	if err != nil {
		return nil, fmt.Errorf("GetMarkers: %w", err)
	}
	return m, nil
}

// SetMarker sets the read position for a timeline (only home/notifications).
func (svc *markerService) SetMarker(ctx context.Context, accountID, timeline, lastReadID string) error {
	if !slices.Contains(allowedTimelines, timeline) {
		return fmt.Errorf("SetMarker timeline %q: %w", timeline, domain.ErrValidation)
	}
	if err := svc.store.SetMarker(ctx, accountID, timeline, lastReadID); err != nil {
		return fmt.Errorf("SetMarker: %w", err)
	}
	return nil
}

func filterAllowedTimelines(timelines []string) []string {
	out := make([]string, 0, len(timelines))
	seen := make(map[string]struct{})
	for _, t := range timelines {
		if slices.Contains(allowedTimelines, t) {
			if _, ok := seen[t]; !ok {
				seen[t] = struct{}{}
				out = append(out, t)
			}
		}
	}
	return out
}
