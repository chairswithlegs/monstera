package postgres

import (
	"encoding/json"
	"time"

	"github.com/chairswithlegs/monstera-fed/internal/domain"
	db "github.com/chairswithlegs/monstera-fed/internal/store/postgres/generated"
	"github.com/jackc/pgx/v5/pgtype"
)

func pgTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

func pgTimePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

func timeToPg(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func timePtrToPg(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// ToDomainAccount converts a sqlc db.Account to a domain.Account.
func ToDomainAccount(a db.Account) domain.Account {
	d := domain.Account{
		ID:             a.ID,
		Username:       a.Username,
		Domain:         a.Domain,
		DisplayName:    a.DisplayName,
		Note:           a.Note,
		AvatarMediaID:  a.AvatarMediaID,
		HeaderMediaID:  a.HeaderMediaID,
		PublicKey:      a.PublicKey,
		PrivateKey:     a.PrivateKey,
		InboxURL:       a.InboxUrl,
		OutboxURL:      a.OutboxUrl,
		FollowersURL:   a.FollowersUrl,
		FollowingURL:   a.FollowingUrl,
		APID:           a.ApID,
		FollowersCount: int(a.FollowersCount),
		FollowingCount: int(a.FollowingCount),
		StatusesCount:  int(a.StatusesCount),
		Bot:            a.Bot,
		Locked:         a.Locked,
		Suspended:      a.Suspended,
		Silenced:       a.Silenced,
		CreatedAt:      pgTime(a.CreatedAt),
		UpdatedAt:      pgTime(a.UpdatedAt),
	}
	if len(a.ApRaw) > 0 {
		d.APRaw = json.RawMessage(a.ApRaw)
	}
	if len(a.Fields) > 0 {
		d.Fields = json.RawMessage(a.Fields)
	}
	return d
}

// ToDomainStatus converts a sqlc db.Status to a domain.Status.
func ToDomainStatus(s db.Status) domain.Status {
	d := domain.Status{
		ID:                 s.ID,
		URI:                s.Uri,
		AccountID:          s.AccountID,
		Text:               s.Text,
		Content:            s.Content,
		ContentWarning:     s.ContentWarning,
		Visibility:         s.Visibility,
		Language:           s.Language,
		InReplyToID:        s.InReplyToID,
		InReplyToAccountID: s.InReplyToAccountID,
		ReblogOfID:         s.ReblogOfID,
		APID:               s.ApID,
		Sensitive:          s.Sensitive,
		Local:              s.Local,
		RepliesCount:       int(s.RepliesCount),
		ReblogsCount:       int(s.ReblogsCount),
		FavouritesCount:    int(s.FavouritesCount),
		CreatedAt:          pgTime(s.CreatedAt),
		UpdatedAt:          pgTime(s.UpdatedAt),
	}
	if len(s.ApRaw) > 0 {
		d.APRaw = json.RawMessage(s.ApRaw)
	}
	d.EditedAt = pgTimePtr(s.EditedAt)
	d.DeletedAt = pgTimePtr(s.DeletedAt)
	return d
}

// ToDomainUser converts a sqlc db.User to a domain.User.
func ToDomainUser(u db.User) domain.User {
	d := domain.User{
		ID:               u.ID,
		AccountID:        u.AccountID,
		Email:            u.Email,
		PasswordHash:     u.PasswordHash,
		Role:             u.Role,
		DefaultPrivacy:   u.DefaultPrivacy,
		DefaultSensitive: u.DefaultSensitive,
		DefaultLanguage:  u.DefaultLanguage,
		CreatedAt:        pgTime(u.CreatedAt),
	}
	d.ConfirmedAt = pgTimePtr(u.ConfirmedAt)
	return d
}

// ToDomainOAuthApplication converts a sqlc db.OauthApplication to a domain.OAuthApplication.
func ToDomainOAuthApplication(a db.OauthApplication) domain.OAuthApplication {
	return domain.OAuthApplication{
		ID:           a.ID,
		Name:         a.Name,
		ClientID:     a.ClientID,
		ClientSecret: a.ClientSecret,
		RedirectURIs: a.RedirectUris,
		Scopes:       a.Scopes,
		Website:      a.Website,
		CreatedAt:    pgTime(a.CreatedAt),
	}
}

// ToDomainOAuthAccessToken converts a sqlc db.OauthAccessToken to a domain.OAuthAccessToken.
func ToDomainOAuthAccessToken(t db.OauthAccessToken) domain.OAuthAccessToken {
	return domain.OAuthAccessToken{
		ID:            t.ID,
		ApplicationID: t.ApplicationID,
		AccountID:     t.AccountID,
		Token:         t.Token,
		Scopes:        t.Scopes,
		ExpiresAt:     pgTimePtr(t.ExpiresAt),
		RevokedAt:     pgTimePtr(t.RevokedAt),
		CreatedAt:     pgTime(t.CreatedAt),
	}
}

// ToDomainOAuthAuthorizationCode converts a sqlc db.OauthAuthorizationCode to a domain.OAuthAuthorizationCode.
func ToDomainOAuthAuthorizationCode(c db.OauthAuthorizationCode) domain.OAuthAuthorizationCode {
	return domain.OAuthAuthorizationCode{
		ID:                  c.ID,
		Code:                c.Code,
		ApplicationID:       c.ApplicationID,
		AccountID:           c.AccountID,
		RedirectURI:         c.RedirectUri,
		Scopes:              c.Scopes,
		CodeChallenge:       c.CodeChallenge,
		CodeChallengeMethod: c.CodeChallengeMethod,
		ExpiresAt:           pgTime(c.ExpiresAt),
		CreatedAt:           pgTime(c.CreatedAt),
	}
}
