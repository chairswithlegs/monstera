//go:build integration

package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

func TestIntegration_AccountStore(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("CreateAccount_and_GetByID", func(t *testing.T) {
		acc := createTestRemoteAccount(t, s, ctx)

		got, err := s.GetAccountByID(ctx, acc.ID)
		require.NoError(t, err)
		assert.Equal(t, acc.ID, got.ID)
		assert.Equal(t, acc.Username, got.Username)
		assert.NotNil(t, got.Domain)
		assert.Equal(t, "remote.example", *got.Domain)
	})

	t.Run("GetAccountByID_not_found", func(t *testing.T) {
		_, err := s.GetAccountByID(ctx, "nonexistent_"+uid.New())
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("GetAccountByAPID", func(t *testing.T) {
		acc := createTestRemoteAccount(t, s, ctx)
		got, err := s.GetAccountByAPID(ctx, acc.APID)
		require.NoError(t, err)
		assert.Equal(t, acc.ID, got.ID)
	})

	t.Run("GetAccountByAPID_not_found", func(t *testing.T) {
		_, err := s.GetAccountByAPID(ctx, "https://nowhere.example/users/ghost")
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("GetLocalAccountByUsername", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		got, err := s.GetLocalAccountByUsername(ctx, acc.Username)
		require.NoError(t, err)
		assert.Equal(t, acc.ID, got.ID)
		assert.Nil(t, got.Domain)
	})

	t.Run("GetLocalAccountByUsername_not_found", func(t *testing.T) {
		_, err := s.GetLocalAccountByUsername(ctx, "nope_"+uid.New())
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("GetRemoteAccountByUsername", func(t *testing.T) {
		acc := createTestRemoteAccount(t, s, ctx)
		got, err := s.GetRemoteAccountByUsername(ctx, acc.Username, acc.Domain)
		require.NoError(t, err)
		assert.Equal(t, acc.ID, got.ID)
	})

	t.Run("GetAccountsByIDs", func(t *testing.T) {
		a1 := createTestRemoteAccount(t, s, ctx)
		a2 := createTestRemoteAccount(t, s, ctx)
		got, err := s.GetAccountsByIDs(ctx, []string{a1.ID, a2.ID})
		require.NoError(t, err)
		assert.Len(t, got, 2)
	})

	t.Run("GetAccountsByIDs_empty", func(t *testing.T) {
		got, err := s.GetAccountsByIDs(ctx, nil)
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("SearchAccounts", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		results, err := s.SearchAccounts(ctx, acc.Username, 10, 0)
		require.NoError(t, err)
		assert.NotEmpty(t, results)

		found := false
		for _, r := range results {
			if r.ID == acc.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "expected account %s in search results", acc.ID)

		// offset skips past the first result
		skipped, err := s.SearchAccounts(ctx, acc.Username, 10, 1000)
		require.NoError(t, err)
		assert.Empty(t, skipped)
	})

	t.Run("SearchAccountsFollowing", func(t *testing.T) {
		viewer := createTestLocalAccount(t, s, ctx)
		followed := createTestLocalAccount(t, s, ctx)
		notFollowed := createTestLocalAccount(t, s, ctx)

		_, err := s.CreateFollow(ctx, store.CreateFollowInput{
			ID:        uid.New(),
			AccountID: viewer.ID,
			TargetID:  followed.ID,
			State:     domain.FollowStateAccepted,
		})
		require.NoError(t, err)

		// Search by a prefix that matches both followed and notFollowed
		results, err := s.SearchAccountsFollowing(ctx, viewer.ID, followed.Username, 10, 0)
		require.NoError(t, err)

		ids := make([]string, 0, len(results))
		for _, r := range results {
			ids = append(ids, r.ID)
		}
		assert.Contains(t, ids, followed.ID, "expected followed account in results")
		assert.NotContains(t, ids, notFollowed.ID, "expected non-followed account excluded")
	})

	t.Run("UpdateAccount", func(t *testing.T) {
		acc := createTestRemoteAccount(t, s, ctx)
		newName := "Updated " + acc.ID[:8]
		err := s.UpdateAccount(ctx, store.UpdateAccountInput{
			ID:          acc.ID,
			DisplayName: &newName,
		})
		require.NoError(t, err)

		got, err := s.GetAccountByID(ctx, acc.ID)
		require.NoError(t, err)
		require.NotNil(t, got.DisplayName)
		assert.Equal(t, newName, *got.DisplayName)
	})

	t.Run("SuspendAccount_UnsuspendAccount", func(t *testing.T) {
		acc := createTestRemoteAccount(t, s, ctx)

		err := s.SuspendAccount(ctx, acc.ID)
		require.NoError(t, err)

		got, err := s.GetAccountByID(ctx, acc.ID)
		require.NoError(t, err)
		assert.True(t, got.Suspended)

		err = s.UnsuspendAccount(ctx, acc.ID)
		require.NoError(t, err)

		got, err = s.GetAccountByID(ctx, acc.ID)
		require.NoError(t, err)
		assert.False(t, got.Suspended)
	})

	t.Run("SilenceAccount_UnsilenceAccount", func(t *testing.T) {
		acc := createTestRemoteAccount(t, s, ctx)

		err := s.SilenceAccount(ctx, acc.ID)
		require.NoError(t, err)

		got, err := s.GetAccountByID(ctx, acc.ID)
		require.NoError(t, err)
		assert.True(t, got.Silenced)

		err = s.UnsilenceAccount(ctx, acc.ID)
		require.NoError(t, err)

		got, err = s.GetAccountByID(ctx, acc.ID)
		require.NoError(t, err)
		assert.False(t, got.Silenced)
	})

	t.Run("CountLocalAccounts", func(t *testing.T) {
		createTestLocalAccount(t, s, ctx)
		n, err := s.CountLocalAccounts(ctx)
		require.NoError(t, err)
		assert.Greater(t, n, int64(0))
	})

	t.Run("ListLocalAccounts", func(t *testing.T) {
		createTestLocalAccount(t, s, ctx)
		accounts, err := s.ListLocalAccounts(ctx, 10, 0)
		require.NoError(t, err)
		assert.NotEmpty(t, accounts)
	})

	t.Run("UpdateAccountKeys", func(t *testing.T) {
		acc := createTestRemoteAccount(t, s, ctx)
		newKey := "-----BEGIN PUBLIC KEY-----\nNEWKEY_" + uid.New() + "\n-----END PUBLIC KEY-----"
		err := s.UpdateAccountKeys(ctx, acc.ID, newKey)
		require.NoError(t, err)

		got, err := s.GetAccountByID(ctx, acc.ID)
		require.NoError(t, err)
		assert.Equal(t, newKey, got.PublicKey)
	})

	t.Run("UpdateAccountURLs", func(t *testing.T) {
		acc := createTestRemoteAccount(t, s, ctx)
		newInbox := "https://new.example/inbox/" + uid.New()
		err := s.UpdateAccountURLs(ctx, acc.ID, newInbox, acc.OutboxURL, acc.FollowersURL, acc.FollowingURL)
		require.NoError(t, err)

		got, err := s.GetAccountByID(ctx, acc.ID)
		require.NoError(t, err)
		assert.Equal(t, newInbox, got.InboxURL)
	})

	t.Run("IncrementStatusesCount_DecrementStatusesCount", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)

		err := s.IncrementStatusesCount(ctx, acc.ID)
		require.NoError(t, err)
		got, err := s.GetAccountByID(ctx, acc.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, got.StatusesCount)

		err = s.DecrementStatusesCount(ctx, acc.ID)
		require.NoError(t, err)
		got, err = s.GetAccountByID(ctx, acc.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, got.StatusesCount)
	})

	t.Run("CreateAccountPin_ListPinnedStatusIDs_DeleteAccountPin", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)

		err := s.CreateAccountPin(ctx, acc.ID, st.ID)
		require.NoError(t, err)

		pinned, err := s.ListPinnedStatusIDs(ctx, acc.ID)
		require.NoError(t, err)
		assert.Contains(t, pinned, st.ID)

		cnt, err := s.CountAccountPins(ctx, acc.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), cnt)

		err = s.DeleteAccountPin(ctx, acc.ID, st.ID)
		require.NoError(t, err)

		pinned, err = s.ListPinnedStatusIDs(ctx, acc.ID)
		require.NoError(t, err)
		assert.NotContains(t, pinned, st.ID)
	})

	t.Run("DeleteAccount", func(t *testing.T) {
		acc := createTestRemoteAccount(t, s, ctx)
		deleted, err := s.DeleteAccount(ctx, acc.ID)
		require.NoError(t, err)
		require.NotNil(t, deleted)
		assert.Equal(t, acc.ID, deleted.ID)

		_, err = s.GetAccountByID(ctx, acc.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("ListDirectoryAccounts", func(t *testing.T) {
		createTestLocalAccount(t, s, ctx)
		accounts, err := s.ListDirectoryAccounts(ctx, "new", true, 0, 10)
		require.NoError(t, err)
		assert.NotEmpty(t, accounts)
	})
}

func TestIntegration_UserStore(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("CreateUser_GetByAccountID", func(t *testing.T) {
		_, user := createTestLocalAccountWithUser(t, s, ctx)

		got, err := s.GetUserByAccountID(ctx, user.AccountID)
		require.NoError(t, err)
		assert.Equal(t, user.ID, got.ID)
		assert.Equal(t, user.Email, got.Email)
		assert.Equal(t, domain.RoleUser, got.Role)
	})

	t.Run("GetUserByEmail", func(t *testing.T) {
		_, user := createTestLocalAccountWithUser(t, s, ctx)

		got, err := s.GetUserByEmail(ctx, user.Email)
		require.NoError(t, err)
		assert.Equal(t, user.ID, got.ID)
	})

	t.Run("GetUserByEmail_not_found", func(t *testing.T) {
		_, err := s.GetUserByEmail(ctx, "nobody_"+uid.New()+"@ghost.example")
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("GetUserByID", func(t *testing.T) {
		_, user := createTestLocalAccountWithUser(t, s, ctx)

		got, err := s.GetUserByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.Email, got.Email)
	})

	t.Run("GetUserByID_not_found", func(t *testing.T) {
		_, err := s.GetUserByID(ctx, "nonexistent_"+uid.New())
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("ConfirmUser", func(t *testing.T) {
		_, user := createTestLocalAccountWithUser(t, s, ctx)
		require.Nil(t, user.ConfirmedAt)

		err := s.ConfirmUser(ctx, user.ID)
		require.NoError(t, err)

		got, err := s.GetUserByID(ctx, user.ID)
		require.NoError(t, err)
		assert.NotNil(t, got.ConfirmedAt)
	})

	t.Run("UpdateUserRole", func(t *testing.T) {
		_, user := createTestLocalAccountWithUser(t, s, ctx)

		err := s.UpdateUserRole(ctx, user.ID, domain.RoleAdmin)
		require.NoError(t, err)

		got, err := s.GetUserByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.RoleAdmin, got.Role)
	})

	t.Run("UpdateUserPreferences", func(t *testing.T) {
		_, user := createTestLocalAccountWithUser(t, s, ctx)

		err := s.UpdateUserPreferences(ctx, store.UpdateUserPreferencesInput{
			UserID:             user.ID,
			DefaultPrivacy:     domain.VisibilityUnlisted,
			DefaultSensitive:   true,
			DefaultLanguage:    "fr",
			DefaultQuotePolicy: domain.QuotePolicyFollowers,
		})
		require.NoError(t, err)

		got, err := s.GetUserByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.VisibilityUnlisted, got.DefaultPrivacy)
		assert.True(t, got.DefaultSensitive)
		assert.Equal(t, "fr", got.DefaultLanguage)
		assert.Equal(t, domain.QuotePolicyFollowers, got.DefaultQuotePolicy)
	})

	t.Run("UpdateUserEmail", func(t *testing.T) {
		_, user := createTestLocalAccountWithUser(t, s, ctx)
		newEmail := "newemail_" + uid.New() + "@test.example"

		err := s.UpdateUserEmail(ctx, user.ID, newEmail)
		require.NoError(t, err)

		got, err := s.GetUserByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, newEmail, got.Email)
	})

	t.Run("UpdateUserPassword", func(t *testing.T) {
		_, user := createTestLocalAccountWithUser(t, s, ctx)
		newHash := "$2a$10$newhashnewhashnewhashnewhashnewhashnewhashnewhash"

		err := s.UpdateUserPassword(ctx, user.ID, newHash)
		require.NoError(t, err)

		got, err := s.GetUserByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, newHash, got.PasswordHash)
	})

	t.Run("ListLocalUsers", func(t *testing.T) {
		createTestLocalAccountWithUser(t, s, ctx)
		users, err := s.ListLocalUsers(ctx, 10, 0)
		require.NoError(t, err)
		assert.NotEmpty(t, users)
	})

	t.Run("GetPendingRegistrations", func(t *testing.T) {
		pending, err := s.GetPendingRegistrations(ctx)
		require.NoError(t, err)
		_ = pending
	})

	t.Run("DeleteUser", func(t *testing.T) {
		_, user := createTestLocalAccountWithUser(t, s, ctx)

		err := s.DeleteUser(ctx, user.ID)
		require.NoError(t, err)

		_, err = s.GetUserByID(ctx, user.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("CreateUser_duplicate_email_conflict", func(t *testing.T) {
		acc1 := createTestLocalAccount(t, s, ctx)
		acc2 := createTestLocalAccount(t, s, ctx)
		email := "dup_" + uid.New() + "@test.example"

		_, err := s.CreateUser(ctx, store.CreateUserInput{
			ID:           uid.New(),
			AccountID:    acc1.ID,
			Email:        email,
			PasswordHash: "$2a$10$fakehash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)

		_, err = s.CreateUser(ctx, store.CreateUserInput{
			ID:           uid.New(),
			AccountID:    acc2.ID,
			Email:        email,
			PasswordHash: "$2a$10$fakehash",
			Role:         domain.RoleUser,
		})
		require.ErrorIs(t, err, domain.ErrConflict)
	})

	t.Run("UpdateUserDefaultQuotePolicy", func(t *testing.T) {
		acc, user := createTestLocalAccountWithUser(t, s, ctx)

		err := s.UpdateUserDefaultQuotePolicy(ctx, acc.ID, domain.QuotePolicyNobody)
		require.NoError(t, err)

		got, err := s.GetUserByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.QuotePolicyNobody, got.DefaultQuotePolicy)
	})
}
