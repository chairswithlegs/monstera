package vocab

import (
	"encoding/json"
)

// TODO: the translation between ActivityPub and Domain models is spread throughout codebase
// We should consolidate the translation logic into this package.
// Specifically, we need:
// - ability to convert domain events -> outgoing activities (used by the federation subscriber)
// - ability to convert incoming activities -> service inputs (used by the inbox processor)
// - ability to convert domain models -> AP objects (used by the AP API handlers)

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
	Type    ObjectType  `json:"type"`
}

type ObjectType string

const (
	ObjectTypeNote      ObjectType = "Note"
	ObjectTypePerson    ObjectType = "Person"
	ObjectTypeTombstone ObjectType = "Tombstone"
	ObjectTypeService   ObjectType = "Service"

	// Activity types
	ObjectTypeFollow   ObjectType = "Follow"
	ObjectTypeLike     ObjectType = "Like"
	ObjectTypeAnnounce ObjectType = "Announce"
	ObjectTypeCreate   ObjectType = "Create"
	ObjectTypeDelete   ObjectType = "Delete"
	ObjectTypeUpdate   ObjectType = "Update"
	ObjectTypeUndo     ObjectType = "Undo"
	ObjectTypeAccept   ObjectType = "Accept"
	ObjectTypeReject   ObjectType = "Reject"
	ObjectTypeBlock    ObjectType = "Block"

	// Collection types
	ObjectTypeOrderedCollection     ObjectType = "OrderedCollection"
	ObjectTypeOrderedCollectionPage ObjectType = "OrderedCollectionPage"
)

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
	Type      ObjectType `json:"type"` // "Image"
	MediaType string     `json:"mediaType,omitempty"`
	URL       string     `json:"url"`
}

// Tag represents a Hashtag or Mention tag embedded in a Note.
type Tag struct {
	Type ObjectType `json:"type"` // "Hashtag" | "Mention"
	Href string     `json:"href,omitempty"`
	Name string     `json:"name"` // "#golang" for hashtags, "@user@domain" for mentions
}

// Attachment represents a media attachment on a Note.
type Attachment struct {
	Type      ObjectType `json:"type"` // "Document"
	MediaType string     `json:"mediaType,omitempty"`
	URL       string     `json:"url"`
	Name      string     `json:"name,omitempty"` // alt text
	Blurhash  string     `json:"blurhash,omitempty"`
	Width     int        `json:"width,omitempty"`
	Height    int        `json:"height,omitempty"`
}

// OrderedCollection represents an AP OrderedCollection.
// Used for outbox, followers, following, and featured endpoints.
// When OrderedItems is non-nil, it is serialized (inline items); otherwise First may point to a page.
type OrderedCollection struct {
	Context      interface{}       `json:"@context,omitempty"`
	ID           string            `json:"id"`
	Type         ObjectType        `json:"type"` // "OrderedCollection"
	TotalItems   int               `json:"totalItems"`
	First        string            `json:"first,omitempty"`        // URL of first page
	OrderedItems []json.RawMessage `json:"orderedItems,omitempty"` // inline items when present
}

// OrderedCollectionPage represents a page within an OrderedCollection.
type OrderedCollectionPage struct {
	Context      interface{}       `json:"@context,omitempty"`
	ID           string            `json:"id"`
	Type         ObjectType        `json:"type"` // "OrderedCollectionPage"
	TotalItems   int               `json:"totalItems"`
	PartOf       string            `json:"partOf"`
	Next         string            `json:"next,omitempty"`
	Prev         string            `json:"prev,omitempty"`
	OrderedItems []json.RawMessage `json:"orderedItems"`
}
