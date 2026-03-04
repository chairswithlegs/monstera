// Package media_test tests the media package with driver registration.
// It uses the external test package to avoid import cycles (media -> local -> media).
package media_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/media"
	_ "github.com/chairswithlegs/monstera/internal/media/local"
	_ "github.com/chairswithlegs/monstera/internal/media/s3"
)

func TestNew_LocalOK(t *testing.T) {
	t.Helper()
	store, err := media.New(media.Config{Driver: "local", LocalPath: t.TempDir(), BaseURL: "https://example.com"})
	require.NoError(t, err)
	require.NotNil(t, store)
}

func TestNew_S3MissingBucket(t *testing.T) {
	t.Helper()
	_, err := media.New(media.Config{Driver: "s3", S3Region: "us-east-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MEDIA_S3_BUCKET")
}
