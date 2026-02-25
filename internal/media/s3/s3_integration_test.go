//go:build integration

package s3

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera-fed/internal/media"
)

// TestStore_Integration requires MinIO (or S3-compatible server) running with credentials set.
// Set MINIO_ENDPOINT (e.g. http://localhost:9000), S3_TEST_BUCKET, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY.
func TestStore_Integration(t *testing.T) {
	t.Helper()
	endpoint := getEnv("MINIO_ENDPOINT", "http://localhost:9000")
	bucket := getEnv("S3_TEST_BUCKET", "test-bucket")
	region := getEnv("AWS_REGION", "us-east-1")

	ensureBucket(t, endpoint, bucket, region)

	cfg := Config{
		Bucket:   bucket,
		Region:   region,
		Endpoint: endpoint,
	}
	store, err := New(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	key := "media/2026/02/25/integration-test.jpg"
	content := []byte("integration test body")

	err = store.Put(ctx, key, bytes.NewReader(content), "image/jpeg")
	require.NoError(t, err)

	rc, err := store.Get(ctx, key)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	_ = rc.Close()
	require.NoError(t, err)
	assert.Equal(t, content, got)

	url, err := store.URL(ctx, key)
	require.NoError(t, err)
	assert.NotEmpty(t, url)

	err = store.Delete(ctx, key)
	require.NoError(t, err)

	_, err = store.Get(ctx, key)
	assert.ErrorIs(t, err, media.ErrNotFound)
}

// ensureBucket creates the test bucket if it does not exist (MinIO does not auto-create buckets).
func ensureBucket(t *testing.T, endpoint, bucket, region string) {
	t.Helper()
	ctx := context.Background()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	require.NoError(t, err)
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})
	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)})
	if err != nil {
		var alreadyExists *s3types.BucketAlreadyExists
		var alreadyOwned *s3types.BucketAlreadyOwnedByYou
		if errors.As(err, &alreadyExists) || errors.As(err, &alreadyOwned) {
			return
		}
		require.NoError(t, err)
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
