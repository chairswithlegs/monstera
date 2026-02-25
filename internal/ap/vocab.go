package ap

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// PublicAddress is the ActivityStreams public addressing constant.
// Activities addressed to this IRI are visible to anyone.
const PublicAddress = "https://www.w3.org/ns/activitystreams#Public"

// mastodonExtContext is the Mastodon-specific extension context map included
// in outgoing activities. It maps short names to their full IRIs so that
// fields like "sensitive" and "manuallyApprovesFollowers" are understood
// by remote servers.
var mastodonExtContext = map[string]any{
	"manuallyApprovesFollowers": "as:manuallyApprovesFollowers",
	"sensitive":                 "as:sensitive",
	"Hashtag":                   "as:Hashtag",
	"toot":                      "http://joinmastodon.org/ns#",
	"Emoji":                     "toot:Emoji",
	"featured": map[string]string{
		"@id":   "toot:featured",
		"@type": "@id",
	},
	"featuredTags": map[string]string{
		"@id":   "toot:featuredTags",
		"@type": "@id",
	},
}

// DefaultContext is the canonical @context value for all outgoing AP activities.
// Structured as a JSON array: [AS2 namespace, Security namespace, Mastodon extensions].
var DefaultContext = []any{
	"https://www.w3.org/ns/activitystreams",
	"https://w3id.org/security/v1",
	mastodonExtContext,
}

// Object is the base ActivityStreams object. All AP types embed it.
// Fields shared across Actor, Note, Activity, and Collection types.
type Object struct {
	Context interface{} `json:"@context,omitempty"`
	ID      string      `json:"id"`
	Type    string      `json:"type"`
}

// PublicKey is the RSA public key embedded in an Actor document.
// Used by remote servers to verify HTTP Signatures on outgoing deliveries.
type PublicKey struct {
	ID           string `json:"id"`
	Owner        string `json:"owner"`
	PublicKeyPem string `json:"publicKeyPem"`
}

// Endpoints holds the Actor's special endpoint URLs.
// Only sharedInbox is used in Phase 1.
type Endpoints struct {
	SharedInbox string `json:"sharedInbox,omitempty"`
}

// Icon represents an Actor's avatar or header image.
type Icon struct {
	Type      string `json:"type"` // "Image"
	MediaType string `json:"mediaType,omitempty"`
	URL       string `json:"url"`
}

// Tag represents a Hashtag or Mention tag embedded in a Note.
type Tag struct {
	Type string `json:"type"` // "Hashtag" | "Mention"
	Href string `json:"href,omitempty"`
	Name string `json:"name"` // "#golang" for hashtags, "@user@domain" for mentions
}

// Attachment represents a media attachment on a Note.
type Attachment struct {
	Type      string `json:"type"` // "Document"
	MediaType string `json:"mediaType,omitempty"`
	URL       string `json:"url"`
	Name      string `json:"name,omitempty"` // alt text
	Blurhash  string `json:"blurhash,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
}

// Actor represents an AP Person (user account). Served at GET /users/:username.
//
// Fields follow Mastodon's Actor shape so that remote instances recognise
// all profile metadata. Fields not relevant to Phase 1 (e.g. movedTo) are
// omitted and added as their features are implemented.
type Actor struct {
	Context                   interface{} `json:"@context"`
	ID                        string      `json:"id"`
	Type                      string      `json:"type"` // "Person" | "Service" (for bot accounts)
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
	Icon                      *Icon       `json:"icon,omitempty"`
	Image                     *Icon       `json:"image,omitempty"` // header image
	ManuallyApprovesFollowers bool        `json:"manuallyApprovesFollowers"`
	Published                 string      `json:"published,omitempty"` // ISO 8601
}

// Note represents an AP Note (status/post). The core content type in the
// Mastodon federation protocol.
//
// ContentMap is a Mastodon extension that maps language codes to localised
// content. Phase 1 populates it with a single entry when language is known.
type Note struct {
	Context      interface{}        `json:"@context,omitempty"`
	ID           string             `json:"id"`
	Type         string             `json:"type"` // "Note"
	AttributedTo string             `json:"attributedTo"`
	Content      string             `json:"content"` // rendered HTML
	ContentMap   map[string]string  `json:"contentMap,omitempty"`
	Source       *NoteSource        `json:"source,omitempty"`
	To           []string           `json:"to"`
	Cc           []string           `json:"cc,omitempty"`
	InReplyTo    *string            `json:"inReplyTo"` // null or parent Note IRI
	Published    string             `json:"published"` // ISO 8601
	Updated      string             `json:"updated,omitempty"`
	URL          string             `json:"url"`
	Sensitive    bool               `json:"sensitive"`
	Summary      *string            `json:"summary"` // content warning; null if none
	Tag          []Tag              `json:"tag,omitempty"`
	Attachment   []Attachment       `json:"attachment,omitempty"`
	Replies      *OrderedCollection `json:"replies,omitempty"`
}

// NoteSource preserves the original plain-text or Markdown source.
// Mastodon includes this for editable posts.
type NoteSource struct {
	Content   string `json:"content"`
	MediaType string `json:"mediaType"` // "text/plain"
}

// Activity is the generic AP Activity wrapper. Used for Follow, Like, Announce,
// Create, Delete, Update, Undo, Accept, Reject, and Block.
//
// The ObjectRaw field holds the polymorphic "object" — it can be a string
// (IRI reference) or an embedded JSON object (e.g. a full Note). Callers
// use the accessor methods to decode it.
type Activity struct {
	Context   interface{}     `json:"@context,omitempty"`
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Actor     string          `json:"actor"`
	ObjectRaw json.RawMessage `json:"object"`
	To        []string        `json:"to,omitempty"`
	Cc        []string        `json:"cc,omitempty"`
	Published string          `json:"published,omitempty"`
}

// ObjectID returns the object field as a plain IRI string.
// Returns ("", false) if the object is an embedded JSON object rather than a string.
func (a *Activity) ObjectID() (string, bool) {
	var id string
	if err := json.Unmarshal(a.ObjectRaw, &id); err != nil {
		return "", false
	}
	return id, true
}

// ObjectActivity unmarshals the object field as an embedded Activity.
// Used for Undo{Follow}, Accept{Follow}, Reject{Follow} where the object
// is the original Follow activity.
func (a *Activity) ObjectActivity() (*Activity, error) {
	var inner Activity
	if err := json.Unmarshal(a.ObjectRaw, &inner); err != nil {
		return nil, fmt.Errorf("object Activity: %w", err)
	}
	return &inner, nil
}

// ObjectNote unmarshals the object field as a Note.
// Used for Create{Note} and Update{Note}.
func (a *Activity) ObjectNote() (*Note, error) {
	var note Note
	if err := json.Unmarshal(a.ObjectRaw, &note); err != nil {
		return nil, fmt.Errorf("object Note: %w", err)
	}
	return &note, nil
}

// ObjectActor unmarshals the object field as an Actor.
// Used for Update{Person}.
func (a *Activity) ObjectActor() (*Actor, error) {
	var actor Actor
	if err := json.Unmarshal(a.ObjectRaw, &actor); err != nil {
		return nil, fmt.Errorf("object Actor: %w", err)
	}
	return &actor, nil
}

// ObjectType peeks at the "type" field of the embedded object without fully
// unmarshalling it. Returns "" if the object is a plain string IRI.
func (a *Activity) ObjectType() string {
	var peek struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(a.ObjectRaw, &peek); err != nil {
		return ""
	}
	return peek.Type
}

// OrderedCollection represents an AP OrderedCollection.
// Used for outbox, followers, and following endpoints.
type OrderedCollection struct {
	Context    interface{} `json:"@context,omitempty"`
	ID         string      `json:"id"`
	Type       string      `json:"type"` // "OrderedCollection"
	TotalItems int         `json:"totalItems"`
	First      string      `json:"first,omitempty"` // URL of first page
}

// OrderedCollectionPage represents a page within an OrderedCollection.
type OrderedCollectionPage struct {
	Context      interface{}       `json:"@context,omitempty"`
	ID           string            `json:"id"`
	Type         string            `json:"type"` // "OrderedCollectionPage"
	TotalItems   int               `json:"totalItems"`
	PartOf       string            `json:"partOf"`
	Next         string            `json:"next,omitempty"`
	Prev         string            `json:"prev,omitempty"`
	OrderedItems []json.RawMessage `json:"orderedItems"`
}

// DomainFromActorID extracts the domain (host) from an AP actor IRI.
// Used for domain block checks and account domain population.
//
// Example: "https://remote.example.com/users/alice" → "remote.example.com"
func DomainFromActorID(actorID string) string {
	u, err := url.Parse(actorID)
	if err != nil {
		return ""
	}
	return u.Host
}

// DomainFromKeyID extracts the domain from an HTTP Signature key ID.
// Key IDs are typically "https://remote.example.com/users/alice#main-key".
func DomainFromKeyID(keyID string) string {
	return DomainFromActorID(keyID)
}
