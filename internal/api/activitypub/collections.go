package activitypub

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera-fed/internal/ap"
	"github.com/chairswithlegs/monstera-fed/internal/api"
)

// CollectionsHandler serves GET /users/{username}/followers, /following, /collections/featured.
// Returns OrderedCollections with totalItems only (no item enumeration).
type CollectionsHandler struct {
	deps Deps
}

// NewCollectionsHandler returns a new CollectionsHandler.
func NewCollectionsHandler(deps Deps) *CollectionsHandler {
	return &CollectionsHandler{deps: deps}
}

// GETFollowers handles GET /users/{username}/followers.
func (h *CollectionsHandler) GETFollowers(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	if username == "" {
		api.WriteError(w, http.StatusBadRequest, "missing username")
		return
	}
	account, err := h.deps.Accounts.GetLocalActorForFederation(r.Context(), username)
	if err != nil {
		api.HandleError(w, r, h.deps.Logger, err)
		return
	}
	count, err := h.deps.Accounts.CountFollowers(r.Context(), account.ID)
	if err != nil {
		api.HandleError(w, r, h.deps.Logger, err)
		return
	}
	base := "https://" + h.deps.Config.InstanceDomain
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
	if username == "" {
		api.WriteError(w, http.StatusBadRequest, "missing username")
		return
	}
	account, err := h.deps.Accounts.GetLocalActorForFederation(r.Context(), username)
	if err != nil {
		api.HandleError(w, r, h.deps.Logger, err)
		return
	}
	count, err := h.deps.Accounts.CountFollowing(r.Context(), account.ID)
	if err != nil {
		api.HandleError(w, r, h.deps.Logger, err)
		return
	}
	base := "https://" + h.deps.Config.InstanceDomain
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
	if username == "" {
		api.WriteError(w, http.StatusBadRequest, "missing username")
		return
	}
	_, err := h.deps.Accounts.GetLocalActorForFederation(r.Context(), username)
	if err != nil {
		api.HandleError(w, r, h.deps.Logger, err)
		return
	}
	base := "https://" + h.deps.Config.InstanceDomain
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
