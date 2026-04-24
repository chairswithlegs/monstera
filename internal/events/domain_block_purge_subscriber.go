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
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// Tunables. Unexported so the subscriber's envelope is a property of its
// code, not a config knob. accountsPerMessage is deliberately conservative:
// one bounded batch of accounts plus their chunked status-deletes must fit
// inside the ConsumerDomainBlockPurge AckWait (60s). statusDeleteBatch is
// the per-chunk size for DeleteStatusesByAccountIDBatched — large enough
// that typical accounts finish in one chunk, small enough that a single
// chunk's DELETE + CASCADE produces bounded WAL and short-lived locks.
const (
	accountsPerMessage = 25
	statusDeleteBatch  = 1000
)

// DomainBlockPurgeDeps is the narrow store surface the subscriber needs.
// Matches the MediaPurgeDeps pattern: declared here so tests can substitute
// a fake without importing the service package's concrete type.
type DomainBlockPurgeDeps interface {
	GetDomainBlockPurge(ctx context.Context, blockID string) (*domain.DomainBlockPurge, error)
	UpdateDomainBlockPurgeCursor(ctx context.Context, blockID, cursor string) error
	MarkDomainBlockPurgeComplete(ctx context.Context, blockID string) error
	ListRemoteAccountsByDomainPaginated(ctx context.Context, domainName, cursor string, limit int) ([]string, error)
	InsertMediaPurgeTargetsForAccount(ctx context.Context, purgeID, accountID string) error
	DeleteStatusesByAccountIDBatched(ctx context.Context, accountID string, limit int) ([]string, error)
	// WithTx is used to group InsertMediaPurgeTargets + EventMediaPurge
	// emission per account, and UpdateDomainBlockPurgeCursor +
	// continuation event emission at the end of a batch.
	WithTx(ctx context.Context, fn func(store.Store) error) error
}

// DomainBlockPurgeSubscriber consumes EventDomainBlockSuspended events and
// drains the remote accounts for the domain in bounded per-message batches.
// Each account is suspended, its media keys are snapshotted into
// media_purge_targets, and its statuses are hard-deleted in chunked batches;
// an EventMediaPurge is emitted so MediaPurgeSubscriber (same stream) can
// delete the blobs out-of-band.
//
// Per-message work is bounded so AckWait (60s) comfortably covers the
// envelope; when more accounts remain, the handler re-publishes the same
// event to continue. State lives in domain_block_purges.cursor so
// redelivery on crash resumes from the right place.
type DomainBlockPurgeSubscriber struct {
	js   jetstream.JetStream
	deps DomainBlockPurgeDeps
}

// NewDomainBlockPurgeSubscriber creates a DomainBlockPurgeSubscriber.
func NewDomainBlockPurgeSubscriber(js jetstream.JetStream, deps DomainBlockPurgeDeps) *DomainBlockPurgeSubscriber {
	return &DomainBlockPurgeSubscriber{js: js, deps: deps}
}

// Start subscribes to ConsumerDomainBlockPurge and processes messages until
// ctx is cancelled.
func (s *DomainBlockPurgeSubscriber) Start(ctx context.Context) error {
	if err := natsutil.RunConsumer(ctx, s.js, StreamDomainEvents, ConsumerDomainBlockPurge,
		func(msg jetstream.Msg) { go s.processMessage(ctx, msg) },
		natsutil.WithLabel("domain block purge subscriber"),
	); err != nil {
		return fmt.Errorf("domain block purge subscriber: %w", err)
	}
	return nil
}

func (s *DomainBlockPurgeSubscriber) processMessage(ctx context.Context, msg jetstream.Msg) {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "domain block purge subscriber: panic in processMessage",
				slog.Any("panic", r), slog.String("subject", msg.Subject()))
			_ = msg.Nak()
		}
	}()
	var event domain.DomainEvent
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		slog.WarnContext(ctx, "domain block purge subscriber: invalid event envelope", slog.Any("error", err))
		_ = msg.Ack()
		return
	}
	if event.EventType != domain.EventDomainBlockSuspended {
		_ = msg.Ack()
		return
	}
	var payload domain.DomainBlockSuspendedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		slog.ErrorContext(ctx, "domain block purge subscriber: unmarshal payload", slog.Any("error", err))
		_ = msg.Ack()
		return
	}
	if payload.BlockID == "" {
		slog.WarnContext(ctx, "domain block purge subscriber: empty block_id")
		_ = msg.Ack()
		return
	}
	s.processBatch(ctx, payload)
	_ = msg.Ack()
}

// processBatch handles one message: drain up to accountsPerMessage accounts
// and either mark the purge complete or re-publish a continuation event.
func (s *DomainBlockPurgeSubscriber) processBatch(ctx context.Context, payload domain.DomainBlockSuspendedPayload) {
	purge, err := s.deps.GetDomainBlockPurge(ctx, payload.BlockID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			// Admin removed the block (CASCADE dropped the purge row), or
			// this is a stray event. Nothing more to do.
			return
		}
		slog.ErrorContext(ctx, "domain block purge subscriber: GetDomainBlockPurge failed",
			slog.String("block_id", payload.BlockID), slog.Any("error", err))
		return
	}
	if purge.CompletedAt != nil {
		// Already complete; redelivery / late continuation event.
		return
	}

	cursor := ""
	if purge.Cursor != nil {
		cursor = *purge.Cursor
	}

	accounts, err := s.deps.ListRemoteAccountsByDomainPaginated(ctx, payload.Domain, cursor, accountsPerMessage)
	if err != nil {
		slog.ErrorContext(ctx, "domain block purge subscriber: ListRemoteAccountsByDomainPaginated failed",
			slog.String("block_id", payload.BlockID), slog.Any("error", err))
		return
	}
	if len(accounts) == 0 {
		if err := s.deps.MarkDomainBlockPurgeComplete(ctx, payload.BlockID); err != nil {
			slog.ErrorContext(ctx, "domain block purge subscriber: MarkDomainBlockPurgeComplete failed",
				slog.String("block_id", payload.BlockID), slog.Any("error", err))
			return
		}
		slog.InfoContext(ctx, "domain block purge subscriber: complete",
			slog.String("block_id", payload.BlockID),
			slog.String("domain", payload.Domain))
		return
	}

	for _, accountID := range accounts {
		if ctx.Err() != nil {
			// Shutdown: Ack the message; next delivery resumes from cursor.
			return
		}
		if err := s.purgeAccount(ctx, payload.BlockID, accountID); err != nil {
			slog.ErrorContext(ctx, "domain block purge subscriber: purgeAccount failed",
				slog.String("block_id", payload.BlockID),
				slog.String("account_id", accountID),
				slog.Any("error", err))
			return
		}
	}

	// Advance cursor and (if more remain) re-publish a continuation event.
	// Both happen in a single tx so a crash between the cursor update and
	// the continuation publish can't leave the purge orphaned.
	lastID := accounts[len(accounts)-1]
	err = s.deps.WithTx(ctx, func(tx store.Store) error {
		if err := tx.UpdateDomainBlockPurgeCursor(ctx, payload.BlockID, lastID); err != nil {
			return fmt.Errorf("UpdateDomainBlockPurgeCursor: %w", err)
		}
		if len(accounts) == accountsPerMessage {
			if err := EmitEvent(ctx, tx, domain.EventDomainBlockSuspended, "domain_block", payload.BlockID, payload); err != nil {
				return fmt.Errorf("emit continuation: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		slog.ErrorContext(ctx, "domain block purge subscriber: cursor/continuation tx failed",
			slog.String("block_id", payload.BlockID), slog.Any("error", err))
		return
	}
	slog.InfoContext(ctx, "domain block purge subscriber: batch processed",
		slog.String("block_id", payload.BlockID),
		slog.String("domain", payload.Domain),
		slog.Int("accounts", len(accounts)),
		slog.String("cursor", lastID))
}

// purgeAccount is the per-account work: snapshot media keys, hard-delete
// all statuses in chunks, emit EventMediaPurge. Visibility (domain_suspended)
// was already flipped atomically with the domain_blocks row at
// CreateDomainBlock time, so by the time this runs the account is already
// hidden from user-facing lookups.
//
// The setup (media-target insert + EventMediaPurge emit) runs in one short
// tx. The chunked delete loop runs outside the setup tx so each chunk's WAL
// and locks are bounded. Redelivery is safe: the media-target insert uses
// ON CONFLICT DO NOTHING, the chunked delete naturally resumes on whatever
// statuses remain, and EventMediaPurge emission is inside the tx so
// duplicate emits can't occur per run — and even if they did,
// MediaPurgeSubscriber is idempotent.
func (s *DomainBlockPurgeSubscriber) purgeAccount(ctx context.Context, blockID, accountID string) error {
	purgeID := uid.New()
	err := s.deps.WithTx(ctx, func(tx store.Store) error {
		if err := tx.InsertMediaPurgeTargetsForAccount(ctx, purgeID, accountID); err != nil {
			return fmt.Errorf("InsertMediaPurgeTargetsForAccount: %w", err)
		}
		if err := EmitEvent(ctx, tx, domain.EventMediaPurge, "account", accountID, domain.MediaPurgePayload{
			PurgeID:   purgeID,
			AccountID: accountID,
		}); err != nil {
			return fmt.Errorf("EmitEvent(media.purge): %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Chunked hard-delete. Each call is its own short tx inside the store.
	for {
		if err := ctx.Err(); err != nil {
			// Shutdown: return the cancel so processBatch's caller knows
			// we bailed partway; the outer cursor update still persists
			// whatever we finished, and redelivery resumes here.
			return err
		}
		ids, err := s.deps.DeleteStatusesByAccountIDBatched(ctx, accountID, statusDeleteBatch)
		if err != nil {
			return fmt.Errorf("DeleteStatusesByAccountIDBatched(%s): %w", accountID, err)
		}
		if len(ids) == 0 {
			break
		}
	}
	slog.DebugContext(ctx, "domain block purge subscriber: account purged",
		slog.String("block_id", blockID),
		slog.String("account_id", accountID),
		slog.String("purge_id", purgeID))
	return nil
}
