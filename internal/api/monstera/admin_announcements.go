package monstera

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/monstera/apimodel"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
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
	var body apimodel.PostAnnouncementRequest
	if err := api.DecodeAndValidateJSON(r, &body); err != nil {
		api.HandleError(w, r, err)
		return
	}
	in := service.CreateAnnouncementInput{
		Content:  body.Content,
		AllDay:   body.AllDay,
		StartsAt: body.ParsedStartsAt,
		EndsAt:   body.ParsedEndsAt,
	}
	a, err := h.announcements.Create(r.Context(), in)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusCreated, domainAnnouncementToAdmin(*a))
}

type putAnnouncementRequest struct {
	Content           *string `json:"content"`
	StartsAt          *string `json:"starts_at"`
	EndsAt            *string `json:"ends_at"`
	AllDay            *bool   `json:"all_day"`
	PublishedAt       *string `json:"published_at"`
	parsedStartsAt    *time.Time
	parsedEndsAt      *time.Time
	parsedPublishedAt time.Time
	hasPublishedAt    bool
}

func (r *putAnnouncementRequest) Validate() error {
	var err error
	r.parsedStartsAt, err = api.ValidateRFC3339Optional(r.StartsAt, "starts_at")
	if err != nil {
		return fmt.Errorf("starts_at: %w", err)
	}
	r.parsedEndsAt, err = api.ValidateRFC3339Optional(r.EndsAt, "ends_at")
	if err != nil {
		return fmt.Errorf("ends_at: %w", err)
	}
	if r.PublishedAt != nil && *r.PublishedAt != "" {
		r.parsedPublishedAt, err = api.ValidateRFC3339(*r.PublishedAt, "published_at")
		if err != nil {
			return fmt.Errorf("published_at: %w", err)
		}
		r.hasPublishedAt = true
	}
	return nil
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
	var body putAnnouncementRequest
	if err := api.DecodeAndValidateJSON(r, &body); err != nil {
		api.HandleError(w, r, err)
		return
	}
	in := service.UpdateAnnouncementInput{
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
		in.StartsAt = body.parsedStartsAt
	}
	if body.EndsAt != nil {
		in.EndsAt = body.parsedEndsAt
	}
	if body.AllDay != nil {
		in.AllDay = *body.AllDay
	}
	if body.hasPublishedAt {
		in.PublishedAt = body.parsedPublishedAt
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
