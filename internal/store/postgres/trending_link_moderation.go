package postgres

import (
	"context"
	"fmt"
)

// AddTrendingLinkFilter adds a URL to the trending link filter list.
func (s *PostgresStore) AddTrendingLinkFilter(ctx context.Context, url string) error {
	if err := s.q.AddTrendingLinkFilter(ctx, url); err != nil {
		return fmt.Errorf("AddTrendingLinkFilter: %w", mapErr(err))
	}
	return nil
}

// RemoveTrendingLinkFilter removes a URL from the trending link filter list.
func (s *PostgresStore) RemoveTrendingLinkFilter(ctx context.Context, url string) error {
	if err := s.q.RemoveTrendingLinkFilter(ctx, url); err != nil {
		return fmt.Errorf("RemoveTrendingLinkFilter: %w", mapErr(err))
	}
	return nil
}

// ListTrendingLinkFilters returns all URLs in the trending link filter list.
func (s *PostgresStore) ListTrendingLinkFilters(ctx context.Context) ([]string, error) {
	rows, err := s.q.ListTrendingLinkFilters(ctx)
	if err != nil {
		return nil, fmt.Errorf("ListTrendingLinkFilters: %w", mapErr(err))
	}

	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Url
	}
	return out, nil
}
