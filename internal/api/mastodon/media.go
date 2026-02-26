package mastodon

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera-fed/internal/api/middleware"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/service"
)

// MediaHandler handles media upload Mastodon API endpoints.
type MediaHandler struct {
	media  *service.MediaService
	logger *slog.Logger
}

// NewMediaHandler returns a new MediaHandler.
func NewMediaHandler(media *service.MediaService, logger *slog.Logger) *MediaHandler {
	return &MediaHandler{media: media, logger: logger}
}

// Upload handles POST /api/v2/media.
func (h *MediaHandler) Upload(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.WriteError(w, http.StatusUnauthorized, "The access token is invalid")
		return
	}
	if err := r.ParseMultipartForm(0); err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid multipart form")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "Missing or invalid file")
		return
	}
	defer func() { _ = file.Close() }()
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	var desc *string
	if d := r.FormValue("description"); d != "" {
		desc = &d
	}
	result, err := h.media.Upload(r.Context(), account.ID, file, contentType, desc)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			api.WriteError(w, http.StatusUnprocessableEntity, "Validation failed")
			return
		}
		api.HandleError(w, r, h.logger, err)
		return
	}
	out := apimodel.MediaFromDomain(result.Attachment)
	api.WriteJSON(w, http.StatusOK, out)
}
