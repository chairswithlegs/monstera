package postgres

import (
	"context"
	"fmt"

	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	db "github.com/chairswithlegs/monstera-fed/internal/store/postgres/generated"
	"github.com/chairswithlegs/monstera-fed/internal/uid"
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

func toDbCreateApplicationParams(in store.CreateApplicationInput) db.CreateApplicationParams {
	return db.CreateApplicationParams{
		ID:           in.ID,
		Name:         in.Name,
		ClientID:     in.ClientID,
		ClientSecret: in.ClientSecret,
		RedirectUris: in.RedirectURIs,
		Scopes:       in.Scopes,
		Website:      in.Website,
	}
}

func toDbCreateAuthorizationCodeParams(in store.CreateAuthorizationCodeInput) db.CreateAuthorizationCodeParams {
	return db.CreateAuthorizationCodeParams{
		ID:                  in.ID,
		Code:                in.Code,
		ApplicationID:       in.ApplicationID,
		AccountID:           in.AccountID,
		RedirectUri:         in.RedirectURI,
		Scopes:              in.Scopes,
		CodeChallenge:       in.CodeChallenge,
		CodeChallengeMethod: in.CodeChallengeMethod,
		ExpiresAt:           timeToPg(in.ExpiresAt),
	}
}

func toDbCreateAccessTokenParams(in store.CreateAccessTokenInput) db.CreateAccessTokenParams {
	return db.CreateAccessTokenParams{
		ID:            in.ID,
		ApplicationID: in.ApplicationID,
		AccountID:     in.AccountID,
		Token:         in.Token,
		Scopes:        in.Scopes,
		ExpiresAt:     timePtrToPg(in.ExpiresAt),
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

func (s *PostgresStore) CreateApplication(ctx context.Context, in store.CreateApplicationInput) (*domain.OAuthApplication, error) {
	app, err := s.q.CreateApplication(ctx, toDbCreateApplicationParams(in))
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainOAuthApplication(app)
	return &d, nil
}

func (s *PostgresStore) GetApplicationByClientID(ctx context.Context, clientID string) (*domain.OAuthApplication, error) {
	app, err := s.q.GetApplicationByClientID(ctx, clientID)
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainOAuthApplication(app)
	return &d, nil
}

func (s *PostgresStore) CreateAuthorizationCode(ctx context.Context, in store.CreateAuthorizationCodeInput) (*domain.OAuthAuthorizationCode, error) {
	ac, err := s.q.CreateAuthorizationCode(ctx, toDbCreateAuthorizationCodeParams(in))
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainOAuthAuthorizationCode(ac)
	return &d, nil
}

func (s *PostgresStore) GetAuthorizationCode(ctx context.Context, code string) (*domain.OAuthAuthorizationCode, error) {
	ac, err := s.q.GetAuthorizationCode(ctx, code)
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainOAuthAuthorizationCode(ac)
	return &d, nil
}

func (s *PostgresStore) DeleteAuthorizationCode(ctx context.Context, code string) error {
	return mapErr(s.q.DeleteAuthorizationCode(ctx, code))
}

func (s *PostgresStore) CreateAccessToken(ctx context.Context, in store.CreateAccessTokenInput) (*domain.OAuthAccessToken, error) {
	tok, err := s.q.CreateAccessToken(ctx, toDbCreateAccessTokenParams(in))
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainOAuthAccessToken(tok)
	return &d, nil
}

func (s *PostgresStore) GetAccessToken(ctx context.Context, token string) (*domain.OAuthAccessToken, error) {
	tok, err := s.q.GetAccessToken(ctx, token)
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainOAuthAccessToken(tok)
	return &d, nil
}

func (s *PostgresStore) RevokeAccessToken(ctx context.Context, token string) error {
	return mapErr(s.q.RevokeAccessToken(ctx, token))
}

func (s *PostgresStore) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	u, err := s.q.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainUser(u)
	return &d, nil
}

func (s *PostgresStore) GetUserByAccountID(ctx context.Context, accountID string) (*domain.User, error) {
	u, err := s.q.GetUserByAccountID(ctx, accountID)
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainUser(u)
	return &d, nil
}

func (s *PostgresStore) CreateStatusMention(ctx context.Context, statusID, accountID string) error {
	return mapErr(s.q.CreateStatusMention(ctx, db.CreateStatusMentionParams{
		StatusID:  statusID,
		AccountID: accountID,
	}))
}

func (s *PostgresStore) GetStatusMentions(ctx context.Context, statusID string) ([]*domain.Account, error) {
	rows, err := s.q.GetStatusMentions(ctx, statusID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]*domain.Account, 0, len(rows))
	for i := range rows {
		acc := ToDomainAccount(rows[i])
		out = append(out, &acc)
	}
	return out, nil
}

func (s *PostgresStore) GetOrCreateHashtag(ctx context.Context, name string) (*domain.Hashtag, error) {
	h, err := s.q.GetOrCreateHashtag(ctx, db.GetOrCreateHashtagParams{
		ID:    uid.New(),
		Lower: name,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainHashtag(h)
	return &d, nil
}

func (s *PostgresStore) AttachHashtagsToStatus(ctx context.Context, statusID string, hashtagIDs []string) error {
	return mapErr(s.q.AttachHashtagsToStatus(ctx, db.AttachHashtagsToStatusParams{
		StatusID: statusID,
		Column2:  hashtagIDs,
	}))
}

func (s *PostgresStore) GetStatusHashtags(ctx context.Context, statusID string) ([]domain.Hashtag, error) {
	rows, err := s.q.GetStatusHashtags(ctx, statusID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.Hashtag, 0, len(rows))
	for _, r := range rows {
		out = append(out, ToDomainHashtag(r))
	}
	return out, nil
}

func (s *PostgresStore) CreateNotification(ctx context.Context, in store.CreateNotificationInput) (*domain.Notification, error) {
	n, err := s.q.CreateNotification(ctx, db.CreateNotificationParams{
		ID:        in.ID,
		AccountID: in.AccountID,
		FromID:    in.FromID,
		Type:      in.Type,
		StatusID:  in.StatusID,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainNotification(n)
	return &d, nil
}

func (s *PostgresStore) GetStatusAttachments(ctx context.Context, statusID string) ([]domain.MediaAttachment, error) {
	rows, err := s.q.ListStatusAttachments(ctx, &statusID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.MediaAttachment, 0, len(rows))
	for _, r := range rows {
		out = append(out, ToDomainMediaAttachment(r))
	}
	return out, nil
}
