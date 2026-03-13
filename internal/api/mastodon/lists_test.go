package mastodon

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListsHandler_GETLists(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	listSvc := service.NewListService(st)
	handler := NewListsHandler(listSvc, accountSvc, "example.com")

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/lists", nil)
		rec := httptest.NewRecorder()
		handler.GETLists(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("authenticated no lists returns 200 and empty array", func(t *testing.T) {
		acc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "alice",
			Email:        "alice@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/lists", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.GETLists(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})

	t.Run("authenticated with lists returns 200 and list array", func(t *testing.T) {
		acc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "bob",
			Email:        "bob@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)

		_, err = listSvc.CreateList(ctx, acc.ID, "My list", domain.ListRepliesPolicyList, false)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/lists", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.GETLists(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		require.Len(t, body, 1)
		assert.Equal(t, "My list", body[0]["title"])
		assert.Equal(t, domain.ListRepliesPolicyList, body[0]["replies_policy"])
		assert.False(t, body[0]["exclusive"].(bool))
	})
}

func TestListsHandler_GETList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	listSvc := service.NewListService(st)
	handler := NewListsHandler(listSvc, accountSvc, "example.com")

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/lists/01abc", nil)
		req = testutil.AddChiURLParam(req, "id", "01abc")
		rec := httptest.NewRecorder()
		handler.GETList(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("missing id returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/lists/", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "")
		rec := httptest.NewRecorder()
		handler.GETList(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("list not found returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/lists/01nonexistent", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "01nonexistent")
		rec := httptest.NewRecorder()
		handler.GETList(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("list exists for account returns 200 and list", func(t *testing.T) {
		l, err := listSvc.CreateList(ctx, acc.ID, "Test list", domain.ListRepliesPolicyNone, true)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/lists/"+l.ID, nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", l.ID)
		rec := httptest.NewRecorder()
		handler.GETList(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, l.ID, body["id"])
		assert.Equal(t, "Test list", body["title"])
		assert.True(t, body["exclusive"].(bool))
	})

	t.Run("list exists for another account returns 403", func(t *testing.T) {
		otherAcc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "other",
			Email:        "other@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)
		l, err := listSvc.CreateList(ctx, otherAcc.ID, "Their list", domain.ListRepliesPolicyList, false)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/lists/"+l.ID, nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", l.ID)
		rec := httptest.NewRecorder()
		handler.GETList(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

func TestListsHandler_POSTLists(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	listSvc := service.NewListService(st)
	handler := NewListsHandler(listSvc, accountSvc, "example.com")

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		body := bytes.NewBufferString(`{"title":"test"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lists", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTLists(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		body := bytes.NewBufferString(`{invalid`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lists", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.POSTLists(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("empty title returns 422", func(t *testing.T) {
		body := bytes.NewBufferString(`{"title":""}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lists", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.POSTLists(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("valid body returns 200 and list", func(t *testing.T) {
		body := bytes.NewBufferString(`{"title":"New list","replies_policy":"none","exclusive":true}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lists", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.POSTLists(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var out map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
		assert.NotEmpty(t, out["id"])
		assert.Equal(t, "New list", out["title"])
		assert.Equal(t, "none", out["replies_policy"])
		assert.True(t, out["exclusive"].(bool))
	})
}

func TestListsHandler_PUTList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	listSvc := service.NewListService(st)
	handler := NewListsHandler(listSvc, accountSvc, "example.com")

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		body := bytes.NewBufferString(`{"title":"updated"}`)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/lists/01abc", body)
		req.Header.Set("Content-Type", "application/json")
		req = testutil.AddChiURLParam(req, "id", "01abc")
		rec := httptest.NewRecorder()
		handler.PUTList(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("missing id returns 404", func(t *testing.T) {
		body := bytes.NewBufferString(`{"title":"updated"}`)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/lists/", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "")
		rec := httptest.NewRecorder()
		handler.PUTList(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		body := bytes.NewBufferString(`{invalid`)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/lists/01abc", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "01abc")
		rec := httptest.NewRecorder()
		handler.PUTList(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("list not found returns 404", func(t *testing.T) {
		body := bytes.NewBufferString(`{"title":"updated"}`)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/lists/01nonexistent", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "01nonexistent")
		rec := httptest.NewRecorder()
		handler.PUTList(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("list for another account returns 403", func(t *testing.T) {
		otherAcc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "other",
			Email:        "other@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)
		l, err := listSvc.CreateList(ctx, otherAcc.ID, "Their list", domain.ListRepliesPolicyList, false)
		require.NoError(t, err)

		body := bytes.NewBufferString(`{"title":"hacked"}`)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/lists/"+l.ID, body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", l.ID)
		rec := httptest.NewRecorder()
		handler.PUTList(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("list exists for account returns 200 and updated list", func(t *testing.T) {
		l, err := listSvc.CreateList(ctx, acc.ID, "Original", domain.ListRepliesPolicyList, false)
		require.NoError(t, err)

		body := bytes.NewBufferString(`{"title":"Updated title","replies_policy":"none","exclusive":true}`)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/lists/"+l.ID, body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", l.ID)
		rec := httptest.NewRecorder()
		handler.PUTList(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var out map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
		assert.Equal(t, l.ID, out["id"])
		assert.Equal(t, "Updated title", out["title"])
		assert.Equal(t, "none", out["replies_policy"])
		assert.True(t, out["exclusive"].(bool))
	})
}

func TestListsHandler_DELETEList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	listSvc := service.NewListService(st)
	handler := NewListsHandler(listSvc, accountSvc, "example.com")

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/lists/01abc", nil)
		req = testutil.AddChiURLParam(req, "id", "01abc")
		rec := httptest.NewRecorder()
		handler.DELETEList(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("missing id returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/lists/", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "")
		rec := httptest.NewRecorder()
		handler.DELETEList(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("list not found returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/lists/01nonexistent", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "01nonexistent")
		rec := httptest.NewRecorder()
		handler.DELETEList(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("list for another account returns 403", func(t *testing.T) {
		otherAcc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "other",
			Email:        "other@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)
		l, err := listSvc.CreateList(ctx, otherAcc.ID, "Their list", domain.ListRepliesPolicyList, false)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/lists/"+l.ID, nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", l.ID)
		rec := httptest.NewRecorder()
		handler.DELETEList(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("list for account returns 200", func(t *testing.T) {
		l, err := listSvc.CreateList(ctx, acc.ID, "To delete", domain.ListRepliesPolicyList, false)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/lists/"+l.ID, nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", l.ID)
		rec := httptest.NewRecorder()
		handler.DELETEList(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, rec.Body.Bytes())

		_, err = listSvc.GetList(ctx, acc.ID, l.ID)
		assert.Error(t, err)
	})
}

func TestListsHandler_GETListAccounts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	listSvc := service.NewListService(st)
	handler := NewListsHandler(listSvc, accountSvc, "example.com")

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/lists/01abc/accounts", nil)
		req = testutil.AddChiURLParam(req, "id", "01abc")
		rec := httptest.NewRecorder()
		handler.GETListAccounts(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("missing id returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/lists//accounts", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "")
		rec := httptest.NewRecorder()
		handler.GETListAccounts(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("list not found returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/lists/01nonexistent/accounts", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "01nonexistent")
		rec := httptest.NewRecorder()
		handler.GETListAccounts(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("list for another account returns 403", func(t *testing.T) {
		otherAcc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "other",
			Email:        "other@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)
		l, err := listSvc.CreateList(ctx, otherAcc.ID, "Their list", domain.ListRepliesPolicyList, false)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/lists/"+l.ID+"/accounts", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", l.ID)
		rec := httptest.NewRecorder()
		handler.GETListAccounts(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("list for account empty members returns 200 and empty array", func(t *testing.T) {
		l, err := listSvc.CreateList(ctx, acc.ID, "Empty list", domain.ListRepliesPolicyList, false)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/lists/"+l.ID+"/accounts", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", l.ID)
		rec := httptest.NewRecorder()
		handler.GETListAccounts(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})

	t.Run("list for account with members returns 200 and account array", func(t *testing.T) {
		l, err := listSvc.CreateList(ctx, acc.ID, "With members", domain.ListRepliesPolicyList, false)
		require.NoError(t, err)
		member1, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "member1"})
		require.NoError(t, err)
		member2, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "member2"})
		require.NoError(t, err)
		err = listSvc.AddAccountsToList(ctx, acc.ID, l.ID, []string{member1.ID, member2.ID})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/lists/"+l.ID+"/accounts", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", l.ID)
		rec := httptest.NewRecorder()
		handler.GETListAccounts(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		require.Len(t, body, 2)
		usernames := []string{body[0]["username"].(string), body[1]["username"].(string)}
		assert.Contains(t, usernames, "member1")
		assert.Contains(t, usernames, "member2")
	})
}

func TestListsHandler_POSTListAccounts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	listSvc := service.NewListService(st)
	handler := NewListsHandler(listSvc, accountSvc, "example.com")

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		body := bytes.NewBufferString(`{"account_ids":["01abc"]}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lists/01abc/accounts", body)
		req.Header.Set("Content-Type", "application/json")
		req = testutil.AddChiURLParam(req, "id", "01abc")
		rec := httptest.NewRecorder()
		handler.POSTListAccounts(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("missing id returns 404", func(t *testing.T) {
		body := bytes.NewBufferString(`{"account_ids":[]}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lists//accounts", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "")
		rec := httptest.NewRecorder()
		handler.POSTListAccounts(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		body := bytes.NewBufferString(`{invalid`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lists/01abc/accounts", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "01abc")
		rec := httptest.NewRecorder()
		handler.POSTListAccounts(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("list not found returns 404", func(t *testing.T) {
		body := bytes.NewBufferString(`{"account_ids":["01someone"]}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lists/01nonexistent/accounts", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "01nonexistent")
		rec := httptest.NewRecorder()
		handler.POSTListAccounts(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("list for another account returns 403", func(t *testing.T) {
		otherAcc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "other",
			Email:        "other@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)
		l, err := listSvc.CreateList(ctx, otherAcc.ID, "Their list", domain.ListRepliesPolicyList, false)
		require.NoError(t, err)
		member, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "member"})
		require.NoError(t, err)

		body := bytes.NewBufferString(`{"account_ids":["` + member.ID + `"]}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lists/"+l.ID+"/accounts", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", l.ID)
		rec := httptest.NewRecorder()
		handler.POSTListAccounts(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("list for account valid account_ids returns 200", func(t *testing.T) {
		l, err := listSvc.CreateList(ctx, acc.ID, "List", domain.ListRepliesPolicyList, false)
		require.NoError(t, err)
		member, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "newmember"})
		require.NoError(t, err)

		body := bytes.NewBufferString(`{"account_ids":["` + member.ID + `"]}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lists/"+l.ID+"/accounts", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", l.ID)
		rec := httptest.NewRecorder()
		handler.POSTListAccounts(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		ids, err := listSvc.ListListAccountIDs(ctx, l.ID)
		require.NoError(t, err)
		assert.Contains(t, ids, member.ID)
	})
}

func TestListsHandler_DELETEListAccounts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	listSvc := service.NewListService(st)
	handler := NewListsHandler(listSvc, accountSvc, "example.com")

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		body := bytes.NewBufferString(`{"account_ids":["01abc"]}`)
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/lists/01abc/accounts", body)
		req.Header.Set("Content-Type", "application/json")
		req = testutil.AddChiURLParam(req, "id", "01abc")
		rec := httptest.NewRecorder()
		handler.DELETEListAccounts(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("missing id returns 404", func(t *testing.T) {
		body := bytes.NewBufferString(`{"account_ids":[]}`)
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/lists//accounts", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "")
		rec := httptest.NewRecorder()
		handler.DELETEListAccounts(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		body := bytes.NewBufferString(`{invalid`)
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/lists/01abc/accounts", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "01abc")
		rec := httptest.NewRecorder()
		handler.DELETEListAccounts(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("list not found returns 404", func(t *testing.T) {
		body := bytes.NewBufferString(`{"account_ids":["01someone"]}`)
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/lists/01nonexistent/accounts", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "01nonexistent")
		rec := httptest.NewRecorder()
		handler.DELETEListAccounts(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("list for another account returns 403", func(t *testing.T) {
		otherAcc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "other",
			Email:        "other@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)
		l, err := listSvc.CreateList(ctx, otherAcc.ID, "Their list", domain.ListRepliesPolicyList, false)
		require.NoError(t, err)

		body := bytes.NewBufferString(`{"account_ids":["01any"]}`)
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/lists/"+l.ID+"/accounts", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", l.ID)
		rec := httptest.NewRecorder()
		handler.DELETEListAccounts(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("list for account valid account_ids returns 200", func(t *testing.T) {
		l, err := listSvc.CreateList(ctx, acc.ID, "List", domain.ListRepliesPolicyList, false)
		require.NoError(t, err)
		member, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "tomember"})
		require.NoError(t, err)
		err = listSvc.AddAccountsToList(ctx, acc.ID, l.ID, []string{member.ID})
		require.NoError(t, err)

		body := bytes.NewBufferString(`{"account_ids":["` + member.ID + `"]}`)
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/lists/"+l.ID+"/accounts", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", l.ID)
		rec := httptest.NewRecorder()
		handler.DELETEListAccounts(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		ids, err := listSvc.ListListAccountIDs(ctx, l.ID)
		require.NoError(t, err)
		assert.NotContains(t, ids, member.ID)
	})
}
