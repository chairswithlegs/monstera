package activitypub

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	ap "github.com/chairswithlegs/monstera/internal/activitypub"
	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/config"
	"github.com/chairswithlegs/monstera/internal/service"
)

// ActorHandler serves GET /users/{username} — AP Actor document.
type ActorHandler struct {
	accounts service.AccountService
	config   *config.Config
}

// NewActorHandler returns a new ActorHandler.
func NewActorHandler(accounts service.AccountService, config *config.Config) *ActorHandler {
	return &ActorHandler{accounts: accounts, config: config}
}

// GETActor returns the ActivityPub Actor JSON for the local user.
func (h *ActorHandler) GETActor(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	if err := api.ValidateRequiredField(username, "username"); err != nil {
		api.HandleError(w, r, err)
		return
	}
	// Domain account -> AP Actor document
	acc, err := h.accounts.GetActiveLocalAccount(r.Context(), username)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	actor := ap.AccountToActor(acc, h.config.InstanceDomain)
	w.Header().Set("Cache-Control", "max-age=300")
	api.WriteActivityJSON(w, http.StatusOK, actor)
}
