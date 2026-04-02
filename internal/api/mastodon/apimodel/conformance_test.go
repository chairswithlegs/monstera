// This file contains tests for validating that the marshalled API model types correctly adhere
// to the official Mastodon API schema. The Mastodon schema files live in testdata/schemas/.
package apimodel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- schema validation helpers ---

// entitySchema describes the expected JSON shape of a Mastodon API entity.
// Schema files live in testdata/schemas/{name}.json.
type entitySchema struct {
	Entity string            `json:"entity"`
	Fields map[string]string `json:"fields"`
}

func loadSchema(t *testing.T, name string) entitySchema {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "schemas", name+".json"))
	require.NoError(t, err, "load schema %s", name)
	var s entitySchema
	require.NoError(t, json.Unmarshal(data, &s), "parse schema %s", name)
	return s
}

// assertConformsToSchema marshals v to JSON and validates it against the named
// schema. It checks that every field declared in the schema is present in the
// JSON output and has the expected type.
func assertConformsToSchema(t *testing.T, schemaName string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err, "marshal to JSON")
	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj), "unmarshal response JSON")
	assertObjectConforms(t, schemaName, obj, "")
}

func assertObjectConforms(t *testing.T, schemaName string, obj map[string]any, path string) {
	t.Helper()
	schema := loadSchema(t, schemaName)
	prefix := schema.Entity
	if path != "" {
		prefix = path
	}

	for field, expectedType := range schema.Fields {
		fullPath := prefix + "." + field
		val, exists := obj[field]

		if !assert.Truef(t, exists, "missing required field %s", fullPath) {
			continue
		}

		assertFieldType(t, fullPath, expectedType, val)
	}
}

func assertFieldType(t *testing.T, path string, expectedType string, val any) {
	t.Helper()

	nullable := strings.HasSuffix(expectedType, "|null")
	baseType := strings.TrimSuffix(expectedType, "|null")

	// Handle $ref (e.g. "$account", "$status|null")
	if strings.HasPrefix(baseType, "$") {
		refName := strings.TrimPrefix(baseType, "$")
		if val == nil {
			if !nullable {
				assert.Failf(t, "unexpected null", "%s: expected $%s object, got null", path, refName)
			}
			return
		}
		obj, ok := val.(map[string]any)
		if !assert.Truef(t, ok, "%s: expected object for $%s, got %T", path, refName, val) {
			return
		}
		assertObjectConforms(t, refName, obj, path)
		return
	}

	if val == nil {
		if !nullable {
			assert.Failf(t, "unexpected null", "%s: expected %s, got null", path, expectedType)
		}
		return
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
	default:
		assert.Failf(t, "unknown schema type", "%s: unknown type %q in schema", path, expectedType)
	}
}

// --- test fixtures ---

func testAccount() *domain.Account {
	return &domain.Account{
		ID:             "01ABCDEF",
		Username:       "alice",
		Domain:         nil,
		DisplayName:    strPtr("Alice"),
		Note:           strPtr("Hello world"),
		AvatarURL:      "https://example.com/avatar.png",
		HeaderURL:      "https://example.com/header.png",
		FollowersCount: 10,
		FollowingCount: 5,
		StatusesCount:  42,
		Fields:         json.RawMessage(`[]`),
		CreatedAt:      time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

const testRemoteDomain = "remote.example"

func testRemoteAccount() *domain.Account {
	d := testRemoteDomain
	return &domain.Account{
		ID:             "01REMOTE",
		Username:       "bob",
		Domain:         &d,
		DisplayName:    strPtr("Bob Remote"),
		Note:           strPtr("I'm remote"),
		AvatarURL:      "https://remote.example/avatar.png",
		HeaderURL:      "https://remote.example/header.png",
		APID:           "https://remote.example/users/bob",
		ProfileURL:     "https://remote.example/@bob",
		FollowersCount: 100,
		FollowingCount: 50,
		StatusesCount:  200,
		Fields:         json.RawMessage(`[{"name":"Website","value":"https://bob.example","verified_at":null}]`),
		CreatedAt:      time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC),
	}
}

func testStatus(account *domain.Account) *domain.Status {
	content := "<p>Hello, world!</p>"
	lang := "en"
	return &domain.Status{
		ID:              "01STATUS",
		URI:             "https://example.com/statuses/01STATUS",
		AccountID:       account.ID,
		Content:         &content,
		Visibility:      "public",
		Language:        &lang,
		Local:           true,
		Sensitive:       false,
		RepliesCount:    2,
		ReblogsCount:    5,
		FavouritesCount: 10,
		CreatedAt:       time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC),
	}
}

func testUser() *domain.User {
	return &domain.User{
		DefaultPrivacy:   "public",
		DefaultSensitive: false,
		DefaultLanguage:  "en",
	}
}

func strPtr(s string) *string { return &s }

// --- conformance tests ---

func TestConformance_Account(t *testing.T) {
	t.Parallel()

	t.Run("local account", func(t *testing.T) {
		t.Parallel()
		acc := ToAccount(testAccount(), "example.com")
		assertConformsToSchema(t, "account", acc)
	})

	t.Run("remote account", func(t *testing.T) {
		t.Parallel()
		acc := ToAccount(testRemoteAccount(), "example.com")
		assertConformsToSchema(t, "account", acc)
	})

	t.Run("account with source", func(t *testing.T) {
		t.Parallel()
		acc := ToAccountWithSource(testAccount(), testUser(), "example.com")
		assertConformsToSchema(t, "account", acc)

		// Source is not in the base schema (omitempty), so verify it explicitly.
		data, err := json.Marshal(acc)
		require.NoError(t, err)
		var obj map[string]any
		require.NoError(t, json.Unmarshal(data, &obj))

		source, ok := obj["source"].(map[string]any)
		require.True(t, ok, "source should be present for verify_credentials")
		assert.IsType(t, "", source["note"])
		assert.IsType(t, "", source["privacy"])
		assert.IsType(t, true, source["sensitive"])
		assert.IsType(t, "", source["language"])
		assert.IsType(t, []any{}, source["fields"])
	})

	t.Run("account with nil display name and note", func(t *testing.T) {
		t.Parallel()
		acc := testAccount()
		acc.DisplayName = nil
		acc.Note = nil
		out := ToAccount(acc, "example.com")
		assertConformsToSchema(t, "account", out)
	})

	t.Run("account with empty fields", func(t *testing.T) {
		t.Parallel()
		acc := testAccount()
		acc.Fields = nil
		out := ToAccount(acc, "example.com")
		assertConformsToSchema(t, "account", out)
	})
}

func TestConformance_Status(t *testing.T) {
	t.Parallel()
	account := testAccount()

	t.Run("basic status", func(t *testing.T) {
		t.Parallel()
		author := ToAccount(account, "example.com")
		st := ToStatus(testStatus(account), author, []Mention{}, []Tag{}, []MediaAttachment{}, nil, "example.com")
		assertConformsToSchema(t, "status", st)
	})

	t.Run("status with nil content and content warning", func(t *testing.T) {
		t.Parallel()
		s := testStatus(account)
		s.Content = nil
		s.ContentWarning = nil
		author := ToAccount(account, "example.com")
		st := ToStatus(s, author, []Mention{}, []Tag{}, []MediaAttachment{}, nil, "example.com")
		assertConformsToSchema(t, "status", st)
	})

	t.Run("remote status has null url", func(t *testing.T) {
		t.Parallel()
		s := testStatus(account)
		s.Local = false
		author := ToAccount(account, "example.com")
		st := ToStatus(s, author, []Mention{}, []Tag{}, []MediaAttachment{}, nil, "example.com")
		assertConformsToSchema(t, "status", st)
	})

	t.Run("status from enriched", func(t *testing.T) {
		t.Parallel()
		s := testStatus(account)
		enriched := service.EnrichedStatus{
			Status:   s,
			Author:   account,
			Mentions: []*domain.Account{},
			Tags:     []domain.Hashtag{},
			Media:    []domain.MediaAttachment{},
		}
		st := StatusFromEnriched(enriched, "example.com")
		assertConformsToSchema(t, "status", st)
	})
}

func TestConformance_Notification(t *testing.T) {
	t.Parallel()

	t.Run("follow notification", func(t *testing.T) {
		t.Parallel()
		n := &domain.Notification{
			ID:        "01NOTIF",
			AccountID: "01ABCDEF",
			FromID:    "01REMOTE",
			Type:      "follow",
			GroupKey:  "follow-01REMOTE",
			CreatedAt: time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC),
		}
		out := ToNotification(n, testRemoteAccount(), nil, "example.com")
		assertConformsToSchema(t, "notification", out)
	})

	t.Run("favourite notification with status", func(t *testing.T) {
		t.Parallel()
		statusID := "01STATUS"
		n := &domain.Notification{
			ID:        "01NOTIF2",
			AccountID: "01ABCDEF",
			FromID:    "01REMOTE",
			Type:      "favourite",
			StatusID:  &statusID,
			GroupKey:  "favourite-01STATUS-01REMOTE",
			CreatedAt: time.Date(2024, 3, 15, 11, 0, 0, 0, time.UTC),
		}
		account := testAccount()
		author := ToAccount(account, "example.com")
		st := ToStatus(testStatus(account), author, []Mention{}, []Tag{}, []MediaAttachment{}, nil, "example.com")
		out := ToNotification(n, testRemoteAccount(), &st, "example.com")
		assertConformsToSchema(t, "notification", out)
	})
}

func TestConformance_Relationship(t *testing.T) {
	t.Parallel()

	t.Run("full relationship", func(t *testing.T) {
		t.Parallel()
		r := &domain.Relationship{
			TargetID:       "01REMOTE",
			Following:      true,
			FollowedBy:     true,
			ShowingReblogs: true,
		}
		out := ToRelationship(r)
		assertConformsToSchema(t, "relationship", out)
	})

	t.Run("nil relationship", func(t *testing.T) {
		t.Parallel()
		out := ToRelationship(nil)
		assertConformsToSchema(t, "relationship", out)
	})
}

func TestConformance_MediaAttachment(t *testing.T) {
	t.Parallel()

	t.Run("image attachment", func(t *testing.T) {
		t.Parallel()
		desc := "A cute cat"
		blur := "eCF6B#Rj"
		preview := "https://example.com/preview.jpg"
		m := &domain.MediaAttachment{
			ID:          "01MEDIA",
			Type:        "image",
			URL:         "https://example.com/image.jpg",
			PreviewURL:  &preview,
			Description: &desc,
			Blurhash:    &blur,
		}
		out := MediaFromDomain(m)
		assertConformsToSchema(t, "media_attachment", out)
	})

	t.Run("attachment with nil optional fields", func(t *testing.T) {
		t.Parallel()
		m := &domain.MediaAttachment{
			ID:   "01MEDIA2",
			Type: "image",
			URL:  "https://example.com/image2.jpg",
		}
		out := MediaFromDomain(m)
		assertConformsToSchema(t, "media_attachment", out)
	})
}

func TestConformance_Poll(t *testing.T) {
	t.Parallel()

	t.Run("active poll", func(t *testing.T) {
		t.Parallel()
		expires := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
		enriched := &service.EnrichedPoll{
			Poll: domain.Poll{
				ID:        "01POLL",
				Multiple:  false,
				ExpiresAt: &expires,
			},
			Options: []service.PollOptionWithCount{
				{Title: "Yes", VotesCount: 3},
				{Title: "No", VotesCount: 1},
			},
			Voted:    false,
			OwnVotes: []int{},
		}
		out := PollFromEnriched(enriched)
		assertConformsToSchema(t, "poll", out)
	})
}

func TestConformance_Card(t *testing.T) {
	t.Parallel()

	t.Run("fetched card", func(t *testing.T) {
		t.Parallel()
		c := &domain.Card{
			URL:             "https://example.com/article",
			Title:           "Test Article",
			Description:     "An article",
			Type:            "link",
			ProviderName:    "Example",
			ProviderURL:     "https://example.com",
			ImageURL:        "https://example.com/image.jpg",
			Width:           800,
			Height:          600,
			ProcessingState: domain.CardStateFetched,
		}
		out := CardFromDomain(c)
		require.NotNil(t, out)
		assertConformsToSchema(t, "card", *out)
	})

	t.Run("card without image", func(t *testing.T) {
		t.Parallel()
		c := &domain.Card{
			URL:             "https://example.com/article",
			Title:           "Test Article",
			Description:     "An article",
			Type:            "link",
			ProcessingState: domain.CardStateFetched,
		}
		out := CardFromDomain(c)
		require.NotNil(t, out)
		assertConformsToSchema(t, "card", *out)
	})
}

func TestConformance_Mention(t *testing.T) {
	t.Parallel()

	t.Run("local mention", func(t *testing.T) {
		t.Parallel()
		out := MentionFromAccount(testAccount(), "example.com")
		assertConformsToSchema(t, "mention", out)
	})

	t.Run("remote mention", func(t *testing.T) {
		t.Parallel()
		out := MentionFromAccount(testRemoteAccount(), "example.com")
		assertConformsToSchema(t, "mention", out)
	})
}
