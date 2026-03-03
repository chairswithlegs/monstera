package activitypub

import (
	"net/url"
	"strings"
)

// subjectToActivityType returns the activity type from a federation subject
// (e.g. "activitypub.outbound.deliver.create" -> "create"), or "unknown" if not matched.
func subjectToActivityType(subject string) string {
	if strings.HasPrefix(subject, subjectPrefixDeliver) {
		return strings.TrimPrefix(subject, subjectPrefixDeliver)
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
