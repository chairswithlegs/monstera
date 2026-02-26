package activitypub

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera-fed/internal/ap"
	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
)

// ActorHandler serves GET /users/{username} — AP Actor document.
type ActorHandler struct {
	deps Deps
}

// NewActorHandler constructs an ActorHandler.
func NewActorHandler(deps Deps) *ActorHandler {
	return &ActorHandler{deps: deps}
}

// ServeHTTP returns the ActivityPub Actor JSON for the local user.
func (h *ActorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	if username == "" {
		api.WriteError(w, http.StatusBadRequest, "missing username")
		return
	}
	withMedia, err := h.deps.Accounts.GetLocalActorWithMedia(r.Context(), username)
	if err != nil {
		api.HandleError(w, r, h.deps.Logger, err)
		return
	}
	actor := accountToActor(withMedia.Account, h.deps)
	if withMedia.AvatarURL != "" {
		actor.Icon = &ap.Icon{Type: "Image", URL: withMedia.AvatarURL}
	}
	if withMedia.HeaderURL != "" {
		actor.Image = &ap.Icon{Type: "Image", URL: withMedia.HeaderURL}
	}
	w.Header().Set("Cache-Control", "max-age=300")
	writeJSON(w, actor)
}

func accountToActor(a *domain.Account, deps Deps) *ap.Actor {
	base := "https://" + deps.Config.InstanceDomain
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
