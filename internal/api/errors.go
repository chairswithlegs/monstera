package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/chairswithlegs/monstera/internal/domain"
)

var (
	ErrUnauthorized        = errors.New("unauthorized")
	ErrForbidden           = errors.New("forbidden")
	ErrInternalServerError = errors.New("internal server error")
	ErrNotFound            = errors.New("not found")
	ErrUnprocessable       = errors.New("unprocessable entity")
	ErrBadRequest          = errors.New("bad request")
)

// NewUnauthorizedError creates a new unauthorized error with the given message.
func NewUnauthorizedError(msg string) error {
	return fmt.Errorf("%w: %s", ErrUnauthorized, msg)
}

// NewForbiddenError creates a new forbidden error with the given message.
func NewForbiddenError(msg string) error {
	return fmt.Errorf("%w: %s", ErrForbidden, msg)
}

// NewUnprocessableError creates a new unprocessable error with the given message.
func NewUnprocessableError(msg string) error {
	return fmt.Errorf("%w: %s", ErrUnprocessable, msg)
}

// NewBadRequestError creates a new bad request error with the given message.
func NewBadRequestError(msg string) error {
	return fmt.Errorf("%w: %s", ErrBadRequest, msg)
}

// ValidateRequiredString is a helper function to validate that a string is not empty.
// It returns a validation error if it is empty.
func ValidateRequiredString(field string) error {
	if field == "" {
		return fmt.Errorf("%w: [%s] is required", ErrBadRequest, field)
	}
	return nil
}

// ErrorResponse is the standard Mastodon-compatible error body.
type ErrorResponse struct {
	Error string `json:"error"`
}

// HandleError maps an internal error to an HTTP response.
// It logs unexpected errors and writes the appropriate status code and message.
func HandleError(w http.ResponseWriter, r *http.Request, err error) {
	slog.DebugContext(r.Context(), "handling api error", slog.Any("error", err))

	switch {
	case errors.Is(err, domain.ErrNotFound) || errors.Is(err, ErrNotFound):
		WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: "Record not found"})

	case errors.Is(err, domain.ErrConflict):
		WriteJSON(w, http.StatusConflict, ErrorResponse{Error: unwrapMessage(err)})

	case errors.Is(err, domain.ErrForbidden) || errors.Is(err, ErrForbidden):
		WriteJSON(w, http.StatusForbidden, ErrorResponse{Error: "Forbidden"})

	case errors.Is(err, domain.ErrUnauthorized):
		WriteJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Unauthorized"})

	case errors.Is(err, ErrUnauthorized):
		WriteJSON(w, http.StatusUnauthorized, ErrorResponse{Error: unwrapMessage(err)})

	case errors.Is(err, domain.ErrValidation) || errors.Is(err, ErrBadRequest):
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: unwrapMessage(err)})

	case errors.Is(err, domain.ErrUnprocessable) || errors.Is(err, ErrUnprocessable):
		WriteJSON(w, http.StatusUnprocessableEntity, ErrorResponse{Error: unwrapMessage(err)})

	case errors.Is(err, domain.ErrRateLimited):
		w.Header().Set("Retry-After", "900")
		WriteJSON(w, http.StatusTooManyRequests, ErrorResponse{Error: "Rate limit exceeded"})

	case errors.Is(err, domain.ErrGone):
		WriteJSON(w, http.StatusGone, ErrorResponse{Error: "Gone"})

	case errors.Is(err, domain.ErrAccountSuspended):
		WriteJSON(w, http.StatusForbidden, ErrorResponse{Error: "Account suspended"})

	case errors.Is(err, ErrInternalServerError):
		WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Internal server error"})

	default:
		slog.ErrorContext(r.Context(), "unhandled error",
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
