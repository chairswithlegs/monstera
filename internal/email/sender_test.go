package email_test

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera-fed/internal/email"
	_ "github.com/chairswithlegs/monstera-fed/internal/email/noop"
	_ "github.com/chairswithlegs/monstera-fed/internal/email/smtp"
)

func TestNew_UnknownDriver(t *testing.T) {
	t.Helper()
	_, err := email.New(email.Config{Driver: "unknown"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown driver")
}

func TestNew_NoopOK(t *testing.T) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	sender, err := email.New(email.Config{
		Driver:   "noop",
		From:     "noreply@test.example",
		FromName: "Test",
		Logger:   logger,
	})
	require.NoError(t, err)
	require.NotNil(t, sender)
}

func TestNew_SMTPMissingHost(t *testing.T) {
	t.Helper()
	_, err := email.New(email.Config{Driver: "smtp"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "EMAIL_SMTP_HOST")
}
