package service

import (
	"context"
	"testing"

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
