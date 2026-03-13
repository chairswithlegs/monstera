package apimodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPatchProfileRequest_Sanitize(t *testing.T) {
	t.Parallel()

	t.Run("strips dangerous tags from display_name", func(t *testing.T) {
		t.Parallel()
		name := `Alice <script>evil()</script>`
		req := PatchProfileRequest{DisplayName: &name}
		req.Sanitize()
		require.NotNil(t, req.DisplayName)
		assert.Equal(t, "Alice ", *req.DisplayName)
	})

	t.Run("strips HTML from display_name (plain text only)", func(t *testing.T) {
		t.Parallel()
		name := `<b>Alice</b>`
		req := PatchProfileRequest{DisplayName: &name}
		req.Sanitize()
		require.NotNil(t, req.DisplayName)
		assert.Equal(t, "Alice", *req.DisplayName)
	})

	t.Run("preserves safe HTML in note (UGC policy)", func(t *testing.T) {
		t.Parallel()
		note := `<p>Hello <a href="https://example.com">link</a></p>`
		req := PatchProfileRequest{Note: &note}
		req.Sanitize()
		require.NotNil(t, req.Note)
		assert.Contains(t, *req.Note, "<p>")
		assert.Contains(t, *req.Note, "<a href=")
	})

	t.Run("strips dangerous tags from note", func(t *testing.T) {
		t.Parallel()
		note := `bio <script>alert(1)</script> text`
		req := PatchProfileRequest{Note: &note}
		req.Sanitize()
		require.NotNil(t, req.Note)
		assert.NotContains(t, *req.Note, "<script>")
		assert.Contains(t, *req.Note, "bio")
	})

	t.Run("nil pointers are not modified", func(t *testing.T) {
		t.Parallel()
		req := PatchProfileRequest{}
		req.Sanitize()
		assert.Nil(t, req.DisplayName)
		assert.Nil(t, req.Note)
	})
}

func TestPatchPreferencesRequest_Sanitize(t *testing.T) {
	t.Parallel()

	t.Run("strips HTML from default_privacy", func(t *testing.T) {
		t.Parallel()
		req := PatchPreferencesRequest{DefaultPrivacy: `public<script>bad()</script>`}
		req.Sanitize()
		assert.Equal(t, "public", req.DefaultPrivacy)
	})

	t.Run("strips HTML from default_language", func(t *testing.T) {
		t.Parallel()
		// StrictPolicy removes tags and the content of script-like elements.
		// Text nodes inside non-script tags are preserved; use a script tag to test full removal.
		req := PatchPreferencesRequest{DefaultLanguage: `en<script>bad()</script>`}
		req.Sanitize()
		assert.Equal(t, "en", req.DefaultLanguage)
	})
}

func TestPatchEmailRequest_Sanitize(t *testing.T) {
	t.Parallel()

	t.Run("strips HTML from email", func(t *testing.T) {
		t.Parallel()
		req := PatchEmailRequest{Email: `user@example.com<script>bad()</script>`}
		req.Sanitize()
		assert.Equal(t, "user@example.com", req.Email)
	})
}

func TestPostAnnouncementRequest_Sanitize(t *testing.T) {
	t.Parallel()

	t.Run("strips dangerous tags", func(t *testing.T) {
		t.Parallel()
		req := PostAnnouncementRequest{Content: `Announcement <script>alert(1)</script>`}
		req.Sanitize()
		assert.NotContains(t, req.Content, "<script>")
		assert.Contains(t, req.Content, "Announcement")
	})

	t.Run("preserves safe HTML formatting (UGC policy)", func(t *testing.T) {
		t.Parallel()
		req := PostAnnouncementRequest{Content: `<p>Server <strong>maintenance</strong> tonight</p>`}
		req.Sanitize()
		assert.Contains(t, req.Content, "<p>")
		assert.Contains(t, req.Content, "<strong>")
	})
}

func TestPostServerFilterRequest_Sanitize(t *testing.T) {
	t.Parallel()

	t.Run("strips HTML from phrase", func(t *testing.T) {
		t.Parallel()
		req := PostServerFilterRequest{Phrase: `badword<script>evil()</script>`}
		req.Sanitize()
		assert.Equal(t, "badword", req.Phrase)
	})
}

func TestPutServerFilterRequest_Sanitize(t *testing.T) {
	t.Parallel()

	t.Run("strips HTML from phrase", func(t *testing.T) {
		t.Parallel()
		req := PutServerFilterRequest{Phrase: `<b>badword</b>`}
		req.Sanitize()
		assert.Equal(t, "badword", req.Phrase)
	})
}

func TestPostDomainBlocksRequest_Sanitize(t *testing.T) {
	t.Parallel()

	t.Run("strips HTML from reason", func(t *testing.T) {
		t.Parallel()
		reason := `spam <script>alert(1)</script>`
		req := PostDomainBlocksRequest{Reason: &reason}
		req.Sanitize()
		require.NotNil(t, req.Reason)
		assert.Equal(t, "spam ", *req.Reason)
	})

	t.Run("nil reason is not modified", func(t *testing.T) {
		t.Parallel()
		req := PostDomainBlocksRequest{Reason: nil}
		req.Sanitize()
		assert.Nil(t, req.Reason)
	})
}

func TestPostRejectRegistrationRequest_Sanitize(t *testing.T) {
	t.Parallel()

	t.Run("strips HTML from reason", func(t *testing.T) {
		t.Parallel()
		req := PostRejectRegistrationRequest{Reason: `not eligible <script>bad()</script>`}
		req.Sanitize()
		assert.Equal(t, "not eligible ", req.Reason)
	})

	t.Run("plain text unchanged", func(t *testing.T) {
		t.Parallel()
		req := PostRejectRegistrationRequest{Reason: "not eligible"}
		req.Sanitize()
		assert.Equal(t, "not eligible", req.Reason)
	})
}
