package activitypub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/chairswithlegs/monstera-fed/internal/cache"
	"github.com/chairswithlegs/monstera-fed/internal/config"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	"github.com/chairswithlegs/monstera-fed/internal/uid"
)

const (
	actorTypeService = "Service"
	objectTypeNote   = "Note"
)

// ErrFatal represent an inbox error that should not be retried.
var ErrFatal = errors.New("fatal inbox error")

// EventPublisher publishes SSE events to connected clients.
type EventPublisher interface {
	PublishStatusEvent(ctx context.Context, accountID, eventType string, payload json.RawMessage) error
	PublishNotificationEvent(ctx context.Context, accountID string, payload json.RawMessage) error
}

// AcceptFollowSender sends an Accept{Follow} activity to a remote inbox (e.g. via NATS).
// If nil, the inbox processor does not send accepts.
type AcceptFollowSender interface {
	SendAcceptFollow(ctx context.Context, target, actor *domain.Account, followID string) error
}

// InboxProcessor dispatches verified incoming AP activities to type-specific handlers.
type InboxProcessor struct {
	store      store.Store
	cache      cache.Store
	blocklist  *BlocklistCache
	events     EventPublisher
	acceptSend AcceptFollowSender
	cfg        *config.Config
	logger     *slog.Logger
	actorFetch func(ctx context.Context, actorIRI string) (*Actor, error)
}

// NewInboxProcessor constructs an InboxProcessor.
func NewInboxProcessor(
	s store.Store,
	c cache.Store,
	bl *BlocklistCache,
	events EventPublisher,
	acceptSend AcceptFollowSender,
	cfg *config.Config,
	logger *slog.Logger,
	actorFetch func(ctx context.Context, actorIRI string) (*Actor, error),
) *InboxProcessor {
	return &InboxProcessor{
		store:      s,
		cache:      c,
		blocklist:  bl,
		events:     events,
		acceptSend: acceptSend,
		cfg:        cfg,
		logger:     logger,
		actorFetch: actorFetch,
	}
}

// Process dispatches a verified incoming activity to the appropriate handler.
func (p *InboxProcessor) Process(ctx context.Context, activity *Activity) error {
	actorDomain := DomainFromActorID(activity.Actor)
	if actorDomain == "" {
		return fmt.Errorf("%w: cannot extract domain from actor %q", ErrFatal, activity.Actor)
	}
	if p.blocklist.IsSuspended(ctx, actorDomain) {
		p.logger.Debug("inbox: dropped activity from suspended domain",
			slog.String("domain", actorDomain), slog.String("type", activity.Type), slog.String("id", activity.ID))
		return nil
	}
	switch activity.Type {
	case "Follow":
		return p.handleFollow(ctx, activity)
	case "Accept":
		return p.handleAcceptFollow(ctx, activity)
	case "Reject":
		return p.handleRejectFollow(ctx, activity)
	case "Undo":
		return p.handleUndo(ctx, activity)
	case "Create":
		return p.handleCreate(ctx, activity, actorDomain)
	case "Announce":
		return p.handleAnnounce(ctx, activity, actorDomain)
	case "Like":
		return p.handleLike(ctx, activity)
	case "Delete":
		return p.handleDelete(ctx, activity)
	case "Update":
		return p.handleUpdate(ctx, activity)
	case "Block":
		return p.handleBlock(ctx, activity)
	default:
		p.logger.Debug("inbox: unsupported activity type", slog.String("type", activity.Type), slog.String("id", activity.ID))
		return nil
	}
}

func usernameFromActorIRI(actorIRI, instanceDomain string) string {
	if instanceDomain == "" {
		return ""
	}
	prefix := "https://" + instanceDomain + "/users/"
	if !strings.HasPrefix(actorIRI, prefix) {
		return ""
	}
	suffix := strings.TrimPrefix(actorIRI, prefix)
	if idx := strings.Index(suffix, "/"); idx >= 0 {
		suffix = suffix[:idx]
	}
	return suffix
}

func (p *InboxProcessor) resolveRemoteAccount(ctx context.Context, actorIRI string) (*domain.Account, error) {
	existing, err := p.store.GetAccountByAPID(ctx, actorIRI)
	if err == nil {
		return existing, nil
	}
	dom := DomainFromActorID(actorIRI)
	username := usernameFromActorIRI(actorIRI, "")
	if username == "" {
		username = "unknown"
	}
	if p.actorFetch != nil {
		actor, err := p.actorFetch(ctx, actorIRI)
		if err == nil {
			return p.syncRemoteActorFromDoc(ctx, actor)
		}
	}
	in := store.CreateAccountInput{
		ID:           uid.New(),
		Username:     username,
		Domain:       &dom,
		PublicKey:    "",
		InboxURL:     actorIRI + "/inbox",
		OutboxURL:    actorIRI + "/outbox",
		FollowersURL: actorIRI + "/followers",
		FollowingURL: actorIRI + "/following",
		APID:         actorIRI,
	}
	acc, err := p.store.CreateAccount(ctx, in)
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			existing, getErr := p.store.GetAccountByAPID(ctx, actorIRI)
			if getErr != nil {
				return nil, fmt.Errorf("CreateAccount conflict GetAccountByAPID: %w", getErr)
			}
			return existing, nil
		}
		return nil, fmt.Errorf("resolveRemoteAccount CreateAccount: %w", err)
	}
	return acc, nil
}

func (p *InboxProcessor) syncRemoteActorFromDoc(ctx context.Context, actor *Actor) (*domain.Account, error) {
	existing, err := p.store.GetAccountByAPID(ctx, actor.ID)
	if err != nil {
		username := usernameFromActorIRI(actor.ID, "")
		if username == "" {
			username = "unknown"
		}
		dom := DomainFromActorID(actor.ID)
		apRaw, _ := json.Marshal(actor)
		in := store.CreateAccountInput{
			ID:           uid.New(),
			Username:     username,
			Domain:       &dom,
			DisplayName:  strPtr(actor.Name),
			Note:         strPtr(actor.Summary),
			PublicKey:    actor.PublicKey.PublicKeyPem,
			InboxURL:     actor.Inbox,
			OutboxURL:    actor.Outbox,
			FollowersURL: actor.Followers,
			FollowingURL: actor.Following,
			APID:         actor.ID,
			ApRaw:        apRaw,
			Bot:          actor.Type == actorTypeService,
			Locked:       actor.ManuallyApprovesFollowers,
		}
		acc, createErr := p.store.CreateAccount(ctx, in)
		if createErr != nil {
			return nil, fmt.Errorf("syncRemoteActorFromDoc CreateAccount: %w", createErr)
		}
		return acc, nil
	}
	apRaw, _ := json.Marshal(actor)
	_ = p.store.UpdateAccount(ctx, store.UpdateAccountInput{
		ID:          existing.ID,
		DisplayName: strPtr(actor.Name),
		Note:        strPtr(actor.Summary),
		APRaw:       apRaw,
		Bot:         actor.Type == actorTypeService,
		Locked:      actor.ManuallyApprovesFollowers,
	})
	if actor.PublicKey.PublicKeyPem != "" && actor.PublicKey.PublicKeyPem != existing.PublicKey {
		_ = p.store.UpdateAccountKeys(ctx, existing.ID, actor.PublicKey.PublicKeyPem, apRaw)
	}
	acc, getErr := p.store.GetAccountByAPID(ctx, actor.ID)
	if getErr != nil {
		return nil, fmt.Errorf("syncRemoteActorFromDoc GetAccountByAPID: %w", getErr)
	}
	return acc, nil
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func isUniqueViolation(err error) bool {
	return errors.Is(err, domain.ErrConflict)
}

func (p *InboxProcessor) createNotification(ctx context.Context, accountID, fromID, notifType string, statusID *string) {
	in := store.CreateNotificationInput{
		ID:        uid.New(),
		AccountID: accountID,
		FromID:    fromID,
		Type:      notifType,
		StatusID:  statusID,
	}
	_, _ = p.store.CreateNotification(ctx, in)
}

func (p *InboxProcessor) handleFollow(ctx context.Context, activity *Activity) error {
	targetID, ok := activity.ObjectID()
	if !ok {
		return fmt.Errorf("%w: follow object is not an actor IRI", ErrFatal)
	}
	targetUsername := usernameFromActorIRI(targetID, p.cfg.InstanceDomain)
	if targetUsername == "" {
		return fmt.Errorf("%w: follow target %q is not a local user", ErrFatal, targetID)
	}
	target, err := p.store.GetLocalAccountByUsername(ctx, targetUsername)
	if err != nil {
		return fmt.Errorf("%w: follow target not found: %s", ErrFatal, targetUsername)
	}
	actor, err := p.resolveRemoteAccount(ctx, activity.Actor)
	if err != nil {
		return fmt.Errorf("inbox: resolve actor %q: %w", activity.Actor, err)
	}
	if activity.ID != "" {
		existing, _ := p.store.GetFollowByAPID(ctx, activity.ID)
		if existing != nil {
			p.logger.Debug("inbox: duplicate Follow ignored", slog.String("ap_id", activity.ID))
			return nil
		}
	}
	state := domain.FollowStateAccepted
	notifType := "follow"
	if target.Locked {
		state = domain.FollowStatePending
		notifType = "follow_request"
	}
	var apID *string
	if activity.ID != "" {
		apID = &activity.ID
	}
	follow, err := p.store.CreateFollow(ctx, store.CreateFollowInput{
		ID:        uid.New(),
		AccountID: actor.ID,
		TargetID:  target.ID,
		State:     state,
		APID:      apID,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil
		}
		return fmt.Errorf("inbox: create follow: %w", err)
	}
	p.createNotification(ctx, target.ID, actor.ID, notifType, nil)
	if state == domain.FollowStateAccepted && p.acceptSend != nil {
		_ = p.acceptSend.SendAcceptFollow(ctx, target, actor, follow.ID)
	}
	return nil
}

func (p *InboxProcessor) handleAcceptFollow(ctx context.Context, activity *Activity) error {
	inner, err := activity.ObjectActivity()
	if err != nil {
		objectID, ok := activity.ObjectID()
		if !ok {
			return fmt.Errorf("%w: accept object is not a follow activity or IRI", ErrFatal)
		}
		follow, err := p.store.GetFollowByAPID(ctx, objectID)
		if err != nil {
			return fmt.Errorf("%w: accept: follow not found for ap_id %q", ErrFatal, objectID)
		}
		if acceptErr := p.store.AcceptFollow(ctx, follow.ID); acceptErr != nil {
			return fmt.Errorf("inbox: AcceptFollow by objectID: %w", acceptErr)
		}
		return nil
	}
	if inner.ID != "" {
		follow, err := p.store.GetFollowByAPID(ctx, inner.ID)
		if err == nil {
			if acceptErr := p.store.AcceptFollow(ctx, follow.ID); acceptErr != nil {
				return fmt.Errorf("inbox: AcceptFollow by inner.ID: %w", acceptErr)
			}
			return nil
		}
	}
	actorAccount, err := p.store.GetAccountByAPID(ctx, inner.Actor)
	if err != nil {
		return fmt.Errorf("%w: accept: actor not found %q", ErrFatal, inner.Actor)
	}
	targetID, _ := inner.ObjectID()
	targetAccount, err := p.store.GetAccountByAPID(ctx, targetID)
	if err != nil {
		return fmt.Errorf("%w: accept: target not found %q", ErrFatal, targetID)
	}
	follow, err := p.store.GetFollow(ctx, actorAccount.ID, targetAccount.ID)
	if err != nil {
		return fmt.Errorf("%w: accept: follow relationship not found", ErrFatal)
	}
	if acceptErr := p.store.AcceptFollow(ctx, follow.ID); acceptErr != nil {
		return fmt.Errorf("inbox: AcceptFollow: %w", acceptErr)
	}
	return nil
}

func (p *InboxProcessor) handleRejectFollow(ctx context.Context, activity *Activity) error {
	inner, err := activity.ObjectActivity()
	if err != nil {
		objectID, ok := activity.ObjectID()
		if !ok {
			return fmt.Errorf("%w: reject object is not a follow activity or IRI", ErrFatal)
		}
		follow, err := p.store.GetFollowByAPID(ctx, objectID)
		if err != nil {
			return fmt.Errorf("inbox: GetFollowByAPID for reject: %w", err)
		}
		if delErr := p.store.DeleteFollow(ctx, follow.AccountID, follow.TargetID); delErr != nil {
			return fmt.Errorf("inbox: DeleteFollow (reject by objectID): %w", delErr)
		}
		return nil
	}
	if inner.ID != "" {
		follow, err := p.store.GetFollowByAPID(ctx, inner.ID)
		if err == nil {
			if delErr := p.store.DeleteFollow(ctx, follow.AccountID, follow.TargetID); delErr != nil {
				return fmt.Errorf("inbox: DeleteFollow (reject by inner.ID): %w", delErr)
			}
			return nil
		}
	}
	actorAccount, _ := p.store.GetAccountByAPID(ctx, inner.Actor)
	targetID, _ := inner.ObjectID()
	targetAccount, _ := p.store.GetAccountByAPID(ctx, targetID)
	if actorAccount != nil && targetAccount != nil {
		_ = p.store.DeleteFollow(ctx, actorAccount.ID, targetAccount.ID)
	}
	return nil
}

func (p *InboxProcessor) handleUndo(ctx context.Context, activity *Activity) error {
	innerType := activity.ObjectType()
	switch innerType {
	case "Follow":
		return p.handleUndoFollow(ctx, activity)
	case "Like":
		return p.handleUndoLike(ctx, activity)
	case "Announce":
		return p.handleUndoAnnounce(ctx, activity)
	default:
		if objectID, ok := activity.ObjectID(); ok {
			if follow, err := p.store.GetFollowByAPID(ctx, objectID); err == nil {
				if delErr := p.store.DeleteFollow(ctx, follow.AccountID, follow.TargetID); delErr != nil {
					return fmt.Errorf("inbox: DeleteFollow (Undo default): %w", delErr)
				}
				return nil
			}
			if fav, err := p.store.GetFavouriteByAPID(ctx, objectID); err == nil {
				return p.undoFavourite(ctx, fav)
			}
		}
		p.logger.Debug("inbox: unsupported Undo object type", slog.String("type", innerType), slog.String("id", activity.ID))
		return nil
	}
}

func (p *InboxProcessor) handleUndoFollow(ctx context.Context, activity *Activity) error {
	inner, err := activity.ObjectActivity()
	if err != nil {
		return fmt.Errorf("%w: undo{Follow} object is not a follow activity", ErrFatal)
	}
	if inner.ID != "" {
		follow, err := p.store.GetFollowByAPID(ctx, inner.ID)
		if err == nil {
			if delErr := p.store.DeleteFollow(ctx, follow.AccountID, follow.TargetID); delErr != nil {
				return fmt.Errorf("inbox: DeleteFollow (UndoFollow): %w", delErr)
			}
			return nil
		}
	}
	actorAccount, err := p.store.GetAccountByAPID(ctx, inner.Actor)
	if err != nil {
		return fmt.Errorf("inbox: GetAccountByAPID actor (UndoFollow): %w", err)
	}
	targetID, _ := inner.ObjectID()
	targetAccount, err := p.store.GetAccountByAPID(ctx, targetID)
	if err != nil {
		return fmt.Errorf("inbox: GetAccountByAPID target (UndoFollow): %w", err)
	}
	if delErr := p.store.DeleteFollow(ctx, actorAccount.ID, targetAccount.ID); delErr != nil {
		return fmt.Errorf("inbox: DeleteFollow (UndoFollow): %w", delErr)
	}
	return nil
}

func (p *InboxProcessor) undoFavourite(ctx context.Context, fav *domain.Favourite) error {
	if err := p.store.DeleteFavourite(ctx, fav.AccountID, fav.StatusID); err != nil {
		return fmt.Errorf("inbox: DeleteFavourite: %w", err)
	}
	if err := p.store.DecrementFavouritesCount(ctx, fav.StatusID); err != nil {
		return fmt.Errorf("inbox: DecrementFavouritesCount: %w", err)
	}
	return nil
}

func (p *InboxProcessor) handleUndoLike(ctx context.Context, activity *Activity) error {
	inner, err := activity.ObjectActivity()
	if err != nil {
		return fmt.Errorf("%w: undo{Like} object is not a like activity", ErrFatal)
	}
	if inner.ID != "" {
		if fav, err := p.store.GetFavouriteByAPID(ctx, inner.ID); err == nil {
			return p.undoFavourite(ctx, fav)
		}
	}
	actorAccount, err := p.store.GetAccountByAPID(ctx, inner.Actor)
	if err != nil {
		return fmt.Errorf("inbox: GetAccountByAPID (UndoLike): %w", err)
	}
	objectID, _ := inner.ObjectID()
	status, err := p.store.GetStatusByAPID(ctx, objectID)
	if err != nil {
		return fmt.Errorf("inbox: GetStatusByAPID (UndoLike): %w", err)
	}
	fav, err := p.store.GetFavouriteByAccountAndStatus(ctx, actorAccount.ID, status.ID)
	if err != nil {
		return fmt.Errorf("inbox: GetFavouriteByAccountAndStatus (UndoLike): %w", err)
	}
	return p.undoFavourite(ctx, fav)
}

func (p *InboxProcessor) handleUndoAnnounce(ctx context.Context, activity *Activity) error {
	inner, err := activity.ObjectActivity()
	if err != nil {
		return fmt.Errorf("%w: undo{Announce} object is not an announce activity", ErrFatal)
	}
	if inner.ID != "" {
		boost, err := p.store.GetStatusByAPID(ctx, inner.ID)
		if err == nil && boost.ReblogOfID != nil {
			if err := p.store.SoftDeleteStatus(ctx, boost.ID); err != nil {
				return fmt.Errorf("inbox: SoftDeleteStatus (UndoAnnounce): %w", err)
			}
			if err := p.store.DecrementReblogsCount(ctx, *boost.ReblogOfID); err != nil {
				return fmt.Errorf("inbox: DecrementReblogsCount (UndoAnnounce): %w", err)
			}
			return nil
		}
	}
	actorAccount, err := p.store.GetAccountByAPID(ctx, inner.Actor)
	if err != nil {
		return fmt.Errorf("inbox: GetAccountByAPID (UndoAnnounce): %w", err)
	}
	objectID, _ := inner.ObjectID()
	originalStatus, err := p.store.GetStatusByAPID(ctx, objectID)
	if err != nil {
		return fmt.Errorf("inbox: GetStatusByAPID (UndoAnnounce): %w", err)
	}
	boost, err := p.store.GetReblogByAccountAndTarget(ctx, actorAccount.ID, originalStatus.ID)
	if err != nil {
		return fmt.Errorf("inbox: GetReblogByAccountAndTarget (UndoAnnounce): %w", err)
	}
	if err := p.store.SoftDeleteStatus(ctx, boost.ID); err != nil {
		return fmt.Errorf("inbox: SoftDeleteStatus (UndoAnnounce): %w", err)
	}
	if err := p.store.DecrementReblogsCount(ctx, originalStatus.ID); err != nil {
		return fmt.Errorf("inbox: DecrementReblogsCount (UndoAnnounce): %w", err)
	}
	return nil
}

func (p *InboxProcessor) resolveVisibility(note *Note, author *domain.Account) string {
	for _, addr := range note.To {
		if addr == PublicAddress {
			return domain.VisibilityPublic
		}
	}
	for _, addr := range note.Cc {
		if addr == PublicAddress {
			return domain.VisibilityUnlisted
		}
	}
	if author != nil {
		for _, addr := range note.To {
			if addr == author.FollowersURL {
				return domain.VisibilityPrivate
			}
		}
	}
	return domain.VisibilityDirect
}

func (p *InboxProcessor) hasLocalFollower(ctx context.Context, remoteAccountID string) (bool, error) {
	followers, err := p.store.GetFollowers(ctx, remoteAccountID, nil, 1)
	if err != nil {
		return false, fmt.Errorf("GetFollowers: %w", err)
	}
	for i := range followers {
		if followers[i].Domain == nil {
			return true, nil
		}
	}
	return false, nil
}

func (p *InboxProcessor) hasLocalRecipient(to []string) bool {
	for _, addr := range to {
		if usernameFromActorIRI(addr, p.cfg.InstanceDomain) != "" {
			return true
		}
	}
	return false
}

func noteLanguage(note *Note) *string {
	if len(note.ContentMap) == 0 {
		return nil
	}
	for k := range note.ContentMap {
		return &k
	}
	return nil
}

func (p *InboxProcessor) storeRemoteMedia(ctx context.Context, attachments []Attachment, accountID string) []string {
	var ids []string
	for _, att := range attachments {
		if att.URL == "" {
			continue
		}
		in := store.CreateMediaAttachmentInput{
			ID:         uid.New(),
			AccountID:  accountID,
			Type:       "image",
			StorageKey: "",
			URL:        att.URL,
			RemoteURL:  &att.URL,
		}
		m, err := p.store.CreateMediaAttachment(ctx, in)
		if err != nil {
			continue
		}
		ids = append(ids, m.ID)
	}
	return ids
}

func (p *InboxProcessor) processMentionNotifications(ctx context.Context, tags []Tag, statusID, fromID string) {
	for _, tag := range tags {
		if tag.Type != "Mention" || tag.Href == "" {
			continue
		}
		username := usernameFromActorIRI(tag.Href, p.cfg.InstanceDomain)
		if username == "" {
			continue
		}
		acc, err := p.store.GetLocalAccountByUsername(ctx, username)
		if err != nil {
			continue
		}
		p.createNotification(ctx, acc.ID, fromID, "mention", &statusID)
	}
}

func (p *InboxProcessor) handleCreate(ctx context.Context, activity *Activity, _ string) error {
	note, err := activity.ObjectNote()
	if err != nil {
		return fmt.Errorf("%w: create object is not a note: %w", ErrFatal, err)
	}
	if note.Type != objectTypeNote {
		return fmt.Errorf("%w: create object type %q is not supported", ErrFatal, note.Type)
	}
	if note.ID != "" {
		if _, err := p.store.GetStatusByAPID(ctx, note.ID); err == nil {
			return nil
		}
	}
	author, err := p.resolveRemoteAccount(ctx, activity.Actor)
	if err != nil {
		return fmt.Errorf("inbox: resolve actor %q: %w", activity.Actor, err)
	}
	visibility := p.resolveVisibility(note, author)
	if visibility == domain.VisibilityPrivate {
		hasLocal, err := p.hasLocalFollower(ctx, author.ID)
		if err != nil {
			return err
		}
		if !hasLocal {
			return nil
		}
	}
	if visibility == domain.VisibilityDirect {
		if !p.hasLocalRecipient(note.To) {
			return nil
		}
	}
	var inReplyToID *string
	if note.InReplyTo != nil && *note.InReplyTo != "" {
		if parent, err := p.store.GetStatusByAPID(ctx, *note.InReplyTo); err == nil {
			inReplyToID = &parent.ID
		}
	}
	mediaIDs := p.storeRemoteMedia(ctx, note.Attachment, author.ID)
	var contentWarning *string
	if note.Summary != nil && *note.Summary != "" {
		contentWarning = note.Summary
	}
	apRaw, _ := json.Marshal(note)
	content := note.Content
	in := store.CreateStatusInput{
		ID:             uid.New(),
		URI:            note.ID,
		AccountID:      author.ID,
		Text:           &content,
		Content:        &content,
		ContentWarning: contentWarning,
		Visibility:     visibility,
		Language:       noteLanguage(note),
		InReplyToID:    inReplyToID,
		APID:           note.ID,
		ApRaw:          apRaw,
		Sensitive:      note.Sensitive,
		Local:          false,
	}
	status, err := p.store.CreateStatus(ctx, in)
	if err != nil {
		if isUniqueViolation(err) {
			return nil
		}
		return fmt.Errorf("inbox: create status: %w", err)
	}
	for _, mediaID := range mediaIDs {
		_ = p.store.AttachMediaToStatus(ctx, mediaID, status.ID, author.ID)
	}
	if inReplyToID != nil {
		_ = p.store.IncrementRepliesCount(ctx, *inReplyToID)
	}
	p.processMentionNotifications(ctx, note.Tag, status.ID, author.ID)
	return nil
}

func (p *InboxProcessor) handleAnnounce(ctx context.Context, activity *Activity, _ string) error {
	if activity.ID != "" {
		if _, err := p.store.GetStatusByAPID(ctx, activity.ID); err == nil {
			return nil
		}
	}
	objectID, ok := activity.ObjectID()
	if !ok {
		return fmt.Errorf("%w: announce object is not a status IRI", ErrFatal)
	}
	original, err := p.store.GetStatusByAPID(ctx, objectID)
	if err != nil {
		p.logger.Debug("inbox: Announce of unknown status", slog.String("object", objectID))
		return fmt.Errorf("inbox: GetStatusByAPID (Announce): %w", err)
	}
	actor, err := p.resolveRemoteAccount(ctx, activity.Actor)
	if err != nil {
		return fmt.Errorf("inbox: resolve actor %q: %w", activity.Actor, err)
	}
	apRaw, _ := json.Marshal(activity)
	reblogOfID := original.ID
	in := store.CreateStatusInput{
		ID:         uid.New(),
		URI:        activity.ID,
		AccountID:  actor.ID,
		Visibility: domain.VisibilityPublic,
		ReblogOfID: &reblogOfID,
		APID:       activity.ID,
		ApRaw:      apRaw,
		Local:      false,
	}
	_, err = p.store.CreateStatus(ctx, in)
	if err != nil {
		if isUniqueViolation(err) {
			return nil
		}
		return fmt.Errorf("inbox: create boost: %w", err)
	}
	_ = p.store.IncrementReblogsCount(ctx, original.ID)
	if original.Local {
		p.createNotification(ctx, original.AccountID, actor.ID, "reblog", &original.ID)
	}
	return nil
}

func (p *InboxProcessor) handleLike(ctx context.Context, activity *Activity) error {
	objectID, ok := activity.ObjectID()
	if !ok {
		return fmt.Errorf("%w: like object is not a status IRI", ErrFatal)
	}
	status, err := p.store.GetStatusByAPID(ctx, objectID)
	if err != nil {
		p.logger.Debug("inbox: Like of unknown status", slog.String("object", objectID))
		return fmt.Errorf("inbox: GetStatusByAPID (Like): %w", err)
	}
	actor, err := p.resolveRemoteAccount(ctx, activity.Actor)
	if err != nil {
		return fmt.Errorf("inbox: resolve actor %q: %w", activity.Actor, err)
	}
	var apID *string
	if activity.ID != "" {
		apID = &activity.ID
	}
	_, err = p.store.CreateFavourite(ctx, store.CreateFavouriteInput{
		ID:        uid.New(),
		AccountID: actor.ID,
		StatusID:  status.ID,
		APID:      apID,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil
		}
		return fmt.Errorf("inbox: create favourite: %w", err)
	}
	_ = p.store.IncrementFavouritesCount(ctx, status.ID)
	if status.Local {
		p.createNotification(ctx, status.AccountID, actor.ID, "favourite", &status.ID)
	}
	return nil
}

func (p *InboxProcessor) handleDelete(ctx context.Context, activity *Activity) error {
	objectType := activity.ObjectType()
	switch objectType {
	case "Tombstone", objectTypeNote, "":
		objectID, ok := activity.ObjectID()
		if !ok {
			var obj struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(activity.ObjectRaw, &obj); err != nil {
				return fmt.Errorf("%w: delete: cannot extract object ID", ErrFatal)
			}
			objectID = obj.ID
		}
		if objectID == "" {
			return nil
		}
		status, err := p.store.GetStatusByAPID(ctx, objectID)
		if err != nil {
			return fmt.Errorf("inbox: GetStatusByAPID (Delete): %w", err)
		}
		statusAuthor, _ := p.store.GetAccountByID(ctx, status.AccountID)
		if statusAuthor != nil && statusAuthor.APID != activity.Actor {
			return fmt.Errorf("%w: delete: actor %q is not the author", ErrFatal, activity.Actor)
		}
		if delErr := p.store.SoftDeleteStatus(ctx, status.ID); delErr != nil {
			return fmt.Errorf("inbox: SoftDeleteStatus (Delete): %w", delErr)
		}
		return nil
	case "Person":
		account, err := p.store.GetAccountByAPID(ctx, activity.Actor)
		if err != nil {
			return fmt.Errorf("inbox: GetAccountByAPID (Delete Person): %w", err)
		}
		if suspendErr := p.store.SuspendAccount(ctx, account.ID); suspendErr != nil {
			return fmt.Errorf("inbox: SuspendAccount: %w", suspendErr)
		}
		return nil
	default:
		p.logger.Debug("inbox: unsupported Delete object type", slog.String("type", objectType))
		return nil
	}
}

func (p *InboxProcessor) handleUpdate(ctx context.Context, activity *Activity) error {
	objectType := activity.ObjectType()
	switch objectType {
	case objectTypeNote:
		note, err := activity.ObjectNote()
		if err != nil {
			return fmt.Errorf("%w: update{Note}: %w", ErrFatal, err)
		}
		status, err := p.store.GetStatusByAPID(ctx, note.ID)
		if err != nil {
			return fmt.Errorf("inbox: GetStatusByAPID (Update Note): %w", err)
		}
		author, _ := p.store.GetAccountByID(ctx, status.AccountID)
		if author != nil && author.APID != activity.Actor {
			return fmt.Errorf("%w: update: actor is not the author", ErrFatal)
		}
		_ = p.store.CreateStatusEdit(ctx, store.CreateStatusEditInput{
			ID:             uid.New(),
			StatusID:       status.ID,
			AccountID:      status.AccountID,
			Text:           status.Text,
			Content:        status.Content,
			ContentWarning: status.ContentWarning,
			Sensitive:      status.Sensitive,
		})
		var cw *string
		if note.Summary != nil {
			cw = note.Summary
		}
		content := note.Content
		if updateErr := p.store.UpdateStatus(ctx, store.UpdateStatusInput{
			ID:             status.ID,
			Text:           &content,
			Content:        &content,
			ContentWarning: cw,
			Sensitive:      note.Sensitive,
		}); updateErr != nil {
			return fmt.Errorf("inbox: UpdateStatus: %w", updateErr)
		}
		return nil
	case "Person", actorTypeService:
		actor, err := activity.ObjectActor()
		if err != nil {
			return fmt.Errorf("%w: Update{Person}: %w", ErrFatal, err)
		}
		_, err = p.syncRemoteActorFromDoc(ctx, actor)
		return err
	default:
		p.logger.Debug("inbox: unsupported Update object type", slog.String("type", objectType))
		return nil
	}
}

func (p *InboxProcessor) handleBlock(ctx context.Context, activity *Activity) error {
	targetID, ok := activity.ObjectID()
	if !ok {
		return fmt.Errorf("%w: block object is not an actor IRI", ErrFatal)
	}
	targetUsername := usernameFromActorIRI(targetID, p.cfg.InstanceDomain)
	if targetUsername == "" {
		return nil
	}
	target, err := p.store.GetLocalAccountByUsername(ctx, targetUsername)
	if err != nil {
		return fmt.Errorf("inbox: GetLocalAccountByUsername (Block): %w", err)
	}
	actor, err := p.resolveRemoteAccount(ctx, activity.Actor)
	if err != nil {
		return fmt.Errorf("inbox: resolve actor: %w", err)
	}
	_ = p.store.CreateBlock(ctx, store.CreateBlockInput{
		ID:        uid.New(),
		AccountID: actor.ID,
		TargetID:  target.ID,
	})
	_ = p.store.DeleteFollow(ctx, actor.ID, target.ID)
	_ = p.store.DeleteFollow(ctx, target.ID, actor.ID)
	return nil
}

// DefaultActorFetch fetches an Actor document from the given IRI using HTTP GET.
func DefaultActorFetch(ctx context.Context, actorIRI string) (*Actor, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, actorIRI, nil)
	if err != nil {
		return nil, fmt.Errorf("actor fetch new request: %w", err)
	}
	req.Header.Set("Accept", "application/activity+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("actor fetch request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("actor fetch: status %d", resp.StatusCode)
	}
	var actor Actor
	if err := json.NewDecoder(resp.Body).Decode(&actor); err != nil {
		return nil, fmt.Errorf("actor fetch decode: %w", err)
	}
	return &actor, nil
}
