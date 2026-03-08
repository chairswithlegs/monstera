package activitypub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/config"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/events"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
)

func TestCollectionsHandler_GETFollowers(t *testing.T) {
	t.Parallel()
	fake := testutil.NewFakeStore()
	ctx := context.Background()
	_, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01HXXX", Username: "alice", APID: "https://example.com/users/alice",
	})
	require.NoError(t, err)
	_, err = fake.CreateUser(ctx, store.CreateUserInput{
		ID: "01USERALICE", AccountID: "01HXXX", Email: "alice@example.com", PasswordHash: "hash", Role: domain.RoleUser,
	})
	require.NoError(t, err)
	require.NoError(t, fake.ConfirmUser(ctx, "01USERALICE"))
	cfg := &config.Config{InstanceDomain: "example.com"}
	accountSvc := service.NewAccountService(fake, "https://"+cfg.InstanceDomain)
	statusSvc := service.NewStatusService(fake, service.NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", cfg.InstanceDomain, 5000, nil)
	h := NewCollectionsHandler(accountSvc, statusSvc, cfg)
	r := httptest.NewRequest(http.MethodGet, "/users/alice/followers", nil)
	r = r.WithContext(ctx)
	r = testutil.AddChiURLParam(r, "username", "alice")
	w := httptest.NewRecorder()
	h.GETFollowers(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	var coll struct {
		Type       string `json:"type"`
		TotalItems int    `json:"totalItems"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&coll))
	assert.Equal(t, "OrderedCollection", coll.Type)
	assert.Equal(t, 0, coll.TotalItems)
}

func TestCollectionsHandler_GETFollowing(t *testing.T) {
	t.Parallel()
	fake := testutil.NewFakeStore()
	ctx := context.Background()
	_, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01HXXX", Username: "alice", APID: "https://example.com/users/alice",
	})
	require.NoError(t, err)
	_, err = fake.CreateUser(ctx, store.CreateUserInput{
		ID: "01USERALICE", AccountID: "01HXXX", Email: "alice@example.com", PasswordHash: "hash", Role: domain.RoleUser,
	})
	require.NoError(t, err)
	require.NoError(t, fake.ConfirmUser(ctx, "01USERALICE"))
	cfg := &config.Config{InstanceDomain: "example.com"}
	accountSvc := service.NewAccountService(fake, "https://"+cfg.InstanceDomain)
	statusSvc := service.NewStatusService(fake, service.NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", cfg.InstanceDomain, 5000, nil)
	h := NewCollectionsHandler(accountSvc, statusSvc, cfg)
	r := httptest.NewRequest(http.MethodGet, "/users/alice/following", nil)
	r = r.WithContext(ctx)
	r = testutil.AddChiURLParam(r, "username", "alice")
	w := httptest.NewRecorder()
	h.GETFollowing(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	var coll struct {
		Type       string `json:"type"`
		TotalItems int    `json:"totalItems"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&coll))
	assert.Equal(t, "OrderedCollection", coll.Type)
	assert.Equal(t, 0, coll.TotalItems)
}

func TestCollectionsHandler_GETFeatured(t *testing.T) {
	t.Parallel()
	fake := testutil.NewFakeStore()
	ctx := context.Background()
	_, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01HXXX", Username: "alice", APID: "https://example.com/users/alice",
	})
	require.NoError(t, err)
	_, err = fake.CreateUser(ctx, store.CreateUserInput{
		ID: "01USERALICE", AccountID: "01HXXX", Email: "alice@example.com", PasswordHash: "hash", Role: domain.RoleUser,
	})
	require.NoError(t, err)
	require.NoError(t, fake.ConfirmUser(ctx, "01USERALICE"))
	cfg := &config.Config{InstanceDomain: "example.com"}
	accountSvc := service.NewAccountService(fake, "https://"+cfg.InstanceDomain)
	statusSvc := service.NewStatusService(fake, service.NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", cfg.InstanceDomain, 5000, nil)
	h := NewCollectionsHandler(accountSvc, statusSvc, cfg)
	r := httptest.NewRequest(http.MethodGet, "/users/alice/collections/featured", nil)
	r = r.WithContext(ctx)
	r = testutil.AddChiURLParam(r, "username", "alice")
	w := httptest.NewRecorder()
	h.GETFeatured(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	var coll struct {
		Type       string `json:"type"`
		TotalItems int    `json:"totalItems"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&coll))
	assert.Equal(t, "OrderedCollection", coll.Type)
	assert.Equal(t, 0, coll.TotalItems)

	t.Run("returns pinned statuses as orderedItems", func(t *testing.T) {
		statusID := "01HPINNED01"
		_, err := fake.CreateStatus(ctx, store.CreateStatusInput{
			ID:         statusID,
			URI:        "https://example.com/statuses/" + statusID,
			AccountID:  "01HXXX",
			Text:       strPtr("pinned post"),
			Content:    strPtr("<p>pinned post</p>"),
			Visibility: "public",
			APID:       "https://example.com/statuses/" + statusID,
			Local:      true,
		})
		require.NoError(t, err)
		err = fake.CreateAccountPin(ctx, "01HXXX", statusID)
		require.NoError(t, err)

		w2 := httptest.NewRecorder()
		h.GETFeatured(w2, r)
		assert.Equal(t, http.StatusOK, w2.Code)
		var coll2 struct {
			Type         string            `json:"type"`
			TotalItems   int               `json:"totalItems"`
			OrderedItems []json.RawMessage `json:"orderedItems"`
		}
		require.NoError(t, json.NewDecoder(w2.Body).Decode(&coll2))
		assert.Equal(t, "OrderedCollection", coll2.Type)
		assert.Equal(t, 1, coll2.TotalItems)
		require.Len(t, coll2.OrderedItems, 1)
	})
}

func strPtr(s string) *string { return &s }
