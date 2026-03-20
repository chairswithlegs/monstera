package service

import (
	"context"
	"testing"
	"time"

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
	mediaSvc := NewMediaService(st, nil, 0)
	svc := NewRemoteStatusWriteService(st, convSvc, mediaSvc, "https://example.com")

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

func TestRemoteStatusWriteService_CreateRemote_PreservesPublishedAt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := NewAccountService(st, "https://example.com")
	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username: "remote2",
		Email:    "r2@example.com",
		Password: "p",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	statusSvc := NewStatusService(st, "https://example.com", "example.com", 5000)
	convSvc := NewConversationService(st, statusSvc)
	mediaSvc := NewMediaService(st, nil, 0)
	svc := NewRemoteStatusWriteService(st, convSvc, mediaSvc, "https://example.com")

	published := time.Date(2023, 6, 15, 12, 0, 0, 0, time.UTC)
	in := CreateRemoteStatusInput{
		AccountID:   acc.ID,
		URI:         "https://remote.example/statuses/old",
		Content:     testutil.StrPtr("old post"),
		Visibility:  domain.VisibilityPublic,
		APID:        "https://remote.example/statuses/old",
		PublishedAt: &published,
	}
	created, err := svc.CreateRemote(ctx, in)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, published, created.CreatedAt, "CreatedAt should match the AP Note Published timestamp")
}

func TestRemoteStatusWriteService_DeleteRemote_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	statusSvc := NewStatusService(st, "https://example.com", "example.com", 5000)
	convSvc := NewConversationService(st, statusSvc)
	mediaSvc := NewMediaService(st, nil, 0)
	svc := NewRemoteStatusWriteService(st, convSvc, mediaSvc, "https://example.com")

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
	mediaSvc := NewMediaService(st, nil, 0)
	svc := NewRemoteStatusWriteService(st, convSvc, mediaSvc, "https://example.com")

	err = svc.DeleteRemote(ctx, localStatusID)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestRemoteStatusWriteService_CreateRemote_DuplicateAPID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := NewAccountService(st, "https://example.com")
	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username: "remote3",
		Email:    "r3@example.com",
		Password: "p",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	statusSvc := NewStatusService(st, "https://example.com", "example.com", 5000)
	convSvc := NewConversationService(st, statusSvc)
	mediaSvc := NewMediaService(st, nil, 0)
	svc := NewRemoteStatusWriteService(st, convSvc, mediaSvc, "https://example.com")

	in := CreateRemoteStatusInput{
		AccountID:  acc.ID,
		URI:        "https://remote.example/statuses/dup",
		Content:    testutil.StrPtr("hello"),
		Visibility: domain.VisibilityPublic,
		APID:       "https://remote.example/statuses/dup",
	}
	_, err = svc.CreateRemote(ctx, in)
	require.NoError(t, err)

	// Second call with the same ap_id should return ErrConflict.
	_, err = svc.CreateRemote(ctx, in)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrConflict)
}

func TestRemoteStatusWriteService_CreateRemote_NoMediaOnConflict(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := NewAccountService(st, "https://example.com")
	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username: "remote4",
		Email:    "r4@example.com",
		Password: "p",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	statusSvc := NewStatusService(st, "https://example.com", "example.com", 5000)
	convSvc := NewConversationService(st, statusSvc)
	mediaSvc := NewMediaService(st, nil, 0)
	svc := NewRemoteStatusWriteService(st, convSvc, mediaSvc, "https://example.com")

	attachment := CreateRemoteMediaInput{
		AccountID: acc.ID,
		RemoteURL: "https://remote.example/image.jpg",
		MediaType: "image/jpeg",
	}
	in := CreateRemoteStatusInput{
		AccountID:   acc.ID,
		URI:         "https://remote.example/statuses/media",
		Content:     testutil.StrPtr("post with media"),
		Visibility:  domain.VisibilityPublic,
		APID:        "https://remote.example/statuses/media",
		Attachments: []CreateRemoteMediaInput{attachment},
	}

	// First call: status and media are created.
	created, err := svc.CreateRemote(ctx, in)
	require.NoError(t, err)
	atts, err := st.GetStatusAttachments(ctx, created.ID)
	require.NoError(t, err)
	assert.Len(t, atts, 1, "media should be attached after successful create")

	// Second call: ErrConflict — no additional media records should be created.
	_, err = svc.CreateRemote(ctx, in)
	require.ErrorIs(t, err, domain.ErrConflict)
	atts2, _ := st.GetStatusAttachments(ctx, created.ID)
	assert.Len(t, atts2, 1, "no extra media should be created when CreateStatus conflicts")
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
	mediaSvc := NewMediaService(st, nil, 0)
	svc := NewRemoteStatusWriteService(st, convSvc, mediaSvc, "https://example.com")

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
