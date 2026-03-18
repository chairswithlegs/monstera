package nats

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/chairswithlegs/monstera/internal/cache"
)

const bucketName = "CACHE"

// KVStore implements cache.SharedStore backed by a NATS JetStream KV bucket.
type KVStore struct {
	kv jetstream.KeyValue
}

// New creates or opens the CACHE KV bucket and returns a SharedStore.
// maxTTL sets the bucket-level maximum TTL for entries.
func New(ctx context.Context, js jetstream.JetStream, maxTTL time.Duration) (cache.SharedStore, error) {
	kv, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:  bucketName,
		TTL:     maxTTL,
		Storage: jetstream.MemoryStorage,
	})
	if err != nil {
		return nil, fmt.Errorf("cache/nats: create KV bucket: %w", err)
	}
	return &KVStore{kv: kv}, nil
}

func (s *KVStore) Get(ctx context.Context, key string) ([]byte, error) {
	entry, err := s.kv.Get(ctx, sanitizeKey(key))
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return nil, cache.ErrCacheMiss
		}
		return nil, fmt.Errorf("cache/nats: get %q: %w", key, err)
	}
	return entry.Value(), nil
}

func (s *KVStore) Set(ctx context.Context, key string, value []byte, _ time.Duration) error {
	// Per-key TTL is not supported by NATS KV; the bucket-level TTL governs expiry.
	// Callers that need different TTLs should use separate buckets or encode
	// expiry in the value. For the current use-cases the bucket TTL is sufficient.
	if _, err := s.kv.Put(ctx, sanitizeKey(key), value); err != nil {
		return fmt.Errorf("cache/nats: set %q: %w", key, err)
	}
	return nil
}

func (s *KVStore) Delete(ctx context.Context, key string) error {
	if err := s.kv.Delete(ctx, sanitizeKey(key)); err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return nil
		}
		return fmt.Errorf("cache/nats: delete %q: %w", key, err)
	}
	return nil
}

func (s *KVStore) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.kv.Get(ctx, sanitizeKey(key))
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("cache/nats: exists %q: %w", key, err)
	}
	return true, nil
}

// Close is a no-op; the KV bucket lifecycle is managed by the NATS connection.
func (s *KVStore) Close() error {
	return nil
}

// sanitizeKey replaces characters not allowed in NATS KV keys.
// NATS KV keys must match [a-zA-Z0-9_-] with dots for hierarchy.
// Colons (used in cache keys like "idempotency:{id}:{key}") are replaced with dots.
func sanitizeKey(key string) string {
	b := make([]byte, len(key))
	for i := range len(key) {
		c := key[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			c == '_', c == '-', c == '.':
			b[i] = c
		default:
			b[i] = '.'
		}
	}
	return string(b)
}
