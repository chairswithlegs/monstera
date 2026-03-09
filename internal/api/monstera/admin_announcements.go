package monstera

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/monstera/apimodel"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// AdminAnnouncementsHandler handles Monstera admin announcement endpoints.
type AdminAnnouncementsHandler struct {
	announcements service.AnnouncementService
}

// NewAdminAnnouncementsHandler returns a new AdminAnnouncementsHandler.
func NewAdminAnnouncementsHandler(announcements service.AnnouncementService) *AdminAnnouncementsHandler {
	return &AdminAnnouncementsHandler{announcements: announcements}
}

// GETAnnouncements handles GET /admin/announcements (list all).
func (h *AdminAnnouncementsHandler) GETAnnouncements(w http.ResponseWriter, r *http.Request) {
	list, err := h.announcements.ListAll(r.Context())
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.AdminAnnouncement, 0, len(list))
	for _, a := range list {
		out = append(out, domainAnnouncementToAdmin(a))
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// POSTAnnouncements handles POST /admin/announcements (create).
func (h *AdminAnnouncementsHandler) POSTAnnouncements(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Content  string  `json:"content"`
		StartsAt *string `json:"starts_at"`
		EndsAt   *string `json:"ends_at"`
		AllDay   bool    `json:"all_day"`
	}
	if err := api.DecodeJSONBody(r, &body); err != nil {
		api.HandleError(w, r, api.NewBadRequestError("invalid JSON"))
		return
	}
	if body.Content == "" {
		api.HandleError(w, r, api.NewBadRequestError("content is required"))
		return
	}
	now := time.Now().UTC()
	in := store.CreateAnnouncementInput{
		ID:          uid.New(),
		Content:     body.Content,
		AllDay:      body.AllDay,
		PublishedAt: now,
	}
	if body.StartsAt != nil && *body.StartsAt != "" {
		t, err := time.Parse(time.RFC3339, *body.StartsAt)
		if err != nil {
			api.HandleError(w, r, api.NewBadRequestError("invalid starts_at"))
			return
		}
		in.StartsAt = &t
	}
	if body.EndsAt != nil && *body.EndsAt != "" {
		t, err := time.Parse(time.RFC3339, *body.EndsAt)
		if err != nil {
			api.HandleError(w, r, api.NewBadRequestError("invalid ends_at"))
			return
		}
		in.EndsAt = &t
	}
	a, err := h.announcements.Create(r.Context(), in)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusCreated, domainAnnouncementToAdmin(*a))
}

// PUTAnnouncement handles PUT /admin/announcements/:id (update).
func (h *AdminAnnouncementsHandler) PUTAnnouncement(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	a, err := h.announcements.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	var body struct {
		Content     *string `json:"content"`
		StartsAt    *string `json:"starts_at"`
		EndsAt      *string `json:"ends_at"`
		AllDay      *bool   `json:"all_day"`
		PublishedAt *string `json:"published_at"`
	}
	if err := api.DecodeJSONBody(r, &body); err != nil {
		api.HandleError(w, r, api.NewBadRequestError("invalid JSON"))
		return
	}
	in := store.UpdateAnnouncementInput{
		ID:          id,
		Content:     a.Content,
		StartsAt:    a.StartsAt,
		EndsAt:      a.EndsAt,
		AllDay:      a.AllDay,
		PublishedAt: a.PublishedAt,
	}
	if body.Content != nil {
		in.Content = *body.Content
	}
	if body.StartsAt != nil {
		if *body.StartsAt == "" {
			in.StartsAt = nil
		} else {
			t, err := time.Parse(time.RFC3339, *body.StartsAt)
			if err != nil {
				api.HandleError(w, r, api.NewBadRequestError("invalid starts_at"))
				return
			}
			in.StartsAt = &t
		}
	}
	if body.EndsAt != nil {
		if *body.EndsAt == "" {
			in.EndsAt = nil
		} else {
			t, err := time.Parse(time.RFC3339, *body.EndsAt)
			if err != nil {
				api.HandleError(w, r, api.NewBadRequestError("invalid ends_at"))
				return
			}
			in.EndsAt = &t
		}
	}
	if body.AllDay != nil {
		in.AllDay = *body.AllDay
	}
	if body.PublishedAt != nil && *body.PublishedAt != "" {
		t, err := time.Parse(time.RFC3339, *body.PublishedAt)
		if err != nil {
			api.HandleError(w, r, api.NewBadRequestError("invalid published_at"))
			return
		}
		in.PublishedAt = t
	}
	if err := h.announcements.Update(r.Context(), in); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	updated, _ := h.announcements.GetByID(r.Context(), id)
	if updated != nil {
		api.WriteJSON(w, http.StatusOK, domainAnnouncementToAdmin(*updated))
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

func domainAnnouncementToAdmin(a domain.Announcement) apimodel.AdminAnnouncement {
	out := apimodel.AdminAnnouncement{
		ID:          a.ID,
		Content:     a.Content,
		AllDay:      a.AllDay,
		PublishedAt: a.PublishedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   a.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if a.StartsAt != nil {
		s := a.StartsAt.UTC().Format(time.RFC3339)
		out.StartsAt = &s
	}
	if a.EndsAt != nil {
		e := a.EndsAt.UTC().Format(time.RFC3339)
		out.EndsAt = &e
	}
	return out
}
