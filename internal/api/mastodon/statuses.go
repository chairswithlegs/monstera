package mastodon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/cache"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
)

const idempotencyTTL = time.Hour

// StatusesHandler handles status-related Mastodon API endpoints.
type StatusesHandler struct {
	accounts       service.AccountService
	statuses       service.StatusService
	instanceDomain string
	cache          cache.Store // optional; when set, Idempotency-Key is honored
}

// NewStatusesHandler returns a new StatusesHandler. idempotencyCache may be nil to disable idempotency.
func NewStatusesHandler(accounts service.AccountService, statuses service.StatusService, instanceDomain string, idempotencyCache cache.Store) *StatusesHandler {
	return &StatusesHandler{accounts: accounts, statuses: statuses, instanceDomain: instanceDomain, cache: idempotencyCache}
}

type idempotencyCached struct {
	Status int    `json:"status"`
	Body   []byte `json:"body"`
}

// CreateStatusRequest is the request body for POST /api/v1/statuses.
type CreateStatusRequest struct {
	Status      string   `json:"status"`
	Visibility  string   `json:"visibility"`
	SpoilerText string   `json:"spoiler_text"`
	Sensitive   bool     `json:"sensitive"`
	Language    string   `json:"language"`
	InReplyToID string   `json:"in_reply_to_id"`
	MediaIDs    []string `json:"media_ids"`
	ScheduledAt string   `json:"scheduled_at"` // if non-empty, return 422 (Phase 1)
}

// POSTStatuses handles POST /api/v1/statuses.
func (h *StatusesHandler) POSTStatuses(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	account := middleware.AccountFromContext(ctx)
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}

	req, err := parseCreateStatusRequest(r)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}

	if req.ScheduledAt != "" {
		api.HandleError(w, r, api.NewUnprocessableError("Scheduled statuses are not yet supported"))
		return
	}

	idemKey := r.Header.Get("Idempotency-Key")
	if idemKey != "" && h.cache != nil {
		cacheKey := "idempotency:" + account.ID + ":" + idemKey
		b, err := h.cache.Get(ctx, cacheKey)
		if err == nil {
			var cached idempotencyCached
			if json.Unmarshal(b, &cached) == nil {
				w.WriteHeader(cached.Status)
				_, _ = w.Write(cached.Body)
				return
			}
		}
	}

	_, user, err := h.accounts.GetAccountWithUser(ctx, account.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrUnauthorized)
			return
		}
		api.HandleError(w, r, err)
		return
	}

	defaultVisibility := ""
	if user != nil {
		defaultVisibility = user.DefaultPrivacy
	}

	var inReplyToID *string
	if req.InReplyToID != "" {
		inReplyToID = &req.InReplyToID
	}
	mediaIDs := req.MediaIDs
	if len(mediaIDs) > 4 {
		mediaIDs = mediaIDs[:4]
	}

	result, err := h.statuses.CreateWithContent(ctx, service.CreateWithContentInput{
		AccountID:         account.ID,
		Username:          account.Username,
		Text:              req.Status,
		Visibility:        req.Visibility,
		DefaultVisibility: defaultVisibility,
		ContentWarning:    req.SpoilerText,
		Language:          req.Language,
		Sensitive:         req.Sensitive,
		InReplyToID:       inReplyToID,
		MediaIDs:          mediaIDs,
	})
	if err != nil {
		api.HandleError(w, r, err)
		return
	}

	out := enrichedStatusToAPIModel(result, h.instanceDomain)
	body, _ := json.Marshal(out)
	if idemKey != "" && h.cache != nil {
		cacheKey := "idempotency:" + account.ID + ":" + idemKey
		cached, _ := json.Marshal(idempotencyCached{Status: http.StatusOK, Body: body})
		_ = h.cache.Set(ctx, cacheKey, cached, idempotencyTTL)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// GETStatuses handles GET /api/v1/statuses/:id. Auth optional.
func (h *StatusesHandler) GETStatuses(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	var viewerID *string
	if account := middleware.AccountFromContext(r.Context()); account != nil {
		viewerID = &account.ID
	}
	result, err := h.statuses.GetByIDEnriched(r.Context(), id, viewerID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := enrichedStatusToAPIModel(result, h.instanceDomain)
	if account := middleware.AccountFromContext(r.Context()); account != nil {
		if ok, err := h.statuses.IsBookmarked(r.Context(), account.ID, id); err == nil {
			out.Bookmarked = ok
		}
		if result.Status.AccountID == account.ID {
			pinnedIDs, _ := h.statuses.ListPinnedStatusIDs(r.Context(), account.ID)
			for _, pid := range pinnedIDs {
				if pid == id {
					out.Pinned = true
					break
				}
			}
		}
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// POSTPin handles POST /api/v1/statuses/:id/pin.
func (h *StatusesHandler) POSTPin(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	result, err := h.statuses.Pin(r.Context(), account.ID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		if errors.Is(err, domain.ErrForbidden) {
			api.HandleError(w, r, api.ErrForbidden)
			return
		}
		if errors.Is(err, domain.ErrUnprocessable) {
			api.HandleError(w, r, api.NewUnprocessableError("Only public and unlisted statuses can be pinned"))
			return
		}
		api.HandleError(w, r, err)
		return
	}
	out := enrichedStatusToAPIModel(result, h.instanceDomain)
	out.Pinned = true
	api.WriteJSON(w, http.StatusOK, out)
}

// POSTUnpin handles POST /api/v1/statuses/:id/unpin.
func (h *StatusesHandler) POSTUnpin(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	result, err := h.statuses.Unpin(r.Context(), account.ID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		if errors.Is(err, domain.ErrForbidden) {
			api.HandleError(w, r, api.ErrForbidden)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	out := enrichedStatusToAPIModel(result, h.instanceDomain)
	api.WriteJSON(w, http.StatusOK, out)
}

// UpdateStatusRequest is the request body for PUT /api/v1/statuses/:id.
type UpdateStatusRequest struct {
	Status      string `json:"status"`
	SpoilerText string `json:"spoiler_text"`
	Sensitive   bool   `json:"sensitive"`
}

// PUTStatuses handles PUT /api/v1/statuses/:id.
func (h *StatusesHandler) PUTStatuses(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	var req UpdateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.HandleError(w, r, api.NewUnprocessableError("invalid JSON"))
		return
	}
	result, err := h.statuses.UpdateStatusFromAPI(r.Context(), account.ID, id, strings.TrimSpace(req.Status), req.SpoilerText, req.Sensitive)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		if errors.Is(err, domain.ErrForbidden) {
			api.HandleError(w, r, api.ErrForbidden)
			return
		}
		if errors.Is(err, domain.ErrUnprocessable) {
			api.HandleError(w, r, api.NewUnprocessableError("cannot edit this status"))
			return
		}
		if errors.Is(err, domain.ErrValidation) {
			api.HandleError(w, r, api.NewUnprocessableError("invalid or empty status"))
			return
		}
		api.HandleError(w, r, err)
		return
	}
	out := enrichedStatusToAPIModel(result, h.instanceDomain)
	pinnedIDs, _ := h.statuses.ListPinnedStatusIDs(r.Context(), account.ID)
	for _, pid := range pinnedIDs {
		if pid == id {
			out.Pinned = true
			break
		}
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// GETStatusHistory handles GET /api/v1/statuses/:id/history.
func (h *StatusesHandler) GETStatusHistory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	var viewerID *string
	if account := middleware.AccountFromContext(r.Context()); account != nil {
		viewerID = &account.ID
	}
	edits, err := h.statuses.GetStatusHistory(r.Context(), id, viewerID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.StatusEdit, 0, len(edits))
	for _, e := range edits {
		out = append(out, apimodel.StatusEditFromDomain(e))
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// GETStatusSource handles GET /api/v1/statuses/:id/source.
func (h *StatusesHandler) GETStatusSource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	var viewerID *string
	if account := middleware.AccountFromContext(r.Context()); account != nil {
		viewerID = &account.ID
	}
	text, spoiler, err := h.statuses.GetStatusSource(r.Context(), id, viewerID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.StatusSource{
		ID:          id,
		Text:        text,
		SpoilerText: spoiler,
	})
}

// DELETEStatuses handles DELETE /api/v1/statuses/:id. Auth required.
func (h *StatusesHandler) DELETEStatuses(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	st, err := h.statuses.GetByID(r.Context(), id)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	if st.AccountID != account.ID {
		api.HandleError(w, r, api.ErrForbidden)
		return
	}
	result, err := h.statuses.GetByIDEnriched(r.Context(), id, &account.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	if err := h.statuses.Delete(r.Context(), id); err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := enrichedStatusToAPIModel(result, h.instanceDomain)
	api.WriteJSON(w, http.StatusOK, out)
}

// parseCreateStatusRequest parses JSON or form body into CreateStatusRequest.
// Returns an error with a client-safe message on validation or parse failure.
func parseCreateStatusRequest(r *http.Request) (CreateStatusRequest, error) {
	var req CreateStatusRequest
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// nolint:wrapcheck
			return CreateStatusRequest{}, api.NewUnprocessableError("invalid JSON")
		}
	} else {
		if err := r.ParseForm(); err != nil {
			// nolint:wrapcheck
			return CreateStatusRequest{}, api.NewUnprocessableError("invalid form")
		}
		req.Status = r.FormValue("status")
		req.Visibility = r.FormValue("visibility")
		req.SpoilerText = r.FormValue("spoiler_text")
		req.Sensitive = r.FormValue("sensitive") == resolveQueryTrue || r.FormValue("sensitive") == "1"
		req.Language = r.FormValue("language")
		req.InReplyToID = r.FormValue("in_reply_to_id")
		req.ScheduledAt = r.FormValue("scheduled_at")
		if ids := r.Form["media_ids[]"]; len(ids) > 0 {
			req.MediaIDs = ids
		} else if ids := r.Form["media_ids"]; len(ids) > 0 {
			req.MediaIDs = ids
		}
	}
	req.Status = strings.TrimSpace(req.Status)
	if req.Status == "" && len(req.MediaIDs) == 0 {
		// nolint:wrapcheck
		return CreateStatusRequest{}, api.NewUnprocessableError("status cannot be blank")
	}
	return req, nil
}

// POSTReblog handles POST /api/v1/statuses/:id/reblog.
func (h *StatusesHandler) POSTReblog(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	result, err := h.statuses.Reblog(r.Context(), account.ID, account.Username, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		if errors.Is(err, domain.ErrForbidden) {
			api.HandleError(w, r, api.ErrForbidden)
			return
		}
		if errors.Is(err, domain.ErrConflict) {
			api.HandleError(w, r, api.NewUnprocessableError("already reblogged"))
			return
		}
		api.HandleError(w, r, err)
		return
	}
	out := enrichedStatusToAPIModelWithReblog(r.Context(), result, id, &account.ID, h)
	api.WriteJSON(w, http.StatusOK, out)
}

// POSTUnreblog handles POST /api/v1/statuses/:id/unreblog.
func (h *StatusesHandler) POSTUnreblog(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	result, err := h.statuses.Unreblog(r.Context(), account.ID, id)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, enrichedStatusToAPIModel(result, h.instanceDomain))
}

// POSTFavourite handles POST /api/v1/statuses/:id/favourite.
func (h *StatusesHandler) POSTFavourite(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	result, err := h.statuses.Favourite(r.Context(), account.ID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	out := enrichedStatusToAPIModel(result, h.instanceDomain)
	out.Favourited = true
	api.WriteJSON(w, http.StatusOK, out)
}

// POSTUnfavourite handles POST /api/v1/statuses/:id/unfavourite.
func (h *StatusesHandler) POSTUnfavourite(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	result, err := h.statuses.Unfavourite(r.Context(), account.ID, id)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := enrichedStatusToAPIModel(result, h.instanceDomain)
	out.Favourited = false
	api.WriteJSON(w, http.StatusOK, out)
}

// POSTBookmark handles POST /api/v1/statuses/:id/bookmark.
func (h *StatusesHandler) POSTBookmark(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	result, err := h.statuses.Bookmark(r.Context(), account.ID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	out := enrichedStatusToAPIModel(result, h.instanceDomain)
	out.Bookmarked = true
	api.WriteJSON(w, http.StatusOK, out)
}

// POSTUnbookmark handles POST /api/v1/statuses/:id/unbookmark.
func (h *StatusesHandler) POSTUnbookmark(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	result, err := h.statuses.Unbookmark(r.Context(), account.ID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	out := enrichedStatusToAPIModel(result, h.instanceDomain)
	out.Bookmarked = false
	api.WriteJSON(w, http.StatusOK, out)
}

// GETContext handles GET /api/v1/statuses/:id/context.
func (h *StatusesHandler) GETContext(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	var viewerID *string
	if account := middleware.AccountFromContext(r.Context()); account != nil {
		viewerID = &account.ID
	}
	ctxResult, err := h.statuses.GetContext(r.Context(), id, viewerID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	ancestors := make([]apimodel.Status, 0, len(ctxResult.Ancestors))
	for i := range ctxResult.Ancestors {
		enriched, err := h.statuses.GetByIDEnriched(r.Context(), ctxResult.Ancestors[i].ID, viewerID)
		if err != nil {
			continue
		}
		ancestors = append(ancestors, enrichedStatusToAPIModel(enriched, h.instanceDomain))
	}
	descendants := make([]apimodel.Status, 0, len(ctxResult.Descendants))
	for i := range ctxResult.Descendants {
		enriched, err := h.statuses.GetByIDEnriched(r.Context(), ctxResult.Descendants[i].ID, viewerID)
		if err != nil {
			continue
		}
		descendants = append(descendants, enrichedStatusToAPIModel(enriched, h.instanceDomain))
	}
	api.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ancestors":   ancestors,
		"descendants": descendants,
	})
}

// GETFavouritedBy handles GET /api/v1/statuses/:id/favourited_by.
func (h *StatusesHandler) GETFavouritedBy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	var viewerID *string
	if account := middleware.AccountFromContext(r.Context()); account != nil {
		viewerID = &account.ID
	}
	params := PageParamsFromRequest(r)
	list, err := h.statuses.GetFavouritedBy(r.Context(), id, viewerID, optionalString(params.MaxID), params.Limit)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Account, 0, len(list))
	for _, a := range list {
		out = append(out, apimodel.ToAccount(a, h.instanceDomain))
	}
	firstID, lastID := firstLastAccountIDs(list)
	if link := LinkHeader(AbsoluteRequestURL(r, h.instanceDomain), firstID, lastID); link != "" {
		w.Header().Set("Link", link)
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// GETRebloggedBy handles GET /api/v1/statuses/:id/reblogged_by.
func (h *StatusesHandler) GETRebloggedBy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	var viewerID *string
	if account := middleware.AccountFromContext(r.Context()); account != nil {
		viewerID = &account.ID
	}
	params := PageParamsFromRequest(r)
	list, err := h.statuses.GetRebloggedBy(r.Context(), id, viewerID, optionalString(params.MaxID), params.Limit)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Account, 0, len(list))
	for _, a := range list {
		out = append(out, apimodel.ToAccount(a, h.instanceDomain))
	}
	firstID, lastID := firstLastAccountIDs(list)
	if link := LinkHeader(AbsoluteRequestURL(r, h.instanceDomain), firstID, lastID); link != "" {
		w.Header().Set("Link", link)
	}
	api.WriteJSON(w, http.StatusOK, out)
}

func firstLastAccountIDs(list []*domain.Account) (firstID, lastID string) {
	if len(list) == 0 {
		return "", ""
	}
	return list[0].ID, list[len(list)-1].ID
}

// enrichedStatusToAPIModelWithReblog returns the boost status with nested reblog (original status).
func enrichedStatusToAPIModelWithReblog(ctx context.Context, result service.EnrichedStatus, originalID string, viewerID *string, h *StatusesHandler) apimodel.Status {
	boost := enrichedStatusToAPIModel(result, h.instanceDomain)
	if result.Status.ReblogOfID != nil && *result.Status.ReblogOfID == originalID {
		origResult, err := h.statuses.GetByIDEnriched(ctx, *result.Status.ReblogOfID, viewerID)
		if err == nil {
			reblogAPI := enrichedStatusToAPIModel(origResult, h.instanceDomain)
			boost.Reblog = &reblogAPI
		}
	}
	return boost
}

// enrichedStatusToAPIModel maps service.EnrichedStatus to apimodel.Status.
func enrichedStatusToAPIModel(result service.EnrichedStatus, instanceDomain string) apimodel.Status {
	authorAcc := apimodel.ToAccount(result.Author, instanceDomain)
	mentionsResp := make([]apimodel.Mention, 0, len(result.Mentions))
	for _, a := range result.Mentions {
		mentionsResp = append(mentionsResp, apimodel.MentionFromAccount(a, instanceDomain))
	}
	tagsResp := make([]apimodel.Tag, 0, len(result.Tags))
	for _, t := range result.Tags {
		tagsResp = append(tagsResp, apimodel.TagFromName(t.Name, instanceDomain))
	}
	mediaResp := make([]apimodel.MediaAttachment, 0, len(result.Media))
	for i := range result.Media {
		mediaResp = append(mediaResp, apimodel.MediaFromDomain(&result.Media[i]))
	}
	return apimodel.ToStatus(result.Status, authorAcc, mentionsResp, tagsResp, mediaResp, instanceDomain)
}
