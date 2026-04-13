package service

import (
	"context"
	"testing"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createV2FilterWithKeyword is a test helper that creates a v2 filter with a single keyword.
func createV2FilterWithKeyword(t *testing.T, ctx context.Context, st *testutil.FakeStore, accountID, keyword string, filterAction domain.FilterAction, contexts []domain.FilterContext) {
	t.Helper()
	filterID := uid.New()
	_, err := st.CreateFilter(ctx, store.CreateFilterInput{
		ID:           filterID,
		AccountID:    accountID,
		Title:        keyword,
		Context:      contexts,
		FilterAction: filterAction,
	})
	require.NoError(t, err)
	_, err = st.AddFilterKeyword(ctx, filterID, uid.New(), keyword, false)
	require.NoError(t, err)
}

func TestContentFilterMatcher(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	recipientID := uid.New()
	content := "this post contains a spoiler about the movie"

	tests := []struct {
		name         string
		keyword      string
		action       domain.FilterAction
		contexts     []domain.FilterContext
		createFilter bool
		wantMatch    bool
	}{
		{
			name:         "no filters returns false",
			createFilter: false,
			wantMatch:    false,
		},
		{
			name:         "matching keyword with hide action returns true",
			keyword:      "spoiler",
			action:       domain.FilterActionHide,
			contexts:     []domain.FilterContext{domain.FilterContextNotifications},
			createFilter: true,
			wantMatch:    true,
		},
		{
			name:         "matching keyword with warn action returns false",
			keyword:      "spoiler",
			action:       domain.FilterActionWarn,
			contexts:     []domain.FilterContext{domain.FilterContextNotifications},
			createFilter: true,
			wantMatch:    false,
		},
		{
			name:         "non-matching keyword returns false",
			keyword:      "unrelated",
			action:       domain.FilterActionHide,
			contexts:     []domain.FilterContext{domain.FilterContextNotifications},
			createFilter: true,
			wantMatch:    false,
		},
		{
			name:         "filter with different context returns false",
			keyword:      "spoiler",
			action:       domain.FilterActionHide,
			contexts:     []domain.FilterContext{domain.FilterContextHome},
			createFilter: true,
			wantMatch:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			st := testutil.NewFakeStore()
			m := &contentFilterMatcher{store: st}
			if tc.createFilter {
				createV2FilterWithKeyword(t, ctx, st, recipientID, tc.keyword, tc.action, tc.contexts)
			}
			status := &domain.Status{ID: uid.New(), Content: &content}
			matched, err := m.StatusMatchesNotificationFilters(ctx, recipientID, status)
			require.NoError(t, err)
			assert.Equal(t, tc.wantMatch, matched)
		})
	}
}
