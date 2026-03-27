package mastodon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
)

func TestInstanceHandler_GetInstance(t *testing.T) {
	t.Parallel()
	handler := NewInstanceHandler("example.com", "Example Instance", 500, 10<<20, []string{"image/jpeg", "image/png"}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/instance", nil)
	rec := httptest.NewRecorder()
	handler.GETInstance(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body InstanceResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "example.com", body.Domain)
	assert.Equal(t, "Example Instance", body.Title)
	assert.Equal(t, "4.3.0", body.Version)
	assert.Equal(t, "wss://example.com", body.Configuration.URLs.Streaming, "clients like Elk require configuration.urls.streaming")
	assert.Equal(t, 500, body.Configuration.Statuses.MaxCharacters)
	assert.Equal(t, 4, body.Configuration.Statuses.MaxMediaAttachments)
	assert.Equal(t, []string{"image/jpeg", "image/png"}, body.Configuration.MediaAttachments.SupportedMimeTypes)
	assert.True(t, body.Registrations.Enabled)
}

func TestInstanceHandler_GetInstance_default_mime_types(t *testing.T) {
	t.Parallel()
	handler := NewInstanceHandler("test.com", "Test", 500, 5<<20, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/instance", nil)
	rec := httptest.NewRecorder()
	handler.GETInstance(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body InstanceResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.NotEmpty(t, body.Configuration.MediaAttachments.SupportedMimeTypes)
	assert.Contains(t, body.Configuration.MediaAttachments.SupportedMimeTypes, "image/jpeg")
}

func TestInstanceHandler_GETInstanceV1(t *testing.T) {
	t.Parallel()
	handler := NewInstanceHandler("example.com", "Example Instance", 500, 10<<20, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/instance", nil)
	rec := httptest.NewRecorder()
	handler.GETInstanceV1(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body InstanceV1Response
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "example.com", body.URI)
	assert.Equal(t, "Example Instance", body.Title)
	assert.NotEmpty(t, body.Version)
	assert.Equal(t, "wss://example.com", body.URLs.StreamingAPI)
	assert.NotNil(t, body.Stats)
	assert.Equal(t, []string{"en"}, body.Languages)
	assert.Nil(t, body.ContactAccount)
	assert.NotNil(t, body.Rules)
}

// mockInstanceService returns fixed stats from GetInstanceStats for testing.
type mockInstanceService struct {
	stats *service.InstanceStats
}

func (m *mockInstanceService) GetNodeInfoStats(context.Context) (*service.NodeInfoStats, error) {
	return nil, nil
}
func (m *mockInstanceService) GetInstanceStats(ctx context.Context) (*service.InstanceStats, error) {
	if m.stats != nil {
		return m.stats, nil
	}
	return &service.InstanceStats{}, nil
}
func (m *mockInstanceService) ListKnownInstances(context.Context, int, int) ([]domain.KnownInstance, error) {
	return nil, nil
}

func TestInstanceHandler_GETInstanceV1_with_stats(t *testing.T) {
	t.Parallel()
	svc := &mockInstanceService{stats: &service.InstanceStats{
		UserCount:   10,
		StatusCount: 100,
		DomainCount: 5,
	}}
	handler := NewInstanceHandler("example.com", "Example", 500, 10<<20, nil, svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/instance", nil)
	rec := httptest.NewRecorder()
	handler.GETInstanceV1(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body InstanceV1Response
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, int64(10), body.Stats.UserCount)
	assert.Equal(t, int64(100), body.Stats.StatusCount)
	assert.Equal(t, int64(5), body.Stats.DomainCount)
}

func TestInstanceHandler_CustomEmojis(t *testing.T) {
	t.Parallel()
	handler := NewInstanceHandler("example.com", "Example", 500, 10<<20, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/custom_emojis", nil)
	rec := httptest.NewRecorder()
	handler.GETCustomEmojis(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body []any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Empty(t, body)
}
