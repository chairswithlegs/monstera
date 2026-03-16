package vocab

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
)

const testInstanceBase = "https://example.com"

func TestStatusToNote(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC)
	content := "<p>Hello world</p>"
	cw := "spoiler"

	status := &domain.Status{
		ID:             "01STATUS",
		APID:           "https://example.com/statuses/01STATUS",
		URI:            "https://example.com/statuses/01STATUS",
		AccountID:      "01ACC",
		Content:        &content,
		ContentWarning: &cw,
		Sensitive:      true,
		CreatedAt:      now,
	}
	account := &domain.Account{
		ID:       "01ACC",
		Username: "alice",
		APID:     "https://example.com/users/alice",
	}

	note := StatusToNote(status, account, testInstanceBase)
	require.NotNil(t, note)
	require.Equal(t, "https://example.com/statuses/01STATUS", note.ID)
	require.Equal(t, ObjectTypeNote, note.Type)
	require.Equal(t, "https://example.com/users/alice", note.AttributedTo)
	require.Equal(t, "<p>Hello world</p>", note.Content)
	require.Equal(t, []string{PublicAddress}, note.To)
	require.Equal(t, "2026-02-25T12:00:00Z", note.Published)
	require.Equal(t, "https://example.com/statuses/01STATUS", note.URL)
	require.True(t, note.Sensitive)
	require.NotNil(t, note.Summary)
	require.Equal(t, "spoiler", *note.Summary)
}

func TestStatusToNote_fallbacks(t *testing.T) {
	t.Parallel()
	now := time.Now()

	t.Run("no APID or URI uses instanceBase and status ID", func(t *testing.T) {
		status := &domain.Status{ID: "01X", AccountID: "01A", CreatedAt: now}
		account := &domain.Account{ID: "01A", Username: "bob", APID: "https://example.com/users/bob"}
		note := StatusToNote(status, account, testInstanceBase)
		require.NotNil(t, note)
		require.Equal(t, "https://example.com/statuses/01X", note.ID)
		require.Equal(t, "https://example.com/statuses/01X", note.URL)
	})

	t.Run("account no APID uses username fallback", func(t *testing.T) {
		status := &domain.Status{ID: "01X", APID: "https://example.com/statuses/01X", AccountID: "01A", CreatedAt: now}
		account := &domain.Account{ID: "01A", Username: "bob"}
		note := StatusToNote(status, account, testInstanceBase)
		require.NotNil(t, note)
		require.Equal(t, "https://example.com/users/bob", note.AttributedTo)
	})

	t.Run("Content nil uses Text with HTML escape", func(t *testing.T) {
		text := "<script>alert('xss')</script>"
		status := &domain.Status{ID: "01X", AccountID: "01A", Text: &text, CreatedAt: now}
		account := &domain.Account{ID: "01A", Username: "bob"}
		note := StatusToNote(status, account, testInstanceBase)
		require.NotNil(t, note)
		require.Equal(t, "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;", note.Content)
	})

	t.Run("InReplyToID populates InReplyTo IRI", func(t *testing.T) {
		parentID := "01PARENT"
		status := &domain.Status{ID: "01X", AccountID: "01A", InReplyToID: &parentID, CreatedAt: now}
		account := &domain.Account{ID: "01A", Username: "bob"}
		note := StatusToNote(status, account, testInstanceBase)
		require.NotNil(t, note)
		require.NotNil(t, note.InReplyTo)
		require.Equal(t, "https://example.com/statuses/01PARENT", *note.InReplyTo)
	})

	t.Run("EditedAt populates Updated", func(t *testing.T) {
		editedAt := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
		status := &domain.Status{ID: "01X", AccountID: "01A", EditedAt: &editedAt, CreatedAt: now}
		account := &domain.Account{ID: "01A", Username: "bob"}
		note := StatusToNote(status, account, testInstanceBase)
		require.NotNil(t, note)
		require.Equal(t, "2026-03-01T10:00:00Z", note.Updated)
	})
}

func TestNote_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	note := Note{
		Context:      DefaultContext,
		ID:           "https://example.com/statuses/1",
		Type:         ObjectTypeNote,
		AttributedTo: "https://example.com/users/alice",
		Content:      "<p>Hello world</p>",
		To:           []string{PublicAddress},
		Published:    "2026-02-25T12:00:00Z",
		URL:          "https://example.com/statuses/1",
		Sensitive:    false,
	}
	data, err := json.Marshal(note)
	require.NoError(t, err)
	var decoded Note
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, note.ID, decoded.ID)
	assert.Equal(t, note.Content, decoded.Content)
	assert.Equal(t, note.AttributedTo, decoded.AttributedTo)
}

func TestNoteVisibility(t *testing.T) {
	t.Parallel()
	followersURL := "https://example.com/users/alice/followers"

	t.Run("public when PublicAddress in To", func(t *testing.T) {
		t.Parallel()
		note := &Note{To: []string{PublicAddress}, Cc: []string{followersURL}}
		assert.Equal(t, domain.VisibilityPublic, NoteVisibility(note, followersURL))
	})

	t.Run("unlisted when PublicAddress in Cc", func(t *testing.T) {
		t.Parallel()
		note := &Note{To: []string{followersURL}, Cc: []string{PublicAddress}}
		assert.Equal(t, domain.VisibilityUnlisted, NoteVisibility(note, followersURL))
	})

	t.Run("private when followersURL in To", func(t *testing.T) {
		t.Parallel()
		note := &Note{To: []string{followersURL}, Cc: []string{}}
		assert.Equal(t, domain.VisibilityPrivate, NoteVisibility(note, followersURL))
	})

	t.Run("direct when no match", func(t *testing.T) {
		t.Parallel()
		note := &Note{To: []string{"https://example.com/users/bob"}, Cc: []string{}}
		assert.Equal(t, domain.VisibilityDirect, NoteVisibility(note, followersURL))
	})
}

func TestNoteLanguage(t *testing.T) {
	t.Parallel()

	t.Run("returns language from ContentMap", func(t *testing.T) {
		t.Parallel()
		note := &Note{ContentMap: map[string]string{"en": "<p>Hello</p>"}}
		lang := NoteLanguage(note)
		require.NotNil(t, lang)
		assert.Equal(t, "en", *lang)
	})

	t.Run("returns nil for empty ContentMap", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, NoteLanguage(&Note{}))
	})
}

func TestNoteToStatusFields(t *testing.T) {
	t.Parallel()

	t.Run("standard note", func(t *testing.T) {
		t.Parallel()
		note := &Note{
			ID:        "https://example.com/statuses/01S",
			Sensitive: true,
		}
		fields := NoteToStatusFields(note)
		assert.Equal(t, "https://example.com/statuses/01S", fields.URI)
		assert.Equal(t, "https://example.com/statuses/01S", fields.APID)
		assert.True(t, fields.Sensitive)
		assert.Nil(t, fields.Language)
		assert.NotEmpty(t, fields.ApRaw)
	})

	t.Run("with ContentMap sets Language", func(t *testing.T) {
		t.Parallel()
		note := &Note{
			ID:         "https://example.com/statuses/01S",
			ContentMap: map[string]string{"fr": "<p>Bonjour</p>"},
		}
		fields := NoteToStatusFields(note)
		require.NotNil(t, fields.Language)
		assert.Equal(t, "fr", *fields.Language)
	})
}
