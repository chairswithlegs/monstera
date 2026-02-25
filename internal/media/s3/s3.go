// Package s3 provides an S3-compatible MediaStore implementation using
// the AWS SDK v2. Compatible with AWS S3, MinIO, Cloudflare R2, and Backblaze B2.
package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/chairswithlegs/monstera-fed/internal/media"
)

// Config holds S3-specific configuration.
type Config struct {
	Bucket   string
	Region   string
	Endpoint string // optional: override for MinIO / R2 / B2
	CDNBase  string // optional: if set, URL() returns CDN URL instead of presigning
}

// Store is the S3-backed MediaStore implementation.
type Store struct {
	client  *s3.Client
	presign *s3.PresignClient
	cfg     Config
}

func init() {
	media.Register("s3", func(cfg media.Config) (media.MediaStore, error) {
		return New(Config{
			Bucket:   cfg.S3Bucket,
			Region:   cfg.S3Region,
			Endpoint: cfg.S3Endpoint,
			CDNBase:  cfg.CDNBase,
		})
	})
}

// New creates an S3 Store.
// Credentials are loaded from the environment (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, etc.).
func New(cfg Config) (*Store, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("media/s3: load AWS config: %w", err)
	}

	s3Opts := []func(*s3.Options){}
	if cfg.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		})
	}

	client := s3.NewFromConfig(awsCfg, s3Opts...)
	presign := s3.NewPresignClient(client)

	return &Store{
		client:  client,
		presign: presign,
		cfg:     cfg,
	}, nil
}

// Put uploads content to S3 using PutObject.
func (s *Store) Put(ctx context.Context, key string, r io.Reader, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.cfg.Bucket),
		Key:         aws.String(key),
		Body:        r,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("media/s3: put %q: %w", key, err)
	}
	return nil
}

// Get downloads the object at key from S3, returning its body as a streaming ReadCloser.
// Returns media.ErrNotFound for NoSuchKey errors.
func (s *Store) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return nil, media.ErrNotFound
		}
		return nil, fmt.Errorf("media/s3: get %q: %w", key, err)
	}
	return out.Body, nil
}

// Delete removes the object at key from S3.
func (s *Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("media/s3: delete %q: %w", key, err)
	}
	return nil
}

// URL returns a public URL for the object.
// If CDNBase is set, returns CDN URL; otherwise returns a presigned GetObject URL (1-hour expiry).
func (s *Store) URL(ctx context.Context, key string) (string, error) {
	if s.cfg.CDNBase != "" {
		return strings.TrimRight(s.cfg.CDNBase, "/") + "/" + key, nil
	}

	req, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(time.Hour))
	if err != nil {
		return "", fmt.Errorf("media/s3: presign %q: %w", key, err)
	}
	return req.URL, nil
}

func isNotFound(err error) bool {
	var nsk *s3types.NoSuchKey
	var nf *s3types.NotFound
	return errors.As(err, &nsk) || errors.As(err, &nf)
}
