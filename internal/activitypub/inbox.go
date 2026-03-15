package activitypub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/chairswithlegs/monstera/internal/activitypub/blocklist"
	"github.com/chairswithlegs/monstera/internal/activitypub/vocab"
	"github.com/chairswithlegs/monstera/internal/cache"
	"github.com/chairswithlegs/monstera/internal/config"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/microcosm-cc/bluemonday"
)

// ErrInboxFatal represent an inbox error that should not be retried.
var ErrInboxFatal = errors.New("fatal inbox error")

// Inbox processes incoming ActivityPub activities.
type Inbox interface {
	Process(ctx context.Context, activity *vocab.Activity) error
}

// NewInbox constructs an Inbox. The inbox is a pure AP-to-service translation
// layer: it calls service methods which internally emit domain events for
// federation and SSE.
func NewInbox(
	accounts service.AccountService,
	follows service.FollowService,
	notifications service.NotificationService,
	statuses service.StatusService,
	statusWrites service.StatusWriteService,
	media service.MediaService,
	remoteResolver *RemoteAccountResolver,
	c cache.Store,
	bl *blocklist.BlocklistCache,
	cfg *config.Config,
) Inbox {
	return &inbox{
		accounts:       accounts,
		follows:        follows,
		notifications:  notifications,
		statuses:       statuses,
		statusWrites:   statusWrites,
		media:          media,
		remoteResolver: remoteResolver,
		cache:          c,
		blocklist:      bl,
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
	statusWrites   service.StatusWriteService
	media          service.MediaService
	remoteResolver *RemoteAccountResolver
	cache          cache.Store
	blocklist      *blocklist.BlocklistCache
	cfg            *config.Config
}

// Process dispatches a verified incoming activity to the appropriate handler.
func (p *inbox) Process(ctx context.Context, activity *vocab.Activity) error {
	slog.DebugContext(ctx, "inbox: processing activity",
		slog.String("type", string(activity.Type)), slog.String("id", activity.ID), slog.String("actor", activity.Actor))

	actorDomain := vocab.DomainFromIRI(activity.Actor)
	if actorDomain == "" {
		return fmt.Errorf("%w: cannot extract domain from actor %q", ErrInboxFatal, activity.Actor)
	}
	if p.blocklist.IsSuspended(ctx, actorDomain) {
		slog.DebugContext(ctx, "inbox: dropped activity from suspended domain",
			slog.String("domain", actorDomain),
			slog.String("type", string(activity.Type)),
			slog.String("id", activity.ID),
		)
		return nil
	}
	switch activity.Type {
	case vocab.ObjectTypeFollow:
		return p.handleFollow(ctx, activity)
	case vocab.ObjectTypeAccept:
		return p.handleAcceptFollow(ctx, activity)
	case vocab.ObjectTypeReject:
		return p.handleRejectFollow(ctx, activity)
	case vocab.ObjectTypeUndo:
		return p.handleUndo(ctx, activity)
	case vocab.ObjectTypeCreate:
		return p.handleCreate(ctx, activity, actorDomain)
	case vocab.ObjectTypeAnnounce:
		return p.handleAnnounce(ctx, activity, actorDomain)
	case vocab.ObjectTypeLike:
		return p.handleLike(ctx, activity)
	case vocab.ObjectTypeDelete:
		return p.handleDelete(ctx, activity)
	case vocab.ObjectTypeUpdate:
		return p.handleUpdate(ctx, activity)
	case vocab.ObjectTypeBlock:
		return p.handleBlock(ctx, activity)
	default:
		slog.DebugContext(ctx, "inbox: unsupported activity type", slog.String("type", string(activity.Type)), slog.String("id", activity.ID))
		return nil
	}
}

// handleFollow handles a Follow activity.
func (p *inbox) handleFollow(ctx context.Context, activity *vocab.Activity) error {
	// Ignore follows without a valid activity ID.
	if activity.ID == "" {
		return nil
	}

	// Get the account being followed.
	targetID, ok := activity.ObjectID()
	if !ok {
		return fmt.Errorf("%w: follow object is not an actor IRI", ErrInboxFatal)
	}
	target, err := p.accounts.GetByAPID(ctx, targetID)
	if err != nil {
		return fmt.Errorf("%w: follow target not found: %s", ErrInboxFatal, targetID)
	}

	// Get the account that is following.
	actor, err := p.remoteResolver.ResolveRemoteAccountByIRI(ctx, activity.Actor)
	if err != nil {
		return fmt.Errorf("inbox: resolve actor %q: %w", activity.Actor, err)
	}

	// Ignore duplicate Follows.
	existing, _ := p.follows.GetFollowByAPID(ctx, activity.ID)
	if existing != nil {
		return nil
	}

	// Check if the target auto-accepts follows, or if we should treat this as a follow request
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

	// Create the follow.
	_, err = p.follows.CreateRemoteFollow(ctx, actor.ID, target.ID, state, apID)
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return nil
		}
		return fmt.Errorf("inbox: create follow: %w", err)
	}
	// CreateRemoteFollow emits follow.accepted domain event when state is accepted,
	// which triggers the federation subscriber to send Accept{Follow}.
	p.createNotificationAndEmit(ctx, target.ID, actor, notifType, nil)
	return nil
}

// handleAcceptFollow handles an Accept{Follow} activity.
func (p *inbox) handleAcceptFollow(ctx context.Context, activity *vocab.Activity) error {
	follow, err := p.resolveFollowFromObject(ctx, activity)
	if err != nil {
		return err
	}
	if err := p.ensureActorIsFollowTarget(ctx, activity, follow); err != nil {
		return err
	}
	if acceptErr := p.follows.AcceptFollow(ctx, follow.ID); acceptErr != nil {
		return fmt.Errorf("inbox: AcceptFollow: %w", acceptErr)
	}
	return nil
}

// handleRejectFollow handles a Reject{Follow} activity.
func (p *inbox) handleRejectFollow(ctx context.Context, activity *vocab.Activity) error {
	follow, err := p.resolveFollowFromObject(ctx, activity)
	if err != nil {
		return err
	}
	if err := p.ensureActorIsFollowTarget(ctx, activity, follow); err != nil {
		return err
	}
	if delErr := p.follows.DeleteRemoteFollow(ctx, follow.AccountID, follow.TargetID); delErr != nil {
		return fmt.Errorf("inbox: DeleteRemoteFollow (Reject): %w", delErr)
	}
	return nil
}

// handleUndo handles an Undo activity.
func (p *inbox) handleUndo(ctx context.Context, activity *vocab.Activity) error {
	innerType := activity.ObjectType()
	switch innerType {
	case vocab.ObjectTypeFollow:
		return p.handleUndoFollow(ctx, activity)
	case vocab.ObjectTypeLike:
		return p.handleUndoLike(ctx, activity)
	case vocab.ObjectTypeAnnounce:
		return p.handleUndoAnnounce(ctx, activity)
	default:
		objectID, ok := activity.ObjectID()
		if !ok {
			slog.DebugContext(ctx, "inbox: unsupported Undo object type", slog.String("type", string(innerType)), slog.String("id", activity.ID))
			return nil
		}
		if follow, err := p.follows.GetFollowByAPID(ctx, objectID); err == nil {
			if err := p.undoActorMatchesAccount(ctx, activity, follow.AccountID); err != nil {
				return err
			}
			if delErr := p.follows.DeleteRemoteFollow(ctx, follow.AccountID, follow.TargetID); delErr != nil {
				return fmt.Errorf("inbox: DeleteFollow (Undo default): %w", delErr)
			}
			return nil
		}
		if fav, err := p.statuses.GetFavouriteByAPID(ctx, objectID); err == nil {
			if err := p.undoActorMatchesAccount(ctx, activity, fav.AccountID); err != nil {
				return err
			}
			return p.undoFavourite(ctx, fav)
		}
		slog.DebugContext(ctx, "inbox: unsupported Undo object type", slog.String("type", string(innerType)), slog.String("id", activity.ID))
		return nil
	}
}

// handleUndoFollow handles an Undo{Follow} activity.
func (p *inbox) handleUndoFollow(ctx context.Context, activity *vocab.Activity) error {
	inner, err := activity.ObjectActivity()
	if err != nil {
		return fmt.Errorf("%w: undo{Follow} object is not a follow activity", ErrInboxFatal)
	}

	var follow *domain.Follow

	if inner.ID != "" {
		if f, err := p.follows.GetFollowByAPID(ctx, inner.ID); err == nil {
			if err := p.undoActorMatchesAccount(ctx, activity, f.AccountID); err != nil {
				return err
			}
			follow = f
		}
	}

	if follow == nil {
		actorAccount, err := p.accounts.GetByAPID(ctx, inner.Actor)
		if err != nil {
			return fmt.Errorf("inbox: GetByAPID actor (UndoFollow): %w", err)
		}
		if err := p.undoActorMatchesAccount(ctx, activity, actorAccount.ID); err != nil {
			return err
		}
		objectID, _ := inner.ObjectID()
		targetAccount, err := p.accounts.GetByAPID(ctx, objectID)
		if err != nil {
			return fmt.Errorf("inbox: GetByAPID target (UndoFollow): %w", err)
		}
		follow, err = p.follows.GetFollow(ctx, actorAccount.ID, targetAccount.ID)
		if err != nil {
			return fmt.Errorf("inbox: GetFollow (UndoFollow): %w", err)
		}
	}

	if delErr := p.follows.DeleteRemoteFollow(ctx, follow.AccountID, follow.TargetID); delErr != nil {
		return fmt.Errorf("inbox: DeleteFollow (UndoFollow): %w", delErr)
	}
	return nil
}

// undoFavourite handles AP Undo{Like} -> domain delete favourite.
func (p *inbox) undoFavourite(ctx context.Context, fav *domain.Favourite) error {
	if err := p.statusWrites.DeleteRemoteFavourite(ctx, fav.AccountID, fav.StatusID); err != nil {
		return fmt.Errorf("inbox: DeleteRemoteFavourite: %w", err)
	}
	return nil
}

// handleUndoLike handles an Undo{Like} activity.
func (p *inbox) handleUndoLike(ctx context.Context, activity *vocab.Activity) error {
	inner, err := activity.ObjectActivity()
	if err != nil {
		return fmt.Errorf("%w: undo{Like} object is not a like activity", ErrInboxFatal)
	}

	var fav *domain.Favourite

	if inner.ID != "" {
		if f, err := p.statuses.GetFavouriteByAPID(ctx, inner.ID); err == nil {
			if err := p.undoActorMatchesAccount(ctx, activity, f.AccountID); err != nil {
				return err
			}
			fav = f
		}
	}

	if fav == nil {
		actorAccount, err := p.accounts.GetByAPID(ctx, inner.Actor)
		if err != nil {
			return fmt.Errorf("inbox: GetByAPID actor (UndoLike): %w", err)
		}
		if err := p.undoActorMatchesAccount(ctx, activity, actorAccount.ID); err != nil {
			return err
		}
		objectID, _ := inner.ObjectID()
		status, err := p.statuses.GetByAPID(ctx, objectID)
		if err != nil {
			return fmt.Errorf("inbox: GetStatusByAPID (UndoLike): %w", err)
		}
		fav, err = p.statuses.GetFavouriteByAccountAndStatus(ctx, actorAccount.ID, status.ID)
		if err != nil {
			return fmt.Errorf("inbox: GetFavouriteByAccountAndStatus (UndoLike): %w", err)
		}
	}

	return p.undoFavourite(ctx, fav)
}

// handleUndoAnnounce handles an Undo{Announce} activity.
func (p *inbox) handleUndoAnnounce(ctx context.Context, activity *vocab.Activity) error {
	inner, err := activity.ObjectActivity()
	if err != nil {
		return fmt.Errorf("%w: undo{Announce} object is not an announce activity", ErrInboxFatal)
	}
	actorAccount, err := p.accounts.GetByAPID(ctx, inner.Actor)
	if err != nil {
		return fmt.Errorf("inbox: GetByAPID actor (UndoAnnounce): %w", err)
	}
	if err := p.undoActorMatchesAccount(ctx, activity, actorAccount.ID); err != nil {
		return err
	}
	objectID, _ := inner.ObjectID()
	originalStatus, err := p.statuses.GetByAPID(ctx, objectID)
	if err != nil {
		return fmt.Errorf("inbox: GetStatusByAPID (UndoAnnounce): %w", err)
	}
	if err := p.statusWrites.Unreblog(ctx, actorAccount.ID, originalStatus.ID); err != nil {
		return fmt.Errorf("inbox: Unreblog (UndoAnnounce): %w", err)
	}
	return nil
}

// handleCreate handles a Create{Note} activity.
func (p *inbox) handleCreate(ctx context.Context, activity *vocab.Activity, _ string) error {
	note, err := activity.ObjectNote()
	if err != nil {
		return fmt.Errorf("%w: create object is not a note: %w", ErrInboxFatal, err)
	}
	if note.Type != vocab.ObjectTypeNote {
		return fmt.Errorf("%w: create object type %q is not supported", ErrInboxFatal, note.Type)
	}
	if note.ID != "" {
		if _, err := p.statuses.GetByAPID(ctx, note.ID); err == nil {
			return nil
		}
	}
	author, err := p.remoteResolver.ResolveRemoteAccountByIRI(ctx, activity.Actor)
	if err != nil {
		return fmt.Errorf("inbox: resolve actor %q: %w", activity.Actor, err)
	}
	visibility := p.resolveVisibility(note, author)

	// If the visibility is private, the status is only meant for local followers.
	if visibility == domain.VisibilityPrivate {
		hasLocal, err := p.hasLocalFollower(ctx, author.ID)
		if err != nil {
			return err
		}
		// If the author has no local followers, the status is not meant for anyone. Drop it.
		if !hasLocal {
			return nil
		}
	}
	// If the visibility is direct, the status is only meant for local recipients.
	if visibility == domain.VisibilityDirect {
		hasLocal, err := p.hasLocalRecipient(ctx, note.To)
		if err != nil {
			return err
		}
		// If the status is not meant for local recipients, drop it.
		if !hasLocal {
			return nil
		}
	}
	createInput := p.buildCreateStatusInput(ctx, note, author, visibility)
	// AP Note -> domain status
	status, err := p.statusWrites.CreateRemote(ctx, createInput.in)
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return nil
		}
		return fmt.Errorf("inbox: create status: %w", err)
	}
	// CreateRemote emits status.created.remote domain event for SSE.
	p.processMentionNotifications(ctx, note.Tag, status.ID, author)
	return nil
}

// handleAnnounce handles an Announce activity.
func (p *inbox) handleAnnounce(ctx context.Context, activity *vocab.Activity, _ string) error {
	if activity.ID != "" {
		if _, err := p.statuses.GetByAPID(ctx, activity.ID); err == nil {
			return nil
		}
	}
	objectID, ok := activity.ObjectID()
	if !ok {
		return fmt.Errorf("%w: announce object is not a status IRI", ErrInboxFatal)
	}
	original, err := p.statuses.GetByAPID(ctx, objectID)
	if err != nil {
		slog.DebugContext(ctx, "inbox: Announce of unknown status", slog.String("object", objectID))
		return fmt.Errorf("inbox: GetByAPID (Announce): %w", err)
	}
	actor, err := p.remoteResolver.ResolveRemoteAccountByIRI(ctx, activity.Actor)
	if err != nil {
		return fmt.Errorf("inbox: resolve actor %q: %w", activity.Actor, err)
	}
	apRaw, _ := json.Marshal(activity)
	// AP Announce -> domain reblog
	_, err = p.statusWrites.CreateRemoteReblog(ctx, service.CreateRemoteReblogInput{
		AccountID:        actor.ID,
		ActivityAPID:     activity.ID,
		ObjectStatusAPID: objectID,
		ApRaw:            apRaw,
	})
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return nil
		}
		return fmt.Errorf("inbox: create reblog: %w", err)
	}
	// AP Announce -> "reblog" notification for the original status author
	if original.Local {
		origID := original.ID
		p.createNotificationAndEmit(ctx, original.AccountID, actor, "reblog", &origID)
	}
	return nil
}

// handleLike handles a Like activity.
func (p *inbox) handleLike(ctx context.Context, activity *vocab.Activity) error {
	objectID, ok := activity.ObjectID()
	if !ok {
		return fmt.Errorf("%w: like object is not a status IRI", ErrInboxFatal)
	}
	status, err := p.statuses.GetByAPID(ctx, objectID)
	if err != nil {
		slog.DebugContext(ctx, "inbox: Like of unknown status", slog.String("object", objectID))
		return fmt.Errorf("inbox: GetByAPID (Like): %w", err)
	}
	actor, err := p.remoteResolver.ResolveRemoteAccountByIRI(ctx, activity.Actor)
	if err != nil {
		return fmt.Errorf("inbox: resolve actor %q: %w", activity.Actor, err)
	}
	var apID *string
	if activity.ID != "" {
		apID = &activity.ID
	}
	// AP Like -> domain favourite
	_, err = p.statusWrites.CreateRemoteFavourite(ctx, actor.ID, status.ID, apID)
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return nil
		}
		return fmt.Errorf("inbox: create favourite: %w", err)
	}
	// AP Like -> "favourite" notification for the status author
	if status.Local {
		statusID := status.ID
		p.createNotificationAndEmit(ctx, status.AccountID, actor, "favourite", &statusID)
	}
	return nil
}

// handleDelete handles a Delete activity.
func (p *inbox) handleDelete(ctx context.Context, activity *vocab.Activity) error {
	objectType := activity.ObjectType()
	switch objectType {
	case vocab.ObjectTypeTombstone, vocab.ObjectTypeNote, "":
		objectID, ok := activity.ObjectID()
		if !ok {
			var obj struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(activity.ObjectRaw, &obj); err != nil {
				return fmt.Errorf("%w: delete: cannot extract object ID", ErrInboxFatal)
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
		statusAuthor, err := p.accounts.GetByID(ctx, status.AccountID)
		if err != nil {
			return fmt.Errorf("inbox: GetByID author (Delete): %w", err)
		}
		if statusAuthor.APID != activity.Actor {
			return fmt.Errorf("%w: delete: actor %q is not the author", ErrInboxFatal, activity.Actor)
		}
		// AP Delete{Note/Tombstone} -> domain delete status
		if err := p.statusWrites.DeleteRemote(ctx, status.ID); err != nil {
			return fmt.Errorf("inbox: DeleteRemote (Delete): %w", err)
		}
		return nil
	case vocab.ObjectTypePerson:
		account, err := p.accounts.GetByAPID(ctx, activity.Actor)
		if err != nil {
			return fmt.Errorf("inbox: GetByAPID (Delete Person): %w", err)
		}
		// AP Delete{Person} -> domain suspend account (AP account deletion treated as suspension)
		if suspendErr := p.accounts.Suspend(ctx, account.ID); suspendErr != nil {
			return fmt.Errorf("inbox: Suspend: %w", suspendErr)
		}
		return nil
	default:
		slog.DebugContext(ctx, "inbox: unsupported Delete object type", slog.String("type", string(objectType)))
		return nil
	}
}

// handleUpdate handles an Update{Note} activity.
func (p *inbox) handleUpdate(ctx context.Context, activity *vocab.Activity) error {
	objectType := activity.ObjectType()
	switch objectType {
	case vocab.ObjectTypeNote:
		note, err := activity.ObjectNote()
		if err != nil {
			return fmt.Errorf("%w: update{Note}: %w", ErrInboxFatal, err)
		}
		status, err := p.statuses.GetByAPID(ctx, note.ID)
		if err != nil {
			return fmt.Errorf("inbox: GetByAPID (Update Note): %w", err)
		}
		author, err := p.accounts.GetByID(ctx, status.AccountID)
		if err != nil {
			return fmt.Errorf("inbox: GetByID author (Update Note): %w", err)
		}
		if author.APID != activity.Actor {
			return fmt.Errorf("%w: update: actor is not the author", ErrInboxFatal)
		}
		var cw *string
		if note.Summary != nil {
			sanitized := bluemonday.StrictPolicy().Sanitize(*note.Summary)
			cw = &sanitized
		}
		content := bluemonday.UGCPolicy().Sanitize(note.Content)
		// AP Update{Note} -> domain update status
		if updateErr := p.statusWrites.UpdateRemote(ctx, status.ID, status, service.UpdateRemoteStatusInput{
			Text:           &content,
			Content:        &content,
			ContentWarning: cw,
			Sensitive:      note.Sensitive,
		}); updateErr != nil {
			return fmt.Errorf("inbox: UpdateRemote: %w", updateErr)
		}
		return nil
	case vocab.ObjectTypePerson, vocab.ObjectTypeService: // TODO: not sure if Service is valid here
		actor, err := activity.ObjectActor()
		if err != nil {
			return fmt.Errorf("%w: Update{Person}: %w", ErrInboxFatal, err)
		}
		if activity.Actor != actor.ID {
			return fmt.Errorf("%w: update: actor %q is not the object being updated", ErrInboxFatal, activity.Actor)
		}
		_, err = p.remoteResolver.SyncActorToStore(ctx, actor)
		return err
	default:
		slog.DebugContext(ctx, "inbox: unsupported Update object type", slog.String("type", string(objectType)))
		return nil
	}
}

// handleBlock handles a Block activity.
func (p *inbox) handleBlock(ctx context.Context, activity *vocab.Activity) error {
	targetID, ok := activity.ObjectID()
	if !ok {
		return fmt.Errorf("%w: block object is not an actor IRI", ErrInboxFatal)
	}
	target, err := p.accounts.GetByAPID(ctx, targetID)
	if err != nil {
		return fmt.Errorf("inbox: GetByAPID (Block): %w", err)
	}
	actor, err := p.remoteResolver.ResolveRemoteAccountByIRI(ctx, activity.Actor)
	if err != nil {
		return fmt.Errorf("inbox: resolve actor: %w", err)
	}
	_, _ = p.follows.Block(ctx, actor.ID, target.ID)
	return nil
}

func (p *inbox) createNotificationAndEmit(ctx context.Context, recipientID string, fromAccount *domain.Account, notifType string, statusID *string) {
	_ = p.notifications.CreateAndEmit(ctx, recipientID, fromAccount, notifType, statusID)
}

// resolveFollowFromObject resolves a Follow from an activity's object (IRI or embedded Follow activity).
func (p *inbox) resolveFollowFromObject(ctx context.Context, activity *vocab.Activity) (*domain.Follow, error) {
	inner, err := activity.ObjectActivity()
	if err != nil {
		objectID, ok := activity.ObjectID()
		if !ok {
			return nil, fmt.Errorf("%w: object is not a follow activity or IRI", ErrInboxFatal)
		}
		follow, err := p.follows.GetFollowByAPID(ctx, objectID)
		if err != nil {
			return nil, fmt.Errorf("inbox: GetFollowByAPID: %w", err)
		}
		return follow, nil
	}
	if inner.ID != "" {
		follow, err := p.follows.GetFollowByAPID(ctx, inner.ID)
		if err == nil {
			return follow, nil
		}
	}
	actorAccount, err := p.accounts.GetByAPID(ctx, inner.Actor)
	if err != nil {
		return nil, fmt.Errorf("%w: actor not found %q", ErrInboxFatal, inner.Actor)
	}
	targetID, _ := inner.ObjectID()
	targetAccount, err := p.accounts.GetByAPID(ctx, targetID)
	if err != nil {
		return nil, fmt.Errorf("%w: target not found %q", ErrInboxFatal, targetID)
	}
	follow, err := p.follows.GetFollow(ctx, actorAccount.ID, targetAccount.ID)
	if err != nil {
		return nil, fmt.Errorf("%w: follow relationship not found", ErrInboxFatal)
	}
	return follow, nil
}

// ensureActorIsFollowTarget ensures the activity actor is the follow target (the account that may accept/reject).
func (p *inbox) ensureActorIsFollowTarget(ctx context.Context, activity *vocab.Activity, follow *domain.Follow) error {
	targetAccount, err := p.accounts.GetByID(ctx, follow.TargetID)
	if err != nil {
		return fmt.Errorf("inbox: GetByID target (Accept/Reject): %w", err)
	}
	if targetAccount.APID != activity.Actor {
		return fmt.Errorf("%w: accept/reject: actor %q is not the follow target", ErrInboxFatal, activity.Actor)
	}
	return nil
}

// undoActorMatchesAccount returns an error if the Undo's actor is not the account that
// performed the original action. Prevents forged Undo from removing another user's follow/like/boost.
func (p *inbox) undoActorMatchesAccount(ctx context.Context, activity *vocab.Activity, performerAccountID string) error {
	undoActor, err := p.accounts.GetByAPID(ctx, activity.Actor)
	if err != nil || undoActor == nil {
		return fmt.Errorf("%w: undo actor %q not found or invalid", ErrInboxFatal, activity.Actor)
	}
	if undoActor.ID != performerAccountID {
		return fmt.Errorf("%w: undo actor %q is not the performer", ErrInboxFatal, activity.Actor)
	}
	return nil
}

func (p *inbox) resolveVisibility(note *vocab.Note, author *domain.Account) string {
	for _, addr := range note.To {
		if addr == vocab.PublicAddress {
			return domain.VisibilityPublic
		}
	}
	for _, addr := range note.Cc {
		if addr == vocab.PublicAddress {
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

// hasLocalRecipient returns true if any of the addresses (IRIs) are local accounts.
func (p *inbox) hasLocalRecipient(ctx context.Context, to []string) (bool, error) {
	for _, addr := range to {
		_, err := p.accounts.GetByAPID(ctx, addr)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return false, fmt.Errorf("GetByAPID: %w", err)
		}
		if err == nil {
			return true, nil
		}
	}
	return false, nil
}

func noteLanguage(note *vocab.Note) *string {
	if len(note.ContentMap) == 0 {
		return nil
	}
	for k := range note.ContentMap {
		return &k
	}
	return nil
}

// storeRemoteMedia stores a reference to the remote media attachments.
// Note that this doesn't actually store the media itself, just a reference to it.
func (p *inbox) storeRemoteMedia(ctx context.Context, attachments []vocab.Attachment, accountID string) []string {
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

func (p *inbox) processMentionNotifications(ctx context.Context, tags []vocab.Tag, statusID string, fromAccount *domain.Account) {
	for _, tag := range tags {
		if tag.Type != "Mention" || tag.Href == "" {
			continue
		}

		// Verify that the mention is a local account.
		domain := vocab.DomainFromIRI(tag.Href)
		if domain == p.cfg.InstanceDomain {
			continue
		}

		// Get the account.
		account, err := p.accounts.GetByAPID(ctx, tag.Href)
		if err != nil {
			slog.DebugContext(ctx, "inbox: mention: account not found", slog.String("href", tag.Href))
			continue
		}

		sid := statusID
		p.createNotificationAndEmit(ctx, account.ID, fromAccount, "mention", &sid)
	}
}

// createStatusInput holds the result of buildCreateStatusInput.
type createStatusInput struct {
	in service.CreateRemoteStatusInput
}

func (p *inbox) buildCreateStatusInput(ctx context.Context, note *vocab.Note, author *domain.Account, visibility string) createStatusInput {
	var inReplyToID *string
	if note.InReplyTo != nil && *note.InReplyTo != "" {
		if parent, err := p.statuses.GetByAPID(ctx, *note.InReplyTo); err == nil {
			inReplyToID = &parent.ID
		}
	}
	mediaIDs := p.storeRemoteMedia(ctx, note.Attachment, author.ID)
	var contentWarning *string
	if note.Summary != nil && *note.Summary != "" {
		cw := bluemonday.StrictPolicy().Sanitize(*note.Summary)
		contentWarning = &cw
	}
	apRaw, _ := json.Marshal(note)

	// Sanitize content using UGC policy to retain safe HTML formatting from remote notes.
	content := bluemonday.UGCPolicy().Sanitize(note.Content)

	in := service.CreateRemoteStatusInput{
		AccountID:      author.ID,
		URI:            note.ID,
		Text:           &content,
		Content:        &content,
		ContentWarning: contentWarning,
		Visibility:     visibility,
		Language:       noteLanguage(note),
		InReplyToID:    inReplyToID,
		MediaIDs:       mediaIDs,
		APID:           note.ID,
		ApRaw:          apRaw,
		Sensitive:      note.Sensitive,
	}
	return createStatusInput{in: in}
}
