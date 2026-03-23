package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/events"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// StatusInteractionService handles status interactions: reblog, favourite, bookmark, pin, and poll votes.
// It depends on StatusService for reads and visibility checks.
type StatusInteractionService interface {
	CreateReblog(ctx context.Context, accountID, username, statusID string) (EnrichedStatus, error)
	DeleteReblog(ctx context.Context, accountID, statusID string) error
	CreateFavourite(ctx context.Context, accountID, statusID string) (EnrichedStatus, error)
	DeleteFavourite(ctx context.Context, accountID, statusID string) (EnrichedStatus, error)
	Bookmark(ctx context.Context, accountID, statusID string) (EnrichedStatus, error)
	Unbookmark(ctx context.Context, accountID, statusID string) (EnrichedStatus, error)
	Pin(ctx context.Context, accountID, statusID string) (EnrichedStatus, error)
	Unpin(ctx context.Context, accountID, statusID string) (EnrichedStatus, error)
	RecordVote(ctx context.Context, pollID, accountID string, optionIndices []int) (*EnrichedPoll, error)
}

type statusInteractionService struct {
	store           store.Store
	statusSvc       StatusService
	instanceBaseURL string
}

// NewStatusInteractionService returns a StatusInteractionService.
func NewStatusInteractionService(s store.Store, statusSvc StatusService, instanceBaseURL string) StatusInteractionService {
	base := strings.TrimSuffix(instanceBaseURL, "/")
	return &statusInteractionService{
		store:           s,
		statusSvc:       statusSvc,
		instanceBaseURL: base,
	}
}

// CreateReblog creates a reblog status for the given status. Returns the new reblog status (with nested original).
func (svc *statusInteractionService) CreateReblog(ctx context.Context, accountID, username, statusID string) (EnrichedStatus, error) {
	orig, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("CreateReblog: %w", err)
	}
	if orig.DeletedAt != nil {
		return EnrichedStatus{}, fmt.Errorf("CreateReblog: %w", domain.ErrNotFound)
	}
	viewerID := &accountID
	ok, err := svc.statusSvc.CanViewStatus(ctx, orig, viewerID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("CreateReblog: %w", err)
	}
	if !ok {
		return EnrichedStatus{}, fmt.Errorf("CreateReblog: %w", domain.ErrNotFound)
	}
	if orig.Visibility != domain.VisibilityPublic && orig.Visibility != domain.VisibilityUnlisted {
		return EnrichedStatus{}, fmt.Errorf("CreateReblog: %w", domain.ErrForbidden)
	}
	existing, err := svc.store.GetReblogByAccountAndTarget(ctx, accountID, statusID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		slog.WarnContext(ctx, "CreateReblog: check existing reblog", slog.Any("error", err))
	}
	if existing != nil {
		out, err := svc.statusSvc.GetByIDEnriched(ctx, existing.ID, &accountID)
		if err != nil {
			return EnrichedStatus{}, fmt.Errorf("CreateReblog: %w", err)
		}
		return out, nil
	}
	reblogID := uid.New()
	reblogURI := fmt.Sprintf("%s/users/%s/statuses/%s", svc.instanceBaseURL, username, reblogID)
	err = svc.store.WithTx(ctx, func(tx store.Store) error {
		_, err := tx.CreateStatus(ctx, store.CreateStatusInput{
			ID:                  reblogID,
			URI:                 reblogURI,
			AccountID:           accountID,
			Text:                nil,
			Content:             nil,
			Visibility:          orig.Visibility,
			Language:            nil,
			InReplyToID:         nil,
			ReblogOfID:          &statusID,
			Sensitive:           orig.Sensitive,
			Local:               true,
			APID:                reblogURI,
			QuoteApprovalPolicy: domain.QuotePolicyPublic,
		})
		if err != nil {
			return fmt.Errorf("CreateStatus: %w", err)
		}
		if err := tx.IncrementReblogsCount(ctx, statusID); err != nil {
			return fmt.Errorf("IncrementReblogsCount: %w", err)
		}
		// Best-effort enrichment: these accounts are known to exist, so failures
		// indicate a transient DB issue. We log and continue rather than failing
		// the TX over event payload enrichment.
		rebloggerAccount, txErr := tx.GetAccountByID(ctx, accountID)
		if txErr != nil {
			slog.WarnContext(ctx, "CreateReblog: get reblogger account for event", slog.Any("error", txErr))
		}
		originalAuthor, txErr := tx.GetAccountByID(ctx, orig.AccountID)
		if txErr != nil {
			slog.WarnContext(ctx, "CreateReblog: get original author for event", slog.Any("error", txErr))
		}
		originalStatusAPID := orig.APID
		if originalStatusAPID == "" {
			originalStatusAPID = orig.URI
		}
		if originalStatusAPID == "" {
			originalStatusAPID = fmt.Sprintf("%s/statuses/%s", svc.instanceBaseURL, orig.ID)
		}
		return events.EmitEvent(ctx, tx, domain.EventReblogCreated, "status", reblogID, domain.ReblogCreatedPayload{
			AccountID:          accountID,
			ReblogStatusID:     reblogID,
			OriginalStatusID:   statusID,
			OriginalAuthorID:   orig.AccountID,
			FromAccount:        rebloggerAccount,
			OriginalAuthor:     originalAuthor,
			OriginalStatusAPID: originalStatusAPID,
		})
	})
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("CreateReblog: %w", err)
	}
	out, err := svc.statusSvc.GetByIDEnriched(ctx, reblogID, &accountID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("CreateReblog: %w", err)
	}
	return out, nil
}

// DeleteReblog removes the viewer's reblog of the given status. Idempotent: if no reblog exists, returns nil.
func (svc *statusInteractionService) DeleteReblog(ctx context.Context, accountID, statusID string) error {
	reblog, err := svc.store.GetReblogByAccountAndTarget(ctx, accountID, statusID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("DeleteReblog: %w", err)
	}
	if reblog == nil {
		return nil
	}
	// Best-effort enrichment: the original status enriches the delete event payload.
	// The delete still proceeds if this lookup fails.
	orig, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil {
		slog.WarnContext(ctx, "DeleteReblog: get original status for event", slog.Any("error", err), slog.String("status_id", statusID))
	}
	var originalStatusAPID string
	if orig != nil {
		originalStatusAPID = orig.APID
		if originalStatusAPID == "" {
			originalStatusAPID = orig.URI
		}
		if originalStatusAPID == "" {
			originalStatusAPID = fmt.Sprintf("%s/statuses/%s", svc.instanceBaseURL, orig.ID)
		}
	}
	if err := svc.store.WithTx(ctx, func(tx store.Store) error {
		if err := tx.SoftDeleteStatus(ctx, reblog.ID); err != nil {
			return fmt.Errorf("SoftDeleteStatus: %w", err)
		}
		if err := tx.DecrementReblogsCount(ctx, statusID); err != nil {
			return fmt.Errorf("DecrementReblogsCount: %w", err)
		}
		fromAccount, err := tx.GetAccountByID(ctx, accountID)
		if err != nil {
			slog.WarnContext(ctx, "DeleteReblog: get account for event", slog.Any("error", err))
		}
		var originalAuthorID string
		if orig != nil {
			originalAuthorID = orig.AccountID
		}
		return events.EmitEvent(ctx, tx, domain.EventReblogRemoved, "status", reblog.ID, domain.ReblogRemovedPayload{
			AccountID:          accountID,
			ReblogStatusID:     reblog.ID,
			OriginalStatusID:   statusID,
			OriginalAuthorID:   originalAuthorID,
			FromAccount:        fromAccount,
			OriginalStatusAPID: originalStatusAPID,
		})
	}); err != nil {
		return fmt.Errorf("DeleteReblog: %w", err)
	}
	return nil
}

// CreateFavourite adds a favourite for the viewer on the status. Returns the status with favourited true.
func (svc *statusInteractionService) CreateFavourite(ctx context.Context, accountID, statusID string) (EnrichedStatus, error) {
	st, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil || st.DeletedAt != nil {
		return EnrichedStatus{}, fmt.Errorf("CreateFavourite: %w", domain.ErrNotFound)
	}
	viewerID := &accountID
	ok, err := svc.statusSvc.CanViewStatus(ctx, st, viewerID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("CreateFavourite: %w", err)
	}
	if !ok {
		return EnrichedStatus{}, fmt.Errorf("CreateFavourite: %w", domain.ErrNotFound)
	}
	statusAPID := st.APID
	if statusAPID == "" {
		statusAPID = st.URI
	}
	if statusAPID == "" {
		statusAPID = fmt.Sprintf("%s/statuses/%s", svc.instanceBaseURL, st.ID)
	}
	err = svc.store.WithTx(ctx, func(tx store.Store) error {
		_, txErr := tx.CreateFavourite(ctx, store.CreateFavouriteInput{
			ID:        uid.New(),
			AccountID: accountID,
			StatusID:  statusID,
			APID:      nil,
		})
		if txErr != nil {
			return fmt.Errorf("CreateFavourite: %w", txErr)
		}
		if txErr = tx.IncrementFavouritesCount(ctx, statusID); txErr != nil {
			return fmt.Errorf("IncrementFavouritesCount: %w", txErr)
		}
		// Best-effort enrichment: these accounts are known to exist, so failures
		// indicate a transient DB issue. We log and continue rather than failing
		// the TX over event payload enrichment.
		favouriterAccount, txErr := tx.GetAccountByID(ctx, accountID)
		if txErr != nil {
			slog.WarnContext(ctx, "CreateFavourite: get favouriter account for event", slog.Any("error", txErr))
		}
		statusAuthor, txErr := tx.GetAccountByID(ctx, st.AccountID)
		if txErr != nil {
			slog.WarnContext(ctx, "CreateFavourite: get status author for event", slog.Any("error", txErr))
		}
		return events.EmitEvent(ctx, tx, domain.EventFavouriteCreated, "favourite", statusID, domain.FavouriteCreatedPayload{
			AccountID:      accountID,
			StatusID:       statusID,
			StatusAuthorID: st.AccountID,
			FromAccount:    favouriterAccount,
			StatusAuthor:   statusAuthor,
			StatusAPID:     statusAPID,
		})
	})
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("CreateFavourite: %w", err)
	}
	out, err := svc.statusSvc.GetByIDEnriched(ctx, statusID, &accountID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("CreateFavourite: %w", err)
	}
	return out, nil
}

// DeleteFavourite removes the viewer's favourite. Returns the status with favourited false.
func (svc *statusInteractionService) DeleteFavourite(ctx context.Context, accountID, statusID string) (EnrichedStatus, error) {
	// Best-effort enrichment: the status enriches the delete event payload.
	// The unfavourite still proceeds if this lookup fails.
	st, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil {
		slog.WarnContext(ctx, "DeleteFavourite: get status for event", slog.Any("error", err), slog.String("status_id", statusID))
	}
	var statusAPID, statusAuthorID string
	if st != nil {
		statusAuthorID = st.AccountID
		statusAPID = st.APID
		if statusAPID == "" {
			statusAPID = st.URI
		}
		if statusAPID == "" {
			statusAPID = fmt.Sprintf("%s/statuses/%s", svc.instanceBaseURL, st.ID)
		}
	}
	err = svc.store.WithTx(ctx, func(tx store.Store) error {
		if err := tx.DeleteFavourite(ctx, accountID, statusID); err != nil && !errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("DeleteFavourite: %w", err)
		}
		if err := tx.DecrementFavouritesCount(ctx, statusID); err != nil && !errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("DecrementFavouritesCount: %w", err)
		}
		// Best-effort enrichment for event payload.
		fromAccount, txErr := tx.GetAccountByID(ctx, accountID)
		if txErr != nil {
			slog.WarnContext(ctx, "DeleteFavourite: get account for event", slog.Any("error", txErr))
		}
		var statusAuthor *domain.Account
		if statusAuthorID != "" {
			statusAuthor, txErr = tx.GetAccountByID(ctx, statusAuthorID)
			if txErr != nil {
				slog.WarnContext(ctx, "DeleteFavourite: get status author for event", slog.Any("error", txErr))
			}
		}
		return events.EmitEvent(ctx, tx, domain.EventFavouriteRemoved, "favourite", statusID, domain.FavouriteRemovedPayload{
			AccountID:      accountID,
			StatusID:       statusID,
			StatusAuthorID: statusAuthorID,
			FromAccount:    fromAccount,
			StatusAuthor:   statusAuthor,
			StatusAPID:     statusAPID,
		})
	})
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("DeleteFavourite: %w", err)
	}
	out, err := svc.statusSvc.GetByIDEnriched(ctx, statusID, &accountID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("DeleteFavourite: %w", err)
	}
	return out, nil
}

// Bookmark adds the status to the account's bookmarks. Idempotent if already bookmarked.
func (svc *statusInteractionService) Bookmark(ctx context.Context, accountID, statusID string) (EnrichedStatus, error) {
	st, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil || st.DeletedAt != nil {
		return EnrichedStatus{}, fmt.Errorf("Bookmark: %w", domain.ErrNotFound)
	}
	viewerID := &accountID
	ok, err := svc.statusSvc.CanViewStatus(ctx, st, viewerID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("Bookmark: %w", err)
	}
	if !ok {
		return EnrichedStatus{}, fmt.Errorf("Bookmark: %w", domain.ErrNotFound)
	}
	err = svc.store.CreateBookmark(ctx, store.CreateBookmarkInput{
		ID:        uid.New(),
		AccountID: accountID,
		StatusID:  statusID,
	})
	if err != nil && !errors.Is(err, domain.ErrConflict) {
		return EnrichedStatus{}, fmt.Errorf("CreateBookmark: %w", err)
	}
	out, err := svc.statusSvc.GetByIDEnriched(ctx, statusID, &accountID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("Bookmark: %w", err)
	}
	return out, nil
}

// Unbookmark removes the status from the account's bookmarks.
func (svc *statusInteractionService) Unbookmark(ctx context.Context, accountID, statusID string) (EnrichedStatus, error) {
	if err := svc.store.DeleteBookmark(ctx, accountID, statusID); err != nil && !errors.Is(err, domain.ErrNotFound) {
		return EnrichedStatus{}, fmt.Errorf("Unbookmark: %w", err)
	}
	out, err := svc.statusSvc.GetByIDEnriched(ctx, statusID, &accountID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("Unbookmark: %w", err)
	}
	return out, nil
}

const maxPinsPerAccount = 5

// Pin pins the status to the account's profile. Only local public/unlisted statuses can be pinned.
func (svc *statusInteractionService) Pin(ctx context.Context, accountID, statusID string) (EnrichedStatus, error) {
	st, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return EnrichedStatus{}, fmt.Errorf("Pin: %w", domain.ErrNotFound)
		}
		return EnrichedStatus{}, fmt.Errorf("Pin: %w", err)
	}
	if err := requireLocal(st.Local, "Pin"); err != nil {
		return EnrichedStatus{}, err
	}
	if st.AccountID != accountID {
		return EnrichedStatus{}, fmt.Errorf("Pin: %w", domain.ErrForbidden)
	}
	if st.Visibility != domain.VisibilityPublic && st.Visibility != domain.VisibilityUnlisted {
		return EnrichedStatus{}, fmt.Errorf("Pin: %w", domain.ErrUnprocessable)
	}
	if st.ReblogOfID != nil && *st.ReblogOfID != "" {
		return EnrichedStatus{}, fmt.Errorf("Pin: %w", domain.ErrUnprocessable)
	}
	count, err := svc.store.CountAccountPins(ctx, accountID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("Pin: %w", err)
	}
	if count >= maxPinsPerAccount {
		return EnrichedStatus{}, fmt.Errorf("Pin: %w", domain.ErrUnprocessable)
	}
	if err := svc.store.CreateAccountPin(ctx, accountID, statusID); err != nil {
		return EnrichedStatus{}, fmt.Errorf("Pin: %w", err)
	}
	out, err := svc.statusSvc.GetByIDEnriched(ctx, statusID, &accountID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("Pin: %w", err)
	}
	return out, nil
}

// Unpin unpins the status from the account's profile.
func (svc *statusInteractionService) Unpin(ctx context.Context, accountID, statusID string) (EnrichedStatus, error) {
	st, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return EnrichedStatus{}, fmt.Errorf("Unpin: %w", domain.ErrNotFound)
		}
		return EnrichedStatus{}, fmt.Errorf("Unpin: %w", err)
	}
	if err := requireLocal(st.Local, "Unpin"); err != nil {
		return EnrichedStatus{}, err
	}
	if st.AccountID != accountID {
		return EnrichedStatus{}, fmt.Errorf("Unpin: %w", domain.ErrForbidden)
	}
	if err := svc.store.DeleteAccountPin(ctx, accountID, statusID); err != nil {
		return EnrichedStatus{}, fmt.Errorf("Unpin: %w", err)
	}
	out, err := svc.statusSvc.GetByIDEnriched(ctx, statusID, &accountID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("Unpin: %w", err)
	}
	return out, nil
}

// RecordVote records the viewer's vote on a poll (replacing any existing vote). Returns the updated EnrichedPoll.
func (svc *statusInteractionService) RecordVote(ctx context.Context, pollID, accountID string, optionIndices []int) (*EnrichedPoll, error) {
	poll, err := svc.store.GetPollByID(ctx, pollID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("RecordVote: %w", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("RecordVote: %w", err)
	}
	if poll.ExpiresAt != nil && poll.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("RecordVote: %w", domain.ErrUnprocessable)
	}
	st, err := svc.store.GetStatusByID(ctx, poll.StatusID)
	if err != nil {
		return nil, fmt.Errorf("RecordVote: %w", err)
	}
	viewerID := &accountID
	ok, err := svc.statusSvc.CanViewStatus(ctx, st, viewerID)
	if err != nil {
		return nil, fmt.Errorf("RecordVote: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("RecordVote: %w", domain.ErrNotFound)
	}
	opts, err := svc.store.ListPollOptions(ctx, pollID)
	if err != nil {
		return nil, fmt.Errorf("RecordVote: %w", err)
	}
	if len(optionIndices) == 0 {
		return nil, fmt.Errorf("RecordVote: %w", domain.ErrValidation)
	}
	if !poll.Multiple && len(optionIndices) > 1 {
		return nil, fmt.Errorf("RecordVote: %w", domain.ErrValidation)
	}
	for _, idx := range optionIndices {
		if idx < 0 || idx >= len(opts) {
			return nil, fmt.Errorf("RecordVote: %w", domain.ErrValidation)
		}
	}
	if err := svc.store.DeletePollVotesByAccount(ctx, pollID, accountID); err != nil {
		return nil, fmt.Errorf("RecordVote: %w", err)
	}
	for _, idx := range optionIndices {
		optionID := opts[idx].ID
		voteID := uid.New()
		if err := svc.store.CreatePollVote(ctx, voteID, pollID, accountID, optionID); err != nil {
			return nil, fmt.Errorf("RecordVote: %w", err)
		}
	}
	poll2, err := svc.statusSvc.GetPoll(ctx, pollID, &accountID)
	if err != nil {
		return nil, fmt.Errorf("RecordVote: %w", err)
	}
	return poll2, nil
}
