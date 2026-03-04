package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeJSONBody(t *testing.T) {
	t.Parallel()

	t.Run("nil body returns error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(nil))
		req.Body = nil
		var v map[string]string
		err := DecodeJSONBody(req, &v)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no body")
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("not json")))
		var v map[string]string
		err := DecodeJSONBody(req, &v)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode JSON")
	})

	t.Run("valid JSON decodes into v", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{"foo":"bar"}`)))
		var v map[string]string
		err := DecodeJSONBody(req, &v)
		require.NoError(t, err)
		assert.Equal(t, "bar", v["foo"])
	})
}
