package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/testutil"
)

// errSentinel is a placeholder used in table tests to indicate "any error".
var errSentinel = errors.New("any error expected")

func TestUserFilterService_CreateFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		phrase      string
		context     []string
		wholeWord   bool
		expiresAt   *string
		wantErr     error
		wantCtx     []string
		wantExpires bool
	}{
		{
			name:    "success",
			phrase:  "spoiler",
			context: []string{domain.FilterContextHome, domain.FilterContextPublic},
			wantCtx: []string{domain.FilterContextHome, domain.FilterContextPublic},
		},
		{
			name:    "empty phrase",
			phrase:  "",
			context: []string{domain.FilterContextHome},
			wantErr: domain.ErrValidation,
		},
		{
			name:    "default context",
			phrase:  "keyword",
			context: nil,
			wantCtx: []string{domain.FilterContextHome},
		},
		{
			name:        "with expires_at",
			phrase:      "temp",
			context:     []string{domain.FilterContextHome},
			expiresAt:   strPtr(time.Now().Add(24 * time.Hour).Format(time.RFC3339)),
			wantCtx:     []string{domain.FilterContextHome},
			wantExpires: true,
		},
		{
			name:      "invalid expires_at",
			phrase:    "temp",
			context:   []string{domain.FilterContextHome},
			expiresAt: strPtr("not-a-date"),
			wantErr:   errSentinel,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := testutil.NewFakeStore()
			svc := NewUserFilterService(s)
			ctx := context.Background()

			f, err := svc.CreateFilter(ctx, "acct-1", tc.phrase, tc.context, tc.wholeWord, tc.expiresAt, false)
			if tc.wantErr != nil {
				require.Error(t, err)
				if !errors.Is(tc.wantErr, errSentinel) {
					require.ErrorIs(t, err, tc.wantErr)
				}
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, f.ID)
			assert.Equal(t, tc.phrase, f.Phrase)
			assert.Equal(t, tc.wantCtx, f.Context)
			if tc.wantExpires {
				assert.NotNil(t, f.ExpiresAt)
			}
		})
	}
}

func TestUserFilterService_GetFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ownerID  string
		callerID string
		wantErr  error
	}{
		{
			name:     "own filter",
			ownerID:  "acct-1",
			callerID: "acct-1",
		},
		{
			name:     "other user's filter",
			ownerID:  "acct-1",
			callerID: "acct-2",
			wantErr:  domain.ErrForbidden,
		},
		{
			name:     "not found",
			ownerID:  "",
			callerID: "acct-1",
			wantErr:  domain.ErrNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := testutil.NewFakeStore()
			svc := NewUserFilterService(s)
			ctx := context.Background()

			filterID := "nonexistent"
			if tc.ownerID != "" {
				f, err := svc.CreateFilter(ctx, tc.ownerID, "phrase", []string{domain.FilterContextHome}, false, nil, false)
				require.NoError(t, err)
				filterID = f.ID
			}

			got, err := svc.GetFilter(ctx, tc.callerID, filterID)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, filterID, got.ID)
		})
	}
}

func TestUserFilterService_ListFilters(t *testing.T) {
	t.Parallel()

	t.Run("returns filters", func(t *testing.T) {
		t.Parallel()
		s := testutil.NewFakeStore()
		svc := NewUserFilterService(s)
		ctx := context.Background()

		_, err := svc.CreateFilter(ctx, "acct-1", "word1", []string{domain.FilterContextHome}, false, nil, false)
		require.NoError(t, err)
		_, err = svc.CreateFilter(ctx, "acct-1", "word2", []string{domain.FilterContextPublic}, false, nil, false)
		require.NoError(t, err)
		_, err = svc.CreateFilter(ctx, "acct-other", "other", []string{domain.FilterContextHome}, false, nil, false)
		require.NoError(t, err)

		list, err := svc.ListFilters(ctx, "acct-1")
		require.NoError(t, err)
		assert.Len(t, list, 2)
	})
}

func TestUserFilterService_UpdateFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		ownerID    string
		callerID   string
		newPhrase  string
		newContext []string
		wantErr    error
		wantPhrase string
		wantCtx    []string
	}{
		{
			name:       "update phrase",
			ownerID:    "acct-1",
			callerID:   "acct-1",
			newPhrase:  "updated",
			newContext: []string{domain.FilterContextPublic},
			wantPhrase: "updated",
			wantCtx:    []string{domain.FilterContextPublic},
		},
		{
			name:     "other user's filter",
			ownerID:  "acct-1",
			callerID: "acct-2",
			wantErr:  domain.ErrForbidden,
		},
		{
			name:       "empty phrase keeps existing",
			ownerID:    "acct-1",
			callerID:   "acct-1",
			newPhrase:  "",
			newContext: []string{domain.FilterContextPublic},
			wantPhrase: "original",
			wantCtx:    []string{domain.FilterContextPublic},
		},
		{
			name:       "empty context keeps existing",
			ownerID:    "acct-1",
			callerID:   "acct-1",
			newPhrase:  "changed",
			newContext: nil,
			wantPhrase: "changed",
			wantCtx:    []string{domain.FilterContextHome},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := testutil.NewFakeStore()
			svc := NewUserFilterService(s)
			ctx := context.Background()

			f, err := svc.CreateFilter(ctx, tc.ownerID, "original", []string{domain.FilterContextHome}, false, nil, false)
			require.NoError(t, err)

			updated, err := svc.UpdateFilter(ctx, tc.callerID, f.ID, tc.newPhrase, tc.newContext, false, nil, false)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantPhrase, updated.Phrase)
			assert.Equal(t, tc.wantCtx, updated.Context)
		})
	}
}

func TestUserFilterService_DeleteFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ownerID  string
		callerID string
		wantErr  error
	}{
		{
			name:     "own filter",
			ownerID:  "acct-1",
			callerID: "acct-1",
		},
		{
			name:     "other user's filter",
			ownerID:  "acct-1",
			callerID: "acct-2",
			wantErr:  domain.ErrForbidden,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := testutil.NewFakeStore()
			svc := NewUserFilterService(s)
			ctx := context.Background()

			f, err := svc.CreateFilter(ctx, tc.ownerID, "phrase", []string{domain.FilterContextHome}, false, nil, false)
			require.NoError(t, err)

			err = svc.DeleteFilter(ctx, tc.callerID, f.ID)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func strPtr(s string) *string { return &s }
