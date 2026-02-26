package activitypub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera-fed/internal/config"
	"github.com/chairswithlegs/monstera-fed/internal/service"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	"github.com/chairswithlegs/monstera-fed/internal/testutil"
)

// testDeps builds Deps from a store and config for handler tests.
func testDeps(s store.Store, cfg *config.Config) Deps {
	return Deps{
		Accounts:  service.NewAccountService(s, "https://"+cfg.InstanceDomain),
		Timelines: service.NewTimelineService(s),
		Instance:  service.NewInstanceService(s),
		Config:    cfg,
		Logger:    slog.Default(),
	}
}

func TestWebFingerHandler_ServeHTTP(t *testing.T) {
	t.Parallel()
	fake := testutil.NewFakeStore()
	ctx := context.Background()
	_, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID:           "01HXXX",
		Username:     "alice",
		Domain:       nil,
		DisplayName:  strPtr("Alice"),
		PublicKey:    "-----BEGIN PUBLIC KEY-----\n...",
		InboxURL:     "https://example.com/users/alice/inbox",
		OutboxURL:    "https://example.com/users/alice/outbox",
		FollowersURL: "https://example.com/users/alice/followers",
		FollowingURL: "https://example.com/users/alice/following",
		APID:         "https://example.com/users/alice",
	})
	require.NoError(t, err)

	cfg := &config.Config{InstanceDomain: "example.com"}
	deps := testDeps(fake, cfg)
	h := NewWebFingerHandler(deps)

	r := httptest.NewRequest(http.MethodGet, "/.well-known/webfinger?resource=acct:alice@example.com", nil)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

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
	deps := testDeps(testutil.NewFakeStore(), &config.Config{InstanceDomain: "example.com"})
	h := NewWebFingerHandler(deps)
	r := httptest.NewRequest(http.MethodGet, "/.well-known/webfinger", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWebFingerHandler_wrongDomain(t *testing.T) {
	t.Parallel()
	fake := testutil.NewFakeStore()
	ctx := context.Background()
	_, _ = fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01HXXX", Username: "alice", APID: "https://example.com/users/alice",
	})
	deps := testDeps(fake, &config.Config{InstanceDomain: "example.com"})
	h := NewWebFingerHandler(deps)
	r := httptest.NewRequest(http.MethodGet, "/.well-known/webfinger?resource=acct:alice@other.com", nil)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestNodeInfoPointerHandler_ServeHTTP(t *testing.T) {
	t.Parallel()
	deps := testDeps(testutil.NewFakeStore(), &config.Config{InstanceDomain: "example.com"})
	h := NewNodeInfoPointerHandler(deps)
	r := httptest.NewRequest(http.MethodGet, "/.well-known/nodeinfo", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Links []struct {
			Rel  string `json:"rel"`
			Href string `json:"href"`
		} `json:"links"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, "http://nodeinfo.diaspora.software/ns/schema/2.0", body.Links[0].Rel)
	assert.Equal(t, "https://example.com/nodeinfo/2.0", body.Links[0].Href)
}

func TestNodeInfoHandler_ServeHTTP(t *testing.T) {
	t.Parallel()
	deps := testDeps(testutil.NewFakeStore(), &config.Config{InstanceDomain: "example.com", Version: "0.1.0"})
	h := NewNodeInfoHandler(deps)
	r := httptest.NewRequest(http.MethodGet, "/nodeinfo/2.0", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Version  string `json:"version"`
		Software struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"software"`
		Protocols []string `json:"protocols"`
		Usage     struct {
			Users      struct{ Total int64 } `json:"users"`
			LocalPosts int64                 `json:"localPosts"`
		} `json:"usage"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, "2.0", body.Version)
	assert.Equal(t, "monstera-fed", body.Software.Name)
	assert.Equal(t, "0.1.0", body.Software.Version)
	assert.Contains(t, body.Protocols, "activitypub")
}

func TestActorHandler_ServeHTTP(t *testing.T) {
	t.Parallel()
	fake := testutil.NewFakeStore()
	ctx := context.Background()
	_, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01HXXX", Username: "bob", Domain: nil, DisplayName: strPtr("Bob"),
		PublicKey: "-----BEGIN PUBLIC KEY-----\nMIIB...", APID: "https://example.com/users/bob",
	})
	require.NoError(t, err)

	cfg := &config.Config{InstanceDomain: "example.com"}
	deps := testDeps(fake, cfg)
	h := NewActorHandler(deps)

	r := httptest.NewRequest(http.MethodGet, "/users/bob", nil)
	r = r.WithContext(ctx)
	r = addChiURLParam(r, "bob")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

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
	deps := testDeps(testutil.NewFakeStore(), &config.Config{InstanceDomain: "example.com"})
	h := NewActorHandler(deps)
	r := httptest.NewRequest(http.MethodGet, "/users/nobody", nil)
	r = addChiURLParam(r, "nobody")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCollectionsHandler_ServeFollowers(t *testing.T) {
	t.Parallel()
	fake := testutil.NewFakeStore()
	ctx := context.Background()
	_, _ = fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01HXXX", Username: "alice", APID: "https://example.com/users/alice",
	})
	deps := testDeps(fake, &config.Config{InstanceDomain: "example.com"})
	h := NewCollectionsHandler(deps)
	r := httptest.NewRequest(http.MethodGet, "/users/alice/followers", nil)
	r = r.WithContext(ctx)
	r = addChiURLParam(r, "alice")
	w := httptest.NewRecorder()
	h.ServeFollowers(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	var coll struct {
		Type       string `json:"type"`
		TotalItems int    `json:"totalItems"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&coll))
	assert.Equal(t, "OrderedCollection", coll.Type)
	assert.Equal(t, 0, coll.TotalItems)
}

func TestCollectionsHandler_ServeFeatured(t *testing.T) {
	t.Parallel()
	fake := testutil.NewFakeStore()
	ctx := context.Background()
	_, _ = fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01HXXX", Username: "alice", APID: "https://example.com/users/alice",
	})
	deps := testDeps(fake, &config.Config{InstanceDomain: "example.com"})
	h := NewCollectionsHandler(deps)
	r := httptest.NewRequest(http.MethodGet, "/users/alice/collections/featured", nil)
	r = r.WithContext(ctx)
	r = addChiURLParam(r, "alice")
	w := httptest.NewRecorder()
	h.ServeFeatured(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	var coll struct {
		Type       string `json:"type"`
		TotalItems int    `json:"totalItems"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&coll))
	assert.Equal(t, "OrderedCollection", coll.Type)
	assert.Equal(t, 0, coll.TotalItems)
}

func TestOutboxHandler_ServeHTTP_collection(t *testing.T) {
	t.Parallel()
	fake := testutil.NewFakeStore()
	ctx := context.Background()
	_, _ = fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01HXXX", Username: "alice", APID: "https://example.com/users/alice",
	})
	deps := testDeps(fake, &config.Config{InstanceDomain: "example.com"})
	h := NewOutboxHandler(deps)
	r := httptest.NewRequest(http.MethodGet, "/users/alice/outbox", nil)
	r = r.WithContext(ctx)
	r = addChiURLParam(r, "alice")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	var coll struct {
		Type       string `json:"type"`
		TotalItems int    `json:"totalItems"`
		First      string `json:"first"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&coll))
	assert.Equal(t, "OrderedCollection", coll.Type)
	assert.Equal(t, "https://example.com/users/alice/outbox?page=true", coll.First)
}

func TestOutboxHandler_ServeHTTP_page(t *testing.T) {
	t.Parallel()
	fake := testutil.NewFakeStore()
	ctx := context.Background()
	_, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01HXXX", Username: "alice", APID: "https://example.com/users/alice",
	})
	require.NoError(t, err)
	content := "<p>hello</p>"
	_, err = fake.CreateStatus(ctx, store.CreateStatusInput{
		ID: "01HYYY", URI: "https://example.com/statuses/01HYYY", AccountID: "01HXXX",
		Content: &content, Visibility: "public", APID: "https://example.com/statuses/01HYYY", Local: true,
	})
	require.NoError(t, err)

	deps := testDeps(fake, &config.Config{InstanceDomain: "example.com"})
	h := NewOutboxHandler(deps)
	r := httptest.NewRequest(http.MethodGet, "/users/alice/outbox?page=true", nil)
	r = r.WithContext(ctx)
	r = addChiURLParam(r, "alice")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	var page struct {
		Type         string            `json:"type"`
		OrderedItems []json.RawMessage `json:"orderedItems"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&page))
	assert.Equal(t, "OrderedCollectionPage", page.Type)
	require.Len(t, page.OrderedItems, 1)
	var create struct {
		Type  string `json:"type"`
		Actor string `json:"actor"`
	}
	require.NoError(t, json.Unmarshal(page.OrderedItems[0], &create))
	assert.Equal(t, "Create", create.Type)
	assert.Equal(t, "https://example.com/users/alice", create.Actor)
}

func strPtr(s string) *string { return &s }

func TestInboxHandler_ServeHTTP_methodNotAllowed(t *testing.T) {
	t.Parallel()
	deps := testDeps(testutil.NewFakeStore(), &config.Config{InstanceDomain: "example.com"})
	h := NewInboxHandler(deps)
	r := httptest.NewRequest(http.MethodGet, "/inbox", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestInboxHandler_ServeHTTP_noProcessor_returns202(t *testing.T) {
	t.Parallel()
	deps := testDeps(testutil.NewFakeStore(), &config.Config{InstanceDomain: "example.com"})
	deps.Cache = nil
	deps.Inbox = nil
	h := NewInboxHandler(deps)
	r := httptest.NewRequest(http.MethodPost, "/inbox", strings.NewReader(`{"type":"Create"}`))
	r.Header.Set("Content-Type", "application/activity+json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusAccepted, w.Code)
}

// addChiURLParam sets chi's "username" URL param on the request for testing.
func addChiURLParam(r *http.Request, username string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("username", username)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}
