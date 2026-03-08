package postgres

import (
	"encoding/json"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
	db "github.com/chairswithlegs/monstera/internal/store/postgres/generated"
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

// ToDomainUserFilter converts a sqlc db.UserFilter to a domain.UserFilter.
func ToDomainUserFilter(u db.UserFilter) domain.UserFilter {
	d := domain.UserFilter{
		ID:           u.ID,
		AccountID:    u.AccountID,
		Phrase:       u.Phrase,
		Context:      u.Context,
		WholeWord:    u.WholeWord,
		Irreversible: u.Irreversible,
		CreatedAt:    pgTime(u.CreatedAt),
	}
	d.ExpiresAt = pgTimePtr(u.ExpiresAt)
	return d
}

// ToDomainStatusEdit converts a sqlc db.StatusEdit to a domain.StatusEdit.
func ToDomainStatusEdit(e db.StatusEdit) domain.StatusEdit {
	return domain.StatusEdit{
		ID:             e.ID,
		StatusID:       e.StatusID,
		AccountID:      e.AccountID,
		Text:           e.Text,
		Content:        e.Content,
		ContentWarning: e.ContentWarning,
		Sensitive:      e.Sensitive,
		CreatedAt:      pgTime(e.CreatedAt),
	}
}

// ToDomainScheduledStatus converts a sqlc db.ScheduledStatus to a domain.ScheduledStatus.
func ToDomainScheduledStatus(s db.ScheduledStatus) domain.ScheduledStatus {
	return domain.ScheduledStatus{
		ID:          s.ID,
		AccountID:   s.AccountID,
		Params:      s.Params,
		ScheduledAt: pgTime(s.ScheduledAt),
		CreatedAt:   pgTime(s.CreatedAt),
	}
}

// ToDomainPoll converts a sqlc db.Poll to a domain.Poll.
func ToDomainPoll(p db.Poll) domain.Poll {
	out := domain.Poll{
		ID:        p.ID,
		StatusID:  p.StatusID,
		Multiple:  p.Multiple,
		CreatedAt: pgTime(p.CreatedAt),
	}
	out.ExpiresAt = pgTimePtr(p.ExpiresAt)
	return out
}

// ToDomainPollOption converts a sqlc db.PollOption to a domain.PollOption.
func ToDomainPollOption(o db.PollOption) domain.PollOption {
	return domain.PollOption{
		ID:       o.ID,
		PollID:   o.PollID,
		Title:    o.Title,
		Position: int(o.Position),
	}
}

// ToDomainList converts a sqlc db.List to a domain.List.
func ToDomainList(l db.List) domain.List {
	return domain.List{
		ID:            l.ID,
		AccountID:     l.AccountID,
		Title:         l.Title,
		RepliesPolicy: l.RepliesPolicy,
		Exclusive:     l.Exclusive,
		CreatedAt:     pgTime(l.CreatedAt),
	}
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

// MutedAccountRowToDomainAccount converts ListMutedAccountsPaginatedRow to domain.Account.
func MutedAccountRowToDomainAccount(r db.ListMutedAccountsPaginatedRow) domain.Account {
	a := db.Account{
		ID:             r.ID,
		Username:       r.Username,
		Domain:         r.Domain,
		DisplayName:    r.DisplayName,
		Note:           r.Note,
		PublicKey:      r.PublicKey,
		PrivateKey:     r.PrivateKey,
		InboxUrl:       r.InboxUrl,
		OutboxUrl:      r.OutboxUrl,
		FollowersUrl:   r.FollowersUrl,
		FollowingUrl:   r.FollowingUrl,
		ApID:           r.ApID,
		ApRaw:          r.ApRaw,
		Bot:            r.Bot,
		Locked:         r.Locked,
		Suspended:      r.Suspended,
		Silenced:       r.Silenced,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
		AvatarMediaID:  r.AvatarMediaID,
		HeaderMediaID:  r.HeaderMediaID,
		FollowersCount: r.FollowersCount,
		FollowingCount: r.FollowingCount,
		StatusesCount:  r.StatusesCount,
		Fields:         r.Fields,
	}
	return ToDomainAccount(a)
}

// RebloggedByRowToDomainAccount converts GetRebloggedByRow to domain.Account (row omits last_status_at).
func RebloggedByRowToDomainAccount(r db.GetRebloggedByRow) domain.Account {
	a := db.Account{
		ID:             r.ID,
		Username:       r.Username,
		Domain:         r.Domain,
		DisplayName:    r.DisplayName,
		Note:           r.Note,
		PublicKey:      r.PublicKey,
		PrivateKey:     r.PrivateKey,
		InboxUrl:       r.InboxUrl,
		OutboxUrl:      r.OutboxUrl,
		FollowersUrl:   r.FollowersUrl,
		FollowingUrl:   r.FollowingUrl,
		ApID:           r.ApID,
		ApRaw:          r.ApRaw,
		Bot:            r.Bot,
		Locked:         r.Locked,
		Suspended:      r.Suspended,
		Silenced:       r.Silenced,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
		AvatarMediaID:  r.AvatarMediaID,
		HeaderMediaID:  r.HeaderMediaID,
		FollowersCount: r.FollowersCount,
		FollowingCount: r.FollowingCount,
		StatusesCount:  r.StatusesCount,
		Fields:         r.Fields,
	}
	return ToDomainAccount(a)
}

// BlockedAccountRowToDomainAccount converts ListBlockedAccountsPaginatedRow to domain.Account.
func BlockedAccountRowToDomainAccount(r db.ListBlockedAccountsPaginatedRow) domain.Account {
	a := db.Account{
		ID:             r.ID,
		Username:       r.Username,
		Domain:         r.Domain,
		DisplayName:    r.DisplayName,
		Note:           r.Note,
		PublicKey:      r.PublicKey,
		PrivateKey:     r.PrivateKey,
		InboxUrl:       r.InboxUrl,
		OutboxUrl:      r.OutboxUrl,
		FollowersUrl:   r.FollowersUrl,
		FollowingUrl:   r.FollowingUrl,
		ApID:           r.ApID,
		ApRaw:          r.ApRaw,
		Bot:            r.Bot,
		Locked:         r.Locked,
		Suspended:      r.Suspended,
		Silenced:       r.Silenced,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
		AvatarMediaID:  r.AvatarMediaID,
		HeaderMediaID:  r.HeaderMediaID,
		FollowersCount: r.FollowersCount,
		FollowingCount: r.FollowingCount,
		StatusesCount:  r.StatusesCount,
		Fields:         r.Fields,
	}
	return ToDomainAccount(a)
}

// PendingFollowRequestRowToDomainAccount converts GetPendingFollowRequestsPaginatedRow to domain.Account.
func PendingFollowRequestRowToDomainAccount(r db.GetPendingFollowRequestsPaginatedRow) domain.Account {
	a := db.Account{
		ID:             r.ID,
		Username:       r.Username,
		Domain:         r.Domain,
		DisplayName:    r.DisplayName,
		Note:           r.Note,
		PublicKey:      r.PublicKey,
		PrivateKey:     r.PrivateKey,
		InboxUrl:       r.InboxUrl,
		OutboxUrl:      r.OutboxUrl,
		FollowersUrl:   r.FollowersUrl,
		FollowingUrl:   r.FollowingUrl,
		ApID:           r.ApID,
		ApRaw:          r.ApRaw,
		Bot:            r.Bot,
		Locked:         r.Locked,
		Suspended:      r.Suspended,
		Silenced:       r.Silenced,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
		AvatarMediaID:  r.AvatarMediaID,
		HeaderMediaID:  r.HeaderMediaID,
		FollowersCount: r.FollowersCount,
		FollowingCount: r.FollowingCount,
		StatusesCount:  r.StatusesCount,
		Fields:         r.Fields,
	}
	return ToDomainAccount(a)
}

// ToDomainStatus converts a sqlc db.Status to a domain.Status.
func ToDomainStatus(s db.Status) domain.Status {
	d := domain.Status{
		ID:                  s.ID,
		URI:                 s.Uri,
		AccountID:           s.AccountID,
		Text:                s.Text,
		Content:             s.Content,
		ContentWarning:      s.ContentWarning,
		Visibility:          s.Visibility,
		Language:            s.Language,
		InReplyToID:         s.InReplyToID,
		InReplyToAccountID:  s.InReplyToAccountID,
		ReblogOfID:          s.ReblogOfID,
		QuotedStatusID:      s.QuotedStatusID,
		QuoteApprovalPolicy: s.QuoteApprovalPolicy,
		QuotesCount:         int(s.QuotesCount),
		APID:                s.ApID,
		Sensitive:           s.Sensitive,
		Local:               s.Local,
		RepliesCount:        int(s.RepliesCount),
		ReblogsCount:        int(s.ReblogsCount),
		FavouritesCount:     int(s.FavouritesCount),
		CreatedAt:           pgTime(s.CreatedAt),
		UpdatedAt:           pgTime(s.UpdatedAt),
	}
	if len(s.ApRaw) > 0 {
		d.APRaw = json.RawMessage(s.ApRaw)
	}
	d.EditedAt = pgTimePtr(s.EditedAt)
	d.DeletedAt = pgTimePtr(s.DeletedAt)
	return d
}

// quoteApprovalToDomain converts a sqlc db.QuoteApproval to domain.QuoteApprovalRecord.
func quoteApprovalToDomain(qa db.QuoteApproval) domain.QuoteApprovalRecord {
	d := domain.QuoteApprovalRecord{
		QuotingStatusID: qa.QuotingStatusID,
		QuotedStatusID:  qa.QuotedStatusID,
	}
	d.RevokedAt = pgTimePtr(qa.RevokedAt)
	return d
}

// statusRowToDomain converts a status row (ancestors/descendants) to domain.Status.
func statusRowToDomain(id, uri, accountID string, text, content, contentWarning *string, visibility string, language *string, inReplyToID, reblogOfID, quotedStatusID *string, quoteApprovalPolicy string, quotesCount int32, apID string, apRaw []byte, sensitive, local bool, editedAt, createdAt, updatedAt, deletedAt pgtype.Timestamptz, repliesCount, reblogsCount, favouritesCount int32, inReplyToAccountID *string) domain.Status {
	d := domain.Status{
		ID:                  id,
		URI:                 uri,
		AccountID:           accountID,
		Text:                text,
		Content:             content,
		ContentWarning:      contentWarning,
		Visibility:          visibility,
		Language:            language,
		InReplyToID:         inReplyToID,
		InReplyToAccountID:  inReplyToAccountID,
		ReblogOfID:          reblogOfID,
		QuotedStatusID:      quotedStatusID,
		QuoteApprovalPolicy: quoteApprovalPolicy,
		QuotesCount:         int(quotesCount),
		APID:                apID,
		Sensitive:           sensitive,
		Local:               local,
		RepliesCount:        int(repliesCount),
		ReblogsCount:        int(reblogsCount),
		FavouritesCount:     int(favouritesCount),
		CreatedAt:           pgTime(createdAt),
		UpdatedAt:           pgTime(updatedAt),
	}
	if len(apRaw) > 0 {
		d.APRaw = json.RawMessage(apRaw)
	}
	d.EditedAt = pgTimePtr(editedAt)
	d.DeletedAt = pgTimePtr(deletedAt)
	return d
}

// AncestorRowToDomain converts GetStatusAncestorsRow to domain.Status.
func AncestorRowToDomain(r db.GetStatusAncestorsRow) domain.Status {
	return statusRowToDomain(
		r.ID, r.Uri, r.AccountID, r.Text, r.Content, r.ContentWarning,
		r.Visibility, r.Language, r.InReplyToID, r.ReblogOfID, r.QuotedStatusID, r.QuoteApprovalPolicy, r.QuotesCount,
		r.ApID, r.ApRaw,
		r.Sensitive, r.Local, r.EditedAt, r.CreatedAt, r.UpdatedAt, r.DeletedAt,
		r.RepliesCount, r.ReblogsCount, r.FavouritesCount, r.InReplyToAccountID,
	)
}

// DescendantRowToDomain converts GetStatusDescendantsRow to domain.Status.
func DescendantRowToDomain(r db.GetStatusDescendantsRow) domain.Status {
	return statusRowToDomain(
		r.ID, r.Uri, r.AccountID, r.Text, r.Content, r.ContentWarning,
		r.Visibility, r.Language, r.InReplyToID, r.ReblogOfID, r.QuotedStatusID, r.QuoteApprovalPolicy, r.QuotesCount,
		r.ApID, r.ApRaw,
		r.Sensitive, r.Local, r.EditedAt, r.CreatedAt, r.UpdatedAt, r.DeletedAt,
		r.RepliesCount, r.ReblogsCount, r.FavouritesCount, r.InReplyToAccountID,
	)
}

// FavouritesTimelineRowToDomain converts GetFavouritesTimelineRow to domain.Status.
// Favourites query does not select quote columns; use zero values.
func FavouritesTimelineRowToDomain(r db.GetFavouritesTimelineRow) domain.Status {
	return statusRowToDomain(
		r.ID, r.Uri, r.AccountID, r.Text, r.Content, r.ContentWarning,
		r.Visibility, r.Language, r.InReplyToID, r.ReblogOfID, nil, "", 0,
		r.ApID, r.ApRaw,
		r.Sensitive, r.Local, r.EditedAt, r.CreatedAt, r.UpdatedAt, r.DeletedAt,
		r.RepliesCount, r.ReblogsCount, r.FavouritesCount, r.InReplyToAccountID,
	)
}

// BookmarksTimelineRowToDomain converts GetBookmarksTimelineRow to domain.Status.
// Bookmarks query does not select quote columns; use zero values.
func BookmarksTimelineRowToDomain(r db.GetBookmarksTimelineRow) domain.Status {
	return statusRowToDomain(
		r.ID, r.Uri, r.AccountID, r.Text, r.Content, r.ContentWarning,
		r.Visibility, r.Language, r.InReplyToID, r.ReblogOfID, nil, "", 0,
		r.ApID, r.ApRaw,
		r.Sensitive, r.Local, r.EditedAt, r.CreatedAt, r.UpdatedAt, r.DeletedAt,
		r.RepliesCount, r.ReblogsCount, r.FavouritesCount, r.InReplyToAccountID,
	)
}

// ToDomainUser converts a sqlc db.User to a domain.User.
func ToDomainUser(u db.User) domain.User {
	d := domain.User{
		ID:                 u.ID,
		AccountID:          u.AccountID,
		Email:              u.Email,
		PasswordHash:       u.PasswordHash,
		Role:               u.Role,
		RegistrationReason: u.RegistrationReason,
		DefaultPrivacy:     u.DefaultPrivacy,
		DefaultSensitive:   u.DefaultSensitive,
		DefaultLanguage:    u.DefaultLanguage,
		DefaultQuotePolicy: u.DefaultQuotePolicy,
		CreatedAt:          pgTime(u.CreatedAt),
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

// ToDomainNotification converts a sqlc db.Notification to a domain.Notification.
func ToDomainNotification(n db.Notification) domain.Notification {
	return domain.Notification{
		ID:        n.ID,
		AccountID: n.AccountID,
		FromID:    n.FromID,
		Type:      n.Type,
		StatusID:  n.StatusID,
		Read:      n.Read,
		CreatedAt: pgTime(n.CreatedAt),
	}
}

// ToDomainHashtag converts a sqlc db.Hashtag to a domain.Hashtag.
func ToDomainHashtag(h db.Hashtag) domain.Hashtag {
	return domain.Hashtag{
		ID:        h.ID,
		Name:      h.Name,
		CreatedAt: pgTime(h.CreatedAt),
		UpdatedAt: pgTime(h.UpdatedAt),
	}
}

// ToDomainMarker converts a sqlc db.Marker to a domain.Marker.
func ToDomainMarker(m db.Marker) domain.Marker {
	return domain.Marker{
		LastReadID: m.LastReadID,
		Version:    int(m.Version),
		UpdatedAt:  pgTime(m.UpdatedAt),
	}
}

// ToDomainAccountConversation converts a sqlc db.AccountConversation to a domain.AccountConversation.
func ToDomainAccountConversation(a db.AccountConversation) domain.AccountConversation {
	d := domain.AccountConversation{
		ID:             a.ID,
		AccountID:      a.AccountID,
		ConversationID: a.ConversationID,
		Unread:         a.Unread,
		CreatedAt:      pgTime(a.CreatedAt),
		UpdatedAt:      pgTime(a.UpdatedAt),
	}
	if a.LastStatusID != nil {
		d.LastStatusID = a.LastStatusID
	}
	return d
}

// ToDomainAnnouncement converts a sqlc db.Announcement to a domain.Announcement.
func ToDomainAnnouncement(a db.Announcement) domain.Announcement {
	out := domain.Announcement{
		ID:          a.ID,
		Content:     a.Content,
		AllDay:      a.AllDay,
		PublishedAt: pgTime(a.PublishedAt),
		UpdatedAt:   pgTime(a.UpdatedAt),
	}
	out.StartsAt = pgTimePtr(a.StartsAt)
	out.EndsAt = pgTimePtr(a.EndsAt)
	return out
}

// ToDomainDomainBlock converts a sqlc db.DomainBlock to a domain.DomainBlock.
func ToDomainDomainBlock(b db.DomainBlock) domain.DomainBlock {
	return domain.DomainBlock{
		ID:        b.ID,
		Domain:    b.Domain,
		Severity:  b.Severity,
		Reason:    b.Reason,
		CreatedAt: pgTime(b.CreatedAt),
	}
}

// ToDomainFollow converts a sqlc db.Follow to a domain.Follow.
func ToDomainFollow(f db.Follow) domain.Follow {
	return domain.Follow{
		ID:        f.ID,
		AccountID: f.AccountID,
		TargetID:  f.TargetID,
		State:     f.State,
		APID:      f.ApID,
		CreatedAt: pgTime(f.CreatedAt),
	}
}

// ToDomainBlock converts a sqlc db.Block to a domain.Block.
func ToDomainBlock(b db.Block) domain.Block {
	return domain.Block{
		ID:        b.ID,
		AccountID: b.AccountID,
		TargetID:  b.TargetID,
		CreatedAt: pgTime(b.CreatedAt),
	}
}

// ToDomainMute converts a sqlc db.Mute to a domain.Mute.
func ToDomainMute(m db.Mute) domain.Mute {
	return domain.Mute{
		ID:                m.ID,
		AccountID:         m.AccountID,
		TargetID:          m.TargetID,
		HideNotifications: m.HideNotifications,
		CreatedAt:         pgTime(m.CreatedAt),
	}
}

// ToDomainFavourite converts a sqlc db.Favourite to a domain.Favourite.
func ToDomainFavourite(f db.Favourite) domain.Favourite {
	d := domain.Favourite{
		ID:        f.ID,
		AccountID: f.AccountID,
		StatusID:  f.StatusID,
		CreatedAt: pgTime(f.CreatedAt),
	}
	if f.ApID != nil {
		d.APID = *f.ApID
	}
	return d
}

// ToDomainMediaAttachment converts a sqlc db.MediaAttachment to a domain.MediaAttachment.
func ToDomainMediaAttachment(m db.MediaAttachment) domain.MediaAttachment {
	d := domain.MediaAttachment{
		ID:          m.ID,
		AccountID:   m.AccountID,
		StatusID:    m.StatusID,
		Type:        m.Type,
		StorageKey:  m.StorageKey,
		URL:         m.Url,
		PreviewURL:  m.PreviewUrl,
		RemoteURL:   m.RemoteUrl,
		Description: m.Description,
		Blurhash:    m.Blurhash,
		CreatedAt:   pgTime(m.CreatedAt),
	}
	if len(m.Meta) > 0 {
		d.Meta = json.RawMessage(m.Meta)
	}
	return d
}

// ToDomainReport converts a sqlc db.Report to a domain.Report.
func ToDomainReport(r db.Report) domain.Report {
	d := domain.Report{
		ID:           r.ID,
		AccountID:    r.AccountID,
		TargetID:     r.TargetID,
		StatusIDs:    r.StatusIds,
		Comment:      r.Comment,
		Category:     r.Category,
		State:        r.State,
		AssignedToID: r.AssignedToID,
		ActionTaken:  r.ActionTaken,
		CreatedAt:    pgTime(r.CreatedAt),
	}
	d.ResolvedAt = pgTimePtr(r.ResolvedAt)
	return d
}

// ToDomainInvite converts a sqlc db.Invite to a domain.Invite.
func ToDomainInvite(i db.Invite) domain.Invite {
	d := domain.Invite{
		ID:        i.ID,
		Code:      i.Code,
		CreatedBy: i.CreatedBy,
		Uses:      int(i.Uses),
		CreatedAt: pgTime(i.CreatedAt),
	}
	if i.MaxUses != nil {
		m := int(*i.MaxUses)
		d.MaxUses = &m
	}
	d.ExpiresAt = pgTimePtr(i.ExpiresAt)
	return d
}

// ToDomainAdminAction converts a sqlc db.AdminAction to a domain.AdminAction.
func ToDomainAdminAction(a db.AdminAction) domain.AdminAction {
	return domain.AdminAction{
		ID:              a.ID,
		ModeratorID:     a.ModeratorID,
		TargetAccountID: a.TargetAccountID,
		Action:          a.Action,
		Comment:         a.Comment,
		Metadata:        a.Metadata,
		CreatedAt:       pgTime(a.CreatedAt),
	}
}

// ListKnownInstancesRowToDomain converts a sqlc ListKnownInstancesRow to a domain.KnownInstance.
func ListKnownInstancesRowToDomain(r db.ListKnownInstancesRow) domain.KnownInstance {
	d := domain.KnownInstance{
		ID:              r.ID,
		Domain:          r.Domain,
		Software:        r.Software,
		SoftwareVersion: r.SoftwareVersion,
		AccountsCount:   r.AccountsCount,
	}
	d.FirstSeenAt = pgTime(r.FirstSeenAt)
	d.LastSeenAt = pgTime(r.LastSeenAt)
	return d
}

// ToDomainServerFilter converts a sqlc db.ServerFilter to a domain.ServerFilter.
func ToDomainServerFilter(s db.ServerFilter) domain.ServerFilter {
	return domain.ServerFilter{
		ID:        s.ID,
		Phrase:    s.Phrase,
		Scope:     s.Scope,
		Action:    s.Action,
		WholeWord: s.WholeWord,
		CreatedAt: pgTime(s.CreatedAt),
		UpdatedAt: pgTime(s.UpdatedAt),
	}
}
