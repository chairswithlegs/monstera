package middleware

import "net/http"

// MaxBodySize returns middleware that limits the request body to the given
// number of bytes using http.MaxBytesReader. Requests that exceed the limit
// surface a *http.MaxBytesError when the body is read by downstream handlers.
func MaxBodySize(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}
			next.ServeHTTP(w, r)
		})
	}
}
