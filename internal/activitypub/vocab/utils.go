package vocab

import "net/url"

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
