package service

import (
	"context"
	"fmt"
	"time"

	"github.com/chairswithlegs/monstera/internal/store"
)

// AccountDeletionService exposes the read-side of the account-deletion side
// tables to the federation workers. The write side (snapshot + targets
// population) lives inside the account-delete tx in deleteLocalAccount — this
// service is read-only for the workers and the scheduler-driven GC.
type AccountDeletionService interface {
	// GetSigningMaterial returns the PEM private key and actor IRI snapshot
	// for a deletion, so the delivery worker can sign Delete{Actor} after the
	// accounts row is gone. Returns domain.ErrNotFound if the snapshot has
	// been GC'd (e.g. delivery ran past the TTL).
	GetSigningMaterial(ctx context.Context, deletionID string) (privateKeyPEM, apID string, err error)
	// ListPendingTargets paginates undelivered follower inbox URLs for a
	// deletion, keyed by deletionID, using keyset pagination on inbox_url.
	ListPendingTargets(ctx context.Context, deletionID, cursor string, limit int) ([]string, error)
	// MarkTargetDelivered flips delivered_at on a single (deletionID,
	// inboxURL) row, preventing re-enqueue if the fanout consumer is
	// redelivered. Called by the fanout worker after the delivery message is
	// successfully enqueued.
	MarkTargetDelivered(ctx context.Context, deletionID, inboxURL string) error
	// ListPendingMediaTargets paginates undelivered storage keys for a
	// deletion, keyset-paginated by storage_key. Drives the media-purge
	// subscriber's blob-deletion loop.
	ListPendingMediaTargets(ctx context.Context, deletionID, cursor string, limit int) ([]string, error)
	// MarkMediaTargetDelivered flips delivered_at on a single (deletionID,
	// storageKey) row, preventing a repeat Delete call if the media-purge
	// subscriber is redelivered.
	MarkMediaTargetDelivered(ctx context.Context, deletionID, storageKey string) error
	// PurgeExpiredSnapshots drops snapshots past their TTL and returns the
	// count removed. Called from the scheduler on a periodic sweep.
	PurgeExpiredSnapshots(ctx context.Context, now time.Time) (int64, error)
}

type accountDeletionService struct {
	store store.Store
}

// NewAccountDeletionService returns an AccountDeletionService backed by the
// given store.
func NewAccountDeletionService(s store.Store) AccountDeletionService {
	return &accountDeletionService{store: s}
}

func (svc *accountDeletionService) GetSigningMaterial(ctx context.Context, deletionID string) (string, string, error) {
	snap, err := svc.store.GetAccountDeletionSnapshot(ctx, deletionID)
	if err != nil {
		return "", "", fmt.Errorf("GetAccountDeletionSnapshot(%s): %w", deletionID, err)
	}
	return snap.PrivateKeyPEM, snap.APID, nil
}

func (svc *accountDeletionService) ListPendingTargets(ctx context.Context, deletionID, cursor string, limit int) ([]string, error) {
	urls, err := svc.store.ListPendingAccountDeletionTargets(ctx, deletionID, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("ListPendingAccountDeletionTargets(%s): %w", deletionID, err)
	}
	return urls, nil
}

func (svc *accountDeletionService) MarkTargetDelivered(ctx context.Context, deletionID, inboxURL string) error {
	if err := svc.store.MarkAccountDeletionTargetDelivered(ctx, deletionID, inboxURL); err != nil {
		return fmt.Errorf("MarkAccountDeletionTargetDelivered: %w", err)
	}
	return nil
}

func (svc *accountDeletionService) ListPendingMediaTargets(ctx context.Context, deletionID, cursor string, limit int) ([]string, error) {
	keys, err := svc.store.ListPendingAccountDeletionMediaTargets(ctx, deletionID, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("ListPendingAccountDeletionMediaTargets(%s): %w", deletionID, err)
	}
	return keys, nil
}

func (svc *accountDeletionService) MarkMediaTargetDelivered(ctx context.Context, deletionID, storageKey string) error {
	if err := svc.store.MarkAccountDeletionMediaTargetDelivered(ctx, deletionID, storageKey); err != nil {
		return fmt.Errorf("MarkAccountDeletionMediaTargetDelivered: %w", err)
	}
	return nil
}

func (svc *accountDeletionService) PurgeExpiredSnapshots(ctx context.Context, now time.Time) (int64, error) {
	n, err := svc.store.DeleteExpiredAccountDeletionSnapshots(ctx, now)
	if err != nil {
		return 0, fmt.Errorf("DeleteExpiredAccountDeletionSnapshots: %w", err)
	}
	return n, nil
}
