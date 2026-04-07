package mastodon

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
)

// POSTFollow handles POST /api/v1/accounts/:id/follow. Auth required.
func (h *AccountsHandler) POSTFollow(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	targetID := chi.URLParam(r, "id")
	if targetID == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	target, err := h.accounts.GetByID(r.Context(), targetID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	rel, err := h.follows.Follow(r.Context(), account.ID, target.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToRelationship(rel))
}

// POSTUnfollow handles POST /api/v1/accounts/:id/unfollow. Auth required.
func (h *AccountsHandler) POSTUnfollow(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	targetID := chi.URLParam(r, "id")
	if targetID == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	target, err := h.accounts.GetByID(r.Context(), targetID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	rel, err := h.follows.Unfollow(r.Context(), account.ID, target.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToRelationship(rel))
}

// GETRelationships handles GET /api/v1/accounts/relationships?id[]=... Returns []Relationship for each requested id.
func (h *AccountsHandler) GETRelationships(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	ids := r.URL.Query()["id[]"]
	if len(ids) == 0 {
		api.WriteJSON(w, http.StatusOK, []apimodel.Relationship{})
		return
	}
	out := make([]apimodel.Relationship, 0, len(ids))
	for _, targetID := range ids {
		if targetID == "" {
			continue
		}
		target, err := h.accounts.GetByID(r.Context(), targetID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				api.HandleError(w, r, api.ErrNotFound)
				return
			}
			api.HandleError(w, r, err)
			return
		}
		rel, err := h.accounts.GetRelationship(r.Context(), account.ID, target.ID)
		if err != nil {
			api.HandleError(w, r, err)
			return
		}
		out = append(out, apimodel.ToRelationship(rel))
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// GETFollowers handles GET /api/v1/accounts/:id/followers.
func (h *AccountsHandler) GETFollowers(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	var viewerID *string
	if account != nil {
		viewerID = &account.ID
	}
	targetID := chi.URLParam(r, "id")
	if targetID == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	target, err := h.accounts.GetByID(r.Context(), targetID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	if target.Locked && (viewerID == nil || target.ID != *viewerID) {
		var follow *domain.Follow
		if viewerID != nil {
			follow, _ = h.follows.GetFollow(r.Context(), *viewerID, target.ID)
		}
		if follow == nil || follow.State != domain.FollowStateAccepted {
			api.WriteJSON(w, http.StatusOK, []apimodel.Account{})
			return
		}
	}
	params := PageParamsFromRequest(r)
	followers, err := h.follows.GetFollowers(r.Context(), target.ID, optionalString(params.MaxID), params.Limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Account, 0, len(followers))
	for _, a := range followers {
		out = append(out, apimodel.ToAccount(&a, h.instanceDomain))
	}
	firstID, lastID := firstLastIDsFromAccounts(followers)
	if link := LinkHeader(AbsoluteRequestURL(r, h.instanceDomain), firstID, lastID); link != "" {
		w.Header().Set("Link", link)
	}
	api.WriteJSON(w, http.StatusOK, out)
}

func firstLastIDsFromAccounts(list []domain.Account) (firstID, lastID string) {
	if len(list) == 0 {
		return "", ""
	}
	return list[0].ID, list[len(list)-1].ID
}

// GETFollowing handles GET /api/v1/accounts/:id/following.
func (h *AccountsHandler) GETFollowing(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	var viewerID *string
	if account != nil {
		viewerID = &account.ID
	}
	targetID := chi.URLParam(r, "id")
	if targetID == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	target, err := h.accounts.GetByID(r.Context(), targetID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	if target.Locked && (viewerID == nil || target.ID != *viewerID) {
		var follow *domain.Follow
		if viewerID != nil {
			follow, _ = h.follows.GetFollow(r.Context(), *viewerID, target.ID)
		}
		if follow == nil || follow.State != domain.FollowStateAccepted {
			api.WriteJSON(w, http.StatusOK, []apimodel.Account{})
			return
		}
	}
	params := PageParamsFromRequest(r)
	list, err := h.follows.GetFollowing(r.Context(), target.ID, optionalString(params.MaxID), params.Limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Account, 0, len(list))
	for _, a := range list {
		out = append(out, apimodel.ToAccount(&a, h.instanceDomain))
	}
	firstID, lastID := firstLastIDsFromAccounts(list)
	if link := LinkHeader(AbsoluteRequestURL(r, h.instanceDomain), firstID, lastID); link != "" {
		w.Header().Set("Link", link)
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// GETBlocks handles GET /api/v1/blocks. Returns paginated blocked accounts for the authenticated user.
func (h *AccountsHandler) GETBlocks(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	params := PageParamsFromRequest(r)
	limit := parseLimitParam(r, DefaultListLimit, MaxListLimit)
	blocks, nextCursor, err := h.follows.ListBlockedAccounts(r.Context(), account.ID, optionalString(params.MaxID), limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Account, 0, len(blocks))
	for i := range blocks {
		out = append(out, apimodel.ToAccount(&blocks[i], h.instanceDomain))
	}
	if nextCursor != nil && *nextCursor != "" {
		if link := linkHeaderWithNext(AbsoluteRequestURL(r, h.instanceDomain), *nextCursor); link != "" {
			w.Header().Set("Link", link)
		}
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// POSTBlock handles POST /api/v1/accounts/:id/block.
func (h *AccountsHandler) POSTBlock(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	targetID := chi.URLParam(r, "id")
	if targetID == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	target, err := h.accounts.GetByID(r.Context(), targetID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	rel, err := h.follows.Block(r.Context(), account.ID, target.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToRelationship(rel))
}

// POSTUnblock handles POST /api/v1/accounts/:id/unblock.
func (h *AccountsHandler) POSTUnblock(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	targetID := chi.URLParam(r, "id")
	if targetID == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	target, err := h.accounts.GetByID(r.Context(), targetID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	rel, err := h.follows.Unblock(r.Context(), account.ID, target.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToRelationship(rel))
}

// GETMutes handles GET /api/v1/mutes. Returns paginated muted accounts for the authenticated user.
func (h *AccountsHandler) GETMutes(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	params := PageParamsFromRequest(r)
	limit := parseLimitParam(r, DefaultListLimit, MaxListLimit)
	mutes, nextCursor, err := h.follows.ListMutedAccounts(r.Context(), account.ID, optionalString(params.MaxID), limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Account, 0, len(mutes))
	for i := range mutes {
		out = append(out, apimodel.ToAccount(&mutes[i], h.instanceDomain))
	}
	if nextCursor != nil && *nextCursor != "" {
		if link := linkHeaderWithNext(AbsoluteRequestURL(r, h.instanceDomain), *nextCursor); link != "" {
			w.Header().Set("Link", link)
		}
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// POSTMute handles POST /api/v1/accounts/:id/mute.
func (h *AccountsHandler) POSTMute(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	targetID := chi.URLParam(r, "id")
	if targetID == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	target, err := h.accounts.GetByID(r.Context(), targetID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}

	if r.Form == nil {
		_ = r.ParseForm() //nolint:gosec // G120: body size limited by upstream MaxBodySize middleware
	}
	hideNotifications := false
	if r.Form != nil {
		hideNotifications = api.FormValueIsTruthy(r.Form, "notifications")
	}
	rel, err := h.follows.Mute(r.Context(), account.ID, target.ID, hideNotifications)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToRelationship(rel))
}

// POSTUnmute handles POST /api/v1/accounts/:id/unmute.
func (h *AccountsHandler) POSTUnmute(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	targetID := chi.URLParam(r, "id")
	if targetID == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	target, err := h.accounts.GetByID(r.Context(), targetID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	rel, err := h.follows.Unmute(r.Context(), account.ID, target.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToRelationship(rel))
}

// familiarFollowersLimit is the maximum number of familiar followers returned per requested account.
// Matches Mastodon's default sample size.
const familiarFollowersLimit = 10

// familiarFollowersEntry is one entry in the familiar followers response (one per requested account ID).
type familiarFollowersEntry struct {
	ID       string             `json:"id"`
	Accounts []apimodel.Account `json:"accounts"`
}

// GETFamiliarFollowers handles GET /api/v1/accounts/familiar_followers?id[]=...
// For each requested account ID, returns the accounts that the viewer follows who also follow that account.
func (h *AccountsHandler) GETFamiliarFollowers(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	ids := r.URL.Query()["id[]"]
	out := make([]familiarFollowersEntry, 0, len(ids))
	for _, targetID := range ids {
		if targetID == "" {
			continue
		}
		accounts, err := h.follows.GetFamiliarFollowers(r.Context(), account.ID, targetID, familiarFollowersLimit)
		if err != nil {
			api.HandleError(w, r, err)
			return
		}
		apiAccounts := make([]apimodel.Account, 0, len(accounts))
		for i := range accounts {
			apiAccounts = append(apiAccounts, apimodel.ToAccount(&accounts[i], h.instanceDomain))
		}
		out = append(out, familiarFollowersEntry{ID: targetID, Accounts: apiAccounts})
	}
	api.WriteJSON(w, http.StatusOK, out)
}
