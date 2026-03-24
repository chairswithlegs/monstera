package activitypub

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera/internal/activitypub/vocab"
	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/service"
)

// ActorHandler serves GET /users/{username} — AP Actor document.
type ActorHandler struct {
	accounts        service.AccountService
	instanceBaseURL string
	uiBaseURL       string
}

// NewActorHandler returns a new ActorHandler.
// instanceBaseURL is the AP/API server base URL; uiBaseURL is the web UI base URL
// (they may differ when the API and UI run on separate origins).
func NewActorHandler(accounts service.AccountService, instanceBaseURL, uiBaseURL string) *ActorHandler {
	return &ActorHandler{
		accounts:        accounts,
		instanceBaseURL: strings.TrimSuffix(instanceBaseURL, "/"),
		uiBaseURL:       strings.TrimSuffix(uiBaseURL, "/"),
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
		profileURL := vocab.BuildActorPublicProfileURL(h.uiBaseURL, url.QueryEscape(username))
		http.Redirect(w, r, profileURL, http.StatusSeeOther)
		return
	}

	acc, err := h.accounts.GetActiveLocalAccount(r.Context(), username)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	actor := vocab.AccountToActor(acc, h.instanceBaseURL, h.uiBaseURL)
	w.Header().Set("Cache-Control", "max-age=300")
	api.WriteActivityJSON(w, http.StatusOK, actor)
}

func acceptsActivityPub(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "application/activity+json") ||
		strings.Contains(accept, "application/ld+json")
}
