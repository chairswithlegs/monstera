package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// DecodeJSONBody decodes the request body as JSON into v.
// Returns an error if body is nil or JSON decoding fails.
func DecodeJSONBody(r *http.Request, v interface{}) error {
	if r.Body == nil {
		return errors.New("no body")
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	return nil
}
