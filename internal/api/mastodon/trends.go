package mastodon

import (
	"net/http"
	"strconv"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera/internal/service"
)

// TrendsHandler handles Mastodon trends API endpoints.
type TrendsHandler struct {
	svc            service.TrendsService
	instanceDomain string
}

// NewTrendsHandler returns a new TrendsHandler.
func NewTrendsHandler(svc service.TrendsService, instanceDomain string) *TrendsHandler {
	return &TrendsHandler{svc: svc, instanceDomain: instanceDomain}
}

// GETTrendsStatuses handles GET /api/v1/trends/statuses.
func (h *TrendsHandler) GETTrendsStatuses(w http.ResponseWriter, r *http.Request) {
	limit := parseTrendsLimit(r, 20)
	enriched, err := h.svc.TrendingStatuses(r.Context(), limit)
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
		out = append(out, apimodel.ToStatus(e.Status, authorAcc, mentionsResp, tagsResp, mediaResp, e.Card, h.instanceDomain))
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// GETTrendsTags handles GET /api/v1/trends/tags.
func (h *TrendsHandler) GETTrendsTags(w http.ResponseWriter, r *http.Request) {
	limit := parseTrendsLimit(r, 10)
	tags, err := h.svc.TrendingTags(r.Context(), limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}

	out := make([]*apimodel.Tag, 0, len(tags))
	for _, t := range tags {
		out = append(out, apimodel.TrendingTagFromDomain(t, h.instanceDomain))
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// GETTrendsLinks handles GET /api/v1/trends/links.
// Deferred — OGP parsing not implemented.
func (h *TrendsHandler) GETTrendsLinks(w http.ResponseWriter, r *http.Request) {
	api.WriteJSON(w, http.StatusOK, []any{})
}

func parseTrendsLimit(r *http.Request, defaultLimit int) int {
	s := r.URL.Query().Get("limit")
	if s == "" {
		return defaultLimit
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return defaultLimit
	}
	if n > 40 {
		return 40
	}
	return n
}
