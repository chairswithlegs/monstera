package activitypub

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/chairswithlegs/monstera/internal/activitypub/vocab"
	"github.com/chairswithlegs/monstera/internal/domain"
)

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
	case vocab.ObjectTypeBlock:
		return p.handleUndoBlock(ctx, activity)
	default:
		objectID, ok := activity.ObjectID()
		if !ok {
			slog.DebugContext(ctx, "inbox: unsupported Undo object type", slog.String("type", string(innerType)), slog.String("id", activity.ID))
			return nil
		}
		if follow, err := p.remoteFollows.GetFollowByAPID(ctx, objectID); err == nil {
			if err := p.undoActorMatchesAccount(ctx, activity, follow.AccountID); err != nil {
				return err
			}
			if delErr := p.remoteFollows.DeleteRemoteFollow(ctx, follow.AccountID, follow.TargetID); delErr != nil {
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

	// Attempt to find the follow by inner ID.
	if inner.ID != "" {
		if f, err := p.remoteFollows.GetFollowByAPID(ctx, inner.ID); err == nil {
			if err := p.undoActorMatchesAccount(ctx, activity, f.AccountID); err != nil {
				return err
			}
			follow = f
		}
	}

	// If the follow is not found, try to find it by looking up the actor and target accounts.
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

	if follow == nil {
		slog.DebugContext(ctx, "inbox: follow not found (UndoFollow)", slog.String("actor", inner.Actor), slog.String("object", inner.ID))
		return nil
	}

	if delErr := p.remoteFollows.DeleteRemoteFollow(ctx, follow.AccountID, follow.TargetID); delErr != nil {
		return fmt.Errorf("inbox: DeleteFollow (UndoFollow): %w", delErr)
	}
	return nil
}

// undoFavourite handles AP Undo{Like} -> domain delete favourite.
func (p *inbox) undoFavourite(ctx context.Context, fav *domain.Favourite) error {
	if err := p.remoteStatusWrites.DeleteRemoteFavourite(ctx, fav.AccountID, fav.StatusID); err != nil {
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

	// Attempt to find the favourite by inner ID.
	if inner.ID != "" {
		if f, err := p.statuses.GetFavouriteByAPID(ctx, inner.ID); err == nil {
			if err := p.undoActorMatchesAccount(ctx, activity, f.AccountID); err != nil {
				return err
			}
			fav = f
		}
	}

	// If the favourite is not found, try to find it by looking up the actor and target accounts.
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

	if fav == nil {
		slog.DebugContext(ctx, "inbox: favourite not found (UndoLike)", slog.String("actor", inner.Actor), slog.String("object", inner.ID))
		return nil
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
	if err := p.remoteStatusWrites.DeleteRemoteReblog(ctx, actorAccount.ID, originalStatus.ID); err != nil {
		return fmt.Errorf("inbox: DeleteRemoteReblog (UndoAnnounce): %w", err)
	}
	return nil
}

// handleUndoBlock handles an Undo{Block} activity.
func (p *inbox) handleUndoBlock(ctx context.Context, activity *vocab.Activity) error {
	inner, err := activity.ObjectActivity()
	if err != nil {
		return fmt.Errorf("%w: undo{Block} object is not a block activity", ErrInboxFatal)
	}
	actorAccount, err := p.accounts.GetByAPID(ctx, inner.Actor)
	if err != nil {
		return fmt.Errorf("inbox: GetByAPID actor (UndoBlock): %w", err)
	}
	if err := p.undoActorMatchesAccount(ctx, activity, actorAccount.ID); err != nil {
		return err
	}
	objectID, _ := inner.ObjectID()
	targetAccount, err := p.accounts.GetByAPID(ctx, objectID)
	if err != nil {
		return fmt.Errorf("inbox: GetByAPID target (UndoBlock): %w", err)
	}
	if err := p.remoteFollows.DeleteRemoteBlock(ctx, actorAccount.ID, targetAccount.ID); err != nil {
		return fmt.Errorf("inbox: DeleteRemoteBlock (UndoBlock): %w", err)
	}
	return nil
}
