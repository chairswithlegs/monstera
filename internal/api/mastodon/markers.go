package mastodon

import (
	"net/http"
	"time"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/service"
)

// MarkerResponse is the Mastodon API marker entity (nested under timeline key).
type MarkerResponse struct {
	LastReadID string `json:"last_read_id"`
	Version    int    `json:"version"`
	UpdatedAt  string `json:"updated_at"`
}

// MarkersHandler handles GET/POST /api/v1/markers.
type MarkersHandler struct {
	markers service.MarkerService
}

// NewMarkersHandler returns a new MarkersHandler.
func NewMarkersHandler(markers service.MarkerService) *MarkersHandler {
	return &MarkersHandler{markers: markers}
}

// GETMarkers handles GET /api/v1/markers?timeline[]=home&timeline[]=notifications.
func (h *MarkersHandler) GETMarkers(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.NewUnauthorizedError("The access token is invalid"))
		return
	}
	timelines := r.URL.Query()["timeline[]"]
	if len(timelines) == 0 {
		timelines = []string{"home", "notifications"}
	}
	markers, err := h.markers.GetMarkers(r.Context(), account.ID, timelines)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make(map[string]MarkerResponse)
	for k, m := range markers {
		out[k] = MarkerResponse{
			LastReadID: m.LastReadID,
			Version:    m.Version,
			UpdatedAt:  m.UpdatedAt.UTC().Format(time.RFC3339Nano),
		}
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// POSTMarkersRequest is the body for POST /api/v1/markers (form or JSON).
type POSTMarkersRequest struct {
	Home          *MarkerTimelineInput `json:"home"`
	Notifications *MarkerTimelineInput `json:"notifications"`
}

// MarkerTimelineInput holds last_read_id for one timeline.
type MarkerTimelineInput struct {
	LastReadID string `json:"last_read_id"`
}

// POSTMarkers handles POST /api/v1/markers.
func (h *MarkersHandler) POSTMarkers(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.NewUnauthorizedError("The access token is invalid"))
		return
	}
	var body POSTMarkersRequest
	if r.Header.Get("Content-Type") == "application/json" {
		if err := api.DecodeJSONBody(r, &body); err != nil {
			api.HandleError(w, r, err)
			return
		}
	} else {
		if err := r.ParseForm(); err != nil { //nolint:gosec // G120: body size limited by upstream MaxBodySize middleware
			api.HandleError(w, r, api.NewBadRequestError("invalid form"))
			return
		}
		body.Home = formMarkerTimeline(r, "home")
		body.Notifications = formMarkerTimeline(r, "notifications")
	}
	ctx := r.Context()
	var requested []string
	if body.Home != nil && body.Home.LastReadID != "" {
		if err := h.markers.SetMarker(ctx, account.ID, "home", body.Home.LastReadID); err != nil {
			api.HandleError(w, r, err)
			return
		}
		requested = append(requested, "home")
	}
	if body.Notifications != nil && body.Notifications.LastReadID != "" {
		if err := h.markers.SetMarker(ctx, account.ID, "notifications", body.Notifications.LastReadID); err != nil {
			api.HandleError(w, r, err)
			return
		}
		requested = append(requested, "notifications")
	}
	out := make(map[string]MarkerResponse)
	if len(requested) > 0 {
		m, err := h.markers.GetMarkers(ctx, account.ID, requested)
		if err != nil {
			api.HandleError(w, r, err)
			return
		}
		for k, marker := range m {
			out[k] = MarkerResponse{
				LastReadID: marker.LastReadID,
				Version:    marker.Version,
				UpdatedAt:  marker.UpdatedAt.UTC().Format(time.RFC3339Nano),
			}
		}
	}
	api.WriteJSON(w, http.StatusOK, out)
}

func formMarkerTimeline(r *http.Request, timeline string) *MarkerTimelineInput {
	id := r.FormValue(timeline + "[last_read_id]")
	if id == "" {
		return nil
	}
	return &MarkerTimelineInput{LastReadID: id}
}
