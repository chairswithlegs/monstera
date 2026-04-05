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
	"github.com/chairswithlegs/monstera/internal/blocklist"
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
	remoteFollows  service.RemoteFollowService
	statuses       service.StatusService
	blocklist      *blocklist.BlocklistCache
	instanceDomain string
	maxPages       int
	// cooldown is the minimum time between backfill runs for the same account.
	cooldown time.Duration
}

// NewBackfillWorker creates a new BackfillWorker.
func NewBackfillWorker(
	js jetstream.JetStream,
	accounts service.AccountService,
	backfill service.BackfillService,
	resolver *RemoteAccountResolver,
	remoteStatuses service.RemoteStatusWriteService,
	remoteFollows service.RemoteFollowService,
	statuses service.StatusService,
	bl *blocklist.BlocklistCache,
	instanceDomain string,
	maxPages int,
	cooldown time.Duration,
) *BackfillWorker {
	return &BackfillWorker{
		js:             js,
		accounts:       accounts,
		backfill:       backfill,
		remoteResolver: resolver,
		remoteStatuses: remoteStatuses,
		remoteFollows:  remoteFollows,
		statuses:       statuses,
		blocklist:      bl,
		instanceDomain: instanceDomain,
		maxPages:       maxPages,
		cooldown:       cooldown,
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
	if w.blocklist != nil && w.blocklist.IsSuspended(ctx, *account.Domain) {
		slog.DebugContext(ctx, "backfill: skipping suspended domain", slog.String("account_id", accountID), slog.String("domain", *account.Domain))
		return
	}
	if account.OutboxURL == "" {
		return
	}

	// Skip if another backfill job already ran for this account within the cooldown window.
	if account.LastBackfilledAt != nil && time.Since(*account.LastBackfilledAt) < w.cooldown {
		slog.DebugContext(ctx, "backfill: skipping, recently backfilled", slog.String("account_id", accountID))
		return
	}

	slog.InfoContext(ctx, "backfill: starting", slog.String("account_id", accountID), slog.String("outbox", account.OutboxURL))

	w.fetchAndProcessOutbox(ctx, account)

	pinnedIDs, shouldUpdate := w.fetchAndProcessFeatured(ctx, account)
	if shouldUpdate {
		if err := w.accounts.SetRemotePins(ctx, accountID, pinnedIDs); err != nil {
			slog.ErrorContext(ctx, "backfill: set remote pins failed", slog.String("account_id", accountID), slog.Any("error", err))
		}
	}

	w.fetchAndProcessFollowing(ctx, account)

	// Always mark backfilled, even on partial failure.
	if err := w.backfill.MarkBackfilled(ctx, accountID); err != nil {
		slog.ErrorContext(ctx, "backfill: mark backfilled failed", slog.String("account_id", accountID), slog.Any("error", err))
	}
}

// fetchAndProcessFeatured fetches the remote account's AP featured collection and returns
// the local status IDs of any pinned notes that are already stored locally, along with a
// boolean indicating whether the caller should proceed to update pins.
// Returns (nil, true) if the account has no featured URL (safe to clear stale pins).
// Returns (nil, false) if the fetch fails (preserve existing pins rather than wiping them).
// Unknown Note IRIs (not yet in the local DB) are skipped gracefully.
func (w *BackfillWorker) fetchAndProcessFeatured(ctx context.Context, account *domain.Account) (statusIDs []string, shouldUpdate bool) {
	if account.FeaturedURL == "" {
		return nil, true
	}
	if w.remoteResolver == nil {
		return nil, false
	}

	var coll vocab.OrderedCollection
	if err := w.remoteResolver.resolveIRIDocument(ctx, account.FeaturedURL, &coll); err != nil {
		slog.WarnContext(ctx, "backfill: fetch featured collection failed",
			slog.String("account_id", account.ID), slog.Any("error", err))
		return nil, false
	}

	for _, raw := range coll.OrderedItems {
		iri := extractIRI(raw)
		if iri == "" {
			continue
		}
		st, err := w.statuses.GetByAPID(ctx, iri)
		if err != nil {
			// Status not yet stored locally — skip.
			continue
		}
		statusIDs = append(statusIDs, st.ID)
	}
	return statusIDs, true
}

// fetchAndProcessFollowing fetches the remote account's AP following collection and stores
// each follow relationship locally. Actors not yet known to this instance are resolved and
// created via RemoteAccountResolver. Duplicate follows are silently discarded.
func (w *BackfillWorker) fetchAndProcessFollowing(ctx context.Context, account *domain.Account) {
	if account.FollowingURL == "" {
		return
	}
	if w.remoteResolver == nil {
		return
	}

	var coll vocab.OrderedCollection
	if err := w.remoteResolver.resolveIRIDocument(ctx, account.FollowingURL, &coll); err != nil {
		slog.WarnContext(ctx, "backfill: fetch following collection failed",
			slog.String("account_id", account.ID), slog.Any("error", err))
		return
	}

	pageURL := coll.First
	if pageURL == "" {
		w.processFollowingItems(ctx, account, coll.OrderedItems)
		return
	}

	for page := 0; page < w.maxPages && pageURL != ""; page++ {
		if page > 0 {
			time.Sleep(pageFetchDelay)
		}
		var collPage vocab.OrderedCollectionPage
		if err := w.remoteResolver.resolveIRIDocument(ctx, pageURL, &collPage); err != nil {
			slog.WarnContext(ctx, "backfill: fetch following page failed",
				slog.String("account_id", account.ID), slog.String("page_url", pageURL), slog.Any("error", err))
			return
		}
		w.processFollowingItems(ctx, account, collPage.OrderedItems)
		pageURL = collPage.Next
	}
}

func (w *BackfillWorker) processFollowingItems(ctx context.Context, account *domain.Account, items []json.RawMessage) {
	for _, raw := range items {
		iri := extractIRI(raw)
		if iri == "" {
			continue
		}
		// Skip self-follows from misconfigured servers.
		if iri == account.APID {
			continue
		}
		target, err := w.remoteResolver.ResolveRemoteAccountByIRI(ctx, iri)
		if err != nil {
			slog.DebugContext(ctx, "backfill: resolve following actor failed",
				slog.String("account_id", account.ID), slog.String("iri", iri), slog.Any("error", err))
			continue
		}
		if _, err := w.remoteFollows.CreateRemoteFollow(ctx, account.ID, target.ID, domain.FollowStateAccepted, nil); err != nil {
			if !errors.Is(err, domain.ErrConflict) {
				slog.WarnContext(ctx, "backfill: store following relationship failed",
					slog.String("account_id", account.ID), slog.String("target_id", target.ID), slog.Any("error", err))
			}
		}
	}
}

// extractIRI returns the IRI from a raw JSON item that is either a bare string
// or an object with an "id" field.
func extractIRI(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var obj struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return obj.ID
	}
	return ""
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
