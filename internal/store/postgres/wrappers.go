package postgres

import (
	"context"

	db "github.com/chairswithlegs/monstera-fed/internal/store/postgres/generated"
)

// Wrapper methods translate pgx/pgconn errors to domain errors (ErrNotFound, ErrConflict).
// They shadow the embedded *db.Queries methods so store callers receive domain errors.

func (s *PostgresStore) GetBlock(ctx context.Context, arg db.GetBlockParams) (db.Block, error) {
	b, err := s.q.GetBlock(ctx, arg)
	return b, mapErr(err)
}

func (s *PostgresStore) GetCustomEmojiByShortcode(ctx context.Context, shortcode string) (db.CustomEmoji, error) {
	e, err := s.q.GetCustomEmojiByShortcode(ctx, shortcode)
	return e, mapErr(err)
}

func (s *PostgresStore) GetDomainBlock(ctx context.Context, domain string) (db.DomainBlock, error) {
	b, err := s.q.GetDomainBlock(ctx, domain)
	return b, mapErr(err)
}

func (s *PostgresStore) GetEmailToken(ctx context.Context, tokenHash string) (db.EmailToken, error) {
	t, err := s.q.GetEmailToken(ctx, tokenHash)
	return t, mapErr(err)
}

func (s *PostgresStore) GetFollowByID(ctx context.Context, id string) (db.Follow, error) {
	f, err := s.q.GetFollowByID(ctx, id)
	return f, mapErr(err)
}

func (s *PostgresStore) GetHashtagByName(ctx context.Context, lower string) (db.Hashtag, error) {
	h, err := s.q.GetHashtagByName(ctx, lower)
	return h, mapErr(err)
}

func (s *PostgresStore) GetInviteByCode(ctx context.Context, code string) (db.Invite, error) {
	inv, err := s.q.GetInviteByCode(ctx, code)
	return inv, mapErr(err)
}

func (s *PostgresStore) GetMute(ctx context.Context, arg db.GetMuteParams) (db.Mute, error) {
	m, err := s.q.GetMute(ctx, arg)
	return m, mapErr(err)
}

func (s *PostgresStore) GetReport(ctx context.Context, id string) (db.Report, error) {
	r, err := s.q.GetReport(ctx, id)
	return r, mapErr(err)
}

func (s *PostgresStore) GetServerFilter(ctx context.Context, id string) (db.ServerFilter, error) {
	f, err := s.q.GetServerFilter(ctx, id)
	return f, mapErr(err)
}

func (s *PostgresStore) ListSettings(ctx context.Context) ([]db.InstanceSetting, error) {
	rows, err := s.q.ListSettings(ctx)
	return rows, mapErr(err)
}

func (s *PostgresStore) CountRemoteAccounts(ctx context.Context) (int64, error) {
	n, err := s.q.CountRemoteAccounts(ctx)
	return n, mapErr(err)
}

func (s *PostgresStore) CreateAdminAction(ctx context.Context, arg db.CreateAdminActionParams) (db.AdminAction, error) {
	aa, err := s.q.CreateAdminAction(ctx, arg)
	return aa, mapErr(err)
}

func (s *PostgresStore) CreateCustomEmoji(ctx context.Context, arg db.CreateCustomEmojiParams) (db.CustomEmoji, error) {
	e, err := s.q.CreateCustomEmoji(ctx, arg)
	return e, mapErr(err)
}

func (s *PostgresStore) CreateDomainBlock(ctx context.Context, arg db.CreateDomainBlockParams) (db.DomainBlock, error) {
	b, err := s.q.CreateDomainBlock(ctx, arg)
	return b, mapErr(err)
}

func (s *PostgresStore) CreateEmailToken(ctx context.Context, arg db.CreateEmailTokenParams) (db.EmailToken, error) {
	t, err := s.q.CreateEmailToken(ctx, arg)
	return t, mapErr(err)
}

func (s *PostgresStore) CreateInvite(ctx context.Context, arg db.CreateInviteParams) (db.Invite, error) {
	inv, err := s.q.CreateInvite(ctx, arg)
	return inv, mapErr(err)
}

func (s *PostgresStore) CreateMute(ctx context.Context, arg db.CreateMuteParams) (db.Mute, error) {
	m, err := s.q.CreateMute(ctx, arg)
	return m, mapErr(err)
}

func (s *PostgresStore) CreateReport(ctx context.Context, arg db.CreateReportParams) (db.Report, error) {
	r, err := s.q.CreateReport(ctx, arg)
	return r, mapErr(err)
}

func (s *PostgresStore) CreateServerFilter(ctx context.Context, arg db.CreateServerFilterParams) (db.ServerFilter, error) {
	f, err := s.q.CreateServerFilter(ctx, arg)
	return f, mapErr(err)
}

func (s *PostgresStore) UpdateDomainBlock(ctx context.Context, arg db.UpdateDomainBlockParams) (db.DomainBlock, error) {
	b, err := s.q.UpdateDomainBlock(ctx, arg)
	return b, mapErr(err)
}

func (s *PostgresStore) UpdateMediaAttachment(ctx context.Context, arg db.UpdateMediaAttachmentParams) (db.MediaAttachment, error) {
	m, err := s.q.UpdateMediaAttachment(ctx, arg)
	return m, mapErr(err)
}

func (s *PostgresStore) UpdateServerFilter(ctx context.Context, arg db.UpdateServerFilterParams) (db.ServerFilter, error) {
	f, err := s.q.UpdateServerFilter(ctx, arg)
	return f, mapErr(err)
}
