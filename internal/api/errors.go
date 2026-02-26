package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/chairswithlegs/monstera-fed/internal/domain"
)

// ErrorResponse is the standard Mastodon-compatible error body.
type ErrorResponse struct {
	Error string `json:"error"`
}

// HandleError maps a service/domain error to an HTTP response.
// It logs unexpected errors and writes the appropriate status code and message.
func HandleError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: "Record not found"})

	case errors.Is(err, domain.ErrConflict):
		WriteJSON(w, http.StatusConflict, ErrorResponse{Error: unwrapMessage(err)})

	case errors.Is(err, domain.ErrForbidden):
		WriteJSON(w, http.StatusForbidden, ErrorResponse{Error: "Forbidden"})

	case errors.Is(err, domain.ErrUnauthorized):
		WriteJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Unauthorized"})

	case errors.Is(err, domain.ErrValidation):
		WriteJSON(w, http.StatusUnprocessableEntity, ErrorResponse{Error: unwrapMessage(err)})

	case errors.Is(err, domain.ErrUnprocessable):
		WriteJSON(w, http.StatusUnprocessableEntity, ErrorResponse{Error: unwrapMessage(err)})

	case errors.Is(err, domain.ErrRateLimited):
		w.Header().Set("Retry-After", "900")
		WriteJSON(w, http.StatusTooManyRequests, ErrorResponse{Error: "Rate limit exceeded"})

	case errors.Is(err, domain.ErrGone):
		WriteJSON(w, http.StatusGone, ErrorResponse{Error: "Gone"})

	case errors.Is(err, domain.ErrAccountSuspended):
		WriteJSON(w, http.StatusForbidden, ErrorResponse{Error: "Account suspended"})

	default:
		logger.ErrorContext(r.Context(), "unhandled error",
			slog.Any("error", err),
			slog.String("path", r.URL.Path),
		)
		WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Internal server error"})
	}
}

// unwrapMessage extracts the outermost message from a wrapped error chain.
func unwrapMessage(err error) string {
	return err.Error()
}

// WriteError writes a JSON error response with the given status and message.
func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, ErrorResponse{Error: msg})
}
