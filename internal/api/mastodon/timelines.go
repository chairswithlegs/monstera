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

	out := enrichedStatusesToAPIModels(enriched, h.instanceDomain)

	firstID, lastID := firstLastIDsFromEnriched(enriched)
	if link := LinkHeader(AbsoluteRequestURL(r, h.instanceDomain), firstID, lastID); link != "" {
		w.Header().Set("Link", link)
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// GETPublic handles GET /api/v1/timelines/public. Auth optional.
func (h *TimelinesHandler) GETPublic(w http.ResponseWriter, r *http.Request) {
	params := PageParamsFromRequest(r)
	localOnly := api.QueryParamIsTrue(r, "local")
	maxID := optionalString(params.MaxID)
	var viewerID *string
	if account := middleware.AccountFromContext(r.Context()); account != nil {
		viewerID = &account.ID
	}
	enriched, err := h.timeline.PublicLocalEnriched(r.Context(), localOnly, viewerID, maxID, params.Limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := enrichedStatusesToAPIModels(enriched, h.instanceDomain)
	firstID, lastID := firstLastIDsFromEnriched(enriched)
	if link := LinkHeader(AbsoluteRequestURL(r, h.instanceDomain), firstID, lastID); link != "" {
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

	out := enrichedStatusesToAPIModels(enriched, h.instanceDomain)
	for i := range out {
		out[i].Favourited = true
	}

	if len(enriched) > 0 {
		firstID := enriched[0].Status.ID
		if link := linkHeaderFavourites(AbsoluteRequestURL(r, h.instanceDomain), firstID, nextCursor); link != "" {
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

	out := enrichedStatusesToAPIModels(enriched, h.instanceDomain)
	for i := range out {
		out[i].Bookmarked = true
	}

	if len(enriched) > 0 {
		firstID := enriched[0].Status.ID
		if link := linkHeaderFavourites(AbsoluteRequestURL(r, h.instanceDomain), firstID, nextCursor); link != "" {
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
	out := enrichedStatusesToAPIModels(enriched, h.instanceDomain)
	firstID, lastID := firstLastIDsFromEnriched(enriched)
	if link := LinkHeader(AbsoluteRequestURL(r, h.instanceDomain), firstID, lastID); link != "" {
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
	var viewerID *string
	if account := middleware.AccountFromContext(r.Context()); account != nil {
		viewerID = &account.ID
	}
	enriched, err := h.timeline.HashtagTimelineEnriched(r.Context(), strings.ToLower(hashtag), viewerID, maxID, params.Limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := enrichedStatusesToAPIModels(enriched, h.instanceDomain)
	firstID, lastID := firstLastIDsFromEnriched(enriched)
	if link := LinkHeader(AbsoluteRequestURL(r, h.instanceDomain), firstID, lastID); link != "" {
		w.Header().Set("Link", link)
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// enrichedStatusesToAPIModels converts a slice of EnrichedStatus to API models.
func enrichedStatusesToAPIModels(enriched []service.EnrichedStatus, instanceDomain string) []apimodel.Status {
	out := make([]apimodel.Status, 0, len(enriched))
	for i := range enriched {
		out = append(out, enrichedStatusToAPIModel(enriched[i], instanceDomain))
	}
	return out
}

// firstLastIDsFromEnriched returns the first and last status IDs for Link header pagination.
func firstLastIDsFromEnriched(enriched []service.EnrichedStatus) (firstID, lastID string) {
	if len(enriched) == 0 {
		return "", ""
	}
	return enriched[0].Status.ID, enriched[len(enriched)-1].Status.ID
}
