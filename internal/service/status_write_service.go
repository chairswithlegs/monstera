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

// CreateStatusInput is the input for creating a status with plain text (content is rendered in-service).
// When DefaultVisibility or DefaultQuotePolicy are empty, the service looks up the account's user for defaults.
type CreateStatusInput struct {
	AccountID           string
	Username            string
	Text                string
	Visibility          string
	DefaultVisibility   string // used when Visibility is empty or invalid; when empty, looked up from user
	DefaultQuotePolicy  string // public | followers | nobody; when empty, looked up from user
	ContentWarning      string
	Language            string
	Sensitive           bool
	InReplyToID         *string     // optional parent status ID for replies
	QuotedStatusID      *string     // optional status ID being quoted
	QuoteApprovalPolicy string      // public | followers | nobody (for the new status); service applies private/direct -> nobody
	MediaIDs            []string    // optional media attachment IDs (max 4; service caps at 4)
	Poll                *PollInput  // optional; when set, status is created with an attached poll
	PollLimits          *PollLimits // required when Poll is set (from instance configuration)
}

// PollInput is the poll payload when creating a status with a poll.
type PollInput struct {
	Options          []string
	ExpiresInSeconds int
	Multiple         bool
}

// PollLimits is instance configuration for poll validation (e.g. from GET /api/v2/instance).
type PollLimits struct {
	MaxOptions    int
	MinExpiration int
	MaxExpiration int
}

// UpdateStatusInput is the input for updating a local status.
type UpdateStatusInput struct {
	AccountID   string
	StatusID    string
	Text        string
	SpoilerText string
	Sensitive   bool
}

// StatusWriteService orchestrates status write operations (create, delete, update) and quote operations.
// Interaction operations (reblog, favourite, bookmark, pin, poll vote) live in StatusInteractionService.
// It depends on StatusService for reads and visibility checks to avoid circular dependency with ConversationService.
type StatusWriteService interface {
	Create(ctx context.Context, in CreateStatusInput) (EnrichedStatus, error)
	Update(ctx context.Context, in UpdateStatusInput) (EnrichedStatus, error)
	Delete(ctx context.Context, id, accountID string) error
	UpdateQuoteApprovalPolicy(ctx context.Context, accountID, statusID, policy string) error
	RevokeQuote(ctx context.Context, accountID, quotedStatusID, quotingStatusID string) error
}

type statusWriteService struct {
	store           store.Store
	statusSvc       StatusService
	conversationSvc ConversationService
	instanceBaseURL string
	instanceDomain  string
	maxStatusChars  int
}

// NewStatusWriteService returns a StatusWriteService.
func NewStatusWriteService(
	s store.Store,
	statusSvc StatusService,
	conversationSvc ConversationService,
	instanceBaseURL, instanceDomain string,
	maxStatusChars int,
) StatusWriteService {
	base := strings.TrimSuffix(instanceBaseURL, "/")
	return &statusWriteService{
		store:           s,
		statusSvc:       statusSvc,
		conversationSvc: conversationSvc,
		instanceBaseURL: base,
		instanceDomain:  instanceDomain,
		maxStatusChars:  maxStatusChars,
	}
}

func requireLocal(local bool, method string) error {
	if !local {
		return fmt.Errorf("%s: %w", method, domain.ErrForbidden)
	}
	return nil
}

func (svc *statusWriteService) buildMentionResolver(ctx context.Context) mentionResolver {
	return func(username string, domain *string) *domain.Account {
		if domain == nil || *domain == "" {
			a, _ := svc.store.GetLocalAccountByUsername(ctx, username)
			return a
		}
		a, _ := svc.store.GetRemoteAccountByUsername(ctx, username, domain)
		return a
	}
}

type createStatusWithContentInput struct {
	AccountID           string
	StatusID            string
	StatusURI           string
	Visibility          string
	Text                string
	Content             string
	ContentWarning      string
	Language            string
	Sensitive           bool
	Rendered            renderResult
	InReplyToID         *string
	InReplyToAccountID  *string
	QuotedStatusID      *string
	QuoteApprovalPolicy string
	QuotedAuthorID      *string
}

func createStatusWithContentTx(ctx context.Context, tx store.Store, in createStatusWithContentInput) (*domain.Status, error) {
	var textPtr, contentPtr *string
	if in.Text != "" {
		t, c := in.Text, in.Content
		textPtr = &t
		contentPtr = &c
	}
	cw := in.ContentWarning
	lang := in.Language
	st, err := tx.CreateStatus(ctx, store.CreateStatusInput{
		ID:                  in.StatusID,
		URI:                 in.StatusURI,
		AccountID:           in.AccountID,
		Text:                textPtr,
		Content:             contentPtr,
		ContentWarning:      &cw,
		Visibility:          in.Visibility,
		Language:            &lang,
		InReplyToID:         in.InReplyToID,
		InReplyToAccountID:  in.InReplyToAccountID,
		QuotedStatusID:      in.QuotedStatusID,
		QuoteApprovalPolicy: in.QuoteApprovalPolicy,
		Sensitive:           in.Sensitive,
		Local:               true,
		APID:                in.StatusURI,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateStatus: %w", err)
	}
	if in.QuotedStatusID != nil && *in.QuotedStatusID != "" {
		if err := tx.CreateQuoteApproval(ctx, in.StatusID, *in.QuotedStatusID); err != nil {
			return nil, fmt.Errorf("CreateQuoteApproval: %w", err)
		}
		if err := tx.IncrementQuotesCount(ctx, *in.QuotedStatusID); err != nil {
			return nil, fmt.Errorf("IncrementQuotesCount: %w", err)
		}
		if in.QuotedAuthorID != nil && *in.QuotedAuthorID != in.AccountID {
			notifID := uid.New()
			statusID := in.StatusID
			_, err = tx.CreateNotification(ctx, store.CreateNotificationInput{
				ID:        notifID,
				AccountID: *in.QuotedAuthorID,
				FromID:    in.AccountID,
				Type:      domain.NotificationTypeQuote,
				StatusID:  &statusID,
			})
			if err != nil {
				return nil, fmt.Errorf("CreateNotification quote: %w", err)
			}
		}
	}
	for _, m := range in.Rendered.Mentions {
		if err := tx.CreateStatusMention(ctx, in.StatusID, m.AccountID); err != nil {
			return nil, fmt.Errorf("CreateStatusMention: %w", err)
		}
	}
	var hashtagIDs []string
	for _, tagName := range in.Rendered.Tags {
		ht, err := tx.GetOrCreateHashtag(ctx, tagName)
		if err != nil {
			return nil, fmt.Errorf("GetOrCreateHashtag(%s): %w", tagName, err)
		}
		hashtagIDs = append(hashtagIDs, ht.ID)
	}
	if len(hashtagIDs) > 0 {
		if err := tx.AttachHashtagsToStatus(ctx, in.StatusID, hashtagIDs); err != nil {
			return nil, fmt.Errorf("AttachHashtagsToStatus: %w", err)
		}
	}
	if err := tx.IncrementStatusesCount(ctx, in.AccountID); err != nil {
		return nil, fmt.Errorf("IncrementStatusesCount: %w", err)
	}
	if err := tx.UpdateAccountLastStatusAt(ctx, in.AccountID); err != nil {
		return nil, fmt.Errorf("UpdateAccountLastStatusAt: %w", err)
	}
	return st, nil
}

// Create creates a status from plain text: validates, renders content (mentions, hashtags),
// persists status with mentions, hashtags, and mention notifications in one transaction.
// Supports Mastodon-style quotes (quoted_status_id, quote approval, quote notification) when QuotedStatusID is set.
// Returns enriched status with author, mentions, tags, and media. Federates the new status.
func (svc *statusWriteService) Create(ctx context.Context, in CreateStatusInput) (EnrichedStatus, error) {
	defaultVisibility := in.DefaultVisibility
	defaultQuotePolicy := in.DefaultQuotePolicy
	if defaultVisibility == "" || defaultQuotePolicy == "" {
		if user, err := svc.store.GetUserByAccountID(ctx, in.AccountID); err == nil {
			if defaultVisibility == "" {
				defaultVisibility = user.DefaultPrivacy
			}
			if defaultQuotePolicy == "" && user.DefaultQuotePolicy != "" {
				defaultQuotePolicy = user.DefaultQuotePolicy
			}
		}
		if defaultQuotePolicy == "" {
			defaultQuotePolicy = domain.QuotePolicyPublic
		}
	}
	mediaIDs := in.MediaIDs
	if len(mediaIDs) > 4 {
		mediaIDs = mediaIDs[:4]
	}
	text := strings.TrimSpace(in.Text)
	if text == "" && len(mediaIDs) == 0 {
		return EnrichedStatus{}, fmt.Errorf("Create: %w", domain.ErrValidation)
	}
	visibility := resolveVisibilityService(in.Visibility, defaultVisibility)
	if text != "" && countStatusCharacters(text) > svc.maxStatusChars {
		return EnrichedStatus{}, fmt.Errorf("Create: %w", domain.ErrValidation)
	}
	var inReplyToAccountID *string
	var parentAPID string
	if in.InReplyToID != nil && *in.InReplyToID != "" {
		parent, err := svc.store.GetStatusByID(ctx, *in.InReplyToID)
		if err != nil {
			return EnrichedStatus{}, fmt.Errorf("Create in_reply_to: %w", err)
		}
		if parent.DeletedAt != nil {
			return EnrichedStatus{}, fmt.Errorf("Create in_reply_to: %w", domain.ErrNotFound)
		}
		inReplyToAccountID = &parent.AccountID
		parentAPID = parent.APID
	}
	for _, mid := range mediaIDs {
		att, err := svc.store.GetMediaAttachment(ctx, mid)
		if err != nil {
			return EnrichedStatus{}, fmt.Errorf("Create media %s: %w", mid, err)
		}
		if att.AccountID != in.AccountID {
			return EnrichedStatus{}, fmt.Errorf("Create media: %w", domain.ErrForbidden)
		}
	}
	var quotedAuthorID *string
	if in.QuotedStatusID != nil && *in.QuotedStatusID != "" {
		if (in.Poll != nil && len(in.Poll.Options) > 0) || len(mediaIDs) > 0 {
			return EnrichedStatus{}, fmt.Errorf("Create: %w", domain.ErrValidation)
		}
		quoted, err := svc.store.GetStatusByID(ctx, *in.QuotedStatusID)
		if err != nil {
			return EnrichedStatus{}, fmt.Errorf("Create quoted_status_id: %w", err)
		}
		if quoted.DeletedAt != nil {
			return EnrichedStatus{}, fmt.Errorf("Create quoted_status_id: %w", domain.ErrNotFound)
		}
		visible, err := svc.statusSvc.CanViewStatus(ctx, quoted, &in.AccountID)
		if err != nil {
			return EnrichedStatus{}, fmt.Errorf("Create quoted_status_id: %w", err)
		}
		if !visible {
			return EnrichedStatus{}, fmt.Errorf("Create quoted_status_id: %w", domain.ErrForbidden)
		}
		switch quoted.QuoteApprovalPolicy {
		case domain.QuotePolicyNobody:
			if quoted.AccountID != in.AccountID {
				return EnrichedStatus{}, fmt.Errorf("Create quoted_status_id: %w", domain.ErrForbidden)
			}
		case domain.QuotePolicyFollowers:
			if quoted.AccountID != in.AccountID {
				follow, followErr := svc.store.GetFollow(ctx, in.AccountID, quoted.AccountID)
				if followErr != nil || follow == nil || follow.State != domain.FollowStateAccepted {
					return EnrichedStatus{}, fmt.Errorf("Create quoted_status_id: %w", domain.ErrForbidden)
				}
			}
		default:
			// public or unknown: allow (block already checked in canViewStatus)
		}
		quotedAuthorID = &quoted.AccountID
	}
	rendered := renderResult{}
	if text != "" {
		var err error
		rendered, err = svc.renderContent(ctx, text)
		if err != nil {
			return EnrichedStatus{}, fmt.Errorf("Create Render: %w", err)
		}
	}
	if in.Poll != nil {
		if in.PollLimits == nil {
			return EnrichedStatus{}, fmt.Errorf("Create: %w", domain.ErrValidation)
		}
		if len(in.Poll.Options) < 2 || len(in.Poll.Options) > in.PollLimits.MaxOptions {
			return EnrichedStatus{}, fmt.Errorf("Create poll options: %w", domain.ErrValidation)
		}
		if in.Poll.ExpiresInSeconds < in.PollLimits.MinExpiration || in.Poll.ExpiresInSeconds > in.PollLimits.MaxExpiration {
			return EnrichedStatus{}, fmt.Errorf("Create poll expires_in: %w", domain.ErrValidation)
		}
	}
	language := in.Language
	if language == "" {
		language = "en"
	}
	statusID := uid.New()
	statusURI := fmt.Sprintf("%s/users/%s/statuses/%s", svc.instanceBaseURL, in.Username, statusID)

	quotePolicy := in.QuoteApprovalPolicy
	if quotePolicy == "" {
		quotePolicy = defaultQuotePolicy
	}
	if visibility == domain.VisibilityPrivate || visibility == domain.VisibilityDirect {
		quotePolicy = domain.QuotePolicyNobody
	}

	var created *domain.Status
	var createdPollID string
	var author *domain.Account
	var mentions []*domain.Account
	var tags []domain.Hashtag
	var media []domain.MediaAttachment
	var mentionedAccountIDs []string
	err := svc.store.WithTx(ctx, func(tx store.Store) error {
		var txErr error
		created, txErr = createStatusWithContentTx(ctx, tx, createStatusWithContentInput{
			AccountID:           in.AccountID,
			StatusID:            statusID,
			StatusURI:           statusURI,
			Visibility:          visibility,
			Text:                text,
			Content:             rendered.HTML,
			ContentWarning:      in.ContentWarning,
			Language:            language,
			Sensitive:           in.Sensitive,
			Rendered:            rendered,
			InReplyToID:         in.InReplyToID,
			InReplyToAccountID:  inReplyToAccountID,
			QuotedStatusID:      in.QuotedStatusID,
			QuoteApprovalPolicy: quotePolicy,
			QuotedAuthorID:      quotedAuthorID,
		})
		if txErr != nil {
			return txErr
		}
		if in.InReplyToID != nil && *in.InReplyToID != "" {
			if txErr = tx.IncrementRepliesCount(ctx, *in.InReplyToID); txErr != nil {
				return fmt.Errorf("IncrementRepliesCount: %w", txErr)
			}
		}
		for _, mid := range mediaIDs {
			if txErr = tx.AttachMediaToStatus(ctx, mid, statusID, in.AccountID); txErr != nil {
				return fmt.Errorf("AttachMediaToStatus: %w", txErr)
			}
		}
		if in.Poll != nil {
			pollID := uid.New()
			expiresAt := time.Now().Add(time.Duration(in.Poll.ExpiresInSeconds) * time.Second)
			if _, txErr = tx.CreatePoll(ctx, store.CreatePollInput{
				ID:        pollID,
				StatusID:  statusID,
				ExpiresAt: &expiresAt,
				Multiple:  in.Poll.Multiple,
			}); txErr != nil {
				return fmt.Errorf("CreatePoll: %w", txErr)
			}
			for i, title := range in.Poll.Options {
				optID := uid.New()
				if _, txErr = tx.CreatePollOption(ctx, store.CreatePollOptionInput{
					ID:       optID,
					PollID:   pollID,
					Title:    strings.TrimSpace(title),
					Position: i,
				}); txErr != nil {
					return fmt.Errorf("CreatePollOption: %w", txErr)
				}
			}
			createdPollID = pollID
		}
		// Gather enrichment data within the transaction for the domain event.
		author, txErr = tx.GetAccountByID(ctx, in.AccountID)
		if txErr != nil {
			return fmt.Errorf("GetAccountByID: %w", txErr)
		}
		mentions, txErr = tx.GetStatusMentions(ctx, statusID)
		if txErr != nil {
			return fmt.Errorf("GetStatusMentions: %w", txErr)
		}
		tags, txErr = tx.GetStatusHashtags(ctx, statusID)
		if txErr != nil {
			return fmt.Errorf("GetStatusHashtags: %w", txErr)
		}
		media, txErr = tx.GetStatusAttachments(ctx, statusID)
		if txErr != nil {
			return fmt.Errorf("GetStatusAttachments: %w", txErr)
		}
		mentionedAccountIDs = make([]string, 0, len(mentions))
		for _, m := range mentions {
			if m != nil {
				mentionedAccountIDs = append(mentionedAccountIDs, m.ID)
			}
		}
		return events.EmitEvent(ctx, tx, domain.EventStatusCreated, "status", statusID, domain.StatusCreatedPayload{
			Status:              created,
			Author:              author,
			Mentions:            mentions,
			Tags:                tags,
			Media:               media,
			MentionedAccountIDs: mentionedAccountIDs,
			ParentAPID:          parentAPID,
		})
	})
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("Create: %w", err)
	}
	if created.Visibility == domain.VisibilityDirect {
		if updErr := svc.conversationSvc.UpdateForDirectStatus(ctx, created, in.AccountID, mentionedAccountIDs); updErr != nil {
			slog.WarnContext(ctx, "conversation update failed after direct status create", slog.Any("error", updErr), slog.String("status_id", created.ID))
		}
	}
	out := EnrichedStatus{
		Status:   created,
		Author:   author,
		Mentions: mentions,
		Tags:     tags,
		Media:    media,
	}
	if createdPollID != "" {
		enrichedPoll, getErr := svc.statusSvc.GetPoll(ctx, createdPollID, &in.AccountID)
		if getErr == nil {
			out.Poll = enrichedPoll
		}
	}
	return out, nil
}

// Delete soft-deletes the status and decrements the account's statuses count atomically.
func (svc *statusWriteService) Delete(ctx context.Context, id, accountID string) error {
	st, err := svc.store.GetStatusByID(ctx, id)
	if err != nil {
		return fmt.Errorf("Delete(%s): %w", id, err)
	}
	if st.AccountID != accountID {
		return fmt.Errorf("Delete(%s): %w", id, domain.ErrForbidden)
	}
	if err := requireLocal(st.Local, "Delete"); err != nil {
		return err
	}
	// Best-effort enrichment: hashtag names, mention IDs, and author enrich the delete
	// event payload. The delete still proceeds if these lookups fail.
	var hashtagNames []string
	tags, err := svc.store.GetStatusHashtags(ctx, id)
	if err != nil {
		slog.WarnContext(ctx, "Delete: get hashtags for event", slog.Any("error", err), slog.String("status_id", id))
	}
	for _, t := range tags {
		hashtagNames = append(hashtagNames, t.Name)
	}
	var mentionedAccountIDs []string
	if st.Visibility == domain.VisibilityDirect {
		mentionedAccountIDs, err = svc.store.GetStatusMentionAccountIDs(ctx, id)
		if err != nil {
			slog.WarnContext(ctx, "Delete: get mention account IDs for event", slog.Any("error", err), slog.String("status_id", id))
		}
	}
	author, err := svc.store.GetAccountByID(ctx, st.AccountID)
	if err != nil {
		slog.WarnContext(ctx, "Delete: get author for event", slog.Any("error", err), slog.String("account_id", st.AccountID))
	}
	err = svc.store.WithTx(ctx, func(tx store.Store) error {
		if err := tx.SoftDeleteStatus(ctx, id); err != nil {
			return fmt.Errorf("SoftDeleteStatus: %w", err)
		}
		if err := tx.DecrementStatusesCount(ctx, st.AccountID); err != nil {
			return fmt.Errorf("DecrementStatusesCount: %w", err)
		}
		return events.EmitEvent(ctx, tx, domain.EventStatusDeleted, "status", id, domain.StatusDeletedPayload{
			StatusID:            st.ID,
			AccountID:           st.AccountID,
			Author:              author,
			Visibility:          st.Visibility,
			Local:               st.Local,
			APID:                st.APID,
			URI:                 st.URI,
			HashtagNames:        hashtagNames,
			MentionedAccountIDs: mentionedAccountIDs,
		})
	})
	if err != nil {
		return fmt.Errorf("Delete(%s): %w", id, err)
	}
	return nil
}

// Update updates a status by its owner: snapshots current to status_edits, re-renders content, updates mentions/hashtags, persists, and federates Update(Note).
func (svc *statusWriteService) Update(ctx context.Context, in UpdateStatusInput) (EnrichedStatus, error) {
	accountID := in.AccountID
	statusID := in.StatusID
	st, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return EnrichedStatus{}, fmt.Errorf("Update: %w", domain.ErrNotFound)
		}
		return EnrichedStatus{}, fmt.Errorf("Update: %w", err)
	}
	if err := requireLocal(st.Local, "Update"); err != nil {
		return EnrichedStatus{}, err
	}
	if st.AccountID != accountID {
		return EnrichedStatus{}, fmt.Errorf("Update: %w", domain.ErrForbidden)
	}
	if st.ReblogOfID != nil && *st.ReblogOfID != "" {
		return EnrichedStatus{}, fmt.Errorf("Update: %w", domain.ErrUnprocessable)
	}
	text := strings.TrimSpace(in.Text)
	if text == "" {
		return EnrichedStatus{}, fmt.Errorf("Update: %w", domain.ErrValidation)
	}
	if countStatusCharacters(text) > svc.maxStatusChars {
		return EnrichedStatus{}, fmt.Errorf("Update: %w", domain.ErrValidation)
	}
	spoilerText := in.SpoilerText
	sensitive := in.Sensitive
	rendered, err := svc.renderContent(ctx, text)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("Update Render: %w", err)
	}
	err = svc.store.WithTx(ctx, func(tx store.Store) error {
		if err := tx.CreateStatusEdit(ctx, store.CreateStatusEditInput{
			ID:             uid.New(),
			StatusID:       statusID,
			AccountID:      accountID,
			Text:           st.Text,
			Content:        st.Content,
			ContentWarning: st.ContentWarning,
			Sensitive:      st.Sensitive,
		}); err != nil {
			return fmt.Errorf("CreateStatusEdit: %w", err)
		}
		contentWarningPtr := &spoilerText
		if spoilerText == "" {
			contentWarningPtr = nil
		}
		if err := tx.UpdateStatus(ctx, store.UpdateStatusInput{
			ID:             statusID,
			Text:           &text,
			Content:        &rendered.HTML,
			ContentWarning: contentWarningPtr,
			Sensitive:      sensitive,
		}); err != nil {
			return fmt.Errorf("UpdateStatus: %w", err)
		}
		if err := tx.DeleteStatusMentions(ctx, statusID); err != nil {
			return fmt.Errorf("DeleteStatusMentions: %w", err)
		}
		for _, m := range rendered.Mentions {
			if err := tx.CreateStatusMention(ctx, statusID, m.AccountID); err != nil {
				return fmt.Errorf("CreateStatusMention: %w", err)
			}
		}
		if err := tx.DeleteStatusHashtags(ctx, statusID); err != nil {
			return fmt.Errorf("DeleteStatusHashtags: %w", err)
		}
		var hashtagIDs []string
		for _, tagName := range rendered.Tags {
			ht, err := tx.GetOrCreateHashtag(ctx, tagName)
			if err != nil {
				return fmt.Errorf("GetOrCreateHashtag: %w", err)
			}
			hashtagIDs = append(hashtagIDs, ht.ID)
		}
		if len(hashtagIDs) > 0 {
			if err := tx.AttachHashtagsToStatus(ctx, statusID, hashtagIDs); err != nil {
				return fmt.Errorf("AttachHashtagsToStatus: %w", err)
			}
		}
		updated, txErr := tx.GetStatusByID(ctx, statusID)
		if txErr != nil {
			return fmt.Errorf("GetStatusByID: %w", txErr)
		}
		updAuthor, txErr := tx.GetAccountByID(ctx, accountID)
		if txErr != nil {
			return fmt.Errorf("GetAccountByID: %w", txErr)
		}
		updMentions, txErr := tx.GetStatusMentions(ctx, statusID)
		if txErr != nil {
			return fmt.Errorf("GetStatusMentions: %w", txErr)
		}
		updTags, txErr := tx.GetStatusHashtags(ctx, statusID)
		if txErr != nil {
			return fmt.Errorf("GetStatusHashtags: %w", txErr)
		}
		updMedia, txErr := tx.GetStatusAttachments(ctx, statusID)
		if txErr != nil {
			return fmt.Errorf("GetStatusAttachments: %w", txErr)
		}
		var updParentAPID string
		if updated.InReplyToID != nil && *updated.InReplyToID != "" {
			if parent, pErr := tx.GetStatusByID(ctx, *updated.InReplyToID); pErr == nil {
				updParentAPID = parent.APID
			}
		}
		return events.EmitEvent(ctx, tx, domain.EventStatusUpdated, "status", statusID, domain.StatusUpdatedPayload{
			Status:     updated,
			Author:     updAuthor,
			Mentions:   updMentions,
			Tags:       updTags,
			Media:      updMedia,
			ParentAPID: updParentAPID,
		})
	})
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("Update: %w", err)
	}
	// Best-effort: notify authors who quoted this status that the original was updated.
	quotes, err := svc.store.ListQuotesOfStatus(ctx, statusID, nil, 500)
	if err != nil {
		slog.WarnContext(ctx, "Update: list quotes for notification", slog.Any("error", err), slog.String("status_id", statusID))
	}
	for i := range quotes {
		quotingAuthorID := quotes[i].AccountID
		if quotingAuthorID == accountID {
			continue
		}
		quotingStatusID := quotes[i].ID
		if _, notifErr := svc.store.CreateNotification(ctx, store.CreateNotificationInput{
			ID:        uid.New(),
			AccountID: quotingAuthorID,
			FromID:    accountID,
			Type:      domain.NotificationTypeQuotedUpdate,
			StatusID:  &quotingStatusID,
		}); notifErr != nil {
			slog.WarnContext(ctx, "Update: create quote update notification", slog.Any("error", notifErr))
		}
	}
	out, err := svc.statusSvc.GetByIDEnriched(ctx, statusID, &accountID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("Update GetByIDEnriched: %w", err)
	}
	return out, nil
}

func resolveVisibilityService(reqVis, defaultVis string) string {
	if reqVis != "" {
		switch reqVis {
		case domain.VisibilityPublic, domain.VisibilityUnlisted, domain.VisibilityPrivate, domain.VisibilityDirect:
			return reqVis
		}
	}
	if defaultVis != "" {
		switch defaultVis {
		case domain.VisibilityPublic, domain.VisibilityUnlisted, domain.VisibilityPrivate, domain.VisibilityDirect:
			return defaultVis
		}
	}
	return domain.VisibilityPublic
}

// UpdateQuoteApprovalPolicy updates the quote_approval_policy of a status (Mastodon-style quotes).
// Caller must be the status owner. Policy must be non-empty; use domain.QuotePolicy* constants.
func (svc *statusWriteService) UpdateQuoteApprovalPolicy(ctx context.Context, accountID, statusID, policy string) error {
	if strings.TrimSpace(policy) == "" {
		return fmt.Errorf("UpdateQuoteApprovalPolicy: %w", domain.ErrValidation)
	}
	st, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("UpdateQuoteApprovalPolicy: %w", domain.ErrNotFound)
		}
		return fmt.Errorf("UpdateQuoteApprovalPolicy: %w", err)
	}
	if err := requireLocal(st.Local, "UpdateQuoteApprovalPolicy"); err != nil {
		return err
	}
	if st.AccountID != accountID {
		return fmt.Errorf("UpdateQuoteApprovalPolicy: %w", domain.ErrForbidden)
	}
	if st.Visibility == domain.VisibilityPrivate || st.Visibility == domain.VisibilityDirect {
		policy = domain.QuotePolicyNobody
	} else {
		switch policy {
		case domain.QuotePolicyPublic, domain.QuotePolicyFollowers, domain.QuotePolicyNobody:
			// valid
		default:
			return fmt.Errorf("UpdateQuoteApprovalPolicy: %w", domain.ErrValidation)
		}
	}
	if err := svc.store.UpdateStatusQuoteApprovalPolicy(ctx, statusID, policy); err != nil {
		return fmt.Errorf("UpdateQuoteApprovalPolicy: %w", err)
	}
	return nil
}

// RevokeQuote revokes a quote of the given status by the quoting status (Mastodon-style quotes).
// Caller must be the author of the quoted status.
func (svc *statusWriteService) RevokeQuote(ctx context.Context, accountID, quotedStatusID, quotingStatusID string) error {
	quoted, err := svc.store.GetStatusByID(ctx, quotedStatusID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("RevokeQuote: %w", domain.ErrNotFound)
		}
		return fmt.Errorf("RevokeQuote: %w", err)
	}
	if quoted.AccountID != accountID {
		return fmt.Errorf("RevokeQuote: %w", domain.ErrForbidden)
	}
	if err := svc.store.RevokeQuote(ctx, quotedStatusID, quotingStatusID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("RevokeQuote: %w", domain.ErrNotFound)
		}
		return fmt.Errorf("RevokeQuote: %w", err)
	}
	if err := svc.store.DecrementQuotesCount(ctx, quotedStatusID); err != nil {
		return fmt.Errorf("RevokeQuote DecrementQuotesCount: %w", err)
	}
	return nil
}
