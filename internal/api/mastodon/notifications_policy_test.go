package mastodon

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
)

func newPolicyHandler(t *testing.T, st *testutil.FakeStore) (*NotificationsPolicyHandler, *domain.Account) {
	t.Helper()
	ctx := context.Background()
	accountSvc := service.NewAccountService(st, "https://example.com")
	policySvc := service.NewNotificationPolicyService(st)
	handler := NewNotificationsPolicyHandler(policySvc, accountSvc, nil, "example.com")
	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	return handler, acc
}

func TestNotificationsPolicyHandler_GETPolicy(t *testing.T) {
	t.Parallel()

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		t.Parallel()
		st := testutil.NewFakeStore()
		handler, _ := newPolicyHandler(t, st)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/policy", nil)
		rec := httptest.NewRecorder()
		handler.GETPolicy(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("returns 200 with default policy", func(t *testing.T) {
		t.Parallel()
		st := testutil.NewFakeStore()
		handler, acc := newPolicyHandler(t, st)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/policy", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.GETPolicy(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, false, body["filter_not_following"])
		assert.Equal(t, false, body["filter_not_followers"])
		assert.Equal(t, false, body["filter_new_accounts"])
		assert.Equal(t, false, body["filter_private_mentions"])
		summary, ok := body["summary"].(map[string]any)
		require.True(t, ok)
		assert.InDelta(t, float64(0), summary["pending_requests_count"], 0)
		assert.InDelta(t, float64(0), summary["pending_notifications_count"], 0)
	})
}

func TestNotificationsPolicyHandler_PATCHPolicy(t *testing.T) {
	t.Parallel()

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		t.Parallel()
		st := testutil.NewFakeStore()
		handler, _ := newPolicyHandler(t, st)
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/notifications/policy", nil)
		rec := httptest.NewRecorder()
		handler.PATCHPolicy(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("updates policy and returns 200", func(t *testing.T) {
		t.Parallel()
		st := testutil.NewFakeStore()
		handler, acc := newPolicyHandler(t, st)
		body := map[string]any{
			"filter_not_following":    true,
			"filter_not_followers":    false,
			"filter_new_accounts":     true,
			"filter_private_mentions": false,
		}
		bodyJSON, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/notifications/policy", bytes.NewReader(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.PATCHPolicy(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var resp map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
		assert.Equal(t, true, resp["filter_not_following"])
		assert.Equal(t, false, resp["filter_not_followers"])
		assert.Equal(t, true, resp["filter_new_accounts"])
		assert.Equal(t, false, resp["filter_private_mentions"])
	})
}

func TestNotificationsPolicyHandler_GETMerged(t *testing.T) {
	t.Parallel()

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		t.Parallel()
		st := testutil.NewFakeStore()
		handler, _ := newPolicyHandler(t, st)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/requests/merged", nil)
		rec := httptest.NewRecorder()
		handler.GETMerged(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("returns merged true", func(t *testing.T) {
		t.Parallel()
		st := testutil.NewFakeStore()
		handler, acc := newPolicyHandler(t, st)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/requests/merged", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.GETMerged(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, true, body["merged"])
	})
}

func TestNotificationsPolicyHandler_GETRequests(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		t.Parallel()
		st := testutil.NewFakeStore()
		handler, _ := newPolicyHandler(t, st)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/requests", nil)
		rec := httptest.NewRecorder()
		handler.GETRequests(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("returns empty list when no requests", func(t *testing.T) {
		t.Parallel()
		st := testutil.NewFakeStore()
		handler, acc := newPolicyHandler(t, st)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/requests", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.GETRequests(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})

	t.Run("returns requests with string notifications_count", func(t *testing.T) {
		t.Parallel()
		st := testutil.NewFakeStore()
		handler, acc := newPolicyHandler(t, st)
		fromAcc, err := service.NewAccountService(st, "https://example.com").Create(ctx, service.CreateAccountInput{Username: "bob"})
		require.NoError(t, err)
		reqID := uid.New()
		_, err = st.UpsertNotificationRequest(ctx, store.UpsertNotificationRequestInput{
			ID:            reqID,
			AccountID:     acc.ID,
			FromAccountID: fromAcc.ID,
		})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/requests", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.GETRequests(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		require.Len(t, body, 1)
		// notifications_count must be a string per Mastodon API spec
		assert.Equal(t, "1", body[0]["notifications_count"])
		assert.Equal(t, reqID, body[0]["id"])
	})
}

func TestNotificationsPolicyHandler_POSTAcceptRequest(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		t.Parallel()
		st := testutil.NewFakeStore()
		handler, _ := newPolicyHandler(t, st)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/requests/x/accept", nil)
		req = testutil.AddChiURLParam(req, "id", "x")
		rec := httptest.NewRecorder()
		handler.POSTAcceptRequest(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("accepts request and returns 200", func(t *testing.T) {
		t.Parallel()
		st := testutil.NewFakeStore()
		handler, acc := newPolicyHandler(t, st)
		fromAcc, err := service.NewAccountService(st, "https://example.com").Create(ctx, service.CreateAccountInput{Username: "charlie"})
		require.NoError(t, err)
		reqID := uid.New()
		_, err = st.UpsertNotificationRequest(ctx, store.UpsertNotificationRequestInput{
			ID:            reqID,
			AccountID:     acc.ID,
			FromAccountID: fromAcc.ID,
		})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/requests/"+reqID+"/accept", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", reqID)
		rec := httptest.NewRecorder()
		handler.POSTAcceptRequest(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestNotificationsPolicyHandler_POSTDismissRequests(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		t.Parallel()
		st := testutil.NewFakeStore()
		handler, _ := newPolicyHandler(t, st)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/requests/dismiss", bytes.NewReader([]byte(`{"id":[]}`)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTDismissRequests(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("bulk dismisses and returns 200", func(t *testing.T) {
		t.Parallel()
		st := testutil.NewFakeStore()
		handler, acc := newPolicyHandler(t, st)
		fromAcc, err := service.NewAccountService(st, "https://example.com").Create(ctx, service.CreateAccountInput{Username: "dave"})
		require.NoError(t, err)
		reqID := uid.New()
		_, err = st.UpsertNotificationRequest(ctx, store.UpsertNotificationRequestInput{
			ID:            reqID,
			AccountID:     acc.ID,
			FromAccountID: fromAcc.ID,
		})
		require.NoError(t, err)
		body := map[string]any{"id": []string{reqID}}
		bodyJSON, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/requests/dismiss", bytes.NewReader(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.POSTDismissRequests(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("returns 400 when too many ids", func(t *testing.T) {
		t.Parallel()
		st := testutil.NewFakeStore()
		handler, acc := newPolicyHandler(t, st)
		ids := make([]string, maxNotificationRequestBulkLimit+1)
		for i := range ids {
			ids[i] = uid.New()
		}
		body := map[string]any{"id": ids}
		bodyJSON, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/requests/dismiss", bytes.NewReader(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.POSTDismissRequests(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestNotificationsPolicyHandler_POSTDismissRequest(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		t.Parallel()
		st := testutil.NewFakeStore()
		handler, _ := newPolicyHandler(t, st)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/requests/x/dismiss", nil)
		req = testutil.AddChiURLParam(req, "id", "x")
		rec := httptest.NewRecorder()
		handler.POSTDismissRequest(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("dismisses request and returns 200", func(t *testing.T) {
		t.Parallel()
		st := testutil.NewFakeStore()
		handler, acc := newPolicyHandler(t, st)
		fromAcc, err := service.NewAccountService(st, "https://example.com").Create(ctx, service.CreateAccountInput{Username: "eve"})
		require.NoError(t, err)
		reqID := uid.New()
		_, err = st.UpsertNotificationRequest(ctx, store.UpsertNotificationRequestInput{
			ID:            reqID,
			AccountID:     acc.ID,
			FromAccountID: fromAcc.ID,
		})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/requests/"+reqID+"/dismiss", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", reqID)
		rec := httptest.NewRecorder()
		handler.POSTDismissRequest(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestNotificationsPolicyHandler_GETRequest(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		t.Parallel()
		st := testutil.NewFakeStore()
		handler, _ := newPolicyHandler(t, st)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/requests/x", nil)
		req = testutil.AddChiURLParam(req, "id", "x")
		rec := httptest.NewRecorder()
		handler.GETRequest(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("returns 404 for unknown id", func(t *testing.T) {
		t.Parallel()
		st := testutil.NewFakeStore()
		handler, acc := newPolicyHandler(t, st)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/requests/notexist", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "notexist")
		rec := httptest.NewRecorder()
		handler.GETRequest(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("returns request by id", func(t *testing.T) {
		t.Parallel()
		st := testutil.NewFakeStore()
		handler, acc := newPolicyHandler(t, st)
		fromAcc, err := service.NewAccountService(st, "https://example.com").Create(ctx, service.CreateAccountInput{Username: "frank"})
		require.NoError(t, err)
		reqID := uid.New()
		_, err = st.UpsertNotificationRequest(ctx, store.UpsertNotificationRequestInput{
			ID:            reqID,
			AccountID:     acc.ID,
			FromAccountID: fromAcc.ID,
		})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/requests/"+reqID, nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", reqID)
		rec := httptest.NewRecorder()
		handler.GETRequest(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, reqID, body["id"])
	})
}

func TestNotificationsPolicyHandler_POSTAcceptRequests(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		t.Parallel()
		st := testutil.NewFakeStore()
		handler, _ := newPolicyHandler(t, st)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/requests/accept", bytes.NewReader([]byte(`{"id":[]}`)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTAcceptRequests(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("bulk accepts and returns 200", func(t *testing.T) {
		t.Parallel()
		st := testutil.NewFakeStore()
		handler, acc := newPolicyHandler(t, st)
		fromAcc, err := service.NewAccountService(st, "https://example.com").Create(ctx, service.CreateAccountInput{Username: "grace"})
		require.NoError(t, err)
		reqID := uid.New()
		_, err = st.UpsertNotificationRequest(ctx, store.UpsertNotificationRequestInput{
			ID:            reqID,
			AccountID:     acc.ID,
			FromAccountID: fromAcc.ID,
		})
		require.NoError(t, err)
		body := map[string]any{"id": []string{reqID}}
		bodyJSON, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/requests/accept", bytes.NewReader(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.POSTAcceptRequests(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("returns 400 when too many ids", func(t *testing.T) {
		t.Parallel()
		st := testutil.NewFakeStore()
		handler, acc := newPolicyHandler(t, st)
		ids := make([]string, maxNotificationRequestBulkLimit+1)
		for i := range ids {
			ids[i] = uid.New()
		}
		body := map[string]any{"id": ids}
		bodyJSON, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/requests/accept", bytes.NewReader(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.POSTAcceptRequests(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
