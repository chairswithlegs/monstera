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
