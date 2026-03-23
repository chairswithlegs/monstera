package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/events"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// FollowService orchestrates follow/unfollow, block/mute, and relationship lookups.
type FollowService interface {
	Follow(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Relationship, error)
	Unfollow(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Relationship, error)
	Block(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Relationship, error)
	Unblock(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Relationship, error)
	Mute(ctx context.Context, actorAccountID, targetAccountID string, hideNotifications bool) (*domain.Relationship, error)
	Unmute(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Relationship, error)
	GetFollow(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Follow, error)
	GetFollowers(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, error)
	GetFollowing(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, error)
	ListPendingFollowRequests(ctx context.Context, targetAccountID string, maxID *string, limit int) ([]domain.Account, *string, error)
	ListBlockedAccounts(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, *string, error)
	ListMutedAccounts(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, *string, error)
	AuthorizeFollowRequest(ctx context.Context, targetAccountID, requesterAccountID string) error
	RejectFollowRequest(ctx context.Context, targetAccountID, requesterAccountID string) error
}

type followService struct {
	store         store.Store
	accounts      AccountService
	remoteFollows RemoteFollowService
}

// NewFollowService returns a FollowService.
func NewFollowService(s store.Store, accounts AccountService, remoteFollows RemoteFollowService) FollowService {
	return &followService{store: s, accounts: accounts, remoteFollows: remoteFollows}
}

// Follow creates a follow from actor to target. Returns the relationship after the change.
// Errors: ErrValidation (self-follow), ErrNotFound (target), ErrForbidden (block in either direction).
func (svc *followService) Follow(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Relationship, error) {
	if actorAccountID == targetAccountID {
		return nil, fmt.Errorf("Follow: %w", domain.ErrValidation)
	}
	target, err := svc.store.GetAccountByID(ctx, targetAccountID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("Follow target: %w", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("GetAccountByID(target): %w", err)
	}
	if target.Suspended {
		return nil, fmt.Errorf("Follow target: %w", domain.ErrNotFound)
	}
	rel, err := svc.accounts.GetRelationship(ctx, actorAccountID, targetAccountID)
	if err != nil {
		return nil, fmt.Errorf("Follow GetRelationship: %w", err)
	}
	if rel.Blocking || rel.BlockedBy {
		return nil, fmt.Errorf("Follow: %w", domain.ErrForbidden)
	}
	actor, err := svc.store.GetAccountByID(ctx, actorAccountID)
	if err != nil {
		return nil, fmt.Errorf("GetAccountByID(actor): %w", err)
	}

	state := domain.FollowStateAccepted
	if target.Locked {
		state = domain.FollowStatePending
	}
	if target.Domain != nil {
		state = domain.FollowStatePending
	}

	existing, err := svc.store.GetFollow(ctx, actorAccountID, targetAccountID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		slog.WarnContext(ctx, "Follow: check existing follow", slog.Any("error", err))
	}
	if existing != nil {
		if existing.State == domain.FollowStateAccepted || existing.State == domain.FollowStatePending {
			rel, err := svc.accounts.GetRelationship(ctx, actorAccountID, targetAccountID)
			if err != nil {
				return nil, fmt.Errorf("Follow GetRelationship existing: %w", err)
			}
			return rel, nil
		}
	}

	var follow *domain.Follow
	err = svc.store.WithTx(ctx, func(tx store.Store) error {
		var txErr error
		follow, txErr = tx.CreateFollow(ctx, store.CreateFollowInput{
			ID:        uid.New(),
			AccountID: actorAccountID,
			TargetID:  targetAccountID,
			State:     state,
			APID:      nil,
		})
		if txErr != nil {
			return fmt.Errorf("CreateFollow: %w", txErr)
		}
		if state == domain.FollowStateAccepted {
			if txErr := tx.IncrementFollowingCount(ctx, actorAccountID); txErr != nil {
				return fmt.Errorf("IncrementFollowingCount: %w", txErr)
			}
			if txErr := tx.IncrementFollowersCount(ctx, targetAccountID); txErr != nil {
				return fmt.Errorf("IncrementFollowersCount: %w", txErr)
			}
		}
		return events.EmitEvent(ctx, tx, domain.EventFollowCreated, "follow", follow.ID, domain.FollowCreatedPayload{
			Follow: follow,
			Actor:  actor,
			Target: target,
			Local:  actor.IsLocal(),
		})
	})
	if err != nil {
		return nil, fmt.Errorf("Follow tx: %w", err)
	}

	rel, err = svc.accounts.GetRelationship(ctx, actorAccountID, targetAccountID)
	if err != nil {
		return nil, fmt.Errorf("Follow GetRelationship after: %w", err)
	}
	return rel, nil
}

// Unfollow removes the follow from actor to target. Returns the relationship after the change.
func (svc *followService) Unfollow(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Relationship, error) {
	follow, err := svc.store.GetFollow(ctx, actorAccountID, targetAccountID)
	if err != nil {
		// Follow not found — return the current relationship (unfollow is idempotent).
		rel, relErr := svc.accounts.GetRelationship(ctx, actorAccountID, targetAccountID)
		if relErr != nil {
			slog.WarnContext(ctx, "Unfollow: get relationship fallback", slog.Any("error", relErr))
		}
		if rel != nil {
			return rel, nil
		}
		return nil, fmt.Errorf("GetFollow: %w", err)
	}
	actor, err := svc.store.GetAccountByID(ctx, actorAccountID)
	if err != nil {
		return nil, fmt.Errorf("Unfollow GetAccountByID(actor): %w", err)
	}
	target, err := svc.store.GetAccountByID(ctx, targetAccountID)
	if err != nil {
		return nil, fmt.Errorf("Unfollow GetAccountByID(target): %w", err)
	}

	err = svc.store.WithTx(ctx, func(tx store.Store) error {
		if err := tx.DeleteFollow(ctx, actorAccountID, targetAccountID); err != nil {
			return fmt.Errorf("DeleteFollow: %w", err)
		}
		if err := tx.DecrementFollowingCount(ctx, actorAccountID); err != nil {
			return fmt.Errorf("DecrementFollowingCount: %w", err)
		}
		if err := tx.DecrementFollowersCount(ctx, targetAccountID); err != nil {
			return fmt.Errorf("DecrementFollowersCount: %w", err)
		}
		return events.EmitEvent(ctx, tx, domain.EventFollowRemoved, "follow", follow.ID, domain.FollowRemovedPayload{
			FollowID: follow.ID,
			Actor:    actor,
			Target:   target,
			Local:    actor.IsLocal(),
		})
	})
	if err != nil {
		return nil, fmt.Errorf("Unfollow tx: %w", err)
	}

	rel, err := svc.accounts.GetRelationship(ctx, actorAccountID, targetAccountID)
	if err != nil {
		return nil, fmt.Errorf("Unfollow GetRelationship: %w", err)
	}
	return rel, nil
}

// Block creates a block from actor to target. Removes follow in either direction and any mute. Returns the relationship.
func (svc *followService) Block(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Relationship, error) {
	if actorAccountID == targetAccountID {
		return nil, fmt.Errorf("Block: %w", domain.ErrValidation)
	}
	_, err := svc.store.GetAccountByID(ctx, targetAccountID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("Block target: %w", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("GetAccountByID(target): %w", err)
	}
	actor, err := svc.store.GetAccountByID(ctx, actorAccountID)
	if err != nil {
		return nil, fmt.Errorf("Block GetAccountByID(actor): %w", err)
	}
	target, err := svc.store.GetAccountByID(ctx, targetAccountID)
	if err != nil {
		return nil, fmt.Errorf("Block GetAccountByID(target): %w", err)
	}

	err = svc.store.WithTx(ctx, func(tx store.Store) error {
		// Best-effort follow/mute cleanup: the block is the critical operation. If
		// follow lookups or count decrements fail, we log and continue rather than
		// aborting the block.
		if fw, fwErr := tx.GetFollow(ctx, actorAccountID, targetAccountID); fwErr != nil && !errors.Is(fwErr, domain.ErrNotFound) {
			slog.WarnContext(ctx, "Block: check forward follow", slog.Any("error", fwErr))
		} else if fw != nil {
			if err := tx.DeleteFollow(ctx, actorAccountID, targetAccountID); err != nil {
				return fmt.Errorf("DeleteFollow: %w", err)
			}
			if err := tx.DecrementFollowingCount(ctx, actorAccountID); err != nil {
				slog.WarnContext(ctx, "Block: decrement following count", slog.Any("error", err))
			}
			if err := tx.DecrementFollowersCount(ctx, targetAccountID); err != nil {
				slog.WarnContext(ctx, "Block: decrement followers count", slog.Any("error", err))
			}
		}
		if bw, bwErr := tx.GetFollow(ctx, targetAccountID, actorAccountID); bwErr != nil && !errors.Is(bwErr, domain.ErrNotFound) {
			slog.WarnContext(ctx, "Block: check reverse follow", slog.Any("error", bwErr))
		} else if bw != nil {
			if err := tx.DeleteFollow(ctx, targetAccountID, actorAccountID); err != nil {
				return fmt.Errorf("DeleteFollow reverse: %w", err)
			}
			if err := tx.DecrementFollowingCount(ctx, targetAccountID); err != nil {
				slog.WarnContext(ctx, "Block: decrement following count (reverse)", slog.Any("error", err))
			}
			if err := tx.DecrementFollowersCount(ctx, actorAccountID); err != nil {
				slog.WarnContext(ctx, "Block: decrement followers count (reverse)", slog.Any("error", err))
			}
		}
		if err := tx.DeleteMute(ctx, actorAccountID, targetAccountID); err != nil && !errors.Is(err, domain.ErrNotFound) {
			slog.WarnContext(ctx, "Block: delete mute", slog.Any("error", err))
		}
		if txErr := tx.CreateBlock(ctx, store.CreateBlockInput{
			ID:        uid.New(),
			AccountID: actorAccountID,
			TargetID:  targetAccountID,
		}); txErr != nil {
			return fmt.Errorf("CreateBlock: %w", txErr)
		}
		return events.EmitEvent(ctx, tx, domain.EventBlockCreated, "block", actorAccountID+":"+targetAccountID, domain.BlockCreatedPayload{
			Actor:  actor,
			Target: target,
			Local:  actor.IsLocal(),
		})
	})
	if err != nil {
		return nil, fmt.Errorf("Block tx: %w", err)
	}
	rel, err := svc.accounts.GetRelationship(ctx, actorAccountID, targetAccountID)
	if err != nil {
		return nil, fmt.Errorf("Block GetRelationship: %w", err)
	}
	return rel, nil
}

// Unblock removes the block from actor to target. Returns the relationship.
func (svc *followService) Unblock(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Relationship, error) {
	actor, err := svc.store.GetAccountByID(ctx, actorAccountID)
	if err != nil {
		return nil, fmt.Errorf("Unblock GetAccountByID(actor): %w", err)
	}
	target, err := svc.store.GetAccountByID(ctx, targetAccountID)
	if err != nil {
		return nil, fmt.Errorf("Unblock GetAccountByID(target): %w", err)
	}
	err = svc.store.WithTx(ctx, func(tx store.Store) error {
		if err := tx.DeleteBlock(ctx, actorAccountID, targetAccountID); err != nil {
			return fmt.Errorf("DeleteBlock: %w", err)
		}
		return events.EmitEvent(ctx, tx, domain.EventBlockRemoved, "block", actorAccountID+":"+targetAccountID, domain.BlockRemovedPayload{
			Actor:  actor,
			Target: target,
			Local:  actor.IsLocal(),
		})
	})
	if err != nil {
		return nil, fmt.Errorf("Unblock: %w", err)
	}
	rel, err := svc.accounts.GetRelationship(ctx, actorAccountID, targetAccountID)
	if err != nil {
		return nil, fmt.Errorf("Unblock GetRelationship: %w", err)
	}
	return rel, nil
}

// Mute creates or updates a mute from actor to target. notifications controls hide_notifications.
func (svc *followService) Mute(ctx context.Context, actorAccountID, targetAccountID string, hideNotifications bool) (*domain.Relationship, error) {
	if actorAccountID == targetAccountID {
		return nil, fmt.Errorf("Mute: %w", domain.ErrValidation)
	}
	_, err := svc.store.GetAccountByID(ctx, targetAccountID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("Mute target: %w", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("GetAccountByID(target): %w", err)
	}
	if err := svc.store.CreateMute(ctx, store.CreateMuteInput{
		ID:                uid.New(),
		AccountID:         actorAccountID,
		TargetID:          targetAccountID,
		HideNotifications: hideNotifications,
	}); err != nil {
		return nil, fmt.Errorf("CreateMute: %w", err)
	}
	rel, err := svc.accounts.GetRelationship(ctx, actorAccountID, targetAccountID)
	if err != nil {
		return nil, fmt.Errorf("Mute GetRelationship: %w", err)
	}
	return rel, nil
}

// Unmute removes the mute from actor to target. Returns the relationship.
func (svc *followService) Unmute(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Relationship, error) {
	if err := svc.store.DeleteMute(ctx, actorAccountID, targetAccountID); err != nil && !errors.Is(err, domain.ErrNotFound) {
		slog.WarnContext(ctx, "Unmute: delete mute", slog.Any("error", err))
	}
	rel, err := svc.accounts.GetRelationship(ctx, actorAccountID, targetAccountID)
	if err != nil {
		return nil, fmt.Errorf("Unmute GetRelationship: %w", err)
	}
	return rel, nil
}

// GetFollow returns the follow relationship from actor to target, or ErrNotFound.
func (svc *followService) GetFollow(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Follow, error) {
	f, err := svc.store.GetFollow(ctx, actorAccountID, targetAccountID)
	if err != nil {
		return nil, fmt.Errorf("GetFollow: %w", err)
	}
	return f, nil
}

// GetFollowers returns the list of followers for the given account (paginated).
func (svc *followService) GetFollowers(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, error) {
	list, err := svc.store.GetFollowers(ctx, accountID, maxID, limit)
	if err != nil {
		return nil, fmt.Errorf("GetFollowers: %w", err)
	}
	return list, nil
}

// GetFollowing returns the list of accounts the given account follows (paginated).
func (svc *followService) GetFollowing(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, error) {
	list, err := svc.store.GetFollowing(ctx, accountID, maxID, limit)
	if err != nil {
		return nil, fmt.Errorf("GetFollowing: %w", err)
	}
	return list, nil
}

// ListPendingFollowRequests returns accounts that have requested to follow the target (paginated).
func (svc *followService) ListPendingFollowRequests(ctx context.Context, targetAccountID string, maxID *string, limit int) ([]domain.Account, *string, error) {
	accounts, nextCursor, err := svc.store.GetPendingFollowRequests(ctx, targetAccountID, maxID, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("GetPendingFollowRequests: %w", err)
	}
	return accounts, nextCursor, nil
}

// ListBlockedAccounts returns blocked accounts for the given account (paginated by block id).
func (svc *followService) ListBlockedAccounts(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, *string, error) {
	limit = ClampLimit(limit, DefaultServiceListLimit, MaxServicePageLimit)
	accounts, nextCursor, err := svc.store.ListBlockedAccounts(ctx, accountID, maxID, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("ListBlockedAccounts: %w", err)
	}
	return accounts, nextCursor, nil
}

// ListMutedAccounts returns muted accounts for the given account (paginated by mute id).
func (svc *followService) ListMutedAccounts(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, *string, error) {
	limit = ClampLimit(limit, DefaultServiceListLimit, MaxServicePageLimit)
	accounts, nextCursor, err := svc.store.ListMutedAccounts(ctx, accountID, maxID, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("ListMutedAccounts: %w", err)
	}
	return accounts, nextCursor, nil
}

// AuthorizeFollowRequest accepts a pending follow request (requesterAccountID requested to follow targetAccountID).
func (svc *followService) AuthorizeFollowRequest(ctx context.Context, targetAccountID, requesterAccountID string) error {
	follow, err := svc.store.GetFollow(ctx, requesterAccountID, targetAccountID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("follow request: %w", domain.ErrNotFound)
		}
		return fmt.Errorf("GetFollow: %w", err)
	}
	if follow.State != domain.FollowStatePending {
		return nil
	}
	if err := svc.remoteFollows.AcceptFollow(ctx, follow.ID); err != nil {
		return fmt.Errorf("AcceptFollow: %w", err)
	}
	return nil
}

// RejectFollowRequest rejects a pending follow request.
func (svc *followService) RejectFollowRequest(ctx context.Context, targetAccountID, requesterAccountID string) error {
	follow, err := svc.store.GetFollow(ctx, requesterAccountID, targetAccountID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("follow request: %w", domain.ErrNotFound)
		}
		return fmt.Errorf("GetFollow: %w", err)
	}
	if follow.State != domain.FollowStatePending {
		return nil
	}
	if err := svc.store.DeleteFollow(ctx, requesterAccountID, targetAccountID); err != nil {
		return fmt.Errorf("DeleteFollow: %w", err)
	}
	return nil
}
