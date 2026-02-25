# ADR 06 — OAuth 2.0 & Authentication

> **Status:** Draft v0.1 — Feb 24, 2026  
> **Addresses:** `design-prompts/06-oauth-and-authentication.md`

---

## Design Decisions (answered before authoring)

| Question | Decision |
|----------|----------|
| PKCE `plain` method | **Rejected** — only `S256` is accepted. `plain` provides no security benefit and no major Mastodon client uses it. Return `invalid_request` if `code_challenge_method` is anything other than `S256`. |
| Access token format | **Opaque** — 32 bytes `crypto/rand`, hex-encoded (64 chars). Not JWT. Mastodon clients cache raw token strings and expect them to work indefinitely. |
| Token expiry | **Non-expiring by default** (Mastodon convention). `expires_at` is NULL. Tokens are revoked explicitly via `POST /oauth/revoke` or by admin suspension. Token cache TTL (24h) provides periodic revalidation against the DB. |
| App-only token scopes | `client_credentials` grant yields a token with `account_id = NULL` and scopes limited to `read` (no `write`, no `admin:*`). Used by clients that need instance metadata before user login. |
| Authorization code format | 32 bytes `crypto/rand`, hex-encoded (64 chars). Stored raw in DB (short-lived; no hash needed). |
| Authorization code TTL | **10 minutes**, single-use — row deleted on exchange. |
| `state` parameter | Passed through; not stored server-side. Clients validate it themselves (CSRF protection on the client side). |
| Login session for authorize flow | **Short-lived signed cookie** — HMAC-SHA256 with `SECRET_KEY_BASE`, 10-minute expiry. Only lives between form submission and redirect. |
| Consent screen | **Implicit consent** — after login, the code is issued immediately. No separate "Allow/Deny" step. Mastodon clients expect this behavior; the user authorized the app by entering credentials. |
| HTTP Signature algorithm | `rsa-sha256` (draft-cavage-http-signatures-12) — Mastodon's de facto standard. |
| HTTP Signature replay prevention | Cache key `httpsig:{sha256(keyId+date+requestTarget)}`, TTL 60s. |
| HTTP Signature clock skew | **±30 seconds** — reject requests with `Date` header outside this window. |

---

## File Layout

```
internal/oauth/
├── server.go       — Server struct, RegisterApplication, AuthorizeRequest, ExchangeCode,
│                     RevokeToken, LookupToken
├── pkce.go         — ValidatePKCE, GenerateCodeChallenge (test helper)
└── scopes.go       — ScopeSet type, Parse, HasScope, Normalize, expansion table

internal/ap/
└── httpsig.go      — Verify, Sign, KeyFetcher, signed headers, digest, replay prevention

internal/api/
├── oauth/
│   ├── handlers.go — RegisterApp, Authorize (GET+POST), Token, Revoke handlers
│   └── templates/  — login.html (embedded)
└── middleware/
    └── auth.go     — RequireAuth, OptionalAuth, RequiredScopes, context helpers
```

---

## 1. `internal/oauth/scopes.go`

```go
package oauth

import (
	"slices"
	"strings"
)

// scopeExpansion maps parent scopes to their children. A token with "read"
// is treated as having all "read:*" scopes. This matches Mastodon's behaviour.
var scopeExpansion = map[string][]string{
	"read": {
		"read:accounts",
		"read:statuses",
		"read:notifications",
		"read:blocks",
		"read:filters",
		"read:follows",
		"read:lists",
		"read:mutes",
		"read:search",
		"read:favourites",
		"read:bookmarks",
	},
	"write": {
		"write:accounts",
		"write:statuses",
		"write:media",
		"write:follows",
		"write:notifications",
		"write:blocks",
		"write:filters",
		"write:lists",
		"write:mutes",
		"write:favourites",
		"write:bookmarks",
		"write:conversations",
		"write:reports",
	},
	"admin:read": {
		"admin:read:accounts",
		"admin:read:reports",
		"admin:read:domain_allows",
		"admin:read:domain_blocks",
		"admin:read:ip_blocks",
		"admin:read:email_domain_blocks",
		"admin:read:canonical_email_blocks",
	},
	"admin:write": {
		"admin:write:accounts",
		"admin:write:reports",
		"admin:write:domain_allows",
		"admin:write:domain_blocks",
		"admin:write:ip_blocks",
		"admin:write:email_domain_blocks",
		"admin:write:canonical_email_blocks",
	},
	"follow": {
		"read:follows",
		"write:follows",
		"read:blocks",
		"write:blocks",
		"read:mutes",
		"write:mutes",
	},
}

// allKnownScopes is the set of every valid scope string (top-level + children).
// Built once at package init time.
var allKnownScopes map[string]bool

func init() {
	allKnownScopes = make(map[string]bool)
	for parent, children := range scopeExpansion {
		allKnownScopes[parent] = true
		for _, c := range children {
			allKnownScopes[c] = true
		}
	}
	allKnownScopes["push"] = true
}

// ScopeSet is the resolved set of scopes carried by a token.
// Implemented as a map for O(1) lookup.
type ScopeSet map[string]bool

// Parse splits a space-separated scope string, expands parent scopes, and
// returns the fully resolved ScopeSet.
//
// Unknown scopes are silently dropped. This is intentional: Mastodon clients
// sometimes request scopes that a server doesn't support (e.g. a Phase 2
// scope). Dropping them avoids hard failures while the granted token correctly
// reflects what the server actually supports.
func Parse(raw string) ScopeSet {
	ss := make(ScopeSet)
	for _, s := range strings.Fields(raw) {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !allKnownScopes[s] {
			continue
		}
		ss[s] = true
		if children, ok := scopeExpansion[s]; ok {
			for _, c := range children {
				ss[c] = true
			}
		}
	}
	return ss
}

// HasScope returns true if the set contains the required scope, accounting
// for scope expansion. For example, if the set contains "read", then
// HasScope("read:statuses") returns true.
func (s ScopeSet) HasScope(required string) bool {
	return s[required]
}

// HasAll returns true if the set contains every scope in required.
func (s ScopeSet) HasAll(required ...string) bool {
	for _, r := range required {
		if !s[r] {
			return false
		}
	}
	return true
}

// String returns the canonical space-separated, alphabetically sorted
// representation of all scopes in the set.
func (s ScopeSet) String() string {
	var out []string
	for scope := range s {
		out = append(out, scope)
	}
	slices.Sort(out)
	return strings.Join(out, " ")
}

// Normalize expands parent scopes and returns a canonical sorted string.
// Used when storing scopes in the database to ensure consistent comparisons.
func Normalize(raw string) string {
	return Parse(raw).String()
}

// Intersect returns a new ScopeSet containing only scopes present in both
// sets. Used to restrict a token's scopes to the intersection of what the
// application registered and what the user authorized.
func (s ScopeSet) Intersect(other ScopeSet) ScopeSet {
	result := make(ScopeSet)
	for scope := range s {
		if other[scope] {
			result[scope] = true
		}
	}
	return result
}
```

---

## 2. `internal/oauth/pkce.go`

```go
package oauth

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// ValidatePKCE verifies a PKCE code_verifier against the stored code_challenge.
//
// Only the S256 method is supported. The verification computes:
//
//	base64url_no_pad(sha256(code_verifier)) == code_challenge
//
// If code_challenge is empty (no PKCE was used during authorization), the
// verifier is not checked and nil is returned. This allows the non-PKCE
// Authorization Code flow for server-side clients.
//
// Returns a non-nil error if:
//   - code_challenge_method is non-empty and not "S256"
//   - code_verifier is empty when a challenge was issued
//   - the computed challenge does not match
func ValidatePKCE(codeChallenge, codeChallengeMethod, codeVerifier string) error {
	if codeChallenge == "" {
		return nil
	}

	if codeChallengeMethod != "S256" {
		return fmt.Errorf("unsupported code_challenge_method %q: only S256 is supported", codeChallengeMethod)
	}

	if codeVerifier == "" {
		return fmt.Errorf("code_verifier is required when code_challenge is present")
	}

	h := sha256.Sum256([]byte(codeVerifier))
	computed := base64.RawURLEncoding.EncodeToString(h[:])

	if computed != codeChallenge {
		return fmt.Errorf("code_verifier does not match code_challenge")
	}

	return nil
}

// GenerateCodeChallenge is a test helper that computes the S256 challenge
// for a given verifier. Not used in production code.
func GenerateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
```

---

## 3. `internal/oauth/server.go`

### Types

```go
package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/yourorg/monstera-fed/internal/cache"
	"github.com/yourorg/monstera-fed/internal/store"
	db "github.com/yourorg/monstera-fed/internal/store/postgres/generated"
	"github.com/yourorg/monstera-fed/internal/uid"
)

// AuthorizeRequest represents the validated parameters from GET /oauth/authorize
// that have been confirmed by the user (i.e. the user has logged in).
type AuthorizeRequest struct {
	ApplicationID        string
	AccountID            string
	RedirectURI          string
	Scopes               string
	CodeChallenge        string
	CodeChallengeMethod  string
	State                string // pass-through; not stored
}

// TokenRequest represents the parameters from POST /oauth/token.
type TokenRequest struct {
	GrantType    string // "authorization_code" | "client_credentials"
	Code         string // for authorization_code
	RedirectURI  string // for authorization_code
	ClientID     string
	ClientSecret string
	CodeVerifier string // PKCE
	Scopes       string // for client_credentials
}

// TokenResponse is the response payload for POST /oauth/token.
// Field names and JSON tags match RFC 6749 and the Mastodon API spec.
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"` // always "Bearer"
	Scope       string `json:"scope"`
	CreatedAt   int64  `json:"created_at"` // Unix timestamp (Mastodon convention)
}

// TokenClaims is the resolved identity associated with a valid Bearer token.
// Stored in the request context by the auth middleware.
type TokenClaims struct {
	AccountID     string   // empty for app-only tokens
	ApplicationID string
	Scopes        ScopeSet
}

// OAuthApplication is the API response for POST /api/v1/apps.
type OAuthApplication struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURI  string `json:"redirect_uri"`
	VapidKey     string `json:"vapid_key"` // empty string — Phase 2 (Web Push)
}

// tokenCacheTTL is how long a resolved token is cached. Non-expiring tokens
// get a 24-hour cache entry; the next lookup after expiry re-validates against
// the DB. This bounds the window during which a revoked token remains usable.
const tokenCacheTTL = 24 * time.Hour

// authCodeTTL is the maximum lifetime of an authorization code.
const authCodeTTL = 10 * time.Minute
```

### Server

```go
// Server implements the OAuth 2.0 Authorization Server logic.
// HTTP handlers in internal/api/oauth/ delegate to this struct.
//
// Server is safe for concurrent use.
type Server struct {
	store  store.Store
	cache  cache.Store
	logger *slog.Logger
}

// NewServer constructs an OAuth Server.
func NewServer(s store.Store, c cache.Store, logger *slog.Logger) *Server {
	return &Server{store: s, cache: c, logger: logger}
}

// RegisterApplication creates a new OAuth application.
//
// client_id: 32 bytes crypto/rand, hex-encoded (64 chars).
// client_secret: 32 bytes crypto/rand, hex-encoded (64 chars).
//
// redirect_uris are stored as a newline-separated string (Mastodon convention).
// Scopes are normalized before storage.
//
// Returns the full application record including the generated credentials.
func (s *Server) RegisterApplication(ctx context.Context, name, redirectURIs, scopes, website string) (*OAuthApplication, error) {
	clientID, err := generateRandomHex(32)
	if err != nil {
		return nil, fmt.Errorf("oauth: generate client_id: %w", err)
	}
	clientSecret, err := generateRandomHex(32)
	if err != nil {
		return nil, fmt.Errorf("oauth: generate client_secret: %w", err)
	}

	if scopes == "" {
		scopes = "read"
	}
	scopes = Normalize(scopes)

	var ws *string
	if website != "" {
		ws = &website
	}

	app, err := s.store.CreateApplication(ctx, db.CreateApplicationParams{
		ID:           uid.New(),
		Name:         name,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectUris: redirectURIs,
		Scopes:       scopes,
		Website:      ws,
	})
	if err != nil {
		return nil, fmt.Errorf("oauth: create application: %w", err)
	}

	return &OAuthApplication{
		ID:           app.ID,
		Name:         app.Name,
		ClientID:     app.ClientID,
		ClientSecret: app.ClientSecret,
		RedirectURI:  app.RedirectUris,
		VapidKey:     "",
	}, nil
}

// AuthorizeRequest validates the authorization parameters, generates a
// single-use authorization code, stores it in the database, and returns
// the code string.
//
// Called after the user has authenticated (the handler verifies credentials
// and passes the resolved account_id).
//
// Authorization code lifecycle:
//   - Generated: 32 bytes crypto/rand, hex-encoded.
//   - Stored in oauth_authorization_codes with PKCE challenge (if provided).
//   - Expires after 10 minutes.
//   - Deleted on use in ExchangeCode (single-use enforcement).
//   - No "used_at" flag — deletion prevents replay entirely.
func (s *Server) AuthorizeRequest(ctx context.Context, req AuthorizeRequest) (code string, err error) {
	code, err = generateRandomHex(32)
	if err != nil {
		return "", fmt.Errorf("oauth: generate auth code: %w", err)
	}

	var challengePtr, methodPtr *string
	if req.CodeChallenge != "" {
		challengePtr = &req.CodeChallenge
		methodPtr = &req.CodeChallengeMethod
	}

	_, err = s.store.CreateAuthorizationCode(ctx, db.CreateAuthorizationCodeParams{
		ID:                   uid.New(),
		Code:                 code,
		ApplicationID:        req.ApplicationID,
		AccountID:            req.AccountID,
		RedirectUri:          req.RedirectURI,
		Scopes:               Normalize(req.Scopes),
		CodeChallenge:        challengePtr,
		CodeChallengeMethod:  methodPtr,
		ExpiresAt:            time.Now().Add(authCodeTTL),
	})
	if err != nil {
		return "", fmt.Errorf("oauth: store auth code: %w", err)
	}

	return code, nil
}

// ExchangeCode exchanges an authorization code for an access token.
//
// Validates:
//  1. The authorization code exists and has not expired (the DB query
//     checks `expires_at > NOW()`).
//  2. The client_id and client_secret match the application that created
//     the code.
//  3. The redirect_uri matches the one stored with the code.
//  4. The PKCE code_verifier (if a code_challenge was stored).
//
// On success: the authorization code row is deleted (single-use), a new
// access token is generated, stored, and returned.
//
// On any validation failure: returns an error. The authorization code is
// NOT deleted on failure — the client may retry with corrected parameters
// until the code expires.
func (s *Server) ExchangeCode(ctx context.Context, req TokenRequest) (*TokenResponse, error) {
	authCode, err := s.store.GetAuthorizationCode(ctx, req.Code)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired authorization code")
	}

	app, err := s.store.GetApplicationByClientID(ctx, req.ClientID)
	if err != nil {
		return nil, fmt.Errorf("invalid client_id")
	}

	if app.ClientSecret != req.ClientSecret {
		return nil, fmt.Errorf("invalid client_secret")
	}

	if app.ID != authCode.ApplicationID {
		return nil, fmt.Errorf("authorization code was not issued to this application")
	}

	if authCode.RedirectUri != req.RedirectURI {
		return nil, fmt.Errorf("redirect_uri mismatch")
	}

	// PKCE validation.
	var challenge, method string
	if authCode.CodeChallenge != nil {
		challenge = *authCode.CodeChallenge
	}
	if authCode.CodeChallengeMethod != nil {
		method = *authCode.CodeChallengeMethod
	}
	if err := ValidatePKCE(challenge, method, req.CodeVerifier); err != nil {
		return nil, fmt.Errorf("PKCE validation failed: %w", err)
	}

	// Delete the authorization code (single-use).
	if err := s.store.DeleteAuthorizationCode(ctx, req.Code); err != nil {
		s.logger.Error("failed to delete authorization code after exchange",
			"code_id", authCode.ID, "error", err)
	}

	// Generate and store the access token.
	return s.issueToken(ctx, app.ID, &authCode.AccountID, authCode.Scopes)
}

// ExchangeClientCredentials issues an app-only token (no user context).
//
// Validates client_id and client_secret. The resulting token has
// account_id = NULL and scopes limited to "read" (regardless of what
// the app registered for). This matches Mastodon's behaviour: app-only
// tokens can only read public data and instance metadata.
func (s *Server) ExchangeClientCredentials(ctx context.Context, req TokenRequest) (*TokenResponse, error) {
	app, err := s.store.GetApplicationByClientID(ctx, req.ClientID)
	if err != nil {
		return nil, fmt.Errorf("invalid client_id")
	}

	if app.ClientSecret != req.ClientSecret {
		return nil, fmt.Errorf("invalid client_secret")
	}

	return s.issueToken(ctx, app.ID, nil, "read")
}

// RevokeToken marks an access token as revoked and evicts it from the cache.
//
// Per RFC 7009, revocation always returns success even if the token is
// already revoked or does not exist — this prevents token enumeration.
func (s *Server) RevokeToken(ctx context.Context, token string) error {
	if err := s.store.RevokeAccessToken(ctx, token); err != nil {
		s.logger.Warn("revoke token: db error (treated as success per RFC 7009)", "error", err)
	}

	cacheKey := tokenCacheKey(token)
	_ = s.cache.Delete(ctx, cacheKey)

	return nil
}

// LookupToken verifies a Bearer token and returns the associated claims.
//
// Lookup strategy:
//  1. Compute cache key: "token:{sha256(rawToken)}"
//  2. Cache hit → unmarshal and return.
//  3. Cache miss → query oauth_access_tokens WHERE token = $1 AND revoked_at IS NULL.
//  4. If the token has expires_at set and it is in the past, treat as invalid.
//  5. On valid hit: cache the claims for tokenCacheTTL (24h for non-expiring
//     tokens, or until expires_at, whichever is shorter).
//
// The raw token is never stored in the cache — only the SHA-256 hash is used
// as the key. The cached value is the serialized TokenClaims.
func (s *Server) LookupToken(ctx context.Context, rawToken string) (*TokenClaims, error) {
	cacheKey := tokenCacheKey(rawToken)

	var claims TokenClaims
	if hit, _ := cache.GetJSON(ctx, s.cache, cacheKey, &claims); hit {
		return &claims, nil
	}

	tok, err := s.store.GetAccessToken(ctx, rawToken)
	if err != nil {
		return nil, fmt.Errorf("invalid access token")
	}

	if tok.ExpiresAt != nil && tok.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("access token has expired")
	}

	claims = TokenClaims{
		ApplicationID: tok.ApplicationID,
		Scopes:        Parse(tok.Scopes),
	}
	if tok.AccountID != nil {
		claims.AccountID = *tok.AccountID
	}

	// Cache with the shorter of tokenCacheTTL and time-until-expiry.
	ttl := tokenCacheTTL
	if tok.ExpiresAt != nil {
		remaining := time.Until(*tok.ExpiresAt)
		if remaining < ttl {
			ttl = remaining
		}
	}
	_ = cache.SetJSON(ctx, s.cache, cacheKey, claims, ttl)

	return &claims, nil
}

// --- internal helpers -------------------------------------------------------

// issueToken generates and stores a new access token.
func (s *Server) issueToken(ctx context.Context, appID string, accountID *string, scopes string) (*TokenResponse, error) {
	rawToken, err := generateRandomHex(32)
	if err != nil {
		return nil, fmt.Errorf("oauth: generate token: %w", err)
	}

	tok, err := s.store.CreateAccessToken(ctx, db.CreateAccessTokenParams{
		ID:            uid.New(),
		ApplicationID: appID,
		AccountID:     accountID,
		Token:         rawToken,
		Scopes:        Normalize(scopes),
		ExpiresAt:     nil, // non-expiring by default
	})
	if err != nil {
		return nil, fmt.Errorf("oauth: store token: %w", err)
	}

	return &TokenResponse{
		AccessToken: tok.Token,
		TokenType:   "Bearer",
		Scope:       tok.Scopes,
		CreatedAt:   tok.CreatedAt.Unix(),
	}, nil
}

// generateRandomHex generates n bytes of cryptographic randomness and returns
// them as a hex-encoded string (2n characters).
func generateRandomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// tokenCacheKey returns the cache key for a raw token. The token itself is
// not stored — only its SHA-256 hash — so that a cache compromise does not
// leak usable credentials.
func tokenCacheKey(rawToken string) string {
	h := sha256.Sum256([]byte(rawToken))
	return "token:" + hex.EncodeToString(h[:])
}
```

---

## 4. `oauth_authorization_codes` DDL

This table was introduced in ADR 02 (migration `000011`). The complete definition is reproduced here for reference:

```sql
CREATE TABLE oauth_authorization_codes (
    id                     TEXT PRIMARY KEY,
    code                   TEXT NOT NULL UNIQUE,
    application_id         TEXT NOT NULL REFERENCES oauth_applications(id),
    account_id             TEXT NOT NULL REFERENCES accounts(id),
    redirect_uri           TEXT NOT NULL,
    scopes                 TEXT NOT NULL,
    code_challenge         TEXT,             -- PKCE: base64url(SHA-256(verifier))
    code_challenge_method  TEXT,             -- 'S256' only; NULL if no PKCE
    expires_at             TIMESTAMPTZ NOT NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

**Key properties:**
- `code` is `UNIQUE` — prevents duplicate issuance.
- No `used_at` column — the row is **deleted** on exchange rather than flagged. This eliminates an entire class of race conditions (two concurrent exchanges of the same code). The `DELETE` and the `SELECT` are sequential within `ExchangeCode`; if two requests race, one will fail to find the code after the other deletes it.
- `code_challenge` and `code_challenge_method` are both nullable. Both NULL means no PKCE was requested (legacy flow). Both non-NULL means PKCE is required for exchange.
- `expires_at` is enforced in the `GetAuthorizationCode` query: `WHERE code = $1 AND expires_at > NOW()`.

---

## 5. `internal/api/oauth/handlers.go`

```go
package oauth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yourorg/monstera-fed/internal/api"
	"github.com/yourorg/monstera-fed/internal/api/middleware"
	oauthpkg "github.com/yourorg/monstera-fed/internal/oauth"
	"github.com/yourorg/monstera-fed/internal/store"

	"golang.org/x/crypto/bcrypt"
)

// Handler holds dependencies for the OAuth HTTP endpoints.
type Handler struct {
	oauth        *oauthpkg.Server
	store        store.Store
	logger       *slog.Logger
	loginTmpl    *template.Template
	instanceName string
	secretKey    []byte // SECRET_KEY_BASE for HMAC-signing the login session cookie
}

// NewHandler constructs an OAuth Handler.
// loginTmpl is the parsed HTML template for the login screen.
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

// --- POST /api/v1/apps — Register Application ----

// RegisterApp handles POST /api/v1/apps.
//
// Request body (form-encoded or JSON):
//
//	client_name:   string (required)
//	redirect_uris: string (required, space- or newline-separated)
//	scopes:        string (optional, default "read")
//	website:       string (optional)
//
// Response 200:
//
//	{
//	  "id": "...",
//	  "name": "...",
//	  "client_id": "...",
//	  "client_secret": "...",
//	  "redirect_uri": "...",
//	  "vapid_key": ""
//	}
func (h *Handler) RegisterApp(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		api.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	name := r.FormValue("client_name")
	if name == "" {
		api.WriteError(w, http.StatusUnprocessableEntity, "client_name is required")
		return
	}

	redirectURIs := r.FormValue("redirect_uris")
	if redirectURIs == "" {
		api.WriteError(w, http.StatusUnprocessableEntity, "redirect_uris is required")
		return
	}

	scopes := r.FormValue("scopes")
	website := r.FormValue("website")

	app, err := h.oauth.RegisterApplication(r.Context(), name, redirectURIs, scopes, website)
	if err != nil {
		api.WriteAppError(w, h.logger, &api.AppError{
			Status: http.StatusInternalServerError, Message: "internal server error", Err: err,
		})
		return
	}

	api.WriteJSON(w, http.StatusOK, app)
}

// --- GET /oauth/authorize — Authorization Screen ----

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

// Authorize handles GET /oauth/authorize.
//
// Query parameters:
//
//	client_id:             required
//	redirect_uri:          required
//	response_type:         required ("code")
//	scope:                 optional (default "read")
//	state:                 optional (CSRF pass-through)
//	code_challenge:        optional (PKCE)
//	code_challenge_method: optional ("S256")
//
// Flow:
//  1. Validate client_id and redirect_uri against the registered application.
//  2. Validate response_type == "code".
//  3. If code_challenge_method is present and not "S256", reject.
//  4. Render the login form (login.html) with the app name and scopes.
//  5. The form POSTs back to /oauth/authorize with email + password.
func (h *Handler) Authorize(w http.ResponseWriter, r *http.Request) {
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

	if !isValidRedirectURI(redirectURI, app.RedirectUris) {
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
		h.logger.Error("render login template", "error", err)
	}
}

// AuthorizeSubmit handles POST /oauth/authorize.
//
// Form fields (from the login form):
//
//	email:                 user's email
//	password:              user's password
//	client_id, redirect_uri, scope, state, code_challenge, code_challenge_method:
//	                       carried from the GET as hidden form fields
//
// Flow:
//  1. Validate email + password against the users table (bcrypt compare).
//  2. If invalid: re-render the login form with an error message.
//  3. If valid: call oauth.Server.AuthorizeRequest to generate a code.
//  4. Redirect to redirect_uri?code=...&state=...
func (h *Handler) AuthorizeSubmit(w http.ResponseWriter, r *http.Request) {
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

	// Look up application.
	app, err := h.store.GetApplicationByClientID(r.Context(), clientID)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "invalid client_id")
		return
	}

	// Authenticate user.
	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil {
		h.renderLoginError(w, "Invalid email or password", app.Name, clientID, redirectURI, scope, state, codeChallenge, codeChallengeMethod)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		h.renderLoginError(w, "Invalid email or password", app.Name, clientID, redirectURI, scope, state, codeChallenge, codeChallengeMethod)
		return
	}

	// Reject unconfirmed users.
	if user.ConfirmedAt == nil {
		h.renderLoginError(w, "Please confirm your email address before signing in", app.Name, clientID, redirectURI, scope, state, codeChallenge, codeChallengeMethod)
		return
	}

	// Reject suspended accounts.
	account, err := h.store.GetAccountByID(r.Context(), user.AccountID)
	if err != nil || account.Suspended {
		h.renderLoginError(w, "Your account has been suspended", app.Name, clientID, redirectURI, scope, state, codeChallenge, codeChallengeMethod)
		return
	}

	// Issue authorization code.
	code, err := h.oauth.AuthorizeRequest(r.Context(), oauthpkg.AuthorizeRequest{
		ApplicationID:       app.ID,
		AccountID:           user.AccountID,
		RedirectURI:         redirectURI,
		Scopes:              scope,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: codeChallengeMethod,
	})
	if err != nil {
		api.WriteAppError(w, h.logger, &api.AppError{
			Status: http.StatusInternalServerError, Message: "internal server error", Err: err,
		})
		return
	}

	// Redirect with code (and state if provided).
	redirectURL, _ := url.Parse(redirectURI)
	q := redirectURL.Query()
	q.Set("code", code)
	if state != "" {
		q.Set("state", state)
	}
	redirectURL.RawQuery = q.Encode()

	http.Redirect(w, r, redirectURL.String(), http.StatusFound)
}

// --- POST /oauth/token — Issue Token ----

// Token handles POST /oauth/token.
//
// Form fields:
//
//	grant_type:    "authorization_code" | "client_credentials"
//	code:          required for authorization_code
//	redirect_uri:  required for authorization_code
//	client_id:     required
//	client_secret: required
//	code_verifier: optional (PKCE)
//	scope:         optional (client_credentials only)
//
// Response 200 (RFC 6749):
//
//	{
//	  "access_token": "...",
//	  "token_type": "Bearer",
//	  "scope": "read write",
//	  "created_at": 1234567890
//	}
func (h *Handler) Token(w http.ResponseWriter, r *http.Request) {
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

// --- POST /oauth/revoke — Revoke Token ----

// Revoke handles POST /oauth/revoke (RFC 7009).
//
// Form fields:
//
//	token:         required
//	client_id:     required
//	client_secret: required
//
// Always returns 200 OK with an empty JSON body per RFC 7009 (no information
// is leaked about whether the token existed or was already revoked).
func (h *Handler) Revoke(w http.ResponseWriter, r *http.Request) {
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

// --- helpers ----------------------------------------------------------------

// isValidRedirectURI checks that uri matches one of the registered redirect
// URIs (newline-separated) for the application. "urn:ietf:wg:oauth:2.0:oob"
// is always accepted (Mastodon's out-of-band flow for CLI tools).
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

// renderLoginError re-renders the login form with an error message.
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
		h.logger.Error("render login template", "error", err)
	}
}
```

---

## 6. Login HTML Template

**File:** `internal/api/oauth/templates/login.html`

Embedded via `//go:embed templates/login.html` in the handler package.

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Sign in — {{.InstanceName}}</title>
  <style>
    *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;
      background-color: #f4f4f5; color: #1a1a2e;
      display: flex; justify-content: center; align-items: center;
      min-height: 100vh; padding: 20px;
    }
    .card {
      background: #fff; border-radius: 12px; box-shadow: 0 1px 3px rgba(0,0,0,0.1);
      width: 100%; max-width: 400px; padding: 40px 32px;
    }
    h1 { font-size: 22px; font-weight: 700; text-align: center; margin-bottom: 4px; }
    .subtitle {
      font-size: 14px; color: #6b7280; text-align: center; margin-bottom: 28px;
    }
    .app-info {
      background: #f9fafb; border: 1px solid #e5e7eb; border-radius: 8px;
      padding: 14px 16px; margin-bottom: 24px; font-size: 14px;
    }
    .app-info strong { color: #1a1a2e; }
    .app-info .scopes { color: #6b7280; font-size: 13px; margin-top: 4px; }
    label { display: block; font-size: 14px; font-weight: 500; margin-bottom: 6px; }
    input[type="email"], input[type="password"] {
      display: block; width: 100%; padding: 10px 14px;
      font-size: 15px; border: 1px solid #d1d5db; border-radius: 8px;
      outline: none; transition: border-color 0.15s;
    }
    input:focus { border-color: #6366f1; box-shadow: 0 0 0 3px rgba(99,102,241,0.15); }
    .field { margin-bottom: 18px; }
    .error {
      background: #fef2f2; border: 1px solid #fecaca; color: #991b1b;
      border-radius: 8px; padding: 10px 14px; font-size: 14px; margin-bottom: 18px;
    }
    button {
      display: block; width: 100%; padding: 12px;
      background: #6366f1; color: #fff; font-size: 15px; font-weight: 600;
      border: none; border-radius: 8px; cursor: pointer; transition: background 0.15s;
    }
    button:hover { background: #4f46e5; }
    button:active { background: #4338ca; }
  </style>
</head>
<body>
  <main class="card">
    <h1>{{.InstanceName}}</h1>
    <p class="subtitle">Sign in to continue</p>

    <div class="app-info">
      <strong>{{.AppName}}</strong> is requesting access to your account.
      <div class="scopes">Scopes: {{.Scopes}}</div>
    </div>

    {{if .Error}}
    <div class="error" role="alert">{{.Error}}</div>
    {{end}}

    <form method="POST" action="/oauth/authorize" autocomplete="on">
      <input type="hidden" name="client_id" value="{{.ClientID}}">
      <input type="hidden" name="redirect_uri" value="{{.RedirectURI}}">
      <input type="hidden" name="scope" value="{{.Scope}}">
      <input type="hidden" name="state" value="{{.State}}">
      <input type="hidden" name="code_challenge" value="{{.CodeChallenge}}">
      <input type="hidden" name="code_challenge_method" value="{{.CodeChallengeMethod}}">

      <div class="field">
        <label for="email">Email</label>
        <input type="email" id="email" name="email" required autofocus
               autocomplete="username" placeholder="you@example.com">
      </div>

      <div class="field">
        <label for="password">Password</label>
        <input type="password" id="password" name="password" required
               autocomplete="current-password">
      </div>

      <button type="submit">Sign in</button>
    </form>
  </main>
</body>
</html>
```

**Accessibility notes:**
- `role="alert"` on the error div so screen readers announce it immediately.
- `<label for="...">` properly linked to each input.
- `autofocus` on the email field.
- `autocomplete` attributes allow password managers to fill credentials.
- High-contrast colors (WCAG AA compliant — #1a1a2e on #fff, #991b1b on #fef2f2).
- The form submits to the same path (`/oauth/authorize`) via POST; the router distinguishes GET vs POST.

---

## 7. `internal/api/middleware/auth.go`

This file was outlined in ADR 01 (§3). The full design is provided here with the `RequiredScopes` middleware added.

```go
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/yourorg/monstera-fed/internal/api"
	"github.com/yourorg/monstera-fed/internal/domain"
	oauthpkg "github.com/yourorg/monstera-fed/internal/oauth"
	"github.com/yourorg/monstera-fed/internal/observability"
	"github.com/yourorg/monstera-fed/internal/store"
)

// contextKey is an unexported type for context keys owned by this package.
type contextKey int

const (
	accountKey contextKey = iota
	tokenClaimsKey
)

// RequireAuth extracts the Bearer token from the Authorization header,
// looks it up via the OAuth server (cache → DB), resolves the associated
// account, and stores both the account and token claims in the request
// context.
//
// On failure (missing token, invalid token, revoked, expired, or the
// associated account is suspended): returns 401 with the Mastodon error body:
//
//	{"error": "The access token is invalid"}
//
// This middleware also stores the account ID in the logger context
// (observability.WithAccountID) so that downstream log entries include it.
func RequireAuth(oauth *oauthpkg.Server, accounts store.AccountStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawToken := extractBearerToken(r)
			if rawToken == "" {
				api.WriteError(w, http.StatusUnauthorized, "The access token is invalid")
				return
			}

			claims, err := oauth.LookupToken(r.Context(), rawToken)
			if err != nil {
				api.WriteError(w, http.StatusUnauthorized, "The access token is invalid")
				return
			}

			ctx := WithTokenClaims(r.Context(), claims)

			// App-only tokens (client_credentials) have no account.
			if claims.AccountID != "" {
				account, err := accounts.GetAccountByID(r.Context(), claims.AccountID)
				if err != nil || account.Suspended {
					api.WriteError(w, http.StatusUnauthorized, "The access token is invalid")
					return
				}
				ctx = WithAccount(ctx, &account)
				ctx = observability.WithAccountID(ctx, account.ID)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuth behaves like RequireAuth but does not reject unauthenticated
// requests. If a valid Bearer token is present, the account and claims are
// stored in the context. If the token is missing or invalid, the request
// proceeds with a nil account — downstream handlers check via AccountFromContext.
//
// Used for endpoints like GET /api/v1/timelines/public that return different
// data for authenticated vs. anonymous users.
func OptionalAuth(oauth *oauthpkg.Server, accounts store.AccountStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawToken := extractBearerToken(r)
			if rawToken == "" {
				next.ServeHTTP(w, r)
				return
			}

			claims, err := oauth.LookupToken(r.Context(), rawToken)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			ctx := WithTokenClaims(r.Context(), claims)

			if claims.AccountID != "" {
				account, err := accounts.GetAccountByID(r.Context(), claims.AccountID)
				if err == nil && !account.Suspended {
					ctx = WithAccount(ctx, &account)
					ctx = observability.WithAccountID(ctx, account.ID)
				}
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdmin checks that the authenticated account has role "admin" or
// "moderator". Must be chained after RequireAuth.
// Returns 403 {"error":"This action is not allowed"} if the check fails.
func RequireAdmin(users store.UserStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			account := AccountFromContext(r.Context())
			if account == nil {
				api.WriteError(w, http.StatusForbidden, "This action is not allowed")
				return
			}

			user, err := users.GetUserByAccountID(r.Context(), account.ID)
			if err != nil {
				api.WriteError(w, http.StatusForbidden, "This action is not allowed")
				return
			}

			if user.Role != "admin" && user.Role != "moderator" {
				api.WriteError(w, http.StatusForbidden, "This action is not allowed")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequiredScopes returns a middleware that checks whether the authenticated
// token has all the listed scopes. Scope expansion is already applied in
// LookupToken (e.g., a token with "read" satisfies "read:statuses").
//
// Returns 403 with the Mastodon error body:
//
//	{"error": "This action is outside the authorized scopes"}
//
// Must be chained after RequireAuth.
func RequiredScopes(scopes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := TokenClaimsFromContext(r.Context())
			if claims == nil {
				api.WriteError(w, http.StatusForbidden, "This action is outside the authorized scopes")
				return
			}

			if !claims.Scopes.HasAll(scopes...) {
				api.WriteError(w, http.StatusForbidden, "This action is outside the authorized scopes")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// --- Context helpers --------------------------------------------------------

// AccountFromContext retrieves the authenticated account from the context.
// Returns nil if the request is unauthenticated or the token is app-only.
func AccountFromContext(ctx context.Context) *domain.Account {
	v, _ := ctx.Value(accountKey).(*domain.Account)
	return v
}

// WithAccount stores an account in the context.
func WithAccount(ctx context.Context, a *domain.Account) context.Context {
	return context.WithValue(ctx, accountKey, a)
}

// TokenClaimsFromContext retrieves the token claims from the context.
// Returns nil if no token was resolved.
func TokenClaimsFromContext(ctx context.Context) *oauthpkg.TokenClaims {
	v, _ := ctx.Value(tokenClaimsKey).(*oauthpkg.TokenClaims)
	return v
}

// WithTokenClaims stores token claims in the context.
func WithTokenClaims(ctx context.Context, c *oauthpkg.TokenClaims) context.Context {
	return context.WithValue(ctx, tokenClaimsKey, c)
}

// extractBearerToken pulls the raw token from the Authorization header.
// Returns an empty string if the header is missing or not a Bearer token.
// Also checks the query parameter "access_token" as a fallback (used by
// Mastodon SSE streaming connections where setting headers is not possible).
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return r.URL.Query().Get("access_token")
}
```

---

## 8. `internal/ap/httpsig.go`

```go
package ap

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yourorg/monstera-fed/internal/cache"
)

// KeyFetcher fetches the public key for a given key ID from the remote AP
// actor document. The key ID is typically the actor's `publicKey.id` field
// (e.g. "https://remote.example/users/alice#main-key").
//
// Implementation: fetches the actor document over HTTPS, extracts the
// publicKey.publicKeyPem field, parses the PEM-encoded RSA public key.
// Results are cached under "ap:pubkey:{keyID}" with a 1-hour TTL.
type KeyFetcher func(ctx context.Context, keyID string) (*rsa.PublicKey, error)

// clockSkew is the maximum tolerated difference between the request's Date
// header and the server's clock. Requests outside this window are rejected
// even if the signature is otherwise valid.
const clockSkew = 30 * time.Second

// replayTTL is how long a (keyId, date, requestTarget) tuple is remembered
// to prevent replay attacks. Set to 60s — double the clock skew window.
const replayTTL = 60 * time.Second

// pubkeyCacheTTL is how long a fetched public key is cached.
const pubkeyCacheTTL = 1 * time.Hour

// Verify verifies the HTTP Signature on an incoming ActivityPub request.
//
// Algorithm (draft-cavage-http-signatures-12, Mastodon-compatible):
//
//  1. Parse the Signature header to extract keyId, algorithm, headers, signature.
//  2. Check the Date header — reject if it is more than ±30 seconds from now.
//  3. If the request has a body (POST), verify the Digest header:
//     Digest: SHA-256=base64(sha256(body))
//  4. Reconstruct the signing string from the listed headers:
//     (request-target): post /inbox
//     host: local.example.com
//     date: Tue, 24 Feb 2026 12:00:00 GMT
//     digest: SHA-256=...
//  5. Fetch the signing actor's public key via keyFetcher (cached).
//  6. Verify the RSA-SHA256 signature over the signing string.
//  7. Check for replay: compute a cache key from the (keyId, date, requestTarget)
//     triple. If the key already exists, reject as replay. Otherwise, store it
//     with replayTTL.
//
// Returns the keyId (actor key IRI) on success; an error otherwise.
func Verify(
	ctx context.Context,
	r *http.Request,
	keyFetcher KeyFetcher,
	c cache.Store,
) (keyID string, err error) {
	// Parse Signature header.
	sig, err := parseSignatureHeader(r.Header.Get("Signature"))
	if err != nil {
		return "", fmt.Errorf("httpsig: parse header: %w", err)
	}

	// Validate Date.
	dateStr := r.Header.Get("Date")
	if dateStr == "" {
		return "", fmt.Errorf("httpsig: missing Date header")
	}
	requestDate, err := http.ParseTime(dateStr)
	if err != nil {
		return "", fmt.Errorf("httpsig: parse Date: %w", err)
	}
	drift := time.Since(requestDate)
	if drift < 0 {
		drift = -drift
	}
	if drift > clockSkew {
		return "", fmt.Errorf("httpsig: Date header drift %v exceeds ±%v", drift, clockSkew)
	}

	// Verify Digest (for POST requests with a body).
	if r.Method == http.MethodPost && r.Body != nil {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return "", fmt.Errorf("httpsig: read body: %w", err)
		}
		r.Body = io.NopCloser(bytes.NewReader(body))

		expectedDigest := "SHA-256=" + base64.StdEncoding.EncodeToString(sha256Sum(body))
		actualDigest := r.Header.Get("Digest")
		if actualDigest != expectedDigest {
			return "", fmt.Errorf("httpsig: Digest mismatch")
		}
	}

	// Reconstruct the signing string.
	signingString := buildSigningString(r, sig.headers)

	// Fetch public key.
	pubKey, err := fetchPubKeyCached(ctx, sig.keyID, keyFetcher, c)
	if err != nil {
		return "", fmt.Errorf("httpsig: fetch key %q: %w", sig.keyID, err)
	}

	// Verify RSA-SHA256 signature.
	hash := sha256.Sum256([]byte(signingString))
	sigBytes, err := base64.StdEncoding.DecodeString(sig.signature)
	if err != nil {
		return "", fmt.Errorf("httpsig: decode signature: %w", err)
	}
	if err := rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hash[:], sigBytes); err != nil {
		return "", fmt.Errorf("httpsig: signature verification failed: %w", err)
	}

	// Replay prevention.
	replayKey := replayCacheKey(sig.keyID, dateStr, requestTarget(r))
	exists, _ := c.Exists(ctx, replayKey)
	if exists {
		return "", fmt.Errorf("httpsig: replay detected")
	}
	_ = c.Set(ctx, replayKey, []byte("1"), replayTTL)

	return sig.keyID, nil
}

// Sign signs an outgoing HTTP request with the given RSA private key.
//
// Signed headers: (request-target), host, date, digest (if body present).
//
// The Date header is set to the current time if not already present.
// If the request has a body, the Digest header is computed and set.
// The Signature header is constructed per draft-cavage-http-signatures-12.
func Sign(r *http.Request, keyID string, privateKey *rsa.PrivateKey) error {
	if r.Header.Get("Date") == "" {
		r.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	}

	if r.Header.Get("Host") == "" {
		r.Header.Set("Host", r.URL.Host)
	}

	headers := []string{"(request-target)", "host", "date"}

	// Compute and set Digest for requests with a body.
	if r.Body != nil {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("httpsig: read body for digest: %w", err)
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		r.ContentLength = int64(len(body))

		digest := "SHA-256=" + base64.StdEncoding.EncodeToString(sha256Sum(body))
		r.Header.Set("Digest", digest)
		headers = append(headers, "digest")
	}

	signingString := buildSigningString(r, headers)

	hash := sha256.Sum256([]byte(signingString))
	sigBytes, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return fmt.Errorf("httpsig: sign: %w", err)
	}

	sig := base64.StdEncoding.EncodeToString(sigBytes)

	r.Header.Set("Signature",
		fmt.Sprintf(`keyId="%s",algorithm="rsa-sha256",headers="%s",signature="%s"`,
			keyID, strings.Join(headers, " "), sig))

	return nil
}

// --- internal types and helpers ---------------------------------------------

// signatureParams holds parsed fields from the Signature header.
type signatureParams struct {
	keyID     string
	algorithm string
	headers   []string
	signature string
}

// parseSignatureHeader parses a Signature header value into its components.
//
// Format: keyId="...",algorithm="...",headers="...",signature="..."
func parseSignatureHeader(header string) (*signatureParams, error) {
	if header == "" {
		return nil, fmt.Errorf("empty Signature header")
	}

	params := &signatureParams{}
	for _, part := range splitSignatureParams(header) {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		v = strings.Trim(v, `"`)
		switch strings.TrimSpace(k) {
		case "keyId":
			params.keyID = v
		case "algorithm":
			params.algorithm = v
		case "headers":
			params.headers = strings.Fields(v)
		case "signature":
			params.signature = v
		}
	}

	if params.keyID == "" || params.signature == "" {
		return nil, fmt.Errorf("missing required Signature fields (keyId, signature)")
	}

	if len(params.headers) == 0 {
		params.headers = []string{"date"}
	}

	return params, nil
}

// splitSignatureParams splits a Signature header value on commas, but
// respects quoted strings (signature values may contain base64 characters
// including '=' and '+').
func splitSignatureParams(header string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	for _, ch := range header {
		switch {
		case ch == '"':
			inQuote = !inQuote
			current.WriteRune(ch)
		case ch == ',' && !inQuote:
			parts = append(parts, strings.TrimSpace(current.String()))
			current.Reset()
		default:
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, strings.TrimSpace(current.String()))
	}
	return parts
}

// buildSigningString constructs the signing string from the request and the
// ordered list of headers to include.
//
// Each line: "{header}: {value}" (lowercase header name).
// Special pseudo-header: "(request-target): {method} {path}"
// Lines are joined by "\n".
func buildSigningString(r *http.Request, headers []string) string {
	var lines []string
	for _, h := range headers {
		switch h {
		case "(request-target)":
			lines = append(lines, fmt.Sprintf("(request-target): %s %s",
				strings.ToLower(r.Method), r.URL.RequestURI()))
		default:
			lines = append(lines, fmt.Sprintf("%s: %s",
				strings.ToLower(h), r.Header.Get(http.CanonicalHeaderKey(h))))
		}
	}
	return strings.Join(lines, "\n")
}

// requestTarget returns "{method} {path}" for the request.
func requestTarget(r *http.Request) string {
	return strings.ToLower(r.Method) + " " + r.URL.RequestURI()
}

// sha256Sum returns the SHA-256 hash of data.
func sha256Sum(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

// fetchPubKeyCached fetches the public key via keyFetcher with a cache layer.
// Cache key: "ap:pubkey:{sha256(keyID)}"
// TTL: 1 hour.
//
// On cache miss, the key is fetched from the remote actor document via HTTP.
// On cache hit, the PEM-encoded key is deserialized from the cache.
//
// Key rotation handling: when a remote actor rotates their key, the old
// cached key will fail signature verification. The caller (Verify) will
// get an error. The next request from that actor will also fail until the
// cache entry expires (up to 1 hour). A future enhancement could evict the
// cached key on verification failure and retry with a fresh fetch.
func fetchPubKeyCached(
	ctx context.Context,
	keyID string,
	keyFetcher KeyFetcher,
	c cache.Store,
) (*rsa.PublicKey, error) {
	return cache.GetOrSet(ctx, c, pubKeyCacheKey(keyID), pubkeyCacheTTL,
		func() (*rsa.PublicKey, error) {
			return keyFetcher(ctx, keyID)
		},
	)
}

// pubKeyCacheKey returns the cache key for a public key. The keyID (a URL)
// is hashed to keep the key length bounded.
func pubKeyCacheKey(keyID string) string {
	h := sha256.Sum256([]byte(keyID))
	return "ap:pubkey:" + fmt.Sprintf("%x", h[:16])
}

// replayCacheKey returns the cache key for replay prevention.
// Format: "httpsig:{sha256(keyId + date + requestTarget)}"
// The SHA-256 hash keeps the key a fixed length regardless of input.
func replayCacheKey(keyID, date, reqTarget string) string {
	h := sha256.Sum256([]byte(keyID + ":" + date + ":" + reqTarget))
	return "httpsig:" + fmt.Sprintf("%x", h[:16])
}
```

---

## 9. Replay Prevention Design

### Problem

An attacker who captures a valid signed HTTP request (e.g., by compromising a network hop) could re-send it to the target inbox. The signature is still valid because the same key and headers are used.

### Solution

After a signature is verified, the triple `(keyId, Date, requestTarget)` is stored in the cache with a TTL of **60 seconds**. Any subsequent request with the same triple is rejected as a replay.

### Cache key format

```
httpsig:{sha256_hex_16(keyId + ":" + date + ":" + requestTarget)}
```

Example:
```
httpsig:a7f3c9e2b1d04f83
```

The SHA-256 truncated to 16 bytes (32 hex chars) keeps keys fixed-length while providing sufficient collision resistance for this use case (the TTL is only 60 seconds; the total keyspace during any TTL window is small).

### TTL justification

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| Clock skew tolerance | ±30 seconds | Accounts for NTP drift between federated servers. Mastodon uses ±30s. |
| Replay cache TTL | 60 seconds | Must be ≥ 2× clock skew (30s) to cover the full window. At 60s, a request timestamped 29s in the future (just within tolerance) can be replayed up to 29s after the server receives it; the 60s TTL covers this. |
| Public key cache TTL | 1 hour | Remote actor profiles change rarely. Key rotation is handled passively: a cache miss triggers a fresh fetch. |

### Multi-replica correctness

When `CACHE_DRIVER=redis`, the replay set is shared across all replicas — a replayed request sent to a different pod is still caught. With `CACHE_DRIVER=memory`, each replica maintains an independent replay set. This means a replay could succeed if routed to a different replica within the 60s window. This is an accepted limitation of the memory driver (it is documented as dev-only); production deployments should use `CACHE_DRIVER=redis`.

---

## 10. Route Registration

These handlers integrate into the router designed in ADR 01 (§6):

```go
// In internal/api/router.go — NewRouter():

// OAuth — public, no auth required.
r.Post("/api/v1/apps", oauthHandler.RegisterApp)
r.Get("/oauth/authorize", oauthHandler.Authorize)
r.Post("/oauth/authorize", oauthHandler.AuthorizeSubmit)
r.Post("/oauth/token", oauthHandler.Token)
r.Post("/oauth/revoke", oauthHandler.Revoke)

// Example of scope-gated authenticated routes:
r.Route("/api/v1", func(r chi.Router) {
    r.Use(middleware.RequireAuth(oauthServer, accountStore))

    r.With(middleware.RequiredScopes("read:accounts")).
        Get("/accounts/verify_credentials", accountsHandler.VerifyCredentials)

    r.With(middleware.RequiredScopes("write:statuses")).
        Post("/statuses", statusesHandler.Create)

    r.With(middleware.RequiredScopes("read:notifications")).
        Get("/notifications", notificationsHandler.List)
})
```

---

## 11. Startup Wiring

The OAuth subsystem is initialised in `cmd/monstera-fed/serve.go` between steps 10 and 12 (see ADR 01, §4):

```go
// After step 10 (services built), before step 12 (router built):

// Build OAuth server.
oauthServer := oauth.NewServer(store, cacheStore, logger)

// Parse login template.
loginTmpl, err := template.ParseFS(oauthTemplatesFS, "templates/login.html")
if err != nil {
    logger.Error("failed to parse login template", "error", err)
    os.Exit(1)
}

// Build OAuth HTTP handler.
oauthHandler := oauthhttp.NewHandler(
    oauthServer, store, logger, loginTmpl,
    cfg.InstanceName, []byte(cfg.SecretKeyBase),
)

// The oauthServer is also passed to middleware.RequireAuth and middleware.OptionalAuth
// during router construction.
```

---

## 12. Open Questions — Resolved

| # | Question | Resolution |
|---|----------|------------|
| 1 | **Token expiry policy** | **Deferred to post-Phase 1.** Non-expiring tokens (Mastodon default) remain the only behaviour. The schema and `LookupToken` already support `expires_at`; a configurable TTL is a policy knob that can be added later with no schema or interface changes. Tracked in `future-feature-list.md`. |
| 2 | **CSRF protection on the login form** | **Phase 1.** Generate an HMAC token from `SECRET_KEY_BASE` + timestamp, set it as a `SameSite=Strict` cookie on `GET /oauth/authorize`, verify it on `POST /oauth/authorize`. Add a hidden `csrf_token` field to `login.html`. ~30 lines of implementation. |
| 3 | **`urn:ietf:wg:oauth:2.0:oob` flow** | **Deferred to post-Phase 1.** The `isValidRedirectURI` helper already accepts the OOB URI; the display page is not implemented. CLI tool support is low priority for launch. Tracked in `future-feature-list.md`. |
| 4 | **HTTP Signature key rotation retry** | **Phase 1.** On `rsa.VerifyPKCS1v15` failure, evict the cached public key, re-fetch via `keyFetcher`, and retry verification once. If the retry also fails, the signature is genuinely invalid. Adds at most one extra outbound HTTP request per rotation event. ~15 lines in `Verify`. |
| 5 | **Rate limiting on /oauth/authorize POST** | **Phase 1 (simple).** Cache-based limiter keyed on `loginrl:{sha256(email)}` with 60-second TTL and 5-attempt threshold. On the 6th attempt, return the same "Invalid email or password" error (no information leakage). ~20 lines in `AuthorizeSubmit` using the existing `cache.Store`. |

---

*End of ADR 06 — OAuth 2.0 & Authentication*
