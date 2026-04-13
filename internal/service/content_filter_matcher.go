package service

import (
	"context"
	"fmt"
	"slices"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/events"
	"github.com/chairswithlegs/monstera/internal/store"
)

type contentFilterMatcher struct {
	store store.Store
}

// NewContentFilterMatcher returns a ContentFilterMatcher that loads v2 keyword
// filters from the store and matches them against status content.
func NewContentFilterMatcher(s store.Store) events.ContentFilterMatcher {
	return &contentFilterMatcher{store: s}
}

func (m *contentFilterMatcher) StatusMatchesNotificationFilters(ctx context.Context, recipientID string, status *domain.Status) (bool, error) {
	filters, err := m.store.GetActiveFilters(ctx, recipientID)
	if err != nil {
		return false, fmt.Errorf("GetActiveFilters: %w", err)
	}
	if len(filters) == 0 {
		return false, nil
	}
	// Only consider filters that include the notifications context.
	var notifFilters []domain.UserFilter
	for _, f := range filters {
		if f.FilterAction == domain.FilterActionHide && slices.Contains(f.Context, domain.FilterContextNotifications) {
			notifFilters = append(notifFilters, f)
		}
	}
	if len(notifFilters) == 0 {
		return false, nil
	}
	compiled := compileFilters(notifFilters)
	if len(compiled) == 0 {
		return false, nil
	}
	content := ""
	if status.Content != nil {
		content = *status.Content
	}
	cw := ""
	if status.ContentWarning != nil {
		cw = *status.ContentWarning
	}
	results := matchCompiledFilters(compiled, status.ID, content, cw)
	return len(results) > 0, nil
}
