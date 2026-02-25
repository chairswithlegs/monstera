package email

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTemplates(t *testing.T) {
	t.Helper()
	tmpl, err := NewTemplates()
	require.NoError(t, err)
	require.NotNil(t, tmpl)
}

func TestTemplates_Render(t *testing.T) {
	t.Helper()
	tmpl, err := NewTemplates()
	require.NoError(t, err)

	tests := []struct {
		name string
		data any
	}{
		{"email_verification", VerificationData{InstanceName: "Test", Username: "alice", ConfirmURL: "https://example.com/confirm"}},
		{"password_reset", PasswordResetData{InstanceName: "Test", Username: "alice", ResetURL: "https://example.com/reset", ExpiresIn: "1 hour"}},
		{"invite", InviteData{InstanceName: "Test", InviterUsername: "bob", InviteURL: "https://example.com/register", Code: "ABC123"}},
		{"moderation_action", ModerationActionData{InstanceName: "Test", Username: "alice", Action: "warning", Reason: "Spam", AppealURL: "https://example.com/appeal"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html, text, err := tmpl.Render(tt.name, tt.data)
			require.NoError(t, err)
			assert.NotEmpty(t, html)
			assert.NotEmpty(t, text)
		})
	}
}

func TestTemplates_Render_UnknownTemplate(t *testing.T) {
	t.Helper()
	tmpl, err := NewTemplates()
	require.NoError(t, err)

	_, _, err = tmpl.Render("nonexistent", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}
