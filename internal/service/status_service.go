package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	"github.com/chairswithlegs/monstera-fed/internal/uid"
)

// StatusFederationPublisher publishes status create/delete to federation (e.g. ap.OutboxPublisher).
type StatusFederationPublisher interface {
	PublishStatus(ctx context.Context, status *domain.Status) error
	DeleteStatus(ctx context.Context, status *domain.Status) error
}

// NoopFederationPublisher is a StatusFederationPublisher that does nothing.
// Use when federation is disabled (e.g. no NATS) or in tests.
var NoopFederationPublisher StatusFederationPublisher = (*noopFederationPublisher)(nil)

type noopFederationPublisher struct{}

func (*noopFederationPublisher) PublishStatus(context.Context, *domain.Status) error { return nil }
func (*noopFederationPublisher) DeleteStatus(context.Context, *domain.Status) error  { return nil }

// StatusService handles status creation, lookup, and soft delete.
type StatusService struct {
	store           store.Store
	fed             StatusFederationPublisher
	instanceBaseURL string
	instanceDomain  string
	maxStatusChars  int
	logger          *slog.Logger
}

// NewStatusService returns a StatusService that uses the given store and instance URLs.
// fed must be non-nil; use NoopFederationPublisher when federation is disabled. logger may be nil (federation failures will not be logged).
func NewStatusService(s store.Store, fed StatusFederationPublisher, instanceBaseURL string, instanceDomain string, maxStatusChars int, logger *slog.Logger) *StatusService {
	if fed == nil {
		panic("StatusService: fed must be non-nil. Use NoopFederationPublisher when federation is disabled")
	}
	base := strings.TrimSuffix(instanceBaseURL, "/")
	return &StatusService{
		store:           s,
		fed:             fed,
		instanceBaseURL: base,
		instanceDomain:  instanceDomain,
		maxStatusChars:  maxStatusChars,
		logger:          logger,
	}
}

// CreateWithContentInput is the input for creating a status with plain text (content is rendered in-service).
type CreateWithContentInput struct {
	AccountID         string
	Username          string
	Text              string
	Visibility        string
	DefaultVisibility string // used when Visibility is empty or invalid
	ContentWarning    string
	Language          string
	Sensitive         bool
}

// CreateResult is the result of CreateWithContent, with all data needed to build the API response.
type CreateResult struct {
	Status   *domain.Status
	Author   *domain.Account
	Mentions []*domain.Account
	Tags     []domain.Hashtag
	Media    []domain.MediaAttachment
}

// CreateStatusInput is the input for creating a status.
type CreateStatusInput struct {
	AccountID      string
	Text           *string
	Content        *string
	ContentWarning *string
	Visibility     string
	Language       *string
	InReplyToID    *string
	ReblogOfID     *string
	Sensitive      bool
}

// Create creates a status and increments the account's statuses count atomically.
func (svc *StatusService) Create(ctx context.Context, in CreateStatusInput) (*domain.Status, error) {
	if in.AccountID == "" {
		return nil, fmt.Errorf("CreateStatus: %w", domain.ErrValidation)
	}
	if in.Text == nil || *in.Text == "" {
		return nil, fmt.Errorf("CreateStatus: %w", domain.ErrValidation)
	}
	switch in.Visibility {
	case domain.VisibilityPublic, domain.VisibilityUnlisted, domain.VisibilityPrivate, domain.VisibilityDirect:
	default:
		return nil, fmt.Errorf("CreateStatus: %w", domain.ErrValidation)
	}
	id := uid.New()
	uri := fmt.Sprintf("%s/statuses/%s", svc.instanceBaseURL, id)
	storeIn := store.CreateStatusInput{
		ID:             id,
		URI:            uri,
		AccountID:      in.AccountID,
		Text:           in.Text,
		Content:        in.Content,
		ContentWarning: in.ContentWarning,
		Visibility:     in.Visibility,
		Language:       in.Language,
		InReplyToID:    in.InReplyToID,
		ReblogOfID:     in.ReblogOfID,
		APID:           uri,
		ApRaw:          nil,
		Sensitive:      in.Sensitive,
		Local:          true,
	}
	var st *domain.Status
	err := svc.store.WithTx(ctx, func(tx store.Store) error {
		var err error
		st, err = tx.CreateStatus(ctx, storeIn)
		if err != nil {
			return fmt.Errorf("CreateStatus: %w", err)
		}
		return tx.IncrementStatusesCount(ctx, in.AccountID)
	})
	if err != nil {
		return nil, fmt.Errorf("CreateStatus: %w", err)
	}
	if err := svc.fed.PublishStatus(ctx, st); err != nil && svc.logger != nil {
		svc.logger.WarnContext(ctx, "federation publish failed after status create", slog.Any("error", err), slog.String("status_id", st.ID))
	}
	return st, nil
}

// GetByID returns the status by ID, or ErrNotFound.
func (svc *StatusService) GetByID(ctx context.Context, id string) (*domain.Status, error) {
	st, err := svc.store.GetStatusByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("GetStatusByID(%s): %w", id, err)
	}
	return st, nil
}

// GetByIDEnriched returns the status with author, mentions, tags, and media for API response.
func (svc *StatusService) GetByIDEnriched(ctx context.Context, id string) (CreateResult, error) {
	st, err := svc.store.GetStatusByID(ctx, id)
	if err != nil {
		return CreateResult{}, fmt.Errorf("GetStatusByID(%s): %w", id, err)
	}
	if st.DeletedAt != nil {
		return CreateResult{}, fmt.Errorf("GetByIDEnriched(%s): %w", id, domain.ErrNotFound)
	}
	author, err := svc.store.GetAccountByID(ctx, st.AccountID)
	if err != nil {
		return CreateResult{}, fmt.Errorf("GetAccountByID: %w", err)
	}
	mentions, err := svc.store.GetStatusMentions(ctx, st.ID)
	if err != nil {
		return CreateResult{}, fmt.Errorf("GetStatusMentions: %w", err)
	}
	tags, err := svc.store.GetStatusHashtags(ctx, st.ID)
	if err != nil {
		return CreateResult{}, fmt.Errorf("GetStatusHashtags: %w", err)
	}
	media, err := svc.store.GetStatusAttachments(ctx, st.ID)
	if err != nil {
		return CreateResult{}, fmt.Errorf("GetStatusAttachments: %w", err)
	}
	return CreateResult{
		Status:   st,
		Author:   author,
		Mentions: mentions,
		Tags:     tags,
		Media:    media,
	}, nil
}

// CreateWithContent creates a status from plain text: validates, renders content (mentions, hashtags),
// persists status with mentions, hashtags, and mention notifications in one transaction,
// then loads author, mentions, tags, and media for the response.
func (svc *StatusService) CreateWithContent(ctx context.Context, in CreateWithContentInput) (CreateResult, error) {
	text := strings.TrimSpace(in.Text)
	if text == "" {
		return CreateResult{}, fmt.Errorf("CreateWithContent: %w", domain.ErrValidation)
	}
	visibility := resolveVisibilityService(in.Visibility, in.DefaultVisibility)
	if CountStatusCharacters(text) > svc.maxStatusChars {
		return CreateResult{}, fmt.Errorf("CreateWithContent: %w", domain.ErrValidation)
	}
	resolver := svc.mentionResolver(ctx)
	renderResult, err := Render(text, svc.instanceDomain, resolver)
	if err != nil {
		return CreateResult{}, fmt.Errorf("CreateWithContent Render: %w", err)
	}
	// TODO: this should be a setting
	language := in.Language
	if language == "" {
		language = "en"
	}
	statusID := uid.New()
	statusURI := fmt.Sprintf("%s/users/%s/statuses/%s", svc.instanceBaseURL, in.Username, statusID)

	var created *domain.Status
	err = svc.store.WithTx(ctx, func(tx store.Store) error {
		var txErr error
		created, txErr = createStatusWithContentTx(ctx, tx, in.AccountID, in.Username, statusID, statusURI, visibility, text, renderResult.HTML, in.ContentWarning, language, in.Sensitive, renderResult)
		return txErr
	})
	if err != nil {
		return CreateResult{}, fmt.Errorf("CreateWithContent: %w", err)
	}

	author, err := svc.store.GetAccountByID(ctx, in.AccountID)
	if err != nil {
		return CreateResult{}, fmt.Errorf("CreateWithContent GetAccountByID: %w", err)
	}
	mentions, err := svc.store.GetStatusMentions(ctx, statusID)
	if err != nil {
		return CreateResult{}, fmt.Errorf("CreateWithContent GetStatusMentions: %w", err)
	}
	tags, err := svc.store.GetStatusHashtags(ctx, statusID)
	if err != nil {
		return CreateResult{}, fmt.Errorf("CreateWithContent GetStatusHashtags: %w", err)
	}
	media, err := svc.store.GetStatusAttachments(ctx, statusID)
	if err != nil {
		return CreateResult{}, fmt.Errorf("CreateWithContent GetStatusAttachments: %w", err)
	}
	if err := svc.fed.PublishStatus(ctx, created); err != nil && svc.logger != nil {
		svc.logger.WarnContext(ctx, "federation publish failed after CreateWithContent", slog.Any("error", err), slog.String("status_id", created.ID))
	}
	return CreateResult{
		Status:   created,
		Author:   author,
		Mentions: mentions,
		Tags:     tags,
		Media:    media,
	}, nil
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

func (svc *StatusService) mentionResolver(ctx context.Context) MentionResolver {
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
) (*domain.Status, error) {
	st, err := tx.CreateStatus(ctx, store.CreateStatusInput{
		ID:             statusID,
		URI:            statusURI,
		AccountID:      accountID,
		Text:           &text,
		Content:        &content,
		ContentWarning: &contentWarning,
		Visibility:     visibility,
		Language:       &language,
		Sensitive:      sensitive,
		Local:          true,
		APID:           statusURI,
		ApRaw:          nil,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateStatus: %w", err)
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
	for _, m := range renderResult.Mentions {
		mentioned, _ := tx.GetAccountByID(ctx, m.AccountID)
		if mentioned == nil || (mentioned.Domain != nil && *mentioned.Domain != "") {
			continue
		}
		notifID := uid.New()
		_, err := tx.CreateNotification(ctx, store.CreateNotificationInput{
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

// Delete soft-deletes the status and decrements the account's statuses count atomically.
func (svc *StatusService) Delete(ctx context.Context, id string) error {
	st, err := svc.store.GetStatusByID(ctx, id)
	if err != nil {
		return fmt.Errorf("Delete(%s): %w", id, err)
	}
	err = svc.store.WithTx(ctx, func(tx store.Store) error {
		if err := tx.DeleteStatus(ctx, id); err != nil {
			return fmt.Errorf("DeleteStatus: %w", err)
		}
		return tx.DecrementStatusesCount(ctx, st.AccountID)
	})
	if err != nil {
		return fmt.Errorf("Delete(%s): %w", id, err)
	}
	if err := svc.fed.DeleteStatus(ctx, st); err != nil && svc.logger != nil {
		svc.logger.WarnContext(ctx, "federation publish failed after status delete", slog.Any("error", err), slog.String("status_id", st.ID))
	}
	return nil
}
