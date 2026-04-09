package mastodon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeMediaService is a minimal MediaService for testing avatar/header upload.
type fakeMediaService struct {
	attachment *domain.MediaAttachment
	err        error
}

func (f *fakeMediaService) upload(accountID string) (*service.UploadResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	att := *f.attachment
	att.AccountID = accountID
	return &service.UploadResult{Attachment: &att}, nil
}

func (f *fakeMediaService) Upload(_ context.Context, accountID string, _ io.Reader, _ string, _ *string) (*service.UploadResult, error) {
	return f.upload(accountID)
}

func (f *fakeMediaService) UploadAvatar(_ context.Context, accountID string, _ io.Reader, _ string) (*service.UploadResult, error) {
	return f.upload(accountID)
}

func (f *fakeMediaService) UploadHeader(_ context.Context, accountID string, _ io.Reader, _ string) (*service.UploadResult, error) {
	return f.upload(accountID)
}

func (f *fakeMediaService) Update(_ context.Context, _, _ string, _ *string, _, _ *float64) (*domain.MediaAttachment, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeMediaService) CreateRemote(_ context.Context, _ service.CreateRemoteMediaInput) (*domain.MediaAttachment, error) {
	return nil, errors.New("not implemented")
}

func newTestFollowServices(st *testutil.FakeStore) (service.FollowService, service.TagFollowService) {
	accountSvc := service.NewAccountService(st, "https://example.com")
	remoteFollowSvc := service.NewRemoteFollowService(st)
	followSvc := service.NewFollowService(st, accountSvc, remoteFollowSvc, nil)
	tagFollowSvc := service.NewTagFollowService(st)
	return followSvc, tagFollowSvc
}

func TestAccountsHandler_VerifyCredentials(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc, tagFollowSvc := newTestFollowServices(st)
	handler := NewAccountsHandler(accountSvc, followSvc, tagFollowSvc, nil, nil, nil, nil, nil, nil, 0, "example.com")

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/verify_credentials", nil)
		rec := httptest.NewRecorder()
		handler.GETVerifyCredentials(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("authenticated with valid account returns 200 and account", func(t *testing.T) {
		acc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username: "alice",
			Email:    "alice@example.com",
			Password: "hash",
			Role:     domain.RoleUser,
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/verify_credentials", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.GETVerifyCredentials(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, "alice", body["username"])
		assert.Equal(t, "alice", body["acct"])
	})

	t.Run("account in context but not in store returns 401", func(t *testing.T) {
		orphan := &domain.Account{ID: "01nonexistent", Username: "orphan"}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/verify_credentials", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), orphan))
		rec := httptest.NewRecorder()
		handler.GETVerifyCredentials(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestAccountsHandler_GETAccountsLookup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc, tagFollowSvc := newTestFollowServices(st)
	handler := NewAccountsHandler(accountSvc, followSvc, tagFollowSvc, nil, nil, nil, nil, nil, nil, 0, "example.com")

	t.Run("missing acct returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/lookup", nil)
		rec := httptest.NewRecorder()
		handler.GETAccountsLookup(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("unknown account returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/lookup?acct=nobody", nil)
		rec := httptest.NewRecorder()
		handler.GETAccountsLookup(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("local account by username returns 200 and account", func(t *testing.T) {
		acc, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "alice"})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/lookup?acct=alice", nil)
		rec := httptest.NewRecorder()
		handler.GETAccountsLookup(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, acc.ID, body["id"])
		assert.Equal(t, "alice", body["username"])
		assert.Equal(t, "alice", body["acct"])
	})

	t.Run("remote account by acct returns 200 and account", func(t *testing.T) {
		remoteDomain := "other.example"
		_, err := st.CreateAccount(ctx, store.CreateAccountInput{
			ID:       "01REMOTE001",
			Username: "bob",
			Domain:   &remoteDomain,
			APID:     "https://other.example/users/bob",
		})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/lookup?acct=bob@other.example", nil)
		rec := httptest.NewRecorder()
		handler.GETAccountsLookup(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, "01REMOTE001", body["id"])
		assert.Equal(t, "bob", body["username"])
		assert.Equal(t, "bob@other.example", body["acct"])
	})
}

func TestAccountsHandler_GETAccounts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc, tagFollowSvc := newTestFollowServices(st)
	handler := NewAccountsHandler(accountSvc, followSvc, tagFollowSvc, nil, nil, nil, nil, nil, nil, 0, "example.com")

	t.Run("missing id returns 422", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/", nil)
		req = testutil.AddChiURLParam(req, "id", "")
		rec := httptest.NewRecorder()
		handler.GETAccounts(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("unknown account returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/01nonexistent", nil)
		req = testutil.AddChiURLParam(req, "id", "01nonexistent")
		rec := httptest.NewRecorder()
		handler.GETAccounts(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("valid local account returns 200 and account", func(t *testing.T) {
		acc, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "alice"})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+acc.ID, nil)
		req = testutil.AddChiURLParam(req, "id", acc.ID)
		rec := httptest.NewRecorder()
		handler.GETAccounts(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, acc.ID, body["id"])
		assert.Equal(t, "alice", body["username"])
		assert.Equal(t, "alice", body["acct"])
	})

	t.Run("valid remote account returns 200 and account", func(t *testing.T) {
		remoteDomain := "other.example"
		acc, err := st.CreateAccount(ctx, store.CreateAccountInput{
			ID: "01REMOTE002", Username: "bob", Domain: &remoteDomain, APID: "https://other.example/users/bob",
		})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+acc.ID, nil)
		req = testutil.AddChiURLParam(req, "id", acc.ID)
		rec := httptest.NewRecorder()
		handler.GETAccounts(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, acc.ID, body["id"])
		assert.Equal(t, "bob", body["username"])
		assert.Equal(t, "bob@other.example", body["acct"])
	})
}

func TestAccountsHandler_GETRelationships(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc, tagFollowSvc := newTestFollowServices(st)
	handler := NewAccountsHandler(accountSvc, followSvc, tagFollowSvc, nil, nil, nil, nil, nil, nil, 0, "example.com")

	alice, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	bob, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/relationships?id[]="+bob.ID, nil)
		rec := httptest.NewRecorder()
		handler.GETRelationships(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("empty id array returns 200 and empty slice", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/relationships", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), alice))
		rec := httptest.NewRecorder()
		handler.GETRelationships(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})

	t.Run("authenticated with one id returns 200 and relationship", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/relationships?id[]="+bob.ID, nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), alice))
		rec := httptest.NewRecorder()
		handler.GETRelationships(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		require.Len(t, body, 1)
		assert.Equal(t, bob.ID, body[0]["id"])
		assert.False(t, body[0]["following"].(bool))
	})

	t.Run("unknown target id returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/relationships?id[]=01nonexistent", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), alice))
		rec := httptest.NewRecorder()
		handler.GETRelationships(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestAccountsHandler_GETAccountStatuses(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc, tagFollowSvc := newTestFollowServices(st)
	statusSvc := service.NewStatusService(st, "https://example.com", "example.com", 500)
	timelineSvc := service.NewTimelineService(st, accountSvc, statusSvc, nil)
	handler := NewAccountsHandler(accountSvc, followSvc, tagFollowSvc, timelineSvc, statusSvc, nil, nil, nil, nil, 0, "example.com")

	t.Run("unauthenticated returns 200", func(t *testing.T) {
		acc, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "alice"})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+acc.ID+"/statuses", nil)
		req = testutil.AddChiURLParam(req, "id", acc.ID)
		rec := httptest.NewRecorder()
		handler.GETAccountStatuses(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("timeline nil returns 422", func(t *testing.T) {
		handlerNoTimeline := NewAccountsHandler(accountSvc, followSvc, tagFollowSvc, nil, nil, nil, nil, nil, nil, 0, "example.com")
		acc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username: "bob",
			Email:    "bob@example.com",
			Password: "hash",
			Role:     domain.RoleUser,
		})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+acc.ID+"/statuses", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", acc.ID)
		rec := httptest.NewRecorder()
		handlerNoTimeline.GETAccountStatuses(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("authenticated returns 200 and empty or status list", func(t *testing.T) {
		acc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username: "charlie",
			Email:    "charlie@example.com",
			Password: "hash",
			Role:     domain.RoleUser,
		})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+acc.ID+"/statuses", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", acc.ID)
		rec := httptest.NewRecorder()
		handler.GETAccountStatuses(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.NotNil(t, body)
	})
}

func TestAccountsHandler_GETFamiliarFollowers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc, tagFollowSvc := newTestFollowServices(st)
	handler := NewAccountsHandler(accountSvc, followSvc, tagFollowSvc, nil, nil, nil, nil, nil, nil, 0, "example.com")

	alice, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "alice-ff",
		Email:    "alice-ff@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	bob, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "bob-ff"})
	require.NoError(t, err)
	// carol is someone alice follows AND who follows bob
	carol, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "carol-ff"})
	require.NoError(t, err)

	// alice follows carol
	_, err = followSvc.Follow(ctx, alice.ID, carol.ID)
	require.NoError(t, err)
	// carol follows bob
	_, err = followSvc.Follow(ctx, carol.ID, bob.ID)
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/familiar_followers?id[]="+bob.ID, nil)
		rec := httptest.NewRecorder()
		handler.GETFamiliarFollowers(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("empty id array returns 200 and empty slice", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/familiar_followers", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), alice))
		rec := httptest.NewRecorder()
		handler.GETFamiliarFollowers(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})

	t.Run("returns familiar followers for requested id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/familiar_followers?id[]="+bob.ID, nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), alice))
		rec := httptest.NewRecorder()
		handler.GETFamiliarFollowers(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		require.Len(t, body, 1)
		assert.Equal(t, bob.ID, body[0]["id"])
		accounts := body[0]["accounts"].([]any)
		require.Len(t, accounts, 1)
		assert.Equal(t, carol.ID, accounts[0].(map[string]any)["id"])
	})

	t.Run("no familiar followers returns entry with empty accounts", func(t *testing.T) {
		// alice has no familiar followers with herself (no mutual follows through a third party)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/familiar_followers?id[]="+alice.ID, nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), alice))
		rec := httptest.NewRecorder()
		handler.GETFamiliarFollowers(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		require.Len(t, body, 1)
		assert.Equal(t, alice.ID, body[0]["id"])
		assert.Empty(t, body[0]["accounts"])
	})
}

func TestAccountsHandler_GETFollowers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc, tagFollowSvc := newTestFollowServices(st)
	handler := NewAccountsHandler(accountSvc, followSvc, tagFollowSvc, nil, nil, nil, nil, nil, nil, 0, "example.com")

	alice, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "alice-followers",
		Email:    "alice-followers@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+alice.ID+"/followers", nil)
		req = testutil.AddChiURLParam(req, "id", alice.ID)
		rec := httptest.NewRecorder()
		handler.GETFollowers(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("target not found returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/nonexistent-id/followers", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), alice))
		req = testutil.AddChiURLParam(req, "id", "nonexistent-id")
		rec := httptest.NewRecorder()
		handler.GETFollowers(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("authenticated returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+alice.ID+"/followers", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), alice))
		req = testutil.AddChiURLParam(req, "id", alice.ID)
		rec := httptest.NewRecorder()
		handler.GETFollowers(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})

	t.Run("locked account own followers visible to self", func(t *testing.T) {
		lockedUser, err := accountSvc.Register(ctx, service.RegisterInput{
			Username: "locked-user-followers",
			Email:    "locked-user-followers@example.com",
			Password: "hash",
			Role:     domain.RoleUser,
		})
		require.NoError(t, err)
		_, _, err = accountSvc.UpdateCredentials(ctx, service.UpdateCredentialsInput{AccountID: lockedUser.ID, Locked: true})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+lockedUser.ID+"/followers", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), lockedUser))
		req = testutil.AddChiURLParam(req, "id", lockedUser.ID)
		rec := httptest.NewRecorder()
		handler.GETFollowers(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("locked account followers hidden from non-followers", func(t *testing.T) {
		lockedUser, err := accountSvc.Register(ctx, service.RegisterInput{
			Username: "locked-hidden-followers",
			Email:    "locked-hidden-followers@example.com",
			Password: "hash",
			Role:     domain.RoleUser,
		})
		require.NoError(t, err)
		_, _, err = accountSvc.UpdateCredentials(ctx, service.UpdateCredentialsInput{AccountID: lockedUser.ID, Locked: true})
		require.NoError(t, err)
		viewer, err := accountSvc.Register(ctx, service.RegisterInput{
			Username: "viewer-followers",
			Email:    "viewer-followers@example.com",
			Password: "hash",
			Role:     domain.RoleUser,
		})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+lockedUser.ID+"/followers", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), viewer))
		req = testutil.AddChiURLParam(req, "id", lockedUser.ID)
		rec := httptest.NewRecorder()
		handler.GETFollowers(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})

	t.Run("locked account followers hidden from unauthenticated viewer", func(t *testing.T) {
		lockedUser, err := accountSvc.Register(ctx, service.RegisterInput{
			Username: "locked-unauth-followers",
			Email:    "locked-unauth-followers@example.com",
			Password: "hash",
			Role:     domain.RoleUser,
		})
		require.NoError(t, err)
		_, _, err = accountSvc.UpdateCredentials(ctx, service.UpdateCredentialsInput{AccountID: lockedUser.ID, Locked: true})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+lockedUser.ID+"/followers", nil)
		req = testutil.AddChiURLParam(req, "id", lockedUser.ID)
		rec := httptest.NewRecorder()
		handler.GETFollowers(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})
}

func TestAccountsHandler_GETFollowing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc, tagFollowSvc := newTestFollowServices(st)
	handler := NewAccountsHandler(accountSvc, followSvc, tagFollowSvc, nil, nil, nil, nil, nil, nil, 0, "example.com")

	alice, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "alice-following",
		Email:    "alice-following@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+alice.ID+"/following", nil)
		req = testutil.AddChiURLParam(req, "id", alice.ID)
		rec := httptest.NewRecorder()
		handler.GETFollowing(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("authenticated returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+alice.ID+"/following", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), alice))
		req = testutil.AddChiURLParam(req, "id", alice.ID)
		rec := httptest.NewRecorder()
		handler.GETFollowing(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})

	t.Run("locked account following hidden from non-followers", func(t *testing.T) {
		lockedUser, err := accountSvc.Register(ctx, service.RegisterInput{
			Username: "locked-hidden-following",
			Email:    "locked-hidden-following@example.com",
			Password: "hash",
			Role:     domain.RoleUser,
		})
		require.NoError(t, err)
		_, _, err = accountSvc.UpdateCredentials(ctx, service.UpdateCredentialsInput{AccountID: lockedUser.ID, Locked: true})
		require.NoError(t, err)
		viewer, err := accountSvc.Register(ctx, service.RegisterInput{
			Username: "viewer-following",
			Email:    "viewer-following@example.com",
			Password: "hash",
			Role:     domain.RoleUser,
		})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+lockedUser.ID+"/following", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), viewer))
		req = testutil.AddChiURLParam(req, "id", lockedUser.ID)
		rec := httptest.NewRecorder()
		handler.GETFollowing(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})

	t.Run("locked account following hidden from unauthenticated viewer", func(t *testing.T) {
		lockedUser, err := accountSvc.Register(ctx, service.RegisterInput{
			Username: "locked-unauth-following",
			Email:    "locked-unauth-following@example.com",
			Password: "hash",
			Role:     domain.RoleUser,
		})
		require.NoError(t, err)
		_, _, err = accountSvc.UpdateCredentials(ctx, service.UpdateCredentialsInput{AccountID: lockedUser.ID, Locked: true})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+lockedUser.ID+"/following", nil)
		req = testutil.AddChiURLParam(req, "id", lockedUser.ID)
		rec := httptest.NewRecorder()
		handler.GETFollowing(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})
}

func TestAccountsHandler_GETBlocks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc, tagFollowSvc := newTestFollowServices(st)
	handler := NewAccountsHandler(accountSvc, followSvc, tagFollowSvc, nil, nil, nil, nil, nil, nil, 0, "example.com")

	actor, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	target, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "bob",
		Email:    "bob@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/blocks", nil)
		rec := httptest.NewRecorder()
		handler.GETBlocks(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("authenticated empty returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/blocks", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		rec := httptest.NewRecorder()
		handler.GETBlocks(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})

	t.Run("authenticated with blocks returns 200 and account list", func(t *testing.T) {
		_, err := followSvc.Block(ctx, actor.ID, target.ID)
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/blocks", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		rec := httptest.NewRecorder()
		handler.GETBlocks(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		require.Len(t, body, 1)
		assert.Equal(t, target.ID, body[0]["id"])
		assert.Equal(t, "bob", body[0]["username"])
	})

	t.Run("authenticated with multiple blocks returns Link pagination and second page", func(t *testing.T) {
		target2, err := accountSvc.Register(ctx, service.RegisterInput{
			Username: "carol",
			Email:    "carol@example.com",
			Password: "hash",
			Role:     domain.RoleUser,
		})
		require.NoError(t, err)
		// actor already blocked target in a previous subtest; add second block only
		_, err = followSvc.Block(ctx, actor.ID, target2.ID)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/blocks?limit=1", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		rec := httptest.NewRecorder()
		handler.GETBlocks(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		require.Len(t, body, 1)
		link := rec.Header().Get("Link")
		require.NotEmpty(t, link)
		assert.Contains(t, link, `rel="next"`)
		nextURL := parseLinkNextURL(t, link)
		require.NotEmpty(t, nextURL)
		maxID := parseQueryParam(t, nextURL, "max_id")
		require.NotEmpty(t, maxID)

		req2 := httptest.NewRequest(http.MethodGet, "/api/v1/blocks?limit=1&max_id="+maxID, nil)
		req2 = req2.WithContext(middleware.WithAccount(req2.Context(), actor))
		rec2 := httptest.NewRecorder()
		handler.GETBlocks(rec2, req2)
		assert.Equal(t, http.StatusOK, rec2.Code)
		var body2 []map[string]any
		require.NoError(t, json.NewDecoder(rec2.Body).Decode(&body2))
		require.Len(t, body2, 1)
		assert.NotEqual(t, body[0]["id"], body2[0]["id"])
	})
}

func TestAccountsHandler_GETMutes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc, tagFollowSvc := newTestFollowServices(st)
	handler := NewAccountsHandler(accountSvc, followSvc, tagFollowSvc, nil, nil, nil, nil, nil, nil, 0, "example.com")

	actor, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	target, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "bob",
		Email:    "bob@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/mutes", nil)
		rec := httptest.NewRecorder()
		handler.GETMutes(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("authenticated empty returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/mutes", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		rec := httptest.NewRecorder()
		handler.GETMutes(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})

	t.Run("authenticated with mutes returns 200 and account list", func(t *testing.T) {
		_, err := followSvc.Mute(ctx, actor.ID, target.ID, false)
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/mutes", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		rec := httptest.NewRecorder()
		handler.GETMutes(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		require.Len(t, body, 1)
		assert.Equal(t, target.ID, body[0]["id"])
		assert.Equal(t, "bob", body[0]["username"])
	})

	t.Run("authenticated with multiple mutes returns Link pagination and second page", func(t *testing.T) {
		target2, err := accountSvc.Register(ctx, service.RegisterInput{
			Username: "carol",
			Email:    "carol@example.com",
			Password: "hash",
			Role:     domain.RoleUser,
		})
		require.NoError(t, err)
		// actor already muted target in a previous subtest; add second mute only
		_, err = followSvc.Mute(ctx, actor.ID, target2.ID, false)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/mutes?limit=1", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		rec := httptest.NewRecorder()
		handler.GETMutes(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		require.Len(t, body, 1)
		link := rec.Header().Get("Link")
		require.NotEmpty(t, link)
		assert.Contains(t, link, `rel="next"`)
		nextURL := parseLinkNextURL(t, link)
		require.NotEmpty(t, nextURL)
		maxID := parseQueryParam(t, nextURL, "max_id")
		require.NotEmpty(t, maxID)

		req2 := httptest.NewRequest(http.MethodGet, "/api/v1/mutes?limit=1&max_id="+maxID, nil)
		req2 = req2.WithContext(middleware.WithAccount(req2.Context(), actor))
		rec2 := httptest.NewRecorder()
		handler.GETMutes(rec2, req2)
		assert.Equal(t, http.StatusOK, rec2.Code)
		var body2 []map[string]any
		require.NoError(t, json.NewDecoder(rec2.Body).Decode(&body2))
		require.Len(t, body2, 1)
		assert.NotEqual(t, body[0]["id"], body2[0]["id"])
	})
}

func TestAccountsHandler_FollowedTags(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	_, tagFollowSvc := newTestFollowServices(st)
	handler := NewAccountsHandler(accountSvc, nil, tagFollowSvc, nil, nil, nil, nil, nil, nil, 0, "example.com")

	actor, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("GET followed_tags unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/followed_tags", nil)
		rec := httptest.NewRecorder()
		handler.GETFollowedTags(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("GET followed_tags authenticated empty returns 200 and empty array", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/followed_tags", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		rec := httptest.NewRecorder()
		handler.GETFollowedTags(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})

	t.Run("GET tag by name unauthenticated returns tag without following", func(t *testing.T) {
		_, err := tagFollowSvc.FollowTag(ctx, actor.ID, "golang")
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tags/golang", nil)
		req = testutil.AddChiURLParam(req, "name", "golang")
		rec := httptest.NewRecorder()
		handler.GETTag(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, "golang", body["name"])
		assert.Contains(t, body["url"], "/tags/golang")
		following, _ := body["following"].(bool)
		assert.False(t, following, "unauthenticated should see following: false or absent")
		assert.NotNil(t, body["history"], "history should always be present")
	})

	t.Run("GET tag by name authenticated returns following true", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tags/golang", nil)
		req = testutil.AddChiURLParam(req, "name", "golang")
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		rec := httptest.NewRecorder()
		handler.GETTag(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.True(t, body["following"].(bool))
	})

	t.Run("GET tag unknown returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tags/nonexistent", nil)
		req = testutil.AddChiURLParam(req, "name", "nonexistent")
		rec := httptest.NewRecorder()
		handler.GETTag(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("POST tag follow unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tags/rust/follow", nil)
		req = testutil.AddChiURLParam(req, "name", "rust")
		rec := httptest.NewRecorder()
		handler.POSTTagFollow(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("POST tag follow by name returns 200 with following true", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tags/rust/follow", nil)
		req = testutil.AddChiURLParam(req, "name", "rust")
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		rec := httptest.NewRecorder()
		handler.POSTTagFollow(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, "rust", body["name"])
		assert.True(t, body["following"].(bool))
	})

	t.Run("POST tag unfollow unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tags/rust/unfollow", nil)
		req = testutil.AddChiURLParam(req, "name", "rust")
		rec := httptest.NewRecorder()
		handler.POSTTagUnfollow(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("POST tag unfollow by name returns 200 with following false", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tags/rust/unfollow", nil)
		req = testutil.AddChiURLParam(req, "name", "rust")
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		rec := httptest.NewRecorder()
		handler.POSTTagUnfollow(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, "rust", body["name"])
		following, _ := body["following"].(bool)
		assert.False(t, following)
	})

	t.Run("GET followed_tags after follow and unfollow shows only golang", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/followed_tags", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		rec := httptest.NewRecorder()
		handler.GETFollowedTags(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		names := make([]string, 0, len(body))
		for _, tag := range body {
			names = append(names, tag["name"].(string))
		}
		assert.Contains(t, names, "golang")
		assert.NotContains(t, names, "rust", "rust should be removed after unfollow")
	})
}

func TestAccountsHandler_BlockUnblock(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc, tagFollowSvc := newTestFollowServices(st)
	handler := NewAccountsHandler(accountSvc, followSvc, tagFollowSvc, nil, nil, nil, nil, nil, nil, 0, "example.com")

	actor, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	target, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "bob",
		Email:    "bob@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated POST block returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/"+target.ID+"/block", nil)
		req = testutil.AddChiURLParam(req, "id", target.ID)
		rec := httptest.NewRecorder()
		handler.POSTBlock(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("POST block returns 200 and relationship", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/"+target.ID+"/block", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		req = testutil.AddChiURLParam(req, "id", target.ID)
		rec := httptest.NewRecorder()
		handler.POSTBlock(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.True(t, body["blocking"].(bool))
	})

	t.Run("POST unblock returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/"+target.ID+"/unblock", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		req = testutil.AddChiURLParam(req, "id", target.ID)
		rec := httptest.NewRecorder()
		handler.POSTUnblock(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.False(t, body["blocking"].(bool))
	})
}

func TestAccountsHandler_MuteUnmute(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc, tagFollowSvc := newTestFollowServices(st)
	handler := NewAccountsHandler(accountSvc, followSvc, tagFollowSvc, nil, nil, nil, nil, nil, nil, 0, "example.com")

	actor, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	target, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "bob",
		Email:    "bob@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated POST mute returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/"+target.ID+"/mute", nil)
		req = testutil.AddChiURLParam(req, "id", target.ID)
		rec := httptest.NewRecorder()
		handler.POSTMute(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("POST mute returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/"+target.ID+"/mute", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		req = testutil.AddChiURLParam(req, "id", target.ID)
		rec := httptest.NewRecorder()
		handler.POSTMute(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.True(t, body["muting"].(bool))
	})

	t.Run("POST unmute returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/"+target.ID+"/unmute", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		req = testutil.AddChiURLParam(req, "id", target.ID)
		rec := httptest.NewRecorder()
		handler.POSTUnmute(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestAccountsHandler_PATCHUpdateCredentials(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc, tagFollowSvc := newTestFollowServices(st)
	handler := NewAccountsHandler(accountSvc, followSvc, tagFollowSvc, nil, nil, nil, nil, nil, nil, 0, "example.com")

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/accounts/update_credentials", nil)
		rec := httptest.NewRecorder()
		handler.PATCHUpdateCredentials(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("authenticated with display_name returns 200", func(t *testing.T) {
		acc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username: "alice",
			Email:    "alice@example.com",
			Password: "hash",
			Role:     domain.RoleUser,
		})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/accounts/update_credentials", bytes.NewBufferString("display_name=Alice+Updated"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.PATCHUpdateCredentials(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, "Alice Updated", body["display_name"])
	})

	t.Run("avatar file upload sets avatar_media_id", func(t *testing.T) {
		t.Parallel()
		localSt := testutil.NewFakeStore()
		localAccountSvc := service.NewAccountService(localSt, "https://example.com")
		acc, err := localAccountSvc.Register(ctx, service.RegisterInput{
			Username: "bob",
			Email:    "bob@example.com",
			Password: "hash",
			Role:     domain.RoleUser,
		})
		require.NoError(t, err)

		uploadedAttachment := &domain.MediaAttachment{ID: "media-123", Type: "image", URL: "https://example.com/media/avatar.jpg"}
		mediaSvc := &fakeMediaService{attachment: uploadedAttachment}
		localHandler := NewAccountsHandler(localAccountSvc, nil, nil, nil, nil, nil, mediaSvc, nil, nil, 10<<20, "example.com")

		imgBytes := testutil.MinimalPNG(t)
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, err := mw.CreateFormFile("avatar", "avatar.png")
		require.NoError(t, err)
		_, err = fw.Write(imgBytes)
		require.NoError(t, err)
		require.NoError(t, mw.Close())

		req := httptest.NewRequest(http.MethodPatch, "/api/v1/accounts/update_credentials", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		localHandler.PATCHUpdateCredentials(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, "https://example.com/media/avatar.jpg", body["avatar"])
		assert.Equal(t, "https://example.com/media/avatar.jpg", body["avatar_static"])
	})
}

func TestAccountsHandler_GETDirectory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc, tagFollowSvc := newTestFollowServices(st)
	handler := NewAccountsHandler(accountSvc, followSvc, tagFollowSvc, nil, nil, nil, nil, nil, nil, 0, "example.com")

	t.Run("returns 200 with accounts and default order active", func(t *testing.T) {
		_, err := accountSvc.Register(ctx, service.RegisterInput{
			Username: "alice",
			Email:    "alice@example.com",
			Password: "hash",
			Role:     domain.RoleUser,
		})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/directory?limit=10", nil)
		rec := httptest.NewRecorder()
		handler.GETDirectory(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		require.GreaterOrEqual(t, len(body), 1)
		assert.Equal(t, "alice", body[0]["username"])
	})

	t.Run("order=new returns accounts by created_at desc", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/directory?order=new&limit=5", nil)
		rec := httptest.NewRecorder()
		handler.GETDirectory(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.NotEmpty(t, body)
	})

	t.Run("limit cap 80", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/directory?limit=200", nil)
		rec := httptest.NewRecorder()
		handler.GETDirectory(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.LessOrEqual(t, len(body), 80)
	})

	t.Run("local=true filters to local accounts only", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/directory?local=true&limit=10", nil)
		rec := httptest.NewRecorder()
		handler.GETDirectory(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		for _, acct := range body {
			acctVal, ok := acct["acct"].(string)
			require.True(t, ok)
			assert.NotContains(t, acctVal, "@", "local=true should not return remote acct")
		}
	})
}

// parseLinkNextURL extracts the URL from a Link header segment containing rel="next".
func parseLinkNextURL(t *testing.T, linkHeader string) string {
	t.Helper()
	for _, part := range strings.Split(linkHeader, ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, `rel="next"`) {
			continue
		}
		start := strings.Index(part, "<")
		end := strings.Index(part, ">")
		if start >= 0 && end > start {
			return part[start+1 : end]
		}
	}
	return ""
}

// parseQueryParam returns the value of the given query parameter in urlStr.
func parseQueryParam(t *testing.T, urlStr, name string) string {
	t.Helper()
	u, err := url.Parse(urlStr)
	require.NoError(t, err)
	return u.Query().Get(name)
}

// fakeSettingsService is a minimal MonsteraSettingsService for testing.
type fakeSettingsService struct {
	mode domain.MonsteraRegistrationMode
}

func (f *fakeSettingsService) Get(_ context.Context) (domain.MonsteraSettings, error) {
	return domain.MonsteraSettings{RegistrationMode: f.mode}, nil
}

func (f *fakeSettingsService) Update(_ context.Context, s domain.MonsteraSettings) error {
	f.mode = s.RegistrationMode
	return nil
}

func TestAccountsHandler_POSTAccounts(t *testing.T) {
	t.Parallel()

	validBody := `{"username":"newuser","email":"new@example.com","password":"password123","agreement":true}`

	newHandler := func(mode domain.MonsteraRegistrationMode) *AccountsHandler {
		st := testutil.NewFakeStore()
		accountSvc := service.NewAccountService(st, "https://example.com")
		settingsSvc := &fakeSettingsService{mode: mode}
		return NewAccountsHandler(accountSvc, nil, nil, nil, nil, settingsSvc, nil, nil, nil, 0, "example.com")
	}

	t.Run("open mode returns 200 with account and pending false", func(t *testing.T) {
		t.Parallel()
		handler := newHandler(domain.MonsteraRegistrationModeOpen)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", strings.NewReader(validBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTAccounts(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, "newuser", body["account"].(map[string]any)["username"])
		assert.False(t, body["pending"].(bool))
	})

	t.Run("approval mode returns 200 with account and pending true", func(t *testing.T) {
		t.Parallel()
		handler := newHandler(domain.MonsteraRegistrationModeApproval)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", strings.NewReader(validBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTAccounts(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, "newuser", body["account"].(map[string]any)["username"])
		assert.True(t, body["pending"].(bool))
	})

	t.Run("closed mode returns 403", func(t *testing.T) {
		t.Parallel()
		handler := newHandler(domain.MonsteraRegistrationModeClosed)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", strings.NewReader(validBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTAccounts(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("missing username returns 422", func(t *testing.T) {
		t.Parallel()
		handler := newHandler(domain.MonsteraRegistrationModeOpen)
		body := `{"email":"new@example.com","password":"password123","agreement":true}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTAccounts(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("missing email returns 422", func(t *testing.T) {
		t.Parallel()
		handler := newHandler(domain.MonsteraRegistrationModeOpen)
		body := `{"username":"newuser","password":"password123","agreement":true}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTAccounts(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("missing password returns 422", func(t *testing.T) {
		t.Parallel()
		handler := newHandler(domain.MonsteraRegistrationModeOpen)
		body := `{"username":"newuser","email":"new@example.com","agreement":true}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTAccounts(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("agreement false returns 422", func(t *testing.T) {
		t.Parallel()
		handler := newHandler(domain.MonsteraRegistrationModeOpen)
		body := `{"username":"newuser","email":"new@example.com","password":"password123","agreement":false}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTAccounts(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("duplicate username returns 409", func(t *testing.T) {
		// Shared store to test conflict
		st := testutil.NewFakeStore()
		accountSvc := service.NewAccountService(st, "https://example.com")
		settingsSvc := &fakeSettingsService{mode: domain.MonsteraRegistrationModeOpen}
		handler := NewAccountsHandler(accountSvc, nil, nil, nil, nil, settingsSvc, nil, nil, nil, 0, "example.com")

		ctx := context.Background()
		_, err := accountSvc.Register(ctx, service.RegisterInput{
			Username: "taken",
			Email:    "taken@example.com",
			Password: "hash",
			Role:     domain.RoleUser,
		})
		require.NoError(t, err)

		body := `{"username":"taken","email":"other@example.com","password":"password123","agreement":true}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTAccounts(rec, req)
		assert.Equal(t, http.StatusConflict, rec.Code)
	})

	t.Run("invite mode missing invite code returns 400", func(t *testing.T) {
		t.Parallel()
		// Both the handler's fakeSettingsService and the store must reflect invite mode
		// so that AccountService.Register also enforces the invite code requirement.
		ctx := context.Background()
		st := testutil.NewFakeStore()
		require.NoError(t, st.UpdateMonsteraSettings(ctx, &domain.MonsteraSettings{RegistrationMode: domain.MonsteraRegistrationModeInvite}))
		accountSvc := service.NewAccountService(st, "https://example.com")
		settingsSvc := &fakeSettingsService{mode: domain.MonsteraRegistrationModeInvite}
		handler := NewAccountsHandler(accountSvc, nil, nil, nil, nil, settingsSvc, nil, nil, nil, 0, "example.com")
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", strings.NewReader(validBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTAccounts(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestAccountsHandler_DomainBlocks(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc, tagFollowSvc := newTestFollowServices(st)
	handler := NewAccountsHandler(accountSvc, followSvc, tagFollowSvc, nil, nil, nil, nil, nil, nil, 0, "example.com")

	actor, err := accountSvc.Register(context.Background(), service.RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("GET unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/domain_blocks", nil)
		rec := httptest.NewRecorder()
		handler.GETDomainBlocks(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("GET empty returns 200 and empty array", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/domain_blocks", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		rec := httptest.NewRecorder()
		handler.GETDomainBlocks(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []string
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})

	t.Run("POST missing domain returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/domain_blocks", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		rec := httptest.NewRecorder()
		handler.POSTDomainBlock(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("POST creates domain block and GET returns it", func(t *testing.T) {
		form := strings.NewReader("domain=evil.example")
		req := httptest.NewRequest(http.MethodPost, "/api/v1/domain_blocks", form)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		rec := httptest.NewRecorder()
		handler.POSTDomainBlock(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		req2 := httptest.NewRequest(http.MethodGet, "/api/v1/domain_blocks", nil)
		req2 = req2.WithContext(middleware.WithAccount(req2.Context(), actor))
		rec2 := httptest.NewRecorder()
		handler.GETDomainBlocks(rec2, req2)
		assert.Equal(t, http.StatusOK, rec2.Code)
		var domains []string
		require.NoError(t, json.NewDecoder(rec2.Body).Decode(&domains))
		assert.Contains(t, domains, "evil.example")
	})

	t.Run("DELETE removes domain block", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/domain_blocks?domain=evil.example", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		rec := httptest.NewRecorder()
		handler.DELETEDomainBlock(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		req2 := httptest.NewRequest(http.MethodGet, "/api/v1/domain_blocks", nil)
		req2 = req2.WithContext(middleware.WithAccount(req2.Context(), actor))
		rec2 := httptest.NewRecorder()
		handler.GETDomainBlocks(rec2, req2)
		assert.Equal(t, http.StatusOK, rec2.Code)
		var domains []string
		require.NoError(t, json.NewDecoder(rec2.Body).Decode(&domains))
		assert.NotContains(t, domains, "evil.example")
	})

	t.Run("POST is idempotent", func(t *testing.T) {
		for range 2 {
			form := strings.NewReader("domain=spam.example")
			req := httptest.NewRequest(http.MethodPost, "/api/v1/domain_blocks", form)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req = req.WithContext(middleware.WithAccount(req.Context(), actor))
			rec := httptest.NewRecorder()
			handler.POSTDomainBlock(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code)
		}
	})
}
