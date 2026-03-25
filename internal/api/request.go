package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type Sanitizable interface {
	Sanitize()
}

// DecodeJSONBody decodes the request body as JSON into v.
func DecodeJSONBody(r *http.Request, v any) error {
	if r.Body == nil {
		return NewInvalidRequestBodyError()
	}
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return NewInvalidRequestBodyError()
	}

	// Sanitize the request body if the type implements Sanitizable.
	if s, ok := v.(Sanitizable); ok {
		s.Sanitize()
	}

	return nil
}

// Validatable is implemented by request structs that can validate their fields after decode.
type Validatable interface {
	Validate() error
}

// DecodeAndValidateJSON decodes the request body as JSON into v and calls v.Validate().
func DecodeAndValidateJSON(r *http.Request, v Validatable) error {
	if err := DecodeJSONBody(r, v); err != nil {
		return err
	}
	if err := v.Validate(); err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	return nil
}

func FormValueIsTruthy(form map[string][]string, key string) bool {
	v := FormValue(form, key)
	return strings.ToLower(v) == "true" || v == "1"
}

func FormValue(form map[string][]string, key string) string {
	if v := form[key]; len(v) > 0 {
		return v[0]
	}
	return ""
}

func QueryParamIsTrue(r *http.Request, key string) bool {
	v := r.URL.Query().Get(key)
	return strings.ToLower(v) == "true"
}
