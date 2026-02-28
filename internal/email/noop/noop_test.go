package noop

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera-fed/internal/email"
)

func TestSender_Send(t *testing.T) {
	t.Helper()
	s, err := New("noreply@test.example", "Test")
	require.NoError(t, err)

	err = s.Send(context.Background(), email.Message{
		To:      "user@example.com",
		Subject: "Test",
		HTML:    "<p>Hi</p>",
		Text:    "Hi",
	})
	assert.NoError(t, err)
}

func TestSender_SendUsesDefaultFrom(t *testing.T) {
	t.Helper()
	s, err := New("default@test.example", "Test")
	require.NoError(t, err)

	err = s.Send(context.Background(), email.Message{
		To:      "user@example.com",
		Subject: "Test",
		HTML:    "",
		Text:    "",
		From:    "", // empty so default is used
	})
	assert.NoError(t, err)
}
