package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/observability"
)

// Recoverer returns a middleware that recovers from panics, logs the panic and
// stack trace with the request ID, and returns a generic 500 JSON response.
func Recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					requestID := observability.RequestIDFromContext(r.Context())
					logger.ErrorContext(r.Context(), "recovered from panic",
						slog.String("request_id", requestID),
						slog.Any("panic", rec),
						slog.String("stack", string(debug.Stack())),
					)
					api.WriteError(w, http.StatusInternalServerError, "Internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
