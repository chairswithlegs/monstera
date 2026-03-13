package apimodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
