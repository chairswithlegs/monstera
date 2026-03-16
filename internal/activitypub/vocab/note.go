package vocab

import (
	"encoding/json"
	"fmt"
	"html"
	"slices"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
)

// Note represents an AP Note (status/post). The core content type in the
// Mastodon federation protocol.
//
// ContentMap is a Mastodon extension that maps language codes to localised
// content.
type Note struct {
	Context      any                `json:"@context,omitempty"`
	ID           string             `json:"id"`
	Type         ObjectType         `json:"type"` // "Note"
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

// StatusToNote builds an ActivityPub Note from a domain status and its author account.
// instanceBase is the scheme + host (e.g. "https://example.com") for building IRIs.
func StatusToNote(s *domain.Status, account *domain.Account, instanceBase string) *Note {
	content := ""
	if s.Content != nil {
		content = *s.Content
	} else if s.Text != nil {
		content = html.EscapeString(*s.Text)
	}
	noteID := StatusNoteID(s, instanceBase)
	actorID := AccountActorID(account, instanceBase)
	published := s.CreatedAt.Format(time.RFC3339)

	var inReplyTo *string
	if s.InReplyToID != nil && *s.InReplyToID != "" {
		iri := fmt.Sprintf("%s/statuses/%s", instanceBase, *s.InReplyToID)
		inReplyTo = &iri
	}

	updated := ""
	if s.EditedAt != nil {
		updated = s.EditedAt.Format(time.RFC3339)
	}

	return &Note{
		Context:      DefaultContext,
		ID:           noteID,
		Type:         ObjectTypeNote,
		AttributedTo: actorID,
		Content:      content,
		To:           []string{PublicAddress},
		InReplyTo:    inReplyTo,
		Published:    published,
		Updated:      updated,
		URL:          noteID,
		Sensitive:    s.Sensitive,
		Summary:      s.ContentWarning,
	}
}

// NoteVisibility derives domain visibility from a Note's To/Cc addressing.
// followersURL is the author's AP followers collection IRI.
// Callers must sanitize note content before use; this function only inspects addressing fields.
func NoteVisibility(note *Note, followersURL string) string {
	if slices.Contains(note.To, PublicAddress) {
		return domain.VisibilityPublic
	}
	if slices.Contains(note.Cc, PublicAddress) {
		return domain.VisibilityUnlisted
	}
	if followersURL != "" && slices.Contains(note.To, followersURL) {
		return domain.VisibilityPrivate
	}
	return domain.VisibilityDirect
}

// NoteLanguage returns the first language code from ContentMap, or nil.
func NoteLanguage(note *Note) *string {
	if len(note.ContentMap) == 0 {
		return nil
	}
	for k := range note.ContentMap {
		return &k
	}
	return nil
}

// NoteStatusFields holds the pure field-mapping result of an inbound Note.
// Content and ContentWarning are excluded — callers must sanitize those at the
// trust boundary before building service.CreateRemoteStatusInput.
// InReplyToID and MediaIDs require I/O and are filled in by the caller.
type NoteStatusFields struct {
	URI       string
	APID      string
	Sensitive bool
	Language  *string
	ApRaw     []byte
}

// NoteToStatusFields extracts the pure (non-sanitized, non-I/O) fields from
// an inbound Note. Callers supply visibility (from NoteVisibility).
func NoteToStatusFields(note *Note) NoteStatusFields {
	apRaw, _ := json.Marshal(note)
	return NoteStatusFields{
		URI:       note.ID,
		APID:      note.ID,
		Sensitive: note.Sensitive,
		Language:  NoteLanguage(note),
		ApRaw:     apRaw,
	}
}
