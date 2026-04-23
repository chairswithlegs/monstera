package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/natsutil"
)

// mediaPurgeBatchSize bounds per-iteration work so AckWait (60s) comfortably
// covers the worst case. At ~50–100ms per object-store DELETE, 100 blobs
// finish in well under 10s; larger accounts just drive more pages in the
// same message.
const mediaPurgeBatchSize = 100

// MediaPurgeDeps is the narrow service surface the subscriber needs. Declared
// here rather than pulled from the service package so tests can substitute a
// fake without importing the service package's concrete type.
type MediaPurgeDeps interface {
	ListPendingMediaTargets(ctx context.Context, deletionID, cursor string, limit int) ([]string, error)
	MarkMediaTargetDelivered(ctx context.Context, deletionID, storageKey string) error
}

// MediaBlobStore is the narrow media-driver surface the subscriber needs:
// idempotent key deletion. Both internal/media/s3 and internal/media/local
// satisfy this and return nil when the key is already absent, which makes
// redelivery safe.
type MediaBlobStore interface {
	Delete(ctx context.Context, key string) error
}

// MediaPurgeSubscriber consumes EventMediaPurge events and deletes every
// object-store blob listed in account_deletion_media_targets for the
// referenced deletion_id.
//
// The flow:
//  1. Message carries only deletion_id + account_id (payload is tiny).
//  2. Subscriber paginates the side table in chunks of mediaPurgeBatchSize,
//     calling MediaBlobStore.Delete for each key and marking the target
//     delivered on success.
//  3. Per-key failures are logged and the loop continues; the target row
//     stays unmarked so redelivery can retry it. The message is Ack'd at
//     the end regardless — the subscriber is idempotent-by-construction
//     (Delete is a no-op on missing keys) and leaving failed rows in the
//     table gives a future retry sweep something to work from.
//  4. On panic, Nak'd for redelivery.
type MediaPurgeSubscriber struct {
	js    jetstream.JetStream
	deps  MediaPurgeDeps
	media MediaBlobStore
}

// NewMediaPurgeSubscriber creates a MediaPurgeSubscriber.
func NewMediaPurgeSubscriber(js jetstream.JetStream, deps MediaPurgeDeps, media MediaBlobStore) *MediaPurgeSubscriber {
	return &MediaPurgeSubscriber{js: js, deps: deps, media: media}
}

// Start subscribes to the domain-events-media-purge consumer and processes
// messages until ctx is cancelled.
func (s *MediaPurgeSubscriber) Start(ctx context.Context) error {
	if err := natsutil.RunConsumer(ctx, s.js, StreamDomainEvents, ConsumerMediaPurge,
		func(msg jetstream.Msg) { go s.processMessage(ctx, msg) },
		natsutil.WithLabel("media purge subscriber"),
	); err != nil {
		return fmt.Errorf("media purge subscriber: %w", err)
	}
	return nil
}

func (s *MediaPurgeSubscriber) processMessage(ctx context.Context, msg jetstream.Msg) {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "media purge subscriber: panic in processMessage",
				slog.Any("panic", r), slog.String("subject", msg.Subject()))
			_ = msg.Nak()
		}
	}()
	var event domain.DomainEvent
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		slog.WarnContext(ctx, "media purge subscriber: invalid event envelope", slog.Any("error", err))
		_ = msg.Ack()
		return
	}
	if event.EventType != domain.EventMediaPurge {
		_ = msg.Ack()
		return
	}
	var payload domain.MediaPurgePayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		slog.ErrorContext(ctx, "media purge subscriber: unmarshal payload", slog.Any("error", err))
		_ = msg.Ack()
		return
	}
	if payload.DeletionID == "" {
		slog.WarnContext(ctx, "media purge subscriber: empty deletion_id")
		_ = msg.Ack()
		return
	}
	s.purge(ctx, payload)
	_ = msg.Ack()
}

// purge walks every pending target for deletionID and deletes the blob,
// marking each target delivered on success. Logs per-key failures but does
// not abort the loop — one bad key shouldn't stop the rest.
//
// The cursor advances unconditionally at the end of each page so that
// transient failures don't create an in-handler infinite loop; failed rows
// stay unmarked and will be picked up by a NATS redelivery (which starts
// over from cursor="").
func (s *MediaPurgeSubscriber) purge(ctx context.Context, payload domain.MediaPurgePayload) {
	cursor := ""
	var deleted, failed int
	for {
		keys, err := s.deps.ListPendingMediaTargets(ctx, payload.DeletionID, cursor, mediaPurgeBatchSize)
		if err != nil {
			slog.ErrorContext(ctx, "media purge subscriber: list pending targets failed",
				slog.String("deletion_id", payload.DeletionID),
				slog.Any("error", err))
			return
		}
		if len(keys) == 0 {
			break
		}
		for _, key := range keys {
			if err := s.media.Delete(ctx, key); err != nil {
				// Ctx cancellation short-circuits everything — no point
				// continuing past shutdown.
				if errors.Is(err, context.Canceled) {
					return
				}
				failed++
				slog.WarnContext(ctx, "media purge subscriber: blob delete failed",
					slog.String("deletion_id", payload.DeletionID),
					slog.String("storage_key", key),
					slog.Any("error", err))
				continue
			}
			if err := s.deps.MarkMediaTargetDelivered(ctx, payload.DeletionID, key); err != nil {
				// Rare (DB blip). Log and move on; redelivery will
				// re-delete the blob (no-op on missing key) and retry
				// the mark.
				slog.WarnContext(ctx, "media purge subscriber: mark delivered failed",
					slog.String("deletion_id", payload.DeletionID),
					slog.String("storage_key", key),
					slog.Any("error", err))
				continue
			}
			deleted++
		}
		// Advance past the last key in this page regardless of per-key
		// outcomes. Without this, a page where every delete fails leaves
		// cursor unchanged and the next ListPending returns the same keys,
		// spinning inside this message until AckWait fires. Failed rows
		// stay unmarked — redelivery (with cursor="") sees them again.
		cursor = keys[len(keys)-1]
		if len(keys) < mediaPurgeBatchSize {
			break
		}
	}
	if deleted > 0 || failed > 0 {
		slog.InfoContext(ctx, "media purge subscriber: sweep complete",
			slog.String("deletion_id", payload.DeletionID),
			slog.String("account_id", payload.AccountID),
			slog.Int("deleted", deleted),
			slog.Int("failed", failed))
	}
}
