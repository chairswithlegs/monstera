package activitypub

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	ap "github.com/chairswithlegs/monstera-fed/internal/activitypub"
	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/config"
	"github.com/chairswithlegs/monstera-fed/internal/service"
)

// CollectionsHandler serves GET /users/{username}/followers, /following, /collections/featured.
// Returns OrderedCollections with totalItems only (no item enumeration).
type CollectionsHandler struct {
	accounts service.AccountService
	config   *config.Config
}

// NewCollectionsHandler returns a new CollectionsHandler.
func NewCollectionsHandler(accounts service.AccountService, config *config.Config) *CollectionsHandler {
	return &CollectionsHandler{accounts: accounts, config: config}
}

// GETFollowers handles GET /users/{username}/followers.
func (h *CollectionsHandler) GETFollowers(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	if err := api.ValidateRequiredString(username); err != nil {
		api.HandleError(w, r, err)
		return
	}
	account, err := h.accounts.GetLocalActorForFederation(r.Context(), username)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	count, err := h.accounts.CountFollowers(r.Context(), account.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	base := "https://" + h.config.InstanceDomain
	id := base + "/users/" + username + "/followers"
	coll := ap.OrderedCollection{
		Context:    ap.DefaultContext,
		ID:         id,
		Type:       "OrderedCollection",
		TotalItems: int(count),
	}
	w.Header().Set("Cache-Control", "max-age=300")
	api.WriteActivityJSON(w, http.StatusOK, coll)
}

// GETFollowing handles GET /users/{username}/following.
func (h *CollectionsHandler) GETFollowing(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	if err := api.ValidateRequiredString(username); err != nil {
		api.HandleError(w, r, err)
		return
	}
	account, err := h.accounts.GetLocalActorForFederation(r.Context(), username)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	count, err := h.accounts.CountFollowing(r.Context(), account.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	base := "https://" + h.config.InstanceDomain
	id := base + "/users/" + username + "/following"
	coll := ap.OrderedCollection{
		Context:    ap.DefaultContext,
		ID:         id,
		Type:       "OrderedCollection",
		TotalItems: int(count),
	}
	w.Header().Set("Cache-Control", "max-age=300")
	api.WriteActivityJSON(w, http.StatusOK, coll)
}

// GETFeatured handles GET /users/{username}/collections/featured.
// Phase 1: stub with totalItems 0 (no pinned posts enumeration).
func (h *CollectionsHandler) GETFeatured(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	if err := api.ValidateRequiredString(username); err != nil {
		api.HandleError(w, r, err)
		return
	}
	_, err := h.accounts.GetLocalActorForFederation(r.Context(), username)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	base := "https://" + h.config.InstanceDomain
	id := base + "/users/" + username + "/collections/featured"
	coll := ap.OrderedCollection{
		Context:    ap.DefaultContext,
		ID:         id,
		Type:       "OrderedCollection",
		TotalItems: 0,
	}
	w.Header().Set("Cache-Control", "max-age=300")
	api.WriteActivityJSON(w, http.StatusOK, coll)
}
