package service

import (
	"context"
	"testing"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testNonexistentID = "nonexistent"

func TestFeaturedTagService_CreateFeaturedTag(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewFeaturedTagService(fake)
	fake.SeedAccount(&domain.Account{ID: "acct1", Username: "alice"})

	tests := []struct {
		name    string
		tagName string
		wantErr error
	}{
		{
			name:    "success",
			tagName: "golang",
		},
		{
			name:    "empty name",
			tagName: "",
			wantErr: domain.ErrValidation,
		},
		{
			name:    "whitespace only",
			tagName: "   ",
			wantErr: domain.ErrValidation,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ft, err := svc.CreateFeaturedTag(ctx, "acct1", tc.tagName)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, ft)
			assert.NotEmpty(t, ft.ID)
			assert.Equal(t, "acct1", ft.AccountID)
			assert.NotEmpty(t, ft.TagID)
		})
	}
}

func TestFeaturedTagService_ListFeaturedTags(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewFeaturedTagService(fake)
	fake.SeedAccount(&domain.Account{ID: "acct1", Username: "alice"})

	t.Run("returns tags", func(t *testing.T) {
		_, err := svc.CreateFeaturedTag(ctx, "acct1", "rust")
		require.NoError(t, err)
		_, err = svc.CreateFeaturedTag(ctx, "acct1", "go")
		require.NoError(t, err)

		tags, err := svc.ListFeaturedTags(ctx, "acct1")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(tags), 2)
	})
}

func TestFeaturedTagService_DeleteFeaturedTag(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewFeaturedTagService(fake)
	fake.SeedAccount(&domain.Account{ID: "acct1", Username: "alice"})

	tests := []struct {
		name    string
		setup   func() string
		wantErr error
	}{
		{
			name: "success",
			setup: func() string {
				ft, err := svc.CreateFeaturedTag(ctx, "acct1", "deleteme")
				require.NoError(t, err)
				return ft.ID
			},
		},
		{
			name: "not found",
			setup: func() string {
				return testNonexistentID
			},
			wantErr: domain.ErrNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id := tc.setup()
			err := svc.DeleteFeaturedTag(ctx, "acct1", id)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)

			tags, err := svc.ListFeaturedTags(ctx, "acct1")
			require.NoError(t, err)
			for _, tag := range tags {
				assert.NotEqual(t, id, tag.ID)
			}
		})
	}
}

func TestFeaturedTagService_GetSuggestions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewFeaturedTagService(fake)
	fake.SeedAccount(&domain.Account{ID: "acct1", Username: "alice"})

	t.Run("returns suggestions", func(t *testing.T) {
		t.Parallel()
		tags, counts, err := svc.GetSuggestions(ctx, "acct1", 10)
		require.NoError(t, err)
		assert.Empty(t, tags)
		assert.Empty(t, counts)
	})

	t.Run("limit clamped", func(t *testing.T) {
		t.Parallel()

		_, _, err := svc.GetSuggestions(ctx, "acct1", 0)
		require.NoError(t, err)

		_, _, err = svc.GetSuggestions(ctx, "acct1", 100)
		require.NoError(t, err)

		clamped := ClampLimit(0, 10, 40)
		assert.Equal(t, 10, clamped)

		clamped = ClampLimit(100, 10, 40)
		assert.Equal(t, 40, clamped)

		clamped = ClampLimit(20, 10, 40)
		assert.Equal(t, 20, clamped)
	})
}
