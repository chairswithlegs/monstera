package middleware

import (
	"net/http"
)

const accessTokenQueryParam = "access_token"

// StreamingTokenFromQuery copies the access_token query parameter into the
// Authorization: Bearer header so that RequireAuth/OptionalAuth can resolve
// the token. Use only on streaming routes; EventSource API does not support
// custom headers, so clients send the token in the URL.
func StreamingTokenFromQuery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token := r.URL.Query().Get(accessTokenQueryParam); token != "" && r.Header.Get("Authorization") == "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		next.ServeHTTP(w, r)
	})
}
