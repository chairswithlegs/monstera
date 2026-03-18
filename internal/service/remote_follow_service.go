package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/events"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// RemoteFollowService handles follow operations originating from the federation/ActivityPub layer.
type RemoteFollowService interface {
	GetFollowByAPID(ctx context.Context, apID string) (*domain.Follow, error)
	CreateRemoteFollow(ctx context.Context, actorAccountID, targetAccountID string, state string, apID *string) (*domain.Follow, error)
	AcceptFollow(ctx context.Context, followID string) error
	DeleteRemoteFollow(ctx context.Context, actorAccountID, targetAccountID string) error
	CreateRemoteBlock(ctx context.Context, actorAccountID, targetAccountID string) error
	DeleteRemoteBlock(ctx context.Context, actorAccountID, targetAccountID string) error
	HasLocalFollower(ctx context.Context, accountID string) (bool, error)
	GetFollowerInboxURLsPaginated(ctx context.Context, accountID string, cursor string, limit int) ([]string, error)
}

type remoteFollowService struct {
	store store.Store
}

func NewRemoteFollowService(s store.Store) RemoteFollowService {
	return &remoteFollowService{store: s}
}

func (svc *remoteFollowService) GetFollowByAPID(ctx context.Context, apID string) (*domain.Follow, error) {
	f, err := svc.store.GetFollowByAPID(ctx, apID)
	if err != nil {
		return nil, fmt.Errorf("GetFollowByAPID: %w", err)
	}
	return f, nil
}

func (svc *remoteFollowService) CreateRemoteFollow(ctx context.Context, actorAccountID, targetAccountID string, state string, apID *string) (*domain.Follow, error) {
	actor, err := svc.store.GetAccountByID(ctx, actorAccountID)
	if err != nil {
		return nil, fmt.Errorf("CreateRemoteFollow GetAccountByID(actor): %w", err)
	}
	target, err := svc.store.GetAccountByID(ctx, targetAccountID)
	if err != nil {
		return nil, fmt.Errorf("CreateRemoteFollow GetAccountByID(target): %w", err)
	}
	var follow *domain.Follow
	err = svc.store.WithTx(ctx, func(tx store.Store) error {
		var txErr error
		follow, txErr = tx.CreateFollow(ctx, store.CreateFollowInput{
			ID:        uid.New(),
			AccountID: actorAccountID,
			TargetID:  targetAccountID,
			State:     state,
			APID:      apID,
		})
		if txErr != nil {
			return fmt.Errorf("CreateFollow: %w", txErr)
		}
		if state == domain.FollowStateAccepted {
			if txErr := tx.IncrementFollowersCount(ctx, targetAccountID); txErr != nil {
				return fmt.Errorf("IncrementFollowersCount: %w", txErr)
			}
			if txErr := tx.IncrementFollowingCount(ctx, actorAccountID); txErr != nil {
				return fmt.Errorf("IncrementFollowingCount: %w", txErr)
			}
			if txErr := events.EmitEvent(ctx, tx, domain.EventFollowCreated, "follow", follow.ID, domain.FollowCreatedPayload{
				Follow: follow,
				Actor:  actor,
				Target: target,
			}); txErr != nil {
				return fmt.Errorf("emit follow.created: %w", txErr)
			}
			return events.EmitEvent(ctx, tx, domain.EventFollowAccepted, "follow", follow.ID, domain.FollowAcceptedPayload{
				Follow: follow,
				Target: target,
				Actor:  actor,
			})
		}
		return events.EmitEvent(ctx, tx, domain.EventFollowRequested, "follow", follow.ID, domain.FollowRequestedPayload{
			Follow: follow,
			Actor:  actor,
			Target: target,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("CreateRemoteFollow: %w", err)
	}
	return follow, nil
}

func (svc *remoteFollowService) AcceptFollow(ctx context.Context, followID string) error {
	follow, err := svc.store.GetFollowByID(ctx, followID)
	if err != nil {
		return fmt.Errorf("AcceptFollow GetFollowByID: %w", err)
	}
	if follow.State == domain.FollowStateAccepted {
		return nil
	}
	actor, err := svc.store.GetAccountByID(ctx, follow.AccountID)
	if err != nil {
		return fmt.Errorf("AcceptFollow GetAccountByID(actor): %w", err)
	}
	target, err := svc.store.GetAccountByID(ctx, follow.TargetID)
	if err != nil {
		return fmt.Errorf("AcceptFollow GetAccountByID(target): %w", err)
	}
	err = svc.store.WithTx(ctx, func(tx store.Store) error {
		if txErr := tx.AcceptFollow(ctx, followID); txErr != nil {
			return fmt.Errorf("AcceptFollow: %w", txErr)
		}
		if txErr := tx.IncrementFollowersCount(ctx, follow.TargetID); txErr != nil {
			return fmt.Errorf("IncrementFollowersCount: %w", txErr)
		}
		if txErr := tx.IncrementFollowingCount(ctx, follow.AccountID); txErr != nil {
			return fmt.Errorf("IncrementFollowingCount: %w", txErr)
		}
		return events.EmitEvent(ctx, tx, domain.EventFollowAccepted, "follow", followID, domain.FollowAcceptedPayload{
			Follow: follow,
			Target: target,
			Actor:  actor,
		})
	})
	if err != nil {
		return fmt.Errorf("AcceptFollow tx: %w", err)
	}
	return nil
}

func (svc *remoteFollowService) DeleteRemoteFollow(ctx context.Context, actorAccountID, targetAccountID string) error {
	follow, err := svc.store.GetFollow(ctx, actorAccountID, targetAccountID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("DeleteRemoteFollow: %w", domain.ErrNotFound)
		}
		return fmt.Errorf("DeleteRemoteFollow GetFollow: %w", err)
	}
	actor, err := svc.store.GetAccountByID(ctx, actorAccountID)
	if err != nil {
		return fmt.Errorf("DeleteRemoteFollow GetAccountByID(actor): %w", err)
	}
	target, err := svc.store.GetAccountByID(ctx, targetAccountID)
	if err != nil {
		return fmt.Errorf("DeleteRemoteFollow GetAccountByID(target): %w", err)
	}

	err = svc.store.WithTx(ctx, func(tx store.Store) error {
		if err := tx.DeleteFollow(ctx, actorAccountID, targetAccountID); err != nil {
			return fmt.Errorf("DeleteFollow: %w", err)
		}
		if follow.State == domain.FollowStateAccepted {
			if err := tx.DecrementFollowersCount(ctx, targetAccountID); err != nil {
				return fmt.Errorf("DecrementFollowersCount: %w", err)
			}
			if err := tx.DecrementFollowingCount(ctx, actorAccountID); err != nil {
				return fmt.Errorf("DecrementFollowingCount: %w", err)
			}
		}
		return events.EmitEvent(ctx, tx, domain.EventFollowRemoved, "follow", follow.ID, domain.FollowRemovedPayload{
			FollowID: follow.ID,
			Actor:    actor,
			Target:   target,
		})
	})
	if err != nil {
		return fmt.Errorf("DeleteRemoteFollow: %w", err)
	}
	return nil
}

func (svc *remoteFollowService) CreateRemoteBlock(ctx context.Context, actorAccountID, targetAccountID string) error {
	if actorAccountID == targetAccountID {
		return fmt.Errorf("CreateRemoteBlock: %w", domain.ErrValidation)
	}
	actor, err := svc.store.GetAccountByID(ctx, actorAccountID)
	if err != nil {
		return fmt.Errorf("CreateRemoteBlock GetAccountByID(actor): %w", err)
	}
	target, err := svc.store.GetAccountByID(ctx, targetAccountID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("CreateRemoteBlock target: %w", domain.ErrNotFound)
		}
		return fmt.Errorf("CreateRemoteBlock GetAccountByID(target): %w", err)
	}

	err = svc.store.WithTx(ctx, func(tx store.Store) error {
		if fw, _ := tx.GetFollow(ctx, actorAccountID, targetAccountID); fw != nil {
			if err := tx.DeleteFollow(ctx, actorAccountID, targetAccountID); err != nil {
				return fmt.Errorf("DeleteFollow: %w", err)
			}
			if err := tx.DecrementFollowingCount(ctx, actorAccountID); err != nil {
				return fmt.Errorf("DecrementFollowingCount: %w", err)
			}
			if err := tx.DecrementFollowersCount(ctx, targetAccountID); err != nil {
				return fmt.Errorf("DecrementFollowersCount: %w", err)
			}
		}
		if bw, _ := tx.GetFollow(ctx, targetAccountID, actorAccountID); bw != nil {
			if err := tx.DeleteFollow(ctx, targetAccountID, actorAccountID); err != nil {
				return fmt.Errorf("DeleteFollow reverse: %w", err)
			}
			if err := tx.DecrementFollowingCount(ctx, targetAccountID); err != nil {
				return fmt.Errorf("DecrementFollowingCount reverse: %w", err)
			}
			if err := tx.DecrementFollowersCount(ctx, actorAccountID); err != nil {
				return fmt.Errorf("DecrementFollowersCount reverse: %w", err)
			}
		}
		if err := tx.DeleteMute(ctx, actorAccountID, targetAccountID); err != nil && !errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("DeleteMute: %w", err)
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
		})
	})
	if err != nil {
		return fmt.Errorf("CreateRemoteBlock tx: %w", err)
	}
	return nil
}

func (svc *remoteFollowService) DeleteRemoteBlock(ctx context.Context, actorAccountID, targetAccountID string) error {
	if err := svc.store.DeleteBlock(ctx, actorAccountID, targetAccountID); err != nil {
		return fmt.Errorf("DeleteRemoteBlock: %w", err)
	}
	return nil
}

func (svc *remoteFollowService) HasLocalFollower(ctx context.Context, accountID string) (bool, error) {
	ids, err := svc.store.GetLocalFollowerAccountIDs(ctx, accountID)
	if err != nil {
		return false, fmt.Errorf("GetLocalFollowerAccountIDs: %w", err)
	}
	return len(ids) > 0, nil
}

func (svc *remoteFollowService) GetFollowerInboxURLsPaginated(ctx context.Context, accountID string, cursor string, limit int) ([]string, error) {
	urls, err := svc.store.GetDistinctFollowerInboxURLsPaginated(ctx, accountID, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("GetFollowerInboxURLsPaginated: %w", err)
	}
	return urls, nil
}
