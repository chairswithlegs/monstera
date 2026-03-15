package vocab

import (
	"fmt"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
)

// Note represents an AP Note (status/post). The core content type in the
// Mastodon federation protocol.
//
// ContentMap is a Mastodon extension that maps language codes to localised
// content.
type Note struct {
	Context      interface{}        `json:"@context,omitempty"`
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
// instanceDomain is the scheme + host (e.g. "example.com") for building IRIs.
func StatusToNote(s *domain.Status, account *domain.Account, instanceDomain string) *Note {
	content := ""
	if s.Content != nil {
		content = *s.Content
	} else if s.Text != nil {
		content = *s.Text
	}
	noteID := s.APID
	if noteID == "" {
		noteID = s.URI
	}
	if noteID == "" {
		noteID = fmt.Sprintf("https://%s/statuses/%s", instanceDomain, s.ID)
	}
	actorID := account.APID
	if actorID == "" {
		actorID = fmt.Sprintf("https://%s/users/%s", instanceDomain, account.Username)
	}
	published := s.CreatedAt.Format(time.RFC3339)
	note := &Note{
		Context:      DefaultContext,
		ID:           noteID,
		Type:         "Note",
		AttributedTo: actorID,
		Content:      content,
		To:           []string{PublicAddress},
		Published:    published,
		URL:          noteID,
		Sensitive:    s.Sensitive,
		Summary:      s.ContentWarning,
	}
	return note
}
