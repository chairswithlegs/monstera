package activitypub

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
)

func TestStatusToNote(t *testing.T) {
	t.Parallel()
	instanceDomain := "example.com"
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

	note := StatusToNote(status, account, instanceDomain)
	require.NotNil(t, note)
	require.Equal(t, "https://example.com/statuses/01STATUS", note.ID)
	require.Equal(t, "Note", note.Type)
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
	instanceDomain := "example.com"
	now := time.Now()

	t.Run("no APID or URI uses instanceDomain and status ID", func(t *testing.T) {
		status := &domain.Status{ID: "01X", AccountID: "01A", CreatedAt: now}
		account := &domain.Account{ID: "01A", Username: "bob", APID: "https://example.com/users/bob"}
		note := StatusToNote(status, account, instanceDomain)
		require.NotNil(t, note)
		require.Equal(t, "https://example.com/statuses/01X", note.ID)
		require.Equal(t, "https://example.com/statuses/01X", note.URL)
	})

	t.Run("account no APID uses username fallback", func(t *testing.T) {
		status := &domain.Status{ID: "01X", APID: "https://example.com/statuses/01X", AccountID: "01A", CreatedAt: now}
		account := &domain.Account{ID: "01A", Username: "bob"}
		note := StatusToNote(status, account, instanceDomain)
		require.NotNil(t, note)
		require.Equal(t, "https://example.com/users/bob", note.AttributedTo)
	})

	t.Run("Content nil uses Text", func(t *testing.T) {
		text := "plain text"
		status := &domain.Status{ID: "01X", AccountID: "01A", Text: &text, CreatedAt: now}
		account := &domain.Account{ID: "01A", Username: "bob"}
		note := StatusToNote(status, account, instanceDomain)
		require.NotNil(t, note)
		require.Equal(t, "plain text", note.Content)
	})
}
