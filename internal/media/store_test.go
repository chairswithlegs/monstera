package media

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageKey_Format(t *testing.T) {
	t.Helper()
	key := StorageKey("image/jpeg")
	require.NotEmpty(t, key)
	assert.Contains(t, key, "media/")
	assert.Contains(t, key, ".jpg")
}

func TestExtensionForContentType(t *testing.T) {
	t.Helper()
	tests := []struct {
		contentType string
		want        string
	}{
		{"image/jpeg", ".jpg"},
		{"image/png", ".png"},
		{"image/gif", ".gif"},
		{"image/webp", ".webp"},
		{"video/mp4", ".mp4"},
		{"video/webm", ".webm"},
		{"audio/mpeg", ".mp3"},
		{"audio/ogg", ".ogg"},
	}
	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			got := extensionForContentType(tt.contentType)
			assert.Equal(t, tt.want, got)
		})
	}
	// application/octet-stream: mime db may return "" or ".bin"
	got := extensionForContentType("application/octet-stream")
	assert.True(t, got == "" || got == ".bin", "got %q", got)
}

func TestAllowedContentTypes(t *testing.T) {
	t.Helper()
	assert.Len(t, AllowedContentTypes, 8)
	assert.Equal(t, "image", AllowedContentTypes["image/jpeg"])
	assert.Equal(t, "gifv", AllowedContentTypes["image/gif"])
	assert.Equal(t, "video", AllowedContentTypes["video/mp4"])
}

func TestNew_UnknownDriver(t *testing.T) {
	t.Helper()
	_, err := New(Config{Driver: "unknown"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown driver")
}

func TestNew_LocalMissingPath(t *testing.T) {
	t.Helper()
	_, err := New(Config{Driver: "local", BaseURL: "https://example.com"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MEDIA_LOCAL_PATH")
}

