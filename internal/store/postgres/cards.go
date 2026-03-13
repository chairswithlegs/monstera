package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
)

// UpsertStatusCard inserts or updates a status card row.
func (s *PostgresStore) UpsertStatusCard(ctx context.Context, in store.UpsertStatusCardInput) error {
	const q = `
		INSERT INTO status_cards
			(status_id, processing_state, url, title, description, card_type, provider_name, provider_url, image_url, width, height, fetched_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW())
		ON CONFLICT (status_id) DO UPDATE SET
			processing_state = EXCLUDED.processing_state,
			url              = EXCLUDED.url,
			title            = EXCLUDED.title,
			description      = EXCLUDED.description,
			card_type        = EXCLUDED.card_type,
			provider_name    = EXCLUDED.provider_name,
			provider_url     = EXCLUDED.provider_url,
			image_url        = EXCLUDED.image_url,
			width            = EXCLUDED.width,
			height           = EXCLUDED.height,
			fetched_at       = EXCLUDED.fetched_at`

	_, err := s.pool.Exec(ctx, q,
		in.StatusID, in.ProcessingState, in.URL, in.Title, in.Description,
		in.CardType, in.ProviderName, in.ProviderURL, in.ImageURL,
		in.Width, in.Height,
	)
	if err != nil {
		return fmt.Errorf("UpsertStatusCard(%s): %w", in.StatusID, err)
	}
	return nil
}

// GetStatusCard returns the card for the given status, or domain.ErrNotFound if no row exists.
func (s *PostgresStore) GetStatusCard(ctx context.Context, statusID string) (*domain.Card, error) {
	const q = `
		SELECT status_id, processing_state, url, title, description, card_type,
		       provider_name, provider_url, image_url, width, height
		FROM status_cards
		WHERE status_id = $1`

	row := s.pool.QueryRow(ctx, q, statusID)
	var c domain.Card
	var width, height int32
	err := row.Scan(
		&c.StatusID, &c.ProcessingState, &c.URL, &c.Title, &c.Description, &c.Type,
		&c.ProviderName, &c.ProviderURL, &c.ImageURL, &width, &height,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("GetStatusCard(%s): %w", statusID, err)
	}
	c.Width = int(width)
	c.Height = int(height)
	return &c, nil
}

// ListStatusIDsNeedingCards returns status IDs that have no status_cards row,
// were created on or after since, are not reblogs, are not deleted, and have URLs in their content.
func (s *PostgresStore) ListStatusIDsNeedingCards(ctx context.Context, since time.Time, limit int) ([]string, error) {
	const q = `
		SELECT s.id
		FROM statuses s
		LEFT JOIN status_cards sc ON sc.status_id = s.id
		WHERE sc.status_id IS NULL
		  AND s.deleted_at IS NULL
		  AND s.reblog_of_id IS NULL
		  AND s.content LIKE '%http%'
		  AND s.created_at >= $1
		ORDER BY s.created_at DESC
		LIMIT $2`

	rows, err := s.pool.Query(ctx, q, pgtype.Timestamptz{Time: since, Valid: true}, int64(limit))
	if err != nil {
		return nil, fmt.Errorf("ListStatusIDsNeedingCards: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("ListStatusIDsNeedingCards scan: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListStatusIDsNeedingCards rows: %w", err)
	}
	return out, nil
}
