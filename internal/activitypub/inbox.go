package activitypub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/chairswithlegs/monstera-fed/internal/cache"
	"github.com/chairswithlegs/monstera-fed/internal/config"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/events"
	"github.com/chairswithlegs/monstera-fed/internal/service"
)

const (
	actorTypeService       = "Service"
	objectTypeNote         = "Note"
	defaultUsernameUnknown = "unknown"
)

// ErrFatal represent an inbox error that should not be retried.
var ErrFatal = errors.New("fatal inbox error")

// Inbox processes incoming ActivityPub activities.
type Inbox interface {
	Process(ctx context.Context, activity *Activity) error
}

// NewInbox constructs an Inbox.
// sseEvents and eventBus must be non-nil; pass the same publisher for both when SSE is enabled.
func NewInbox(
	accounts service.AccountService,
	follows service.FollowService,
	notifications service.NotificationService,
	statuses service.StatusService,
	media service.MediaService,
	remoteResolver *RemoteAccountResolver,
	c cache.Store,
	bl *BlocklistCache,
	outbox *Outbox,
	sseEvents InboxEventPublisher,
	eventBus events.EventBus,
	cfg *config.Config,
) Inbox {
	return &inbox{
		accounts:       accounts,
		follows:        follows,
		notifications:  notifications,
		statuses:       statuses,
		media:          media,
		remoteResolver: remoteResolver,
		cache:          c,
		blocklist:      bl,
		outbox:         outbox,
		sseEvents:      sseEvents,
		eventBus:       eventBus,
		cfg:            cfg,
	}
}

// inbox dispatches verified incoming ActivityPub activities to type-specific handlers.
// It also caches the actor documents and the blocklist for fast lookup.
type inbox struct {
	accounts       service.AccountService
	follows        service.FollowService
	notifications  service.NotificationService
	statuses       service.StatusService
	media          service.MediaService
	remoteResolver *RemoteAccountResolver
	cache          cache.Store
	blocklist      *BlocklistCache
	outbox         *Outbox
	sseEvents      InboxEventPublisher
	eventBus       events.EventBus
	cfg            *config.Config
}

// Process dispatches a verified incoming activity to the appropriate handler.
func (p *inbox) Process(ctx context.Context, activity *Activity) error {
	slog.DebugContext(ctx, "inbox: processing activity",
		slog.String("type", activity.Type), slog.String("id", activity.ID), slog.String("actor", activity.Actor))

	actorDomain := DomainFromActorID(activity.Actor)
	if actorDomain == "" {
		return fmt.Errorf("%w: cannot extract domain from actor %q", ErrFatal, activity.Actor)
	}
	if p.blocklist.IsSuspended(ctx, actorDomain) {
		slog.DebugContext(ctx, "inbox: dropped activity from suspended domain",
			slog.String("domain", actorDomain),
			slog.String("type", activity.Type),
			slog.String("id", activity.ID),
		)
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
		slog.DebugContext(ctx, "inbox: unsupported activity type", slog.String("type", activity.Type), slog.String("id", activity.ID))
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

func (p *inbox) resolveRemoteAccount(ctx context.Context, actorIRI string) (*domain.Account, error) {
	existing, err := p.accounts.GetByAPID(ctx, actorIRI)
	if err == nil {
		return existing, nil
	}
	dom := DomainFromActorID(actorIRI)
	username := usernameFromActorIRI(actorIRI, dom)
	if username == "" {
		username = defaultUsernameUnknown
	}
	actor, err := p.remoteResolver.FetchActor(ctx, actorIRI)
	if err == nil {
		return p.syncRemoteActorFromDoc(ctx, actor)
	}
	in := service.CreateOrUpdateRemoteInput{
		APID:         actorIRI,
		Username:     username,
		Domain:       dom,
		PublicKey:    "",
		InboxURL:     actorIRI + "/inbox",
		OutboxURL:    actorIRI + "/outbox",
		FollowersURL: actorIRI + "/followers",
		FollowingURL: actorIRI + "/following",
	}
	acc, err := p.accounts.CreateOrUpdateRemoteAccount(ctx, in)
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			existing, getErr := p.accounts.GetByAPID(ctx, actorIRI)
			if getErr != nil {
				return nil, fmt.Errorf("resolveRemoteAccount conflict GetByAPID: %w", getErr)
			}
			return existing, nil
		}
		return nil, fmt.Errorf("resolveRemoteAccount CreateOrUpdateRemoteAccount: %w", err)
	}
	return acc, nil
}

func (p *inbox) syncRemoteActorFromDoc(ctx context.Context, actor *Actor) (*domain.Account, error) {
	apRaw, _ := json.Marshal(actor)
	dom := DomainFromActorID(actor.ID)
	username := usernameFromActorIRI(actor.ID, dom)
	if username == "" {
		username = defaultUsernameUnknown
	}
	in := service.CreateOrUpdateRemoteInput{
		APID:         actor.ID,
		Username:     username,
		Domain:       dom,
		DisplayName:  &actor.Name,
		Note:         &actor.Summary,
		PublicKey:    actor.PublicKey.PublicKeyPem,
		InboxURL:     actor.Inbox,
		OutboxURL:    actor.Outbox,
		FollowersURL: actor.Followers,
		FollowingURL: actor.Following,
		Bot:          actor.Type == actorTypeService,
		Locked:       actor.ManuallyApprovesFollowers,
		ApRaw:        apRaw,
	}
	acc, err := p.accounts.CreateOrUpdateRemoteAccount(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("syncRemoteActorFromDoc: %w", err)
	}
	return acc, nil
}

func isUniqueViolation(err error) bool {
	return errors.Is(err, domain.ErrConflict)
}

func (p *inbox) createNotification(ctx context.Context, accountID, fromID, notifType string, statusID *string) {
	_ = p.notifications.Create(ctx, accountID, fromID, notifType, statusID)
}

func (p *inbox) createNotificationAndPublish(ctx context.Context, recipientID string, fromAccount *domain.Account, notifType string, statusID *string) {
	p.createNotification(ctx, recipientID, fromAccount.ID, notifType, statusID)
	list, _ := p.notifications.List(ctx, recipientID, nil, 1)
	if len(list) == 0 {
		return
	}
	n := &list[0]
	if n.FromID != fromAccount.ID || n.Type != notifType {
		return
	}
	if statusID != nil && (n.StatusID == nil || *n.StatusID != *statusID) {
		return
	}
	if statusID == nil && n.StatusID != nil {
		return
	}
	p.eventBus.PublishNotificationCreated(ctx, events.NotificationCreatedEvent{
		RecipientAccountID: recipientID,
		Notification:       n,
		FromAccount:        fromAccount,
		StatusID:           statusID,
	})
}

func (p *inbox) handleFollow(ctx context.Context, activity *Activity) error {
	targetID, ok := activity.ObjectID()
	if !ok {
		return fmt.Errorf("%w: follow object is not an actor IRI", ErrFatal)
	}
	targetUsername := usernameFromActorIRI(targetID, p.cfg.InstanceDomain)
	if targetUsername == "" {
		return fmt.Errorf("%w: follow target %q is not a local user", ErrFatal, targetID)
	}
	target, err := p.accounts.GetLocalByUsername(ctx, targetUsername)
	if err != nil {
		return fmt.Errorf("%w: follow target not found: %s", ErrFatal, targetUsername)
	}
	actor, err := p.resolveRemoteAccount(ctx, activity.Actor)
	if err != nil {
		return fmt.Errorf("inbox: resolve actor %q: %w", activity.Actor, err)
	}
	if activity.ID != "" {
		existing, _ := p.follows.GetFollowByAPID(ctx, activity.ID)
		if existing != nil {
			slog.Debug("inbox: duplicate Follow ignored", slog.String("ap_id", activity.ID))
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
	follow, err := p.follows.CreateFollowFromInbox(ctx, actor.ID, target.ID, state, apID)
	if err != nil {
		if isUniqueViolation(err) {
			return nil
		}
		return fmt.Errorf("inbox: create follow: %w", err)
	}
	p.createNotificationAndPublish(ctx, target.ID, actor, notifType, nil)
	if state == domain.FollowStateAccepted {
		_ = p.outbox.SendAcceptFollow(ctx, target, actor, follow.ID)
	}
	return nil
}

func (p *inbox) handleAcceptFollow(ctx context.Context, activity *Activity) error {
	inner, err := activity.ObjectActivity()
	if err != nil {
		objectID, ok := activity.ObjectID()
		if !ok {
			return fmt.Errorf("%w: accept object is not a follow activity or IRI", ErrFatal)
		}
		follow, err := p.follows.GetFollowByAPID(ctx, objectID)
		if err != nil {
			return fmt.Errorf("%w: accept: follow not found for ap_id %q", ErrFatal, objectID)
		}
		if acceptErr := p.follows.AcceptFollow(ctx, follow.ID); acceptErr != nil {
			return fmt.Errorf("inbox: AcceptFollow by objectID: %w", acceptErr)
		}
		return nil
	}
	if inner.ID != "" {
		follow, err := p.follows.GetFollowByAPID(ctx, inner.ID)
		if err == nil {
			if acceptErr := p.follows.AcceptFollow(ctx, follow.ID); acceptErr != nil {
				return fmt.Errorf("inbox: AcceptFollow by inner.ID: %w", acceptErr)
			}
			return nil
		}
	}
	actorAccount, err := p.accounts.GetByAPID(ctx, inner.Actor)
	if err != nil {
		return fmt.Errorf("%w: accept: actor not found %q", ErrFatal, inner.Actor)
	}
	targetID, _ := inner.ObjectID()
	targetAccount, err := p.accounts.GetByAPID(ctx, targetID)
	if err != nil {
		return fmt.Errorf("%w: accept: target not found %q", ErrFatal, targetID)
	}
	follow, err := p.follows.GetFollow(ctx, actorAccount.ID, targetAccount.ID)
	if err != nil {
		return fmt.Errorf("%w: accept: follow relationship not found", ErrFatal)
	}
	if acceptErr := p.follows.AcceptFollow(ctx, follow.ID); acceptErr != nil {
		return fmt.Errorf("inbox: AcceptFollow: %w", acceptErr)
	}
	return nil
}

func (p *inbox) handleRejectFollow(ctx context.Context, activity *Activity) error {
	inner, err := activity.ObjectActivity()
	if err != nil {
		objectID, ok := activity.ObjectID()
		if !ok {
			return fmt.Errorf("%w: reject object is not a follow activity or IRI", ErrFatal)
		}
		follow, err := p.follows.GetFollowByAPID(ctx, objectID)
		if err != nil {
			return fmt.Errorf("inbox: GetFollowByAPID for reject: %w", err)
		}
		if delErr := p.follows.DeleteFollowFromInbox(ctx, follow.AccountID, follow.TargetID); delErr != nil {
			return fmt.Errorf("inbox: DeleteFollow (reject by objectID): %w", delErr)
		}
		return nil
	}
	if inner.ID != "" {
		follow, err := p.follows.GetFollowByAPID(ctx, inner.ID)
		if err == nil {
			if delErr := p.follows.DeleteFollowFromInbox(ctx, follow.AccountID, follow.TargetID); delErr != nil {
				return fmt.Errorf("inbox: DeleteFollow (reject by inner.ID): %w", delErr)
			}
			return nil
		}
	}
	actorAccount, _ := p.accounts.GetByAPID(ctx, inner.Actor)
	targetID, _ := inner.ObjectID()
	targetAccount, _ := p.accounts.GetByAPID(ctx, targetID)
	if actorAccount != nil && targetAccount != nil {
		_ = p.follows.DeleteFollowFromInbox(ctx, actorAccount.ID, targetAccount.ID)
	}
	return nil
}

func (p *inbox) handleUndo(ctx context.Context, activity *Activity) error {
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
			if follow, err := p.follows.GetFollowByAPID(ctx, objectID); err == nil {
				if delErr := p.follows.DeleteFollowFromInbox(ctx, follow.AccountID, follow.TargetID); delErr != nil {
					return fmt.Errorf("inbox: DeleteFollow (Undo default): %w", delErr)
				}
				return nil
			}
			if fav, err := p.statuses.GetFavouriteByAPID(ctx, objectID); err == nil {
				return p.undoFavourite(ctx, fav)
			}
		}
		slog.Debug("inbox: unsupported Undo object type", slog.String("type", innerType), slog.String("id", activity.ID))
		return nil
	}
}

func (p *inbox) handleUndoFollow(ctx context.Context, activity *Activity) error {
	inner, err := activity.ObjectActivity()
	if err != nil {
		return fmt.Errorf("%w: undo{Follow} object is not a follow activity", ErrFatal)
	}
	if inner.ID != "" {
		follow, err := p.follows.GetFollowByAPID(ctx, inner.ID)
		if err == nil {
			if delErr := p.follows.DeleteFollowFromInbox(ctx, follow.AccountID, follow.TargetID); delErr != nil {
				return fmt.Errorf("inbox: DeleteFollow (UndoFollow): %w", delErr)
			}
			return nil
		}
	}
	actorAccount, err := p.accounts.GetByAPID(ctx, inner.Actor)
	if err != nil {
		return fmt.Errorf("inbox: GetAccountByAPID actor (UndoFollow): %w", err)
	}
	targetID, _ := inner.ObjectID()
	targetAccount, err := p.accounts.GetByAPID(ctx, targetID)
	if err != nil {
		return fmt.Errorf("inbox: GetAccountByAPID target (UndoFollow): %w", err)
	}
	if delErr := p.follows.DeleteFollowFromInbox(ctx, actorAccount.ID, targetAccount.ID); delErr != nil {
		return fmt.Errorf("inbox: DeleteFollow (UndoFollow): %w", delErr)
	}
	return nil
}

func (p *inbox) undoFavourite(ctx context.Context, fav *domain.Favourite) error {
	if err := p.statuses.DeleteFavourite(ctx, fav.AccountID, fav.StatusID); err != nil {
		return fmt.Errorf("inbox: DeleteFavourite: %w", err)
	}
	if err := p.statuses.DecrementFavouritesCount(ctx, fav.StatusID); err != nil {
		return fmt.Errorf("inbox: DecrementFavouritesCount: %w", err)
	}
	return nil
}

func (p *inbox) handleUndoLike(ctx context.Context, activity *Activity) error {
	inner, err := activity.ObjectActivity()
	if err != nil {
		return fmt.Errorf("%w: undo{Like} object is not a like activity", ErrFatal)
	}
	if inner.ID != "" {
		if fav, err := p.statuses.GetFavouriteByAPID(ctx, inner.ID); err == nil {
			return p.undoFavourite(ctx, fav)
		}
	}
	actorAccount, err := p.accounts.GetByAPID(ctx, inner.Actor)
	if err != nil {
		return fmt.Errorf("inbox: GetAccountByAPID (UndoLike): %w", err)
	}
	objectID, _ := inner.ObjectID()
	status, err := p.statuses.GetByAPID(ctx, objectID)
	if err != nil {
		return fmt.Errorf("inbox: GetStatusByAPID (UndoLike): %w", err)
	}
	fav, err := p.statuses.GetFavouriteByAccountAndStatus(ctx, actorAccount.ID, status.ID)
	if err != nil {
		return fmt.Errorf("inbox: GetFavouriteByAccountAndStatus (UndoLike): %w", err)
	}
	return p.undoFavourite(ctx, fav)
}

func (p *inbox) handleUndoAnnounce(ctx context.Context, activity *Activity) error {
	inner, err := activity.ObjectActivity()
	if err != nil {
		return fmt.Errorf("%w: undo{Announce} object is not an announce activity", ErrFatal)
	}
	if inner.ID != "" {
		boost, err := p.statuses.GetByAPID(ctx, inner.ID)
		if err == nil && boost.ReblogOfID != nil {
			if err := p.statuses.SoftDelete(ctx, boost.ID); err != nil {
				return fmt.Errorf("inbox: SoftDelete (UndoAnnounce): %w", err)
			}
			if err := p.statuses.DecrementReblogsCount(ctx, *boost.ReblogOfID); err != nil {
				return fmt.Errorf("inbox: DecrementReblogsCount (UndoAnnounce): %w", err)
			}
			return nil
		}
	}
	actorAccount, err := p.accounts.GetByAPID(ctx, inner.Actor)
	if err != nil {
		return fmt.Errorf("inbox: GetAccountByAPID (UndoAnnounce): %w", err)
	}
	objectID, _ := inner.ObjectID()
	originalStatus, err := p.statuses.GetByAPID(ctx, objectID)
	if err != nil {
		return fmt.Errorf("inbox: GetStatusByAPID (UndoAnnounce): %w", err)
	}
	boost, err := p.statuses.GetReblogByAccountAndTarget(ctx, actorAccount.ID, originalStatus.ID)
	if err != nil {
		return fmt.Errorf("inbox: GetReblogByAccountAndTarget (UndoAnnounce): %w", err)
	}
	if err := p.statuses.SoftDelete(ctx, boost.ID); err != nil {
		return fmt.Errorf("inbox: SoftDelete (UndoAnnounce): %w", err)
	}
	if err := p.statuses.DecrementReblogsCount(ctx, originalStatus.ID); err != nil {
		return fmt.Errorf("inbox: DecrementReblogsCount (UndoAnnounce): %w", err)
	}
	return nil
}

func (p *inbox) resolveVisibility(note *Note, author *domain.Account) string {
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

func (p *inbox) hasLocalFollower(ctx context.Context, remoteAccountID string) (bool, error) {
	followers, err := p.follows.GetFollowers(ctx, remoteAccountID, nil, 1)
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

func (p *inbox) hasLocalRecipient(to []string) bool {
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

func (p *inbox) storeRemoteMedia(ctx context.Context, attachments []Attachment, accountID string) []string {
	var ids []string
	for _, att := range attachments {
		if att.URL == "" {
			continue
		}
		m, err := p.media.CreateRemote(ctx, accountID, att.URL)
		if err != nil {
			continue
		}
		ids = append(ids, m.ID)
	}
	return ids
}

func (p *inbox) processMentionNotifications(ctx context.Context, tags []Tag, statusID string, fromAccount *domain.Account) {
	for _, tag := range tags {
		if tag.Type != "Mention" || tag.Href == "" {
			continue
		}
		username := usernameFromActorIRI(tag.Href, p.cfg.InstanceDomain)
		if username == "" {
			continue
		}
		acc, err := p.accounts.GetLocalByUsername(ctx, username)
		if err != nil {
			continue
		}
		sid := statusID
		p.createNotificationAndPublish(ctx, acc.ID, fromAccount, "mention", &sid)
	}
}

func (p *inbox) handleCreate(ctx context.Context, activity *Activity, _ string) error {
	note, err := activity.ObjectNote()
	if err != nil {
		return fmt.Errorf("%w: create object is not a note: %w", ErrFatal, err)
	}
	if note.Type != objectTypeNote {
		return fmt.Errorf("%w: create object type %q is not supported", ErrFatal, note.Type)
	}
	if note.ID != "" {
		if _, err := p.statuses.GetByAPID(ctx, note.ID); err == nil {
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
		if parent, err := p.statuses.GetByAPID(ctx, *note.InReplyTo); err == nil {
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
	in := service.CreateStatusFromInboxInput{
		AccountID:      author.ID,
		URI:            note.ID,
		Text:           &content,
		Content:        &content,
		ContentWarning: contentWarning,
		Visibility:     visibility,
		Language:       noteLanguage(note),
		InReplyToID:    inReplyToID,
		APID:           note.ID,
		ApRaw:          apRaw,
		Sensitive:      note.Sensitive,
	}
	status, err := p.statuses.CreateFromInbox(ctx, in)
	if err != nil {
		if isUniqueViolation(err) {
			return nil
		}
		return fmt.Errorf("inbox: create status: %w", err)
	}
	for _, mediaID := range mediaIDs {
		_ = p.statuses.AttachMediaToStatus(ctx, mediaID, status.ID, author.ID)
	}
	if inReplyToID != nil {
		_ = p.statuses.IncrementRepliesCount(ctx, *inReplyToID)
	}
	p.processMentionNotifications(ctx, note.Tag, status.ID, author)
	enriched, err := p.statuses.GetByIDEnriched(ctx, status.ID)
	if err == nil {
		mentionedIDs := make([]string, 0, len(enriched.Mentions))
		for _, m := range enriched.Mentions {
			if m != nil {
				mentionedIDs = append(mentionedIDs, m.ID)
			}
		}
		p.eventBus.PublishStatusCreated(ctx, events.StatusCreatedEvent{
			Status:              enriched.Status,
			Author:              enriched.Author,
			Mentions:            enriched.Mentions,
			Tags:                enriched.Tags,
			Media:               enriched.Media,
			MentionedAccountIDs: mentionedIDs,
		})
	}
	return nil
}

func (p *inbox) handleAnnounce(ctx context.Context, activity *Activity, _ string) error {
	if activity.ID != "" {
		if _, err := p.statuses.GetByAPID(ctx, activity.ID); err == nil {
			return nil
		}
	}
	objectID, ok := activity.ObjectID()
	if !ok {
		return fmt.Errorf("%w: announce object is not a status IRI", ErrFatal)
	}
	original, err := p.statuses.GetByAPID(ctx, objectID)
	if err != nil {
		slog.Debug("inbox: Announce of unknown status", slog.String("object", objectID))
		return fmt.Errorf("inbox: GetByAPID (Announce): %w", err)
	}
	actor, err := p.resolveRemoteAccount(ctx, activity.Actor)
	if err != nil {
		return fmt.Errorf("inbox: resolve actor %q: %w", activity.Actor, err)
	}
	apRaw, _ := json.Marshal(activity)
	_, err = p.statuses.CreateBoostFromInbox(ctx, actor.ID, activity.ID, objectID, apRaw)
	if err != nil {
		if isUniqueViolation(err) {
			return nil
		}
		return fmt.Errorf("inbox: create boost: %w", err)
	}
	if original.Local {
		origID := original.ID
		p.createNotificationAndPublish(ctx, original.AccountID, actor, "reblog", &origID)
	}
	return nil
}

func (p *inbox) handleLike(ctx context.Context, activity *Activity) error {
	objectID, ok := activity.ObjectID()
	if !ok {
		return fmt.Errorf("%w: like object is not a status IRI", ErrFatal)
	}
	status, err := p.statuses.GetByAPID(ctx, objectID)
	if err != nil {
		slog.Debug("inbox: Like of unknown status", slog.String("object", objectID))
		return fmt.Errorf("inbox: GetByAPID (Like): %w", err)
	}
	actor, err := p.resolveRemoteAccount(ctx, activity.Actor)
	if err != nil {
		return fmt.Errorf("inbox: resolve actor %q: %w", activity.Actor, err)
	}
	var apID *string
	if activity.ID != "" {
		apID = &activity.ID
	}
	_, err = p.statuses.CreateFavouriteFromInbox(ctx, actor.ID, status.ID, apID)
	if err != nil {
		if isUniqueViolation(err) {
			return nil
		}
		return fmt.Errorf("inbox: create favourite: %w", err)
	}
	if status.Local {
		statusID := status.ID
		p.createNotificationAndPublish(ctx, status.AccountID, actor, "favourite", &statusID)
	}
	return nil
}

func (p *inbox) handleDelete(ctx context.Context, activity *Activity) error {
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
		status, err := p.statuses.GetByAPID(ctx, objectID)
		if err != nil {
			return fmt.Errorf("inbox: GetByAPID (Delete): %w", err)
		}
		statusAuthor, _ := p.accounts.GetByID(ctx, status.AccountID)
		if statusAuthor != nil && statusAuthor.APID != activity.Actor {
			return fmt.Errorf("%w: delete: actor %q is not the author", ErrFatal, activity.Actor)
		}
		enriched, _ := p.statuses.GetByIDEnriched(ctx, status.ID)
		if enriched.Status != nil {
			hashtagNames := make([]string, 0, len(enriched.Tags))
			for _, t := range enriched.Tags {
				hashtagNames = append(hashtagNames, t.Name)
			}
			mentionedIDs := make([]string, 0, len(enriched.Mentions))
			for _, m := range enriched.Mentions {
				if m != nil {
					mentionedIDs = append(mentionedIDs, m.ID)
				}
			}
			p.sseEvents.PublishStatusDeletedRaw(ctx, status.ID, StatusEventOpts{
				AccountID:           status.AccountID,
				Visibility:          status.Visibility,
				Local:               status.Local,
				HashtagNames:        hashtagNames,
				MentionedAccountIDs: mentionedIDs,
			})
		}
		if delErr := p.statuses.SoftDelete(ctx, status.ID); delErr != nil {
			return fmt.Errorf("inbox: SoftDelete (Delete): %w", delErr)
		}
		return nil
	case "Person":
		account, err := p.accounts.GetByAPID(ctx, activity.Actor)
		if err != nil {
			return fmt.Errorf("inbox: GetByAPID (Delete Person): %w", err)
		}
		if suspendErr := p.accounts.Suspend(ctx, account.ID); suspendErr != nil {
			return fmt.Errorf("inbox: Suspend: %w", suspendErr)
		}
		return nil
	default:
		slog.Debug("inbox: unsupported Delete object type", slog.String("type", objectType))
		return nil
	}
}

func (p *inbox) handleUpdate(ctx context.Context, activity *Activity) error {
	objectType := activity.ObjectType()
	switch objectType {
	case objectTypeNote:
		note, err := activity.ObjectNote()
		if err != nil {
			return fmt.Errorf("%w: update{Note}: %w", ErrFatal, err)
		}
		status, err := p.statuses.GetByAPID(ctx, note.ID)
		if err != nil {
			return fmt.Errorf("inbox: GetByAPID (Update Note): %w", err)
		}
		author, _ := p.accounts.GetByID(ctx, status.AccountID)
		if author != nil && author.APID != activity.Actor {
			return fmt.Errorf("%w: update: actor is not the author", ErrFatal)
		}
		var cw *string
		if note.Summary != nil {
			cw = note.Summary
		}
		content := note.Content
		if updateErr := p.statuses.UpdateFromInbox(ctx, status.ID, status, service.UpdateStatusFromInboxInput{
			Text:           &content,
			Content:        &content,
			ContentWarning: cw,
			Sensitive:      note.Sensitive,
		}); updateErr != nil {
			return fmt.Errorf("inbox: UpdateFromInbox: %w", updateErr)
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
		slog.Debug("inbox: unsupported Update object type", slog.String("type", objectType))
		return nil
	}
}

func (p *inbox) handleBlock(ctx context.Context, activity *Activity) error {
	targetID, ok := activity.ObjectID()
	if !ok {
		return fmt.Errorf("%w: block object is not an actor IRI", ErrFatal)
	}
	targetUsername := usernameFromActorIRI(targetID, p.cfg.InstanceDomain)
	if targetUsername == "" {
		return nil
	}
	target, err := p.accounts.GetLocalByUsername(ctx, targetUsername)
	if err != nil {
		return fmt.Errorf("inbox: GetLocalByUsername (Block): %w", err)
	}
	actor, err := p.resolveRemoteAccount(ctx, activity.Actor)
	if err != nil {
		return fmt.Errorf("inbox: resolve actor: %w", err)
	}
	_, _ = p.follows.Block(ctx, actor.ID, target.ID)
	return nil
}
