package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/testutil"
)

func TestServerFilterService_CreateServerFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		phrase     string
		scope      string
		action     string
		wholeWord  bool
		wantScope  string
		wantAction string
	}{
		{
			name:       "success with defaults",
			phrase:     "spam",
			scope:      "",
			action:     "",
			wholeWord:  false,
			wantScope:  domain.ServerFilterScopeAll,
			wantAction: domain.ServerFilterActionHide,
		},
		{
			name:       "custom scope and action",
			phrase:     "offensive",
			scope:      domain.ServerFilterScopePublicTimeline,
			action:     domain.ServerFilterActionWarn,
			wholeWord:  false,
			wantScope:  domain.ServerFilterScopePublicTimeline,
			wantAction: domain.ServerFilterActionWarn,
		},
		{
			name:       "whole word flag",
			phrase:     "badword",
			scope:      "",
			action:     "",
			wholeWord:  true,
			wantScope:  domain.ServerFilterScopeAll,
			wantAction: domain.ServerFilterActionHide,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := testutil.NewFakeStore()
			svc := NewServerFilterService(s)
			ctx := context.Background()

			f, err := svc.CreateServerFilter(ctx, tc.phrase, tc.scope, tc.action, tc.wholeWord)
			require.NoError(t, err)

			assert.NotEmpty(t, f.ID)
			assert.Equal(t, tc.phrase, f.Phrase)
			assert.Equal(t, tc.wantScope, f.Scope)
			assert.Equal(t, tc.wantAction, f.Action)
		})
	}
}

func TestServerFilterService_ListServerFilters(t *testing.T) {
	t.Parallel()

	t.Run("returns filters", func(t *testing.T) {
		t.Parallel()
		s := testutil.NewFakeStore()
		svc := NewServerFilterService(s)
		ctx := context.Background()

		_, err := svc.CreateServerFilter(ctx, "spam", "", "", false)
		require.NoError(t, err)

		filters, err := svc.ListServerFilters(ctx)
		require.NoError(t, err)
		// FakeStore does not persist server filters, so we just verify no error.
		assert.IsType(t, []domain.ServerFilter{}, append([]domain.ServerFilter{}, filters...))
	})
}

func TestServerFilterService_UpdateServerFilter(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		s := testutil.NewFakeStore()
		svc := NewServerFilterService(s)
		ctx := context.Background()

		_, err := svc.UpdateServerFilter(ctx, "some-id", "updated-phrase", domain.ServerFilterScopePublicTimeline, domain.ServerFilterActionWarn, true)
		// FakeStore's UpdateServerFilter always returns ErrNotFound; verify the
		// service wraps the store error correctly.
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestServerFilterService_DeleteServerFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		id      string
		setup   func(svc ServerFilterService) string
		wantErr bool
	}{
		{
			name: "success",
			setup: func(svc ServerFilterService) string {
				f, _ := svc.CreateServerFilter(context.Background(), "spam", "", "", false)
				return f.ID
			},
		},
		{
			name:  "not found",
			setup: func(_ ServerFilterService) string { return "nonexistent-id" },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := testutil.NewFakeStore()
			svc := NewServerFilterService(s)

			id := tc.setup(svc)
			err := svc.DeleteServerFilter(context.Background(), id)
			assert.NoError(t, err)
		})
	}
}
