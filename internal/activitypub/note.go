package activitypub

import (
	"fmt"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
)

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
