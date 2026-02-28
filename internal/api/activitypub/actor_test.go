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
	"github.com/chairswithlegs/monstera-fed/internal/service"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	"github.com/chairswithlegs/monstera-fed/internal/testutil"
)

func TestActorHandler_GETActor(t *testing.T) {
	t.Parallel()
	fake := testutil.NewFakeStore()
	ctx := context.Background()
	_, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01HXXX", Username: "bob", Domain: nil, DisplayName: testutil.StrPtr("Bob"),
		PublicKey: "-----BEGIN PUBLIC KEY-----\nMIIB...", APID: "https://example.com/users/bob",
	})
	require.NoError(t, err)

	cfg := &config.Config{InstanceDomain: "example.com"}
	h := NewActorHandler(service.NewAccountService(fake, "https://example.com"), cfg)

	r := httptest.NewRequest(http.MethodGet, "/users/bob", nil)
	r = r.WithContext(ctx)
	r = testutil.AddChiURLParam(r, "username", "bob")
	w := httptest.NewRecorder()
	h.GETActor(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/activity+json; charset=utf-8", w.Header().Get("Content-Type"))
	var actor struct {
		Type              string `json:"type"`
		ID                string `json:"id"`
		PreferredUsername string `json:"preferredUsername"`
		Name              string `json:"name"`
		Inbox             string `json:"inbox"`
		PublicKey         struct {
			ID           string `json:"id"`
			PublicKeyPem string `json:"publicKeyPem"`
		} `json:"publicKey"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&actor))
	assert.Equal(t, "Person", actor.Type)
	assert.Equal(t, "https://example.com/users/bob", actor.ID)
	assert.Equal(t, "bob", actor.PreferredUsername)
	assert.Equal(t, "Bob", actor.Name)
	assert.Equal(t, "-----BEGIN PUBLIC KEY-----\nMIIB...", actor.PublicKey.PublicKeyPem)
}

func TestActorHandler_notFound(t *testing.T) {
	t.Parallel()
	h := NewActorHandler(service.NewAccountService(testutil.NewFakeStore(), "https://example.com"), &config.Config{InstanceDomain: "example.com"})
	r := httptest.NewRequest(http.MethodGet, "/users/nobody", nil)
	r = testutil.AddChiURLParam(r, "username", "nobody")
	w := httptest.NewRecorder()
	h.GETActor(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
