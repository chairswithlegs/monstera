package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// RedisStore is the Redis cache implementation.
type RedisStore struct {
	client *goredis.Client
}

// NewRedis parses redisURL and returns a connected Store.
func NewRedis(redisURL string) (*RedisStore, error) {
	opts, err := goredis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("cache/redis: parse URL: %w", err)
	}
	opts.PoolSize = 10
	opts.MinIdleConns = 2
	opts.ConnMaxIdleTime = 5 * time.Minute
	client := goredis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("cache/redis: ping: %w", err)
	}
	return &RedisStore{client: client}, nil
}

// Get retrieves the raw bytes stored at key.
func (s *RedisStore) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, ErrCacheMiss
		}
		return nil, fmt.Errorf("cache/redis: get %q: %w", key, err)
	}
	return val, nil
}

// Set stores value at key with the given TTL.
func (s *RedisStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := s.client.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("cache/redis: set %q: %w", key, err)
	}
	return nil
}

// Delete removes key from Redis.
func (s *RedisStore) Delete(ctx context.Context, key string) error {
	if err := s.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("cache/redis: del %q: %w", key, err)
	}
	return nil
}

// Exists reports whether key is present and unexpired.
func (s *RedisStore) Exists(ctx context.Context, key string) (bool, error) {
	n, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("cache/redis: exists %q: %w", key, err)
	}
	return n > 0, nil
}

// Ping satisfies the Pinger interface.
func (s *RedisStore) Ping(ctx context.Context) error {
	if err := s.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("cache/redis: ping: %w", err)
	}
	return nil
}

// Close gracefully shuts down the connection pool.
func (s *RedisStore) Close() error {
	if err := s.client.Close(); err != nil {
		return fmt.Errorf("cache/redis: close: %w", err)
	}
	return nil
}
