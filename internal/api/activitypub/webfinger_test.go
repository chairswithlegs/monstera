package activitypub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
)

func TestWebFingerHandler_GETWebFinger(t *testing.T) {
	t.Parallel()
	fake := testutil.NewFakeStore()
	ctx := context.Background()
	_, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID:           "01HXXX",
		Username:     "alice",
		Domain:       nil,
		DisplayName:  testutil.StrPtr("Alice"),
		PublicKey:    "-----BEGIN PUBLIC KEY-----\n...",
		InboxURL:     "https://example.com/users/alice/inbox",
		OutboxURL:    "https://example.com/users/alice/outbox",
		FollowersURL: "https://example.com/users/alice/followers",
		FollowingURL: "https://example.com/users/alice/following",
		APID:         "https://example.com/users/alice",
	})
	require.NoError(t, err)
	_, err = fake.CreateUser(ctx, store.CreateUserInput{
		ID:           "01USERALICE",
		AccountID:    "01HXXX",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	require.NoError(t, fake.ConfirmUser(ctx, "01USERALICE"))

	h := NewWebFingerHandler(service.NewAccountService(fake, "https://example.com"), "example.com", "https://example.com")

	r := httptest.NewRequest(http.MethodGet, "/.well-known/webfinger?resource=acct:alice@example.com", nil)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()
	h.GETWebFinger(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/jrd+json; charset=utf-8", w.Header().Get("Content-Type"))
	var jrd struct {
		Subject string   `json:"subject"`
		Aliases []string `json:"aliases"`
		Links   []struct {
			Rel  string `json:"rel"`
			Type string `json:"type"`
			Href string `json:"href"`
		} `json:"links"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&jrd))
	assert.Equal(t, "acct:alice@example.com", jrd.Subject)
	assert.Contains(t, jrd.Aliases, "https://example.com/users/alice")
	assert.Equal(t, "self", jrd.Links[0].Rel)
	assert.Equal(t, "application/activity+json", jrd.Links[0].Type)
}

func TestWebFingerHandler_missingResource(t *testing.T) {
	t.Parallel()
	h := NewWebFingerHandler(service.NewAccountService(testutil.NewFakeStore(), "https://example.com"), "example.com", "https://example.com")
	r := httptest.NewRequest(http.MethodGet, "/.well-known/webfinger", nil)
	w := httptest.NewRecorder()
	h.GETWebFinger(w, r)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestWebFingerHandler_wrongDomain(t *testing.T) {
	t.Parallel()
	fake := testutil.NewFakeStore()
	ctx := context.Background()
	_, _ = fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01HXXX", Username: "alice", APID: "https://example.com/users/alice",
	})
	h := NewWebFingerHandler(service.NewAccountService(fake, "https://example.com"), "example.com", "https://example.com")
	r := httptest.NewRequest(http.MethodGet, "/.well-known/webfinger?resource=acct:alice@other.com", nil)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()
	h.GETWebFinger(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestWebFingerHandler_pendingUser_returnsNotFound(t *testing.T) {
	t.Parallel()
	fake := testutil.NewFakeStore()
	ctx := context.Background()
	_, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01HPENDING", Username: "pending", APID: "https://example.com/users/pending",
	})
	require.NoError(t, err)
	_, err = fake.CreateUser(ctx, store.CreateUserInput{
		ID:           "01USERPENDING",
		AccountID:    "01HPENDING",
		Email:        "pending@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	// Do not call ConfirmUser — user remains pending.

	h := NewWebFingerHandler(service.NewAccountService(fake, "https://example.com"), "example.com", "https://example.com")
	r := httptest.NewRequest(http.MethodGet, "/.well-known/webfinger?resource=acct:pending@example.com", nil)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()
	h.GETWebFinger(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
