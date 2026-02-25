package api

import (
	"encoding/json"
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
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "Record not found"})

	case errors.Is(err, domain.ErrConflict):
		writeJSON(w, http.StatusConflict, ErrorResponse{Error: unwrapMessage(err)})

	case errors.Is(err, domain.ErrForbidden):
		writeJSON(w, http.StatusForbidden, ErrorResponse{Error: "Forbidden"})

	case errors.Is(err, domain.ErrUnauthorized):
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Unauthorized"})

	case errors.Is(err, domain.ErrValidation):
		writeJSON(w, http.StatusUnprocessableEntity, ErrorResponse{Error: unwrapMessage(err)})

	case errors.Is(err, domain.ErrUnprocessable):
		writeJSON(w, http.StatusUnprocessableEntity, ErrorResponse{Error: unwrapMessage(err)})

	case errors.Is(err, domain.ErrRateLimited):
		w.Header().Set("Retry-After", "900")
		writeJSON(w, http.StatusTooManyRequests, ErrorResponse{Error: "Rate limit exceeded"})

	case errors.Is(err, domain.ErrGone):
		writeJSON(w, http.StatusGone, ErrorResponse{Error: "Gone"})

	case errors.Is(err, domain.ErrAccountSuspended):
		writeJSON(w, http.StatusForbidden, ErrorResponse{Error: "Account suspended"})

	default:
		logger.ErrorContext(r.Context(), "unhandled error",
			slog.Any("error", err),
			slog.String("path", r.URL.Path),
		)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Internal server error"})
	}
}

// unwrapMessage extracts the outermost message from a wrapped error chain.
func unwrapMessage(err error) string {
	return err.Error()
}

// WriteJSON encodes v as JSON and writes it with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	writeJSON(w, status, v)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// WriteError writes a JSON error response with the given status and message.
func WriteError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{Error: msg})
}
