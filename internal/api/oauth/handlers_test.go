package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/chairswithlegs/monstera/internal/cache"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/oauth"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func TestHandler_POSTRegisterApp(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	c, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()
	srv := oauth.NewServer(st, c, "")
	authSvc := service.NewAuthService(st, "ui.example.com", oauth.MONSTERA_UI_APPLICATION_ID)
	h := NewHandler(srv, authSvc, mustParseURL("https://ui.example.com"))

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/apps", bytes.NewReader([]byte("not json")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.POSTRegisterApp(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("missing client_name returns 400", func(t *testing.T) {
		body := map[string]string{"redirect_uris": "https://app.example/cb", "scopes": "read"}
		enc, _ := json.Marshal(body)
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/apps", bytes.NewReader(enc))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.POSTRegisterApp(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("missing redirect_uris returns 400", func(t *testing.T) {
		body := map[string]string{"client_name": "My App", "scopes": "read"}
		enc, _ := json.Marshal(body)
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/apps", bytes.NewReader(enc))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.POSTRegisterApp(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("happy path JSON returns 200 with client_id and client_secret", func(t *testing.T) {
		body := map[string]any{
			"client_name":   "Test App",
			"redirect_uris": "https://app.example/callback",
			"scopes":        "read write",
			"website":       "https://app.example",
		}
		enc, _ := json.Marshal(body)
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/apps", bytes.NewReader(enc))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.POSTRegisterApp(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var resp map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
		assert.NotEmpty(t, resp["client_id"])
		assert.NotEmpty(t, resp["client_secret"])
		assert.Equal(t, "Test App", resp["name"])
	})

	t.Run("happy path form returns 200", func(t *testing.T) {
		form := "client_name=Form+App&redirect_uris=https://form.example/cb&scopes=read"
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/apps", bytes.NewReader([]byte(form)))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		h.POSTRegisterApp(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var resp map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
		assert.NotEmpty(t, resp["client_id"])
		assert.NotEmpty(t, resp["client_secret"])
	})
}

func TestHandler_GETAuthorize(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	c, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()
	srv := oauth.NewServer(st, c, "")
	authSvc := service.NewAuthService(st, "ui.example.com", oauth.MONSTERA_UI_APPLICATION_ID)
	h := NewHandler(srv, authSvc, mustParseURL("https://ui.example.com"))

	t.Run("response_type not code returns 400", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/oauth/authorize?response_type=token&client_id=foo&redirect_uri=https://app.example/cb", nil)
		rec := httptest.NewRecorder()
		h.GETAuthorize(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid client_id returns 400", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/oauth/authorize?response_type=code&client_id=nonexistent&redirect_uri=https://app.example/cb", nil)
		rec := httptest.NewRecorder()
		h.GETAuthorize(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("happy path returns 302 redirect to UI", func(t *testing.T) {
		app, err := st.CreateApplication(ctx, store.CreateApplicationInput{
			ID:           "app1",
			Name:         "Test",
			ClientID:     "client64charsxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			ClientSecret: "secret",
			RedirectURIs: "https://app.example/cb",
			Scopes:       "read",
			Website:      nil,
		})
		require.NoError(t, err)
		require.NotNil(t, app)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/oauth/authorize?response_type=code&client_id="+app.ClientID+"&redirect_uri=https://app.example/cb&state=xyz", nil)
		rec := httptest.NewRecorder()
		h.GETAuthorize(rec, req)
		assert.Equal(t, http.StatusFound, rec.Code)
		loc := rec.Header().Get("Location")
		require.NotEmpty(t, loc)
		assert.Contains(t, loc, "https://ui.example.com/oauth/authorize")
		assert.Contains(t, loc, "client_id=")
		assert.Contains(t, loc, "state=xyz")
	})
}

func TestHandler_POSTToken(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	c, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()
	srv := oauth.NewServer(st, c, "")
	authSvc := service.NewAuthService(st, "", "")
	h := NewHandler(srv, authSvc, nil)

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/oauth/token", bytes.NewReader([]byte("not json")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.POSTToken(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("unsupported grant_type returns 400", func(t *testing.T) {
		body := map[string]string{"grant_type": "password", "client_id": "c", "client_secret": "s"}
		enc, _ := json.Marshal(body)
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/oauth/token", bytes.NewReader(enc))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.POSTToken(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("client_credentials happy path returns 200 with access_token", func(t *testing.T) {
		app, err := st.CreateApplication(ctx, store.CreateApplicationInput{
			ID:           "app1",
			Name:         "App",
			ClientID:     "client64charsxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			ClientSecret: "secret64charsxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			RedirectURIs: "https://app.example/cb",
			Scopes:       "read write",
			Website:      nil,
		})
		require.NoError(t, err)

		form := "grant_type=client_credentials&client_id=" + app.ClientID + "&client_secret=" + app.ClientSecret + "&scope=read"
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/oauth/token", bytes.NewReader([]byte(form)))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		h.POSTToken(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var resp map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
		assert.NotEmpty(t, resp["access_token"])
		assert.Equal(t, "Bearer", resp["token_type"])
	})
}

func TestHandler_POSTRevoke(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	c, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()
	srv := oauth.NewServer(st, c, "")
	authSvc := service.NewAuthService(st, "", "")
	h := NewHandler(srv, authSvc, nil)

	t.Run("missing token returns 422", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/oauth/revoke", bytes.NewReader([]byte("")))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		h.POSTRevoke(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("happy path returns 200", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/oauth/revoke", bytes.NewReader([]byte("token=some-token")))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		h.POSTRevoke(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestHandler_POSTLogin(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	c, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()
	srv := oauth.NewServer(st, c, "")
	authSvc := service.NewAuthService(st, "", "")
	h := NewHandler(srv, authSvc, nil)

	app, err := st.CreateApplication(ctx, store.CreateApplicationInput{
		ID:           "app1",
		Name:         "App",
		ClientID:     "client64charsxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		ClientSecret: "secret",
		RedirectURIs: "https://app.example/cb",
		Scopes:       "read",
		Website:      nil,
	})
	require.NoError(t, err)
	acc, err := st.CreateAccount(ctx, store.CreateAccountInput{
		ID:       "01ACC",
		Username: "alice",
		APID:     "https://example.com/users/alice",
	})
	require.NoError(t, err)
	hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	require.NoError(t, err)
	user, err := st.CreateUser(ctx, store.CreateUserInput{
		ID:           "01USER",
		AccountID:    acc.ID,
		Email:        "alice@example.com",
		PasswordHash: string(hash),
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	require.NoError(t, st.ConfirmUser(ctx, user.ID))

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/oauth/login", bytes.NewReader([]byte("not json")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.POSTLogin(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid client_id returns 400", func(t *testing.T) {
		body := map[string]string{
			"email":        "alice@example.com",
			"password":     "password",
			"client_id":    "nonexistent",
			"redirect_uri": "https://app.example/cb",
		}
		enc, _ := json.Marshal(body)
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/oauth/login", bytes.NewReader(enc))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.POSTLogin(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid credentials returns 401", func(t *testing.T) {
		body := map[string]string{
			"email":        "alice@example.com",
			"password":     "wrong",
			"client_id":    app.ClientID,
			"redirect_uri": "https://app.example/cb",
		}
		enc, _ := json.Marshal(body)
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/oauth/login", bytes.NewReader(enc))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.POSTLogin(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("happy path returns 200 with redirect_url", func(t *testing.T) {
		body := map[string]string{
			"email":        "alice@example.com",
			"password":     "password",
			"client_id":    app.ClientID,
			"redirect_uri": "https://app.example/cb",
			"scope":        "read",
			"state":        "abc",
		}
		enc, _ := json.Marshal(body)
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/oauth/login", bytes.NewReader(enc))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.POSTLogin(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var resp map[string]string
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
		assert.NotEmpty(t, resp["redirect_url"])
		assert.Contains(t, resp["redirect_url"], "code=")
		assert.Contains(t, resp["redirect_url"], "state=abc")
	})
}
