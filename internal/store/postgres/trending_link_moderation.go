package postgres

import (
	"context"
	"fmt"
)

// AddTrendingLinkDenylist adds a URL to the trending link denylist.
func (s *PostgresStore) AddTrendingLinkDenylist(ctx context.Context, url string) error {
	const q = `INSERT INTO trending_link_denylist (url) VALUES ($1) ON CONFLICT (url) DO NOTHING`
	if _, err := s.pool.Exec(ctx, q, url); err != nil {
		return fmt.Errorf("AddTrendingLinkDenylist: %w", mapErr(err))
	}
	return nil
}

// RemoveTrendingLinkDenylist removes a URL from the trending link denylist.
func (s *PostgresStore) RemoveTrendingLinkDenylist(ctx context.Context, url string) error {
	const q = `DELETE FROM trending_link_denylist WHERE url = $1`
	if _, err := s.pool.Exec(ctx, q, url); err != nil {
		return fmt.Errorf("RemoveTrendingLinkDenylist: %w", mapErr(err))
	}
	return nil
}

// ListTrendingLinkDenylist returns all URLs in the trending link denylist.
func (s *PostgresStore) ListTrendingLinkDenylist(ctx context.Context) ([]string, error) {
	const q = `SELECT url FROM trending_link_denylist ORDER BY created_at DESC`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("ListTrendingLinkDenylist: %w", mapErr(err))
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			return nil, fmt.Errorf("ListTrendingLinkDenylist scan: %w", mapErr(err))
		}
		out = append(out, url)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListTrendingLinkDenylist rows: %w", mapErr(err))
	}
	return out, nil
}
