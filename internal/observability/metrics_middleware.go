package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// MetricsMiddleware returns middleware that records HTTP request count and duration
// via the metrics singleton (RecordHTTPRequest, ObserveHTTPRequestDuration).
func MetricsMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			duration := time.Since(start).Seconds()
			method := r.Method
			path := r.URL.Path
			if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
				path = rc.RoutePattern()
			}
			status := strconv.Itoa(rec.status)
			RecordHTTPRequest(method, path, status)
			ObserveHTTPRequestDuration(method, path, duration)
		})
	}
}
