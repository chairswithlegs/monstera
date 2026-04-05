package mastodon

import (
	"log/slog"
	"net/http"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/service"
)

// TrendsHandler handles Mastodon trends API endpoints.
type TrendsHandler struct {
	svc            service.TrendsService
	tagFollows     service.TagFollowService
	instanceDomain string
}

// NewTrendsHandler returns a new TrendsHandler.
func NewTrendsHandler(svc service.TrendsService, tagFollows service.TagFollowService, instanceDomain string) *TrendsHandler {
	return &TrendsHandler{svc: svc, tagFollows: tagFollows, instanceDomain: instanceDomain}
}

// GETTrendsStatuses handles GET /api/v1/trends/statuses.
func (h *TrendsHandler) GETTrendsStatuses(w http.ResponseWriter, r *http.Request) {
	offset := parseOffsetParam(r)
	limit := parseLimitParam(r, 20, 40)
	enriched, err := h.svc.TrendingStatuses(r.Context(), offset, limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}

	out := make([]apimodel.Status, 0, len(enriched))
	for i := range enriched {
		out = append(out, apimodel.StatusFromEnriched(enriched[i], h.instanceDomain))
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// GETTrendsTags handles GET /api/v1/trends/tags.
func (h *TrendsHandler) GETTrendsTags(w http.ResponseWriter, r *http.Request) {
	offset := parseOffsetParam(r)
	limit := parseLimitParam(r, 10, 40)
	tags, err := h.svc.TrendingTags(r.Context(), offset, limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}

	// For authenticated users, look up which of the trending tag names the user
	// follows. This is bounded by the trending list size (≤ 40), not follow count.
	// Unauthenticated requests omit the following field entirely.
	var followedNames map[string]bool
	if account := middleware.AccountFromContext(r.Context()); account != nil {
		names := make([]string, 0, len(tags))
		for _, t := range tags {
			names = append(names, t.Hashtag.Name)
		}
		fm, err := h.tagFollows.AreFollowingTagsByName(r.Context(), account.ID, names)
		if err != nil {
			slog.WarnContext(r.Context(), "failed to fetch followed tags for trends response", slog.Any("error", err))
		} else {
			followedNames = fm
		}
	}

	out := make([]*apimodel.Tag, 0, len(tags))
	for _, t := range tags {
		out = append(out, apimodel.TrendingTagFromDomain(t, h.instanceDomain, followedNames))
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// GETTrendsLinks handles GET /api/v1/trends/links.
func (h *TrendsHandler) GETTrendsLinks(w http.ResponseWriter, r *http.Request) {
	offset := parseOffsetParam(r)
	limit := parseLimitParam(r, 10, 40)
	links, err := h.svc.TrendingLinks(r.Context(), offset, limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}

	out := make([]apimodel.TrendingLink, 0, len(links))
	for _, l := range links {
		out = append(out, apimodel.TrendingLinkFromDomain(l))
	}
	api.WriteJSON(w, http.StatusOK, out)
}
