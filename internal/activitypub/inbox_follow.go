package activitypub

import (
	"context"
	"errors"
	"fmt"

	"github.com/chairswithlegs/monstera/internal/activitypub/vocab"
	"github.com/chairswithlegs/monstera/internal/domain"
)

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
	existing, _ := p.remoteFollows.GetFollowByAPID(ctx, activity.ID)
	if existing != nil {
		return nil
	}

	state := domain.FollowStateAccepted
	if target.Locked {
		state = domain.FollowStatePending
	}
	var apID *string
	if activity.ID != "" {
		apID = &activity.ID
	}

	// Create the follow.
	_, err = p.remoteFollows.CreateRemoteFollow(ctx, actor.ID, target.ID, state, apID)
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return nil
		}
		return fmt.Errorf("inbox: create follow: %w", err)
	}
	// CreateRemoteFollow emits follow.created/follow.requested domain events;
	// the notification subscriber handles notification creation.
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
	if acceptErr := p.remoteFollows.AcceptFollow(ctx, follow.ID); acceptErr != nil {
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
	if delErr := p.remoteFollows.DeleteRemoteFollow(ctx, follow.AccountID, follow.TargetID); delErr != nil {
		return fmt.Errorf("inbox: DeleteRemoteFollow (Reject): %w", delErr)
	}
	return nil
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
	if err := p.remoteFollows.CreateRemoteBlock(ctx, actor.ID, target.ID); err != nil {
		return fmt.Errorf("inbox: CreateRemoteBlock: %w", err)
	}
	return nil
}

// resolveFollowFromObject resolves a Follow from an activity's object (IRI or embedded Follow activity).
func (p *inbox) resolveFollowFromObject(ctx context.Context, activity *vocab.Activity) (*domain.Follow, error) {
	inner, err := activity.ObjectActivity()
	if err != nil {
		objectID, ok := activity.ObjectID()
		if !ok {
			return nil, fmt.Errorf("%w: object is not a follow activity or IRI", ErrInboxFatal)
		}
		follow, err := p.remoteFollows.GetFollowByAPID(ctx, objectID)
		if err != nil {
			return nil, fmt.Errorf("inbox: GetFollowByAPID: %w", err)
		}
		return follow, nil
	}
	if inner.ID != "" {
		follow, err := p.remoteFollows.GetFollowByAPID(ctx, inner.ID)
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

func (p *inbox) hasLocalFollower(ctx context.Context, remoteAccountID string) (bool, error) {
	ok, err := p.remoteFollows.HasLocalFollower(ctx, remoteAccountID)
	if err != nil {
		return false, fmt.Errorf("HasLocalFollower(%s): %w", remoteAccountID, err)
	}
	return ok, nil
}

// hasLocalRecipient returns true if any of the addresses (IRIs) are local accounts.
func (p *inbox) hasLocalRecipient(ctx context.Context, to []string) (bool, error) {
	for _, addr := range to {
		acc, err := p.accounts.GetByAPID(ctx, addr)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return false, fmt.Errorf("GetByAPID: %w", err)
		}
		if err == nil && acc.Domain == nil {
			return true, nil
		}
	}
	return false, nil
}
