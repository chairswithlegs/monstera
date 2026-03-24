package vocab

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
)

func BuildActorPublicProfileURL(uiBaseURL, username string) string {
	return fmt.Sprintf("%s/public/profile?u=%s", uiBaseURL, username)
}

// PropertyValue represents a Mastodon-style profile metadata field from an
// Actor's attachment array. Only entries with Type == "PropertyValue" are used.
type PropertyValue struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Actor represents an AP Person (user account). Served at GET /users/:username.
//
// Fields follow Mastodon's Actor shape so that remote instances recognise
// all profile metadata.
type Actor struct {
	Context                   any             `json:"@context"`
	ID                        string          `json:"id"`
	Type                      ObjectType      `json:"type"` // "Person" | "Service" (for bot accounts)
	PreferredUsername         string          `json:"preferredUsername"`
	Name                      string          `json:"name,omitempty"`
	Summary                   string          `json:"summary,omitempty"` // bio HTML
	URL                       string          `json:"url"`
	Inbox                     string          `json:"inbox"`
	Outbox                    string          `json:"outbox"`
	Followers                 string          `json:"followers"`
	Following                 string          `json:"following"`
	Featured                  string          `json:"featured,omitempty"`
	PublicKey                 PublicKey       `json:"publicKey"`
	Endpoints                 *Endpoints      `json:"endpoints,omitempty"`
	Icon                      *Icon           `json:"icon,omitempty"`  // avatar image
	Image                     *Icon           `json:"image,omitempty"` // header image
	Attachment                []PropertyValue `json:"attachment,omitempty"`
	ManuallyApprovesFollowers bool            `json:"manuallyApprovesFollowers"`
	Published                 string          `json:"published,omitempty"` // ISO 8601
}

// AccountToActor builds an ActivityPub Actor from a domain account.
// serverBaseURL is the base URL (e.g. "https://api.example.com") for building AP IRIs.
// uiBaseURL is the base URL of the web UI (e.g. "https://example.com") used for the
// human-readable Actor.URL field, which may differ from the API server.
func AccountToActor(a *domain.Account, serverBaseURL, uiBaseURL string) *Actor {
	base := strings.TrimSuffix(serverBaseURL, "/")
	ui := strings.TrimSuffix(uiBaseURL, "/")
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
		URL:                       BuildActorPublicProfileURL(ui, a.Username),
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
		actor.Icon = &Icon{Type: ObjectTypeImage, URL: a.AvatarURL}
	}
	if a.HeaderURL != "" {
		actor.Image = &Icon{Type: ObjectTypeImage, URL: a.HeaderURL}
	}
	return actor
}

// RemoteActorFields holds the pure field-mapping result of an inbound Actor.
// It contains no service or store types so the vocab package stays dependency-free.
type RemoteActorFields struct {
	APID           string
	Username       string
	Domain         string
	DisplayName    string
	Note           string
	PublicKey      string
	InboxURL       string
	OutboxURL      string
	FollowersURL   string
	FollowingURL   string
	SharedInboxURL string
	AvatarURL      string
	HeaderURL      string
	Bot            bool
	Locked         bool
	URL            string // Human-readable profile page URL (Actor.URL)
	FeaturedURL    string // ActivityPub featured collection URL (Actor.Featured)
	// Fields holds profile metadata parsed from Actor.Attachment PropertyValue
	// entries. Stored as JSON array of {"name":"...","value":"..."}. The
	// verified_at field is not included because it is not present in the
	// Actor document — only the originating server knows verification state.
	Fields json.RawMessage
}

// ActorToRemoteFields maps a sanitized Actor to RemoteActorFields.
// Caller is responsible for sanitizing actor.PreferredUsername, actor.Name, and
// actor.Summary before calling.
func ActorToRemoteFields(actor *Actor) RemoteActorFields {
	f := RemoteActorFields{
		APID:         actor.ID,
		Username:     actor.PreferredUsername,
		Domain:       DomainFromIRI(actor.ID),
		DisplayName:  actor.Name,
		Note:         actor.Summary,
		PublicKey:    actor.PublicKey.PublicKeyPem,
		InboxURL:     actor.Inbox,
		OutboxURL:    actor.Outbox,
		FollowersURL: actor.Followers,
		FollowingURL: actor.Following,
		Bot:          actor.Type == ObjectTypeService,
		Locked:       actor.ManuallyApprovesFollowers,
	}
	if actor.Endpoints != nil {
		f.SharedInboxURL = actor.Endpoints.SharedInbox
	}
	if actor.Icon != nil {
		f.AvatarURL = actor.Icon.URL
	}
	if actor.Image != nil {
		f.HeaderURL = actor.Image.URL
	}
	f.URL = actor.URL
	f.FeaturedURL = actor.Featured
	f.Fields = propertyValuesToFields(actor.Attachment)
	return f
}

func propertyValuesToFields(attachments []PropertyValue) json.RawMessage {
	type field struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	var fields []field
	for _, pv := range attachments {
		if pv.Type != "PropertyValue" {
			continue
		}
		fields = append(fields, field{Name: pv.Name, Value: pv.Value})
	}
	if len(fields) == 0 {
		return nil
	}
	b, err := json.Marshal(fields)
	if err != nil {
		return nil
	}
	return b
}
