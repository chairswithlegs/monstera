package service

import (
	"context"
	"fmt"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/events"
	"github.com/chairswithlegs/monstera/internal/store"
)

type contentFilterMatcher struct {
	store store.Store
}

// NewContentFilterMatcher returns a ContentFilterMatcher that loads v1 phrase
// filters from the store and matches them against status content.
func NewContentFilterMatcher(s store.Store) events.ContentFilterMatcher {
	return &contentFilterMatcher{store: s}
}

func (m *contentFilterMatcher) StatusMatchesNotificationFilters(ctx context.Context, recipientID string, status *domain.Status) (bool, error) {
	filters, err := m.store.GetActiveUserFiltersByContext(ctx, recipientID, domain.FilterContextNotifications)
	if err != nil {
		return false, fmt.Errorf("GetActiveUserFiltersByContext: %w", err)
	}
	if len(filters) == 0 {
		return false, nil
	}
	compiled := compilePhraseFilters(filters)
	if len(compiled) == 0 {
		return false, nil
	}
	return statusMatchesAnyFilter(status, compiled), nil
}
