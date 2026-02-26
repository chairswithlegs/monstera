package service

import (
	"context"
	"testing"

	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hashed",
		Role:         "superadmin",
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
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hashed",
		Role:         domain.RoleUser,
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
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
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
