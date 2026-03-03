package activitypub

import (
	"net/url"
	"strings"

	natsutil "github.com/chairswithlegs/monstera-fed/internal/nats"
)

// subjectToActivityType returns the activity type from a federation subject
// (e.g. "federation.deliver.create" -> "create"), or "unknown" if not matched.
func subjectToActivityType(subject string) string {
	if strings.HasPrefix(subject, natsutil.SubjectPrefixActivityPubDeliver) {
		return strings.TrimPrefix(subject, natsutil.SubjectPrefixActivityPubDeliver)
	}
	return "unknown"
}

// domainFromURL returns the host from rawURL, or empty string if parsing fails.
func domainFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Host
}
