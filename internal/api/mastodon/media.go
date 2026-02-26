package mastodon

import (
	"net/http"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera-fed/internal/api/middleware"
)

// MediaHandler handles media upload Mastodon API endpoints.
type MediaHandler struct {
	deps Deps
}

// NewMediaHandler returns a new MediaHandler.
func NewMediaHandler(deps Deps) *MediaHandler {
	return &MediaHandler{deps: deps}
}

// POSTMedia handles POSTMedia /api/v2/media.
func (h *MediaHandler) POSTMedia(w http.ResponseWriter, r *http.Request) {
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
	result, err := h.deps.Media.Upload(r.Context(), account.ID, file, contentType, desc)
	if err != nil {
		api.HandleError(w, r, h.deps.Logger, err)
		return
	}
	out := apimodel.MediaFromDomain(result.Attachment)
	api.WriteJSON(w, http.StatusOK, out)
}
