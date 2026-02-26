package oauth

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	oauthpkg "github.com/chairswithlegs/monstera-fed/internal/oauth"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	"golang.org/x/crypto/bcrypt"
)

//go:embed templates/login.html
var templatesFS embed.FS

// Handler holds dependencies for the OAuth HTTP endpoints.
type Handler struct {
	oauth        *oauthpkg.Server
	store        store.Store
	logger       *slog.Logger
	loginTmpl    *template.Template
	instanceName string
	secretKey    []byte
}

// ParseLoginTemplate parses the embedded login template.
func ParseLoginTemplate() (*template.Template, error) {
	data, err := templatesFS.ReadFile("templates/login.html")
	if err != nil {
		return nil, fmt.Errorf("read login template: %w", err)
	}
	tmpl, err := template.New("login").Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse login template: %w", err)
	}
	return tmpl, nil
}

// NewHandler constructs an OAuth Handler.
func NewHandler(
	oauth *oauthpkg.Server,
	store store.Store,
	logger *slog.Logger,
	loginTmpl *template.Template,
	instanceName string,
	secretKey []byte,
) *Handler {
	return &Handler{
		oauth:        oauth,
		store:        store,
		logger:       logger,
		loginTmpl:    loginTmpl,
		instanceName: instanceName,
		secretKey:    secretKey,
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
			api.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		name = strings.TrimSpace(body.ClientName)
		scopes = strings.TrimSpace(body.Scopes)
		website = strings.TrimSpace(body.Website)
		var ok bool
		redirectURIs, ok = redirectURIsToString(body.RedirectURIs)
		if !ok {
			api.WriteError(w, http.StatusUnprocessableEntity, "redirect_uris is required")
			return
		}
	} else {
		if err := r.ParseForm(); err != nil {
			api.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		name = strings.TrimSpace(r.FormValue("client_name"))
		redirectURIs = strings.TrimSpace(r.FormValue("redirect_uris"))
		scopes = r.FormValue("scopes")
		website = r.FormValue("website")
	}

	if name == "" {
		api.WriteError(w, http.StatusUnprocessableEntity, "client_name is required")
		return
	}
	if redirectURIs == "" {
		api.WriteError(w, http.StatusUnprocessableEntity, "redirect_uris is required")
		return
	}

	app, err := h.oauth.RegisterApplication(r.Context(), name, redirectURIs, scopes, website)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "register application failed", slog.Any("error", err))
		api.WriteError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	api.WriteJSON(w, http.StatusOK, app)
}

// loginPageData is the template data for login.html.
type loginPageData struct {
	InstanceName        string
	AppName             string
	Scopes              string
	ClientID            string
	RedirectURI         string
	ResponseType        string
	Scope               string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	Error               string
}

// GETAuthorize handles GET /oauth/authorize.
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
		api.WriteError(w, http.StatusBadRequest, "response_type must be 'code'")
		return
	}

	if codeChallengeMethod != "" && codeChallengeMethod != "S256" {
		api.WriteError(w, http.StatusBadRequest, "code_challenge_method must be 'S256'")
		return
	}

	app, err := h.store.GetApplicationByClientID(r.Context(), clientID)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "invalid client_id")
		return
	}

	if !isValidRedirectURI(redirectURI, app.RedirectURIs) {
		api.WriteError(w, http.StatusBadRequest, "redirect_uri is not registered")
		return
	}

	data := loginPageData{
		InstanceName:        h.instanceName,
		AppName:             app.Name,
		Scopes:              scope,
		ClientID:            clientID,
		RedirectURI:         redirectURI,
		ResponseType:        responseType,
		Scope:               scope,
		State:               state,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: codeChallengeMethod,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := h.loginTmpl.Execute(w, data); err != nil {
		h.logger.Error("render login template", slog.Any("error", err))
	}
}

// POSTAuthorizeSubmit handles POST /oauth/authorize.
func (h *Handler) POSTAuthorizeSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		api.WriteError(w, http.StatusBadRequest, "invalid form data")
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")
	clientID := r.FormValue("client_id")
	redirectURI := r.FormValue("redirect_uri")
	scope := r.FormValue("scope")
	state := r.FormValue("state")
	codeChallenge := r.FormValue("code_challenge")
	codeChallengeMethod := r.FormValue("code_challenge_method")

	app, err := h.store.GetApplicationByClientID(r.Context(), clientID)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "invalid client_id")
		return
	}

	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil {
		h.renderLoginError(w, "Invalid email or password", app.Name, clientID, redirectURI, scope, state, codeChallenge, codeChallengeMethod)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		h.renderLoginError(w, "Invalid email or password", app.Name, clientID, redirectURI, scope, state, codeChallenge, codeChallengeMethod)
		return
	}

	if user.ConfirmedAt == nil {
		h.renderLoginError(w, "Please confirm your email address before signing in", app.Name, clientID, redirectURI, scope, state, codeChallenge, codeChallengeMethod)
		return
	}

	account, err := h.store.GetAccountByID(r.Context(), user.AccountID)
	if err != nil || account.Suspended {
		h.renderLoginError(w, "Your account has been suspended", app.Name, clientID, redirectURI, scope, state, codeChallenge, codeChallengeMethod)
		return
	}

	code, err := h.oauth.AuthorizeRequest(r.Context(), oauthpkg.AuthorizeRequest{
		ApplicationID:       app.ID,
		AccountID:           user.AccountID,
		RedirectURI:         redirectURI,
		Scopes:              scope,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: codeChallengeMethod,
	})
	if err != nil {
		h.logger.ErrorContext(r.Context(), "authorize request failed", slog.Any("error", err))
		api.WriteError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	redirectURL, _ := url.Parse(redirectURI)
	redirectQuery := redirectURL.Query()
	redirectQuery.Set("code", code)
	if state != "" {
		redirectQuery.Set("state", state)
	}
	redirectURL.RawQuery = redirectQuery.Encode()

	http.Redirect(w, r, redirectURL.String(), http.StatusFound)
}

// POSTToken handles POST /oauth/token.
func (h *Handler) POSTToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		api.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	grantType := r.FormValue("grant_type")

	switch grantType {
	case "authorization_code":
		resp, err := h.oauth.ExchangeCode(r.Context(), oauthpkg.TokenRequest{
			GrantType:    grantType,
			Code:         r.FormValue("code"),
			RedirectURI:  r.FormValue("redirect_uri"),
			ClientID:     r.FormValue("client_id"),
			ClientSecret: r.FormValue("client_secret"),
			CodeVerifier: r.FormValue("code_verifier"),
		})
		if err != nil {
			api.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		api.WriteJSON(w, http.StatusOK, resp)

	case "client_credentials":
		resp, err := h.oauth.ExchangeClientCredentials(r.Context(), oauthpkg.TokenRequest{
			GrantType:    grantType,
			ClientID:     r.FormValue("client_id"),
			ClientSecret: r.FormValue("client_secret"),
			Scopes:       r.FormValue("scope"),
		})
		if err != nil {
			api.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		api.WriteJSON(w, http.StatusOK, resp)

	default:
		api.WriteError(w, http.StatusBadRequest, "unsupported grant_type")
	}
}

// POSTRevoke handles POST /oauth/revoke (RFC 7009).
func (h *Handler) POSTRevoke(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		api.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	token := r.FormValue("token")
	if token == "" {
		api.WriteError(w, http.StatusBadRequest, "token is required")
		return
	}

	_ = h.oauth.RevokeToken(r.Context(), token)

	api.WriteJSON(w, http.StatusOK, struct{}{})
}

func isValidRedirectURI(uri, registered string) bool {
	if uri == "urn:ietf:wg:oauth:2.0:oob" {
		return true
	}
	for _, r := range strings.Split(registered, "\n") {
		if strings.TrimSpace(r) == uri {
			return true
		}
	}
	return false
}

func (h *Handler) renderLoginError(w http.ResponseWriter, errMsg, appName, clientID, redirectURI, scope, state, codeChallenge, codeChallengeMethod string) {
	data := loginPageData{
		InstanceName:        h.instanceName,
		AppName:             appName,
		Scopes:              scope,
		ClientID:            clientID,
		RedirectURI:         redirectURI,
		Scope:               scope,
		State:               state,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: codeChallengeMethod,
		Error:               errMsg,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if err := h.loginTmpl.Execute(w, data); err != nil {
		h.logger.Error("render login template", slog.Any("error", err))
	}
}
