package postgres

import (
	"context"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	db "github.com/chairswithlegs/monstera/internal/store/postgres/generated"
)

// Store surface for the domain-block purge flow (issue #104). Tracks
// progress of the async account/status/media purge triggered by a
// severity=suspend domain block.

func (s *PostgresStore) CreateDomainBlockPurge(ctx context.Context, blockID, domainName string) error {
	return mapErr(s.q.CreateDomainBlockPurge(ctx, db.CreateDomainBlockPurgeParams{
		BlockID: blockID,
		Domain:  domainName,
	}))
}

func (s *PostgresStore) GetDomainBlockPurge(ctx context.Context, blockID string) (*domain.DomainBlockPurge, error) {
	row, err := s.q.GetDomainBlockPurge(ctx, blockID)
	if err != nil {
		return nil, mapErr(err)
	}
	return &domain.DomainBlockPurge{
		BlockID:     row.BlockID,
		Domain:      row.Domain,
		Cursor:      row.Cursor,
		CreatedAt:   pgTime(row.CreatedAt),
		CompletedAt: pgTimePtr(row.CompletedAt),
	}, nil
}

func (s *PostgresStore) UpdateDomainBlockPurgeCursor(ctx context.Context, blockID, cursor string) error {
	return mapErr(s.q.UpdateDomainBlockPurgeCursor(ctx, db.UpdateDomainBlockPurgeCursorParams{
		BlockID: blockID,
		Cursor:  &cursor,
	}))
}

func (s *PostgresStore) MarkDomainBlockPurgeComplete(ctx context.Context, blockID string) error {
	return mapErr(s.q.MarkDomainBlockPurgeComplete(ctx, blockID))
}

func (s *PostgresStore) ListDomainBlocksWithPurge(ctx context.Context) ([]store.DomainBlockWithPurge, error) {
	rows, err := s.q.ListDomainBlocksWithPurge(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]store.DomainBlockWithPurge, 0, len(rows))
	for _, r := range rows {
		item := store.DomainBlockWithPurge{
			Block: domain.DomainBlock{
				ID:        r.BlockID,
				Domain:    r.Domain,
				Severity:  r.Severity,
				Reason:    r.Reason,
				CreatedAt: pgTime(r.CreatedAt),
			},
		}
		if r.PurgeCreatedAt.Valid {
			item.Purge = &domain.DomainBlockPurge{
				BlockID:     r.BlockID,
				Domain:      r.Domain,
				Cursor:      r.PurgeCursor,
				CreatedAt:   pgTime(r.PurgeCreatedAt),
				CompletedAt: pgTimePtr(r.PurgeCompletedAt),
			}
		}
		out = append(out, item)
	}
	return out, nil
}

// ListRemoteAccountsByDomainPaginated returns the next page of remote
// account ids on the given domain, using keyset pagination (id > cursor).
// Pass cursor="" to start at the beginning.
func (s *PostgresStore) ListRemoteAccountsByDomainPaginated(ctx context.Context, domainName, cursor string, limit int) ([]string, error) {
	rows, err := s.q.ListRemoteAccountsByDomainPaginated(ctx, db.ListRemoteAccountsByDomainPaginatedParams{
		Domain:  &domainName,
		Column2: cursor,
		Limit:   int32(limit), //nolint:gosec // G115: limit bounded by caller
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return rows, nil
}

// CountRemoteAccountsByDomainAfterCursor returns the number of remote
// account rows on the given domain with id > cursor. Pass cursor="" to
// count all remote accounts on the domain.
func (s *PostgresStore) CountRemoteAccountsByDomainAfterCursor(ctx context.Context, domainName, cursor string) (int64, error) {
	n, err := s.q.CountRemoteAccountsByDomainAfterCursor(ctx, db.CountRemoteAccountsByDomainAfterCursorParams{
		Domain:  &domainName,
		Column2: cursor,
	})
	if err != nil {
		return 0, mapErr(err)
	}
	return n, nil
}

// DeleteStatusesByAccountIDBatched hard-deletes up to `limit` statuses owned
// by accountID in a single statement, returning the deleted ids. Intended to
// be called in a loop until the return slice is empty. DB-level CASCADE
// cleans up dependent rows.
func (s *PostgresStore) DeleteStatusesByAccountIDBatched(ctx context.Context, accountID string, limit int) ([]string, error) {
	ids, err := s.q.DeleteStatusesByAccountIDBatched(ctx, db.DeleteStatusesByAccountIDBatchedParams{
		AccountID: accountID,
		Limit:     int32(limit), //nolint:gosec // G115: limit bounded by caller
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return ids, nil
}

// SetAccountsDomainSuspendedByDomain flips the domain_suspended flag for
// every account on the given domain. Returns the count of rows updated.
func (s *PostgresStore) SetAccountsDomainSuspendedByDomain(ctx context.Context, domainName string, suspended bool) (int64, error) {
	n, err := s.q.SetAccountsDomainSuspendedByDomain(ctx, db.SetAccountsDomainSuspendedByDomainParams{
		Domain:          &domainName,
		DomainSuspended: suspended,
	})
	if err != nil {
		return 0, mapErr(err)
	}
	return n, nil
}
