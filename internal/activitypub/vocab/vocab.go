package vocab

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
	Context any        `json:"@context,omitempty"`
	ID      string     `json:"id"`
	Type    ObjectType `json:"type"`
}

// ObjectType is the AP "type" field discriminator.
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

	// Content types
	ObjectTypeQuestion ObjectType = "Question"

	// Object types
	ObjectTypeImage    ObjectType = "Image"
	ObjectTypeVideo    ObjectType = "Video"
	ObjectTypeAudio    ObjectType = "Audio"
	ObjectTypeDocument ObjectType = "Document"

	// Tag types
	ObjectTypeHashtag ObjectType = "Hashtag"
	ObjectTypeMention ObjectType = "Mention"

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
