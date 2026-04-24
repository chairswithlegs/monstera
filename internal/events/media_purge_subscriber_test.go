package events

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
)

// fakeMediaDeps is a minimal in-memory implementation of MediaPurgeDeps for
// tests. It holds an ordered list of storage keys per deletion_id; Mark
// removes a key from that list.
type fakeMediaDeps struct {
	mu      sync.Mutex
	pending map[string][]string
	listErr error
	markErr error
}

func newFakeMediaDeps(deletionID string, keys []string) *fakeMediaDeps {
	// Defensive copy — MarkMediaTargetDelivered shifts the slice in place,
	// which would otherwise corrupt the caller's local variable via the
	// shared backing array.
	cp := append([]string(nil), keys...)
	return &fakeMediaDeps{pending: map[string][]string{deletionID: cp}}
}

func (f *fakeMediaDeps) ListPendingMediaTargets(_ context.Context, deletionID, cursor string, limit int) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := []string{}
	for _, k := range f.pending[deletionID] {
		if cursor != "" && k <= cursor {
			continue
		}
		out = append(out, k)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (f *fakeMediaDeps) MarkMediaTargetDelivered(_ context.Context, deletionID, storageKey string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.markErr != nil {
		return f.markErr
	}
	keys := f.pending[deletionID]
	for i, k := range keys {
		if k == storageKey {
			f.pending[deletionID] = append(keys[:i], keys[i+1:]...)
			return nil
		}
	}
	return nil
}

// fakeMediaStore records Delete calls and optionally fails specific keys.
type fakeMediaStore struct {
	mu         sync.Mutex
	deleted    []string
	failKeys   map[string]error
	allFailErr error
}

func (f *fakeMediaStore) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.allFailErr != nil {
		return f.allFailErr
	}
	if err, ok := f.failKeys[key]; ok {
		return err
	}
	f.deleted = append(f.deleted, key)
	return nil
}

func (f *fakeMediaStore) deletedKeys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.deleted))
	copy(out, f.deleted)
	return out
}

func TestMediaPurgeSubscriber_purge_happy_path(t *testing.T) {
	t.Parallel()
	keys := []string{"a", "b", "c"}
	deps := newFakeMediaDeps("del1", keys)
	media := &fakeMediaStore{}
	sub := &MediaPurgeSubscriber{deps: deps, media: media}

	sub.purge(context.Background(), domain.MediaPurgePayload{PurgeID: "del1", AccountID: "acc1"})

	assert.Equal(t, keys, media.deletedKeys(), "all seeded blobs should be deleted in order")
	assert.Empty(t, deps.pending["del1"], "every target should be marked delivered (removed from pending)")
}

func TestMediaPurgeSubscriber_purge_empty_targets(t *testing.T) {
	t.Parallel()
	deps := newFakeMediaDeps("del1", nil)
	media := &fakeMediaStore{}
	sub := &MediaPurgeSubscriber{deps: deps, media: media}

	sub.purge(context.Background(), domain.MediaPurgePayload{PurgeID: "del1"})

	assert.Empty(t, media.deletedKeys())
}

func TestMediaPurgeSubscriber_purge_continues_past_delete_failure(t *testing.T) {
	t.Parallel()
	deps := newFakeMediaDeps("del1", []string{"a", "b", "c"})
	media := &fakeMediaStore{failKeys: map[string]error{"b": errors.New("s3 transient")}}
	sub := &MediaPurgeSubscriber{deps: deps, media: media}

	sub.purge(context.Background(), domain.MediaPurgePayload{PurgeID: "del1"})

	// "a" and "c" succeed; "b" is left pending for a future retry.
	assert.ElementsMatch(t, []string{"a", "c"}, media.deletedKeys())
	assert.Equal(t, []string{"b"}, deps.pending["del1"],
		"failed key must stay pending so NATS redelivery can retry it")
}

func TestMediaPurgeSubscriber_purge_advances_cursor_on_all_failed_page(t *testing.T) {
	// Regression: if every key in a page fails, the cursor must still
	// advance so we don't spin inside a single message handler.
	t.Parallel()
	keys := make([]string, mediaPurgeBatchSize+5)
	for i := range keys {
		keys[i] = string(rune('a'+i/26)) + string(rune('a'+i%26))
	}
	deps := newFakeMediaDeps("del1", keys)
	// All keys in the FIRST page fail; the tail keys delete cleanly.
	failing := map[string]error{}
	for i := range mediaPurgeBatchSize {
		failing[keys[i]] = errors.New("s3 down")
	}
	media := &fakeMediaStore{failKeys: failing}
	sub := &MediaPurgeSubscriber{deps: deps, media: media}

	sub.purge(context.Background(), domain.MediaPurgePayload{PurgeID: "del1"})

	assert.Equal(t, keys[mediaPurgeBatchSize:], media.deletedKeys(),
		"first page failed; second page should still be processed (cursor advanced)")
	assert.Equal(t, keys[:mediaPurgeBatchSize], deps.pending["del1"],
		"failed keys stay pending; successful tail was marked delivered")
}

func TestMediaPurgeSubscriber_purge_respects_context_cancellation(t *testing.T) {
	t.Parallel()
	deps := newFakeMediaDeps("del1", []string{"a", "b", "c"})
	media := &fakeMediaStore{allFailErr: context.Canceled}
	sub := &MediaPurgeSubscriber{deps: deps, media: media}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	sub.purge(ctx, domain.MediaPurgePayload{PurgeID: "del1"})

	// Ctx.Canceled from the first Delete aborts the whole sweep — we
	// should NOT log all three as failures.
	assert.Empty(t, media.deletedKeys())
	// All still pending (nothing marked).
	assert.Equal(t, []string{"a", "b", "c"}, deps.pending["del1"])
}

func TestMediaPurgeSubscriber_purge_paginates_across_multiple_pages(t *testing.T) {
	t.Parallel()
	total := mediaPurgeBatchSize * 2 // forces two full pages + an empty third
	keys := make([]string, total)
	for i := range total {
		// Lexically sortable keys: "0000".."0199".
		keys[i] = padInt(i)
	}
	deps := newFakeMediaDeps("del1", keys)
	media := &fakeMediaStore{}
	sub := &MediaPurgeSubscriber{deps: deps, media: media}

	sub.purge(context.Background(), domain.MediaPurgePayload{PurgeID: "del1"})

	assert.Equal(t, keys, media.deletedKeys())
	assert.Empty(t, deps.pending["del1"])
}

func TestMediaPurgeSubscriber_purge_stops_on_list_error(t *testing.T) {
	t.Parallel()
	deps := newFakeMediaDeps("del1", []string{"a", "b"})
	deps.listErr = errors.New("db unreachable")
	media := &fakeMediaStore{}
	sub := &MediaPurgeSubscriber{deps: deps, media: media}

	sub.purge(context.Background(), domain.MediaPurgePayload{PurgeID: "del1"})

	// List error → return early; no deletes attempted.
	assert.Empty(t, media.deletedKeys())
}

func TestNewMediaPurgeSubscriber_smoke(t *testing.T) {
	t.Parallel()
	// Smoke-test the constructor signature so it isn't silently broken
	// by a refactor. No Start() here — that requires a real JetStream.
	sub := NewMediaPurgeSubscriber(nil, newFakeMediaDeps("", nil), &fakeMediaStore{})
	require.NotNil(t, sub)
}

// padInt returns a 4-char zero-padded decimal so keys sort lexically.
func padInt(i int) string {
	return fmt.Sprintf("%04d", i)
}
