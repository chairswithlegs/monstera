package postgres

import (
	"context"
	"fmt"

	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	db "github.com/chairswithlegs/monstera-fed/internal/store/postgres/generated"
)

const noCursorSentinel = "ZZZZZZZZZZZZZZZZZZZZZZZZZZ"

// Ensure PostgresStore implements store.Store.
var _ store.Store = (*PostgresStore)(nil)

func toDbCreateAccountParams(in store.CreateAccountInput) db.CreateAccountParams {
	return db.CreateAccountParams{
		ID:           in.ID,
		Username:     in.Username,
		Domain:       in.Domain,
		DisplayName:  in.DisplayName,
		Note:         in.Note,
		PublicKey:    in.PublicKey,
		PrivateKey:   in.PrivateKey,
		InboxUrl:     in.InboxURL,
		OutboxUrl:    in.OutboxURL,
		FollowersUrl: in.FollowersURL,
		FollowingUrl: in.FollowingURL,
		ApID:         in.APID,
		ApRaw:        in.ApRaw,
		Bot:          in.Bot,
		Locked:       in.Locked,
	}
}

func toDbCreateUserParams(in store.CreateUserInput) db.CreateUserParams {
	return db.CreateUserParams{
		ID:           in.ID,
		AccountID:    in.AccountID,
		Email:        in.Email,
		PasswordHash: in.PasswordHash,
		Role:         in.Role,
	}
}

func toDbCreateStatusParams(in store.CreateStatusInput) db.CreateStatusParams {
	return db.CreateStatusParams{
		ID:             in.ID,
		Uri:            in.URI,
		AccountID:      in.AccountID,
		Text:           in.Text,
		Content:        in.Content,
		ContentWarning: in.ContentWarning,
		Visibility:     in.Visibility,
		Language:       in.Language,
		InReplyToID:    in.InReplyToID,
		ReblogOfID:     in.ReblogOfID,
		ApID:           in.APID,
		ApRaw:          in.ApRaw,
		Sensitive:      in.Sensitive,
		Local:          in.Local,
	}
}

func (s *PostgresStore) CreateAccount(ctx context.Context, in store.CreateAccountInput) (*domain.Account, error) {
	dbAcc, err := s.q.CreateAccount(ctx, toDbCreateAccountParams(in))
	if err != nil {
		return nil, mapErr(err)
	}
	acc := ToDomainAccount(dbAcc)
	return &acc, nil
}

func (s *PostgresStore) GetAccountByID(ctx context.Context, id string) (*domain.Account, error) {
	dbAcc, err := s.q.GetAccountByID(ctx, id)
	if err != nil {
		return nil, mapErr(err)
	}
	acc := ToDomainAccount(dbAcc)
	return &acc, nil
}

func (s *PostgresStore) GetLocalAccountByUsername(ctx context.Context, username string) (*domain.Account, error) {
	dbAcc, err := s.q.GetLocalAccountByUsername(ctx, username)
	if err != nil {
		return nil, mapErr(err)
	}
	acc := ToDomainAccount(dbAcc)
	return &acc, nil
}

func (s *PostgresStore) GetRemoteAccountByUsername(ctx context.Context, username string, domain *string) (*domain.Account, error) {
	dbAcc, err := s.q.GetRemoteAccountByUsername(ctx, db.GetRemoteAccountByUsernameParams{
		Username: username,
		Domain:   domain,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	acc := ToDomainAccount(dbAcc)
	return &acc, nil
}

func (s *PostgresStore) CreateUser(ctx context.Context, in store.CreateUserInput) (*domain.User, error) {
	dbUser, err := s.q.CreateUser(ctx, toDbCreateUserParams(in))
	if err != nil {
		return nil, mapErr(err)
	}
	u := ToDomainUser(dbUser)
	return &u, nil
}

func (s *PostgresStore) CreateStatus(ctx context.Context, in store.CreateStatusInput) (*domain.Status, error) {
	dbSt, err := s.q.CreateStatus(ctx, toDbCreateStatusParams(in))
	if err != nil {
		return nil, mapErr(err)
	}
	st := ToDomainStatus(dbSt)
	return &st, nil
}

func (s *PostgresStore) GetStatusByID(ctx context.Context, id string) (*domain.Status, error) {
	dbSt, err := s.q.GetStatusByID(ctx, id)
	if err != nil {
		return nil, mapErr(err)
	}
	st := ToDomainStatus(dbSt)
	return &st, nil
}

func (s *PostgresStore) DeleteStatus(ctx context.Context, id string) error {
	if err := s.q.SoftDeleteStatus(ctx, id); err != nil {
		return fmt.Errorf("DeleteStatus(%s): %w", id, mapErr(err))
	}
	return nil
}

func (s *PostgresStore) IncrementStatusesCount(ctx context.Context, accountID string) error {
	return mapErr(s.q.IncrementStatusesCount(ctx, accountID))
}

func (s *PostgresStore) DecrementStatusesCount(ctx context.Context, accountID string) error {
	return mapErr(s.q.DecrementStatusesCount(ctx, accountID))
}

func (s *PostgresStore) GetHomeTimeline(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, error) {
	cursor := noCursorSentinel
	if maxID != nil && *maxID != "" {
		cursor = *maxID
	}
	dbRows, err := s.q.GetHomeTimeline(ctx, db.GetHomeTimelineParams{
		AccountID: accountID,
		Column2:   cursor,
		Limit:     int32(limit), //nolint:gosec // clamped by TimelineService before reaching this layer
	})
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.Status, 0, len(dbRows))
	for _, r := range dbRows {
		out = append(out, ToDomainStatus(r))
	}
	return out, nil
}

func (s *PostgresStore) GetPublicTimeline(ctx context.Context, localOnly bool, maxID *string, limit int) ([]domain.Status, error) {
	cursor := noCursorSentinel
	if maxID != nil && *maxID != "" {
		cursor = *maxID
	}
	dbRows, err := s.q.GetPublicTimeline(ctx, db.GetPublicTimelineParams{
		Column1: localOnly,
		Column2: cursor,
		Limit:   int32(limit), //nolint:gosec // clamped by TimelineService before reaching this layer
	})
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.Status, 0, len(dbRows))
	for _, r := range dbRows {
		out = append(out, ToDomainStatus(r))
	}
	return out, nil
}
