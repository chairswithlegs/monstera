package activitypub

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	ap "github.com/chairswithlegs/monstera/internal/activitypub"
	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/config"
	"github.com/chairswithlegs/monstera/internal/domain"
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
	withMedia, err := h.accounts.GetActiveLocalAccountWithMedia(r.Context(), username)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	actor := accountToActor(withMedia.Account, h.config)
	if withMedia.AvatarURL != "" {
		actor.Icon = &ap.Icon{Type: "Image", URL: withMedia.AvatarURL}
	}
	if withMedia.HeaderURL != "" {
		actor.Image = &ap.Icon{Type: "Image", URL: withMedia.HeaderURL}
	}
	w.Header().Set("Cache-Control", "max-age=300")
	api.WriteActivityJSON(w, http.StatusOK, actor)
}

func accountToActor(a *domain.Account, config *config.Config) *ap.Actor {
	base := "https://" + config.InstanceDomain
	id := a.APID
	if id == "" {
		id = base + "/users/" + a.Username
	}
	actorType := "Person"
	if a.Bot {
		actorType = "Service"
	}
	name := ""
	if a.DisplayName != nil && *a.DisplayName != "" {
		name = *a.DisplayName
	}
	summary := ""
	if a.Note != nil {
		summary = *a.Note
	}
	inbox := a.InboxURL
	if inbox == "" {
		inbox = fmt.Sprintf("%s/users/%s/inbox", base, a.Username)
	}
	outbox := a.OutboxURL
	if outbox == "" {
		outbox = fmt.Sprintf("%s/users/%s/outbox", base, a.Username)
	}
	followers := a.FollowersURL
	if followers == "" {
		followers = fmt.Sprintf("%s/users/%s/followers", base, a.Username)
	}
	following := a.FollowingURL
	if following == "" {
		following = fmt.Sprintf("%s/users/%s/following", base, a.Username)
	}
	featured := fmt.Sprintf("%s/users/%s/collections/featured", base, a.Username)
	published := a.CreatedAt.Format(time.RFC3339)
	return &ap.Actor{
		Context:                   ap.DefaultContext,
		ID:                        id,
		Type:                      actorType,
		PreferredUsername:         a.Username,
		Name:                      name,
		Summary:                   summary,
		URL:                       id,
		Inbox:                     inbox,
		Outbox:                    outbox,
		Followers:                 followers,
		Following:                 following,
		Featured:                  featured,
		PublicKey:                 ap.PublicKey{ID: id + "#main-key", Owner: id, PublicKeyPem: a.PublicKey},
		Endpoints:                 &ap.Endpoints{SharedInbox: base + "/inbox"},
		ManuallyApprovesFollowers: a.Locked,
		Published:                 published,
	}
}
