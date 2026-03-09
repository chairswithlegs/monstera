package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// DecodeJSONBody decodes the request body as JSON into v.
// Returns ErrBadRequest if body is nil or JSON decoding fails; callers can pass the error to HandleError.
func DecodeJSONBody(r *http.Request, v any) error {
	if r.Body == nil {
		return NewBadRequestError("request body is required")
	}
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return NewBadRequestError("invalid JSON")
	}
	return nil
}

// Validatable is implemented by request structs that can validate their fields after decode.
type Validatable interface {
	Validate() error
}

// DecodeAndValidateJSON decodes the request body as JSON into v and calls v.Validate().
// Returns the first error from decode or validation; callers can pass it to HandleError.
func DecodeAndValidateJSON(r *http.Request, v Validatable) error {
	if err := DecodeJSONBody(r, v); err != nil {
		return err
	}
	if err := v.Validate(); err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	return nil
}
