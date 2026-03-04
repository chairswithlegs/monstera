package mastodon

import (
	"net/http"

	"github.com/chairswithlegs/monstera/internal/api"
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
	instanceDomain     string
	instanceName       string
	maxStatusChars     int
	mediaMaxBytes      int64
	supportedMimeTypes []string
}

// NewInstanceHandler returns a new InstanceHandler.
func NewInstanceHandler(instanceDomain, instanceName string, maxStatusChars int, mediaMaxBytes int64, supportedMimeTypes []string) *InstanceHandler {
	return &InstanceHandler{
		instanceDomain:     instanceDomain,
		instanceName:       instanceName,
		maxStatusChars:     maxStatusChars,
		mediaMaxBytes:      mediaMaxBytes,
		supportedMimeTypes: supportedMimeTypes,
	}
}

// GETInstance handles GET /api/v2/instance.
func (h *InstanceHandler) GETInstance(w http.ResponseWriter, r *http.Request) {
	mimeTypes := h.supportedMimeTypes
	if mimeTypes == nil {
		mimeTypes = []string{"image/jpeg", "image/png", "image/gif", "image/webp"}
	}
	resp := InstanceResponse{
		Domain:      h.instanceDomain,
		Title:       h.instanceName,
		Version:     "0.1.0 (compatible; Monstera)",
		SourceURL:   "",
		Description: "",
		Languages:   []string{"en"},
		Rules:       []any{},
	}
	resp.Configuration.Statuses.MaxCharacters = h.maxStatusChars
	resp.Configuration.Statuses.MaxMediaAttachments = 4
	resp.Configuration.MediaAttachments.SupportedMimeTypes = mimeTypes
	resp.Configuration.MediaAttachments.ImageSizeLimit = h.mediaMaxBytes
	resp.Configuration.MediaAttachments.VideoSizeLimit = h.mediaMaxBytes
	resp.Registrations.Enabled = true
	api.WriteJSON(w, http.StatusOK, resp)
}

// GETCustomEmojis handles GET /api/v1/custom_emojis.
func (h *InstanceHandler) GETCustomEmojis(w http.ResponseWriter, r *http.Request) {
	api.WriteJSON(w, http.StatusOK, []any{})
}
