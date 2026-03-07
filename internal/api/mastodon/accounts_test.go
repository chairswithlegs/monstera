package mastodon

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestAccountsHandler_VerifyCredentials(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc := service.NewFollowService(st, nil, nil)
	handler := NewAccountsHandler(accountSvc, followSvc, nil, "example.com")

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/verify_credentials", nil)
		rec := httptest.NewRecorder()
		handler.GETVerifyCredentials(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("authenticated with valid account returns 200 and account", func(t *testing.T) {
		acc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "alice",
			Email:        "alice@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
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
	followSvc := service.NewFollowService(st, nil, nil)
	handler := NewAccountsHandler(accountSvc, followSvc, nil, "example.com")

	t.Run("missing acct returns 422", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/lookup", nil)
		rec := httptest.NewRecorder()
		handler.GETAccountsLookup(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
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

func TestAccountsHandler_GETAccountStatuses(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc := service.NewFollowService(st, nil, nil)
	timelineSvc := service.NewTimelineService(st, &allowAllVisibilityChecker{})
	handler := NewAccountsHandler(accountSvc, followSvc, timelineSvc, "example.com")

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		acc, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "alice"})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+acc.ID+"/statuses", nil)
		req = testutil.AddChiURLParam(req, "id", acc.ID)
		rec := httptest.NewRecorder()
		handler.GETAccountStatuses(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("timeline nil returns 422", func(t *testing.T) {
		handlerNoTimeline := NewAccountsHandler(accountSvc, followSvc, nil, "example.com")
		acc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "bob",
			Email:        "bob@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
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
			Username:     "charlie",
			Email:        "charlie@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
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

func TestAccountsHandler_GETFollowers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc := service.NewFollowService(st, nil, nil)
	handler := NewAccountsHandler(accountSvc, followSvc, nil, "example.com")

	alice, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice-followers",
		Email:        "alice-followers@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+alice.ID+"/followers", nil)
		req = testutil.AddChiURLParam(req, "id", alice.ID)
		rec := httptest.NewRecorder()
		handler.GETFollowers(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
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
}

func TestAccountsHandler_GETFollowing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc := service.NewFollowService(st, nil, nil)
	handler := NewAccountsHandler(accountSvc, followSvc, nil, "example.com")

	alice, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice-following",
		Email:        "alice-following@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+alice.ID+"/following", nil)
		req = testutil.AddChiURLParam(req, "id", alice.ID)
		rec := httptest.NewRecorder()
		handler.GETFollowing(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
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
}

func TestAccountsHandler_GETBlocks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc := service.NewFollowService(st, nil, nil)
	handler := NewAccountsHandler(accountSvc, followSvc, nil, "example.com")

	actor, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	target, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "bob",
		Email:        "bob@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
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
			Username:     "carol",
			Email:        "carol@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
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
	followSvc := service.NewFollowService(st, nil, nil)
	handler := NewAccountsHandler(accountSvc, followSvc, nil, "example.com")

	actor, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	target, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "bob",
		Email:        "bob@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
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
			Username:     "carol",
			Email:        "carol@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
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
	followSvc := service.NewFollowService(st, nil, nil)
	handler := NewAccountsHandler(accountSvc, followSvc, nil, "example.com")

	actor, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("GET unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/followed_tags", nil)
		rec := httptest.NewRecorder()
		handler.GETFollowedTags(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("GET authenticated empty returns 200 and empty array", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/followed_tags", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		rec := httptest.NewRecorder()
		handler.GETFollowedTags(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})

	t.Run("POST unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/followed_tags", strings.NewReader(`{"name":"golang"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTFollowedTags(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("POST with name returns 200 and tag with following true", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/followed_tags", strings.NewReader(`{"name":"golang"}`))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		rec := httptest.NewRecorder()
		handler.POSTFollowedTags(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, "golang", body["name"])
		assert.True(t, body["following"].(bool))
		assert.NotEmpty(t, body["id"])
		assert.Contains(t, body["url"], "/tags/golang")
	})

	t.Run("GET after follow returns tag in list", func(t *testing.T) {
		_, err := followSvc.FollowTag(ctx, actor.ID, "rust")
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/followed_tags", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		rec := httptest.NewRecorder()
		handler.GETFollowedTags(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		var found bool
		for _, tag := range body {
			if tag["name"] == "rust" {
				found = true
				assert.True(t, tag["following"].(bool))
				break
			}
		}
		assert.True(t, found, "expected tag 'rust' in list")
	})

	t.Run("DELETE unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/followed_tags/tag-rust", nil)
		rec := httptest.NewRecorder()
		handler.DELETEFollowedTag(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("DELETE by tag id returns 200 and removes from list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/followed_tags/tag-rust", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		req = testutil.AddChiURLParam(req, "id", "tag-rust")
		rec := httptest.NewRecorder()
		handler.DELETEFollowedTag(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		listReq := httptest.NewRequest(http.MethodGet, "/api/v1/followed_tags", nil)
		listReq = listReq.WithContext(middleware.WithAccount(listReq.Context(), actor))
		listRec := httptest.NewRecorder()
		handler.GETFollowedTags(listRec, listReq)
		var list []map[string]any
		require.NoError(t, json.NewDecoder(listRec.Body).Decode(&list))
		for _, tag := range list {
			assert.NotEqual(t, "rust", tag["name"], "rust should be removed after DELETE")
		}
	})
}

func TestAccountsHandler_BlockUnblock(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc := service.NewFollowService(st, nil, nil)
	handler := NewAccountsHandler(accountSvc, followSvc, nil, "example.com")

	actor, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	target, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "bob",
		Email:        "bob@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
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
	followSvc := service.NewFollowService(st, nil, nil)
	handler := NewAccountsHandler(accountSvc, followSvc, nil, "example.com")

	actor, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	target, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "bob",
		Email:        "bob@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
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
	followSvc := service.NewFollowService(st, nil, nil)
	handler := NewAccountsHandler(accountSvc, followSvc, nil, "example.com")

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/accounts/update_credentials", nil)
		rec := httptest.NewRecorder()
		handler.PATCHUpdateCredentials(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("authenticated with display_name returns 200", func(t *testing.T) {
		acc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "alice",
			Email:        "alice@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
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
}

func TestAccountsHandler_GETDirectory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc := service.NewFollowService(st, nil, nil)
	handler := NewAccountsHandler(accountSvc, followSvc, nil, "example.com")

	t.Run("returns 200 with accounts and default order active", func(t *testing.T) {
		_, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "alice",
			Email:        "alice@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
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
