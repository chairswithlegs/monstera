package apimodel

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Expected JSON fields for each Mastodon API entity. Every field listed here
// must be present in the serialised JSON with the declared type. This catches
// omitempty bugs and accidental type changes that break Mastodon clients.
//
// Type strings: "string", "number", "bool", "array", "object",
// plus "|null" suffix for nullable fields (e.g. "string|null").

var accountFields = map[string]string{
	"id":              "string",
	"username":        "string",
	"acct":            "string",
	"display_name":    "string",
	"locked":          "bool",
	"bot":             "bool",
	"created_at":      "string",
	"note":            "string",
	"url":             "string",
	"avatar":          "string",
	"avatar_static":   "string",
	"header":          "string",
	"header_static":   "string",
	"followers_count": "number",
	"following_count": "number",
	"statuses_count":  "number",
	"last_status_at":  "string|null",
	"emojis":          "array",
	"fields":          "array",
	"roles":           "array",
}

var statusFields = map[string]string{
	"id":                     "string",
	"created_at":             "string",
	"in_reply_to_id":         "string|null",
	"in_reply_to_account_id": "string|null",
	"sensitive":              "bool",
	"spoiler_text":           "string",
	"visibility":             "string",
	"language":               "string|null",
	"uri":                    "string",
	"url":                    "string|null",
	"replies_count":          "number",
	"reblogs_count":          "number",
	"favourites_count":       "number",
	"content":                "string",
	"reblog":                 "object|null",
	"account":                "object",
	"media_attachments":      "array",
	"mentions":               "array",
	"tags":                   "array",
	"emojis":                 "array",
	"card":                   "object|null",
	"poll":                   "object|null",
	"favourited":             "bool",
	"reblogged":              "bool",
	"muted":                  "bool",
	"bookmarked":             "bool",
	"pinned":                 "bool",
	"filtered":               "array",
}

var relationshipFields = map[string]string{
	"id":                   "string",
	"following":            "bool",
	"showing_reblogs":      "bool",
	"notifying":            "bool",
	"followed_by":          "bool",
	"blocking":             "bool",
	"blocked_by":           "bool",
	"muting":               "bool",
	"muting_notifications": "bool",
	"requested":            "bool",
	"requested_by":         "bool",
	"domain_blocking":      "bool",
	"endorsed":             "bool",
	"note":                 "string",
	"languages":            "array",
}

var notificationFields = map[string]string{
	"id":         "string",
	"type":       "string",
	"created_at": "string",
	"account":    "object",
	"group_key":  "string",
}

var mediaAttachmentFields = map[string]string{
	"id":          "string",
	"type":        "string",
	"url":         "string",
	"preview_url": "string",
	"description": "string",
}

var pollFields = map[string]string{
	"id":           "string",
	"expires_at":   "string|null",
	"expired":      "bool",
	"multiple":     "bool",
	"votes_count":  "number",
	"voters_count": "number|null",
	"voted":        "bool",
	"own_votes":    "array",
	"options":      "array",
	"emojis":       "array",
}

var cardFields = map[string]string{
	"url":           "string",
	"title":         "string",
	"description":   "string",
	"type":          "string",
	"author_name":   "string",
	"author_url":    "string",
	"provider_name": "string",
	"provider_url":  "string",
	"html":          "string",
	"width":         "number",
	"height":        "number",
	"image":         "string|null",
	"blurhash":      "string|null",
	"published_at":  "string|null",
}

var mentionFields = map[string]string{
	"id":       "string",
	"username": "string",
	"acct":     "string",
	"url":      "string",
}

// assertJSONShape marshals v to JSON and asserts that every field in expected
// is present with the correct JSON type. entity is used in failure messages.
func assertJSONShape(t *testing.T, entity string, v any, expected map[string]string) {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err, "marshal %s to JSON", entity)
	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj), "unmarshal %s JSON", entity)

	for field, expectedType := range expected {
		path := entity + "." + field
		val, exists := obj[field]
		if !assert.Truef(t, exists, "missing required field %s", path) {
			continue
		}

		nullable := strings.HasSuffix(expectedType, "|null")
		baseType := strings.TrimSuffix(expectedType, "|null")

		if val == nil {
			if !nullable {
				assert.Failf(t, "unexpected null", "%s: expected %s, got null", path, expectedType)
			}
			continue
		}

		switch baseType {
		case "string":
			assert.IsTypef(t, "", val, "%s: expected string, got %T", path, val)
		case "number":
			assert.IsTypef(t, float64(0), val, "%s: expected number, got %T", path, val)
		case "bool":
			assert.IsTypef(t, true, val, "%s: expected bool, got %T", path, val)
		case "object":
			assert.IsTypef(t, map[string]any{}, val, "%s: expected object, got %T", path, val)
		case "array":
			assert.IsTypef(t, []any{}, val, "%s: expected array, got %T", path, val)
		}
	}
}
