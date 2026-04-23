package cli

// seed.go — database seeding helpers for the fanout load test.

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/store/postgres"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// FanoutSeeder seeds remote accounts and accepted follows into the database.
type FanoutSeeder struct {
	s store.Store
}

// NewFanoutSeeder connects to the database and returns a FanoutSeeder.
func NewFanoutSeeder(ctx context.Context, dbURL string) (*FanoutSeeder, error) {
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return nil, fmt.Errorf("seed: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("seed: ping: %w", err)
	}
	return &FanoutSeeder{s: postgres.New(pool)}, nil
}

// SeedFollowers creates n remote accounts and accepted follow records pointing to targetID.
// Inbox URLs are distributed across the given inboxURLs slice (round-robin).
// Returns the IDs of the created accounts so they can be cleaned up later.
func (f *FanoutSeeder) SeedFollowers(ctx context.Context, targetID string, n int, inboxURLs []string) ([]string, error) {
	accountIDs := make([]string, 0, n)
	for i := range n {
		inboxURL := inboxURLs[i%len(inboxURLs)]
		remoteDomain := fmt.Sprintf("loadtest-%d.example.invalid", i)
		actorID := fmt.Sprintf("https://%s/users/loadtest%d", remoteDomain, i)
		accountID := uid.New()

		_, err := f.s.CreateAccount(ctx, store.CreateAccountInput{
			ID:           accountID,
			Username:     fmt.Sprintf("loadtest%d", i),
			Domain:       &remoteDomain,
			APID:         actorID,
			InboxURL:     inboxURL,
			OutboxURL:    actorID + "/outbox",
			FollowersURL: actorID + "/followers",
			FollowingURL: actorID + "/following",
			PublicKey:    "placeholder",
		})
		if err != nil {
			return accountIDs, fmt.Errorf("seed: create account %d: %w", i, err)
		}
		accountIDs = append(accountIDs, accountID)

		_, err = f.s.CreateFollow(ctx, store.CreateFollowInput{
			ID:        uid.New(),
			AccountID: accountID,
			TargetID:  targetID,
			State:     domain.FollowStateAccepted,
		})
		if err != nil {
			return accountIDs, fmt.Errorf("seed: create follow %d: %w", i, err)
		}
	}
	return accountIDs, nil
}

// CleanupFollowers deletes the seeded follows and accounts.
// Follows must be removed before accounts due to the foreign key constraint.
func (f *FanoutSeeder) CleanupFollowers(ctx context.Context, targetID string, accountIDs []string) error {
	for _, id := range accountIDs {
		if err := f.s.DeleteFollow(ctx, id, targetID); err != nil {
			return fmt.Errorf("seed: delete follow %s→%s: %w", id, targetID, err)
		}
		if _, err := f.s.DeleteAccount(ctx, id); err != nil {
			return fmt.Errorf("seed: delete account %s: %w", id, err)
		}
	}
	return nil
}

// GetLocalAccount returns the local account for the given username.
func (f *FanoutSeeder) GetLocalAccount(ctx context.Context, username string) (*domain.Account, error) {
	acc, err := f.s.GetLocalAccountByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("seed: get local account %s: %w", username, err)
	}
	return acc, nil
}
