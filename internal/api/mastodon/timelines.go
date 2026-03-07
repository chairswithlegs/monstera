package mastodon

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
)

// TimelinesHandler handles timeline Mastodon API endpoints.
type TimelinesHandler struct {
	timeline       service.TimelineService
	instanceDomain string
}

// NewTimelinesHandler returns a new TimelinesHandler.
func NewTimelinesHandler(timeline service.TimelineService, instanceDomain string) *TimelinesHandler {
	return &TimelinesHandler{timeline: timeline, instanceDomain: instanceDomain}
}

// GETHome handles GET /api/v1/timelines/home.
func (h *TimelinesHandler) GETHome(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	account := middleware.AccountFromContext(ctx)
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}

	params := PageParamsFromRequest(r)
	maxID := optionalString(params.MaxID)
	enriched, err := h.timeline.HomeEnriched(ctx, account.ID, maxID, params.Limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}

	out := make([]apimodel.Status, 0, len(enriched))
	for i := range enriched {
		e := &enriched[i]
		authorAcc := apimodel.ToAccount(e.Author, h.instanceDomain)
		mentionsResp := make([]apimodel.Mention, 0, len(e.Mentions))
		for _, a := range e.Mentions {
			mentionsResp = append(mentionsResp, apimodel.MentionFromAccount(a, h.instanceDomain))
		}
		tagsResp := make([]apimodel.Tag, 0, len(e.Tags))
		for _, t := range e.Tags {
			tagsResp = append(tagsResp, apimodel.TagFromName(t.Name, h.instanceDomain))
		}
		mediaResp := make([]apimodel.MediaAttachment, 0, len(e.Media))
		for j := range e.Media {
			mediaResp = append(mediaResp, apimodel.MediaFromDomain(&e.Media[j]))
		}
		out = append(out, apimodel.ToStatus(e.Status, authorAcc, mentionsResp, tagsResp, mediaResp, h.instanceDomain, false))
	}

	firstID, lastID := firstLastIDsFromEnriched(enriched)
	if link := LinkHeader(r.URL.String(), firstID, lastID); link != "" {
		w.Header().Set("Link", link)
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// GETPublic handles GET /api/v1/timelines/public. Auth optional.
func (h *TimelinesHandler) GETPublic(w http.ResponseWriter, r *http.Request) {
	params := PageParamsFromRequest(r)
	localOnly := r.URL.Query().Get("local") == "true"
	maxID := optionalString(params.MaxID)
	enriched, err := h.timeline.PublicLocalEnriched(r.Context(), localOnly, maxID, params.Limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Status, 0, len(enriched))
	for i := range enriched {
		e := &enriched[i]
		authorAcc := apimodel.ToAccount(e.Author, h.instanceDomain)
		mentionsResp := make([]apimodel.Mention, 0, len(e.Mentions))
		for _, a := range e.Mentions {
			mentionsResp = append(mentionsResp, apimodel.MentionFromAccount(a, h.instanceDomain))
		}
		tagsResp := make([]apimodel.Tag, 0, len(e.Tags))
		for _, t := range e.Tags {
			tagsResp = append(tagsResp, apimodel.TagFromName(t.Name, h.instanceDomain))
		}
		mediaResp := make([]apimodel.MediaAttachment, 0, len(e.Media))
		for j := range e.Media {
			mediaResp = append(mediaResp, apimodel.MediaFromDomain(&e.Media[j]))
		}
		out = append(out, apimodel.ToStatus(e.Status, authorAcc, mentionsResp, tagsResp, mediaResp, h.instanceDomain, false))
	}
	firstID, lastID := firstLastIDsFromEnriched(enriched)
	if link := LinkHeader(r.URL.String(), firstID, lastID); link != "" {
		w.Header().Set("Link", link)
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// GETFavourites handles GET /api/v1/favourites.
func (h *TimelinesHandler) GETFavourites(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	account := middleware.AccountFromContext(ctx)
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}

	params := PageParamsFromRequest(r)
	maxID := optionalString(params.MaxID)
	enriched, nextCursor, err := h.timeline.FavouritesEnriched(ctx, account.ID, maxID, params.Limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}

	out := make([]apimodel.Status, 0, len(enriched))
	for i := range enriched {
		e := &enriched[i]
		authorAcc := apimodel.ToAccount(e.Author, h.instanceDomain)
		mentionsResp := make([]apimodel.Mention, 0, len(e.Mentions))
		for _, a := range e.Mentions {
			mentionsResp = append(mentionsResp, apimodel.MentionFromAccount(a, h.instanceDomain))
		}
		tagsResp := make([]apimodel.Tag, 0, len(e.Tags))
		for _, t := range e.Tags {
			tagsResp = append(tagsResp, apimodel.TagFromName(t.Name, h.instanceDomain))
		}
		mediaResp := make([]apimodel.MediaAttachment, 0, len(e.Media))
		for j := range e.Media {
			mediaResp = append(mediaResp, apimodel.MediaFromDomain(&e.Media[j]))
		}
		st := apimodel.ToStatus(e.Status, authorAcc, mentionsResp, tagsResp, mediaResp, h.instanceDomain, false)
		st.Favourited = true
		out = append(out, st)
	}

	if len(enriched) > 0 {
		firstID := enriched[0].Status.ID
		if link := linkHeaderFavourites(r.URL.String(), firstID, nextCursor); link != "" {
			w.Header().Set("Link", link)
		}
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// GETBookmarks handles GET /api/v1/bookmarks.
func (h *TimelinesHandler) GETBookmarks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	account := middleware.AccountFromContext(ctx)
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}

	params := PageParamsFromRequest(r)
	maxID := optionalString(params.MaxID)
	enriched, nextCursor, err := h.timeline.BookmarksEnriched(ctx, account.ID, maxID, params.Limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}

	out := make([]apimodel.Status, 0, len(enriched))
	for i := range enriched {
		e := &enriched[i]
		authorAcc := apimodel.ToAccount(e.Author, h.instanceDomain)
		mentionsResp := make([]apimodel.Mention, 0, len(e.Mentions))
		for _, a := range e.Mentions {
			mentionsResp = append(mentionsResp, apimodel.MentionFromAccount(a, h.instanceDomain))
		}
		tagsResp := make([]apimodel.Tag, 0, len(e.Tags))
		for _, t := range e.Tags {
			tagsResp = append(tagsResp, apimodel.TagFromName(t.Name, h.instanceDomain))
		}
		mediaResp := make([]apimodel.MediaAttachment, 0, len(e.Media))
		for j := range e.Media {
			mediaResp = append(mediaResp, apimodel.MediaFromDomain(&e.Media[j]))
		}
		st := apimodel.ToStatus(e.Status, authorAcc, mentionsResp, tagsResp, mediaResp, h.instanceDomain, true)
		out = append(out, st)
	}

	if len(enriched) > 0 {
		firstID := enriched[0].Status.ID
		if link := linkHeaderFavourites(r.URL.String(), firstID, nextCursor); link != "" {
			w.Header().Set("Link", link)
		}
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// GETListTimeline handles GET /api/v1/timelines/list/:id.
func (h *TimelinesHandler) GETListTimeline(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	listID := chi.URLParam(r, "id")
	if listID == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	params := PageParamsFromRequest(r)
	maxID := optionalString(params.MaxID)
	enriched, err := h.timeline.ListTimelineEnriched(r.Context(), account.ID, listID, maxID, params.Limit)
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			api.HandleError(w, r, api.ErrForbidden)
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Status, 0, len(enriched))
	for i := range enriched {
		e := &enriched[i]
		authorAcc := apimodel.ToAccount(e.Author, h.instanceDomain)
		mentionsResp := make([]apimodel.Mention, 0, len(e.Mentions))
		for _, a := range e.Mentions {
			mentionsResp = append(mentionsResp, apimodel.MentionFromAccount(a, h.instanceDomain))
		}
		tagsResp := make([]apimodel.Tag, 0, len(e.Tags))
		for _, t := range e.Tags {
			tagsResp = append(tagsResp, apimodel.TagFromName(t.Name, h.instanceDomain))
		}
		mediaResp := make([]apimodel.MediaAttachment, 0, len(e.Media))
		for j := range e.Media {
			mediaResp = append(mediaResp, apimodel.MediaFromDomain(&e.Media[j]))
		}
		out = append(out, apimodel.ToStatus(e.Status, authorAcc, mentionsResp, tagsResp, mediaResp, h.instanceDomain, false))
	}
	firstID, lastID := firstLastIDsFromEnriched(enriched)
	if link := LinkHeader(r.URL.String(), firstID, lastID); link != "" {
		w.Header().Set("Link", link)
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// GETTag handles GET /api/v1/timelines/tag/:hashtag.
func (h *TimelinesHandler) GETTag(w http.ResponseWriter, r *http.Request) {
	hashtag := strings.TrimLeft(chi.URLParam(r, "hashtag"), "#")
	if hashtag == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	params := PageParamsFromRequest(r)
	maxID := optionalString(params.MaxID)
	enriched, err := h.timeline.HashtagTimelineEnriched(r.Context(), strings.ToLower(hashtag), maxID, params.Limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Status, 0, len(enriched))
	for i := range enriched {
		e := &enriched[i]
		authorAcc := apimodel.ToAccount(e.Author, h.instanceDomain)
		mentionsResp := make([]apimodel.Mention, 0, len(e.Mentions))
		for _, a := range e.Mentions {
			mentionsResp = append(mentionsResp, apimodel.MentionFromAccount(a, h.instanceDomain))
		}
		tagsResp := make([]apimodel.Tag, 0, len(e.Tags))
		for _, t := range e.Tags {
			tagsResp = append(tagsResp, apimodel.TagFromName(t.Name, h.instanceDomain))
		}
		mediaResp := make([]apimodel.MediaAttachment, 0, len(e.Media))
		for j := range e.Media {
			mediaResp = append(mediaResp, apimodel.MediaFromDomain(&e.Media[j]))
		}
		out = append(out, apimodel.ToStatus(e.Status, authorAcc, mentionsResp, tagsResp, mediaResp, h.instanceDomain, false))
	}
	firstID, lastID := firstLastIDsFromEnriched(enriched)
	if link := LinkHeader(r.URL.String(), firstID, lastID); link != "" {
		w.Header().Set("Link", link)
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// firstLastIDsFromEnriched returns the first and last status IDs for Link header pagination.
func firstLastIDsFromEnriched(enriched []service.EnrichedStatus) (firstID, lastID string) {
	if len(enriched) == 0 {
		return "", ""
	}
	return enriched[0].Status.ID, enriched[len(enriched)-1].Status.ID
}
