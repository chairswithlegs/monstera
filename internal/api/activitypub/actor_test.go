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

func TestActorHandler_GETActor(t *testing.T) {
	t.Parallel()
	fake := testutil.NewFakeStore()
	ctx := context.Background()
	_, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01HXXX", Username: "bob", Domain: nil, DisplayName: testutil.StrPtr("Bob"),
		PublicKey: "-----BEGIN PUBLIC KEY-----\nMIIB...", APID: "https://example.com/users/bob",
	})
	require.NoError(t, err)
	_, err = fake.CreateUser(ctx, store.CreateUserInput{
		ID: "01USERBOB", AccountID: "01HXXX", Email: "bob@example.com", PasswordHash: "hash", Role: domain.RoleUser,
	})
	require.NoError(t, err)
	require.NoError(t, fake.ConfirmUser(ctx, "01USERBOB"))

	h := NewActorHandler(service.NewAccountService(fake, "https://example.com"), "https://example.com", "https://ui.example.com")

	r := httptest.NewRequest(http.MethodGet, "/users/bob", nil)
	r.Header.Set("Accept", "application/activity+json")
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

func TestActorHandler_browser_redirects_to_profile(t *testing.T) {
	t.Parallel()
	h := NewActorHandler(&mockAccountService{
		GetActiveLocalAccountFunc: func(_ context.Context, _ string) (*domain.Account, error) {
			return &domain.Account{Username: "alice"}, nil
		},
	}, "https://example.com", "https://ui.example.com")

	r := httptest.NewRequest(http.MethodGet, "/users/alice", nil)
	r.Header.Set("Accept", "text/html")
	r = testutil.AddChiURLParam(r, "username", "alice")
	w := httptest.NewRecorder()
	h.GETActor(w, r)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "https://ui.example.com/public/profile?u=alice", w.Header().Get("Location"))
}

func TestActorHandler_ld_json_accept_returns_actor(t *testing.T) {
	t.Parallel()
	h := NewActorHandler(&mockAccountService{
		GetActiveLocalAccountFunc: func(_ context.Context, _ string) (*domain.Account, error) {
			return &domain.Account{ID: "01ALICE", Username: "alice", APID: "https://example.com/users/alice"}, nil
		},
	}, "https://example.com", "https://ui.example.com")

	r := httptest.NewRequest(http.MethodGet, "/users/alice", nil)
	r.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	r = testutil.AddChiURLParam(r, "username", "alice")
	w := httptest.NewRecorder()
	h.GETActor(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/activity+json; charset=utf-8", w.Header().Get("Content-Type"))
}

func TestActorHandler_notFound(t *testing.T) {
	t.Parallel()
	h := NewActorHandler(&mockAccountService{
		GetActiveLocalAccountFunc: func(_ context.Context, _ string) (*domain.Account, error) {
			return nil, domain.ErrNotFound
		},
	}, "https://example.com", "https://ui.example.com")
	r := httptest.NewRequest(http.MethodGet, "/users/nobody", nil)
	r.Header.Set("Accept", "application/activity+json")
	r = testutil.AddChiURLParam(r, "username", "nobody")
	w := httptest.NewRecorder()
	h.GETActor(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

type mockAccountService struct {
	service.AccountService
	GetActiveLocalAccountFunc func(ctx context.Context, username string) (*domain.Account, error)
}

func (m *mockAccountService) GetActiveLocalAccount(ctx context.Context, username string) (*domain.Account, error) {
	return m.GetActiveLocalAccountFunc(ctx, username)
}

func (m *mockAccountService) SuspendRemote(context.Context, string) error { panic("unused") }
