package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/chairswithlegs/monstera/internal/api"
)

// Recoverer returns a middleware that recovers from panics, logs the panic and
// stack trace, and returns a generic 500 JSON response.
func Recoverer() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					slog.ErrorContext(r.Context(), "recovered from panic",
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
