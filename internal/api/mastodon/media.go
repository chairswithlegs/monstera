package mastodon

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
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

// POSTMedia handles POST /api/v1/media and POST /api/v2/media.
func (h *MediaHandler) POSTMedia(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	if err := r.ParseMultipartForm(0); err != nil { //nolint:gosec // G120: body size limited by upstream MaxBodySize middleware
		api.HandleError(w, r, api.NewInvalidRequestBodyError())
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		api.HandleError(w, r, api.NewInvalidRequestBodyError())
		return
	}
	defer func() { _ = file.Close() }()
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = contentTypeOctetStream
	}
	var desc *string
	if d := r.Form.Get("description"); d != "" {
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

// PUTMedia handles PUT /api/v1/media/:id (update description and/or focus).
func (h *MediaHandler) PUTMedia(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if err := api.ValidateRequiredField(id, "id"); err != nil {
		api.HandleError(w, r, err)
		return
	}
	// Accept either multipart/form-data or application/x-www-form-urlencoded.
	// If multipart parsing fails (e.g. not multipart), try ParseForm so urlencoded
	// bodies are still read. ParseForm errors are ignored so we proceed with nil
	// description/focus when the body is neither form type.
	if err := r.ParseMultipartForm(0); err != nil { //nolint:gosec // G120: body size limited by upstream MaxBodySize middleware
		_ = r.ParseForm() //nolint:gosec // G120: same as above
	}
	description := optionalFormString(r, "description")
	focusX, focusY, err := parseFocusParam(r.Form.Get("focus"))
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	attachment, err := h.media.Update(r.Context(), account.ID, id, description, focusX, focusY)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	out := apimodel.MediaFromDomain(attachment)
	api.WriteJSON(w, http.StatusOK, out)
}

func optionalFormString(r *http.Request, key string) *string {
	if s := strings.TrimSpace(r.FormValue(key)); s != "" {
		return &s
	}
	return nil
}

// parseFocusParam parses the "focus" form value ("x,y" in range -1.0 to 1.0).
// Returns (nil, nil, nil) when s is empty, (focusX, focusY, err) with a validation error on failure.
func parseFocusParam(s string) (*float64, *float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil, nil
	}
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return nil, nil, fmt.Errorf("parse focus: %w", api.NewInvalidValueError("focus"))
	}
	x, errX := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	y, errY := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if errX != nil || errY != nil {
		return nil, nil, fmt.Errorf("parse focus: %w", api.NewInvalidValueError("focus"))
	}
	if x < -1 || x > 1 || y < -1 || y > 1 {
		return nil, nil, fmt.Errorf("parse focus: %w", api.NewInvalidValueError("focus"))
	}
	return &x, &y, nil
}
