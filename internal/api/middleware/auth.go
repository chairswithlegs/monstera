package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	oauthpkg "github.com/chairswithlegs/monstera-fed/internal/oauth"
	"github.com/chairswithlegs/monstera-fed/internal/observability"
	"github.com/chairswithlegs/monstera-fed/internal/store"
)

type contextKey int

const (
	accountKey contextKey = iota
	tokenClaimsKey
)

// RequireAuth extracts the Bearer token from the Authorization header,
// looks it up via the OAuth server (cache → DB), resolves the associated
// account, and stores both the account and token claims in the request context.
//
// On failure (missing token, invalid token, revoked, expired, or the
// associated account is suspended): returns 401 with the Mastodon error body:
//
//	{"error": "The access token is invalid"}
func RequireAuth(oauth *oauthpkg.Server, s store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawToken := extractBearerToken(r)
			if rawToken == "" {
				api.HandleError(w, r, api.ErrUnauthorized)
				return
			}

			claims, err := oauth.LookupToken(r.Context(), rawToken)
			if err != nil {
				slog.ErrorContext(r.Context(), "lookup token failed", slog.Any("error", err))
				api.HandleError(w, r, api.ErrUnauthorized)
				return
			}

			ctx := WithTokenClaims(r.Context(), claims)

			if claims.AccountID != "" {
				account, err := s.GetAccountByID(r.Context(), claims.AccountID)
				if errors.Is(err, domain.ErrNotFound) || account.Suspended {
					api.HandleError(w, r, api.ErrUnauthorized)
					return
				}
				if err != nil {
					slog.ErrorContext(r.Context(), "get account by id failed", slog.Any("error", err))
					api.HandleError(w, r, err)
					return
				}
				ctx = WithAccount(ctx, account)
				ctx = observability.WithAccountID(ctx, account.ID)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuth behaves like RequireAuth but does not reject unauthenticated
// requests. If a valid Bearer token is present, the account and claims are
// stored in the context. If the token is missing or invalid, the request
// proceeds with a nil account.
func OptionalAuth(oauth *oauthpkg.Server, s store.Store) func(http.Handler) http.Handler {
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
				account, err := s.GetAccountByID(r.Context(), claims.AccountID)
				if err == nil && !account.Suspended {
					ctx = WithAccount(ctx, account)
					ctx = observability.WithAccountID(ctx, account.ID)
				}
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdmin checks that the authenticated account has role "admin" or
// "moderator". Must be chained after RequireAuth.
func RequireAdmin(s store.Store, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			account := AccountFromContext(r.Context())
			if account == nil {
				api.HandleError(w, r, api.ErrForbidden)
				return
			}

			user, err := s.GetUserByAccountID(r.Context(), account.ID)
			if errors.Is(err, domain.ErrNotFound) {
				api.HandleError(w, r, api.ErrForbidden)
				return
			}
			if err != nil {
				slog.ErrorContext(r.Context(), "get user by account id failed", slog.Any("error", err))
				api.HandleError(w, r, err)
				return
			}

			if user.Role != domain.RoleAdmin && user.Role != domain.RoleModerator {
				api.HandleError(w, r, api.ErrForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequiredScopes returns a middleware that checks whether the authenticated
// token has all the listed scopes. Must be chained after RequireAuth.
func RequiredScopes(scopes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := TokenClaimsFromContext(r.Context())
			if claims == nil {
				err := api.NewForbiddenError("This action is outside the authorized scopes")
				api.HandleError(w, r, err)
				return
			}

			if !claims.Scopes.HasAll(scopes...) {
				err := api.NewForbiddenError("This action is outside the authorized scopes")
				api.HandleError(w, r, err)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// AccountFromContext retrieves the authenticated account from the context.
func AccountFromContext(ctx context.Context) *domain.Account {
	v, _ := ctx.Value(accountKey).(*domain.Account)
	return v
}

// WithAccount stores an account in the context.
func WithAccount(ctx context.Context, a *domain.Account) context.Context {
	return context.WithValue(ctx, accountKey, a)
}

// TokenClaimsFromContext retrieves the token claims from the context.
func TokenClaimsFromContext(ctx context.Context) *oauthpkg.TokenClaims {
	v, _ := ctx.Value(tokenClaimsKey).(*oauthpkg.TokenClaims)
	return v
}

// WithTokenClaims stores token claims in the context.
func WithTokenClaims(ctx context.Context, c *oauthpkg.TokenClaims) context.Context {
	return context.WithValue(ctx, tokenClaimsKey, c)
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return r.URL.Query().Get("access_token")
}
