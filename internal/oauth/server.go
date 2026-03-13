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

	"github.com/chairswithlegs/monstera/internal/cache"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// AuthorizeRequest represents the validated parameters from GET /oauth/authorize
// that have been confirmed by the user (i.e. the user has logged in).
type AuthorizeRequest struct {
	ApplicationID       string
	AccountID           string
	RedirectURI         string
	Scopes              string
	CodeChallenge       string
	CodeChallengeMethod string
	State               string // pass-through; not stored
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
	AccountID     string // empty for app-only tokens
	ApplicationID string
	Scopes        ScopeSet
}

// AppResponse is the API response for POST /api/v1/apps.
type AppResponse struct {
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
func (s *Server) RegisterApplication(ctx context.Context, name, redirectURIs, scopes, website string) (*AppResponse, error) {
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

	app, err := s.store.CreateApplication(ctx, store.CreateApplicationInput{
		ID:           uid.New(),
		Name:         name,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURIs: redirectURIs,
		Scopes:       scopes,
		Website:      ws,
	})
	if err != nil {
		return nil, fmt.Errorf("oauth: create application: %w", err)
	}

	return &AppResponse{
		ID:           app.ID,
		Name:         app.Name,
		ClientID:     app.ClientID,
		ClientSecret: app.ClientSecret,
		RedirectURI:  app.RedirectURIs,
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

	expiresAt := time.Now().Add(authCodeTTL)
	_, err = s.store.CreateAuthorizationCode(ctx, store.CreateAuthorizationCodeInput{
		ID:                  uid.New(),
		Code:                code,
		ApplicationID:       req.ApplicationID,
		AccountID:           req.AccountID,
		RedirectURI:         req.RedirectURI,
		Scopes:              Normalize(req.Scopes),
		CodeChallenge:       challengePtr,
		CodeChallengeMethod: methodPtr,
		ExpiresAt:           expiresAt,
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
		return nil, errors.New("invalid or expired authorization code")
	}

	app, err := s.store.GetApplicationByClientID(ctx, req.ClientID)
	if err != nil {
		return nil, errors.New("invalid client_id")
	}

	if app.ClientSecret != req.ClientSecret {
		return nil, errors.New("invalid client_secret")
	}

	if app.ID != authCode.ApplicationID {
		return nil, errors.New("authorization code was not issued to this application")
	}

	if authCode.RedirectURI != req.RedirectURI {
		return nil, errors.New("redirect_uri mismatch")
	}

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

	if err := s.store.DeleteAuthorizationCode(ctx, req.Code); err != nil {
		err = fmt.Errorf("delete authorization code after exchange: %w", err)
		s.logger.Error("failed to delete authorization code after exchange",
			slog.String("code_id", authCode.ID), slog.Any("error", err))
	}

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
		return nil, errors.New("invalid client_id")
	}

	if app.ClientSecret != req.ClientSecret {
		return nil, errors.New("invalid client_secret")
	}

	return s.issueToken(ctx, app.ID, nil, "read")
}

// RevokeToken marks an access token as revoked and evicts it from the cache.
//
// Per RFC 7009, revocation always returns success even if the token is
// already revoked or does not exist — this prevents token enumeration.
func (s *Server) RevokeToken(ctx context.Context, token string) error {
	if err := s.store.RevokeAccessToken(ctx, token); err != nil {
		s.logger.Warn("revoke token: db error (treated as success per RFC 7009)", slog.Any("error", err))
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
		return nil, errors.New("invalid access token")
	}

	if tok.ExpiresAt != nil && tok.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("access token has expired")
	}

	claims = TokenClaims{
		ApplicationID: tok.ApplicationID,
		Scopes:        Parse(tok.Scopes),
	}
	if tok.AccountID != nil {
		claims.AccountID = *tok.AccountID
	}

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

func (s *Server) issueToken(ctx context.Context, appID string, accountID *string, scopes string) (*TokenResponse, error) {
	rawToken, err := generateRandomHex(32)
	if err != nil {
		return nil, fmt.Errorf("oauth: generate token: %w", err)
	}

	tok, err := s.store.CreateAccessToken(ctx, store.CreateAccessTokenInput{
		ID:            uid.New(),
		ApplicationID: appID,
		AccountID:     accountID,
		Token:         rawToken,
		Scopes:        Normalize(scopes),
		ExpiresAt:     nil,
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

// generateRandomHex returns n random bytes as a hex string. n is fixed at 32 for token generation.
func generateRandomHex(n int) (string, error) { //nolint:unparam
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func tokenCacheKey(rawToken string) string {
	h := sha256.Sum256([]byte(rawToken))
	return "token:" + hex.EncodeToString(h[:])
}
