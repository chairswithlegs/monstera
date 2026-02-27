package mastodon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstanceHandler_GetInstance(t *testing.T) {
	t.Parallel()
	deps := Deps{
		InstanceDomain: "example.com", InstanceName: "Example Instance",
		MaxStatusChars: 500, MediaMaxBytes: 10 << 20,
		SupportedMimeTypes: []string{"image/jpeg", "image/png"},
	}
	handler := NewInstanceHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/instance", nil)
	rec := httptest.NewRecorder()
	handler.GETInstance(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body InstanceResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "example.com", body.Domain)
	assert.Equal(t, "Example Instance", body.Title)
	assert.Equal(t, "0.1.0 (compatible; Monstera-fed)", body.Version)
	assert.Equal(t, 500, body.Configuration.Statuses.MaxCharacters)
	assert.Equal(t, 4, body.Configuration.Statuses.MaxMediaAttachments)
	assert.Equal(t, []string{"image/jpeg", "image/png"}, body.Configuration.MediaAttachments.SupportedMimeTypes)
	assert.True(t, body.Registrations.Enabled)
}

func TestInstanceHandler_GetInstance_default_mime_types(t *testing.T) {
	t.Parallel()
	deps := Deps{InstanceDomain: "test.com", InstanceName: "Test", MaxStatusChars: 500, MediaMaxBytes: 5 << 20}
	handler := NewInstanceHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/instance", nil)
	rec := httptest.NewRecorder()
	handler.GETInstance(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body InstanceResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.NotEmpty(t, body.Configuration.MediaAttachments.SupportedMimeTypes)
	assert.Contains(t, body.Configuration.MediaAttachments.SupportedMimeTypes, "image/jpeg")
}

func TestInstanceHandler_CustomEmojis(t *testing.T) {
	t.Parallel()
	deps := Deps{InstanceDomain: "example.com", InstanceName: "Example", MaxStatusChars: 500, MediaMaxBytes: 10 << 20}
	handler := NewInstanceHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/custom_emojis", nil)
	rec := httptest.NewRecorder()
	handler.GETCustomEmojis(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body []any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Empty(t, body)
}
