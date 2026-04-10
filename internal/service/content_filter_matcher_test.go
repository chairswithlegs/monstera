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

func TestContentFilterMatcher(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	recipientID := uid.New()
	content := "this post contains a spoiler about the movie"

	tests := []struct {
		name      string
		filters   []store.CreateUserFilterInput
		status    *domain.Status
		wantMatch bool
	}{
		{
			name:      "no filters returns false",
			filters:   nil,
			status:    &domain.Status{ID: "s1", Content: &content},
			wantMatch: false,
		},
		{
			name: "matching phrase returns true",
			filters: []store.CreateUserFilterInput{
				{ID: uid.New(), AccountID: recipientID, Phrase: "spoiler", Context: []string{domain.FilterContextNotifications}},
			},
			status:    &domain.Status{ID: "s2", Content: &content},
			wantMatch: true,
		},
		{
			name: "non-matching phrase returns false",
			filters: []store.CreateUserFilterInput{
				{ID: uid.New(), AccountID: recipientID, Phrase: "unrelated", Context: []string{domain.FilterContextNotifications}},
			},
			status:    &domain.Status{ID: "s3", Content: &content},
			wantMatch: false,
		},
		{
			name: "filter with different context not loaded",
			filters: []store.CreateUserFilterInput{
				{ID: uid.New(), AccountID: recipientID, Phrase: "spoiler", Context: []string{domain.FilterContextHome}},
			},
			status:    &domain.Status{ID: "s4", Content: &content},
			wantMatch: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			st := testutil.NewFakeStore()
			m := &contentFilterMatcher{store: st}
			for _, f := range tc.filters {
				_, err := st.CreateUserFilter(ctx, f)
				require.NoError(t, err)
			}
			matched, err := m.StatusMatchesNotificationFilters(ctx, recipientID, tc.status)
			require.NoError(t, err)
			assert.Equal(t, tc.wantMatch, matched)
		})
	}
}
