package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/chairswithlegs/monstera/internal/natsutil"
	"github.com/chairswithlegs/monstera/internal/store"
)

const (
	// BackfillSubjectPrefix is the NATS subject prefix for backfill requests.
	BackfillSubjectPrefix = "backfill.account."
)

// BackfillService enqueues background outbox backfills for remote accounts.
type BackfillService interface {
	// RequestBackfill enqueues a backfill job for the given remote account.
	// No-ops if: account is local, has no outbox URL, or was backfilled recently.
	RequestBackfill(ctx context.Context, accountID string) error
	// MarkBackfilled records the current time as the last backfill time for the account.
	MarkBackfilled(ctx context.Context, accountID string) error
}

type backfillService struct {
	store    store.Store
	js       jetstream.JetStream
	cooldown time.Duration
}

// NewBackfillService creates a new BackfillService.
func NewBackfillService(s store.Store, js jetstream.JetStream, cooldown time.Duration) BackfillService {
	return &backfillService{store: s, js: js, cooldown: cooldown}
}

func (svc *backfillService) RequestBackfill(ctx context.Context, accountID string) error {
	account, err := svc.store.GetAccountByID(ctx, accountID)
	if err != nil {
		return fmt.Errorf("RequestBackfill(%s): %w", accountID, err)
	}

	// Only backfill remote accounts.
	if account.IsLocal() {
		return nil
	}

	// Must have an outbox URL to backfill.
	if account.OutboxURL == "" {
		return nil
	}

	// Respect cooldown period.
	if account.LastBackfilledAt != nil && time.Since(*account.LastBackfilledAt) < svc.cooldown {
		return nil
	}

	// Use account ID + date-hour bucket as message ID for NATS dedup.
	bucket := time.Now().UTC().Format("2006010215")
	msgID := accountID + "-" + bucket

	subject := BackfillSubjectPrefix + accountID
	if err := natsutil.Publish(ctx, svc.js, subject, []byte(accountID), jetstream.WithMsgID(msgID)); err != nil {
		slog.WarnContext(ctx, "backfill publish failed", slog.String("account_id", accountID), slog.Any("error", err))
		return nil // fire-and-forget; don't fail the caller
	}

	return nil
}

func (svc *backfillService) MarkBackfilled(ctx context.Context, accountID string) error {
	if err := svc.store.UpdateAccountLastBackfilledAt(ctx, accountID, time.Now()); err != nil {
		return fmt.Errorf("MarkBackfilled(%s): %w", accountID, err)
	}
	return nil
}
