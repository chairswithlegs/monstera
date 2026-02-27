package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

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

func (s *PostgresStore) GetAccountByAPID(ctx context.Context, apID string) (*domain.Account, error) {
	dbAcc, err := s.q.GetAccountByAPID(ctx, apID)
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

func (s *PostgresStore) SearchAccounts(ctx context.Context, query string, limit int) ([]*domain.Account, error) {
	rows, err := s.q.SearchAccounts(ctx, db.SearchAccountsParams{
		Lower: query,
		Limit: int32(limit), //nolint:gosec // limit clamped by caller
	})
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

func (s *PostgresStore) CountLocalAccounts(ctx context.Context) (int64, error) {
	n, err := s.q.CountLocalAccounts(ctx)
	return n, mapErr(err)
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

func (s *PostgresStore) GetStatusByAPID(ctx context.Context, apID string) (*domain.Status, error) {
	dbSt, err := s.q.GetStatusByAPID(ctx, apID)
	if err != nil {
		return nil, mapErr(err)
	}
	st := ToDomainStatus(dbSt)
	return &st, nil
}

func (s *PostgresStore) GetAccountStatuses(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, error) {
	cursor := noCursorSentinel
	if maxID != nil && *maxID != "" {
		cursor = *maxID
	}
	dbRows, err := s.q.GetAccountStatuses(ctx, db.GetAccountStatusesParams{
		AccountID: accountID,
		Column2:   cursor,
		Limit:     int32(limit), //nolint:gosec // limit clamped by caller
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

func (s *PostgresStore) GetAccountPublicStatuses(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, error) {
	cursor := noCursorSentinel
	if maxID != nil && *maxID != "" {
		cursor = *maxID
	}
	dbRows, err := s.q.GetAccountPublicStatuses(ctx, db.GetAccountPublicStatusesParams{
		AccountID: accountID,
		Column2:   cursor,
		Limit:     int32(limit), //nolint:gosec // limit clamped by caller
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

func (s *PostgresStore) CountLocalStatuses(ctx context.Context) (int64, error) {
	n, err := s.q.CountLocalStatuses(ctx)
	return n, mapErr(err)
}

func (s *PostgresStore) CountAccountPublicStatuses(ctx context.Context, accountID string) (int64, error) {
	n, err := s.q.CountAccountPublicStatuses(ctx, accountID)
	return n, mapErr(err)
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

func (s *PostgresStore) ConfirmUser(ctx context.Context, userID string) error {
	return mapErr(s.q.ConfirmUser(ctx, userID))
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

func (s *PostgresStore) SearchHashtagsByPrefix(ctx context.Context, prefix string, limit int) ([]domain.Hashtag, error) {
	rows, err := s.q.SearchHashtagsByPrefix(ctx, db.SearchHashtagsByPrefixParams{
		Lower: prefix,
		Limit: int32(limit), //nolint:gosec // limit clamped by caller
	})
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

func (s *PostgresStore) ListNotifications(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Notification, error) {
	cursor := noCursorSentinel
	if maxID != nil && *maxID != "" {
		cursor = *maxID
	}
	rows, err := s.q.ListNotifications(ctx, db.ListNotificationsParams{
		AccountID: accountID,
		Column2:   cursor,
		Limit:     int32(limit), //nolint:gosec // limit clamped by caller
	})
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.Notification, 0, len(rows))
	for _, r := range rows {
		out = append(out, ToDomainNotification(r))
	}
	return out, nil
}

func (s *PostgresStore) GetNotification(ctx context.Context, id, accountID string) (*domain.Notification, error) {
	n, err := s.q.GetNotification(ctx, db.GetNotificationParams{ID: id, AccountID: accountID})
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainNotification(n)
	return &d, nil
}

func (s *PostgresStore) ClearNotifications(ctx context.Context, accountID string) error {
	return mapErr(s.q.ClearNotifications(ctx, accountID))
}

func (s *PostgresStore) DismissNotification(ctx context.Context, id, accountID string) error {
	return mapErr(s.q.DismissNotification(ctx, db.DismissNotificationParams{ID: id, AccountID: accountID}))
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

func (s *PostgresStore) GetSetting(ctx context.Context, key string) (string, error) {
	v, err := s.q.GetSetting(ctx, key)
	return v, mapErr(err)
}

func (s *PostgresStore) GetMediaAttachment(ctx context.Context, id string) (*domain.MediaAttachment, error) {
	m, err := s.q.GetMediaAttachment(ctx, id)
	if err != nil {
		return nil, mapErr(err)
	}
	att := ToDomainMediaAttachment(m)
	return &att, nil
}

func (s *PostgresStore) CountFollowers(ctx context.Context, accountID string) (int64, error) {
	n, err := s.q.CountFollowers(ctx, accountID)
	return n, mapErr(err)
}

func (s *PostgresStore) CountFollowing(ctx context.Context, accountID string) (int64, error) {
	n, err := s.q.CountFollowing(ctx, accountID)
	return n, mapErr(err)
}

func (s *PostgresStore) IncrementFollowersCount(ctx context.Context, accountID string) error {
	return mapErr(s.q.IncrementFollowersCount(ctx, accountID))
}

func (s *PostgresStore) DecrementFollowersCount(ctx context.Context, accountID string) error {
	return mapErr(s.q.DecrementFollowersCount(ctx, accountID))
}

func (s *PostgresStore) IncrementFollowingCount(ctx context.Context, accountID string) error {
	return mapErr(s.q.IncrementFollowingCount(ctx, accountID))
}

func (s *PostgresStore) DecrementFollowingCount(ctx context.Context, accountID string) error {
	return mapErr(s.q.DecrementFollowingCount(ctx, accountID))
}

func (s *PostgresStore) GetRelationship(ctx context.Context, accountID, targetID string) (*domain.Relationship, error) {
	rel := &domain.Relationship{
		TargetID:       targetID,
		ShowingReblogs: true,
		Notifying:      false,
		Endorsed:       false,
		Note:           "",
	}
	fw, err := s.q.GetFollow(ctx, db.GetFollowParams{AccountID: accountID, TargetID: targetID})
	if err == nil {
		switch fw.State {
		case domain.FollowStateAccepted:
			rel.Following = true
		case domain.FollowStatePending:
			rel.Requested = true
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("GetFollow(actor->target): %w", mapErr(err))
	}
	bw, err := s.q.GetFollow(ctx, db.GetFollowParams{AccountID: targetID, TargetID: accountID})
	if err == nil && bw.State == domain.FollowStateAccepted {
		rel.FollowedBy = true
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("GetFollow(target->actor): %w", mapErr(err))
	}
	_, err = s.q.GetBlock(ctx, db.GetBlockParams{AccountID: accountID, TargetID: targetID})
	if err == nil {
		rel.Blocking = true
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("GetBlock(actor->target): %w", mapErr(err))
	}
	_, err = s.q.GetBlock(ctx, db.GetBlockParams{AccountID: targetID, TargetID: accountID})
	if err == nil {
		rel.BlockedBy = true
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("GetBlock(target->actor): %w", mapErr(err))
	}
	m, err := s.q.GetMute(ctx, db.GetMuteParams{AccountID: accountID, TargetID: targetID})
	if err == nil {
		rel.Muting = true
		rel.MutingNotifications = m.HideNotifications
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("GetMute: %w", mapErr(err))
	}
	return rel, nil
}

func (s *PostgresStore) ListDomainBlocks(ctx context.Context) ([]domain.DomainBlock, error) {
	rows, err := s.q.ListDomainBlocks(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.DomainBlock, 0, len(rows))
	for _, r := range rows {
		out = append(out, ToDomainDomainBlock(r))
	}
	return out, nil
}

func (s *PostgresStore) GetFollow(ctx context.Context, accountID, targetID string) (*domain.Follow, error) {
	f, err := s.q.GetFollow(ctx, db.GetFollowParams{AccountID: accountID, TargetID: targetID})
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainFollow(f)
	return &d, nil
}

func (s *PostgresStore) GetFollowByAPID(ctx context.Context, apID string) (*domain.Follow, error) {
	f, err := s.q.GetFollowByAPID(ctx, &apID)
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainFollow(f)
	return &d, nil
}

func toDbCreateFollowParams(in store.CreateFollowInput) db.CreateFollowParams {
	return db.CreateFollowParams{
		ID:        in.ID,
		AccountID: in.AccountID,
		TargetID:  in.TargetID,
		State:     in.State,
		ApID:      in.APID,
	}
}

func (s *PostgresStore) CreateFollow(ctx context.Context, in store.CreateFollowInput) (*domain.Follow, error) {
	f, err := s.q.CreateFollow(ctx, toDbCreateFollowParams(in))
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainFollow(f)
	return &d, nil
}

func (s *PostgresStore) AcceptFollow(ctx context.Context, followID string) error {
	return mapErr(s.q.AcceptFollow(ctx, followID))
}

func (s *PostgresStore) DeleteFollow(ctx context.Context, accountID, targetID string) error {
	return mapErr(s.q.DeleteFollow(ctx, db.DeleteFollowParams{AccountID: accountID, TargetID: targetID}))
}

func (s *PostgresStore) GetFollowers(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, error) {
	cursor := noCursorSentinel
	if maxID != nil && *maxID != "" {
		cursor = *maxID
	}
	rows, err := s.q.GetFollowers(ctx, db.GetFollowersParams{
		TargetID: accountID,
		Column2:  cursor,
		Limit:    int32(limit), //nolint:gosec // limit clamped by caller
	})
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.Account, 0, len(rows))
	for _, r := range rows {
		out = append(out, ToDomainAccount(r))
	}
	return out, nil
}

func (s *PostgresStore) GetFollowing(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, error) {
	cursor := noCursorSentinel
	if maxID != nil && *maxID != "" {
		cursor = *maxID
	}
	rows, err := s.q.GetFollowing(ctx, db.GetFollowingParams{
		AccountID: accountID,
		Column2:   cursor,
		Limit:     int32(limit), //nolint:gosec // limit clamped by caller
	})
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.Account, 0, len(rows))
	for _, r := range rows {
		out = append(out, ToDomainAccount(r))
	}
	return out, nil
}

func (s *PostgresStore) SoftDeleteStatus(ctx context.Context, id string) error {
	return mapErr(s.q.SoftDeleteStatus(ctx, id))
}

func (s *PostgresStore) SuspendAccount(ctx context.Context, id string) error {
	return mapErr(s.q.SuspendAccount(ctx, id))
}

func (s *PostgresStore) CreateBlock(ctx context.Context, in store.CreateBlockInput) error {
	_, err := s.q.CreateBlock(ctx, db.CreateBlockParams{
		ID:        in.ID,
		AccountID: in.AccountID,
		TargetID:  in.TargetID,
	})
	return mapErr(err)
}

func (s *PostgresStore) DeleteBlock(ctx context.Context, accountID, targetID string) error {
	return mapErr(s.q.DeleteBlock(ctx, db.DeleteBlockParams{AccountID: accountID, TargetID: targetID}))
}

func toDbCreateFavouriteParams(in store.CreateFavouriteInput) db.CreateFavouriteParams {
	return db.CreateFavouriteParams{
		ID:        in.ID,
		AccountID: in.AccountID,
		StatusID:  in.StatusID,
		ApID:      in.APID,
	}
}

func (s *PostgresStore) CreateFavourite(ctx context.Context, in store.CreateFavouriteInput) (*domain.Favourite, error) {
	f, err := s.q.CreateFavourite(ctx, toDbCreateFavouriteParams(in))
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainFavourite(f)
	return &d, nil
}

func (s *PostgresStore) DeleteFavourite(ctx context.Context, accountID, statusID string) error {
	return mapErr(s.q.DeleteFavourite(ctx, db.DeleteFavouriteParams{AccountID: accountID, StatusID: statusID}))
}

func (s *PostgresStore) GetFavouriteByAPID(ctx context.Context, apID string) (*domain.Favourite, error) {
	f, err := s.q.GetFavouriteByAPID(ctx, &apID)
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainFavourite(f)
	return &d, nil
}

func (s *PostgresStore) GetFavouriteByAccountAndStatus(ctx context.Context, accountID, statusID string) (*domain.Favourite, error) {
	f, err := s.q.GetFavouriteByAccountAndStatus(ctx, db.GetFavouriteByAccountAndStatusParams{
		AccountID: accountID,
		StatusID:  statusID,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainFavourite(f)
	return &d, nil
}

func (s *PostgresStore) IncrementFavouritesCount(ctx context.Context, statusID string) error {
	return mapErr(s.q.IncrementFavouritesCount(ctx, statusID))
}

func (s *PostgresStore) DecrementFavouritesCount(ctx context.Context, statusID string) error {
	return mapErr(s.q.DecrementFavouritesCount(ctx, statusID))
}

func (s *PostgresStore) IncrementReblogsCount(ctx context.Context, statusID string) error {
	return mapErr(s.q.IncrementReblogsCount(ctx, statusID))
}

func (s *PostgresStore) DecrementReblogsCount(ctx context.Context, statusID string) error {
	return mapErr(s.q.DecrementReblogsCount(ctx, statusID))
}

func (s *PostgresStore) IncrementRepliesCount(ctx context.Context, statusID string) error {
	return mapErr(s.q.IncrementRepliesCount(ctx, statusID))
}

func (s *PostgresStore) GetReblogByAccountAndTarget(ctx context.Context, accountID, statusID string) (*domain.Status, error) {
	st, err := s.q.GetReblogByAccountAndTarget(ctx, db.GetReblogByAccountAndTargetParams{
		AccountID:  accountID,
		ReblogOfID: &statusID,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainStatus(st)
	return &d, nil
}

func toDbUpdateAccountParams(in store.UpdateAccountInput) db.UpdateAccountParams {
	return db.UpdateAccountParams{
		ID:            in.ID,
		DisplayName:   in.DisplayName,
		Note:          in.Note,
		AvatarMediaID: in.AvatarMediaID,
		HeaderMediaID: in.HeaderMediaID,
		ApRaw:         in.APRaw,
		Bot:           in.Bot,
		Locked:        in.Locked,
	}
}

func (s *PostgresStore) UpdateAccount(ctx context.Context, in store.UpdateAccountInput) error {
	_, err := s.q.UpdateAccount(ctx, toDbUpdateAccountParams(in))
	return mapErr(err)
}

func (s *PostgresStore) UpdateAccountKeys(ctx context.Context, id, publicKey string, apRaw []byte) error {
	return mapErr(s.q.UpdateAccountKeys(ctx, db.UpdateAccountKeysParams{
		ID:        id,
		PublicKey: publicKey,
		ApRaw:     apRaw,
	}))
}

func (s *PostgresStore) AttachMediaToStatus(ctx context.Context, mediaID, statusID, accountID string) error {
	return mapErr(s.q.AttachMediaToStatus(ctx, db.AttachMediaToStatusParams{
		ID:        mediaID,
		StatusID:  &statusID,
		AccountID: accountID,
	}))
}

func toDbCreateMediaAttachmentParams(in store.CreateMediaAttachmentInput) db.CreateMediaAttachmentParams {
	return db.CreateMediaAttachmentParams{
		ID:          in.ID,
		AccountID:   in.AccountID,
		Type:        in.Type,
		StorageKey:  in.StorageKey,
		Url:         in.URL,
		PreviewUrl:  in.PreviewURL,
		RemoteUrl:   in.RemoteURL,
		Description: in.Description,
		Blurhash:    in.Blurhash,
		Meta:        in.Meta,
	}
}

func (s *PostgresStore) CreateMediaAttachment(ctx context.Context, in store.CreateMediaAttachmentInput) (*domain.MediaAttachment, error) {
	m, err := s.q.CreateMediaAttachment(ctx, toDbCreateMediaAttachmentParams(in))
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainMediaAttachment(m)
	return &d, nil
}

func toDbCreateStatusEditParams(in store.CreateStatusEditInput) db.CreateStatusEditParams {
	return db.CreateStatusEditParams{
		ID:             in.ID,
		StatusID:       in.StatusID,
		AccountID:      in.AccountID,
		Text:           in.Text,
		Content:        in.Content,
		ContentWarning: in.ContentWarning,
		Sensitive:      in.Sensitive,
	}
}

func (s *PostgresStore) CreateStatusEdit(ctx context.Context, in store.CreateStatusEditInput) error {
	_, err := s.q.CreateStatusEdit(ctx, toDbCreateStatusEditParams(in))
	return mapErr(err)
}

func toDbUpdateStatusParams(in store.UpdateStatusInput) db.UpdateStatusParams {
	return db.UpdateStatusParams{
		ID:             in.ID,
		Text:           in.Text,
		Content:        in.Content,
		ContentWarning: in.ContentWarning,
		Sensitive:      in.Sensitive,
	}
}

func (s *PostgresStore) UpdateStatus(ctx context.Context, in store.UpdateStatusInput) error {
	_, err := s.q.UpdateStatus(ctx, toDbUpdateStatusParams(in))
	return mapErr(err)
}

func (s *PostgresStore) GetFollowerInboxURLs(ctx context.Context, accountID string) ([]string, error) {
	urls, err := s.q.GetFollowerInboxURLs(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("GetFollowerInboxURLs: %w", mapErr(err))
	}
	return urls, nil
}
