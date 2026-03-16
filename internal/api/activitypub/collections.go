package activitypub

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera/internal/activitypub/vocab"
	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/config"
	"github.com/chairswithlegs/monstera/internal/service"
)

// CollectionsHandler serves GET /users/{username}/followers, /following, /collections/featured.
// Returns OrderedCollections with totalItems only (no item enumeration).
type CollectionsHandler struct {
	accounts service.AccountService
	statuses service.StatusService
	config   *config.Config
}

// NewCollectionsHandler returns a new CollectionsHandler.
func NewCollectionsHandler(accounts service.AccountService, statuses service.StatusService, config *config.Config) *CollectionsHandler {
	return &CollectionsHandler{accounts: accounts, statuses: statuses, config: config}
}

// GETFollowers handles GET /users/{username}/followers.
func (h *CollectionsHandler) GETFollowers(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	if err := api.ValidateRequiredField(username, "username"); err != nil {
		api.HandleError(w, r, err)
		return
	}
	account, err := h.accounts.GetActiveLocalAccount(r.Context(), username)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	count, err := h.accounts.CountFollowers(r.Context(), account.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	base := h.config.InstanceBaseURL()
	id := base + "/users/" + username + "/followers"
	coll := vocab.NewOrderedCollection(id, int(count))
	w.Header().Set("Cache-Control", "max-age=300")
	api.WriteActivityJSON(w, http.StatusOK, coll)
}

// GETFollowing handles GET /users/{username}/following.
func (h *CollectionsHandler) GETFollowing(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	if err := api.ValidateRequiredField(username, "username"); err != nil {
		api.HandleError(w, r, err)
		return
	}
	account, err := h.accounts.GetActiveLocalAccount(r.Context(), username)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	count, err := h.accounts.CountFollowing(r.Context(), account.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	base := h.config.InstanceBaseURL()
	id := base + "/users/" + username + "/following"
	coll := vocab.NewOrderedCollection(id, int(count))
	w.Header().Set("Cache-Control", "max-age=300")
	api.WriteActivityJSON(w, http.StatusOK, coll)
}

// GETFeatured handles GET /users/{username}/collections/featured.
// Returns pinned statuses as an OrderedCollection of Notes.
func (h *CollectionsHandler) GETFeatured(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	if err := api.ValidateRequiredField(username, "username"); err != nil {
		api.HandleError(w, r, err)
		return
	}
	account, err := h.accounts.GetActiveLocalAccount(r.Context(), username)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	pinnedIDs, err := h.statuses.ListPinnedStatusIDs(r.Context(), account.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	orderedItems := make([]json.RawMessage, 0, len(pinnedIDs))
	for _, statusID := range pinnedIDs {
		st, err := h.statuses.GetByID(r.Context(), statusID)
		if err != nil || st == nil {
			continue
		}
		note := vocab.StatusToNote(st, account, h.config.InstanceBaseURL())
		raw, err := json.Marshal(note)
		if err != nil {
			continue
		}
		orderedItems = append(orderedItems, raw)
	}
	base := h.config.InstanceBaseURL()
	id := base + "/users/" + username + "/collections/featured"
	coll := vocab.NewOrderedCollectionWithItems(id, orderedItems)
	w.Header().Set("Cache-Control", "max-age=300")
	api.WriteActivityJSON(w, http.StatusOK, coll)
}
