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

// CreateRemoteStatusInput is the input for creating a remote status (e.g. from federation).
type CreateRemoteStatusInput struct {
	AccountID      string
	URI            string
	Text           *string
	Content        *string
	ContentWarning *string
	Visibility     string
	Language       *string
	InReplyToID    *string
	Attachments    []CreateRemoteMediaInput // optional; created and attached after status is created (max 4)
	APID           string
	Sensitive      bool
	HashtagNames   []string   // hashtag names (without '#') from inbound Note tags
	MentionIRIs    []string   // actor IRIs of mentioned accounts from inbound Note tags
	PublishedAt    *time.Time // original publish time from AP Note; nil falls back to NOW()
}

// CreateRemoteReblogInput is the input for creating a remote reblog (e.g. from federation Announce).
type CreateRemoteReblogInput struct {
	AccountID        string
	ActivityAPID     string
	ObjectStatusAPID string
}

// UpdateRemoteStatusInput is the input for updating a remote status (e.g. from federation Update{Note}).
type UpdateRemoteStatusInput struct {
	Text           *string
	Content        *string
	ContentWarning *string
	Sensitive      bool
}

// RemoteStatusWriteService handles remote status write operations (create, update, delete, reblog, favourite)
// from federation. It does not emit outbound federation events.
type RemoteStatusWriteService interface {
	CreateRemote(ctx context.Context, in CreateRemoteStatusInput) (*domain.Status, error)
	UpdateRemote(ctx context.Context, statusID string, st *domain.Status, in UpdateRemoteStatusInput) error
	DeleteRemote(ctx context.Context, statusID string) error
	CreateRemoteReblog(ctx context.Context, in CreateRemoteReblogInput) (*domain.Status, error)
	DeleteRemoteReblog(ctx context.Context, accountID, statusID string) error
	CreateRemoteFavourite(ctx context.Context, accountID, statusID string, apID *string) (*domain.Favourite, error)
	DeleteRemoteFavourite(ctx context.Context, accountID, statusID string) error
}

type remoteStatusWriteService struct {
	store           store.Store
	conversationSvc ConversationService
	media           MediaService
	instanceBaseURL string
}

// NewRemoteStatusWriteService returns a RemoteStatusWriteService.
func NewRemoteStatusWriteService(s store.Store, c ConversationService, m MediaService, instanceBaseURL string) RemoteStatusWriteService {
	base := strings.TrimSuffix(instanceBaseURL, "/")
	return &remoteStatusWriteService{
		store:           s,
		conversationSvc: c,
		media:           m,
		instanceBaseURL: base,
	}
}

func (svc *remoteStatusWriteService) CreateRemote(ctx context.Context, in CreateRemoteStatusInput) (*domain.Status, error) {
	// Pre-generate the ID so it's available to best-effort operations after the transaction.
	// Use the original publish time for the ULID so backfilled statuses sort chronologically.
	statusID := uid.New()
	if in.PublishedAt != nil {
		statusID = uid.NewWithTime(*in.PublishedAt)
	}
	var st *domain.Status
	err := svc.store.WithTx(ctx, func(tx store.Store) error {
		var txErr error
		st, txErr = tx.CreateStatus(ctx, store.CreateStatusInput{
			ID:                  statusID,
			URI:                 in.URI,
			AccountID:           in.AccountID,
			Text:                in.Text,
			Content:             in.Content,
			ContentWarning:      in.ContentWarning,
			Visibility:          in.Visibility,
			Language:            in.Language,
			InReplyToID:         in.InReplyToID,
			APID:                in.APID,
			Sensitive:           in.Sensitive,
			Local:               false,
			QuoteApprovalPolicy: domain.QuotePolicyPublic,
			CreatedAt:           in.PublishedAt,
		})
		if txErr != nil {
			return fmt.Errorf("CreateStatus: %w", txErr)
		}
		author, txErr := tx.GetAccountByID(ctx, in.AccountID)
		if txErr != nil {
			return fmt.Errorf("GetAccountByID: %w", txErr)
		}
		return events.EmitEvent(ctx, tx, domain.EventStatusCreatedRemote, "status", st.ID, domain.StatusCreatedPayload{
			Status: st,
			Author: author,
			Local:  st.IsLocal(),
		})
	})
	if err != nil {
		return nil, fmt.Errorf("CreateRemote: %w", err)
	}
	// Best-effort post-commit operations: partial failures leave the status intact.

	// Create attachments
	attachments := in.Attachments
	if len(attachments) > 4 {
		attachments = attachments[:4]
	}
	for _, att := range attachments {
		m, mediaErr := svc.media.CreateRemote(ctx, att)
		if mediaErr != nil {
			slog.WarnContext(ctx, "CreateRemote: create media failed", slog.String("url", att.RemoteURL), slog.Any("error", mediaErr))
			continue
		}
		if attErr := svc.store.AttachMediaToStatus(ctx, m.ID, st.ID, in.AccountID); attErr != nil {
			slog.WarnContext(ctx, "CreateRemote: attach media failed", slog.String("media_id", m.ID), slog.Any("error", attErr))
		}
	}

	// Increment replies count
	if in.InReplyToID != nil && *in.InReplyToID != "" {
		if incErr := svc.store.IncrementRepliesCount(ctx, *in.InReplyToID); incErr != nil {
			slog.WarnContext(ctx, "CreateRemote: increment replies count failed", slog.String("parent_id", *in.InReplyToID), slog.Any("error", incErr))
		}
	}

	// Store hashtags and mentions
	svc.storeRemoteHashtags(ctx, st.ID, in.HashtagNames)
	svc.storeRemoteMentions(ctx, st.ID, in.MentionIRIs)

	// Update conversation for direct statuses
	if in.Visibility == domain.VisibilityDirect {
		mentionedIDs, err := svc.store.GetStatusMentionAccountIDs(ctx, st.ID)
		if err != nil {
			slog.WarnContext(ctx, "CreateRemote: GetStatusMentionAccountIDs failed", slog.String("status_id", st.ID), slog.Any("error", err))
		}
		err = svc.conversationSvc.UpdateForDirectStatus(ctx, st, st.AccountID, mentionedIDs)
		if err != nil {
			slog.WarnContext(ctx, "CreateRemote: conversation update failed after direct status from inbox", slog.String("status_id", st.ID), slog.Any("error", err))
		}
	}
	return st, nil
}

func (svc *remoteStatusWriteService) storeRemoteHashtags(ctx context.Context, statusID string, names []string) {
	if len(names) == 0 {
		return
	}
	var hashtagIDs []string
	for _, name := range names {
		ht, err := svc.store.GetOrCreateHashtag(ctx, name)
		if err != nil {
			slog.WarnContext(ctx, "CreateRemote: GetOrCreateHashtag failed", slog.String("tag", name), slog.Any("error", err))
			continue
		}
		hashtagIDs = append(hashtagIDs, ht.ID)
	}
	if len(hashtagIDs) > 0 {
		if err := svc.store.AttachHashtagsToStatus(ctx, statusID, hashtagIDs); err != nil {
			slog.WarnContext(ctx, "CreateRemote: AttachHashtagsToStatus failed", slog.String("status_id", statusID), slog.Any("error", err))
		}
	}
}

func (svc *remoteStatusWriteService) storeRemoteMentions(ctx context.Context, statusID string, mentionIRIs []string) {
	if len(mentionIRIs) == 0 {
		return
	}
	for _, iri := range mentionIRIs {
		acc, err := svc.store.GetAccountByAPID(ctx, iri)
		if err != nil {
			continue
		}
		if err := svc.store.CreateStatusMention(ctx, statusID, acc.ID); err != nil {
			slog.WarnContext(ctx, "CreateRemote: CreateStatusMention failed", slog.String("status_id", statusID), slog.String("account_id", acc.ID), slog.Any("error", err))
		}
	}
}

func (svc *remoteStatusWriteService) DeleteRemote(ctx context.Context, statusID string) error {
	st, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil {
		return fmt.Errorf("DeleteRemote(%s): %w", statusID, err)
	}
	if err := requireRemote(st.Local, "DeleteRemote"); err != nil {
		return err
	}
	// Best-effort enrichment: hashtag names and mention IDs enrich the delete event payload.
	// The delete still proceeds if these lookups fail — the status is removed regardless.
	var hashtagNames []string
	tags, err := svc.store.GetStatusHashtags(ctx, statusID)
	if err != nil {
		slog.WarnContext(ctx, "DeleteRemote: get hashtags for event", slog.Any("error", err), slog.String("status_id", statusID))
	}
	for _, t := range tags {
		hashtagNames = append(hashtagNames, t.Name)
	}
	var mentionedAccountIDs []string
	if st.Visibility == domain.VisibilityDirect {
		mentionedAccountIDs, err = svc.store.GetStatusMentionAccountIDs(ctx, statusID)
		if err != nil {
			slog.WarnContext(ctx, "DeleteRemote: get mention account IDs for event", slog.Any("error", err), slog.String("status_id", statusID))
		}
	}
	err = svc.store.WithTx(ctx, func(tx store.Store) error {
		if err := tx.SoftDeleteStatus(ctx, statusID); err != nil {
			return fmt.Errorf("SoftDeleteStatus: %w", err)
		}
		return events.EmitEvent(ctx, tx, domain.EventStatusDeletedRemote, "status", statusID, domain.StatusDeletedPayload{
			StatusID:            st.ID,
			AccountID:           st.AccountID,
			Visibility:          st.Visibility,
			Local:               st.Local,
			APID:                st.APID,
			URI:                 st.URI,
			HashtagNames:        hashtagNames,
			MentionedAccountIDs: mentionedAccountIDs,
		})
	})
	if err != nil {
		return fmt.Errorf("DeleteRemote(%s): %w", statusID, err)
	}
	return nil
}

func (svc *remoteStatusWriteService) CreateRemoteReblog(ctx context.Context, in CreateRemoteReblogInput) (*domain.Status, error) {
	original, err := svc.store.GetStatusByAPID(ctx, in.ObjectStatusAPID)
	if err != nil {
		return nil, fmt.Errorf("CreateRemoteReblog GetStatusByAPID: %w", err)
	}
	reblogOfID := original.ID
	reblogID := uid.New()
	originalStatusAPID := original.APID
	if originalStatusAPID == "" {
		originalStatusAPID = original.URI
	}
	if originalStatusAPID == "" {
		originalStatusAPID = fmt.Sprintf("%s/statuses/%s", svc.instanceBaseURL, original.ID)
	}
	var st *domain.Status
	if err := svc.store.WithTx(ctx, func(tx store.Store) error {
		var txErr error
		st, txErr = tx.CreateStatus(ctx, store.CreateStatusInput{
			ID:                  reblogID,
			URI:                 in.ActivityAPID,
			AccountID:           in.AccountID,
			Visibility:          domain.VisibilityPublic,
			ReblogOfID:          &reblogOfID,
			APID:                in.ActivityAPID,
			Local:               false,
			QuoteApprovalPolicy: domain.QuotePolicyPublic,
		})
		if txErr != nil {
			return fmt.Errorf("CreateRemoteReblog CreateStatus: %w", txErr)
		}
		if txErr = tx.IncrementReblogsCount(ctx, original.ID); txErr != nil {
			return fmt.Errorf("CreateRemoteReblog IncrementReblogsCount: %w", txErr)
		}
		// Best-effort enrichment: these accounts are known to exist (we just used their IDs
		// above), so a failure here indicates a transient DB issue rather than missing data.
		// We log and continue rather than failing the TX over event payload enrichment.
		fromAccount, txErr := tx.GetAccountByID(ctx, in.AccountID)
		if txErr != nil {
			slog.WarnContext(ctx, "CreateRemoteReblog: get account for event", slog.Any("error", txErr), slog.String("account_id", in.AccountID))
		}
		originalAuthor, txErr := tx.GetAccountByID(ctx, original.AccountID)
		if txErr != nil {
			slog.WarnContext(ctx, "CreateRemoteReblog: get original author for event", slog.Any("error", txErr), slog.String("account_id", original.AccountID))
		}
		localFromAccount := fromAccount != nil && fromAccount.IsLocal()
		return events.EmitEvent(ctx, tx, domain.EventReblogCreated, "status", reblogID, domain.ReblogCreatedPayload{
			AccountID:          in.AccountID,
			ReblogStatusID:     reblogID,
			OriginalStatusID:   original.ID,
			OriginalAuthorID:   original.AccountID,
			FromAccount:        fromAccount,
			OriginalAuthor:     originalAuthor,
			OriginalStatusAPID: originalStatusAPID,
			Local:              localFromAccount,
		})
	}); err != nil {
		return nil, fmt.Errorf("CreateRemoteReblog: %w", err)
	}
	return st, nil
}

func (svc *remoteStatusWriteService) UpdateRemote(ctx context.Context, statusID string, st *domain.Status, in UpdateRemoteStatusInput) error {
	if err := requireRemote(st.Local, "UpdateRemote"); err != nil {
		return err
	}
	if err := svc.store.CreateStatusEdit(ctx, store.CreateStatusEditInput{
		ID:             uid.New(),
		StatusID:       statusID,
		AccountID:      st.AccountID,
		Text:           st.Text,
		Content:        st.Content,
		ContentWarning: st.ContentWarning,
		Sensitive:      st.Sensitive,
	}); err != nil {
		return fmt.Errorf("UpdateRemote CreateStatusEdit: %w", err)
	}
	if err := svc.store.UpdateStatus(ctx, store.UpdateStatusInput{
		ID:             statusID,
		Text:           in.Text,
		Content:        in.Content,
		ContentWarning: in.ContentWarning,
		Sensitive:      in.Sensitive,
	}); err != nil {
		return fmt.Errorf("UpdateRemote UpdateStatus: %w", err)
	}
	updated, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil {
		slog.WarnContext(ctx, "UpdateRemote: GetStatusByID for event emission", slog.String("status_id", statusID), slog.Any("error", err))
		return nil
	}
	author, err := svc.store.GetAccountByID(ctx, st.AccountID)
	if err != nil {
		slog.WarnContext(ctx, "UpdateRemote: GetAccountByID for event emission", slog.String("account_id", st.AccountID), slog.Any("error", err))
		return nil
	}
	if emitErr := events.EmitEvent(ctx, svc.store, domain.EventStatusUpdatedRemote, "status", statusID, domain.StatusUpdatedPayload{
		Status: updated,
		Author: author,
		Local:  updated.IsLocal(),
	}); emitErr != nil {
		slog.WarnContext(ctx, "UpdateRemote: EmitEvent failed", slog.String("status_id", statusID), slog.Any("error", emitErr))
	}
	return nil
}

func (svc *remoteStatusWriteService) CreateRemoteFavourite(ctx context.Context, accountID, statusID string, apID *string) (*domain.Favourite, error) {
	var fav *domain.Favourite
	if err := svc.store.WithTx(ctx, func(tx store.Store) error {
		var txErr error
		fav, txErr = tx.CreateFavourite(ctx, store.CreateFavouriteInput{
			ID:        uid.New(),
			AccountID: accountID,
			StatusID:  statusID,
			APID:      apID,
		})
		if txErr != nil {
			return fmt.Errorf("CreateRemoteFavourite: %w", txErr)
		}
		if txErr = tx.IncrementFavouritesCount(ctx, statusID); txErr != nil {
			return fmt.Errorf("CreateRemoteFavourite IncrementFavouritesCount: %w", txErr)
		}
		// Best-effort enrichment: the status and accounts are known to exist (the inbox
		// resolved them before calling us), so failures indicate a transient DB issue
		// rather than missing data. We log and continue rather than failing the TX over
		// event payload enrichment.
		st, txErr := tx.GetStatusByID(ctx, statusID)
		if txErr != nil {
			slog.WarnContext(ctx, "CreateRemoteFavourite: get status for event", slog.Any("error", txErr), slog.String("status_id", statusID))
		}
		var statusAuthorID, statusAPID string
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
		fromAccount, txErr := tx.GetAccountByID(ctx, accountID)
		if txErr != nil {
			slog.WarnContext(ctx, "CreateRemoteFavourite: get account for event", slog.Any("error", txErr), slog.String("account_id", accountID))
		}
		var statusAuthor *domain.Account
		if statusAuthorID != "" {
			statusAuthor, txErr = tx.GetAccountByID(ctx, statusAuthorID)
			if txErr != nil {
				slog.WarnContext(ctx, "CreateRemoteFavourite: get status author for event", slog.Any("error", txErr), slog.String("account_id", statusAuthorID))
			}
		}
		localFromAccount := fromAccount != nil && fromAccount.IsLocal()
		return events.EmitEvent(ctx, tx, domain.EventFavouriteCreated, "favourite", statusID, domain.FavouriteCreatedPayload{
			AccountID:      accountID,
			StatusID:       statusID,
			StatusAuthorID: statusAuthorID,
			FromAccount:    fromAccount,
			StatusAuthor:   statusAuthor,
			StatusAPID:     statusAPID,
			Local:          localFromAccount,
		})
	}); err != nil {
		return nil, fmt.Errorf("CreateRemoteFavourite: %w", err)
	}
	return fav, nil
}

func (svc *remoteStatusWriteService) DeleteRemoteReblog(ctx context.Context, accountID, statusID string) error {
	reblog, err := svc.store.GetReblogByAccountAndTarget(ctx, accountID, statusID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("DeleteRemoteReblog: %w", err)
	}
	if reblog == nil {
		return nil
	}
	orig, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil {
		slog.WarnContext(ctx, "DeleteRemoteReblog: get original status", slog.Any("error", err), slog.String("status_id", statusID))
	}
	var originalAuthorID, originalStatusAPID string
	if orig != nil {
		originalAuthorID = orig.AccountID
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
			return fmt.Errorf("DeleteRemoteReblog SoftDeleteStatus: %w", err)
		}
		if err := tx.DecrementReblogsCount(ctx, statusID); err != nil {
			return fmt.Errorf("DeleteRemoteReblog DecrementReblogsCount: %w", err)
		}
		fromAccount, err := tx.GetAccountByID(ctx, accountID)
		if err != nil {
			slog.WarnContext(ctx, "DeleteRemoteReblog: get account", slog.Any("error", err), slog.String("account_id", accountID))
		}
		localFromAccount := fromAccount != nil && fromAccount.IsLocal()
		return events.EmitEvent(ctx, tx, domain.EventReblogRemoved, "status", reblog.ID, domain.ReblogRemovedPayload{
			AccountID:          accountID,
			ReblogStatusID:     reblog.ID,
			OriginalStatusID:   statusID,
			OriginalAuthorID:   originalAuthorID,
			FromAccount:        fromAccount,
			OriginalStatusAPID: originalStatusAPID,
			Local:              localFromAccount,
		})
	}); err != nil {
		return fmt.Errorf("DeleteRemoteReblog: %w", err)
	}
	return nil
}

func (svc *remoteStatusWriteService) DeleteRemoteFavourite(ctx context.Context, accountID, statusID string) error {
	st, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil {
		slog.WarnContext(ctx, "DeleteRemoteFavourite: get status", slog.Any("error", err), slog.String("status_id", statusID))
	}
	var statusAuthorID, statusAPID string
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
	if err := svc.store.WithTx(ctx, func(tx store.Store) error {
		if err := tx.DeleteFavourite(ctx, accountID, statusID); err != nil {
			return fmt.Errorf("DeleteRemoteFavourite: %w", err)
		}
		if err := tx.DecrementFavouritesCount(ctx, statusID); err != nil {
			return fmt.Errorf("DeleteRemoteFavourite DecrementFavouritesCount: %w", err)
		}
		// Best-effort enrichment: these accounts are known to exist, so failures indicate
		// a transient DB issue. We log and continue rather than failing the TX over
		// event payload enrichment.
		fromAccount, txErr := tx.GetAccountByID(ctx, accountID)
		if txErr != nil {
			slog.WarnContext(ctx, "DeleteRemoteFavourite: get account for event", slog.Any("error", txErr), slog.String("account_id", accountID))
		}
		var statusAuthor *domain.Account
		if statusAuthorID != "" {
			statusAuthor, txErr = tx.GetAccountByID(ctx, statusAuthorID)
			if txErr != nil {
				slog.WarnContext(ctx, "DeleteRemoteFavourite: get status author for event", slog.Any("error", txErr), slog.String("account_id", statusAuthorID))
			}
		}
		localFromAccount := fromAccount != nil && fromAccount.IsLocal()
		return events.EmitEvent(ctx, tx, domain.EventFavouriteRemoved, "favourite", statusID, domain.FavouriteRemovedPayload{
			AccountID:      accountID,
			StatusID:       statusID,
			StatusAuthorID: statusAuthorID,
			FromAccount:    fromAccount,
			StatusAuthor:   statusAuthor,
			StatusAPID:     statusAPID,
			Local:          localFromAccount,
		})
	}); err != nil {
		return fmt.Errorf("DeleteRemoteFavourite: %w", err)
	}
	return nil
}
