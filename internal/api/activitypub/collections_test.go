package activitypub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera-fed/internal/config"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/service"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	"github.com/chairswithlegs/monstera-fed/internal/testutil"
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
	h := NewCollectionsHandler(service.NewAccountService(fake, "https://"+cfg.InstanceDomain), cfg)
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
	h := NewCollectionsHandler(service.NewAccountService(fake, "https://"+cfg.InstanceDomain), cfg)
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
}
