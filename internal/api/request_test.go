package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeJSONBody(t *testing.T) {
	t.Parallel()

	t.Run("nil body returns ErrBadRequest", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", bytes.NewReader(nil))
		req.Body = nil
		var v map[string]string
		err := DecodeJSONBody(req, &v)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrBadRequest)
		assert.Contains(t, err.Error(), "request body is required")
	})

	t.Run("invalid JSON returns ErrBadRequest", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", bytes.NewReader([]byte("not json")))
		var v map[string]string
		err := DecodeJSONBody(req, &v)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrBadRequest)
		assert.Contains(t, err.Error(), "invalid JSON")
	})

	t.Run("valid JSON decodes into v", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", bytes.NewReader([]byte(`{"foo":"bar"}`)))
		var v map[string]string
		err := DecodeJSONBody(req, &v)
		require.NoError(t, err)
		assert.Equal(t, "bar", v["foo"])
	})
}

type decodeAndValidateTestRequest struct {
	Name string `json:"name"`
}

func (r *decodeAndValidateTestRequest) Validate() error {
	return ValidateRequiredField(r.Name, "name")
}

func TestDecodeAndValidateJSON(t *testing.T) {
	t.Parallel()

	t.Run("decode error is returned", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", bytes.NewReader([]byte("not json")))
		var body decodeAndValidateTestRequest
		err := DecodeAndValidateJSON(req, &body)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrBadRequest)
	})

	t.Run("validation error is returned", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", bytes.NewReader([]byte(`{}`)))
		var body decodeAndValidateTestRequest
		err := DecodeAndValidateJSON(req, &body)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrUnprocessable)
	})

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", bytes.NewReader([]byte(`{"name":"x"}`)))
		var body decodeAndValidateTestRequest
		err := DecodeAndValidateJSON(req, &body)
		require.NoError(t, err)
		assert.Equal(t, "x", body.Name)
	})
}
