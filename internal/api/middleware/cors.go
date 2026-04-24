package middleware

import "net/http"

// CORS returns a middleware that sets CORS headers for a wildcard origin.
//
// The wildcard (`Access-Control-Allow-Origin: *`) is intentional and required
// for Mastodon API compatibility. Mastodon itself serves its REST API with
// wildcard CORS, and Mastodon-compatible web clients (Elk, Pinafore, semaphore,
// and others running on arbitrary origins) rely on it to make cross-origin
// requests to any instance. Native clients (Ivory, Tusky, Mona) are unaffected
// either way, but restricting the origin would break web clients.
//
// Because the API uses bearer-token auth (not cookies) and we do not set
// `Access-Control-Allow-Credentials`, the wildcard does not expose
// cookie-authenticated endpoints to cross-origin callers.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Expose-Headers", "Link")
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token")
			w.Header().Set("Access-Control-Max-Age", "300")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
