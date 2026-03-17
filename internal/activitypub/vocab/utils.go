package vocab

import (
	"fmt"
	"net/url"

	"github.com/chairswithlegs/monstera/internal/domain"
)

// DomainFromIRI extracts the domain (host) from an AP IRI.
// Used for domain block checks and account domain population.
//
// Example: "https://remote.example.com/users/alice" → "remote.example.com"
func DomainFromIRI(iri string) string {
	u, err := url.Parse(iri)
	if err != nil {
		return ""
	}
	return u.Host
}

// StatusNoteID derives the canonical AP IRI for a status/note.
// Falls back: APID → URI → constructed IRI using instanceBase.
func StatusNoteID(s *domain.Status, instanceBase string) string {
	if s.APID != "" {
		return s.APID
	}
	if s.URI != "" {
		return s.URI
	}
	return fmt.Sprintf("%s/statuses/%s", instanceBase, s.ID)
}

// AccountActorID derives the canonical AP IRI for an account's actor.
// Falls back: APID → constructed IRI using instanceBase.
func AccountActorID(a *domain.Account, instanceBase string) string {
	if a.APID != "" {
		return a.APID
	}
	return fmt.Sprintf("%s/users/%s", instanceBase, a.Username)
}

// AccountFollowersURL derives the AP followers collection IRI for an account.
// Falls back: FollowersURL → actorID + "/followers".
func AccountFollowersURL(a *domain.Account, instanceBase string) string {
	if a.FollowersURL != "" {
		return a.FollowersURL
	}
	return AccountActorID(a, instanceBase) + "/followers"
}
