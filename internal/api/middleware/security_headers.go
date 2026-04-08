package middleware

import "net/http"

// SecurityHeaders returns a middleware that sets standard security headers on all responses.
//
//   - X-Content-Type-Options: nosniff — prevents MIME type sniffing
//   - X-Frame-Options: DENY — prevents clickjacking via framing
//   - Referrer-Policy: same-origin — limits referrer leakage
//   - Strict-Transport-Security — enforces HTTPS (only when useHSTS is true)
//
// Pass useHSTS=true when the server is configured behind HTTPS (i.e. the
// MonsteraServerURL scheme is "https"). Mastodon sets these same headers.
func SecurityHeaders(useHSTS bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "same-origin")
			if useHSTS {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}
