package service

import (
	"context"
	"encoding/json"
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
	MediaIDs       []string // optional; attached after status is created (max 4)
	APID           string
	ApRaw          []byte
	Sensitive      bool
}

// CreateRemoteReblogInput is the input for creating a remote reblog (e.g. from federation Announce).
type CreateRemoteReblogInput struct {
	AccountID        string
	ActivityAPID     string
	ObjectStatusAPID string
	ApRaw            []byte
}

// UpdateStatusInput is the input for updating a local status.
type UpdateStatusInput struct {
	AccountID   string
	StatusID    string
	Text        string
	SpoilerText string
	Sensitive   bool
}

// UpdateRemoteStatusInput is the input for updating a remote status (e.g. from federation Update{Note}).
type UpdateRemoteStatusInput struct {
	Text           *string
	Content        *string
	ContentWarning *string
	Sensitive      bool
}

// StatusWriteService orchestrates status write operations (create, delete, reblog, favourite, bookmark, pin, etc.)
// and their cross-service side effects (federation, event bus, conversation updates).
// It depends on StatusService for reads and visibility checks to avoid circular dependency with ConversationService.
type StatusWriteService interface {
	// Local status CRUD.
	Create(ctx context.Context, in CreateStatusInput) (EnrichedStatus, error)
	Update(ctx context.Context, in UpdateStatusInput) (EnrichedStatus, error)
	Delete(ctx context.Context, id string) error

	// Local reblog.
	CreateReblog(ctx context.Context, accountID, username, statusID string) (EnrichedStatus, error)
	DeleteReblog(ctx context.Context, accountID, statusID string) error

	// Local favourite.
	CreateFavourite(ctx context.Context, accountID, statusID string) (EnrichedStatus, error)
	DeleteFavourite(ctx context.Context, accountID, statusID string) (EnrichedStatus, error)

	// Local bookmark.
	Bookmark(ctx context.Context, accountID, statusID string) (EnrichedStatus, error)
	Unbookmark(ctx context.Context, accountID, statusID string) (EnrichedStatus, error)

	// Local pin.
	Pin(ctx context.Context, accountID, statusID string) (EnrichedStatus, error)
	Unpin(ctx context.Context, accountID, statusID string) (EnrichedStatus, error)

	// Scheduled status writes.
	CreateScheduledStatus(ctx context.Context, accountID string, params []byte, scheduledAt time.Time) (*domain.ScheduledStatus, error)
	UpdateScheduledStatus(ctx context.Context, id, accountID string, params []byte, scheduledAt time.Time) (*domain.ScheduledStatus, error)
	DeleteScheduledStatus(ctx context.Context, id, accountID string) error
	PublishScheduled(ctx context.Context, scheduledID string) error
	PublishDueStatuses(ctx context.Context, limit int) error

	// Poll writes.
	RecordVote(ctx context.Context, pollID, accountID string, optionIndices []int) (*EnrichedPoll, error)

	// Conversation writes.
	MuteConversation(ctx context.Context, accountID, statusID string) error
	UnmuteConversation(ctx context.Context, accountID, statusID string) error

	// Quote writes.
	UpdateQuoteApprovalPolicy(ctx context.Context, accountID, statusID, policy string) error
	RevokeQuote(ctx context.Context, accountID, quotedStatusID, quotingStatusID string) error

	// Remote status write operations.
	CreateRemote(ctx context.Context, in CreateRemoteStatusInput) (*domain.Status, error)
	UpdateRemote(ctx context.Context, statusID string, st *domain.Status, in UpdateRemoteStatusInput) error
	DeleteRemote(ctx context.Context, statusID string) error
	CreateRemoteReblog(ctx context.Context, in CreateRemoteReblogInput) (*domain.Status, error)
	CreateRemoteFavourite(ctx context.Context, accountID, statusID string, apID *string) (*domain.Favourite, error)
	DeleteRemoteFavourite(ctx context.Context, accountID, statusID string) error
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

func (svc *statusWriteService) mentionResolver(ctx context.Context) MentionResolver {
	return func(username string, domain *string) *domain.Account {
		if domain == nil || *domain == "" {
			a, _ := svc.store.GetLocalAccountByUsername(ctx, username)
			return a
		}
		a, _ := svc.store.GetRemoteAccountByUsername(ctx, username, domain)
		return a
	}
}

func createStatusWithContentTx(
	ctx context.Context,
	tx store.Store,
	accountID, _, statusID, statusURI, visibility, text, content, contentWarning, language string,
	sensitive bool,
	renderResult RenderResult,
	inReplyToID, inReplyToAccountID *string,
	quotedStatusID *string,
	quoteApprovalPolicy string,
	quotedAuthorID *string,
	_ []string, // mediaIDs are attached by caller after CreateStatus
) (*domain.Status, error) {
	var textPtr, contentPtr *string
	if text != "" {
		textPtr = &text
		contentPtr = &content
	}
	st, err := tx.CreateStatus(ctx, store.CreateStatusInput{
		ID:                  statusID,
		URI:                 statusURI,
		AccountID:           accountID,
		Text:                textPtr,
		Content:             contentPtr,
		ContentWarning:      &contentWarning,
		Visibility:          visibility,
		Language:            &language,
		InReplyToID:         inReplyToID,
		InReplyToAccountID:  inReplyToAccountID,
		QuotedStatusID:      quotedStatusID,
		QuoteApprovalPolicy: quoteApprovalPolicy,
		Sensitive:           sensitive,
		Local:               true,
		APID:                statusURI,
		ApRaw:               nil,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateStatus: %w", err)
	}
	if quotedStatusID != nil && *quotedStatusID != "" {
		if err := tx.CreateQuoteApproval(ctx, statusID, *quotedStatusID); err != nil {
			return nil, fmt.Errorf("CreateQuoteApproval: %w", err)
		}
		if err := tx.IncrementQuotesCount(ctx, *quotedStatusID); err != nil {
			return nil, fmt.Errorf("IncrementQuotesCount: %w", err)
		}
		if quotedAuthorID != nil && *quotedAuthorID != accountID {
			notifID := uid.New()
			_, err = tx.CreateNotification(ctx, store.CreateNotificationInput{
				ID:        notifID,
				AccountID: *quotedAuthorID,
				FromID:    accountID,
				Type:      domain.NotificationTypeQuote,
				StatusID:  &statusID,
			})
			if err != nil {
				return nil, fmt.Errorf("CreateNotification quote: %w", err)
			}
		}
	}
	for _, m := range renderResult.Mentions {
		if err := tx.CreateStatusMention(ctx, statusID, m.AccountID); err != nil {
			return nil, fmt.Errorf("CreateStatusMention: %w", err)
		}
	}
	var hashtagIDs []string
	for _, tagName := range renderResult.Tags {
		ht, err := tx.GetOrCreateHashtag(ctx, tagName)
		if err != nil {
			return nil, fmt.Errorf("GetOrCreateHashtag(%s): %w", tagName, err)
		}
		hashtagIDs = append(hashtagIDs, ht.ID)
	}
	if len(hashtagIDs) > 0 {
		if err := tx.AttachHashtagsToStatus(ctx, statusID, hashtagIDs); err != nil {
			return nil, fmt.Errorf("AttachHashtagsToStatus: %w", err)
		}
	}
	if err := tx.IncrementStatusesCount(ctx, accountID); err != nil {
		return nil, fmt.Errorf("IncrementStatusesCount: %w", err)
	}
	if err := tx.UpdateAccountLastStatusAt(ctx, accountID); err != nil {
		return nil, fmt.Errorf("UpdateAccountLastStatusAt: %w", err)
	}
	root, err := tx.GetConversationRoot(ctx, statusID)
	if err != nil {
		return nil, fmt.Errorf("GetConversationRoot: %w", err)
	}
	for _, m := range renderResult.Mentions {
		mentioned, _ := tx.GetAccountByID(ctx, m.AccountID)
		if mentioned == nil || (mentioned.Domain != nil && *mentioned.Domain != "") {
			continue
		}
		muted, err := tx.IsConversationMuted(ctx, mentioned.ID, root)
		if err != nil {
			return nil, fmt.Errorf("IsConversationMuted: %w", err)
		}
		if muted {
			continue
		}
		notifID := uid.New()
		_, err = tx.CreateNotification(ctx, store.CreateNotificationInput{
			ID:        notifID,
			AccountID: mentioned.ID,
			FromID:    accountID,
			Type:      domain.NotificationTypeMention,
			StatusID:  &statusID,
		})
		if err != nil {
			return nil, fmt.Errorf("CreateNotification: %w", err)
		}
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
	if text != "" && CountStatusCharacters(text) > svc.maxStatusChars {
		return EnrichedStatus{}, fmt.Errorf("Create: %w", domain.ErrValidation)
	}
	var inReplyToAccountID *string
	if in.InReplyToID != nil && *in.InReplyToID != "" {
		parent, err := svc.store.GetStatusByID(ctx, *in.InReplyToID)
		if err != nil {
			return EnrichedStatus{}, fmt.Errorf("Create in_reply_to: %w", err)
		}
		if parent.DeletedAt != nil {
			return EnrichedStatus{}, fmt.Errorf("Create in_reply_to: %w", domain.ErrNotFound)
		}
		inReplyToAccountID = &parent.AccountID
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
	renderResult := RenderResult{}
	if text != "" {
		resolver := svc.mentionResolver(ctx)
		var err error
		renderResult, err = Render(text, svc.instanceDomain, resolver)
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
		created, txErr = createStatusWithContentTx(ctx, tx, in.AccountID, in.Username, statusID, statusURI, visibility, text, renderResult.HTML, in.ContentWarning, language, in.Sensitive, renderResult, in.InReplyToID, inReplyToAccountID, in.QuotedStatusID, quotePolicy, quotedAuthorID, mediaIDs)
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

// CreateRemote creates a remote status. Does not publish to federation or increment account statuses count.
// If MediaIDs is set, attaches those media to the status. If InReplyToID is set, increments the parent's replies count.
func (svc *statusWriteService) CreateRemote(ctx context.Context, in CreateRemoteStatusInput) (*domain.Status, error) {
	st, err := svc.store.CreateStatus(ctx, store.CreateStatusInput{
		ID:                  uid.New(),
		URI:                 in.URI,
		AccountID:           in.AccountID,
		Text:                in.Text,
		Content:             in.Content,
		ContentWarning:      in.ContentWarning,
		Visibility:          in.Visibility,
		Language:            in.Language,
		InReplyToID:         in.InReplyToID,
		APID:                in.APID,
		ApRaw:               in.ApRaw,
		Sensitive:           in.Sensitive,
		Local:               false,
		QuoteApprovalPolicy: domain.QuotePolicyPublic,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateRemote: %w", err)
	}
	mediaIDs := in.MediaIDs
	if len(mediaIDs) > 4 {
		mediaIDs = mediaIDs[:4]
	}
	for _, mediaID := range mediaIDs {
		if attErr := svc.store.AttachMediaToStatus(ctx, mediaID, st.ID, in.AccountID); attErr != nil {
			slog.WarnContext(ctx, "CreateRemote: attach media failed", slog.String("media_id", mediaID), slog.Any("error", attErr))
		}
	}
	if in.InReplyToID != nil && *in.InReplyToID != "" {
		if incErr := svc.store.IncrementRepliesCount(ctx, *in.InReplyToID); incErr != nil {
			slog.WarnContext(ctx, "CreateRemote: increment replies count failed", slog.String("parent_id", *in.InReplyToID), slog.Any("error", incErr))
		}
	}
	if in.Visibility == domain.VisibilityDirect {
		mentionedIDs, _ := svc.store.GetStatusMentionAccountIDs(ctx, st.ID)
		if updErr := svc.conversationSvc.UpdateForDirectStatus(ctx, st, st.AccountID, mentionedIDs); updErr != nil {
			slog.WarnContext(ctx, "conversation update failed after direct status from inbox", slog.Any("error", updErr), slog.String("status_id", st.ID))
		}
	}
	return st, nil
}

// Delete soft-deletes the status and decrements the account's statuses count atomically.
func (svc *statusWriteService) Delete(ctx context.Context, id string) error {
	st, err := svc.store.GetStatusByID(ctx, id)
	if err != nil {
		return fmt.Errorf("Delete(%s): %w", id, err)
	}
	var hashtagNames []string
	tags, _ := svc.store.GetStatusHashtags(ctx, id)
	for _, t := range tags {
		hashtagNames = append(hashtagNames, t.Name)
	}
	var mentionedAccountIDs []string
	if st.Visibility == domain.VisibilityDirect {
		mentionedAccountIDs, _ = svc.store.GetStatusMentionAccountIDs(ctx, id)
	}
	author, _ := svc.store.GetAccountByID(ctx, st.AccountID)
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

// DeleteRemote soft-deletes a remote status. Publishes SSE delete event; does not decrement account statuses count or publish to federation.
func (svc *statusWriteService) DeleteRemote(ctx context.Context, statusID string) error {
	st, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil {
		return fmt.Errorf("DeleteRemote(%s): %w", statusID, err)
	}
	var hashtagNames []string
	tags, _ := svc.store.GetStatusHashtags(ctx, statusID)
	for _, t := range tags {
		hashtagNames = append(hashtagNames, t.Name)
	}
	var mentionedAccountIDs []string
	if st.Visibility == domain.VisibilityDirect {
		mentionedAccountIDs, _ = svc.store.GetStatusMentionAccountIDs(ctx, statusID)
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

// CreateReblog creates a reblog status for the given status. Returns the new reblog status (with nested original).
func (svc *statusWriteService) CreateReblog(ctx context.Context, accountID, username, statusID string) (EnrichedStatus, error) {
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
	existing, _ := svc.store.GetReblogByAccountAndTarget(ctx, accountID, statusID)
	if existing != nil {
		return EnrichedStatus{}, fmt.Errorf("CreateReblog: %w", domain.ErrConflict)
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
			ApRaw:               nil,
			QuoteApprovalPolicy: domain.QuotePolicyPublic,
		})
		if err != nil {
			return fmt.Errorf("CreateStatus: %w", err)
		}
		if err := tx.IncrementReblogsCount(ctx, statusID); err != nil {
			return fmt.Errorf("IncrementReblogsCount: %w", err)
		}
		origAuthor, _ := tx.GetAccountByID(ctx, orig.AccountID)
		if origAuthor != nil && (origAuthor.Domain == nil || *origAuthor.Domain == "") {
			_, _ = tx.CreateNotification(ctx, store.CreateNotificationInput{
				ID:        uid.New(),
				AccountID: orig.AccountID,
				FromID:    accountID,
				Type:      domain.NotificationTypeReblog,
				StatusID:  &statusID,
			})
		}
		return nil
	})
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("CreateReblog: %w", err)
	}
	origAuthor, _ := svc.store.GetAccountByID(ctx, orig.AccountID)
	rebloggerAccount, _ := svc.store.GetAccountByID(ctx, accountID)
	if origAuthor != nil && (origAuthor.Domain == nil || *origAuthor.Domain == "") && rebloggerAccount != nil {
		notifs, _ := svc.store.ListNotifications(ctx, orig.AccountID, nil, 1)
		if len(notifs) > 0 && notifs[0].FromID == accountID && notifs[0].Type == domain.NotificationTypeReblog && notifs[0].StatusID != nil && *notifs[0].StatusID == statusID {
			if emitErr := events.EmitEvent(ctx, svc.store, domain.EventNotificationCreated, "notification", notifs[0].ID, domain.NotificationCreatedPayload{
				RecipientAccountID: orig.AccountID,
				Notification:       &notifs[0],
				FromAccount:        rebloggerAccount,
				StatusID:           &statusID,
			}); emitErr != nil {
				slog.WarnContext(ctx, "emit reblog notification event failed", slog.Any("error", emitErr))
			}
		}
	}
	out, err := svc.statusSvc.GetByIDEnriched(ctx, reblogID, &accountID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("CreateReblog: %w", err)
	}
	return out, nil
}

// DeleteReblog removes the viewer's reblog of the given status. Idempotent: if no reblog exists, returns nil.
func (svc *statusWriteService) DeleteReblog(ctx context.Context, accountID, statusID string) error {
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
	if err := svc.store.WithTx(ctx, func(tx store.Store) error {
		if err := tx.SoftDeleteStatus(ctx, reblog.ID); err != nil {
			return fmt.Errorf("SoftDeleteStatus: %w", err)
		}
		if err := tx.DecrementReblogsCount(ctx, statusID); err != nil {
			return fmt.Errorf("DecrementReblogsCount: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("DeleteReblog: %w", err)
	}
	return nil
}

// CreateFavourite adds a favourite for the viewer on the status. Returns the status with favourited true.
func (svc *statusWriteService) CreateFavourite(ctx context.Context, accountID, statusID string) (EnrichedStatus, error) {
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
	_, err = svc.store.CreateFavourite(ctx, store.CreateFavouriteInput{
		ID:        uid.New(),
		AccountID: accountID,
		StatusID:  statusID,
		APID:      nil,
	})
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("CreateFavourite: %w", err)
	}
	if err := svc.store.IncrementFavouritesCount(ctx, statusID); err != nil {
		return EnrichedStatus{}, fmt.Errorf("IncrementFavouritesCount: %w", err)
	}
	author, _ := svc.store.GetAccountByID(ctx, st.AccountID)
	var createdNotif *domain.Notification
	if author != nil && (author.Domain == nil || *author.Domain == "") {
		createdNotif, _ = svc.store.CreateNotification(ctx, store.CreateNotificationInput{
			ID:        uid.New(),
			AccountID: st.AccountID,
			FromID:    accountID,
			Type:      domain.NotificationTypeFavourite,
			StatusID:  &statusID,
		})
	}
	if createdNotif != nil {
		favouriterAccount, _ := svc.store.GetAccountByID(ctx, accountID)
		if favouriterAccount != nil {
			if emitErr := events.EmitEvent(ctx, svc.store, domain.EventNotificationCreated, "notification", createdNotif.ID, domain.NotificationCreatedPayload{
				RecipientAccountID: st.AccountID,
				Notification:       createdNotif,
				FromAccount:        favouriterAccount,
				StatusID:           &statusID,
			}); emitErr != nil {
				slog.WarnContext(ctx, "emit favourite notification event failed", slog.Any("error", emitErr))
			}
		}
	}
	out, err := svc.statusSvc.GetByIDEnriched(ctx, statusID, &accountID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("CreateFavourite: %w", err)
	}
	return out, nil
}

// DeleteFavourite removes the viewer's favourite. Returns the status with favourited false.
func (svc *statusWriteService) DeleteFavourite(ctx context.Context, accountID, statusID string) (EnrichedStatus, error) {
	_ = svc.store.DeleteFavourite(ctx, accountID, statusID)
	_ = svc.store.DecrementFavouritesCount(ctx, statusID)
	out, err := svc.statusSvc.GetByIDEnriched(ctx, statusID, &accountID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("DeleteFavourite: %w", err)
	}
	return out, nil
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
	if CountStatusCharacters(text) > svc.maxStatusChars {
		return EnrichedStatus{}, fmt.Errorf("Update: %w", domain.ErrValidation)
	}
	spoilerText := in.SpoilerText
	sensitive := in.Sensitive
	resolver := svc.mentionResolver(ctx)
	renderResult, err := Render(text, svc.instanceDomain, resolver)
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
			Content:        &renderResult.HTML,
			ContentWarning: contentWarningPtr,
			Sensitive:      sensitive,
		}); err != nil {
			return fmt.Errorf("UpdateStatus: %w", err)
		}
		if err := tx.DeleteStatusMentions(ctx, statusID); err != nil {
			return fmt.Errorf("DeleteStatusMentions: %w", err)
		}
		for _, m := range renderResult.Mentions {
			if err := tx.CreateStatusMention(ctx, statusID, m.AccountID); err != nil {
				return fmt.Errorf("CreateStatusMention: %w", err)
			}
		}
		if err := tx.DeleteStatusHashtags(ctx, statusID); err != nil {
			return fmt.Errorf("DeleteStatusHashtags: %w", err)
		}
		var hashtagIDs []string
		for _, tagName := range renderResult.Tags {
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
		return events.EmitEvent(ctx, tx, domain.EventStatusUpdated, "status", statusID, domain.StatusUpdatedPayload{
			Status: updated,
			Author: updAuthor,
		})
	})
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("Update: %w", err)
	}
	quotes, err := svc.store.ListQuotesOfStatus(ctx, statusID, nil, 500)
	if err == nil {
		for i := range quotes {
			quotingAuthorID := quotes[i].AccountID
			if quotingAuthorID == accountID {
				continue
			}
			quotingStatusID := quotes[i].ID
			_, _ = svc.store.CreateNotification(ctx, store.CreateNotificationInput{
				ID:        uid.New(),
				AccountID: quotingAuthorID,
				FromID:    accountID,
				Type:      domain.NotificationTypeQuotedUpdate,
				StatusID:  &quotingStatusID,
			})
		}
	}
	out, err := svc.statusSvc.GetByIDEnriched(ctx, statusID, &accountID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("Update GetByIDEnriched: %w", err)
	}
	return out, nil
}

// CreateRemoteReblog creates a remote reblog status. Increments reblogs count on the original.
func (svc *statusWriteService) CreateRemoteReblog(ctx context.Context, in CreateRemoteReblogInput) (*domain.Status, error) {
	original, err := svc.store.GetStatusByAPID(ctx, in.ObjectStatusAPID)
	if err != nil {
		return nil, fmt.Errorf("CreateRemoteReblog GetStatusByAPID: %w", err)
	}
	reblogOfID := original.ID
	st, err := svc.store.CreateStatus(ctx, store.CreateStatusInput{
		ID:                  uid.New(),
		URI:                 in.ActivityAPID,
		AccountID:           in.AccountID,
		Visibility:          domain.VisibilityPublic,
		ReblogOfID:          &reblogOfID,
		APID:                in.ActivityAPID,
		ApRaw:               in.ApRaw,
		Local:               false,
		QuoteApprovalPolicy: domain.QuotePolicyPublic,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateRemoteReblog CreateStatus: %w", err)
	}
	if err := svc.store.IncrementReblogsCount(ctx, original.ID); err != nil {
		return nil, fmt.Errorf("CreateRemoteReblog IncrementReblogsCount: %w", err)
	}
	return st, nil
}

// UpdateRemote creates a status edit record and updates the status for a remote status.
func (svc *statusWriteService) UpdateRemote(ctx context.Context, statusID string, st *domain.Status, in UpdateRemoteStatusInput) error {
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
	return nil
}

// CreateRemoteFavourite creates a favourite from a remote actor. Increments favourites count.
func (svc *statusWriteService) CreateRemoteFavourite(ctx context.Context, accountID, statusID string, apID *string) (*domain.Favourite, error) {
	fav, err := svc.store.CreateFavourite(ctx, store.CreateFavouriteInput{
		ID:        uid.New(),
		AccountID: accountID,
		StatusID:  statusID,
		APID:      apID,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateRemoteFavourite: %w", err)
	}
	if err := svc.store.IncrementFavouritesCount(ctx, statusID); err != nil {
		return nil, fmt.Errorf("CreateRemoteFavourite IncrementFavouritesCount: %w", err)
	}
	return fav, nil
}

// DeleteRemoteFavourite removes a remote actor's favourite and decrements the status favourites count.
func (svc *statusWriteService) DeleteRemoteFavourite(ctx context.Context, accountID, statusID string) error {
	if err := svc.store.DeleteFavourite(ctx, accountID, statusID); err != nil {
		return fmt.Errorf("DeleteRemoteFavourite: %w", err)
	}
	if err := svc.store.DecrementFavouritesCount(ctx, statusID); err != nil {
		return fmt.Errorf("DeleteRemoteFavourite DecrementFavouritesCount: %w", err)
	}
	return nil
}

func (svc *statusWriteService) PublishScheduled(ctx context.Context, scheduledID string) error {
	s, err := svc.store.GetScheduledStatusByID(ctx, scheduledID)
	if err != nil {
		return fmt.Errorf("PublishScheduled GetScheduledStatusByID: %w", err)
	}
	var p domain.ScheduledStatusParams
	if err := json.Unmarshal(s.Params, &p); err != nil {
		return fmt.Errorf("PublishScheduled invalid params: %w", err)
	}
	acc, err := svc.store.GetAccountByID(ctx, s.AccountID)
	if err != nil {
		return fmt.Errorf("PublishScheduled GetAccountByID: %w", err)
	}
	user, _ := svc.store.GetUserByAccountID(ctx, s.AccountID)
	defaultVisibility := ""
	if user != nil {
		defaultVisibility = user.DefaultPrivacy
	}
	var inReplyToID *string
	if p.InReplyToID != "" {
		inReplyToID = &p.InReplyToID
	}
	_, err = svc.Create(ctx, CreateStatusInput{
		AccountID:         s.AccountID,
		Username:          acc.Username,
		Text:              p.Text,
		Visibility:        p.Visibility,
		DefaultVisibility: defaultVisibility,
		ContentWarning:    p.SpoilerText,
		Language:          p.Language,
		Sensitive:         p.Sensitive,
		InReplyToID:       inReplyToID,
		MediaIDs:          p.MediaIDs,
	})
	if err != nil {
		return fmt.Errorf("PublishScheduled Create: %w", err)
	}
	if err := svc.store.DeleteScheduledStatus(ctx, scheduledID); err != nil {
		return fmt.Errorf("PublishScheduled DeleteScheduledStatus: %w", err)
	}
	return nil
}

func (svc *statusWriteService) PublishDueStatuses(ctx context.Context, limit int) error {
	due, err := svc.store.ListScheduledStatusesDue(ctx, limit)
	if err != nil {
		return fmt.Errorf("list due: %w", err)
	}
	for i := range due {
		if err := svc.PublishScheduled(ctx, due[i].ID); err != nil {
			slog.WarnContext(ctx, "scheduled status publish failed",
				slog.String("id", due[i].ID), slog.Any("error", err))
		}
	}
	return nil
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

// Bookmark adds the status to the account's bookmarks. Idempotent if already bookmarked.
func (svc *statusWriteService) Bookmark(ctx context.Context, accountID, statusID string) (EnrichedStatus, error) {
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
func (svc *statusWriteService) Unbookmark(ctx context.Context, accountID, statusID string) (EnrichedStatus, error) {
	_ = svc.store.DeleteBookmark(ctx, accountID, statusID)
	out, err := svc.statusSvc.GetByIDEnriched(ctx, statusID, &accountID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("Unbookmark: %w", err)
	}
	return out, nil
}

const maxPinsPerAccount = 5

func (svc *statusWriteService) Pin(ctx context.Context, accountID, statusID string) (EnrichedStatus, error) {
	st, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return EnrichedStatus{}, fmt.Errorf("Pin: %w", domain.ErrNotFound)
		}
		return EnrichedStatus{}, fmt.Errorf("Pin: %w", err)
	}
	if st.AccountID != accountID {
		return EnrichedStatus{}, fmt.Errorf("Pin: %w", domain.ErrForbidden)
	}
	if st.Visibility != "public" && st.Visibility != "unlisted" {
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

func (svc *statusWriteService) Unpin(ctx context.Context, accountID, statusID string) (EnrichedStatus, error) {
	st, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return EnrichedStatus{}, fmt.Errorf("Unpin: %w", domain.ErrNotFound)
		}
		return EnrichedStatus{}, fmt.Errorf("Unpin: %w", err)
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

func (svc *statusWriteService) CreateScheduledStatus(ctx context.Context, accountID string, params []byte, scheduledAt time.Time) (*domain.ScheduledStatus, error) {
	now := time.Now()
	if !scheduledAt.After(now) {
		return nil, fmt.Errorf("CreateScheduledStatus scheduled_at must be in the future: %w", domain.ErrValidation)
	}
	var p domain.ScheduledStatusParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("CreateScheduledStatus invalid params: %w", domain.ErrValidation)
	}
	id := uid.New()
	s, err := svc.store.CreateScheduledStatus(ctx, store.CreateScheduledStatusInput{
		ID:          id,
		AccountID:   accountID,
		Params:      params,
		ScheduledAt: scheduledAt,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateScheduledStatus: %w", err)
	}
	return s, nil
}

func (svc *statusWriteService) UpdateScheduledStatus(ctx context.Context, id, accountID string, params []byte, scheduledAt time.Time) (*domain.ScheduledStatus, error) {
	s, err := svc.store.GetScheduledStatusByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("UpdateScheduledStatus: %w", err)
		}
		return nil, fmt.Errorf("UpdateScheduledStatus: %w", err)
	}
	if s.AccountID != accountID {
		return nil, domain.ErrNotFound
	}
	var p domain.ScheduledStatusParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("UpdateScheduledStatus invalid params: %w", domain.ErrValidation)
	}
	now := time.Now()
	if !scheduledAt.After(now) {
		return nil, fmt.Errorf("UpdateScheduledStatus scheduled_at must be in the future: %w", domain.ErrValidation)
	}
	updated, err := svc.store.UpdateScheduledStatus(ctx, store.UpdateScheduledStatusInput{
		ID:          id,
		Params:      params,
		ScheduledAt: scheduledAt,
	})
	if err != nil {
		return nil, fmt.Errorf("UpdateScheduledStatus: %w", err)
	}
	return updated, nil
}

func (svc *statusWriteService) DeleteScheduledStatus(ctx context.Context, id, accountID string) error {
	s, err := svc.store.GetScheduledStatusByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("DeleteScheduledStatus: %w", err)
		}
		return fmt.Errorf("DeleteScheduledStatus: %w", err)
	}
	if s.AccountID != accountID {
		return domain.ErrNotFound
	}
	if err := svc.store.DeleteScheduledStatus(ctx, id); err != nil {
		return fmt.Errorf("DeleteScheduledStatus: %w", err)
	}
	return nil
}

// RecordVote records the viewer's vote on a poll (replacing any existing vote). Returns the updated EnrichedPoll.
func (svc *statusWriteService) RecordVote(ctx context.Context, pollID, accountID string, optionIndices []int) (*EnrichedPoll, error) {
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

// MuteConversation mutes the conversation (thread) containing the given status for the account.
func (svc *statusWriteService) MuteConversation(ctx context.Context, accountID, statusID string) error {
	root, err := svc.store.GetConversationRoot(ctx, statusID)
	if err != nil {
		return fmt.Errorf("MuteConversation GetConversationRoot: %w", err)
	}
	if err := svc.store.CreateConversationMute(ctx, accountID, root); err != nil {
		return fmt.Errorf("CreateConversationMute: %w", err)
	}
	return nil
}

// UnmuteConversation unmutes the conversation containing the given status for the account.
func (svc *statusWriteService) UnmuteConversation(ctx context.Context, accountID, statusID string) error {
	root, err := svc.store.GetConversationRoot(ctx, statusID)
	if err != nil {
		return fmt.Errorf("UnmuteConversation GetConversationRoot: %w", err)
	}
	if err := svc.store.DeleteConversationMute(ctx, accountID, root); err != nil {
		return fmt.Errorf("DeleteConversationMute: %w", err)
	}
	return nil
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
