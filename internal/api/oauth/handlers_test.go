package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/cache"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/oauth"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
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

	t.Run("missing client_name returns 422", func(t *testing.T) {
		body := map[string]string{"redirect_uris": "https://app.example/cb", "scopes": "read"}
		enc, _ := json.Marshal(body)
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/apps", bytes.NewReader(enc))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.POSTRegisterApp(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("missing redirect_uris returns 422", func(t *testing.T) {
		body := map[string]string{"client_name": "My App", "scopes": "read"}
		enc, _ := json.Marshal(body)
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/apps", bytes.NewReader(enc))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.POSTRegisterApp(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
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

	t.Run("response_type not code returns 422", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/oauth/authorize?response_type=token&client_id=foo&redirect_uri=https://app.example/cb", nil)
		rec := httptest.NewRecorder()
		h.GETAuthorize(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("invalid client_id returns 422", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/oauth/authorize?response_type=code&client_id=nonexistent&redirect_uri=https://app.example/cb", nil)
		rec := httptest.NewRecorder()
		h.GETAuthorize(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
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

func TestHandler_GETVerifyCredentials(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	c, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	website := "https://client.example"
	app, err := st.CreateApplication(context.Background(), store.CreateApplicationInput{
		ID: "app_vc_1", Name: "Mona", ClientID: "mona_cid", ClientSecret: "mona_secret",
		RedirectURIs: "monstera://cb", Scopes: "read", Website: &website,
	})
	require.NoError(t, err)

	srv := oauth.NewServer(st, c, "vapid-123")
	h := NewHandler(srv, service.NewAuthService(st, "", ""), nil)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/apps/verify_credentials", nil)
		rec := httptest.NewRecorder()
		h.GETVerifyCredentials(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("happy path omits secrets", func(t *testing.T) {
		ctx := middleware.WithTokenClaims(context.Background(), &oauth.TokenClaims{
			AccessTokenID: "tok_vc_1", ApplicationID: app.ID, AccountID: "", Scopes: oauth.Parse("read"),
		})
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/apps/verify_credentials", nil)
		rec := httptest.NewRecorder()
		h.GETVerifyCredentials(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		var out map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
		assert.Equal(t, app.ID, out["id"])
		assert.Equal(t, "Mona", out["name"])
		assert.Equal(t, "vapid-123", out["vapid_key"])
		assert.NotContains(t, out, "client_id")
		assert.NotContains(t, out, "client_secret")
	})
}

func TestHandler_GETAuthorizedApplications(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	c, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()
	srv := oauth.NewServer(st, c, "")
	h := NewHandler(srv, service.NewAuthService(st, "", ""), nil)

	accountID := "acct_list_1"
	app, err := st.CreateApplication(context.Background(), store.CreateApplicationInput{
		ID: "app_list_1", Name: "Ice Cubes", ClientID: "ic_cid", ClientSecret: "ic_secret",
		RedirectURIs: "monstera://cb", Scopes: "read write",
	})
	require.NoError(t, err)
	_, err = st.CreateAccessToken(context.Background(), store.CreateAccessTokenInput{
		ID: "tok_list_1", ApplicationID: app.ID, AccountID: &accountID,
		Token: "ic_raw_token", Scopes: "read write",
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/oauth/authorized_applications", nil)
		rec := httptest.NewRecorder()
		h.GETAuthorizedApplications(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("happy path returns list", func(t *testing.T) {
		acc := &domain.Account{ID: accountID}
		ctx := middleware.WithAccount(context.Background(), acc)
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/oauth/authorized_applications", nil)
		rec := httptest.NewRecorder()
		h.GETAuthorizedApplications(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		var apps []map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &apps))
		require.Len(t, apps, 1)
		assert.Equal(t, app.ID, apps[0]["id"])
		assert.Equal(t, "Ice Cubes", apps[0]["name"])
	})
}

func TestHandler_DELETEAuthorizedApplication(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	c, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()
	srv := oauth.NewServer(st, c, "")
	h := NewHandler(srv, service.NewAuthService(st, "", ""), nil)

	accountID := "acct_del_1"
	app, err := st.CreateApplication(context.Background(), store.CreateApplicationInput{
		ID: "app_del_1", Name: "Tusky", ClientID: "tusky_cid", ClientSecret: "tusky_secret",
		RedirectURIs: "monstera://cb", Scopes: "read write",
	})
	require.NoError(t, err)
	_, err = st.CreateAccessToken(context.Background(), store.CreateAccessTokenInput{
		ID: "tok_del_1", ApplicationID: app.ID, AccountID: &accountID,
		Token: "tusky_raw_token", Scopes: "read write",
	})
	require.NoError(t, err)

	withAuthAndParam := func(t *testing.T, id string) *http.Request {
		t.Helper()
		acc := &domain.Account{ID: accountID}
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", id)
		ctx := middleware.WithAccount(context.Background(), acc)
		ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
		return httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/oauth/authorized_applications/"+id, nil)
	}

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/oauth/authorized_applications/"+app.ID, nil)
		rec := httptest.NewRecorder()
		h.DELETEAuthorizedApplication(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("unknown app returns 404", func(t *testing.T) {
		req := withAuthAndParam(t, "app_does_not_exist")
		rec := httptest.NewRecorder()
		h.DELETEAuthorizedApplication(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("happy path returns 200 and subsequent delete is 404", func(t *testing.T) {
		req := withAuthAndParam(t, app.ID)
		rec := httptest.NewRecorder()
		h.DELETEAuthorizedApplication(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		req = withAuthAndParam(t, app.ID)
		rec = httptest.NewRecorder()
		h.DELETEAuthorizedApplication(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
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

	t.Run("invalid client_id returns 422", func(t *testing.T) {
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
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
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
