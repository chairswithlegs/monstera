package postgres

import (
	"context"

	db "github.com/chairswithlegs/monstera-fed/internal/store/postgres/generated"
)

// Wrapper methods translate pgx/pgconn errors to domain errors (ErrNotFound, ErrConflict).
// They shadow the embedded *db.Queries methods so store callers receive domain errors.

func (s *PostgresStore) GetAccessToken(ctx context.Context, token string) (db.OauthAccessToken, error) {
	tok, err := s.Queries.GetAccessToken(ctx, token)
	return tok, mapErr(err)
}

func (s *PostgresStore) GetAccountByAPID(ctx context.Context, apID string) (db.Account, error) {
	a, err := s.Queries.GetAccountByAPID(ctx, apID)
	return a, mapErr(err)
}

func (s *PostgresStore) GetAccountByID(ctx context.Context, id string) (db.Account, error) {
	a, err := s.Queries.GetAccountByID(ctx, id)
	return a, mapErr(err)
}

func (s *PostgresStore) GetApplicationByClientID(ctx context.Context, clientID string) (db.OauthApplication, error) {
	app, err := s.Queries.GetApplicationByClientID(ctx, clientID)
	return app, mapErr(err)
}

func (s *PostgresStore) GetAuthorizationCode(ctx context.Context, code string) (db.OauthAuthorizationCode, error) {
	ac, err := s.Queries.GetAuthorizationCode(ctx, code)
	return ac, mapErr(err)
}

func (s *PostgresStore) GetBlock(ctx context.Context, arg db.GetBlockParams) (db.Block, error) {
	b, err := s.Queries.GetBlock(ctx, arg)
	return b, mapErr(err)
}

func (s *PostgresStore) GetCustomEmojiByShortcode(ctx context.Context, shortcode string) (db.CustomEmoji, error) {
	e, err := s.Queries.GetCustomEmojiByShortcode(ctx, shortcode)
	return e, mapErr(err)
}

func (s *PostgresStore) GetDomainBlock(ctx context.Context, domain string) (db.DomainBlock, error) {
	b, err := s.Queries.GetDomainBlock(ctx, domain)
	return b, mapErr(err)
}

func (s *PostgresStore) GetEmailToken(ctx context.Context, tokenHash string) (db.EmailToken, error) {
	t, err := s.Queries.GetEmailToken(ctx, tokenHash)
	return t, mapErr(err)
}

func (s *PostgresStore) GetFollow(ctx context.Context, arg db.GetFollowParams) (db.Follow, error) {
	f, err := s.Queries.GetFollow(ctx, arg)
	return f, mapErr(err)
}

func (s *PostgresStore) GetFollowByAPID(ctx context.Context, apID *string) (db.Follow, error) {
	f, err := s.Queries.GetFollowByAPID(ctx, apID)
	return f, mapErr(err)
}

func (s *PostgresStore) GetFollowByID(ctx context.Context, id string) (db.Follow, error) {
	f, err := s.Queries.GetFollowByID(ctx, id)
	return f, mapErr(err)
}

func (s *PostgresStore) GetHashtagByName(ctx context.Context, lower string) (db.Hashtag, error) {
	h, err := s.Queries.GetHashtagByName(ctx, lower)
	return h, mapErr(err)
}

func (s *PostgresStore) GetInviteByCode(ctx context.Context, code string) (db.Invite, error) {
	inv, err := s.Queries.GetInviteByCode(ctx, code)
	return inv, mapErr(err)
}

func (s *PostgresStore) GetLocalAccountByUsername(ctx context.Context, username string) (db.Account, error) {
	a, err := s.Queries.GetLocalAccountByUsername(ctx, username)
	return a, mapErr(err)
}

func (s *PostgresStore) GetMediaAttachment(ctx context.Context, id string) (db.MediaAttachment, error) {
	m, err := s.Queries.GetMediaAttachment(ctx, id)
	return m, mapErr(err)
}

func (s *PostgresStore) GetMute(ctx context.Context, arg db.GetMuteParams) (db.Mute, error) {
	m, err := s.Queries.GetMute(ctx, arg)
	return m, mapErr(err)
}

func (s *PostgresStore) GetNotification(ctx context.Context, arg db.GetNotificationParams) (db.Notification, error) {
	n, err := s.Queries.GetNotification(ctx, arg)
	return n, mapErr(err)
}

func (s *PostgresStore) GetOrCreateHashtag(ctx context.Context, arg db.GetOrCreateHashtagParams) (db.Hashtag, error) {
	h, err := s.Queries.GetOrCreateHashtag(ctx, arg)
	return h, mapErr(err)
}

func (s *PostgresStore) GetReblogByAccountAndTarget(ctx context.Context, arg db.GetReblogByAccountAndTargetParams) (db.Status, error) {
	st, err := s.Queries.GetReblogByAccountAndTarget(ctx, arg)
	return st, mapErr(err)
}

func (s *PostgresStore) GetRemoteAccountByUsername(ctx context.Context, arg db.GetRemoteAccountByUsernameParams) (db.Account, error) {
	a, err := s.Queries.GetRemoteAccountByUsername(ctx, arg)
	return a, mapErr(err)
}

func (s *PostgresStore) GetReport(ctx context.Context, id string) (db.Report, error) {
	r, err := s.Queries.GetReport(ctx, id)
	return r, mapErr(err)
}

func (s *PostgresStore) GetServerFilter(ctx context.Context, id string) (db.ServerFilter, error) {
	f, err := s.Queries.GetServerFilter(ctx, id)
	return f, mapErr(err)
}

func (s *PostgresStore) GetSetting(ctx context.Context, key string) (string, error) {
	v, err := s.Queries.GetSetting(ctx, key)
	return v, mapErr(err)
}

func (s *PostgresStore) GetStatusByAPID(ctx context.Context, apID string) (db.Status, error) {
	st, err := s.Queries.GetStatusByAPID(ctx, apID)
	return st, mapErr(err)
}

func (s *PostgresStore) GetStatusByID(ctx context.Context, id string) (db.Status, error) {
	st, err := s.Queries.GetStatusByID(ctx, id)
	return st, mapErr(err)
}

func (s *PostgresStore) GetUserByAccountID(ctx context.Context, accountID string) (db.User, error) {
	u, err := s.Queries.GetUserByAccountID(ctx, accountID)
	return u, mapErr(err)
}

func (s *PostgresStore) GetUserByEmail(ctx context.Context, email string) (db.User, error) {
	u, err := s.Queries.GetUserByEmail(ctx, email)
	return u, mapErr(err)
}

func (s *PostgresStore) GetUserByID(ctx context.Context, id string) (db.User, error) {
	u, err := s.Queries.GetUserByID(ctx, id)
	return u, mapErr(err)
}

func (s *PostgresStore) CreateAccessToken(ctx context.Context, arg db.CreateAccessTokenParams) (db.OauthAccessToken, error) {
	tok, err := s.Queries.CreateAccessToken(ctx, arg)
	return tok, mapErr(err)
}

func (s *PostgresStore) CreateAccount(ctx context.Context, arg db.CreateAccountParams) (db.Account, error) {
	a, err := s.Queries.CreateAccount(ctx, arg)
	return a, mapErr(err)
}

func (s *PostgresStore) CreateAdminAction(ctx context.Context, arg db.CreateAdminActionParams) (db.AdminAction, error) {
	aa, err := s.Queries.CreateAdminAction(ctx, arg)
	return aa, mapErr(err)
}

func (s *PostgresStore) CreateApplication(ctx context.Context, arg db.CreateApplicationParams) (db.OauthApplication, error) {
	app, err := s.Queries.CreateApplication(ctx, arg)
	return app, mapErr(err)
}

func (s *PostgresStore) CreateAuthorizationCode(ctx context.Context, arg db.CreateAuthorizationCodeParams) (db.OauthAuthorizationCode, error) {
	ac, err := s.Queries.CreateAuthorizationCode(ctx, arg)
	return ac, mapErr(err)
}

func (s *PostgresStore) CreateBlock(ctx context.Context, arg db.CreateBlockParams) (db.Block, error) {
	b, err := s.Queries.CreateBlock(ctx, arg)
	return b, mapErr(err)
}

func (s *PostgresStore) CreateCustomEmoji(ctx context.Context, arg db.CreateCustomEmojiParams) (db.CustomEmoji, error) {
	e, err := s.Queries.CreateCustomEmoji(ctx, arg)
	return e, mapErr(err)
}

func (s *PostgresStore) CreateDomainBlock(ctx context.Context, arg db.CreateDomainBlockParams) (db.DomainBlock, error) {
	b, err := s.Queries.CreateDomainBlock(ctx, arg)
	return b, mapErr(err)
}

func (s *PostgresStore) CreateEmailToken(ctx context.Context, arg db.CreateEmailTokenParams) (db.EmailToken, error) {
	t, err := s.Queries.CreateEmailToken(ctx, arg)
	return t, mapErr(err)
}

func (s *PostgresStore) CreateFavourite(ctx context.Context, arg db.CreateFavouriteParams) (db.Favourite, error) {
	f, err := s.Queries.CreateFavourite(ctx, arg)
	return f, mapErr(err)
}

func (s *PostgresStore) CreateFollow(ctx context.Context, arg db.CreateFollowParams) (db.Follow, error) {
	f, err := s.Queries.CreateFollow(ctx, arg)
	return f, mapErr(err)
}

func (s *PostgresStore) CreateInvite(ctx context.Context, arg db.CreateInviteParams) (db.Invite, error) {
	inv, err := s.Queries.CreateInvite(ctx, arg)
	return inv, mapErr(err)
}

func (s *PostgresStore) CreateMediaAttachment(ctx context.Context, arg db.CreateMediaAttachmentParams) (db.MediaAttachment, error) {
	m, err := s.Queries.CreateMediaAttachment(ctx, arg)
	return m, mapErr(err)
}

func (s *PostgresStore) CreateMute(ctx context.Context, arg db.CreateMuteParams) (db.Mute, error) {
	m, err := s.Queries.CreateMute(ctx, arg)
	return m, mapErr(err)
}

func (s *PostgresStore) CreateNotification(ctx context.Context, arg db.CreateNotificationParams) (db.Notification, error) {
	n, err := s.Queries.CreateNotification(ctx, arg)
	return n, mapErr(err)
}

func (s *PostgresStore) CreateReport(ctx context.Context, arg db.CreateReportParams) (db.Report, error) {
	r, err := s.Queries.CreateReport(ctx, arg)
	return r, mapErr(err)
}

func (s *PostgresStore) CreateServerFilter(ctx context.Context, arg db.CreateServerFilterParams) (db.ServerFilter, error) {
	f, err := s.Queries.CreateServerFilter(ctx, arg)
	return f, mapErr(err)
}

func (s *PostgresStore) CreateStatus(ctx context.Context, arg db.CreateStatusParams) (db.Status, error) {
	st, err := s.Queries.CreateStatus(ctx, arg)
	return st, mapErr(err)
}

func (s *PostgresStore) CreateStatusEdit(ctx context.Context, arg db.CreateStatusEditParams) (db.StatusEdit, error) {
	se, err := s.Queries.CreateStatusEdit(ctx, arg)
	return se, mapErr(err)
}

func (s *PostgresStore) CreateUser(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
	u, err := s.Queries.CreateUser(ctx, arg)
	return u, mapErr(err)
}

func (s *PostgresStore) UpdateAccount(ctx context.Context, arg db.UpdateAccountParams) (db.Account, error) {
	a, err := s.Queries.UpdateAccount(ctx, arg)
	return a, mapErr(err)
}

func (s *PostgresStore) UpdateDomainBlock(ctx context.Context, arg db.UpdateDomainBlockParams) (db.DomainBlock, error) {
	b, err := s.Queries.UpdateDomainBlock(ctx, arg)
	return b, mapErr(err)
}

func (s *PostgresStore) UpdateMediaAttachment(ctx context.Context, arg db.UpdateMediaAttachmentParams) (db.MediaAttachment, error) {
	m, err := s.Queries.UpdateMediaAttachment(ctx, arg)
	return m, mapErr(err)
}

func (s *PostgresStore) UpdateServerFilter(ctx context.Context, arg db.UpdateServerFilterParams) (db.ServerFilter, error) {
	f, err := s.Queries.UpdateServerFilter(ctx, arg)
	return f, mapErr(err)
}

func (s *PostgresStore) UpdateStatus(ctx context.Context, arg db.UpdateStatusParams) (db.Status, error) {
	st, err := s.Queries.UpdateStatus(ctx, arg)
	return st, mapErr(err)
}

