package activitypub

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera/internal/activitypub/vocab"
	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/service"
)

// ActorHandler serves GET /users/{username} — AP Actor document.
type ActorHandler struct {
	accounts        service.AccountService
	instanceDomain  string
	instanceBaseURL string
}

// NewActorHandler returns a new ActorHandler.
func NewActorHandler(accounts service.AccountService, instanceDomain, instanceBaseURL string) *ActorHandler {
	return &ActorHandler{
		accounts:        accounts,
		instanceDomain:  instanceDomain,
		instanceBaseURL: strings.TrimSuffix(instanceBaseURL, "/"),
	}
}

// GETActor returns the ActivityPub Actor JSON for AP clients, or redirects
// browsers to the web profile page via content negotiation.
func (h *ActorHandler) GETActor(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	if err := api.ValidateRequiredField(username, "username"); err != nil {
		api.HandleError(w, r, err)
		return
	}

	if !acceptsActivityPub(r) {
		http.Redirect(w, r, h.instanceBaseURL+"/@"+username, http.StatusSeeOther)
		return
	}

	acc, err := h.accounts.GetActiveLocalAccount(r.Context(), username)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	actor := vocab.AccountToActor(acc, h.instanceDomain)
	w.Header().Set("Cache-Control", "max-age=300")
	api.WriteActivityJSON(w, http.StatusOK, actor)
}

func acceptsActivityPub(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "application/activity+json") ||
		strings.Contains(accept, "application/ld+json")
}
