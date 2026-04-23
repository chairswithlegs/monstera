package oauth

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/oauth"
	"github.com/chairswithlegs/monstera/internal/service"
)

// Handler holds dependencies for the OAuth HTTP endpoints.
type Handler struct {
	oauth         *oauth.Server
	auth          service.AuthService
	monsteraUIURL *url.URL
}

// NewHandler constructs an OAuth Handler.
func NewHandler(
	oauth *oauth.Server,
	auth service.AuthService,
	monsteraUIURL *url.URL,
) *Handler {
	return &Handler{
		oauth:         oauth,
		auth:          auth,
		monsteraUIURL: monsteraUIURL,
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
		if err := api.DecodeJSONBody(r, &body); err != nil {
			api.HandleError(w, r, err)
			return
		}
		name = strings.TrimSpace(body.ClientName)
		scopes = strings.TrimSpace(body.Scopes)
		website = strings.TrimSpace(body.Website)
		var ok bool
		redirectURIs, ok = redirectURIsToString(body.RedirectURIs)
		if !ok {
			api.HandleError(w, r, api.NewMissingRequiredFieldError("redirect_uris"))
			return
		}
	} else {
		if err := r.ParseForm(); err != nil { //nolint:gosec // G120: body size limited by global MaxBodySize middleware
			api.HandleError(w, r, api.NewInvalidRequestBodyError())
			return
		}
		name = strings.TrimSpace(r.Form.Get("client_name"))
		redirectURIs = strings.TrimSpace(r.Form.Get("redirect_uris"))
		scopes = r.Form.Get("scopes")
		website = r.Form.Get("website")
	}

	if name == "" {
		api.HandleError(w, r, api.NewMissingRequiredFieldError("client_name"))
		return
	}
	if redirectURIs == "" {
		api.HandleError(w, r, api.NewMissingRequiredFieldError("redirect_uris"))
		return
	}

	app, err := h.oauth.RegisterApplication(r.Context(), name, redirectURIs, scopes, website)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}

	slog.InfoContext(r.Context(), "oauth app registered",
		slog.String("client_id", app.ClientID),
		slog.String("name", app.Name))
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
		api.HandleError(w, r, api.NewInvalidValueError("response_type"))
		return
	}

	if codeChallengeMethod != "" && codeChallengeMethod != "S256" {
		api.HandleError(w, r, api.NewInvalidValueError("code_challenge_method"))
		return
	}

	app, err := h.auth.GetApplicationByClientID(r.Context(), clientID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.NewInvalidValueError("client_id"))
			return
		}
		api.HandleError(w, r, err)
		return
	}

	if !h.auth.ValidateRedirectURI(redirectURI, app) {
		api.HandleError(w, r, api.NewInvalidValueError("redirect_uri"))
		return
	}

	// Forward all OAuth params to the Monstera UI OAuth authorize page
	authorizeURL := h.monsteraUIURL.JoinPath("/oauth/authorize")
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
	if err := api.DecodeJSONBody(r, &body); err != nil {
		api.HandleError(w, r, err)
		return
	}

	app, err := h.auth.GetApplicationByClientID(r.Context(), body.ClientID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.NewInvalidValueError("client_id"))
			return
		}
		api.HandleError(w, r, err)
		return
	}

	if !h.auth.ValidateRedirectURI(body.RedirectURI, app) {
		api.HandleError(w, r, api.NewInvalidValueError("redirect_uri"))
		return
	}

	accountID, err := h.auth.Authenticate(r.Context(), body.Email, body.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUnconfirmed):
			api.HandleError(w, r, api.NewUnconfirmedError())
		case errors.Is(err, service.ErrSuspended):
			api.HandleError(w, r, api.NewSuspendedError())
		default:
			api.HandleError(w, r, api.NewInvalidEmailOrPasswordError())
		}
		return
	}

	code, err := h.oauth.AuthorizeRequest(r.Context(), oauth.AuthorizeRequest{
		ApplicationID:       app.ID,
		AccountID:           accountID,
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
		api.HandleError(w, r, api.NewInvalidValueError("redirect_uri"))
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

// tokenRequestBody is the JSON body for POST /oauth/token (some clients send JSON instead of form).
type tokenRequestBody struct {
	GrantType    string `json:"grant_type"`
	Code         string `json:"code"`
	RedirectURI  string `json:"redirect_uri"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	CodeVerifier string `json:"code_verifier"`
	Scope        string `json:"scope"`
}

// POSTToken handles POST /oauth/token. Accepts application/x-www-form-urlencoded or application/json.
func (h *Handler) POSTToken(w http.ResponseWriter, r *http.Request) {
	var grantType, code, redirectURI, clientID, clientSecret, codeVerifier, scope string

	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") {
		var body tokenRequestBody
		if err := api.DecodeJSONBody(r, &body); err != nil {
			api.HandleError(w, r, err)
			return
		}
		grantType = strings.TrimSpace(body.GrantType)
		code = body.Code
		redirectURI = body.RedirectURI
		clientID = body.ClientID
		clientSecret = body.ClientSecret
		codeVerifier = body.CodeVerifier
		scope = body.Scope
	} else {
		if err := r.ParseForm(); err != nil { //nolint:gosec // G120: body size limited by global MaxBodySize middleware
			api.HandleError(w, r, api.NewInvalidRequestBodyError())
			return
		}
		grantType = r.Form.Get("grant_type")
		code = r.Form.Get("code")
		redirectURI = r.Form.Get("redirect_uri")
		clientID = r.Form.Get("client_id")
		clientSecret = r.Form.Get("client_secret")
		codeVerifier = r.Form.Get("code_verifier")
		scope = r.Form.Get("scope")
	}

	switch grantType {
	case "authorization_code":
		resp, err := h.oauth.ExchangeCode(r.Context(), oauth.TokenRequest{
			GrantType:    grantType,
			Code:         code,
			RedirectURI:  redirectURI,
			ClientID:     clientID,
			ClientSecret: clientSecret,
			CodeVerifier: codeVerifier,
		})
		if err != nil {
			api.HandleError(w, r, err)
			return
		}
		api.WriteJSON(w, http.StatusOK, resp)

	case "client_credentials":
		resp, err := h.oauth.ExchangeClientCredentials(r.Context(), oauth.TokenRequest{
			GrantType:    grantType,
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Scopes:       scope,
		})
		if err != nil {
			api.HandleError(w, r, err)
			return
		}
		api.WriteJSON(w, http.StatusOK, resp)

	default:
		api.HandleError(w, r, api.NewUnsupportedGrantTypeError(grantType))
	}
}

// POSTRevoke handles POST /oauth/revoke (RFC 7009).
func (h *Handler) POSTRevoke(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil { //nolint:gosec // G120: body size limited by global MaxBodySize middleware
		api.HandleError(w, r, api.NewInvalidRequestBodyError())
		return
	}

	token := r.Form.Get("token")
	if err := api.ValidateRequiredField(token, "token"); err != nil {
		api.HandleError(w, r, err)
		return
	}

	_ = h.oauth.RevokeToken(r.Context(), token)

	api.WriteJSON(w, http.StatusOK, struct{}{})
}

// GETVerifyCredentials handles GET /api/v1/apps/verify_credentials.
// Returns public metadata about the application the current access token was
// issued for. Secrets (client_id, client_secret) are not included.
func (h *Handler) GETVerifyCredentials(w http.ResponseWriter, r *http.Request) {
	claims := middleware.TokenClaimsFromContext(r.Context())
	if claims == nil || claims.ApplicationID == "" {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	info, err := h.oauth.GetApplicationInfo(r.Context(), claims.ApplicationID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrUnauthorized)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, info)
}

// GETAuthorizedApplications handles GET /api/v1/oauth/authorized_applications.
// Returns one entry per OAuth application that currently holds an active
// token for the authenticated account.
func (h *Handler) GETAuthorizedApplications(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	apps, err := h.oauth.ListAuthorizedApplications(r.Context(), account.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apps)
}

// DELETEAuthorizedApplication handles DELETE /api/v1/oauth/authorized_applications/:id.
// Revokes every active access token the authenticated account holds for the
// given application id.
func (h *Handler) DELETEAuthorizedApplication(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if err := api.ValidateRequiredField(id, "id"); err != nil {
		api.HandleError(w, r, err)
		return
	}
	if err := h.oauth.RevokeApplicationAuthorization(r.Context(), account.ID, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, struct{}{})
}
