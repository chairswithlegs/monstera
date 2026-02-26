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

// NewCollectionsHandler constructs a CollectionsHandler.
func NewCollectionsHandler(deps Deps) *CollectionsHandler {
	return &CollectionsHandler{deps: deps}
}

// ServeFollowers handles GET /users/{username}/followers.
func (h *CollectionsHandler) ServeFollowers(w http.ResponseWriter, r *http.Request) {
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
		count = 0
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
	writeJSON(w, coll)
}

// ServeFollowing handles GET /users/{username}/following.
func (h *CollectionsHandler) ServeFollowing(w http.ResponseWriter, r *http.Request) {
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
		count = 0
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
	writeJSON(w, coll)
}

// ServeFeatured handles GET /users/{username}/collections/featured.
// Phase 1: stub with totalItems 0 (no pinned posts enumeration).
func (h *CollectionsHandler) ServeFeatured(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, coll)
}
