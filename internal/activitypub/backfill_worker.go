package activitypub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/chairswithlegs/monstera/internal/activitypub/vocab"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/natsutil"
	"github.com/chairswithlegs/monstera/internal/service"
)

const pageFetchDelay = 500 * time.Millisecond

// BackfillWorker consumes backfill requests from the BACKFILL NATS stream
// and fetches remote account outboxes to backfill their statuses.
type BackfillWorker struct {
	js             jetstream.JetStream
	accounts       service.AccountService
	backfill       service.BackfillService
	remoteResolver *RemoteAccountResolver
	remoteStatuses service.RemoteStatusWriteService
	statuses       service.StatusService
	instanceDomain string
	maxPages       int
}

// NewBackfillWorker creates a new BackfillWorker.
func NewBackfillWorker(
	js jetstream.JetStream,
	accounts service.AccountService,
	backfill service.BackfillService,
	resolver *RemoteAccountResolver,
	remoteStatuses service.RemoteStatusWriteService,
	statuses service.StatusService,
	instanceDomain string,
	maxPages int,
) *BackfillWorker {
	return &BackfillWorker{
		js:             js,
		accounts:       accounts,
		backfill:       backfill,
		remoteResolver: resolver,
		remoteStatuses: remoteStatuses,
		statuses:       statuses,
		instanceDomain: instanceDomain,
		maxPages:       maxPages,
	}
}

// Start begins consuming backfill messages. Blocks until ctx is cancelled.
func (w *BackfillWorker) Start(ctx context.Context) error {
	return fmt.Errorf("backfill worker: %w", natsutil.RunConsumer(ctx, w.js, StreamBackfill, ConsumerBackfill,
		func(msg jetstream.Msg) {
			w.handleMessage(ctx, msg)
		},
		natsutil.WithMaxMessages(3),
		natsutil.WithLabel("backfill-worker"),
	))
}

func (w *BackfillWorker) handleMessage(ctx context.Context, msg jetstream.Msg) {
	accountID := string(msg.Data())
	if accountID == "" {
		slog.WarnContext(ctx, "backfill: empty account ID in message")
		if err := msg.Ack(); err != nil {
			slog.ErrorContext(ctx, "backfill: ack failed", slog.Any("error", err))
		}
		return
	}

	w.processBackfill(ctx, accountID)

	if err := msg.Ack(); err != nil {
		slog.ErrorContext(ctx, "backfill: ack failed", slog.String("account_id", accountID), slog.Any("error", err))
	}
}

func (w *BackfillWorker) processBackfill(ctx context.Context, accountID string) {
	account, err := w.accounts.GetByID(ctx, accountID)
	if err != nil {
		slog.WarnContext(ctx, "backfill: account not found", slog.String("account_id", accountID), slog.Any("error", err))
		return
	}

	if account.Domain == nil {
		return
	}
	if account.OutboxURL == "" {
		return
	}

	slog.InfoContext(ctx, "backfill: starting", slog.String("account_id", accountID), slog.String("outbox", account.OutboxURL))

	w.fetchAndProcessOutbox(ctx, account)

	// Always mark backfilled, even on partial failure.
	if err := w.backfill.MarkBackfilled(ctx, accountID); err != nil {
		slog.ErrorContext(ctx, "backfill: mark backfilled failed", slog.String("account_id", accountID), slog.Any("error", err))
	}
}

func (w *BackfillWorker) fetchAndProcessOutbox(ctx context.Context, account *domain.Account) {
	if w.remoteResolver == nil {
		slog.WarnContext(ctx, "backfill: remote resolver not configured", slog.String("account_id", account.ID))
		return
	}

	// Fetch the outbox OrderedCollection to get the first page URL.
	var outbox vocab.OrderedCollection
	if err := w.remoteResolver.resolveIRIDocument(ctx, account.OutboxURL, &outbox); err != nil {
		slog.WarnContext(ctx, "backfill: fetch outbox failed", slog.String("account_id", account.ID), slog.Any("error", err))
		return
	}

	pageURL := outbox.First
	if pageURL == "" {
		// Some implementations include items directly in the collection.
		if len(outbox.OrderedItems) > 0 {
			w.processItems(ctx, account, outbox.OrderedItems)
		}
		return
	}

	for page := 0; page < w.maxPages && pageURL != ""; page++ {
		if page > 0 {
			time.Sleep(pageFetchDelay)
		}

		var collPage vocab.OrderedCollectionPage
		if err := w.remoteResolver.resolveIRIDocument(ctx, pageURL, &collPage); err != nil {
			slog.WarnContext(ctx, "backfill: fetch page failed",
				slog.String("account_id", account.ID), slog.String("page_url", pageURL), slog.Any("error", err))
			return
		}

		w.processItems(ctx, account, collPage.OrderedItems)

		pageURL = collPage.Next
	}
}

func (w *BackfillWorker) processItems(ctx context.Context, account *domain.Account, items []json.RawMessage) {
	for _, raw := range items {
		w.processItem(ctx, account, raw)
	}
}

func (w *BackfillWorker) processItem(ctx context.Context, account *domain.Account, raw json.RawMessage) {
	// Try to parse as an Activity first (Create{Note}).
	var activity vocab.Activity
	if err := json.Unmarshal(raw, &activity); err != nil {
		slog.DebugContext(ctx, "backfill: unmarshal activity failed", slog.Any("error", err))
		return
	}

	switch activity.Type {
	case vocab.ObjectTypeCreate:
		w.processCreateActivity(ctx, account, &activity)
	case vocab.ObjectTypeNote:
		// Bare Note (not wrapped in Create).
		w.processBareNote(ctx, account, raw)
	default:
		// Ignore other activity types (Announce, Like, etc.).
	}
}

func (w *BackfillWorker) processCreateActivity(ctx context.Context, account *domain.Account, activity *vocab.Activity) {
	note, err := activity.ObjectNote()
	if err != nil {
		slog.DebugContext(ctx, "backfill: activity object is not a Note", slog.Any("error", err))
		return
	}
	if note.Type != vocab.ObjectTypeNote {
		return
	}
	w.createStatusFromNote(ctx, account, note)
}

func (w *BackfillWorker) processBareNote(ctx context.Context, account *domain.Account, raw json.RawMessage) {
	var note vocab.Note
	if err := json.Unmarshal(raw, &note); err != nil {
		return
	}
	if note.Type != vocab.ObjectTypeNote {
		return
	}
	w.createStatusFromNote(ctx, account, &note)
}

func (w *BackfillWorker) createStatusFromNote(ctx context.Context, account *domain.Account, note *vocab.Note) {
	// Skip if APID already exists locally.
	if note.ID != "" {
		if _, err := w.statuses.GetByAPID(ctx, note.ID); err == nil {
			return
		}
	}

	visibility := vocab.NoteVisibility(note, account.FollowersURL)

	// Only backfill public and unlisted statuses.
	if visibility != domain.VisibilityPublic && visibility != domain.VisibilityUnlisted {
		return
	}

	input := buildCreateStatusInput(ctx, note, account, visibility, w.statuses)

	if _, err := w.remoteStatuses.CreateRemote(ctx, input); err != nil {
		if !errors.Is(err, domain.ErrConflict) {
			slog.WarnContext(ctx, "backfill: create status failed",
				slog.String("account_id", account.ID),
				slog.String("apid", note.ID),
				slog.Any("error", err))
		}
	}
}
