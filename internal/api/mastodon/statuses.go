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

// Default poll limits when instance configuration is not provided (Mastodon-compatible defaults).
const (
	DefaultPollMaxOptions    = 4
	DefaultPollMinExpiration = 300
	DefaultPollMaxExpiration = 2629746
)

// StatusesHandler handles status-related Mastodon API endpoints.
type StatusesHandler struct {
	accounts       service.AccountService
	statuses       service.StatusService
	statusWrites   service.StatusWriteService
	instanceDomain string
	cache          cache.Store // optional; when set, Idempotency-Key is honored
	pollLimits     *service.PollLimits
}

// NewStatusesHandler returns a new StatusesHandler. idempotencyCache may be nil to disable idempotency. pollLimits may be nil to use defaults.
func NewStatusesHandler(accounts service.AccountService, statuses service.StatusService, statusWrites service.StatusWriteService, instanceDomain string, idempotencyCache cache.Store, pollLimits *service.PollLimits) *StatusesHandler {
	if pollLimits == nil {
		pollLimits = &service.PollLimits{
			MaxOptions:    DefaultPollMaxOptions,
			MinExpiration: DefaultPollMinExpiration,
			MaxExpiration: DefaultPollMaxExpiration,
		}
	}
	return &StatusesHandler{accounts: accounts, statuses: statuses, statusWrites: statusWrites, instanceDomain: instanceDomain, cache: idempotencyCache, pollLimits: pollLimits}
}

type idempotencyCached struct {
	Status int    `json:"status"`
	Body   []byte `json:"body"`
}

// POSTStatuses handles POST /api/v1/statuses.
func (h *StatusesHandler) POSTStatuses(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	account := middleware.AccountFromContext(ctx)
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}

	// Statuses can be created with JSON or form body,
	// hence the special ParseCreateStatusRequest function.
	req, err := apimodel.ParseCreateStatusRequest(r)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}

	if req.ScheduledAt != "" {
		scheduledAt, err := time.Parse(time.RFC3339, req.ScheduledAt)
		if err != nil {
			api.HandleError(w, r, api.NewUnprocessableError("scheduled_at must be a valid ISO8601 datetime"))
			return
		}
		params := domain.ScheduledStatusParams{
			Text:        req.Status,
			Visibility:  req.Visibility,
			SpoilerText: req.SpoilerText,
			Sensitive:   req.Sensitive,
			Language:    req.Language,
			InReplyToID: req.InReplyToID,
			MediaIDs:    req.MediaIDs,
		}
		if params.Language == "" {
			params.Language = "en"
		}
		paramsJSON, err := json.Marshal(params)
		if err != nil {
			api.HandleError(w, r, err)
			return
		}
		s, err := h.statuses.CreateScheduledStatus(ctx, account.ID, paramsJSON, scheduledAt)
		if err != nil {
			if errors.Is(err, domain.ErrValidation) {
				api.HandleError(w, r, api.NewUnprocessableError("scheduled_at must be in the future"))
				return
			}
			api.HandleError(w, r, err)
			return
		}
		out := apimodel.ScheduledStatus{
			ID:               s.ID,
			ScheduledAt:      s.ScheduledAt.UTC().Format(time.RFC3339),
			Params:           mastodonScheduledParams(s.Params),
			MediaAttachments: nil,
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(out)
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

	_, _, err = h.accounts.GetAccountWithUser(ctx, account.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrUnauthorized)
			return
		}
		api.HandleError(w, r, err)
		return
	}

	var inReplyToID *string
	if req.InReplyToID != "" {
		inReplyToID = &req.InReplyToID
	}
	var quotedStatusID *string
	if req.QuotedStatusID != "" {
		quotedStatusID = &req.QuotedStatusID
	}

	createInput := service.CreateWithContentInput{
		AccountID:           account.ID,
		Username:            account.Username,
		Text:                req.Status,
		Visibility:          req.Visibility,
		ContentWarning:      req.SpoilerText,
		Language:            req.Language,
		Sensitive:           req.Sensitive,
		InReplyToID:         inReplyToID,
		QuotedStatusID:      quotedStatusID,
		QuoteApprovalPolicy: req.QuoteApprovalPolicy,
		MediaIDs:            req.MediaIDs,
	}
	if req.Poll != nil && len(req.Poll.Options) > 0 {
		createInput.Poll = &service.PollInput{
			Options:          req.Poll.Options,
			ExpiresInSeconds: req.Poll.ExpiresIn,
			Multiple:         req.Poll.Multiple,
		}
		createInput.PollLimits = h.pollLimits
	}
	result, err := h.statusWrites.Create(ctx, createInput)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}

	out := enrichedStatusToAPIModel(result, h.instanceDomain)
	h.setQuoteApprovalOnStatus(ctx, result, &out, &account.ID)
	body, _ := json.Marshal(out)
	if idemKey != "" && h.cache != nil {
		cacheKey := "idempotency:" + account.ID + ":" + idemKey
		cached, _ := json.Marshal(idempotencyCached{Status: http.StatusCreated, Body: body})
		_ = h.cache.Set(ctx, cacheKey, cached, idempotencyTTL)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
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
	h.setQuoteApprovalOnStatus(r.Context(), result, &out, viewerID)
	api.WriteJSON(w, http.StatusOK, out)
}

// GETQuotes handles GET /api/v1/statuses/:id/quotes (Mastodon-style quotes). Auth required. Returns statuses that quote the given status.
func (h *StatusesHandler) GETQuotes(w http.ResponseWriter, r *http.Request) {
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
	params := PageParamsFromRequest(r)
	maxID := optionalString(params.MaxID)
	limit := params.Limit
	if limit <= 0 {
		limit = DefaultPageLimit
	}
	if limit > MaxPageLimit {
		limit = MaxPageLimit
	}
	enriched, err := h.statuses.ListQuotesOfStatus(r.Context(), id, maxID, limit, &account.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Status, 0, len(enriched))
	for i := range enriched {
		s := enrichedStatusToAPIModel(enriched[i], h.instanceDomain)
		h.setQuoteApprovalOnStatus(r.Context(), enriched[i], &s, &account.ID)
		out = append(out, s)
	}
	firstID, lastID := firstLastIDsFromEnriched(enriched)
	if link := LinkHeader(AbsoluteRequestURL(r, h.instanceDomain), firstID, lastID); link != "" {
		w.Header().Set("Link", link)
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// POSTRevokeQuote handles POST /api/v1/statuses/:id/quotes/:quoting_status_id/revoke (Mastodon-style quotes).
// Caller must be the author of the quoted status (:id). Revokes the quote approval for the quoting status.
func (h *StatusesHandler) POSTRevokeQuote(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	quotedStatusID := chi.URLParam(r, "id")
	quotingStatusID := chi.URLParam(r, "quoting_status_id")
	if quotedStatusID == "" || quotingStatusID == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	if err := h.statuses.RevokeQuote(r.Context(), account.ID, quotedStatusID, quotingStatusID); err != nil {
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
	w.WriteHeader(http.StatusNoContent)
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

// POSTMuteConversation handles POST /api/v1/statuses/:id/mute (thread mute).
func (h *StatusesHandler) POSTMuteConversation(w http.ResponseWriter, r *http.Request) {
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
	if err := h.statuses.MuteConversation(r.Context(), account.ID, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	viewerID := &account.ID
	result, err := h.statuses.GetByIDEnriched(r.Context(), id, viewerID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := enrichedStatusToAPIModel(result, h.instanceDomain)
	out.Muted = true
	api.WriteJSON(w, http.StatusOK, out)
}

// POSTUnmuteConversation handles POST /api/v1/statuses/:id/unmute (thread unmute).
func (h *StatusesHandler) POSTUnmuteConversation(w http.ResponseWriter, r *http.Request) {
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
	if err := h.statuses.UnmuteConversation(r.Context(), account.ID, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	viewerID := &account.ID
	result, err := h.statuses.GetByIDEnriched(r.Context(), id, viewerID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := enrichedStatusToAPIModel(result, h.instanceDomain)
	out.Muted = false
	api.WriteJSON(w, http.StatusOK, out)
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
	var req apimodel.UpdateStatusRequest
	if err := api.DecodeJSONBody(r, &req); err != nil {
		api.HandleError(w, r, err)
		return
	}
	result, err := h.statusWrites.Update(r.Context(), account.ID, id, strings.TrimSpace(req.Status), req.SpoilerText, req.Sensitive)
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
	h.setQuoteApprovalOnStatus(r.Context(), result, &out, &account.ID)
	pinnedIDs, _ := h.statuses.ListPinnedStatusIDs(r.Context(), account.ID)
	for _, pid := range pinnedIDs {
		if pid == id {
			out.Pinned = true
			break
		}
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// PUTInteractionPolicy handles PUT /api/v1/statuses/:id/interaction_policy (Mastodon-style quotes).
// Updates the status quote_approval_policy (public, followers, or nobody). Caller must own the status.
func (h *StatusesHandler) PUTInteractionPolicy(w http.ResponseWriter, r *http.Request) {
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
	var req apimodel.PUTInteractionPolicyRequest
	if err := api.DecodeAndValidateJSON(r, &req); err != nil {
		api.HandleError(w, r, err)
		return
	}
	policy := req.QuoteApprovalPolicy
	if err := h.statuses.UpdateQuoteApprovalPolicy(r.Context(), account.ID, id, policy); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		if errors.Is(err, domain.ErrForbidden) {
			api.HandleError(w, r, api.ErrForbidden)
			return
		}
		if errors.Is(err, domain.ErrValidation) {
			api.HandleError(w, r, api.NewUnprocessableError("quote_approval_policy must be public, followers, or nobody"))
			return
		}
		api.HandleError(w, r, err)
		return
	}
	result, err := h.statuses.GetByIDEnriched(r.Context(), id, &account.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := enrichedStatusToAPIModel(result, h.instanceDomain)
	h.setQuoteApprovalOnStatus(r.Context(), result, &out, &account.ID)
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
	if len(edits) > 0 {
		author, err := h.accounts.GetByID(r.Context(), edits[0].AccountID)
		if err != nil {
			api.HandleError(w, r, err)
			return
		}
		authorAPI := apimodel.ToAccount(author, h.instanceDomain)
		for _, e := range edits {
			out = append(out, apimodel.StatusEditFromDomain(e, authorAPI))
		}
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
	if err := h.statusWrites.Delete(r.Context(), id); err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := enrichedStatusToAPIModel(result, h.instanceDomain)
	api.WriteJSON(w, http.StatusOK, out)
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
	result, err := h.statusWrites.Reblog(r.Context(), account.ID, account.Username, id)
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
	if err := h.statusWrites.Unreblog(r.Context(), account.ID, id); err != nil {
		api.HandleError(w, r, err)
		return
	}
	result, err := h.statuses.GetByIDEnriched(r.Context(), id, &account.ID)
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
	result, err := h.statusWrites.Favourite(r.Context(), account.ID, id)
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
	result, err := h.statusWrites.Unfavourite(r.Context(), account.ID, id)
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

// enrichedStatusToAPIModelWithReblog returns the reblog status with nested original status.
func enrichedStatusToAPIModelWithReblog(ctx context.Context, result service.EnrichedStatus, originalID string, viewerID *string, h *StatusesHandler) apimodel.Status {
	reblog := enrichedStatusToAPIModel(result, h.instanceDomain)
	if result.Status.ReblogOfID != nil && *result.Status.ReblogOfID == originalID {
		origResult, err := h.statuses.GetByIDEnriched(ctx, *result.Status.ReblogOfID, viewerID)
		if err == nil {
			reblogAPI := enrichedStatusToAPIModel(origResult, h.instanceDomain)
			reblog.Reblog = &reblogAPI
		}
	}
	return reblog
}

// setQuoteApprovalOnStatus populates out.QuoteApproval when the status is a quote (has QuotedStatusID).
func (h *StatusesHandler) setQuoteApprovalOnStatus(ctx context.Context, result service.EnrichedStatus, out *apimodel.Status, viewerID *string) {
	if result.Status.QuotedStatusID == nil || *result.Status.QuotedStatusID == "" {
		return
	}
	rec, err := h.statuses.GetQuoteApproval(ctx, result.Status.ID)
	if err != nil {
		return
	}
	state := "accepted"
	if rec.RevokedAt != nil {
		state = "revoked"
	}
	var quoted *apimodel.Status
	if state == "accepted" {
		quotedEnriched, err := h.statuses.GetByIDEnriched(ctx, *result.Status.QuotedStatusID, viewerID)
		if err == nil {
			q := enrichedStatusToAPIModel(quotedEnriched, h.instanceDomain)
			q.QuoteApproval = nil
			quoted = &q
		}
	}
	out.QuoteApproval = &apimodel.QuoteApproval{State: state, QuotedStatus: quoted}
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
	out := apimodel.ToStatus(result.Status, authorAcc, mentionsResp, tagsResp, mediaResp, result.Card, instanceDomain)
	if result.Poll != nil {
		p := enrichedPollToAPIModel(result.Poll)
		out.Poll = &p
	}
	out.Bookmarked = result.Bookmarked
	out.Pinned = result.Pinned
	out.Muted = result.Muted
	return out
}

// enrichedPollToAPIModel maps service.EnrichedPoll to apimodel.Poll.
func enrichedPollToAPIModel(p *service.EnrichedPoll) apimodel.Poll {
	var expiresAt *string
	if p.Poll.ExpiresAt != nil {
		s := p.Poll.ExpiresAt.UTC().Format(time.RFC3339)
		expiresAt = &s
	}
	expired := p.Poll.ExpiresAt != nil && p.Poll.ExpiresAt.Before(time.Now())
	options := make([]apimodel.PollOption, 0, len(p.Options))
	var votesCount int
	for _, o := range p.Options {
		votesCount += o.VotesCount
		options = append(options, apimodel.PollOption{Title: o.Title, VotesCount: o.VotesCount})
	}
	return apimodel.Poll{
		ID:          p.Poll.ID,
		ExpiresAt:   expiresAt,
		Expired:     expired,
		Multiple:    p.Poll.Multiple,
		VotesCount:  votesCount,
		VotersCount: nil,
		Voted:       p.Voted,
		OwnVotes:    p.OwnVotes,
		Options:     options,
		Emojis:      []any{},
	}
}
