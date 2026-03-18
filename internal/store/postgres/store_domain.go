package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	db "github.com/chairswithlegs/monstera/internal/store/postgres/generated"
	"github.com/chairswithlegs/monstera/internal/uid"
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
		Bot:          in.Bot,
		Locked:       in.Locked,
		Url:          in.URL,
	}
}

func toDbCreateUserParams(in store.CreateUserInput) db.CreateUserParams {
	return db.CreateUserParams{
		ID:                 in.ID,
		AccountID:          in.AccountID,
		Email:              in.Email,
		PasswordHash:       in.PasswordHash,
		Role:               in.Role,
		RegistrationReason: in.RegistrationReason,
	}
}

func toDbCreateStatusParams(in store.CreateStatusInput) db.CreateStatusParams {
	return db.CreateStatusParams{
		ID:                  in.ID,
		Uri:                 in.URI,
		AccountID:           in.AccountID,
		Text:                in.Text,
		Content:             in.Content,
		ContentWarning:      in.ContentWarning,
		Visibility:          in.Visibility,
		Language:            in.Language,
		InReplyToID:         in.InReplyToID,
		InReplyToAccountID:  in.InReplyToAccountID,
		ReblogOfID:          in.ReblogOfID,
		QuotedStatusID:      in.QuotedStatusID,
		QuoteApprovalPolicy: in.QuoteApprovalPolicy,
		ApID:                in.APID,
		Sensitive:           in.Sensitive,
		Local:               in.Local,
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
	row, err := s.q.GetAccountByID(ctx, id)
	if err != nil {
		return nil, mapErr(err)
	}
	acc := rowWithURLsToDomainAccount(row.Account, row.AvatarUrl, row.HeaderUrl)
	return &acc, nil
}

func (s *PostgresStore) GetAccountsByIDs(ctx context.Context, ids []string) ([]*domain.Account, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.q.GetAccountsByIDs(ctx, ids)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]*domain.Account, 0, len(rows))
	for _, row := range rows {
		acc := rowWithURLsToDomainAccount(row.Account, row.AvatarUrl, row.HeaderUrl)
		out = append(out, &acc)
	}
	return out, nil
}

func (s *PostgresStore) GetLocalAccountByUsername(ctx context.Context, username string) (*domain.Account, error) {
	row, err := s.q.GetLocalAccountByUsername(ctx, username)
	if err != nil {
		return nil, mapErr(err)
	}
	acc := rowWithURLsToDomainAccount(row.Account, row.AvatarUrl, row.HeaderUrl)
	return &acc, nil
}

func (s *PostgresStore) GetAccountByAPID(ctx context.Context, apID string) (*domain.Account, error) {
	row, err := s.q.GetAccountByAPID(ctx, apID)
	if err != nil {
		return nil, mapErr(err)
	}
	acc := rowWithURLsToDomainAccount(row.Account, row.AvatarUrl, row.HeaderUrl)
	return &acc, nil
}

func (s *PostgresStore) GetRemoteAccountByUsername(ctx context.Context, username string, domain *string) (*domain.Account, error) {
	row, err := s.q.GetRemoteAccountByUsername(ctx, db.GetRemoteAccountByUsernameParams{
		Username: username,
		Domain:   domain,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	acc := rowWithURLsToDomainAccount(row.Account, row.AvatarUrl, row.HeaderUrl)
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
	for _, row := range rows {
		acc := rowWithURLsToDomainAccount(row.Account, row.AvatarUrl, row.HeaderUrl)
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

func (s *PostgresStore) IncrementStatusesCount(ctx context.Context, accountID string) error {
	return mapErr(s.q.IncrementStatusesCount(ctx, accountID))
}

func (s *PostgresStore) UpdateAccountLastStatusAt(ctx context.Context, accountID string) error {
	return mapErr(s.q.UpdateAccountLastStatusAt(ctx, accountID))
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

func (s *PostgresStore) GetFavouritesTimeline(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, *string, error) {
	cursor := noCursorSentinel
	if maxID != nil && *maxID != "" {
		cursor = *maxID
	}
	dbRows, err := s.q.GetFavouritesTimeline(ctx, db.GetFavouritesTimelineParams{
		AccountID: accountID,
		Column2:   cursor,
		Limit:     int32(limit), //nolint:gosec // clamped by TimelineService before reaching this layer
	})
	if err != nil {
		return nil, nil, mapErr(err)
	}
	out := make([]domain.Status, 0, len(dbRows))
	var nextCursor *string
	for i, r := range dbRows {
		out = append(out, FavouritesTimelineRowToDomain(r))
		if i == len(dbRows)-1 && len(dbRows) == limit {
			nextCursor = &r.Cursor
		}
	}
	return out, nextCursor, nil
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

func (s *PostgresStore) GetHashtagTimeline(ctx context.Context, tagName string, maxID *string, limit int) ([]domain.Status, error) {
	cursor := noCursorSentinel
	if maxID != nil && *maxID != "" {
		cursor = *maxID
	}
	dbRows, err := s.q.GetHashtagTimeline(ctx, db.GetHashtagTimelineParams{
		Lower:   strings.ToLower(tagName),
		Column2: cursor,
		Limit:   int32(limit), //nolint:gosec // limit clamped by caller
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

func (s *PostgresStore) GetStatusAncestors(ctx context.Context, statusID string) ([]domain.Status, error) {
	rows, err := s.q.GetStatusAncestors(ctx, statusID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.Status, 0, len(rows))
	for _, r := range rows {
		out = append(out, AncestorRowToDomain(r))
	}
	return out, nil
}

func (s *PostgresStore) GetStatusDescendants(ctx context.Context, statusID string) ([]domain.Status, error) {
	rows, err := s.q.GetStatusDescendants(ctx, &statusID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.Status, 0, len(rows))
	for _, r := range rows {
		out = append(out, DescendantRowToDomain(r))
	}
	return out, nil
}

func (s *PostgresStore) GetStatusFavouritedBy(ctx context.Context, statusID string, maxID *string, limit int) ([]domain.Account, error) {
	cursor := noCursorSentinel
	if maxID != nil && *maxID != "" {
		cursor = *maxID
	}
	rows, err := s.q.GetStatusFavouritedBy(ctx, db.GetStatusFavouritedByParams{
		StatusID: statusID,
		Column2:  cursor,
		Limit:    int32(limit), //nolint:gosec // limit clamped by caller
	})
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.Account, 0, len(rows))
	for _, row := range rows {
		out = append(out, rowWithURLsToDomainAccount(row.Account, row.AvatarUrl, row.HeaderUrl))
	}
	return out, nil
}

func (s *PostgresStore) GetRebloggedBy(ctx context.Context, statusID string, maxID *string, limit int) ([]domain.Account, error) {
	cursor := noCursorSentinel
	if maxID != nil && *maxID != "" {
		cursor = *maxID
	}
	rows, err := s.q.GetRebloggedBy(ctx, db.GetRebloggedByParams{
		ReblogOfID: &statusID,
		Column2:    cursor,
		Limit:      int32(limit), //nolint:gosec // limit clamped by caller
	})
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.Account, 0, len(rows))
	for _, r := range rows {
		out = append(out, RebloggedByRowToDomainAccount(r))
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

func (s *PostgresStore) DeleteStatusMentions(ctx context.Context, statusID string) error {
	return mapErr(s.q.DeleteStatusMentions(ctx, statusID))
}

func (s *PostgresStore) GetStatusMentions(ctx context.Context, statusID string) ([]*domain.Account, error) {
	rows, err := s.q.GetStatusMentions(ctx, statusID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]*domain.Account, 0, len(rows))
	for i := range rows {
		acc := rowWithURLsToDomainAccount(rows[i].Account, rows[i].AvatarUrl, rows[i].HeaderUrl)
		out = append(out, &acc)
	}
	return out, nil
}

func (s *PostgresStore) GetStatusMentionAccountIDs(ctx context.Context, statusID string) ([]string, error) {
	ids, err := s.q.GetStatusMentionAccountIDs(ctx, statusID)
	if err != nil {
		return nil, fmt.Errorf("GetStatusMentionAccountIDs: %w", mapErr(err))
	}
	return ids, nil
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

func (s *PostgresStore) DeleteStatusHashtags(ctx context.Context, statusID string) error {
	return mapErr(s.q.DeleteStatusHashtags(ctx, statusID))
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

func (s *PostgresStore) CreatePushSubscription(ctx context.Context, in store.CreatePushSubscriptionInput) (*domain.PushSubscription, error) {
	alertsJSON, err := json.Marshal(in.Alerts)
	if err != nil {
		return nil, fmt.Errorf("CreatePushSubscription: marshal alerts: %w", err)
	}
	row, err := s.q.CreatePushSubscription(ctx, db.CreatePushSubscriptionParams{
		ID:            in.ID,
		AccessTokenID: in.AccessTokenID,
		AccountID:     in.AccountID,
		Endpoint:      in.Endpoint,
		KeyP256dh:     in.KeyP256DH,
		KeyAuth:       in.KeyAuth,
		Alerts:        alertsJSON,
		Policy:        in.Policy,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	d := toDomainPushSubscription(row)
	return &d, nil
}

func (s *PostgresStore) GetPushSubscription(ctx context.Context, accessTokenID string) (*domain.PushSubscription, error) {
	row, err := s.q.GetPushSubscriptionByAccessToken(ctx, accessTokenID)
	if err != nil {
		return nil, mapErr(err)
	}
	d := toDomainPushSubscription(row)
	return &d, nil
}

func (s *PostgresStore) UpdatePushSubscription(ctx context.Context, accessTokenID string, alerts domain.PushAlerts, policy string) (*domain.PushSubscription, error) {
	alertsJSON, err := json.Marshal(alerts)
	if err != nil {
		return nil, fmt.Errorf("UpdatePushSubscription: marshal alerts: %w", err)
	}
	row, err := s.q.UpdatePushSubscriptionAlerts(ctx, db.UpdatePushSubscriptionAlertsParams{
		AccessTokenID: accessTokenID,
		Alerts:        alertsJSON,
		Policy:        policy,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	d := toDomainPushSubscription(row)
	return &d, nil
}

func (s *PostgresStore) DeletePushSubscription(ctx context.Context, accessTokenID string) error {
	return mapErr(s.q.DeletePushSubscription(ctx, accessTokenID))
}

func (s *PostgresStore) ListPushSubscriptionsByAccountID(ctx context.Context, accountID string) ([]domain.PushSubscription, error) {
	rows, err := s.q.ListPushSubscriptionsByAccountID(ctx, accountID)
	if err != nil {
		return nil, mapErr(err)
	}
	result := make([]domain.PushSubscription, len(rows))
	for i, row := range rows {
		result[i] = toDomainPushSubscription(row)
	}
	return result, nil
}

func (s *PostgresStore) GetHashtagByName(ctx context.Context, name string) (*domain.Hashtag, error) {
	h, err := s.q.GetHashtagByName(ctx, name)
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainHashtag(h)
	return &d, nil
}

func (s *PostgresStore) IsFollowingTag(ctx context.Context, accountID, tagID string) (bool, error) {
	following, err := s.q.IsFollowingTag(ctx, db.IsFollowingTagParams{
		AccountID: accountID,
		TagID:     tagID,
	})
	if err != nil {
		return false, mapErr(err)
	}
	return following, nil
}

func (s *PostgresStore) FollowTag(ctx context.Context, id, accountID, tagID string) error {
	return mapErr(s.q.FollowTag(ctx, db.FollowTagParams{
		ID:        id,
		AccountID: accountID,
		TagID:     tagID,
	}))
}

func (s *PostgresStore) UnfollowTag(ctx context.Context, accountID, tagID string) error {
	return mapErr(s.q.UnfollowTag(ctx, db.UnfollowTagParams{
		AccountID: accountID,
		TagID:     tagID,
	}))
}

func (s *PostgresStore) ListFollowedTags(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Hashtag, *string, error) {
	cursor := noCursorSentinel
	if maxID != nil && *maxID != "" {
		cursor = *maxID
	}
	rows, err := s.q.ListFollowedTagsPaginated(ctx, db.ListFollowedTagsPaginatedParams{
		AccountID: accountID,
		Column2:   cursor,
		Limit:     int32(limit), //nolint:gosec // limit clamped by caller
	})
	if err != nil {
		return nil, nil, mapErr(err)
	}
	out := make([]domain.Hashtag, 0, len(rows))
	var nextCursor *string
	for i, r := range rows {
		out = append(out, domain.Hashtag{
			ID:        r.ID,
			Name:      r.Name,
			CreatedAt: pgTime(r.CreatedAt),
			UpdatedAt: pgTime(r.UpdatedAt),
		})
		if i == len(rows)-1 && len(rows) == limit {
			nextCursor = &r.Cursor
		}
	}
	return out, nextCursor, nil
}

func featuredTagRowToDomain(r db.ListFeaturedTagsByAccountRow) domain.FeaturedTag {
	var lastAt *time.Time
	if r.LastStatusAt != nil {
		switch v := r.LastStatusAt.(type) {
		case time.Time:
			lastAt = &v
		case *time.Time:
			lastAt = v
		case pgtype.Timestamptz:
			if v.Valid {
				t := v.Time
				lastAt = &t
			}
		}
	}
	return domain.FeaturedTag{
		ID:            r.ID,
		AccountID:     r.AccountID,
		TagID:         r.TagID,
		Name:          r.Name,
		StatusesCount: int(r.StatusesCount),
		LastStatusAt:  lastAt,
		CreatedAt:     pgTime(r.CreatedAt),
	}
}

func (s *PostgresStore) CreateFeaturedTag(ctx context.Context, id, accountID, tagID string) error {
	return mapErr(s.q.CreateFeaturedTag(ctx, db.CreateFeaturedTagParams{
		ID:        id,
		AccountID: accountID,
		TagID:     tagID,
	}))
}

func (s *PostgresStore) DeleteFeaturedTag(ctx context.Context, id, accountID string) error {
	return mapErr(s.q.DeleteFeaturedTag(ctx, db.DeleteFeaturedTagParams{
		ID:        id,
		AccountID: accountID,
	}))
}

func (s *PostgresStore) ListFeaturedTags(ctx context.Context, accountID string) ([]domain.FeaturedTag, error) {
	rows, err := s.q.ListFeaturedTagsByAccount(ctx, accountID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.FeaturedTag, 0, len(rows))
	for _, r := range rows {
		out = append(out, featuredTagRowToDomain(r))
	}
	return out, nil
}

func (s *PostgresStore) GetFeaturedTagByID(ctx context.Context, id, accountID string) (*domain.FeaturedTag, error) {
	row, err := s.q.GetFeaturedTagByID(ctx, db.GetFeaturedTagByIDParams{
		ID:        id,
		AccountID: accountID,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return &domain.FeaturedTag{
		ID:            row.ID,
		AccountID:     row.AccountID,
		TagID:         row.TagID,
		Name:          row.Name,
		StatusesCount: 0,
		LastStatusAt:  nil,
		CreatedAt:     pgTime(row.CreatedAt),
	}, nil
}

func (s *PostgresStore) ListFeaturedTagSuggestions(ctx context.Context, accountID string, limit int) ([]domain.Hashtag, []int64, error) {
	rows, err := s.q.ListAccountTagSuggestions(ctx, db.ListAccountTagSuggestionsParams{
		AccountID: accountID,
		Limit:     int32(limit), //nolint:gosec // limit clamped by caller
	})
	if err != nil {
		return nil, nil, mapErr(err)
	}
	tags := make([]domain.Hashtag, 0, len(rows))
	counts := make([]int64, 0, len(rows))
	for _, r := range rows {
		tags = append(tags, domain.Hashtag{ID: r.ID, Name: r.Name})
		counts = append(counts, r.UseCount)
	}
	return tags, counts, nil
}

func (s *PostgresStore) GetConversationRoot(ctx context.Context, statusID string) (string, error) {
	root, err := s.q.GetConversationRoot(ctx, statusID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return statusID, nil
		}
		return "", mapErr(err)
	}
	return root, nil
}

func (s *PostgresStore) CreateConversationMute(ctx context.Context, accountID, conversationID string) error {
	return mapErr(s.q.CreateConversationMute(ctx, db.CreateConversationMuteParams{
		AccountID:      accountID,
		ConversationID: conversationID,
	}))
}

func (s *PostgresStore) DeleteConversationMute(ctx context.Context, accountID, conversationID string) error {
	return mapErr(s.q.DeleteConversationMute(ctx, db.DeleteConversationMuteParams{
		AccountID:      accountID,
		ConversationID: conversationID,
	}))
}

func (s *PostgresStore) IsConversationMuted(ctx context.Context, accountID, conversationID string) (bool, error) {
	ok, err := s.q.IsConversationMuted(ctx, db.IsConversationMutedParams{
		AccountID:      accountID,
		ConversationID: conversationID,
	})
	if err != nil {
		return false, mapErr(err)
	}
	return ok, nil
}

func (s *PostgresStore) CreateConversation(ctx context.Context, id string) error {
	return mapErr(s.q.CreateConversation(ctx, id))
}

func (s *PostgresStore) SetStatusConversationID(ctx context.Context, statusID, conversationID string) error {
	return mapErr(s.q.SetStatusConversationID(ctx, db.SetStatusConversationIDParams{
		ID:             statusID,
		ConversationID: &conversationID,
	}))
}

func (s *PostgresStore) GetStatusConversationID(ctx context.Context, statusID string) (*string, error) {
	cid, err := s.q.GetStatusConversationID(ctx, statusID)
	if err != nil {
		return nil, mapErr(err)
	}
	return cid, nil
}

func (s *PostgresStore) UpsertAccountConversation(ctx context.Context, in store.UpsertAccountConversationInput) error {
	return mapErr(s.q.UpsertAccountConversation(ctx, db.UpsertAccountConversationParams{
		ID:             in.ID,
		AccountID:      in.AccountID,
		ConversationID: in.ConversationID,
		LastStatusID:   &in.LastStatusID,
		Unread:         in.Unread,
	}))
}

func (s *PostgresStore) ListAccountConversations(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.AccountConversation, *string, error) {
	cursor := noCursorSentinel
	if maxID != nil && *maxID != "" {
		cursor = *maxID
	}
	rows, err := s.q.ListAccountConversationsPaginated(ctx, db.ListAccountConversationsPaginatedParams{
		AccountID: accountID,
		Column2:   cursor,
		Limit:     int32(limit), //nolint:gosec // limit clamped by caller
	})
	if err != nil {
		return nil, nil, mapErr(err)
	}
	out := make([]domain.AccountConversation, 0, len(rows))
	var nextCursor *string
	for i, r := range rows {
		out = append(out, ToDomainAccountConversation(r))
		if i == len(rows)-1 && len(rows) == limit {
			nextCursor = &r.ID
		}
	}
	return out, nextCursor, nil
}

func (s *PostgresStore) GetAccountConversation(ctx context.Context, accountID, conversationID string) (*domain.AccountConversation, error) {
	ac, err := s.q.GetAccountConversation(ctx, db.GetAccountConversationParams{
		AccountID:      accountID,
		ConversationID: conversationID,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainAccountConversation(ac)
	return &d, nil
}

func (s *PostgresStore) MarkAccountConversationRead(ctx context.Context, accountID, conversationID string) error {
	return mapErr(s.q.MarkAccountConversationRead(ctx, db.MarkAccountConversationReadParams{
		AccountID:      accountID,
		ConversationID: conversationID,
	}))
}

func (s *PostgresStore) DeleteAccountConversation(ctx context.Context, accountID, conversationID string) error {
	return mapErr(s.q.DeleteAccountConversation(ctx, db.DeleteAccountConversationParams{
		AccountID:      accountID,
		ConversationID: conversationID,
	}))
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

func (s *PostgresStore) GetMonsteraSettings(ctx context.Context) (*domain.MonsteraSettings, error) {
	row, err := s.q.GetMonsteraSettings(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	rules := []string{}
	if row.ServerRules != nil && *row.ServerRules != "" {
		rules = strings.Split(*row.ServerRules, "\n")
	}
	return &domain.MonsteraSettings{
		RegistrationMode:    domain.MonsteraRegistrationMode(row.RegistrationMode),
		InviteMaxUses:       int32PtrToIntPtr(row.InviteMaxUses),
		InviteExpiresInDays: int32PtrToIntPtr(row.InviteExpiresInDays),
		ServerName:          row.ServerName,
		ServerDescription:   row.ServerDescription,
		ServerRules:         rules,
	}, nil
}

func (s *PostgresStore) UpdateMonsteraSettings(ctx context.Context, in *domain.MonsteraSettings) error {
	var rulesText *string
	if len(in.ServerRules) > 0 {
		r := strings.Join(in.ServerRules, "\n")
		rulesText = &r
	}
	return mapErr(s.q.UpdateMonsteraSettings(ctx, db.UpdateMonsteraSettingsParams{
		RegistrationMode:    string(in.RegistrationMode),
		InviteMaxUses:       intPtrToInt32Ptr(in.InviteMaxUses),
		InviteExpiresInDays: intPtrToInt32Ptr(in.InviteExpiresInDays),
		ServerName:          in.ServerName,
		ServerDescription:   in.ServerDescription,
		ServerRules:         rulesText,
	}))
}

func int32PtrToIntPtr(v *int32) *int {
	if v == nil {
		return nil
	}
	i := int(*v)
	return &i
}

func intPtrToInt32Ptr(v *int) *int32 {
	if v == nil {
		return nil
	}
	i := int32(*v) //nolint:gosec // domain value bounded by caller
	return &i
}

func (s *PostgresStore) GetMediaAttachment(ctx context.Context, id string) (*domain.MediaAttachment, error) {
	m, err := s.q.GetMediaAttachment(ctx, id)
	if err != nil {
		return nil, mapErr(err)
	}
	att := ToDomainMediaAttachment(m)
	return &att, nil
}

func (s *PostgresStore) UpdateMediaAttachment(ctx context.Context, in store.UpdateMediaAttachmentInput) (*domain.MediaAttachment, error) {
	m, err := s.q.UpdateMediaAttachment(ctx, db.UpdateMediaAttachmentParams{
		ID:          in.ID,
		Description: in.Description,
		Meta:        in.Meta,
		AccountID:   in.AccountID,
	})
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

func (s *PostgresStore) GetBlock(ctx context.Context, accountID, targetID string) (*domain.Block, error) {
	b, err := s.q.GetBlock(ctx, db.GetBlockParams{AccountID: accountID, TargetID: targetID})
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainBlock(b)
	return &d, nil
}

func (s *PostgresStore) GetMute(ctx context.Context, accountID, targetID string) (*domain.Mute, error) {
	m, err := s.q.GetMute(ctx, db.GetMuteParams{AccountID: accountID, TargetID: targetID})
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainMute(m)
	return &d, nil
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

func (s *PostgresStore) GetFollowByID(ctx context.Context, id string) (*domain.Follow, error) {
	f, err := s.q.GetFollowByID(ctx, id)
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
	for _, row := range rows {
		out = append(out, rowWithURLsToDomainAccount(row.Account, row.AvatarUrl, row.HeaderUrl))
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
	for _, row := range rows {
		out = append(out, rowWithURLsToDomainAccount(row.Account, row.AvatarUrl, row.HeaderUrl))
	}
	return out, nil
}

func (s *PostgresStore) GetPendingFollowRequests(ctx context.Context, targetID string, maxID *string, limit int) ([]domain.Account, *string, error) {
	cursor := noCursorSentinel
	if maxID != nil && *maxID != "" {
		cursor = *maxID
	}
	rows, err := s.q.GetPendingFollowRequestsPaginated(ctx, db.GetPendingFollowRequestsPaginatedParams{
		TargetID: targetID,
		Column2:  cursor,
		Limit:    int32(limit), //nolint:gosec // limit clamped by caller
	})
	if err != nil {
		return nil, nil, mapErr(err)
	}
	out := make([]domain.Account, 0, len(rows))
	var nextCursor *string
	for i, r := range rows {
		out = append(out, PendingFollowRequestRowToDomainAccount(r))
		if i == len(rows)-1 && len(rows) == limit {
			nextCursor = &r.Cursor
		}
	}
	return out, nextCursor, nil
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

func (s *PostgresStore) ListBlockedAccounts(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, *string, error) {
	cursor := noCursorSentinel
	if maxID != nil && *maxID != "" {
		cursor = *maxID
	}
	rows, err := s.q.ListBlockedAccountsPaginated(ctx, db.ListBlockedAccountsPaginatedParams{
		AccountID: accountID,
		Column2:   cursor,
		Limit:     int32(limit), //nolint:gosec // limit clamped by caller
	})
	if err != nil {
		return nil, nil, mapErr(err)
	}
	out := make([]domain.Account, 0, len(rows))
	var nextCursor *string
	for i, r := range rows {
		out = append(out, BlockedAccountRowToDomainAccount(r))
		if i == len(rows)-1 && len(rows) == limit {
			nextCursor = &r.Cursor
		}
	}
	return out, nextCursor, nil
}

func (s *PostgresStore) IsBlockedEitherDirection(ctx context.Context, accountID, targetID string) (bool, error) {
	ok, err := s.q.IsBlockedEitherDirection(ctx, db.IsBlockedEitherDirectionParams{AccountID: accountID, TargetID: targetID})
	if err != nil {
		return false, mapErr(err)
	}
	return ok, nil
}

func (s *PostgresStore) CreateMute(ctx context.Context, in store.CreateMuteInput) error {
	_, err := s.q.CreateMute(ctx, db.CreateMuteParams{
		ID:                in.ID,
		AccountID:         in.AccountID,
		TargetID:          in.TargetID,
		HideNotifications: in.HideNotifications,
	})
	return mapErr(err)
}

func (s *PostgresStore) DeleteMute(ctx context.Context, accountID, targetID string) error {
	return mapErr(s.q.DeleteMute(ctx, db.DeleteMuteParams{AccountID: accountID, TargetID: targetID}))
}

func (s *PostgresStore) ListMutedAccounts(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, *string, error) {
	cursor := noCursorSentinel
	if maxID != nil && *maxID != "" {
		cursor = *maxID
	}
	rows, err := s.q.ListMutedAccountsPaginated(ctx, db.ListMutedAccountsPaginatedParams{
		AccountID: accountID,
		Column2:   cursor,
		Limit:     int32(limit), //nolint:gosec // limit clamped by caller
	})
	if err != nil {
		return nil, nil, mapErr(err)
	}
	out := make([]domain.Account, 0, len(rows))
	var nextCursor *string
	for i, r := range rows {
		out = append(out, MutedAccountRowToDomainAccount(r))
		if i == len(rows)-1 && len(rows) == limit {
			nextCursor = &r.Cursor
		}
	}
	return out, nextCursor, nil
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

func (s *PostgresStore) CreateBookmark(ctx context.Context, in store.CreateBookmarkInput) error {
	return mapErr(s.q.CreateBookmark(ctx, db.CreateBookmarkParams{
		ID:        in.ID,
		AccountID: in.AccountID,
		StatusID:  in.StatusID,
	}))
}

func (s *PostgresStore) DeleteBookmark(ctx context.Context, accountID, statusID string) error {
	return mapErr(s.q.DeleteBookmark(ctx, db.DeleteBookmarkParams{
		AccountID: accountID,
		StatusID:  statusID,
	}))
}

func (s *PostgresStore) GetBookmarks(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, *string, error) {
	cursor := noCursorSentinel
	if maxID != nil && *maxID != "" {
		cursor = *maxID
	}
	rows, err := s.q.GetBookmarksTimeline(ctx, db.GetBookmarksTimelineParams{
		AccountID: accountID,
		Column2:   cursor,
		Limit:     int32(limit), //nolint:gosec // clamped by caller
	})
	if err != nil {
		return nil, nil, mapErr(err)
	}
	out := make([]domain.Status, 0, len(rows))
	var nextCursor *string
	for i, r := range rows {
		out = append(out, BookmarksTimelineRowToDomain(r))
		if i == len(rows)-1 && len(rows) == limit {
			nextCursor = &r.Cursor
		}
	}
	return out, nextCursor, nil
}

func (s *PostgresStore) IsBookmarked(ctx context.Context, accountID, statusID string) (bool, error) {
	ok, err := s.q.IsBookmarked(ctx, db.IsBookmarkedParams{
		AccountID: accountID,
		StatusID:  statusID,
	})
	if err != nil {
		return false, mapErr(err)
	}
	return ok, nil
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

func (s *PostgresStore) IncrementQuotesCount(ctx context.Context, quotedStatusID string) error {
	return mapErr(s.q.IncrementQuotesCount(ctx, quotedStatusID))
}

func (s *PostgresStore) DecrementQuotesCount(ctx context.Context, quotedStatusID string) error {
	return mapErr(s.q.DecrementQuotesCount(ctx, quotedStatusID))
}

func (s *PostgresStore) CreateQuoteApproval(ctx context.Context, quotingStatusID, quotedStatusID string) error {
	return mapErr(s.q.CreateQuoteApproval(ctx, db.CreateQuoteApprovalParams{
		QuotingStatusID: quotingStatusID,
		QuotedStatusID:  quotedStatusID,
	}))
}

func (s *PostgresStore) RevokeQuote(ctx context.Context, quotedStatusID, quotingStatusID string) error {
	_, err := s.q.RevokeQuote(ctx, db.RevokeQuoteParams{
		QuotedStatusID:  quotedStatusID,
		QuotingStatusID: quotingStatusID,
	})
	return mapErr(err)
}

func (s *PostgresStore) ListQuotesOfStatus(ctx context.Context, quotedStatusID string, maxID *string, limit int) ([]domain.Status, error) {
	cursor := noCursorSentinel
	if maxID != nil && *maxID != "" {
		cursor = *maxID
	}
	rows, err := s.q.ListQuotesOfStatus(ctx, db.ListQuotesOfStatusParams{
		QuotedStatusID: quotedStatusID,
		Column2:        cursor,
		Limit:          int32(limit), //nolint:gosec // limit clamped by caller
	})
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.Status, 0, len(rows))
	for _, r := range rows {
		out = append(out, ToDomainStatus(r))
	}
	return out, nil
}

func (s *PostgresStore) GetQuoteApproval(ctx context.Context, quotingStatusID string) (*domain.QuoteApprovalRecord, error) {
	qa, err := s.q.GetQuoteApproval(ctx, quotingStatusID)
	if err != nil {
		return nil, mapErr(err)
	}
	d := quoteApprovalToDomain(qa)
	return &d, nil
}

func (s *PostgresStore) UpdateStatusQuoteApprovalPolicy(ctx context.Context, statusID, policy string) error {
	return mapErr(s.q.UpdateStatusQuoteApprovalPolicy(ctx, db.UpdateStatusQuoteApprovalPolicyParams{
		ID:                  statusID,
		QuoteApprovalPolicy: policy,
	}))
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

func (s *PostgresStore) CreateAccountPin(ctx context.Context, accountID, statusID string) error {
	return mapErr(s.q.CreateAccountPin(ctx, db.CreateAccountPinParams{
		AccountID: accountID,
		StatusID:  statusID,
	}))
}

func (s *PostgresStore) DeleteAccountPin(ctx context.Context, accountID, statusID string) error {
	return mapErr(s.q.DeleteAccountPin(ctx, db.DeleteAccountPinParams{
		AccountID: accountID,
		StatusID:  statusID,
	}))
}

func (s *PostgresStore) ListPinnedStatusIDs(ctx context.Context, accountID string) ([]string, error) {
	ids, err := s.q.ListPinnedStatusIDs(ctx, accountID)
	if err != nil {
		return nil, mapErr(err)
	}
	return ids, nil
}

func (s *PostgresStore) CountAccountPins(ctx context.Context, accountID string) (int64, error) {
	n, err := s.q.CountAccountPins(ctx, accountID)
	if err != nil {
		return 0, mapErr(err)
	}
	return n, nil
}

func toDbUpdateAccountParams(in store.UpdateAccountInput) db.UpdateAccountParams {
	fields := []byte(in.Fields)
	return db.UpdateAccountParams{
		ID:            in.ID,
		DisplayName:   in.DisplayName,
		Note:          in.Note,
		AvatarMediaID: in.AvatarMediaID,
		HeaderMediaID: in.HeaderMediaID,
		Bot:           in.Bot,
		Locked:        in.Locked,
		Fields:        fields,
		Url:           in.URL,
	}
}

func (s *PostgresStore) UpdateAccount(ctx context.Context, in store.UpdateAccountInput) error {
	_, err := s.q.UpdateAccount(ctx, toDbUpdateAccountParams(in))
	return mapErr(err)
}

func (s *PostgresStore) UpdateAccountKeys(ctx context.Context, id, publicKey string) error {
	return mapErr(s.q.UpdateAccountKeys(ctx, db.UpdateAccountKeysParams{
		ID:        id,
		PublicKey: publicKey,
	}))
}

func (s *PostgresStore) UpdateAccountURLs(ctx context.Context, id, inboxURL, outboxURL, followersURL, followingURL string) error {
	return mapErr(s.q.UpdateAccountURLs(ctx, db.UpdateAccountURLsParams{
		ID:           id,
		InboxURL:     inboxURL,
		OutboxURL:    outboxURL,
		FollowersURL: followersURL,
		FollowingURL: followingURL,
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
		ContentType: in.ContentType,
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

func (s *PostgresStore) ListStatusEdits(ctx context.Context, statusID string) ([]domain.StatusEdit, error) {
	rows, err := s.q.ListStatusEdits(ctx, statusID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.StatusEdit, 0, len(rows))
	for _, r := range rows {
		out = append(out, ToDomainStatusEdit(r))
	}
	return out, nil
}

func (s *PostgresStore) CreateScheduledStatus(ctx context.Context, in store.CreateScheduledStatusInput) (*domain.ScheduledStatus, error) {
	row, err := s.q.CreateScheduledStatus(ctx, db.CreateScheduledStatusParams{
		ID:          in.ID,
		AccountID:   in.AccountID,
		Params:      in.Params,
		ScheduledAt: timeToPg(in.ScheduledAt),
	})
	if err != nil {
		return nil, mapErr(err)
	}
	out := ToDomainScheduledStatus(row)
	return &out, nil
}

func (s *PostgresStore) GetScheduledStatusByID(ctx context.Context, id string) (*domain.ScheduledStatus, error) {
	row, err := s.q.GetScheduledStatusByID(ctx, id)
	if err != nil {
		return nil, mapErr(err)
	}
	out := ToDomainScheduledStatus(row)
	return &out, nil
}

func (s *PostgresStore) ListScheduledStatuses(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.ScheduledStatus, error) {
	maxIDVal := ""
	if maxID != nil {
		maxIDVal = *maxID
	}
	rows, err := s.q.ListScheduledStatuses(ctx, db.ListScheduledStatusesParams{
		AccountID: accountID,
		Column2:   maxIDVal,
		Limit:     int32(limit), //nolint:gosec // limit clamped by caller
	})
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.ScheduledStatus, 0, len(rows))
	for _, r := range rows {
		out = append(out, ToDomainScheduledStatus(r))
	}
	return out, nil
}

func (s *PostgresStore) UpdateScheduledStatus(ctx context.Context, in store.UpdateScheduledStatusInput) (*domain.ScheduledStatus, error) {
	row, err := s.q.UpdateScheduledStatus(ctx, db.UpdateScheduledStatusParams{
		ID:          in.ID,
		Params:      in.Params,
		ScheduledAt: timeToPg(in.ScheduledAt),
	})
	if err != nil {
		return nil, mapErr(err)
	}
	out := ToDomainScheduledStatus(row)
	return &out, nil
}

func (s *PostgresStore) DeleteScheduledStatus(ctx context.Context, id string) error {
	return mapErr(s.q.DeleteScheduledStatus(ctx, id))
}

func (s *PostgresStore) ListScheduledStatusesDue(ctx context.Context, limit int) ([]domain.ScheduledStatus, error) {
	rows, err := s.q.ListScheduledStatusesDue(ctx, int32(limit)) //nolint:gosec // limit clamped by caller
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.ScheduledStatus, 0, len(rows))
	for _, r := range rows {
		out = append(out, ToDomainScheduledStatus(r))
	}
	return out, nil
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

func (s *PostgresStore) GetDistinctFollowerInboxURLsPaginated(ctx context.Context, accountID string, cursor string, limit int) ([]string, error) {
	pageLimit := limit
	if pageLimit <= 0 || pageLimit > 10000 {
		pageLimit = 10000
	}
	urls, err := s.q.GetDistinctFollowerInboxURLsPaginated(ctx, db.GetDistinctFollowerInboxURLsPaginatedParams{
		TargetID: accountID,
		Column2:  cursor,
		Limit:    int32(pageLimit), //nolint:gosec // G115: capped at 10000
	})
	if err != nil {
		return nil, fmt.Errorf("GetDistinctFollowerInboxURLsPaginated: %w", mapErr(err))
	}
	return urls, nil
}

func (s *PostgresStore) GetLocalFollowerAccountIDs(ctx context.Context, targetID string) ([]string, error) {
	ids, err := s.q.GetLocalFollowerAccountIDs(ctx, targetID)
	if err != nil {
		return nil, fmt.Errorf("GetLocalFollowerAccountIDs: %w", mapErr(err))
	}
	return ids, nil
}

func (s *PostgresStore) CreateReport(ctx context.Context, in store.CreateReportInput) (*domain.Report, error) {
	r, err := s.q.CreateReport(ctx, db.CreateReportParams{
		ID:        in.ID,
		AccountID: in.AccountID,
		TargetID:  in.TargetID,
		StatusIds: in.StatusIDs,
		Comment:   in.Comment,
		Category:  in.Category,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainReport(r)
	return &d, nil
}

func (s *PostgresStore) GetReportByID(ctx context.Context, id string) (*domain.Report, error) {
	r, err := s.q.GetReport(ctx, id)
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainReport(r)
	return &d, nil
}

func (s *PostgresStore) ListReports(ctx context.Context, state string, limit, offset int) ([]domain.Report, error) {
	rows, err := s.q.ListReports(ctx, db.ListReportsParams{
		Column1: state,
		Limit:   int32(limit),  //nolint:gosec // G115: limit/offset clamped by caller
		Offset:  int32(offset), //nolint:gosec // G115: limit/offset clamped by caller
	})
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.Report, 0, len(rows))
	for _, r := range rows {
		out = append(out, ToDomainReport(r))
	}
	return out, nil
}

func (s *PostgresStore) AssignReport(ctx context.Context, reportID string, assigneeID *string) error {
	return mapErr(s.q.AssignReport(ctx, db.AssignReportParams{ID: reportID, AssignedToID: assigneeID}))
}

func (s *PostgresStore) ResolveReport(ctx context.Context, reportID string, actionTaken *string) error {
	return mapErr(s.q.ResolveReport(ctx, db.ResolveReportParams{ID: reportID, ActionTaken: actionTaken}))
}

func (s *PostgresStore) CreateAnnouncement(ctx context.Context, in store.CreateAnnouncementInput) (*domain.Announcement, error) {
	a, err := s.q.CreateAnnouncement(ctx, db.CreateAnnouncementParams{
		ID:          in.ID,
		Content:     in.Content,
		StartsAt:    timePtrToPg(in.StartsAt),
		EndsAt:      timePtrToPg(in.EndsAt),
		AllDay:      in.AllDay,
		PublishedAt: timeToPg(in.PublishedAt),
		UpdatedAt:   timeToPg(in.PublishedAt),
	})
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainAnnouncement(a)
	return &d, nil
}

func (s *PostgresStore) UpdateAnnouncement(ctx context.Context, in store.UpdateAnnouncementInput) error {
	return mapErr(s.q.UpdateAnnouncement(ctx, db.UpdateAnnouncementParams{
		ID:          in.ID,
		Content:     in.Content,
		StartsAt:    timePtrToPg(in.StartsAt),
		EndsAt:      timePtrToPg(in.EndsAt),
		AllDay:      in.AllDay,
		PublishedAt: timeToPg(in.PublishedAt),
	}))
}

func (s *PostgresStore) GetAnnouncementByID(ctx context.Context, id string) (*domain.Announcement, error) {
	a, err := s.q.GetAnnouncementByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("GetAnnouncementByID(%s): %w", id, domain.ErrNotFound)
		}
		return nil, fmt.Errorf("GetAnnouncementByID(%s): %w", id, err)
	}
	d := ToDomainAnnouncement(a)
	return &d, nil
}

func (s *PostgresStore) ListActiveAnnouncements(ctx context.Context) ([]domain.Announcement, error) {
	rows, err := s.q.ListActiveAnnouncements(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.Announcement, 0, len(rows))
	for _, r := range rows {
		out = append(out, ToDomainAnnouncement(r))
	}
	return out, nil
}

func (s *PostgresStore) ListAllAnnouncements(ctx context.Context) ([]domain.Announcement, error) {
	rows, err := s.q.ListAllAnnouncements(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.Announcement, 0, len(rows))
	for _, r := range rows {
		out = append(out, ToDomainAnnouncement(r))
	}
	return out, nil
}

func (s *PostgresStore) DismissAnnouncement(ctx context.Context, accountID, announcementID string) error {
	return mapErr(s.q.DismissAnnouncement(ctx, db.DismissAnnouncementParams{
		AccountID:      accountID,
		AnnouncementID: announcementID,
	}))
}

func (s *PostgresStore) ListReadAnnouncementIDs(ctx context.Context, accountID string) ([]string, error) {
	ids, err := s.q.ListReadAnnouncementIDs(ctx, accountID)
	if err != nil {
		return nil, mapErr(err)
	}
	return ids, nil
}

func (s *PostgresStore) AddAnnouncementReaction(ctx context.Context, announcementID, accountID, name string) error {
	return mapErr(s.q.AddAnnouncementReaction(ctx, db.AddAnnouncementReactionParams{
		AnnouncementID: announcementID,
		AccountID:      accountID,
		Name:           name,
	}))
}

func (s *PostgresStore) RemoveAnnouncementReaction(ctx context.Context, announcementID, accountID, name string) error {
	return mapErr(s.q.RemoveAnnouncementReaction(ctx, db.RemoveAnnouncementReactionParams{
		AnnouncementID: announcementID,
		AccountID:      accountID,
		Name:           name,
	}))
}

func (s *PostgresStore) ListAnnouncementReactionCounts(ctx context.Context, announcementID string) ([]domain.AnnouncementReactionCount, error) {
	rows, err := s.q.ListAnnouncementReactionCounts(ctx, announcementID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.AnnouncementReactionCount, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.AnnouncementReactionCount{Name: r.Name, Count: int(r.Count), Me: false})
	}
	return out, nil
}

func (s *PostgresStore) ListAccountAnnouncementReactionNames(ctx context.Context, announcementID, accountID string) ([]string, error) {
	names, err := s.q.ListAccountAnnouncementReactionNames(ctx, db.ListAccountAnnouncementReactionNamesParams{
		AnnouncementID: announcementID,
		AccountID:      accountID,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return names, nil
}

func (s *PostgresStore) CreateDomainBlock(ctx context.Context, in store.CreateDomainBlockInput) (*domain.DomainBlock, error) {
	b, err := s.q.CreateDomainBlock(ctx, db.CreateDomainBlockParams{
		ID:       in.ID,
		Domain:   in.Domain,
		Severity: in.Severity,
		Reason:   in.Reason,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainDomainBlock(b)
	return &d, nil
}

func (s *PostgresStore) DeleteDomainBlock(ctx context.Context, domain string) error {
	return mapErr(s.q.DeleteDomainBlock(ctx, domain))
}

func (s *PostgresStore) CreateAdminAction(ctx context.Context, in store.CreateAdminActionInput) error {
	_, err := s.q.CreateAdminAction(ctx, db.CreateAdminActionParams{
		ID:              in.ID,
		ModeratorID:     in.ModeratorID,
		TargetAccountID: in.TargetAccountID,
		Action:          in.Action,
		Comment:         in.Comment,
		Metadata:        in.Metadata,
	})
	return mapErr(err)
}

func (s *PostgresStore) CreateInvite(ctx context.Context, in store.CreateInviteInput) (*domain.Invite, error) {
	var maxUses *int32
	if in.MaxUses != nil {
		m := int32(*in.MaxUses) //nolint:gosec // G115: admin input
		maxUses = &m
	}
	inv, err := s.q.CreateInvite(ctx, db.CreateInviteParams{
		ID:        in.ID,
		Code:      in.Code,
		CreatedBy: in.CreatedBy,
		MaxUses:   maxUses,
		ExpiresAt: timePtrToPg(in.ExpiresAt),
	})
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainInvite(inv)
	return &d, nil
}

func (s *PostgresStore) GetInviteByCode(ctx context.Context, code string) (*domain.Invite, error) {
	inv, err := s.q.GetInviteByCode(ctx, code)
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainInvite(inv)
	return &d, nil
}

func (s *PostgresStore) ListInvitesByCreator(ctx context.Context, createdByUserID string) ([]domain.Invite, error) {
	rows, err := s.q.ListInvitesByCreator(ctx, createdByUserID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.Invite, 0, len(rows))
	for _, r := range rows {
		out = append(out, ToDomainInvite(r))
	}
	return out, nil
}

func (s *PostgresStore) DeleteInvite(ctx context.Context, id string) error {
	return mapErr(s.q.DeleteInvite(ctx, id))
}

func (s *PostgresStore) IncrementInviteUses(ctx context.Context, code string) error {
	return mapErr(s.q.IncrementInviteUses(ctx, code))
}

func (s *PostgresStore) UpsertKnownInstance(ctx context.Context, id, domain string) error {
	return mapErr(s.q.UpsertKnownInstance(ctx, db.UpsertKnownInstanceParams{ID: id, Domain: domain}))
}

func (s *PostgresStore) ListKnownInstances(ctx context.Context, limit, offset int) ([]domain.KnownInstance, error) {
	rows, err := s.q.ListKnownInstances(ctx, db.ListKnownInstancesParams{Limit: int32(limit), Offset: int32(offset)}) //nolint:gosec // G115: limit/offset clamped by caller
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.KnownInstance, 0, len(rows))
	for _, r := range rows {
		out = append(out, ListKnownInstancesRowToDomain(r))
	}
	return out, nil
}

func (s *PostgresStore) CountKnownInstances(ctx context.Context) (int64, error) {
	n, err := s.q.CountKnownInstances(ctx)
	if err != nil {
		return 0, mapErr(err)
	}
	return n, nil
}

func (s *PostgresStore) CreateServerFilter(ctx context.Context, in store.CreateServerFilterInput) (*domain.ServerFilter, error) {
	f, err := s.q.CreateServerFilter(ctx, db.CreateServerFilterParams{
		ID:     in.ID,
		Phrase: in.Phrase,
		Scope:  in.Scope,
		Action: in.Action,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainServerFilter(f)
	return &d, nil
}

func (s *PostgresStore) ListServerFilters(ctx context.Context) ([]domain.ServerFilter, error) {
	rows, err := s.q.ListServerFilters(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.ServerFilter, 0, len(rows))
	for _, r := range rows {
		out = append(out, ToDomainServerFilter(r))
	}
	return out, nil
}

func (s *PostgresStore) UpdateServerFilter(ctx context.Context, in store.UpdateServerFilterInput) (*domain.ServerFilter, error) {
	f, err := s.q.UpdateServerFilter(ctx, db.UpdateServerFilterParams{
		ID:        in.ID,
		Phrase:    in.Phrase,
		Scope:     in.Scope,
		Action:    in.Action,
		WholeWord: in.WholeWord,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainServerFilter(f)
	return &d, nil
}

func (s *PostgresStore) DeleteServerFilter(ctx context.Context, id string) error {
	return mapErr(s.q.DeleteServerFilter(ctx, id))
}

func (s *PostgresStore) ListLocalUsers(ctx context.Context, limit, offset int) ([]domain.User, error) {
	rows, err := s.q.ListLocalUsers(ctx, db.ListLocalUsersParams{Limit: int32(limit), Offset: int32(offset)}) //nolint:gosec // G115: limit/offset clamped by caller
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.User, 0, len(rows))
	for _, r := range rows {
		out = append(out, ToDomainUser(r))
	}
	return out, nil
}

func (s *PostgresStore) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	u, err := s.q.GetUserByID(ctx, id)
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainUser(u)
	return &d, nil
}

func (s *PostgresStore) UpdateUserRole(ctx context.Context, userID string, role string) error {
	return mapErr(s.q.UpdateUserRole(ctx, db.UpdateUserRoleParams{ID: userID, Role: role}))
}

func (s *PostgresStore) UpdateUserDefaultQuotePolicy(ctx context.Context, accountID, policy string) error {
	return mapErr(s.q.UpdateUserDefaultQuotePolicy(ctx, db.UpdateUserDefaultQuotePolicyParams{
		AccountID:          accountID,
		DefaultQuotePolicy: policy,
	}))
}

func (s *PostgresStore) UpdateUserPreferences(ctx context.Context, in store.UpdateUserPreferencesInput) error {
	return mapErr(s.q.UpdateUserPreferences(ctx, db.UpdateUserPreferencesParams{
		ID:                 in.UserID,
		DefaultPrivacy:     in.DefaultPrivacy,
		DefaultSensitive:   in.DefaultSensitive,
		DefaultLanguage:    in.DefaultLanguage,
		DefaultQuotePolicy: in.DefaultQuotePolicy,
	}))
}

func (s *PostgresStore) UpdateUserEmail(ctx context.Context, userID, email string) error {
	return mapErr(s.q.UpdateUserEmail(ctx, db.UpdateUserEmailParams{ID: userID, Email: email}))
}

func (s *PostgresStore) UpdateUserPassword(ctx context.Context, userID, passwordHash string) error {
	return mapErr(s.q.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{ID: userID, PasswordHash: passwordHash}))
}

func (s *PostgresStore) GetPendingRegistrations(ctx context.Context) ([]domain.User, error) {
	rows, err := s.q.GetPendingRegistrations(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.User, 0, len(rows))
	for _, r := range rows {
		out = append(out, ToDomainUser(r))
	}
	return out, nil
}

func (s *PostgresStore) DeleteUser(ctx context.Context, id string) error {
	return mapErr(s.q.DeleteUser(ctx, id))
}

func (s *PostgresStore) SilenceAccount(ctx context.Context, id string) error {
	return mapErr(s.q.SilenceAccount(ctx, id))
}

func (s *PostgresStore) UnsuspendAccount(ctx context.Context, id string) error {
	return mapErr(s.q.UnsuspendAccount(ctx, id))
}

func (s *PostgresStore) UnsilenceAccount(ctx context.Context, id string) error {
	return mapErr(s.q.UnsilenceAccount(ctx, id))
}

func (s *PostgresStore) DeleteAccount(ctx context.Context, id string) error {
	return mapErr(s.q.DeleteAccount(ctx, id))
}

func (s *PostgresStore) ListLocalAccounts(ctx context.Context, limit, offset int) ([]domain.Account, error) {
	rows, err := s.q.ListLocalAccounts(ctx, db.ListLocalAccountsParams{Limit: int32(limit), Offset: int32(offset)}) //nolint:gosec // G115: limit/offset clamped by caller
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.Account, 0, len(rows))
	for _, row := range rows {
		out = append(out, rowWithURLsToDomainAccount(row.Account, row.AvatarUrl, row.HeaderUrl))
	}
	return out, nil
}

func (s *PostgresStore) CreateUserFilter(ctx context.Context, in store.CreateUserFilterInput) (*domain.UserFilter, error) {
	u, err := s.q.CreateUserFilter(ctx, db.CreateUserFilterParams{
		ID:           in.ID,
		AccountID:    in.AccountID,
		Phrase:       in.Phrase,
		Context:      in.Context,
		WholeWord:    in.WholeWord,
		ExpiresAt:    timePtrToPg(in.ExpiresAt),
		Irreversible: in.Irreversible,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainUserFilter(u)
	return &d, nil
}

func (s *PostgresStore) GetUserFilterByID(ctx context.Context, id string) (*domain.UserFilter, error) {
	u, err := s.q.GetUserFilter(ctx, id)
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainUserFilter(u)
	return &d, nil
}

func (s *PostgresStore) ListUserFilters(ctx context.Context, accountID string) ([]domain.UserFilter, error) {
	rows, err := s.q.ListUserFilters(ctx, accountID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.UserFilter, 0, len(rows))
	for _, r := range rows {
		out = append(out, ToDomainUserFilter(r))
	}
	return out, nil
}

func (s *PostgresStore) UpdateUserFilter(ctx context.Context, in store.UpdateUserFilterInput) (*domain.UserFilter, error) {
	u, err := s.q.UpdateUserFilter(ctx, db.UpdateUserFilterParams{
		ID:           in.ID,
		Phrase:       in.Phrase,
		Context:      in.Context,
		WholeWord:    in.WholeWord,
		ExpiresAt:    timePtrToPg(in.ExpiresAt),
		Irreversible: in.Irreversible,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainUserFilter(u)
	return &d, nil
}

func (s *PostgresStore) DeleteUserFilter(ctx context.Context, id string) error {
	return mapErr(s.q.DeleteUserFilter(ctx, id))
}

func (s *PostgresStore) GetActiveUserFiltersByContext(ctx context.Context, accountID, filterContext string) ([]domain.UserFilter, error) {
	rows, err := s.q.GetActiveUserFiltersByContext(ctx, db.GetActiveUserFiltersByContextParams{
		AccountID: accountID,
		Column2:   filterContext,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.UserFilter, 0, len(rows))
	for _, r := range rows {
		out = append(out, ToDomainUserFilter(r))
	}
	return out, nil
}

func (s *PostgresStore) DeleteFollowsByDomain(ctx context.Context, domain string) error {
	return mapErr(s.q.DeleteFollowsByDomain(ctx, &domain))
}

func (s *PostgresStore) CreateList(ctx context.Context, in store.CreateListInput) (*domain.List, error) {
	l, err := s.q.CreateList(ctx, db.CreateListParams{
		ID:            in.ID,
		AccountID:     in.AccountID,
		Title:         in.Title,
		RepliesPolicy: in.RepliesPolicy,
		Exclusive:     in.Exclusive,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainList(l)
	return &d, nil
}

func (s *PostgresStore) GetListByID(ctx context.Context, id string) (*domain.List, error) {
	l, err := s.q.GetListByID(ctx, id)
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainList(l)
	return &d, nil
}

func (s *PostgresStore) ListLists(ctx context.Context, accountID string) ([]domain.List, error) {
	rows, err := s.q.ListListsByAccount(ctx, accountID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.List, 0, len(rows))
	for _, r := range rows {
		out = append(out, ToDomainList(r))
	}
	return out, nil
}

func (s *PostgresStore) UpdateList(ctx context.Context, in store.UpdateListInput) (*domain.List, error) {
	l, err := s.q.UpdateList(ctx, db.UpdateListParams{
		ID:            in.ID,
		Title:         in.Title,
		RepliesPolicy: in.RepliesPolicy,
		Exclusive:     in.Exclusive,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	d := ToDomainList(l)
	return &d, nil
}

func (s *PostgresStore) DeleteList(ctx context.Context, id string) error {
	return mapErr(s.q.DeleteList(ctx, id))
}

func (s *PostgresStore) ListListAccountIDs(ctx context.Context, listID string) ([]string, error) {
	ids, err := s.q.ListListAccountIDs(ctx, listID)
	if err != nil {
		return nil, mapErr(err)
	}
	return ids, nil
}

func (s *PostgresStore) GetListIDsByMemberAccountID(ctx context.Context, accountID string) ([]string, error) {
	ids, err := s.q.GetListIDsByMemberAccountID(ctx, accountID)
	if err != nil {
		return nil, mapErr(err)
	}
	return ids, nil
}

func (s *PostgresStore) AddAccountToList(ctx context.Context, listID, accountID string) error {
	return mapErr(s.q.AddAccountToList(ctx, db.AddAccountToListParams{
		ListID:    listID,
		AccountID: accountID,
	}))
}

func (s *PostgresStore) RemoveAccountFromList(ctx context.Context, listID, accountID string) error {
	return mapErr(s.q.RemoveAccountFromList(ctx, db.RemoveAccountFromListParams{
		ListID:    listID,
		AccountID: accountID,
	}))
}

func (s *PostgresStore) GetListTimeline(ctx context.Context, listID string, maxID *string, limit int) ([]domain.Status, error) {
	cursor := noCursorSentinel
	if maxID != nil && *maxID != "" {
		cursor = *maxID
	}
	rows, err := s.q.GetListTimeline(ctx, db.GetListTimelineParams{
		ListID:  listID,
		Column2: cursor,
		Limit:   int32(limit), //nolint:gosec // clamped by caller
	})
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.Status, 0, len(rows))
	for _, r := range rows {
		out = append(out, ToDomainStatus(r))
	}
	return out, nil
}

func (s *PostgresStore) GetMarkers(ctx context.Context, accountID string, timelines []string) (map[string]domain.Marker, error) {
	if len(timelines) == 0 {
		return map[string]domain.Marker{}, nil
	}
	rows, err := s.q.GetMarkers(ctx, db.GetMarkersParams{
		AccountID: accountID,
		Column2:   timelines,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	out := make(map[string]domain.Marker, len(rows))
	for _, r := range rows {
		out[r.Timeline] = ToDomainMarker(r)
	}
	return out, nil
}

func (s *PostgresStore) SetMarker(ctx context.Context, accountID, timeline, lastReadID string) error {
	return mapErr(s.q.SetMarker(ctx, db.SetMarkerParams{
		AccountID:  accountID,
		Timeline:   timeline,
		LastReadID: lastReadID,
	}))
}

func (s *PostgresStore) ListDirectoryAccounts(ctx context.Context, order string, localOnly bool, offset, limit int) ([]domain.Account, error) {
	rows, err := s.q.ListDirectoryAccounts(ctx, db.ListDirectoryAccountsParams{
		Column1: localOnly,
		Column2: order,
		Limit:   int32(limit),  //nolint:gosec // clamped by caller
		Offset:  int32(offset), //nolint:gosec // caller-controlled
	})
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.Account, 0, len(rows))
	for _, row := range rows {
		out = append(out, rowWithURLsToDomainAccount(row.Account, row.AvatarUrl, row.HeaderUrl))
	}
	return out, nil
}

func (s *PostgresStore) CreatePoll(ctx context.Context, in store.CreatePollInput) (*domain.Poll, error) {
	p, err := s.q.CreatePoll(ctx, db.CreatePollParams{
		ID:        in.ID,
		StatusID:  in.StatusID,
		ExpiresAt: timePtrToPg(in.ExpiresAt),
		Multiple:  in.Multiple,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	out := ToDomainPoll(p)
	return &out, nil
}

func (s *PostgresStore) CreatePollOption(ctx context.Context, in store.CreatePollOptionInput) (*domain.PollOption, error) {
	o, err := s.q.CreatePollOption(ctx, db.CreatePollOptionParams{
		ID:       in.ID,
		PollID:   in.PollID,
		Title:    in.Title,
		Position: int32(in.Position), //nolint:gosec // position from API
	})
	if err != nil {
		return nil, mapErr(err)
	}
	out := ToDomainPollOption(o)
	return &out, nil
}

func (s *PostgresStore) GetPollByID(ctx context.Context, id string) (*domain.Poll, error) {
	p, err := s.q.GetPollByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("GetPollByID: %w", domain.ErrNotFound)
		}
		return nil, mapErr(err)
	}
	out := ToDomainPoll(p)
	return &out, nil
}

func (s *PostgresStore) GetPollByStatusID(ctx context.Context, statusID string) (*domain.Poll, error) {
	p, err := s.q.GetPollByStatusID(ctx, statusID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("GetPollByStatusID: %w", domain.ErrNotFound)
		}
		return nil, mapErr(err)
	}
	out := ToDomainPoll(p)
	return &out, nil
}

func (s *PostgresStore) ListPollOptions(ctx context.Context, pollID string) ([]domain.PollOption, error) {
	rows, err := s.q.ListPollOptions(ctx, pollID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.PollOption, 0, len(rows))
	for _, r := range rows {
		out = append(out, ToDomainPollOption(r))
	}
	return out, nil
}

func (s *PostgresStore) DeletePollVotesByAccount(ctx context.Context, pollID, accountID string) error {
	return mapErr(s.q.DeletePollVotesByAccount(ctx, db.DeletePollVotesByAccountParams{
		PollID:    pollID,
		AccountID: accountID,
	}))
}

func (s *PostgresStore) CreatePollVote(ctx context.Context, id, pollID, accountID, optionID string) error {
	_, err := s.q.CreatePollVote(ctx, db.CreatePollVoteParams{
		ID:        id,
		PollID:    pollID,
		AccountID: accountID,
		OptionID:  optionID,
	})
	return mapErr(err)
}

func (s *PostgresStore) GetVoteCountsByPoll(ctx context.Context, pollID string) (map[string]int, error) {
	rows, err := s.q.GetVoteCountsByPoll(ctx, pollID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make(map[string]int, len(rows))
	for _, r := range rows {
		out[r.OptionID] = int(r.VotesCount)
	}
	return out, nil
}

func (s *PostgresStore) HasVotedOnPoll(ctx context.Context, pollID, accountID string) (bool, error) {
	voted, err := s.q.HasVotedOnPoll(ctx, db.HasVotedOnPollParams{
		PollID:    pollID,
		AccountID: accountID,
	})
	if err != nil {
		return false, mapErr(err)
	}
	return voted, nil
}

func (s *PostgresStore) GetOwnVoteOptionIDs(ctx context.Context, pollID, accountID string) ([]string, error) {
	ids, err := s.q.GetOwnVoteOptionIDs(ctx, db.GetOwnVoteOptionIDsParams{
		PollID:    pollID,
		AccountID: accountID,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return ids, nil
}

// --- Transactional outbox ---

func (s *PostgresStore) InsertOutboxEvent(ctx context.Context, in store.InsertOutboxEventInput) error {
	err := s.q.InsertOutboxEvent(ctx, db.InsertOutboxEventParams{
		ID:            in.ID,
		EventType:     in.EventType,
		AggregateType: in.AggregateType,
		AggregateID:   in.AggregateID,
		Payload:       in.Payload,
	})
	if err != nil {
		return fmt.Errorf("InsertOutboxEvent(%s): %w", in.ID, mapErr(err))
	}
	return nil
}

func (s *PostgresStore) GetAndLockUnpublishedOutboxEvents(ctx context.Context, limit int) ([]domain.DomainEvent, error) {
	l := int32(1000) //nolint:gosec // limit is bounded below
	if limit < 1000 {
		l = int32(limit) //nolint:gosec // bounded
	}
	rows, err := s.q.GetAndLockUnpublishedOutboxEvents(ctx, l)
	if err != nil {
		return nil, fmt.Errorf("GetAndLockUnpublishedOutboxEvents: %w", mapErr(err))
	}
	events := make([]domain.DomainEvent, len(rows))
	for i, r := range rows {
		events[i] = domain.DomainEvent{
			ID:            r.ID,
			EventType:     r.EventType,
			AggregateType: r.AggregateType,
			AggregateID:   r.AggregateID,
			Payload:       r.Payload,
		}
	}
	return events, nil
}

func (s *PostgresStore) MarkOutboxEventsPublished(ctx context.Context, ids []string) error {
	if err := s.q.MarkOutboxEventsPublished(ctx, ids); err != nil {
		return fmt.Errorf("MarkOutboxEventsPublished: %w", mapErr(err))
	}
	return nil
}

func (s *PostgresStore) DeletePublishedOutboxEventsBefore(ctx context.Context, before time.Time) error {
	ts := pgtype.Timestamptz{Time: before, Valid: true}
	if err := s.q.DeletePublishedOutboxEventsBefore(ctx, ts); err != nil {
		return fmt.Errorf("DeletePublishedOutboxEventsBefore: %w", mapErr(err))
	}
	return nil
}
