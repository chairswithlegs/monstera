package mastodon

import (
	"net/http"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/service"
)

// MediaHandler handles media upload Mastodon API endpoints.
type MediaHandler struct {
	media service.MediaService
}

// NewMediaHandler returns a new MediaHandler.
func NewMediaHandler(media service.MediaService) *MediaHandler {
	return &MediaHandler{media: media}
}

// POSTMedia handles POSTMedia /api/v2/media.
func (h *MediaHandler) POSTMedia(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	if err := r.ParseMultipartForm(0); err != nil {
		api.HandleError(w, r, api.NewBadRequestError("invalid multipart form"))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		api.HandleError(w, r, api.NewBadRequestError("missing or invalid file"))
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
		api.HandleError(w, r, err)
		return
	}
	out := apimodel.MediaFromDomain(result.Attachment)
	api.WriteJSON(w, http.StatusOK, out)
}
