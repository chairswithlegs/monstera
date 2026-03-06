package oauth

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/config"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/oauth"
	"github.com/chairswithlegs/monstera/internal/store"
	"golang.org/x/crypto/bcrypt"
)

// Handler holds dependencies for the OAuth HTTP endpoints.
type Handler struct {
	oauth *oauth.Server
	store store.Store
	cfg   *config.Config
}

// NewHandler constructs an OAuth Handler.
func NewHandler(
	oauth *oauth.Server,
	store store.Store,
	cfg *config.Config,
) *Handler {
	return &Handler{
		oauth: oauth,
		store: store,
		cfg:   cfg,
	}
}

// registerAppRequest is the JSON body for POST /api/v1/apps (Mastodon allows redirect_uris as string or array).
type registerAppRequest struct {
	ClientName   string      `json:"client_name"`
	RedirectURIs interface{} `json:"redirect_uris"` // string or []string
	Scopes       string      `json:"scopes"`
	Website      string      `json:"website"`
}

// redirectURIsToString normalizes redirect_uris from JSON (string or []string) to newline-separated string.
func redirectURIsToString(v interface{}) (string, bool) {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x), x != ""
	case []interface{}:
		var parts []string
		for _, item := range x {
			if s, ok := item.(string); ok && s != "" {
				parts = append(parts, strings.TrimSpace(s))
			}
		}
		return strings.Join(parts, "\n"), len(parts) > 0
	default:
		return "", false
	}
}

// POSTRegisterApp handles POST /api/v1/apps. Accepts application/x-www-form-urlencoded or application/json.
func (h *Handler) POSTRegisterApp(w http.ResponseWriter, r *http.Request) {
	var name, redirectURIs, scopes, website string

	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") {
		var body registerAppRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			api.HandleError(w, r, api.NewBadRequestError("invalid request body"))
			return
		}
		name = strings.TrimSpace(body.ClientName)
		scopes = strings.TrimSpace(body.Scopes)
		website = strings.TrimSpace(body.Website)
		var ok bool
		redirectURIs, ok = redirectURIsToString(body.RedirectURIs)
		if !ok {
			api.HandleError(w, r, api.NewBadRequestError("redirect_uris is required"))
			return
		}
	} else {
		if err := r.ParseForm(); err != nil {
			api.HandleError(w, r, api.NewBadRequestError("invalid request body"))
			return
		}
		name = strings.TrimSpace(r.FormValue("client_name"))
		redirectURIs = strings.TrimSpace(r.FormValue("redirect_uris"))
		scopes = r.FormValue("scopes")
		website = r.FormValue("website")
	}

	if name == "" {
		api.HandleError(w, r, api.NewBadRequestError("client_name is required"))
		return
	}
	if redirectURIs == "" {
		api.HandleError(w, r, api.NewBadRequestError("redirect_uris is required"))
		return
	}

	app, err := h.oauth.RegisterApplication(r.Context(), name, redirectURIs, scopes, website)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, app)
}

// GETAuthorize handles GET /oauth/authorize.
// Redirects to the UI's OAuth authorize page with all OAuth params.
func (h *Handler) GETAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	clientID := q.Get("client_id")
	redirectURI := q.Get("redirect_uri")
	responseType := q.Get("response_type")
	scope := q.Get("scope")
	state := q.Get("state")
	codeChallenge := q.Get("code_challenge")
	codeChallengeMethod := q.Get("code_challenge_method")

	if scope == "" {
		scope = "read"
	}

	if responseType != "code" {
		api.HandleError(w, r, api.NewBadRequestError("response_type must be 'code'"))
		return
	}

	if codeChallengeMethod != "" && codeChallengeMethod != "S256" {
		api.HandleError(w, r, api.NewBadRequestError("code_challenge_method must be 'S256'"))
		return
	}

	app, err := h.store.GetApplicationByClientID(r.Context(), clientID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.NewBadRequestError("invalid client_id"))
			return
		}
		api.HandleError(w, r, err)
		return
	}

	if !h.isValidRedirectURI(redirectURI, app) {
		api.HandleError(w, r, api.NewBadRequestError("redirect_uri is not registered"))
		return
	}

	// Forward all OAuth params to the Monstera UI OAuth authorize page
	authorizeURL := h.cfg.MonsteraUiUrl.JoinPath("/oauth/authorize")
	authorizeQuery := authorizeURL.Query()
	authorizeQuery.Set("client_id", clientID)
	authorizeQuery.Set("redirect_uri", redirectURI)
	authorizeQuery.Set("response_type", responseType)
	authorizeQuery.Set("scope", scope)
	authorizeQuery.Set("code_challenge", codeChallenge)
	authorizeQuery.Set("code_challenge_method", codeChallengeMethod)
	if state != "" {
		authorizeQuery.Set("state", state)
	}
	authorizeQuery.Set("app_name", app.Name)
	if app.Website != nil {
		authorizeQuery.Set("app_website", *app.Website)
	}
	authorizeURL.RawQuery = authorizeQuery.Encode()

	http.Redirect(w, r, authorizeURL.String(), http.StatusFound)
}

type loginRequest struct {
	Email               string `json:"email"`
	Password            string `json:"password"`
	ClientID            string `json:"client_id"`
	RedirectURI         string `json:"redirect_uri"`
	Scope               string `json:"scope"`
	State               string `json:"state"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
}

// POSTLogin handles POST /oauth/login.
// Validates credentials and authorizes the application.
func (h *Handler) POSTLogin(w http.ResponseWriter, r *http.Request) {
	var body loginRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.HandleError(w, r, api.NewBadRequestError("invalid request body"))
		return
	}

	app, err := h.store.GetApplicationByClientID(r.Context(), body.ClientID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.NewBadRequestError("invalid client_id"))
			return
		}
		api.HandleError(w, r, err)
		return
	}

	if !h.isValidRedirectURI(body.RedirectURI, app) {
		api.HandleError(w, r, api.NewBadRequestError("redirect_uri is not registered"))
		return
	}

	user, err := h.store.GetUserByEmail(r.Context(), body.Email)
	if err != nil {
		api.HandleError(w, r, api.NewUnauthorizedError("invalid email or password"))
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.Password)); err != nil {
		api.HandleError(w, r, api.NewUnauthorizedError("invalid email or password"))
		return
	}

	if user.ConfirmedAt == nil {
		api.HandleError(w, r, api.NewUnauthorizedError("please confirm your email address before signing in"))
		return
	}

	account, err := h.store.GetAccountByID(r.Context(), user.AccountID)
	if err != nil || account.Suspended {
		api.HandleError(w, r, api.NewUnauthorizedError("your account has been suspended"))
		return
	}

	code, err := h.oauth.AuthorizeRequest(r.Context(), oauth.AuthorizeRequest{
		ApplicationID:       app.ID,
		AccountID:           user.AccountID,
		RedirectURI:         body.RedirectURI,
		Scopes:              body.Scope,
		CodeChallenge:       body.CodeChallenge,
		CodeChallengeMethod: body.CodeChallengeMethod,
	})
	if err != nil {
		api.HandleError(w, r, err)
		return
	}

	redirectURL, err := url.Parse(body.RedirectURI)
	if err != nil {
		api.HandleError(w, r, api.NewBadRequestError("invalid redirect_uri"))
		return
	}
	redirectQuery := redirectURL.Query()
	redirectQuery.Set("code", code)
	if body.State != "" {
		redirectQuery.Set("state", body.State)
	}
	redirectURL.RawQuery = redirectQuery.Encode()
	api.WriteJSON(w, http.StatusOK, map[string]string{"redirect_url": redirectURL.String()})
}

// POSTToken handles POST /oauth/token.
func (h *Handler) POSTToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		api.HandleError(w, r, api.NewBadRequestError("invalid request body"))
		return
	}

	grantType := r.FormValue("grant_type")

	switch grantType {
	case "authorization_code":
		resp, err := h.oauth.ExchangeCode(r.Context(), oauth.TokenRequest{
			GrantType:    grantType,
			Code:         r.FormValue("code"),
			RedirectURI:  r.FormValue("redirect_uri"),
			ClientID:     r.FormValue("client_id"),
			ClientSecret: r.FormValue("client_secret"),
			CodeVerifier: r.FormValue("code_verifier"),
		})
		if err != nil {
			api.HandleError(w, r, err)
			return
		}
		api.WriteJSON(w, http.StatusOK, resp)

	case "client_credentials":
		resp, err := h.oauth.ExchangeClientCredentials(r.Context(), oauth.TokenRequest{
			GrantType:    grantType,
			ClientID:     r.FormValue("client_id"),
			ClientSecret: r.FormValue("client_secret"),
			Scopes:       r.FormValue("scope"),
		})
		if err != nil {
			api.HandleError(w, r, err)
			return
		}
		api.WriteJSON(w, http.StatusOK, resp)

	default:
		api.HandleError(w, r, api.NewBadRequestError("unsupported grant_type"))
	}
}

// POSTRevoke handles POST /oauth/revoke (RFC 7009).
func (h *Handler) POSTRevoke(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		api.HandleError(w, r, api.NewBadRequestError("invalid request body"))
		return
	}

	token := r.FormValue("token")
	if err := api.ValidateRequiredString(token); err != nil {
		api.HandleError(w, r, err)
		return
	}

	_ = h.oauth.RevokeToken(r.Context(), token)

	api.WriteJSON(w, http.StatusOK, struct{}{})
}

func (h *Handler) isValidRedirectURI(uri string, app *domain.OAuthApplication) bool {
	if app == nil {
		slog.Error("application is nil")
		return false
	}

	if uri == "urn:ietf:wg:oauth:2.0:oob" {
		return true
	}

	parsedURI, err := url.Parse(uri)
	if err != nil {
		slog.Error("failed to parse redirect URI", slog.Any("error", err))
		return false
	}

	if app.ClientID == oauth.MONSTERA_UI_APPLICATION_ID && parsedURI.Host == h.cfg.MonsteraUiUrl.Host {
		slog.Info("valid internal redirect URI", slog.String("uri", uri))
		return true
	}

	for _, r := range strings.Split(app.RedirectURIs, "\n") {
		if strings.TrimSpace(r) == uri {
			return true
		}
	}
	slog.Info("Fall through case")
	return false
}
