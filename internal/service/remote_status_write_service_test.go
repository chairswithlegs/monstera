package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
)

func TestRemoteStatusWriteService_CreateRemote_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := NewAccountService(st, "https://example.com")
	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username: "remote",
		Email:    "r@example.com",
		Password: "p",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	require.NotNil(t, acc)

	statusSvc := NewStatusService(st, "https://example.com", "example.com", 5000)
	convSvc := NewConversationService(st, statusSvc)
	svc := NewRemoteStatusWriteService(st, convSvc, "https://example.com")

	in := CreateRemoteStatusInput{
		AccountID:  acc.ID,
		URI:        "https://remote.example/statuses/1",
		Content:    testutil.StrPtr("hello"),
		Visibility: domain.VisibilityPublic,
		APID:       "https://remote.example/statuses/1",
	}
	st2, err := svc.CreateRemote(ctx, in)
	require.NoError(t, err)
	require.NotNil(t, st2)
	assert.Equal(t, acc.ID, st2.AccountID)
	assert.Equal(t, "https://remote.example/statuses/1", st2.URI)
	assert.Equal(t, "https://remote.example/statuses/1", st2.APID)
	assert.False(t, st2.Local)
}

func TestRemoteStatusWriteService_DeleteRemote_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	statusSvc := NewStatusService(st, "https://example.com", "example.com", 5000)
	convSvc := NewConversationService(st, statusSvc)
	svc := NewRemoteStatusWriteService(st, convSvc, "https://example.com")

	err := svc.DeleteRemote(ctx, "01nonexistent-status-id")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestRemoteStatusWriteService_DeleteRemote_ForbiddenLocal(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := NewAccountService(st, "https://example.com")
	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username: "local",
		Email:    "l@example.com",
		Password: "p",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	require.NotNil(t, acc)

	localStatusID := uid.New()
	_, err = st.CreateStatus(ctx, store.CreateStatusInput{
		ID:                  localStatusID,
		URI:                 "https://example.com/statuses/" + localStatusID,
		AccountID:           acc.ID,
		Content:             testutil.StrPtr("hello"),
		Visibility:          domain.VisibilityPublic,
		APID:                "https://example.com/statuses/" + localStatusID,
		Local:               true,
		QuoteApprovalPolicy: domain.QuotePolicyPublic,
	})
	require.NoError(t, err)

	statusSvc := NewStatusService(st, "https://example.com", "example.com", 5000)
	convSvc := NewConversationService(st, statusSvc)
	svc := NewRemoteStatusWriteService(st, convSvc, "https://example.com")

	err = svc.DeleteRemote(ctx, localStatusID)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestRemoteStatusWriteService_CreateRemoteFavourite_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := NewAccountService(st, "https://example.com")
	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username: "remote",
		Email:    "r@example.com",
		Password: "p",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	require.NotNil(t, acc)

	statusSvc := NewStatusService(st, "https://example.com", "example.com", 5000)
	convSvc := NewConversationService(st, statusSvc)
	svc := NewRemoteStatusWriteService(st, convSvc, "https://example.com")

	in := CreateRemoteStatusInput{
		AccountID:  acc.ID,
		URI:        "https://remote.example/statuses/1",
		Content:    testutil.StrPtr("hello"),
		Visibility: domain.VisibilityPublic,
		APID:       "https://remote.example/statuses/1",
	}
	st2, err := svc.CreateRemote(ctx, in)
	require.NoError(t, err)
	require.NotNil(t, st2)

	fav, err := svc.CreateRemoteFavourite(ctx, acc.ID, st2.ID, nil)
	require.NoError(t, err)
	require.NotNil(t, fav)
	assert.Equal(t, acc.ID, fav.AccountID)
	assert.Equal(t, st2.ID, fav.StatusID)
}
