package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testRemoteDomain = "remote.example"

func TestAccountService_Create(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")

	acc, err := svc.Create(ctx, CreateAccountInput{
		Username: "alice",
		Bot:      false,
		Locked:   false,
	})
	require.NoError(t, err)
	require.NotNil(t, acc)
	assert.Equal(t, "alice", acc.Username)
	assert.Empty(t, acc.Domain)
	assert.NotEmpty(t, acc.PublicKey)
	assert.NotEmpty(t, acc.PrivateKey)
	assert.Contains(t, acc.InboxURL, "alice/inbox")
	assert.Contains(t, acc.OutboxURL, "alice/outbox")
	assert.Contains(t, acc.APID, "users/alice")
}

func TestAccountService_Create_duplicate_username_returns_conflict(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")

	_, err := svc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)
	_, err = svc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrConflict)
}

func TestAccountService_Create_empty_username_returns_validation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")

	_, err := svc.Create(ctx, CreateAccountInput{Username: ""})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestAccountService_GetByID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")

	created, err := svc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	got, err := svc.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "alice", got.Username)
}

func TestAccountService_GetByID_not_found(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")

	_, err := svc.GetByID(ctx, "01nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestAccountService_GetByUsername_local(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")

	_, err := svc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	got, err := svc.GetByUsername(ctx, "alice", nil)
	require.NoError(t, err)
	assert.Equal(t, "alice", got.Username)
}

func TestAccountService_GetByUsername_not_found(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")

	_, err := svc.GetByUsername(ctx, "nobody", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestAccountService_Register_invalid_role_returns_validation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")

	_, err := svc.Register(ctx, RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hashed",
		Role:     "superadmin",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestAccountService_Register(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")

	acc, err := svc.Register(ctx, RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hashed",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	require.NotNil(t, acc)
	assert.Equal(t, "alice", acc.Username)

	got, err := svc.GetByID(ctx, acc.ID)
	require.NoError(t, err)
	assert.Equal(t, acc.ID, got.ID)
}

func TestAccountService_GetAccountWithUser(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")

	registered, err := svc.Register(ctx, RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	acc, user, err := svc.GetAccountWithUser(ctx, registered.ID)
	require.NoError(t, err)
	require.NotNil(t, acc)
	require.NotNil(t, user)
	assert.Equal(t, registered.ID, acc.ID)
	assert.Equal(t, "alice", acc.Username)
	assert.Equal(t, registered.ID, user.AccountID)
	assert.Equal(t, "alice@example.com", user.Email)
}

func TestAccountService_GetAccountWithUser_account_not_found(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")

	_, _, err := svc.GetAccountWithUser(ctx, "01nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestAccountService_GetAccountWithUser_user_not_found(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")

	acc, err := svc.Create(ctx, CreateAccountInput{Username: "orphan"})
	require.NoError(t, err)

	_, _, err = svc.GetAccountWithUser(ctx, acc.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestAccountService_GetActiveLocalAccount_confirmed_returns_account(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")

	acc, err := svc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)
	_, err = fake.CreateUser(ctx, store.CreateUserInput{
		ID:           "01USERBOB",
		AccountID:    acc.ID,
		Email:        "bob@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	require.NoError(t, fake.ConfirmUser(ctx, "01USERBOB"))

	got, err := svc.GetActiveLocalAccount(ctx, "bob")
	require.NoError(t, err)
	assert.Equal(t, "bob", got.Username)
	assert.Equal(t, acc.ID, got.ID)
}

func TestAccountService_GetActiveLocalAccount_pending_returns_not_found(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")

	acc, err := svc.Create(ctx, CreateAccountInput{Username: "pending"})
	require.NoError(t, err)
	_, err = fake.CreateUser(ctx, store.CreateUserInput{
		ID:           "01USERPEND",
		AccountID:    acc.ID,
		Email:        "pending@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	// Do not confirm — user remains pending.

	_, err = svc.GetActiveLocalAccount(ctx, "pending")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestAccountService_GetByID_returns_avatar_url_from_store(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")

	acc, err := svc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	media, err := fake.CreateMediaAttachment(ctx, store.CreateMediaAttachmentInput{
		ID:         "01MEDIA",
		AccountID:  acc.ID,
		Type:       "image",
		StorageKey: "avatar.jpg",
		URL:        "https://example.com/media/avatar.jpg",
	})
	require.NoError(t, err)

	err = fake.UpdateAccount(ctx, store.UpdateAccountInput{
		ID:            acc.ID,
		AvatarMediaID: &media.ID,
		AvatarURL:     &media.URL,
	})
	require.NoError(t, err)

	got, err := svc.GetByID(ctx, acc.ID)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/media/avatar.jpg", got.AvatarURL)
}

func TestAccountService_GetActiveLocalAccount_returns_avatar_and_header_urls(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")

	acc, err := svc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	_, err = fake.CreateUser(ctx, store.CreateUserInput{
		ID:           "01USERALICE",
		AccountID:    acc.ID,
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	require.NoError(t, fake.ConfirmUser(ctx, "01USERALICE"))

	avatarMedia, err := fake.CreateMediaAttachment(ctx, store.CreateMediaAttachmentInput{
		ID:         "01AVATAR",
		AccountID:  acc.ID,
		Type:       "image",
		StorageKey: "avatar.jpg",
		URL:        "https://example.com/media/avatar.jpg",
	})
	require.NoError(t, err)

	headerMedia, err := fake.CreateMediaAttachment(ctx, store.CreateMediaAttachmentInput{
		ID:         "01HEADER",
		AccountID:  acc.ID,
		Type:       "image",
		StorageKey: "header.jpg",
		URL:        "https://example.com/media/header.jpg",
	})
	require.NoError(t, err)

	err = fake.UpdateAccount(ctx, store.UpdateAccountInput{
		ID:            acc.ID,
		AvatarMediaID: &avatarMedia.ID,
		HeaderMediaID: &headerMedia.ID,
		AvatarURL:     &avatarMedia.URL,
		HeaderURL:     &headerMedia.URL,
	})
	require.NoError(t, err)

	got, err := svc.GetActiveLocalAccount(ctx, "alice")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/media/avatar.jpg", got.AvatarURL)
	assert.Equal(t, "https://example.com/media/header.jpg", got.HeaderURL)
}

func TestAccountService_SuspendRemote_remote_account(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")

	remoteDomain := testRemoteDomain
	acc, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01remote", Username: "bob", Domain: &remoteDomain,
	})
	require.NoError(t, err)

	err = svc.SuspendRemote(ctx, acc.ID)
	require.NoError(t, err)

	got, err := svc.GetByID(ctx, acc.ID)
	require.NoError(t, err)
	assert.True(t, got.Suspended)
}

func TestAccountService_SuspendRemote_local_account_returns_forbidden(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")

	acc, err := svc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	err = svc.SuspendRemote(ctx, acc.ID)
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestAccountService_SuspendRemote_not_found(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")

	err := svc.SuspendRemote(ctx, "nonexistent")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestAccountService_CreateOrUpdateRemoteAccount_local_account_returns_forbidden(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")

	acc, err := svc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	evilName := "Evil"
	_, err = svc.CreateOrUpdateRemoteAccount(ctx, CreateOrUpdateRemoteInput{
		APID:         acc.APID,
		Username:     "alice",
		Domain:       "evil.example",
		DisplayName:  &evilName,
		PublicKey:    "fake-key",
		InboxURL:     "https://evil.example/users/alice/inbox",
		OutboxURL:    "https://evil.example/users/alice/outbox",
		FollowersURL: "https://evil.example/users/alice/followers",
		FollowingURL: "https://evil.example/users/alice/following",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestAccountService_CreateOrUpdateRemoteAccount_empty_domain_returns_validation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")

	_, err := svc.CreateOrUpdateRemoteAccount(ctx, CreateOrUpdateRemoteInput{
		APID:         "https://" + testRemoteDomain + "/users/bob",
		Username:     "bob",
		Domain:       "",
		PublicKey:    "fake-key",
		InboxURL:     "https://" + testRemoteDomain + "/users/bob/inbox",
		OutboxURL:    "https://" + testRemoteDomain + "/users/bob/outbox",
		FollowersURL: "https://" + testRemoteDomain + "/users/bob/followers",
		FollowingURL: "https://" + testRemoteDomain + "/users/bob/following",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestAccountService_CreateOrUpdateRemoteAccount_create_persists_avatar_header_and_counts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")

	displayName := "Kevin"
	note := "<p>columnist</p>"
	acc, err := svc.CreateOrUpdateRemoteAccount(ctx, CreateOrUpdateRemoteInput{
		APID:           "https://" + testRemoteDomain + "/users/kevin",
		Username:       "kevin",
		Domain:         testRemoteDomain,
		DisplayName:    &displayName,
		Note:           &note,
		PublicKey:      "fake-key",
		InboxURL:       "https://" + testRemoteDomain + "/users/kevin/inbox",
		OutboxURL:      "https://" + testRemoteDomain + "/users/kevin/outbox",
		FollowersURL:   "https://" + testRemoteDomain + "/users/kevin/followers",
		FollowingURL:   "https://" + testRemoteDomain + "/users/kevin/following",
		AvatarURL:      "https://" + testRemoteDomain + "/avatars/kevin.jpg",
		HeaderURL:      "https://" + testRemoteDomain + "/headers/kevin.jpg",
		FollowersCount: 1234,
		FollowingCount: 56,
		StatusesCount:  789,
	})
	require.NoError(t, err)

	// Verify the returned account has the correct values.
	assert.Equal(t, "https://"+testRemoteDomain+"/avatars/kevin.jpg", acc.AvatarURL)
	assert.Equal(t, "https://"+testRemoteDomain+"/headers/kevin.jpg", acc.HeaderURL)
	assert.Equal(t, 1234, acc.FollowersCount)
	assert.Equal(t, 56, acc.FollowingCount)
	assert.Equal(t, 789, acc.StatusesCount)

	// Verify a subsequent fetch returns the same values (persisted, not in-memory only).
	fetched, err := svc.GetByID(ctx, acc.ID)
	require.NoError(t, err)
	assert.Equal(t, "https://"+testRemoteDomain+"/avatars/kevin.jpg", fetched.AvatarURL)
	assert.Equal(t, "https://"+testRemoteDomain+"/headers/kevin.jpg", fetched.HeaderURL)
	assert.Equal(t, 1234, fetched.FollowersCount)
	assert.Equal(t, 56, fetched.FollowingCount)
	assert.Equal(t, 789, fetched.StatusesCount)
}

func TestAccountService_CreateOrUpdateRemoteAccount_update_refreshes_avatar_header_and_counts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")

	displayName := "Kevin"
	base := CreateOrUpdateRemoteInput{
		APID:           "https://" + testRemoteDomain + "/users/kevin",
		Username:       "kevin",
		Domain:         testRemoteDomain,
		DisplayName:    &displayName,
		PublicKey:      "fake-key",
		InboxURL:       "https://" + testRemoteDomain + "/users/kevin/inbox",
		OutboxURL:      "https://" + testRemoteDomain + "/users/kevin/outbox",
		FollowersURL:   "https://" + testRemoteDomain + "/users/kevin/followers",
		FollowingURL:   "https://" + testRemoteDomain + "/users/kevin/following",
		AvatarURL:      "https://" + testRemoteDomain + "/avatars/old.jpg",
		HeaderURL:      "https://" + testRemoteDomain + "/headers/old.jpg",
		FollowersCount: 100,
		FollowingCount: 10,
		StatusesCount:  50,
	}
	_, err := svc.CreateOrUpdateRemoteAccount(ctx, base)
	require.NoError(t, err)

	// Update with new avatar, header, and counts.
	base.AvatarURL = "https://" + testRemoteDomain + "/avatars/new.jpg"
	base.HeaderURL = "https://" + testRemoteDomain + "/headers/new.jpg"
	base.FollowersCount = 200
	base.FollowingCount = 20
	base.StatusesCount = 100
	updated, err := svc.CreateOrUpdateRemoteAccount(ctx, base)
	require.NoError(t, err)

	assert.Equal(t, "https://"+testRemoteDomain+"/avatars/new.jpg", updated.AvatarURL)
	assert.Equal(t, "https://"+testRemoteDomain+"/headers/new.jpg", updated.HeaderURL)
	assert.Equal(t, 200, updated.FollowersCount)
	assert.Equal(t, 20, updated.FollowingCount)
	assert.Equal(t, 100, updated.StatusesCount)

	// Verify persisted via a fresh fetch.
	fetched, err := svc.GetByID(ctx, updated.ID)
	require.NoError(t, err)
	assert.Equal(t, "https://"+testRemoteDomain+"/avatars/new.jpg", fetched.AvatarURL)
	assert.Equal(t, "https://"+testRemoteDomain+"/headers/new.jpg", fetched.HeaderURL)
	assert.Equal(t, 200, fetched.FollowersCount)
	assert.Equal(t, 20, fetched.FollowingCount)
	assert.Equal(t, 100, fetched.StatusesCount)
}

func TestAccountService_SetRemotePins_replaces_pins(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")

	remoteDomain := testRemoteDomain
	acc, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01remote", Username: "bob", Domain: &remoteDomain,
	})
	require.NoError(t, err)

	// Seed an existing pin that should be replaced.
	require.NoError(t, fake.CreateAccountPin(ctx, acc.ID, "old-status"))

	err = svc.SetRemotePins(ctx, acc.ID, []string{"new-status-1", "new-status-2"})
	require.NoError(t, err)

	pins, err := fake.ListPinnedStatusIDs(ctx, acc.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"new-status-1", "new-status-2"}, pins)
}

func TestAccountService_SetRemotePins_local_account_returns_forbidden(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")

	acc, err := svc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	err = svc.SetRemotePins(ctx, acc.ID, []string{"status-1"})
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

// seedLocalUserWithPassword creates a local account + confirmed user + active
// OAuth access token, all with a fixed "alice" identity (each test has its
// own FakeStore so the names don't collide across tests).
// Returns accountID, userID, and the raw password.
func seedLocalUserWithPassword(t *testing.T, fake *testutil.FakeStore, svc AccountService) (accountID, userID, password string) {
	t.Helper()
	ctx := context.Background()
	const username = "alice"
	acc, err := svc.Create(ctx, CreateAccountInput{Username: username})
	require.NoError(t, err)
	password = "correct-horse-battery-staple"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	userID = "user-" + username
	_, err = fake.CreateUser(ctx, store.CreateUserInput{
		ID:           userID,
		AccountID:    acc.ID,
		Email:        username + "@example.com",
		PasswordHash: string(hash),
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	require.NoError(t, fake.ConfirmUser(ctx, userID))
	// Seed an OAuth token so we can assert revocation.
	_, err = fake.CreateApplication(ctx, store.CreateApplicationInput{
		ID: "app-" + username, Name: "Test", ClientID: "client-" + username, ClientSecret: "secret",
	})
	require.NoError(t, err)
	accID := acc.ID
	_, err = fake.CreateAccessToken(ctx, store.CreateAccessTokenInput{
		ID: "tok-" + username, ApplicationID: "app-" + username, AccountID: &accID,
		Token: "token-" + username, Scopes: "read write",
	})
	require.NoError(t, err)
	return acc.ID, userID, password
}

func TestAccountService_DeleteSelf_success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")
	accID, userID, password := seedLocalUserWithPassword(t, fake, svc)

	err := svc.DeleteSelf(ctx, userID, password)
	require.NoError(t, err)

	// Account row is gone (CASCADE drops user, tokens, auth codes).
	_, err = svc.GetByID(ctx, accID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = fake.GetUserByAccountID(ctx, accID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = fake.GetAccessToken(ctx, "token-alice")
	require.ErrorIs(t, err, domain.ErrNotFound)

	// Federation event is queued in the outbox so remote followers get a Delete{Actor}.
	var found bool
	for _, ev := range fake.OutboxEvents {
		if ev.EventType == domain.EventAccountDeleted && ev.AggregateID == accID {
			found = true
			break
		}
	}
	assert.True(t, found, "expected account.deleted outbox event for %s", accID)
}

func TestAccountService_DeleteSelf_wrong_password(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")
	accID, userID, _ := seedLocalUserWithPassword(t, fake, svc)

	err := svc.DeleteSelf(ctx, userID, "nope")
	require.ErrorIs(t, err, domain.ErrForbidden)

	// Account intact; no event emitted.
	acc, err := svc.GetByID(ctx, accID)
	require.NoError(t, err)
	assert.False(t, acc.Suspended)
	assert.Empty(t, fake.OutboxEvents)
}

func TestAccountService_DeleteLocalAccount_remote_account_refused(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")

	remoteDomain := testRemoteDomain
	acc, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01remote", Username: "bob", Domain: &remoteDomain,
	})
	require.NoError(t, err)

	err = svc.DeleteLocalAccount(ctx, acc.ID)
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

// CASCADE FK on oauth_authorization_codes drops outstanding codes when the
// account row is deleted, so a captured code can't be exchanged afterwards.
func TestAccountService_DeleteSelf_drops_authorization_codes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")
	accID, userID, password := seedLocalUserWithPassword(t, fake, svc)

	_, err := fake.CreateAuthorizationCode(ctx, store.CreateAuthorizationCodeInput{
		ID:            "code-1",
		Code:          "code-raw",
		ApplicationID: "app-alice",
		AccountID:     accID,
		RedirectURI:   "https://client/callback",
		Scopes:        "read",
		ExpiresAt:     time.Now().Add(5 * time.Minute),
	})
	require.NoError(t, err)

	require.NoError(t, svc.DeleteSelf(ctx, userID, password))

	_, err = fake.GetAuthorizationCode(ctx, "code-raw")
	require.ErrorIs(t, err, domain.ErrNotFound)
}

// DeleteSelf captures a snapshot of the signing material and the remote
// follower inbox URLs BEFORE the account row (and its follows) are dropped.
// Without this, federation would have no way to deliver Delete{Actor} —
// the accounts row is gone and the follows CASCADE-wipe leaves no inbox
// list. Verifies that:
//   - a snapshot row exists keyed by the payload's DeletionID,
//     carrying the account's APID and private key;
//   - an account_deletion_targets row exists for each distinct remote
//     follower inbox;
//   - the payload itself carries only DeletionID+APID (private key never
//     hits outbox_events / NATS).
func TestAccountService_DeleteSelf_populates_snapshot_and_targets(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")
	accID, userID, password := seedLocalUserWithPassword(t, fake, svc)

	// Two remote followers on distinct instances, both accepted.
	remote1Domain := "remote.example"
	remote2Domain := "other.example"
	f1, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "follower-1", Username: "bob", Domain: &remote1Domain,
		InboxURL: "https://remote.example/users/bob/inbox",
	})
	require.NoError(t, err)
	f2, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "follower-2", Username: "carol", Domain: &remote2Domain,
		InboxURL: "https://other.example/users/carol/inbox",
	})
	require.NoError(t, err)
	_, err = fake.CreateFollow(ctx, store.CreateFollowInput{
		ID: "f1", AccountID: f1.ID, TargetID: accID, State: domain.FollowStateAccepted,
	})
	require.NoError(t, err)
	_, err = fake.CreateFollow(ctx, store.CreateFollowInput{
		ID: "f2", AccountID: f2.ID, TargetID: accID, State: domain.FollowStateAccepted,
	})
	require.NoError(t, err)

	require.NoError(t, svc.DeleteSelf(ctx, userID, password))

	// Locate the emitted event to read the DeletionID.
	var ev domain.DomainEvent
	for _, e := range fake.OutboxEvents {
		if e.EventType == domain.EventAccountDeleted && e.AggregateID == accID {
			ev = e
			break
		}
	}
	require.NotEmpty(t, ev.ID, "expected an account.deleted event")
	var payload domain.AccountDeletedPayload
	require.NoError(t, json.Unmarshal(ev.Payload, &payload))
	require.NotEmpty(t, payload.DeletionID, "payload must carry deletion id")
	require.NotEmpty(t, payload.APID, "payload must carry actor IRI")
	assert.True(t, payload.Local)
	assert.NotContains(t, string(ev.Payload), "private_key", "payload must not leak private key onto NATS/outbox")

	// Signing material is in the side table, not the payload.
	snap, err := fake.GetAccountDeletionSnapshot(ctx, payload.DeletionID)
	require.NoError(t, err)
	assert.Equal(t, payload.APID, snap.APID)
	assert.NotEmpty(t, snap.PrivateKeyPEM, "snapshot must retain PEM so delivery worker can sign post-CASCADE")

	// Targets captured one row per distinct remote follower inbox.
	urls, err := fake.ListPendingAccountDeletionTargets(ctx, payload.DeletionID, "", 100)
	require.NoError(t, err)
	assert.ElementsMatch(t,
		[]string{"https://other.example/users/carol/inbox", "https://remote.example/users/bob/inbox"},
		urls,
		"fanout worker must find both remote follower inboxes via the snapshot",
	)
}

// Concurrent self-delete + admin-delete calls on the same account commit
// exactly one EventAccountDeleted. Postgres row-locks serialize the deletes;
// the second tx reads the locked row, finds it gone after the first commits,
// and bails with ErrNotFound before emitting a second event or writing a
// second audit row. Without this guarantee the federation subscriber would
// fan out Delete{Actor} twice.
func TestAccountService_DeleteSelf_concurrent_delete_race(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewAccountService(fake, "https://example.com")
	accID, userID, password := seedLocalUserWithPassword(t, fake, svc)

	// Sequential on the fake (WithTx is not concurrent) — exercises the
	// "second caller sees no row" branch that the Postgres row lock
	// enforces concurrently.
	err1 := svc.DeleteSelf(ctx, userID, password)
	require.NoError(t, err1)
	err2 := svc.DeleteLocalAccount(ctx, accID)
	require.ErrorIs(t, err2, domain.ErrNotFound)

	var events int
	for _, e := range fake.OutboxEvents {
		if e.EventType == domain.EventAccountDeleted && e.AggregateID == accID {
			events++
		}
	}
	assert.Equal(t, 1, events, "exactly one account.deleted event must be emitted across both callers")
}

// ModerationService.DeleteAccount writes its admin_action row inside the same
// tx as the delete + event emit. If any inner step fails, the whole tx must
// roll back — otherwise an audit row lands for a delete that didn't happen,
// or the account is gone without an audit trail.
//
// Exercises the rollback by injecting an error at InsertOutboxEvent (which
// fires after DeleteAccount but before CreateAdminAction — the mid-tx
// position where rollback matters most).
func TestModerationService_DeleteAccount_rolls_back_on_tx_error(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	// Wrap the store with a shim that fails the outbox write. The fake's
	// WithTx restores its snapshot on fn error, so post-error state should
	// look as if the delete never happened.
	shim := &outboxInsertFailingStore{Store: fake, fail: true}
	mod := NewModerationService(shim, noopBlocklistRefresher{})

	// Seed a local target with a private key so deleteLocalAccount proceeds
	// far enough to hit the failing InsertOutboxEvent.
	const username = "alice"
	acc, err := NewAccountService(fake, "https://example.com").Create(ctx, CreateAccountInput{Username: username})
	require.NoError(t, err)
	_, err = fake.CreateUser(ctx, store.CreateUserInput{
		ID: "user-" + username, AccountID: acc.ID, Email: username + "@example.com",
		PasswordHash: "hash", Role: domain.RoleUser,
	})
	require.NoError(t, err)

	err = mod.DeleteAccount(ctx, "mod-1", acc.ID)
	require.Error(t, err, "expected DeleteAccount to surface the injected outbox failure")

	// Account must still exist — the tx rolled back.
	got, err := fake.GetAccountByID(ctx, acc.ID)
	require.NoError(t, err)
	assert.Equal(t, acc.ID, got.ID)

	// No partial event, no partial audit.
	for _, e := range fake.OutboxEvents {
		assert.NotEqual(t, domain.EventAccountDeleted, e.EventType, "no account.deleted event may survive rollback")
	}
	assert.Empty(t, fake.AdminActions, "no admin_actions row may survive rollback")
}

// outboxInsertFailingStore wraps a store.Store and forces InsertOutboxEvent
// to return an error when fail is true. Used to exercise tx-rollback paths.
type outboxInsertFailingStore struct {
	store.Store
	fail bool
}

func (f *outboxInsertFailingStore) InsertOutboxEvent(ctx context.Context, in store.InsertOutboxEventInput) error {
	if f.fail {
		return errors.New("injected outbox failure")
	}
	return f.Store.InsertOutboxEvent(ctx, in)
}

func (f *outboxInsertFailingStore) WithTx(ctx context.Context, fn func(store.Store) error) error {
	return f.Store.WithTx(ctx, func(tx store.Store) error {
		return fn(&outboxInsertFailingStore{Store: tx, fail: f.fail})
	})
}
