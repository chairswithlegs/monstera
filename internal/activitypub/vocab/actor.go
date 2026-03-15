package vocab

import (
	"fmt"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
)

// Actor represents an AP Person (user account). Served at GET /users/:username.
//
// Fields follow Mastodon's Actor shape so that remote instances recognise
// all profile metadata.
type Actor struct {
	Context                   interface{} `json:"@context"`
	ID                        string      `json:"id"`
	Type                      ObjectType  `json:"type"` // "Person" | "Service" (for bot accounts)
	PreferredUsername         string      `json:"preferredUsername"`
	Name                      string      `json:"name,omitempty"`
	Summary                   string      `json:"summary,omitempty"` // bio HTML
	URL                       string      `json:"url"`
	Inbox                     string      `json:"inbox"`
	Outbox                    string      `json:"outbox"`
	Followers                 string      `json:"followers"`
	Following                 string      `json:"following"`
	Featured                  string      `json:"featured,omitempty"`
	PublicKey                 PublicKey   `json:"publicKey"`
	Endpoints                 *Endpoints  `json:"endpoints,omitempty"`
	Icon                      *Icon       `json:"icon,omitempty"`  // avatar image
	Image                     *Icon       `json:"image,omitempty"` // header image
	ManuallyApprovesFollowers bool        `json:"manuallyApprovesFollowers"`
	Published                 string      `json:"published,omitempty"` // ISO 8601
}

// AccountToActor builds an ActivityPub Actor from a domain account.
// instanceDomain is the host (e.g. "example.com") for building IRIs.
func AccountToActor(a *domain.Account, instanceDomain string) *Actor {
	base := "https://" + instanceDomain
	id := a.APID
	if id == "" {
		id = base + "/users/" + a.Username
	}
	actorType := ObjectTypePerson
	if a.Bot {
		actorType = ObjectTypeService
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
	actor := &Actor{
		Context:                   DefaultContext,
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
		PublicKey:                 PublicKey{ID: id + "#main-key", Owner: id, PublicKeyPem: a.PublicKey},
		Endpoints:                 &Endpoints{SharedInbox: base + "/inbox"},
		ManuallyApprovesFollowers: a.Locked,
		Published:                 published,
	}
	if a.AvatarURL != "" {
		actor.Icon = &Icon{Type: "Image", URL: a.AvatarURL}
	}
	if a.HeaderURL != "" {
		actor.Image = &Icon{Type: "Image", URL: a.HeaderURL}
	}
	return actor
}
