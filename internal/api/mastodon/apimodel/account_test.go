package apimodel

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToAccount_remote_account_includes_avatar_header_and_counts(t *testing.T) {
	t.Parallel()
	remoteDomain := "remote.example"
	displayName := "Kevin Roose"
	note := "<p>NYT tech columnist</p>"
	acc := &domain.Account{
		ID:             "01ABC",
		Username:       "kevin",
		Domain:         &remoteDomain,
		DisplayName:    &displayName,
		Note:           &note,
		AvatarURL:      "https://remote.example/avatars/kevin.jpg",
		HeaderURL:      "https://remote.example/headers/kevin.jpg",
		FollowersCount: 1234,
		FollowingCount: 56,
		StatusesCount:  789,
		APID:           "https://remote.example/users/kevin",
		ProfileURL:     "https://remote.example/@kevin",
		CreatedAt:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	result := ToAccount(acc, "local.example")

	assertJSONShape(t, "Account", result, accountFields)
	assert.Equal(t, "kevin@remote.example", result.Acct)
	assert.Equal(t, "https://remote.example/@kevin", result.URL)
	assert.Equal(t, "https://remote.example/avatars/kevin.jpg", result.Avatar)
	assert.Equal(t, "https://remote.example/avatars/kevin.jpg", result.AvatarStatic)
	assert.Equal(t, "https://remote.example/headers/kevin.jpg", result.Header)
	assert.Equal(t, "https://remote.example/headers/kevin.jpg", result.HeaderStatic)
	assert.Equal(t, 1234, result.FollowersCount)
	assert.Equal(t, 56, result.FollowingCount)
	assert.Equal(t, 789, result.StatusesCount)
	assert.Equal(t, "Kevin Roose", result.DisplayName)
	assert.Equal(t, "<p>NYT tech columnist</p>", result.Note)
}

func TestToAccount_local_account(t *testing.T) {
	t.Parallel()
	displayName := "Alice"
	acc := &domain.Account{
		ID:          "01LOCAL",
		Username:    "alice",
		Domain:      nil,
		DisplayName: &displayName,
		AvatarURL:   "https://local.example/media/avatar.jpg",
		HeaderURL:   "https://local.example/media/header.jpg",
		CreatedAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	result := ToAccount(acc, "local.example")

	assertJSONShape(t, "Account", result, accountFields)
	assert.Equal(t, "alice", result.Acct)
	assert.Equal(t, "https://local.example/@alice", result.URL)
	assert.Equal(t, "https://local.example/media/avatar.jpg", result.Avatar)
	assert.Equal(t, "https://local.example/media/header.jpg", result.Header)
}

func TestToAccount_empty_avatar_header(t *testing.T) {
	t.Parallel()
	acc := &domain.Account{
		ID:        "01EMPTY",
		Username:  "ghost",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	result := ToAccount(acc, "local.example")

	assertJSONShape(t, "Account", result, accountFields)
	assert.Empty(t, result.Avatar)
	assert.Empty(t, result.AvatarStatic)
	assert.Empty(t, result.Header)
	assert.Empty(t, result.HeaderStatic)
	assert.Zero(t, result.FollowersCount)
	assert.Zero(t, result.FollowingCount)
	assert.Zero(t, result.StatusesCount)
}

func TestToAccount_fields_parsed(t *testing.T) {
	t.Parallel()
	fields := json.RawMessage(`[{"name":"Website","value":"https://example.com"}]`)
	acc := &domain.Account{
		ID:        "01FIELDS",
		Username:  "carol",
		Fields:    fields,
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	result := ToAccount(acc, "local.example")

	require.Len(t, result.Fields, 1)
	assert.Equal(t, "Website", result.Fields[0].Name)
	assert.Equal(t, "https://example.com", result.Fields[0].Value)
}

func TestRegisterAccountRequest_Sanitize(t *testing.T) {
	t.Parallel()

	t.Run("strips HTML from reason", func(t *testing.T) {
		t.Parallel()
		reason := `I want to join <script>evil()</script>`
		req := RegisterAccountRequest{Reason: &reason}
		req.Sanitize()
		require.NotNil(t, req.Reason)
		assert.Equal(t, "I want to join ", *req.Reason)
	})

	t.Run("nil reason is not modified", func(t *testing.T) {
		t.Parallel()
		req := RegisterAccountRequest{Reason: nil}
		req.Sanitize()
		assert.Nil(t, req.Reason)
	})

	t.Run("plain text reason unchanged", func(t *testing.T) {
		t.Parallel()
		reason := "I am a researcher"
		req := RegisterAccountRequest{Reason: &reason}
		req.Sanitize()
		require.NotNil(t, req.Reason)
		assert.Equal(t, "I am a researcher", *req.Reason)
	})
}

func TestPostFollowedTagRequest_Sanitize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "strips HTML from tag name",
			input: `golang<script>alert(1)</script>`,
			want:  "golang",
		},
		{
			name:  "plain tag name unchanged",
			input: "golang",
			want:  "golang",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := PostFollowedTagRequest{Name: tc.input}
			req.Sanitize()
			assert.Equal(t, tc.want, req.Name)
		})
	}
}
