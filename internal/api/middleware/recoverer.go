package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/observability"
)

// Recoverer returns a middleware that recovers from panics, logs the panic and
// stack trace with the request ID, and returns a generic 500 JSON response.
func Recoverer() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					requestID := observability.RequestIDFromContext(r.Context())
					slog.ErrorContext(r.Context(), "recovered from panic",
						slog.String("request_id", requestID),
						slog.Any("panic", rec),
						slog.String("stack", string(debug.Stack())),
					)
					api.HandleError(w, r, api.ErrInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
