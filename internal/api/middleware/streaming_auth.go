package middleware

import (
	"net/http"
)

const accessTokenQueryParam = "access_token"

// StreamingTokenFromQuery copies the access_token query parameter or
// Sec-WebSocket-Protocol header into the Authorization: Bearer header so that
// RequireAuth/OptionalAuth can resolve the token. Use only on streaming routes;
// EventSource API does not support custom headers, so clients send the token in
// the URL. Browser WebSocket clients send the token as a subprotocol because the
// browser WebSocket API also does not allow custom headers.
//
// Priority: Authorization header > access_token query param > Sec-WebSocket-Protocol.
func StreamingTokenFromQuery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			if token := r.URL.Query().Get(accessTokenQueryParam); token != "" {
				r.Header.Set("Authorization", "Bearer "+token)
			} else if proto := r.Header.Get("Sec-WebSocket-Protocol"); proto != "" {
				r.Header.Set("Authorization", "Bearer "+proto)
			}
		}
		next.ServeHTTP(w, r)
	})
}
