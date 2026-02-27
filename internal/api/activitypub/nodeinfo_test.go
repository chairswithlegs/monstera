package activitypub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera-fed/internal/config"
	"github.com/chairswithlegs/monstera-fed/internal/testutil"
)

func TestNodeInfoPointerHandler_GETNodeInfoPointer(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{InstanceDomain: "example.com"}
	h := NewNodeInfoPointerHandler(cfg)
	r := httptest.NewRequest(http.MethodGet, "/.well-known/nodeinfo", nil)
	w := httptest.NewRecorder()
	h.GETNodeInfoPointer(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Links []struct {
			Rel  string `json:"rel"`
			Href string `json:"href"`
		} `json:"links"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, "http://nodeinfo.diaspora.software/ns/schema/2.0", body.Links[0].Rel)
	assert.Equal(t, "https://example.com/nodeinfo/2.0", body.Links[0].Href)
}

func TestNodeInfoHandler_GETNodeInfo(t *testing.T) {
	t.Parallel()
	fake := testutil.NewFakeStore()
	cfg := &config.Config{InstanceDomain: "example.com", Version: "0.1.0"}
	h := NewNodeInfoHandler(testInstanceService(fake), cfg)
	r := httptest.NewRequest(http.MethodGet, "/nodeinfo/2.0", nil)
	w := httptest.NewRecorder()
	h.GETNodeInfo(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Version  string `json:"version"`
		Software struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"software"`
		Protocols []string `json:"protocols"`
		Usage     struct {
			Users      struct{ Total int64 } `json:"users"`
			LocalPosts int64                 `json:"localPosts"`
		} `json:"usage"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, "2.0", body.Version)
	assert.Equal(t, "monstera-fed", body.Software.Name)
	assert.Equal(t, "0.1.0", body.Software.Version)
	assert.Contains(t, body.Protocols, "activitypub")
}
