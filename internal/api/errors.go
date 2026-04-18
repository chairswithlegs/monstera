package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/chairswithlegs/monstera/internal/domain"
)

// API sentinel errors.
// When included in the error chain, HandleError will translate them to the appropriate HTTP status code and error response.
var (
	ErrUnauthorized        = errors.New("unauthorized")
	ErrForbidden           = errors.New("forbidden")
	ErrInternalServerError = errors.New("internal server error")
	ErrNotFound            = errors.New("not found")
	ErrUnprocessable       = errors.New("unprocessable entity")
	ErrBadRequest          = errors.New("bad request")
)

const (
	// Error code constants for machine-readable error identification.
	CodeNotFound                 = "not_found"
	CodeConflict                 = "conflict"
	CodeForbidden                = "forbidden"
	CodeUnauthorized             = "unauthorized"
	CodeBadRequest               = "bad_request"
	CodeValidationFailed         = "validation_failed"
	CodeRateLimited              = "rate_limited"
	CodePayloadTooLarge          = "payload_too_large"
	CodeGone                     = "gone"
	CodeAccountSuspended         = "account_suspended"
	CodeDeletionAlreadyRequested = "deletion_already_requested"
	CodeInternalError            = "internal_error"

	// Parameter names for error responses.
	ParamKeyReason = "reason"
	ParamKeyField  = "field"

	// Parameter values for error responses.
	ParamValueInvalidRequestBody                       = "invalid_request_body"
	ParamValueMissingRequiredField                     = "missing_required_field"
	ParamValueMissingRequiredFields                    = "missing_required_fields"
	ParamValueInvalidValue                             = "invalid_value"
	ParamValueInvalidRFC3339                           = "invalid_rfc3339"
	ParamValuePositiveIntRequired                      = "positive_int_required"
	ParamValueUnsupportedGrantType                     = "unsupported_grant_type"
	ParamValueUnconfirmed                              = "unconfirmed"
	ParamValueSuspended                                = "suspended"
	ParamValueInvalidEmailOrPassword                   = "invalid_email_or_password"
	ParamValueCannotSuspendOrSilenceAdminAccount       = "cannot_suspend_or_silence_admin_account"
	ParamValueCannotPerformActionOnOwnAccount          = "cannot_perform_action_on_own_account"
	ParamValueOutsideOfScopes                          = "outside_of_scopes"
	ParamValueRegistrationClosed                       = "registration_closed"
	ParamValuePollEnded                                = "poll_ended"
	ParamValueMustBeInTheFuture                        = "must_be_in_the_future"
	ParamValueOnlyPublicAndUnlistedStatusesCanBePinned = "only_public_and_unlisted_statuses_can_be_pinned"
	ParamValueCannotEditStatus                         = "cannot_edit_status"
	ParamValueUnsupportedContentType                   = "unsupported_content_type"
)

// APIError is a typed error that carries optional i18n params for the frontend.
// It wraps an api sentinel for error type assertions and status code mapping.
// msg is the human-readable detail included in the JSON "error" field for
// Mastodon client compatibility; params carries structured data for frontend i18n.
type APIError struct {
	sentinel error
	msg      string
	params   map[string]string
}

func (e *APIError) Error() string {
	if e.msg == "" {
		return e.sentinel.Error()
	}
	return e.sentinel.Error() + ": " + e.msg
}

// Unwrap returns the wrapped sentinel, preserving errors.Is() behaviour.
func (e *APIError) Unwrap() error { return e.sentinel }

// WithParams attaches interpolation params for frontend translation and returns
// the same error for chaining.
func (e *APIError) WithParams(params map[string]string) *APIError {
	e.params = params
	return e
}

// API error constructors.
// These helper functions return an APIError populated with the appropriate sentinel and params for the given error.
// When returning errors from a handler, it is recommended to use one of the following functions.

// 401 (Unauthorized) errors
func NewOutsideOfScopesError() *APIError {
	return &APIError{sentinel: ErrUnauthorized, msg: "Outside of scopes", params: map[string]string{ParamKeyReason: ParamValueOutsideOfScopes}}
}

func NewInvalidEmailOrPasswordError() *APIError {
	return &APIError{sentinel: ErrUnauthorized, msg: "Invalid email or password", params: map[string]string{ParamKeyReason: ParamValueInvalidEmailOrPassword}}
}

// 403 (Forbidden) errors
func NewRegistrationClosedError() *APIError {
	return &APIError{sentinel: ErrForbidden, msg: "Registration is closed", params: map[string]string{ParamKeyReason: ParamValueRegistrationClosed}}
}

// 422 (Unprocessable Entity) errors
func NewMissingRequiredFieldError(fieldName string) *APIError {
	return &APIError{sentinel: ErrUnprocessable, msg: fieldName + " is required", params: map[string]string{ParamKeyField: fieldName, ParamKeyReason: ParamValueMissingRequiredField}}
}

func NewMissingRequiredFieldsError(fieldNames []string) *APIError {
	return &APIError{sentinel: ErrUnprocessable, msg: strings.Join(fieldNames, ", ") + " are required", params: map[string]string{ParamKeyField: strings.Join(fieldNames, ","), ParamKeyReason: ParamValueMissingRequiredFields}}
}

func NewInvalidValueError(fieldName string) *APIError {
	return &APIError{sentinel: ErrUnprocessable, msg: fieldName + ": invalid value", params: map[string]string{ParamKeyField: fieldName, ParamKeyReason: ParamValueInvalidValue}}
}

func NewInvalidRFC3339Error(fieldName string) *APIError {
	return &APIError{sentinel: ErrUnprocessable, msg: fieldName + " must be a valid RFC3339 datetime", params: map[string]string{ParamKeyField: fieldName, ParamKeyReason: ParamValueInvalidRFC3339}}
}

func NewPositiveIntRequiredError(fieldName string) *APIError {
	return &APIError{sentinel: ErrUnprocessable, msg: fieldName + " must be a positive integer", params: map[string]string{ParamKeyField: fieldName, ParamKeyReason: ParamValuePositiveIntRequired}}
}

func NewUnconfirmedError() *APIError {
	return &APIError{sentinel: ErrUnprocessable, msg: "Account is not confirmed", params: map[string]string{ParamKeyReason: ParamValueUnconfirmed}}
}

func NewSuspendedError() *APIError {
	return &APIError{sentinel: ErrUnprocessable, msg: "Account is suspended", params: map[string]string{ParamKeyReason: ParamValueSuspended}}
}

func NewCannotSuspendOrSilenceAdminAccountError() *APIError {
	return &APIError{sentinel: ErrUnprocessable, msg: "Cannot suspend or silence an admin account", params: map[string]string{ParamKeyReason: ParamValueCannotSuspendOrSilenceAdminAccount}}
}

func NewCannotPerformActionOnOwnAccountError() *APIError {
	return &APIError{sentinel: ErrUnprocessable, msg: "Cannot perform this action on your own account", params: map[string]string{ParamKeyReason: ParamValueCannotPerformActionOnOwnAccount}}
}

func NewPollEndedError() *APIError {
	return &APIError{sentinel: ErrUnprocessable, msg: "Poll has ended", params: map[string]string{ParamKeyReason: ParamValuePollEnded}}
}

func NewMustBeInTheFutureError(fieldName string) *APIError {
	return &APIError{sentinel: ErrUnprocessable, msg: fieldName + " must be in the future", params: map[string]string{ParamKeyReason: ParamValueMustBeInTheFuture, ParamKeyField: fieldName}}
}

func NewOnlyPublicAndUnlistedStatusesCanBePinnedError() *APIError {
	return &APIError{sentinel: ErrUnprocessable, msg: "Only public and unlisted statuses can be pinned", params: map[string]string{ParamKeyReason: ParamValueOnlyPublicAndUnlistedStatusesCanBePinned}}
}

func NewCannotEditStatusError() *APIError {
	return &APIError{sentinel: ErrUnprocessable, msg: "Cannot edit this status", params: map[string]string{ParamKeyReason: ParamValueCannotEditStatus}}
}

// 400 (Bad Request) errors
func NewUnsupportedGrantTypeError(grantType string) *APIError {
	return &APIError{sentinel: ErrBadRequest, msg: "Unsupported grant type: " + grantType, params: map[string]string{ParamKeyReason: ParamValueUnsupportedGrantType, ParamKeyField: grantType}}
}

func NewInvalidRequestBodyError() *APIError {
	return &APIError{sentinel: ErrBadRequest, msg: "Invalid request body", params: map[string]string{ParamKeyReason: ParamValueInvalidRequestBody}}
}

func NewMissingRequiredParamError(paramName string) *APIError {
	return &APIError{sentinel: ErrBadRequest, msg: paramName + " is required", params: map[string]string{ParamKeyReason: ParamValueMissingRequiredField, ParamKeyField: paramName}}
}

func NewUnsupportedContentTypeError(contentType string) *APIError {
	return &APIError{sentinel: ErrBadRequest, msg: "Unsupported content type: " + contentType, params: map[string]string{ParamKeyReason: ParamValueUnsupportedContentType, ParamKeyField: contentType}}
}

// ErrorResponse is the standard (Mastodon-compatible) error body.
type ErrorResponse struct {
	Error  string            `json:"error"`
	Code   string            `json:"code,omitempty"`   // Error code for machine-readable identification.
	Params map[string]string `json:"params,omitempty"` // Optional interpolation params for frontend translation.
}

// HandleError maps an error to an HTTP response.
// It logs unexpected errors and writes the appropriate status code and message.
// Handlers should always call this function to handle errors.
func HandleError(w http.ResponseWriter, r *http.Request, err error) {
	slog.DebugContext(r.Context(), "handling api error", slog.Any("error", err))

	// Determine the HTTP status code and error response based on the error.
	status, resp := classifyError(err)

	// Log unexpected internal server errors.
	if status == http.StatusInternalServerError {
		slog.ErrorContext(r.Context(), "internal server error",
			slog.Any("error", err),
			slog.String("path", r.URL.Path),
		)
	}

	WriteJSON(w, status, resp)
}

// classifyError maps an error to an HTTP status code and ErrorResponse.
// This is the single source of truth for error translation to HTTP status codes.
func classifyError(err error) (int, ErrorResponse) {
	// Extract params from *APIError if present.
	var params map[string]string
	var apiErr *APIError
	errors.As(err, &apiErr)
	if apiErr != nil {
		params = apiErr.params
	}

	// Determine the HTTP status code and error response based on the error.
	switch {
	case errors.Is(err, domain.ErrNotFound) || errors.Is(err, ErrNotFound):
		return http.StatusNotFound, ErrorResponse{Error: "Record not found", Code: CodeNotFound, Params: params}

	case errors.Is(err, domain.ErrDeletionAlreadyRequested):
		return http.StatusConflict, ErrorResponse{Error: err.Error(), Code: CodeDeletionAlreadyRequested, Params: params}

	case errors.Is(err, domain.ErrConflict):
		return http.StatusConflict, ErrorResponse{Error: err.Error(), Code: CodeConflict, Params: params}

	case errors.Is(err, domain.ErrForbidden) || errors.Is(err, ErrForbidden):
		return http.StatusForbidden, ErrorResponse{Error: "Forbidden", Code: CodeForbidden, Params: params}

	case errors.Is(err, domain.ErrUnauthorized):
		return http.StatusUnauthorized, ErrorResponse{Error: "Unauthorized", Code: CodeUnauthorized, Params: params}

	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized, ErrorResponse{Error: err.Error(), Code: CodeUnauthorized, Params: params}

	case errors.Is(err, domain.ErrValidation) || errors.Is(err, ErrBadRequest):
		return http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: CodeBadRequest, Params: params}

	case errors.Is(err, domain.ErrUnprocessable) || errors.Is(err, ErrUnprocessable):
		return http.StatusUnprocessableEntity, ErrorResponse{Error: err.Error(), Code: CodeValidationFailed, Params: params}

	case errors.Is(err, domain.ErrRateLimited):
		return http.StatusTooManyRequests, ErrorResponse{Error: "Rate limit exceeded", Code: CodeRateLimited, Params: params}

	case isMaxBytesError(err):
		return http.StatusRequestEntityTooLarge, ErrorResponse{Error: "Payload too large", Code: CodePayloadTooLarge, Params: params}

	case errors.Is(err, domain.ErrGone):
		return http.StatusGone, ErrorResponse{Error: "Gone", Code: CodeGone, Params: params}

	case errors.Is(err, domain.ErrAccountSuspended):
		return http.StatusForbidden, ErrorResponse{Error: "Account suspended", Code: CodeAccountSuspended, Params: params}

	case errors.Is(err, ErrInternalServerError):
		return http.StatusInternalServerError, ErrorResponse{Error: "Internal server error", Code: CodeInternalError, Params: params}

	default:
		return http.StatusInternalServerError, ErrorResponse{Error: "Internal server error", Code: CodeInternalError, Params: params}
	}
}

func isMaxBytesError(err error) bool {
	var mbe *http.MaxBytesError
	return errors.As(err, &mbe)
}
