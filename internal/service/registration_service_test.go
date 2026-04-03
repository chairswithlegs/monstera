package service

import (
	"context"
	"testing"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeApprovedMailer struct {
	called bool
	to     string
}

func (m *fakeApprovedMailer) SendAccountApproved(_ context.Context, to, username, instanceName, instanceURL string) error {
	m.called = true
	m.to = to
	return nil
}

type fakeRejectedMailer struct {
	called bool
	to     string
}

func (m *fakeRejectedMailer) SendRegistrationRejected(_ context.Context, to, username, instanceName, reason string) error {
	m.called = true
	m.to = to
	return nil
}

func seedUnconfirmedUser(t *testing.T, fake *testutil.FakeStore) (*domain.User, *domain.Account) {
	t.Helper()
	acc := &domain.Account{
		ID:       uid.New(),
		Username: "pending",
	}
	u := &domain.User{
		ID:        uid.New(),
		AccountID: acc.ID,
		Email:     "pending@example.com",
		Role:      domain.RoleUser,
	}
	err := fake.SeedUserAndAccount(u, acc)
	require.NoError(t, err)
	return u, acc
}

func TestRegistrationService_Confirm(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewRegistrationService(fake, nil, nil, "https://example.com", nil)

		u, _ := seedUnconfirmedUser(t, fake)
		require.Nil(t, u.ConfirmedAt)

		err := svc.Confirm(ctx, u.ID)
		require.NoError(t, err)

		updated, err := fake.GetUserByID(ctx, u.ID)
		require.NoError(t, err)
		assert.NotNil(t, updated.ConfirmedAt)
	})
}

func TestRegistrationService_ListPending(t *testing.T) {
	t.Parallel()

	t.Run("returns pending users with accounts", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewRegistrationService(fake, nil, nil, "https://example.com", nil)

		u, acc := seedUnconfirmedUser(t, fake)

		pending, err := svc.ListPending(ctx)
		require.NoError(t, err)
		require.Len(t, pending, 1)
		assert.Equal(t, u.ID, pending[0].User.ID)
		assert.Equal(t, acc.ID, pending[0].Account.ID)
	})
}

func TestRegistrationService_Approve(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		mailer := &fakeApprovedMailer{}
		svc := NewRegistrationService(fake, mailer, nil, "https://example.com", nil)

		u, _ := seedUnconfirmedUser(t, fake)
		moderatorAcc := seedModerator(t, fake)

		err := svc.Approve(ctx, moderatorAcc.ID, u.ID)
		require.NoError(t, err)

		updated, err := fake.GetUserByID(ctx, u.ID)
		require.NoError(t, err)
		assert.NotNil(t, updated.ConfirmedAt)
		assert.True(t, mailer.called)
		assert.Equal(t, "pending@example.com", mailer.to)
	})

	t.Run("nil mailer", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewRegistrationService(fake, nil, nil, "https://example.com", nil)

		u, _ := seedUnconfirmedUser(t, fake)
		moderatorAcc := seedModerator(t, fake)

		err := svc.Approve(ctx, moderatorAcc.ID, u.ID)
		require.NoError(t, err)

		updated, err := fake.GetUserByID(ctx, u.ID)
		require.NoError(t, err)
		assert.NotNil(t, updated.ConfirmedAt)
	})

	t.Run("user not found", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewRegistrationService(fake, nil, nil, "https://example.com", nil)

		err := svc.Approve(ctx, "mod-id", "nonexistent")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestRegistrationService_Reject(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		mailer := &fakeRejectedMailer{}
		svc := NewRegistrationService(fake, nil, mailer, "https://example.com", nil)

		u, acc := seedUnconfirmedUser(t, fake)
		moderatorAcc := seedModerator(t, fake)

		err := svc.Reject(ctx, moderatorAcc.ID, u.ID, "not a good fit")
		require.NoError(t, err)

		assert.True(t, mailer.called)
		assert.Equal(t, "pending@example.com", mailer.to)

		_, err = fake.GetUserByID(ctx, u.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)

		_, err = fake.GetAccountByID(ctx, acc.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("nil mailer", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewRegistrationService(fake, nil, nil, "https://example.com", nil)

		u, _ := seedUnconfirmedUser(t, fake)
		moderatorAcc := seedModerator(t, fake)

		err := svc.Reject(ctx, moderatorAcc.ID, u.ID, "reason")
		require.NoError(t, err)
	})
}

func TestRegistrationService_CreateInvite(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewRegistrationService(fake, nil, nil, "https://example.com", nil)

		inv, err := svc.CreateInvite(ctx, "user-1", nil, nil)
		require.NoError(t, err)
		require.NotNil(t, inv)
		assert.NotEmpty(t, inv.Code)
		assert.Equal(t, "user-1", inv.CreatedBy)
		assert.Nil(t, inv.MaxUses)
		assert.Nil(t, inv.ExpiresAt)
	})

	t.Run("with max uses", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewRegistrationService(fake, nil, nil, "https://example.com", nil)

		maxUses := 5
		inv, err := svc.CreateInvite(ctx, "user-1", &maxUses, nil)
		require.NoError(t, err)
		require.NotNil(t, inv)
		require.NotNil(t, inv.MaxUses)
		assert.Equal(t, 5, *inv.MaxUses)
	})

	t.Run("with expires at", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewRegistrationService(fake, nil, nil, "https://example.com", nil)

		exp := time.Now().Add(24 * time.Hour)
		inv, err := svc.CreateInvite(ctx, "user-1", nil, &exp)
		require.NoError(t, err)
		require.NotNil(t, inv)
		require.NotNil(t, inv.ExpiresAt)
		assert.WithinDuration(t, exp, *inv.ExpiresAt, time.Second)
	})
}

func TestRegistrationService_ListInvites(t *testing.T) {
	t.Parallel()

	t.Run("returns invites", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewRegistrationService(fake, nil, nil, "https://example.com", nil)

		_, err := svc.CreateInvite(ctx, "user-1", nil, nil)
		require.NoError(t, err)

		invites, err := svc.ListInvites(ctx, "user-1")
		require.NoError(t, err)
		_ = invites
	})
}

func TestRegistrationService_RevokeInvite(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewRegistrationService(fake, nil, nil, "https://example.com", nil)

		inv, err := svc.CreateInvite(ctx, "user-1", nil, nil)
		require.NoError(t, err)

		err = svc.RevokeInvite(ctx, inv.ID)
		require.NoError(t, err)
	})
}

func seedModerator(t *testing.T, fake *testutil.FakeStore) *domain.Account {
	t.Helper()
	ctx := context.Background()
	accountSvc := NewAccountService(fake, "https://example.com")
	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username: "moderator",
		Email:    "mod@example.com",
		Password: "hash",
		Role:     domain.RoleModerator,
	})
	require.NoError(t, err)
	return acc
}
