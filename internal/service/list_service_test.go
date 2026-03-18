package service

import (
	"context"
	"testing"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListService_CreateList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewListService(fake)
	fake.SeedAccount(&domain.Account{ID: "acct1", Username: "alice"})

	tests := []struct {
		name          string
		title         string
		repliesPolicy string
		exclusive     bool
		wantPolicy    string
		wantErr       error
	}{
		{
			name:       "success",
			title:      "My List",
			wantPolicy: domain.ListRepliesPolicyList,
		},
		{
			name:    "empty title",
			title:   "",
			wantErr: domain.ErrValidation,
		},
		{
			name:          "custom replies policy",
			title:         "Followed Only",
			repliesPolicy: domain.ListRepliesPolicyFollowed,
			wantPolicy:    domain.ListRepliesPolicyFollowed,
		},
		{
			name:          "invalid replies policy",
			title:         "Bad Policy",
			repliesPolicy: "bogus",
			wantPolicy:    domain.ListRepliesPolicyList,
		},
		{
			name:       "exclusive flag",
			title:      "Exclusive List",
			exclusive:  true,
			wantPolicy: domain.ListRepliesPolicyList,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l, err := svc.CreateList(ctx, "acct1", tc.title, tc.repliesPolicy, tc.exclusive)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, l)
			assert.Equal(t, tc.title, l.Title)
			assert.Equal(t, tc.wantPolicy, l.RepliesPolicy)
			assert.Equal(t, tc.exclusive, l.Exclusive)
			assert.Equal(t, "acct1", l.AccountID)
			assert.NotEmpty(t, l.ID)
		})
	}
}

func TestListService_GetList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewListService(fake)
	fake.SeedAccount(&domain.Account{ID: "acct1", Username: "alice"})
	fake.SeedAccount(&domain.Account{ID: "acct2", Username: "bob"})

	created, err := svc.CreateList(ctx, "acct1", "My List", "", false)
	require.NoError(t, err)

	tests := []struct {
		name      string
		accountID string
		listID    string
		wantErr   error
	}{
		{
			name:      "own list",
			accountID: "acct1",
			listID:    created.ID,
		},
		{
			name:      "other user's list",
			accountID: "acct2",
			listID:    created.ID,
			wantErr:   domain.ErrForbidden,
		},
		{
			name:      "not found",
			accountID: "acct1",
			listID:    "nonexistent",
			wantErr:   domain.ErrNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l, err := svc.GetList(ctx, tc.accountID, tc.listID)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, created.ID, l.ID)
			assert.Equal(t, "My List", l.Title)
		})
	}
}

func TestListService_UpdateList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewListService(fake)
	fake.SeedAccount(&domain.Account{ID: "acct1", Username: "alice"})
	fake.SeedAccount(&domain.Account{ID: "acct2", Username: "bob"})

	created, err := svc.CreateList(ctx, "acct1", "Original", domain.ListRepliesPolicyFollowed, false)
	require.NoError(t, err)

	tests := []struct {
		name          string
		accountID     string
		title         string
		repliesPolicy string
		wantTitle     string
		wantPolicy    string
		wantErr       error
	}{
		{
			name:       "update title",
			accountID:  "acct1",
			title:      "Updated",
			wantTitle:  "Updated",
			wantPolicy: domain.ListRepliesPolicyFollowed,
		},
		{
			name:      "other user's list",
			accountID: "acct2",
			title:     "Hacked",
			wantErr:   domain.ErrForbidden,
		},
		{
			name:       "empty title keeps existing",
			accountID:  "acct1",
			title:      "",
			wantTitle:  "Updated",
			wantPolicy: domain.ListRepliesPolicyFollowed,
		},
		{
			name:          "invalid replies policy keeps existing",
			accountID:     "acct1",
			repliesPolicy: "bogus",
			wantTitle:     "Updated",
			wantPolicy:    domain.ListRepliesPolicyFollowed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l, err := svc.UpdateList(ctx, tc.accountID, created.ID, tc.title, tc.repliesPolicy, false)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantTitle, l.Title)
			assert.Equal(t, tc.wantPolicy, l.RepliesPolicy)
		})
	}
}

func TestListService_DeleteList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewListService(fake)
	fake.SeedAccount(&domain.Account{ID: "acct1", Username: "alice"})
	fake.SeedAccount(&domain.Account{ID: "acct2", Username: "bob"})

	tests := []struct {
		name      string
		accountID string
		wantErr   error
	}{
		{
			name:      "own list",
			accountID: "acct1",
		},
		{
			name:      "other user's list",
			accountID: "acct2",
			wantErr:   domain.ErrForbidden,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			created, err := svc.CreateList(ctx, "acct1", "To Delete", "", false)
			require.NoError(t, err)

			err = svc.DeleteList(ctx, tc.accountID, created.ID)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)

			_, err = svc.GetList(ctx, "acct1", created.ID)
			assert.ErrorIs(t, err, domain.ErrNotFound)
		})
	}
}

func TestListService_GetListAccounts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewListService(fake)
	fake.SeedAccount(&domain.Account{ID: "owner", Username: "alice"})
	fake.SeedAccount(&domain.Account{ID: "member1", Username: "bob"})
	fake.SeedAccount(&domain.Account{ID: "member2", Username: "carol"})
	fake.SeedAccount(&domain.Account{ID: "other", Username: "dave"})

	created, err := svc.CreateList(ctx, "owner", "Friends", "", false)
	require.NoError(t, err)
	require.NoError(t, svc.AddAccountsToList(ctx, "owner", created.ID, []string{"member1", "member2"}))

	tests := []struct {
		name      string
		ownerID   string
		wantCount int
		wantErr   error
	}{
		{
			name:      "returns accounts",
			ownerID:   "owner",
			wantCount: 2,
		},
		{
			name:    "other user's list",
			ownerID: "other",
			wantErr: domain.ErrForbidden,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			accounts, err := svc.GetListAccounts(ctx, tc.ownerID, created.ID)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Len(t, accounts, tc.wantCount)
		})
	}

	t.Run("skips suspended", func(t *testing.T) {
		t.Parallel()
		suspendFake := testutil.NewFakeStore()
		suspendSvc := NewListService(suspendFake)
		suspendFake.SeedAccount(&domain.Account{ID: "owner2", Username: "alice2"})
		suspendFake.SeedAccount(&domain.Account{ID: "active", Username: "active"})
		suspendFake.SeedAccount(&domain.Account{ID: "suspended", Username: "suspended"})

		list, err := suspendSvc.CreateList(ctx, "owner2", "Mixed", "", false)
		require.NoError(t, err)
		require.NoError(t, suspendSvc.AddAccountsToList(ctx, "owner2", list.ID, []string{"active", "suspended"}))
		require.NoError(t, suspendFake.SuspendAccount(ctx, "suspended"))

		accounts, err := suspendSvc.GetListAccounts(ctx, "owner2", list.ID)
		require.NoError(t, err)
		assert.Len(t, accounts, 1)
		assert.Equal(t, "active", accounts[0].ID)
	})
}

func TestListService_AddAccountsToList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewListService(fake)
	fake.SeedAccount(&domain.Account{ID: "owner", Username: "alice"})
	fake.SeedAccount(&domain.Account{ID: "other", Username: "bob"})
	fake.SeedAccount(&domain.Account{ID: "member", Username: "carol"})

	created, err := svc.CreateList(ctx, "owner", "Friends", "", false)
	require.NoError(t, err)

	tests := []struct {
		name      string
		accountID string
		wantErr   error
	}{
		{
			name:      "success",
			accountID: "owner",
		},
		{
			name:      "other user's list",
			accountID: "other",
			wantErr:   domain.ErrForbidden,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.AddAccountsToList(ctx, tc.accountID, created.ID, []string{"member"})
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)

			accounts, err := svc.GetListAccounts(ctx, "owner", created.ID)
			require.NoError(t, err)
			assert.NotEmpty(t, accounts)
		})
	}
}

func TestListService_RemoveAccountsFromList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewListService(fake)
	fake.SeedAccount(&domain.Account{ID: "owner", Username: "alice"})
	fake.SeedAccount(&domain.Account{ID: "other", Username: "bob"})
	fake.SeedAccount(&domain.Account{ID: "member", Username: "carol"})

	created, err := svc.CreateList(ctx, "owner", "Friends", "", false)
	require.NoError(t, err)
	require.NoError(t, svc.AddAccountsToList(ctx, "owner", created.ID, []string{"member"}))

	tests := []struct {
		name      string
		accountID string
		wantErr   error
	}{
		{
			name:      "success",
			accountID: "owner",
		},
		{
			name:      "other user's list",
			accountID: "other",
			wantErr:   domain.ErrForbidden,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.RemoveAccountsFromList(ctx, tc.accountID, created.ID, []string{"member"})
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
