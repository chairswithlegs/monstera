package mastodon

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
)

// ScheduledStatusesHandler handles GET/PUT/DELETE /api/v1/scheduled_statuses.
type ScheduledStatusesHandler struct {
	statuses service.StatusService
}

// NewScheduledStatusesHandler returns a new ScheduledStatusesHandler.
func NewScheduledStatusesHandler(statuses service.StatusService) *ScheduledStatusesHandler {
	return &ScheduledStatusesHandler{statuses: statuses}
}

// mastodonScheduledParams returns params as a Mastodon-shaped JSON object (with application_id, poll: null, etc.) for client compatibility.
func mastodonScheduledParams(stored json.RawMessage) json.RawMessage {
	var p domain.ScheduledStatusParams
	if len(stored) > 0 {
		if err := json.Unmarshal(stored, &p); err != nil {
			return stored
		}
	}
	out := map[string]any{
		"text":            p.Text,
		"media_ids":       p.MediaIDs,
		"sensitive":       p.Sensitive,
		"spoiler_text":    p.SpoilerText,
		"visibility":      p.Visibility,
		"language":        p.Language,
		"in_reply_to_id":  p.InReplyToID,
		"poll":            nil,
		"application_id":  0,
		"scheduled_at":    nil,
		"idempotency":     nil,
		"with_rate_limit": false,
	}
	if out["spoiler_text"] == "" {
		out["spoiler_text"] = nil
	}
	if out["visibility"] == "" {
		out["visibility"] = nil
	}
	if out["language"] == "" {
		out["language"] = nil
	}
	if out["in_reply_to_id"] == "" {
		out["in_reply_to_id"] = nil
	}
	b, err := json.Marshal(out)
	if err != nil {
		return stored
	}
	return b
}

func scheduledStatusToAPIModel(s *domain.ScheduledStatus) apimodel.ScheduledStatus {
	return apimodel.ScheduledStatus{
		ID:               s.ID,
		ScheduledAt:      s.ScheduledAt.UTC().Format(time.RFC3339),
		Params:           mastodonScheduledParams(s.Params),
		MediaAttachments: nil,
	}
}

// GETScheduledStatuses handles GET /api/v1/scheduled_statuses (list).
func (h *ScheduledStatusesHandler) GETScheduledStatuses(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	account := middleware.AccountFromContext(ctx)
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	maxID := r.URL.Query().Get("max_id")
	var maxIDPtr *string
	if maxID != "" {
		maxIDPtr = &maxID
	}
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 40 {
			limit = n
		}
	}
	list, err := h.statuses.ListScheduledStatuses(ctx, account.ID, maxIDPtr, limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.ScheduledStatus, 0, len(list))
	for i := range list {
		out = append(out, scheduledStatusToAPIModel(&list[i]))
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// GETScheduledStatus handles GET /api/v1/scheduled_statuses/:id.
func (h *ScheduledStatusesHandler) GETScheduledStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	account := middleware.AccountFromContext(ctx)
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	s, err := h.statuses.GetScheduledStatus(ctx, id, account.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, scheduledStatusToAPIModel(s))
}

// UpdateScheduledStatusRequest is the body for PUT /api/v1/scheduled_statuses/:id.
type UpdateScheduledStatusRequest struct {
	ScheduledAt string          `json:"scheduled_at"`
	Params      json.RawMessage `json:"params,omitempty"`
}

// PUTScheduledStatus handles PUT /api/v1/scheduled_statuses/:id.
func (h *ScheduledStatusesHandler) PUTScheduledStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	account := middleware.AccountFromContext(ctx)
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	s, err := h.statuses.GetScheduledStatus(ctx, id, account.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	var req UpdateScheduledStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.HandleError(w, r, api.NewUnprocessableError("invalid JSON"))
		return
	}
	scheduledAt, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil || req.ScheduledAt == "" {
		api.HandleError(w, r, api.NewUnprocessableError("scheduled_at must be a valid ISO8601 datetime"))
		return
	}
	params := s.Params
	if len(req.Params) > 0 {
		params = req.Params
	}
	updated, err := h.statuses.UpdateScheduledStatus(ctx, id, account.ID, params, scheduledAt)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		if errors.Is(err, domain.ErrValidation) {
			api.HandleError(w, r, api.NewUnprocessableError("scheduled_at must be in the future"))
			return
		}
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, scheduledStatusToAPIModel(updated))
}

// DELETEScheduledStatus handles DELETE /api/v1/scheduled_statuses/:id.
func (h *ScheduledStatusesHandler) DELETEScheduledStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	account := middleware.AccountFromContext(ctx)
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	err := h.statuses.DeleteScheduledStatus(ctx, id, account.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
