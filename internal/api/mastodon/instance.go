package mastodon

import (
	"net/http"

	"github.com/chairswithlegs/monstera-fed/internal/api"
)

// InstanceConfig is the configuration sub-object in the instance response.
type InstanceConfig struct {
	Statuses struct {
		MaxCharacters       int `json:"max_characters"`
		MaxMediaAttachments int `json:"max_media_attachments"`
	} `json:"statuses"`
	MediaAttachments struct {
		SupportedMimeTypes []string `json:"supported_mime_types"`
		ImageSizeLimit     int64    `json:"image_size_limit"`
		VideoSizeLimit     int64    `json:"video_size_limit"`
	} `json:"media_attachments"`
}

// InstanceResponse is the Mastodon API v2 instance response.
type InstanceResponse struct {
	Domain        string         `json:"domain"`
	Title         string         `json:"title"`
	Version       string         `json:"version"`
	SourceURL     string         `json:"source_url"`
	Description   string         `json:"description"`
	Languages     []string       `json:"languages"`
	Configuration InstanceConfig `json:"configuration"`
	Registrations struct {
		Enabled bool `json:"enabled"`
	} `json:"registrations"`
	Contact struct {
		Email string `json:"email"`
	} `json:"contact"`
	Rules []any `json:"rules"`
}

// InstanceHandler handles instance metadata endpoints.
type InstanceHandler struct {
	deps Deps
}

// NewInstanceHandler returns a new InstanceHandler.
func NewInstanceHandler(deps Deps) *InstanceHandler {
	return &InstanceHandler{deps: deps}
}

// GETInstance handles GET /api/v2/instance.
func (h *InstanceHandler) GETInstance(w http.ResponseWriter, r *http.Request) {
	mimeTypes := h.deps.SupportedMimeTypes
	if mimeTypes == nil {
		mimeTypes = []string{"image/jpeg", "image/png", "image/gif", "image/webp"}
	}
	resp := InstanceResponse{
		Domain:      h.deps.InstanceDomain,
		Title:       h.deps.InstanceName,
		Version:     "0.1.0 (compatible; Monstera-fed)",
		SourceURL:   "",
		Description: "",
		Languages:   []string{"en"},
		Rules:       []any{},
	}
	resp.Configuration.Statuses.MaxCharacters = h.deps.MaxStatusChars
	resp.Configuration.Statuses.MaxMediaAttachments = 4
	resp.Configuration.MediaAttachments.SupportedMimeTypes = mimeTypes
	resp.Configuration.MediaAttachments.ImageSizeLimit = h.deps.MediaMaxBytes
	resp.Configuration.MediaAttachments.VideoSizeLimit = h.deps.MediaMaxBytes
	resp.Registrations.Enabled = true
	api.WriteJSON(w, http.StatusOK, resp)
}

// GETCustomEmojis handles GET /api/v1/custom_emojis.
func (h *InstanceHandler) GETCustomEmojis(w http.ResponseWriter, r *http.Request) {
	api.WriteJSON(w, http.StatusOK, []any{})
}
