package mastodon

import (
	"net/http"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera/internal/service"
)

// InstanceV1Response is the Mastodon API v1 instance response.
type InstanceV1Response struct {
	URI              string            `json:"uri"`
	Title            string            `json:"title"`
	ShortDescription string            `json:"short_description"`
	Description      string            `json:"description"`
	Email            string            `json:"email"`
	Version          string            `json:"version"`
	URLs             InstanceV1URLs    `json:"urls"`
	Stats            InstanceV1Stats   `json:"stats"`
	Languages        []string          `json:"languages"`
	ContactAccount   *apimodel.Account `json:"contact_account"`
	Rules            []any             `json:"rules"`
}

// InstanceV1URLs holds streaming_api URL for v1 instance.
type InstanceV1URLs struct {
	StreamingAPI string `json:"streaming_api"`
}

// InstanceV1Stats holds instance counts for v1.
type InstanceV1Stats struct {
	UserCount   int64 `json:"user_count"`
	StatusCount int64 `json:"status_count"`
	DomainCount int64 `json:"domain_count"`
}

// InstanceConfigURLs holds configuration URLs (v2 instance); clients like Elk read configuration.urls.streaming.
type InstanceConfigURLs struct {
	Streaming      string  `json:"streaming"`
	Status         *string `json:"status,omitempty"`
	About          *string `json:"about,omitempty"`
	PrivacyPolicy  *string `json:"privacy_policy,omitempty"`
	TermsOfService *string `json:"terms_of_service,omitempty"`
}

// InstanceConfig is the configuration sub-object in the instance response.
type InstanceConfig struct {
	URLs     InstanceConfigURLs `json:"urls"`
	Statuses struct {
		MaxCharacters            int `json:"max_characters"`
		MaxMediaAttachments      int `json:"max_media_attachments"`
		CharactersReservedPerURL int `json:"characters_reserved_per_url"`
	} `json:"statuses"`
	MediaAttachments struct {
		SupportedMimeTypes []string `json:"supported_mime_types"`
		ImageSizeLimit     int64    `json:"image_size_limit"`
		VideoSizeLimit     int64    `json:"video_size_limit"`
	} `json:"media_attachments"`
	Polls struct {
		MaxOptions             int `json:"max_options"`
		MaxCharactersPerOption int `json:"max_characters_per_option"`
		MinExpiration          int `json:"min_expiration"`
		MaxExpiration          int `json:"max_expiration"`
	} `json:"polls"`
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
	instanceSvc        service.InstanceService
}

// NewInstanceHandler returns a new InstanceHandler. instanceSvc is used for v1 stats; may be nil for tests that only need v2.
func NewInstanceHandler(instanceDomain, instanceName string, maxStatusChars int, mediaMaxBytes int64, supportedMimeTypes []string, instanceSvc service.InstanceService) *InstanceHandler {
	return &InstanceHandler{
		instanceDomain:     instanceDomain,
		instanceName:       instanceName,
		maxStatusChars:     maxStatusChars,
		mediaMaxBytes:      mediaMaxBytes,
		supportedMimeTypes: supportedMimeTypes,
		instanceSvc:        instanceSvc,
	}
}

const instanceVersion = "4.1.0"

// GETInstanceV1 handles GET /api/v1/instance (Mastodon v1 entity shape).
func (h *InstanceHandler) GETInstanceV1(w http.ResponseWriter, r *http.Request) {
	stats := InstanceV1Stats{}
	if h.instanceSvc != nil {
		s, err := h.instanceSvc.GetInstanceStats(r.Context())
		if err == nil {
			stats.UserCount = s.UserCount
			stats.StatusCount = s.StatusCount
			stats.DomainCount = s.DomainCount
		}
	}
	resp := InstanceV1Response{
		URI:              h.instanceDomain,
		Title:            h.instanceName,
		ShortDescription: "",
		Description:      "",
		Email:            "",
		Version:          instanceVersion,
		URLs: InstanceV1URLs{
			StreamingAPI: "wss://" + h.instanceDomain,
		},
		Stats:          stats,
		Languages:      []string{"en"},
		ContactAccount: nil,
		Rules:          []any{},
	}
	api.WriteJSON(w, http.StatusOK, resp)
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
		Version:     instanceVersion,
		SourceURL:   "",
		Description: "",
		Languages:   []string{"en"},
		Rules:       []any{},
	}
	resp.Configuration.URLs = InstanceConfigURLs{
		Streaming: "wss://" + h.instanceDomain,
	}
	resp.Configuration.Statuses.MaxCharacters = h.maxStatusChars
	resp.Configuration.Statuses.MaxMediaAttachments = 4
	resp.Configuration.Statuses.CharactersReservedPerURL = 23
	resp.Configuration.MediaAttachments.SupportedMimeTypes = mimeTypes
	resp.Configuration.MediaAttachments.ImageSizeLimit = h.mediaMaxBytes
	resp.Configuration.MediaAttachments.VideoSizeLimit = h.mediaMaxBytes
	resp.Configuration.Polls.MaxOptions = 4
	resp.Configuration.Polls.MaxCharactersPerOption = 50
	resp.Configuration.Polls.MinExpiration = 300
	resp.Configuration.Polls.MaxExpiration = 2629746
	resp.Registrations.Enabled = true
	api.WriteJSON(w, http.StatusOK, resp)
}

// GETCustomEmojis handles GET /api/v1/custom_emojis.
func (h *InstanceHandler) GETCustomEmojis(w http.ResponseWriter, r *http.Request) {
	api.WriteJSON(w, http.StatusOK, []any{})
}
