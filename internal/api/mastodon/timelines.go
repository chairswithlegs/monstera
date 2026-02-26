package mastodon

import (
	"log/slog"
	"net/http"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera-fed/internal/api/middleware"
	"github.com/chairswithlegs/monstera-fed/internal/service"
)

// TimelinesHandler handles timeline Mastodon API endpoints.
type TimelinesHandler struct {
	timeline *service.TimelineService
	logger   *slog.Logger
	domain   string
}

// NewTimelinesHandler returns a new TimelinesHandler.
func NewTimelinesHandler(timeline *service.TimelineService, logger *slog.Logger, instanceDomain string) *TimelinesHandler {
	return &TimelinesHandler{
		timeline: timeline,
		logger:   logger,
		domain:   instanceDomain,
	}
}

// Home handles GET /api/v1/timelines/home.
func (h *TimelinesHandler) Home(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	account := middleware.AccountFromContext(ctx)
	if account == nil {
		api.WriteError(w, http.StatusUnauthorized, "The access token is invalid")
		return
	}

	params := PageParamsFromRequest(r)
	maxID := optionalString(params.MaxID)
	enriched, err := h.timeline.HomeEnriched(ctx, account.ID, maxID, params.Limit)
	if err != nil {
		api.HandleError(w, r, h.logger, err)
		return
	}

	out := make([]apimodel.Status, 0, len(enriched))
	for i := range enriched {
		e := &enriched[i]
		authorAcc := apimodel.ToAccount(e.Author, h.domain)
		mentionsResp := make([]apimodel.Mention, 0, len(e.Mentions))
		for _, a := range e.Mentions {
			mentionsResp = append(mentionsResp, apimodel.MentionFromAccount(a, h.domain))
		}
		tagsResp := make([]apimodel.Tag, 0, len(e.Tags))
		for _, t := range e.Tags {
			tagsResp = append(tagsResp, apimodel.TagFromName(t.Name, h.domain))
		}
		mediaResp := make([]apimodel.MediaAttachment, 0, len(e.Media))
		for j := range e.Media {
			mediaResp = append(mediaResp, apimodel.MediaFromDomain(&e.Media[j]))
		}
		out = append(out, apimodel.ToStatus(e.Status, authorAcc, mentionsResp, tagsResp, mediaResp, h.domain))
	}

	firstID, lastID := firstLastIDsFromEnriched(enriched)
	if link := LinkHeader(r.URL.String(), firstID, lastID); link != "" {
		w.Header().Set("Link", link)
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// Public handles GET /api/v1/timelines/public. Auth optional.
func (h *TimelinesHandler) Public(w http.ResponseWriter, r *http.Request) {
	params := PageParamsFromRequest(r)
	localOnly := r.URL.Query().Get("local") == "true"
	maxID := optionalString(params.MaxID)
	enriched, err := h.timeline.PublicLocalEnriched(r.Context(), localOnly, maxID, params.Limit)
	if err != nil {
		api.HandleError(w, r, h.logger, err)
		return
	}
	out := make([]apimodel.Status, 0, len(enriched))
	for i := range enriched {
		e := &enriched[i]
		authorAcc := apimodel.ToAccount(e.Author, h.domain)
		mentionsResp := make([]apimodel.Mention, 0, len(e.Mentions))
		for _, a := range e.Mentions {
			mentionsResp = append(mentionsResp, apimodel.MentionFromAccount(a, h.domain))
		}
		tagsResp := make([]apimodel.Tag, 0, len(e.Tags))
		for _, t := range e.Tags {
			tagsResp = append(tagsResp, apimodel.TagFromName(t.Name, h.domain))
		}
		mediaResp := make([]apimodel.MediaAttachment, 0, len(e.Media))
		for j := range e.Media {
			mediaResp = append(mediaResp, apimodel.MediaFromDomain(&e.Media[j]))
		}
		out = append(out, apimodel.ToStatus(e.Status, authorAcc, mentionsResp, tagsResp, mediaResp, h.domain))
	}
	firstID, lastID := firstLastIDsFromEnriched(enriched)
	if link := LinkHeader(r.URL.String(), firstID, lastID); link != "" {
		w.Header().Set("Link", link)
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// optionalString returns a pointer to s if non-empty, otherwise nil.
func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// firstLastIDsFromEnriched returns the first and last status IDs for Link header pagination.
func firstLastIDsFromEnriched(enriched []service.EnrichedStatus) (firstID, lastID string) {
	if len(enriched) == 0 {
		return "", ""
	}
	return enriched[0].Status.ID, enriched[len(enriched)-1].Status.ID
}
