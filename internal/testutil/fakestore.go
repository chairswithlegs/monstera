package testutil

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
)

// FakeStore implements store.Store for unit tests using in-memory domain types.
// Use NewFakeStore() to create an instance.
type FakeStore struct {
	mu sync.Mutex

	accountsByID           map[string]*domain.Account
	accountsByUsername     map[string]*domain.Account
	usersByAccountID       map[string]*domain.User
	statusesByID           map[string]*domain.Status
	statusesCount          map[string]int
	homeTimeline           map[string][]*domain.Status
	publicTimeline         []*domain.Status
	followsByKey           map[string]*domain.Follow // "accountID:targetID"
	blocksByKey            map[string]struct{}       // "accountID:targetID"
	blocksList             []blockEntry              // for ListBlockedAccounts order
	mutesByKey             map[string]struct{}       // "accountID:targetID"
	mutesList              []muteEntry               // for ListMutedAccounts order
	suspendedAccountIDs    map[string]struct{}
	mediaByID              map[string]*domain.MediaAttachment
	notificationsByAccount map[string][]*domain.Notification
	hashtagsByName         map[string]*domain.Hashtag
	hashtagsByID           map[string]*domain.Hashtag
	followedTagsList       []followedTagEntry      // for ListFollowedTags order
	featuredTagsList       []featuredTagEntry      // for ListFeaturedTags order
	conversationMutesByKey map[string]struct{}     // "accountID:conversationID"
	conversationMutesList  []conversationMuteEntry // for ListMutedConversationIDs
	conversationIDs        map[string]struct{}     // for CreateConversation
	statusConversationIDs  map[string]string       // statusID -> conversationID
	accountConversations   []accountConversationEntry
	announcementsByID      map[string]*domain.Announcement
	announcementReads      map[string]struct{} // "accountID:announcementID"
	announcementReactions  []announcementReactionEntry
	monsteraSettings       *domain.MonsteraSettings

	applications     map[string]*domain.OAuthApplication
	applicationsByID map[string]*domain.OAuthApplication
	authCodes        map[string]*domain.OAuthAuthorizationCode
	tokens           map[string]*domain.OAuthAccessToken

	userFiltersByID map[string]*domain.UserFilter

	listsByID      map[string]*domain.List
	listAccountIDs map[string][]string

	mentionsByStatusID map[string][]string // statusID -> accountIDs mentioned

	markersByKey map[string]*domain.Marker // "accountID:timeline"

	lastStatusAtByAccount map[string]time.Time // for ListDirectoryAccounts "active" order

	accountPins       []pinEntry          // accountID, statusID, created_at for ListPinnedStatusIDs order
	statusEdits       []domain.StatusEdit // for ListStatusEdits
	scheduledStatuses map[string]*domain.ScheduledStatus

	pollsByID       map[string]*domain.Poll
	pollsByStatusID map[string]*domain.Poll
	pollOptions     map[string][]domain.PollOption // pollID -> options ordered by position
	pollVotes       []pollVoteEntry                // poll_id, account_id, option_id

	quoteApprovalsByQuoting map[string]*quoteApprovalEntry // quotingStatusID -> entry

	domainBlocksByDomain map[string]*domain.DomainBlock // domain (normalized) -> block

	favouritesByID            map[string]*domain.Favourite // id -> favourite
	favouritesByAPID          map[string]*domain.Favourite // apID -> favourite (when set)
	favouritesByAccountStatus map[string]*domain.Favourite // accountID+":"+statusID -> favourite
}

type quoteApprovalEntry struct {
	quotedStatusID string
	revokedAt      *time.Time
}

type pollVoteEntry struct {
	pollID    string
	accountID string
	optionID  string
}

type pinEntry struct {
	accountID string
	statusID  string
	createdAt time.Time
}

type followedTagEntry struct {
	ID        string
	AccountID string
	TagID     string
}

type featuredTagEntry struct {
	ID        string
	AccountID string
	TagID     string
}

type conversationMuteEntry struct {
	AccountID      string
	ConversationID string
}

type accountConversationEntry struct {
	ID             string
	AccountID      string
	ConversationID string
	LastStatusID   string
	Unread         bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type announcementReactionEntry struct {
	AnnouncementID string
	AccountID      string
	Name           string
}

// NewFakeStore returns a new FakeStore for use in tests.
func NewFakeStore() *FakeStore {
	return &FakeStore{
		accountsByID:              make(map[string]*domain.Account),
		accountsByUsername:        make(map[string]*domain.Account),
		usersByAccountID:          make(map[string]*domain.User),
		statusesByID:              make(map[string]*domain.Status),
		statusesCount:             make(map[string]int),
		homeTimeline:              make(map[string][]*domain.Status),
		followsByKey:              make(map[string]*domain.Follow),
		blocksByKey:               make(map[string]struct{}),
		mutesByKey:                make(map[string]struct{}),
		suspendedAccountIDs:       make(map[string]struct{}),
		mediaByID:                 make(map[string]*domain.MediaAttachment),
		notificationsByAccount:    make(map[string][]*domain.Notification),
		hashtagsByName:            make(map[string]*domain.Hashtag),
		hashtagsByID:              make(map[string]*domain.Hashtag),
		applications:              make(map[string]*domain.OAuthApplication),
		applicationsByID:          make(map[string]*domain.OAuthApplication),
		authCodes:                 make(map[string]*domain.OAuthAuthorizationCode),
		tokens:                    make(map[string]*domain.OAuthAccessToken),
		userFiltersByID:           make(map[string]*domain.UserFilter),
		listsByID:                 make(map[string]*domain.List),
		listAccountIDs:            make(map[string][]string),
		mentionsByStatusID:        make(map[string][]string),
		markersByKey:              make(map[string]*domain.Marker),
		lastStatusAtByAccount:     make(map[string]time.Time),
		accountPins:               nil,
		statusEdits:               nil,
		scheduledStatuses:         make(map[string]*domain.ScheduledStatus),
		pollsByID:                 make(map[string]*domain.Poll),
		pollsByStatusID:           make(map[string]*domain.Poll),
		pollOptions:               make(map[string][]domain.PollOption),
		pollVotes:                 nil,
		quoteApprovalsByQuoting:   make(map[string]*quoteApprovalEntry),
		domainBlocksByDomain:      make(map[string]*domain.DomainBlock),
		favouritesByID:            make(map[string]*domain.Favourite),
		favouritesByAPID:          make(map[string]*domain.Favourite),
		favouritesByAccountStatus: make(map[string]*domain.Favourite),
		conversationMutesByKey:    make(map[string]struct{}),
		conversationMutesList:     nil,
		conversationIDs:           make(map[string]struct{}),
		statusConversationIDs:     make(map[string]string),
		accountConversations:      nil,
		announcementsByID:         make(map[string]*domain.Announcement),
		announcementReads:         make(map[string]struct{}),
		announcementReactions:     nil,
	}
}

// Ensure FakeStore implements store.Store.
var _ store.Store = (*FakeStore)(nil)

// SeedUserAndAccount inserts a user and account directly into the FakeStore for test setup.
func (f *FakeStore) SeedUserAndAccount(u *domain.User, a *domain.Account) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	uCopy := *u
	aCopy := *a
	f.accountsByID[a.ID] = &aCopy
	f.accountsByUsername[a.Username] = &aCopy
	f.usersByAccountID[a.ID] = &uCopy
	return nil
}

const noCursorSentinel = "ZZZZZZZZZZZZZZZZZZZZZZZZZZ"

type blockEntry struct {
	ID        string
	AccountID string
	TargetID  string
}

type muteEntry struct {
	ID        string
	AccountID string
	TargetID  string
}

func (f *FakeStore) WithTx(ctx context.Context, fn func(store.Store) error) error {
	return fn(f)
}

func (f *FakeStore) CreateAccount(ctx context.Context, in store.CreateAccountInput) (*domain.Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := in.Username
	if in.Domain != nil {
		key = in.Username + "@" + *in.Domain
	}
	if _, exists := f.accountsByUsername[key]; exists {
		return nil, domain.ErrConflict
	}
	now := time.Now()
	acc := &domain.Account{
		ID:           in.ID,
		Username:     in.Username,
		Domain:       in.Domain,
		DisplayName:  in.DisplayName,
		Note:         in.Note,
		PublicKey:    in.PublicKey,
		PrivateKey:   in.PrivateKey,
		InboxURL:     in.InboxURL,
		OutboxURL:    in.OutboxURL,
		FollowersURL: in.FollowersURL,
		FollowingURL: in.FollowingURL,
		APID:         in.APID,
		APRaw:        in.ApRaw,
		Bot:          in.Bot,
		Locked:       in.Locked,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	f.accountsByID[acc.ID] = acc
	f.accountsByUsername[key] = acc
	return acc, nil
}

func (f *FakeStore) GetAccountByID(ctx context.Context, id string) (*domain.Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.accountsByID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	out := *a
	if _, suspended := f.suspendedAccountIDs[id]; suspended {
		out.Suspended = true
	}
	return &out, nil
}

func (f *FakeStore) GetAccountsByIDs(ctx context.Context, ids []string) ([]*domain.Account, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*domain.Account, 0, len(ids))
	for _, id := range ids {
		a, ok := f.accountsByID[id]
		if !ok {
			continue
		}
		acc := *a
		if _, suspended := f.suspendedAccountIDs[id]; suspended {
			acc.Suspended = true
		}
		out = append(out, &acc)
	}
	return out, nil
}

func (f *FakeStore) GetAccountByAPID(ctx context.Context, apID string) (*domain.Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, a := range f.accountsByID {
		if a.APID == apID {
			return a, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *FakeStore) CountLocalAccounts(ctx context.Context) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var n int64
	for _, a := range f.accountsByID {
		if a.Domain == nil {
			n++
		}
	}
	return n, nil
}

func (f *FakeStore) GetLocalAccountByUsername(ctx context.Context, username string) (*domain.Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.accountsByUsername[username]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return a, nil
}

func (f *FakeStore) GetRemoteAccountByUsername(ctx context.Context, username string, accountDomain *string) (*domain.Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := username
	if accountDomain != nil {
		key = username + "@" + *accountDomain
	}
	a, ok := f.accountsByUsername[key]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return a, nil
}

func (f *FakeStore) SearchAccounts(ctx context.Context, query string, limit int) ([]*domain.Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	query = strings.ToLower(query)
	var local, remote []*domain.Account
	for _, a := range f.accountsByID {
		if _, suspended := f.suspendedAccountIDs[a.ID]; suspended {
			continue
		}
		var acct string
		if a.Domain != nil && *a.Domain != "" {
			acct = strings.ToLower(a.Username + "@" + *a.Domain)
		} else {
			acct = strings.ToLower(a.Username)
		}
		if strings.HasPrefix(acct, query) || strings.Contains(acct, query) {
			if a.Domain == nil || *a.Domain == "" {
				local = append(local, a)
			} else {
				remote = append(remote, a)
			}
		}
	}
	out := make([]*domain.Account, 0, limit)
	for _, a := range local {
		if len(out) >= limit {
			break
		}
		out = append(out, a)
	}
	for _, a := range remote {
		if len(out) >= limit {
			break
		}
		out = append(out, a)
	}
	return out, nil
}

func (f *FakeStore) CreateUser(ctx context.Context, in store.CreateUserInput) (*domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := time.Now()
	u := &domain.User{
		ID:                 in.ID,
		AccountID:          in.AccountID,
		Email:              in.Email,
		PasswordHash:       in.PasswordHash,
		Role:               in.Role,
		DefaultQuotePolicy: "public",
		CreatedAt:          now,
	}
	f.usersByAccountID[in.AccountID] = u
	return u, nil
}

func (f *FakeStore) CreateStatus(ctx context.Context, in store.CreateStatusInput) (*domain.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := time.Now()
	var apRaw json.RawMessage
	if len(in.ApRaw) > 0 {
		apRaw = in.ApRaw
	}
	policy := in.QuoteApprovalPolicy
	if policy == "" {
		policy = "public"
	}
	s := &domain.Status{
		ID:                  in.ID,
		URI:                 in.URI,
		AccountID:           in.AccountID,
		Text:                in.Text,
		Content:             in.Content,
		ContentWarning:      in.ContentWarning,
		Visibility:          in.Visibility,
		Language:            in.Language,
		InReplyToID:         in.InReplyToID,
		ReblogOfID:          in.ReblogOfID,
		QuotedStatusID:      in.QuotedStatusID,
		QuoteApprovalPolicy: policy,
		QuotesCount:         0,
		APID:                in.APID,
		APRaw:               apRaw,
		Sensitive:           in.Sensitive,
		Local:               in.Local,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	f.statusesByID[s.ID] = s
	if f.homeTimeline[in.AccountID] == nil {
		f.homeTimeline[in.AccountID] = []*domain.Status{}
	}
	f.homeTimeline[in.AccountID] = append([]*domain.Status{s}, f.homeTimeline[in.AccountID]...)
	if in.Visibility == domain.VisibilityPublic {
		f.publicTimeline = append([]*domain.Status{s}, f.publicTimeline...)
	}
	return s, nil
}

func (f *FakeStore) GetStatusByID(ctx context.Context, id string) (*domain.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.statusesByID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	if s.DeletedAt != nil {
		return nil, domain.ErrNotFound
	}
	return s, nil
}

func (f *FakeStore) GetStatusByAPID(ctx context.Context, apID string) (*domain.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.statusesByID {
		if s.DeletedAt != nil {
			continue
		}
		if s.APID == apID {
			return s, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *FakeStore) GetAccountStatuses(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var list []*domain.Status
	for _, s := range f.statusesByID {
		if s.DeletedAt != nil || s.AccountID != accountID || s.ReblogOfID != nil {
			continue
		}
		list = append(list, s)
	}
	for i := 0; i < len(list)-1; i++ {
		for j := i + 1; j < len(list); j++ {
			if list[j].ID > list[i].ID {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	cursor := noCursorSentinel
	if maxID != nil && *maxID != "" {
		cursor = *maxID
	}
	out := make([]domain.Status, 0, limit)
	for _, s := range list {
		if cursor != noCursorSentinel && s.ID >= cursor {
			continue
		}
		if len(out) >= limit {
			break
		}
		out = append(out, *s)
	}
	return out, nil
}

func (f *FakeStore) GetAccountPublicStatuses(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var list []*domain.Status
	for _, s := range f.statusesByID {
		if s.DeletedAt != nil || s.AccountID != accountID || s.ReblogOfID != nil || s.Visibility != domain.VisibilityPublic {
			continue
		}
		list = append(list, s)
	}
	for i := 0; i < len(list)-1; i++ {
		for j := i + 1; j < len(list); j++ {
			if list[j].ID > list[i].ID {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	cursor := noCursorSentinel
	if maxID != nil && *maxID != "" {
		cursor = *maxID
	}
	out := make([]domain.Status, 0, limit)
	for _, s := range list {
		if cursor != noCursorSentinel && s.ID >= cursor {
			continue
		}
		if len(out) >= limit {
			break
		}
		out = append(out, *s)
	}
	return out, nil
}

func (f *FakeStore) CountLocalStatuses(ctx context.Context) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var n int64
	for _, s := range f.statusesByID {
		if s.Local && s.DeletedAt == nil {
			n++
		}
	}
	return n, nil
}

func (f *FakeStore) CountAccountPublicStatuses(ctx context.Context, accountID string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var n int64
	for _, s := range f.statusesByID {
		if s.AccountID == accountID && s.DeletedAt == nil && s.Visibility == domain.VisibilityPublic && s.ReblogOfID == nil {
			n++
		}
	}
	return n, nil
}

func (f *FakeStore) SoftDeleteStatus(ctx context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.statusesByID[id]
	if !ok {
		return domain.ErrNotFound
	}
	now := time.Now()
	s.DeletedAt = &now
	newList := make([]*domain.Status, 0, len(f.homeTimeline[s.AccountID]))
	for _, st := range f.homeTimeline[s.AccountID] {
		if st.ID != id {
			newList = append(newList, st)
		}
	}
	f.homeTimeline[s.AccountID] = newList
	newPublic := make([]*domain.Status, 0, len(f.publicTimeline))
	for _, st := range f.publicTimeline {
		if st.ID != id {
			newPublic = append(newPublic, st)
		}
	}
	f.publicTimeline = newPublic
	return nil
}

func (f *FakeStore) IncrementStatusesCount(ctx context.Context, accountID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statusesCount[accountID]++
	return nil
}

func (f *FakeStore) DecrementStatusesCount(ctx context.Context, accountID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statusesCount[accountID]--
	return nil
}

func (f *FakeStore) GetHomeTimeline(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	list := f.homeTimeline[accountID]
	if list == nil {
		return nil, nil
	}
	cursor := noCursorSentinel
	if maxID != nil && *maxID != "" {
		cursor = *maxID
	}
	out := make([]domain.Status, 0, limit)
	for i := 0; i < len(list) && len(out) < limit; i++ {
		s := list[i]
		if s.DeletedAt != nil {
			continue
		}
		if cursor != noCursorSentinel && s.ID >= cursor {
			continue
		}
		out = append(out, *s)
	}
	return out, nil
}

func (f *FakeStore) GetFavouritesTimeline(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, *string, error) {
	return nil, nil, nil
}

func (f *FakeStore) GetPublicTimeline(ctx context.Context, localOnly bool, maxID *string, limit int) ([]domain.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cursor := noCursorSentinel
	if maxID != nil && *maxID != "" {
		cursor = *maxID
	}
	out := make([]domain.Status, 0, limit)
	for i := 0; i < len(f.publicTimeline) && len(out) < limit; i++ {
		s := f.publicTimeline[i]
		if s.DeletedAt != nil {
			continue
		}
		if localOnly && !s.Local {
			continue
		}
		if cursor != noCursorSentinel && s.ID >= cursor {
			continue
		}
		out = append(out, *s)
	}
	return out, nil
}

func (f *FakeStore) GetHashtagTimeline(ctx context.Context, tagName string, maxID *string, limit int) ([]domain.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// FakeStore does not track status-hashtag joins; return empty.
	return nil, nil
}

func (f *FakeStore) GetStatusAncestors(ctx context.Context, statusID string) ([]domain.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// FakeStore does not walk reply chains; return empty.
	return nil, nil
}

func (f *FakeStore) GetStatusDescendants(ctx context.Context, statusID string) ([]domain.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// FakeStore does not walk reply chains; return empty.
	return nil, nil
}

func (f *FakeStore) GetStatusFavouritedBy(ctx context.Context, statusID string, maxID *string, limit int) ([]domain.Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// FakeStore does not track favourites; return empty.
	return nil, nil
}

func (f *FakeStore) GetRebloggedBy(ctx context.Context, statusID string, maxID *string, limit int) ([]domain.Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// FakeStore: collect accounts that have a reblog status for this ID.
	out := make([]domain.Account, 0)
	for _, s := range f.statusesByID {
		if s.DeletedAt != nil {
			continue
		}
		if s.ReblogOfID != nil && *s.ReblogOfID == statusID {
			acc := f.accountsByID[s.AccountID]
			if acc != nil {
				out = append(out, *acc)
			}
		}
	}
	return out, nil
}

func (f *FakeStore) CreateApplication(ctx context.Context, in store.CreateApplicationInput) (*domain.OAuthApplication, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	app := &domain.OAuthApplication{
		ID:           in.ID,
		Name:         in.Name,
		ClientID:     in.ClientID,
		ClientSecret: in.ClientSecret,
		RedirectURIs: in.RedirectURIs,
		Scopes:       in.Scopes,
		Website:      in.Website,
		CreatedAt:    time.Now(),
	}
	f.applications[in.ClientID] = app
	f.applicationsByID[in.ID] = app
	return app, nil
}

func (f *FakeStore) GetApplicationByClientID(ctx context.Context, clientID string) (*domain.OAuthApplication, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if app, ok := f.applications[clientID]; ok {
		return app, nil
	}
	return nil, domain.ErrNotFound
}

func (f *FakeStore) CreateAuthorizationCode(ctx context.Context, in store.CreateAuthorizationCodeInput) (*domain.OAuthAuthorizationCode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ac := &domain.OAuthAuthorizationCode{
		ID:                  in.ID,
		Code:                in.Code,
		ApplicationID:       in.ApplicationID,
		AccountID:           in.AccountID,
		RedirectURI:         in.RedirectURI,
		Scopes:              in.Scopes,
		CodeChallenge:       in.CodeChallenge,
		CodeChallengeMethod: in.CodeChallengeMethod,
		ExpiresAt:           in.ExpiresAt,
		CreatedAt:           time.Now(),
	}
	f.authCodes[in.Code] = ac
	return ac, nil
}

func (f *FakeStore) GetAuthorizationCode(ctx context.Context, code string) (*domain.OAuthAuthorizationCode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if ac, ok := f.authCodes[code]; ok && ac.ExpiresAt.After(time.Now()) {
		return ac, nil
	}
	return nil, domain.ErrNotFound
}

func (f *FakeStore) DeleteAuthorizationCode(ctx context.Context, code string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.authCodes, code)
	return nil
}

func (f *FakeStore) CreateAccessToken(ctx context.Context, in store.CreateAccessTokenInput) (*domain.OAuthAccessToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	tok := &domain.OAuthAccessToken{
		ID:            in.ID,
		ApplicationID: in.ApplicationID,
		AccountID:     in.AccountID,
		Token:         in.Token,
		Scopes:        in.Scopes,
		ExpiresAt:     in.ExpiresAt,
		CreatedAt:     time.Now(),
	}
	f.tokens[in.Token] = tok
	return tok, nil
}

func (f *FakeStore) GetAccessToken(ctx context.Context, token string) (*domain.OAuthAccessToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if tok, ok := f.tokens[token]; ok && tok.RevokedAt == nil {
		return tok, nil
	}
	return nil, domain.ErrNotFound
}

func (f *FakeStore) RevokeAccessToken(ctx context.Context, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if tok, ok := f.tokens[token]; ok {
		now := time.Now()
		tok.RevokedAt = &now
	}
	return nil
}
func (f *FakeStore) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.usersByAccountID {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (f *FakeStore) GetUserByAccountID(ctx context.Context, accountID string) (*domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.usersByAccountID[accountID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return u, nil
}

func (f *FakeStore) ConfirmUser(ctx context.Context, userID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.usersByAccountID {
		if u.ID == userID {
			now := time.Now()
			u.ConfirmedAt = &now
			return nil
		}
	}
	return domain.ErrNotFound
}

func (f *FakeStore) CreateStatusMention(ctx context.Context, statusID, accountID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mentionsByStatusID[statusID] = append(f.mentionsByStatusID[statusID], accountID)
	return nil
}

func (f *FakeStore) GetStatusMentions(ctx context.Context, statusID string) ([]*domain.Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ids := f.mentionsByStatusID[statusID]
	out := make([]*domain.Account, 0, len(ids))
	for _, id := range ids {
		if a := f.accountsByID[id]; a != nil {
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *FakeStore) GetStatusMentionAccountIDs(ctx context.Context, statusID string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ids := f.mentionsByStatusID[statusID]
	if len(ids) == 0 {
		return nil, nil
	}
	out := make([]string, len(ids))
	copy(out, ids)
	return out, nil
}

func (f *FakeStore) GetOrCreateHashtag(ctx context.Context, name string) (*domain.Hashtag, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := strings.ToLower(name)
	if h, ok := f.hashtagsByName[key]; ok {
		return h, nil
	}
	h := &domain.Hashtag{ID: "tag-" + key, Name: key, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	f.hashtagsByName[key] = h
	f.hashtagsByID[h.ID] = h
	return h, nil
}
func (f *FakeStore) AttachHashtagsToStatus(ctx context.Context, statusID string, hashtagIDs []string) error {
	return nil
}

func (f *FakeStore) DeleteStatusMentions(ctx context.Context, statusID string) error {
	return nil
}

func (f *FakeStore) DeleteStatusHashtags(ctx context.Context, statusID string) error {
	return nil
}

func (f *FakeStore) GetStatusHashtags(ctx context.Context, statusID string) ([]domain.Hashtag, error) {
	return nil, nil
}
func (f *FakeStore) SearchHashtagsByPrefix(ctx context.Context, prefix string, limit int) ([]domain.Hashtag, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	prefix = strings.ToLower(prefix)
	var out []domain.Hashtag
	for _, h := range f.hashtagsByName {
		if strings.HasPrefix(h.Name, prefix) {
			out = append(out, *h)
		}
	}
	for i := 0; i < len(out)-1; i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Name < out[i].Name {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *FakeStore) FollowTag(ctx context.Context, id, accountID, tagID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range f.followedTagsList {
		if e.AccountID == accountID && e.TagID == tagID {
			return nil
		}
	}
	f.followedTagsList = append(f.followedTagsList, followedTagEntry{ID: id, AccountID: accountID, TagID: tagID})
	return nil
}

func (f *FakeStore) UnfollowTag(ctx context.Context, accountID, tagID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, e := range f.followedTagsList {
		if e.AccountID == accountID && e.TagID == tagID {
			f.followedTagsList = append(f.followedTagsList[:i], f.followedTagsList[i+1:]...)
			break
		}
	}
	return nil
}

func (f *FakeStore) ListFollowedTags(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Hashtag, *string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var entries []followedTagEntry
	for _, e := range f.followedTagsList {
		if e.AccountID == accountID {
			if maxID != nil && *maxID != "" && e.ID >= *maxID {
				continue
			}
			entries = append(entries, e)
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID > entries[j].ID })
	if limit <= 0 {
		limit = 40
	}
	var nextCursor *string
	if len(entries) > limit {
		nextCursor = &entries[limit-1].ID
		entries = entries[:limit]
	}
	out := make([]domain.Hashtag, 0, len(entries))
	for _, e := range entries {
		if h, ok := f.hashtagsByID[e.TagID]; ok && h != nil {
			out = append(out, *h)
		}
	}
	return out, nextCursor, nil
}

func (f *FakeStore) CreateFeaturedTag(ctx context.Context, id, accountID, tagID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range f.featuredTagsList {
		if e.AccountID == accountID && e.TagID == tagID {
			return nil
		}
	}
	f.featuredTagsList = append(f.featuredTagsList, featuredTagEntry{ID: id, AccountID: accountID, TagID: tagID})
	return nil
}

func (f *FakeStore) DeleteFeaturedTag(ctx context.Context, id, accountID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, e := range f.featuredTagsList {
		if e.ID == id && e.AccountID == accountID {
			f.featuredTagsList = append(f.featuredTagsList[:i], f.featuredTagsList[i+1:]...)
			break
		}
	}
	return nil
}

func (f *FakeStore) ListFeaturedTags(ctx context.Context, accountID string) ([]domain.FeaturedTag, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]domain.FeaturedTag, 0)
	for _, e := range f.featuredTagsList {
		if e.AccountID != accountID {
			continue
		}
		h, ok := f.hashtagsByID[e.TagID]
		if !ok || h == nil {
			continue
		}
		out = append(out, domain.FeaturedTag{
			ID:            e.ID,
			AccountID:     e.AccountID,
			TagID:         e.TagID,
			Name:          h.Name,
			StatusesCount: 0,
			LastStatusAt:  nil,
			CreatedAt:     time.Now(),
		})
	}
	return out, nil
}

func (f *FakeStore) GetFeaturedTagByID(ctx context.Context, id, accountID string) (*domain.FeaturedTag, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range f.featuredTagsList {
		if e.ID == id && e.AccountID == accountID {
			name := ""
			if h, ok := f.hashtagsByID[e.TagID]; ok && h != nil {
				name = h.Name
			}
			return &domain.FeaturedTag{
				ID:            e.ID,
				AccountID:     e.AccountID,
				TagID:         e.TagID,
				Name:          name,
				StatusesCount: 0,
				LastStatusAt:  nil,
				CreatedAt:     time.Now(),
			}, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *FakeStore) ListFeaturedTagSuggestions(ctx context.Context, accountID string, limit int) ([]domain.Hashtag, []int64, error) {
	return nil, nil, nil
}

func convMuteKey(accountID, conversationID string) string {
	return accountID + ":" + conversationID
}

func (f *FakeStore) GetConversationRoot(ctx context.Context, statusID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	visited := make(map[string]struct{})
	currentID := statusID
	for {
		if _, ok := visited[currentID]; ok {
			return currentID, nil
		}
		visited[currentID] = struct{}{}
		st, ok := f.statusesByID[currentID]
		if !ok || st == nil {
			return "", domain.ErrNotFound
		}
		if st.InReplyToID == nil || *st.InReplyToID == "" {
			return currentID, nil
		}
		currentID = *st.InReplyToID
	}
}

func (f *FakeStore) CreateConversationMute(ctx context.Context, accountID, conversationID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := convMuteKey(accountID, conversationID)
	f.conversationMutesByKey[key] = struct{}{}
	f.conversationMutesList = append(f.conversationMutesList, conversationMuteEntry{AccountID: accountID, ConversationID: conversationID})
	return nil
}

func (f *FakeStore) DeleteConversationMute(ctx context.Context, accountID, conversationID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.conversationMutesByKey, convMuteKey(accountID, conversationID))
	for i, e := range f.conversationMutesList {
		if e.AccountID == accountID && e.ConversationID == conversationID {
			f.conversationMutesList = append(f.conversationMutesList[:i], f.conversationMutesList[i+1:]...)
			break
		}
	}
	return nil
}

func (f *FakeStore) IsConversationMuted(ctx context.Context, accountID, conversationID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.conversationMutesByKey[convMuteKey(accountID, conversationID)]
	return ok, nil
}

func (f *FakeStore) ListMutedConversationIDs(ctx context.Context, accountID string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, e := range f.conversationMutesList {
		if e.AccountID == accountID {
			out = append(out, e.ConversationID)
		}
	}
	return out, nil
}

func (f *FakeStore) CreateConversation(ctx context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.conversationIDs[id] = struct{}{}
	return nil
}

func (f *FakeStore) SetStatusConversationID(ctx context.Context, statusID, conversationID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statusConversationIDs[statusID] = conversationID
	return nil
}

func (f *FakeStore) GetStatusConversationID(ctx context.Context, statusID string) (*string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cid, ok := f.statusConversationIDs[statusID]
	if !ok {
		if _, hasStatus := f.statusesByID[statusID]; hasStatus {
			return nil, nil
		}
		return nil, domain.ErrNotFound
	}
	return &cid, nil
}

func (f *FakeStore) UpsertAccountConversation(ctx context.Context, in store.UpsertAccountConversationInput) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := time.Now()
	for i := range f.accountConversations {
		if f.accountConversations[i].AccountID == in.AccountID && f.accountConversations[i].ConversationID == in.ConversationID {
			f.accountConversations[i].LastStatusID = in.LastStatusID
			f.accountConversations[i].Unread = in.Unread
			f.accountConversations[i].UpdatedAt = now
			return nil
		}
	}
	f.accountConversations = append(f.accountConversations, accountConversationEntry{
		ID:             in.ID,
		AccountID:      in.AccountID,
		ConversationID: in.ConversationID,
		LastStatusID:   in.LastStatusID,
		Unread:         in.Unread,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	return nil
}

func (f *FakeStore) ListAccountConversations(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.AccountConversation, *string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var filtered []accountConversationEntry
	for _, e := range f.accountConversations {
		if e.AccountID == accountID {
			if maxID != nil && *maxID != "" && e.ID >= *maxID {
				continue
			}
			filtered = append(filtered, e)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		if !filtered[i].UpdatedAt.Equal(filtered[j].UpdatedAt) {
			return filtered[i].UpdatedAt.After(filtered[j].UpdatedAt)
		}
		return filtered[i].ID > filtered[j].ID
	})
	if limit <= 0 {
		limit = 40
	}
	var nextCursor *string
	page := filtered
	if len(filtered) > limit {
		page = filtered[:limit]
		nextCursor = &page[len(page)-1].ID
	}
	out := make([]domain.AccountConversation, 0, len(page))
	for _, e := range page {
		lastID := e.LastStatusID
		out = append(out, domain.AccountConversation{
			ID:             e.ID,
			AccountID:      e.AccountID,
			ConversationID: e.ConversationID,
			LastStatusID:   &lastID,
			Unread:         e.Unread,
			CreatedAt:      e.CreatedAt,
			UpdatedAt:      e.UpdatedAt,
		})
	}
	return out, nextCursor, nil
}

func (f *FakeStore) GetAccountConversation(ctx context.Context, accountID, conversationID string) (*domain.AccountConversation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range f.accountConversations {
		if e.AccountID == accountID && e.ConversationID == conversationID {
			lastID := e.LastStatusID
			return &domain.AccountConversation{
				ID:             e.ID,
				AccountID:      e.AccountID,
				ConversationID: e.ConversationID,
				LastStatusID:   &lastID,
				Unread:         e.Unread,
				CreatedAt:      e.CreatedAt,
				UpdatedAt:      e.UpdatedAt,
			}, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *FakeStore) MarkAccountConversationRead(ctx context.Context, accountID, conversationID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.accountConversations {
		if f.accountConversations[i].AccountID == accountID && f.accountConversations[i].ConversationID == conversationID {
			f.accountConversations[i].Unread = false
			f.accountConversations[i].UpdatedAt = time.Now()
			return nil
		}
	}
	return domain.ErrNotFound
}

func (f *FakeStore) DeleteAccountConversation(ctx context.Context, accountID, conversationID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, e := range f.accountConversations {
		if e.AccountID == accountID && e.ConversationID == conversationID {
			f.accountConversations = append(f.accountConversations[:i], f.accountConversations[i+1:]...)
			return nil
		}
	}
	return nil
}

func announcementReadKey(accountID, announcementID string) string {
	return accountID + ":" + announcementID
}

func (f *FakeStore) CreateAnnouncement(ctx context.Context, in store.CreateAnnouncementInput) (*domain.Announcement, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a := &domain.Announcement{
		ID:          in.ID,
		Content:     in.Content,
		StartsAt:    in.StartsAt,
		EndsAt:      in.EndsAt,
		AllDay:      in.AllDay,
		PublishedAt: in.PublishedAt,
		UpdatedAt:   in.PublishedAt,
	}
	f.announcementsByID[in.ID] = a
	return a, nil
}

func (f *FakeStore) UpdateAnnouncement(ctx context.Context, in store.UpdateAnnouncementInput) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.announcementsByID[in.ID]
	if !ok {
		return domain.ErrNotFound
	}
	a.Content = in.Content
	a.StartsAt = in.StartsAt
	a.EndsAt = in.EndsAt
	a.AllDay = in.AllDay
	a.PublishedAt = in.PublishedAt
	a.UpdatedAt = time.Now()
	return nil
}

func (f *FakeStore) GetAnnouncementByID(ctx context.Context, id string) (*domain.Announcement, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.announcementsByID[id]
	if !ok || a == nil {
		return nil, domain.ErrNotFound
	}
	cp := *a
	return &cp, nil
}

func isAnnouncementActive(a *domain.Announcement, now time.Time) bool {
	if a.PublishedAt.After(now) {
		return false
	}
	if a.StartsAt != nil && a.StartsAt.After(now) {
		return false
	}
	if a.EndsAt != nil && !a.EndsAt.After(now) {
		return false
	}
	return true
}

func (f *FakeStore) ListActiveAnnouncements(ctx context.Context) ([]domain.Announcement, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.Announcement
	for _, a := range f.announcementsByID {
		if a != nil && isAnnouncementActive(a, time.Now()) {
			out = append(out, *a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PublishedAt.After(out[j].PublishedAt) })
	return out, nil
}

func (f *FakeStore) ListAllAnnouncements(ctx context.Context) ([]domain.Announcement, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.Announcement
	for _, a := range f.announcementsByID {
		if a != nil {
			out = append(out, *a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PublishedAt.After(out[j].PublishedAt) })
	return out, nil
}

func (f *FakeStore) DismissAnnouncement(ctx context.Context, accountID, announcementID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.announcementReads[announcementReadKey(accountID, announcementID)] = struct{}{}
	return nil
}

func (f *FakeStore) ListReadAnnouncementIDs(ctx context.Context, accountID string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for k := range f.announcementReads {
		parts := strings.SplitN(k, ":", 2)
		if len(parts) == 2 && parts[0] == accountID {
			out = append(out, parts[1])
		}
	}
	return out, nil
}

func (f *FakeStore) AddAnnouncementReaction(ctx context.Context, announcementID, accountID, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.announcementReactions {
		if r.AnnouncementID == announcementID && r.AccountID == accountID && r.Name == name {
			return nil
		}
	}
	f.announcementReactions = append(f.announcementReactions, announcementReactionEntry{
		AnnouncementID: announcementID,
		AccountID:      accountID,
		Name:           name,
	})
	return nil
}

func (f *FakeStore) RemoveAnnouncementReaction(ctx context.Context, announcementID, accountID, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, r := range f.announcementReactions {
		if r.AnnouncementID == announcementID && r.AccountID == accountID && r.Name == name {
			f.announcementReactions = append(f.announcementReactions[:i], f.announcementReactions[i+1:]...)
			break
		}
	}
	return nil
}

func (f *FakeStore) ListAnnouncementReactionCounts(ctx context.Context, announcementID string) ([]domain.AnnouncementReactionCount, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	counts := make(map[string]int)
	for _, r := range f.announcementReactions {
		if r.AnnouncementID == announcementID {
			counts[r.Name]++
		}
	}
	out := make([]domain.AnnouncementReactionCount, 0, len(counts))
	for name, count := range counts {
		out = append(out, domain.AnnouncementReactionCount{Name: name, Count: count, Me: false})
	}
	return out, nil
}

func (f *FakeStore) ListAccountAnnouncementReactionNames(ctx context.Context, announcementID, accountID string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, r := range f.announcementReactions {
		if r.AnnouncementID == announcementID && r.AccountID == accountID {
			out = append(out, r.Name)
		}
	}
	return out, nil
}

func (f *FakeStore) CreateNotification(ctx context.Context, in store.CreateNotificationInput) (*domain.Notification, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := &domain.Notification{
		ID:        in.ID,
		AccountID: in.AccountID,
		FromID:    in.FromID,
		Type:      in.Type,
		StatusID:  in.StatusID,
		CreatedAt: time.Now(),
	}
	f.notificationsByAccount[in.AccountID] = append(f.notificationsByAccount[in.AccountID], n)
	return n, nil
}
func (f *FakeStore) ListNotifications(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Notification, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	list := f.notificationsByAccount[accountID]
	if list == nil {
		return nil, nil
	}
	cursor := noCursorSentinel
	if maxID != nil && *maxID != "" {
		cursor = *maxID
	}
	out := make([]domain.Notification, 0, limit)
	for i := len(list) - 1; i >= 0 && len(out) < limit; i-- {
		n := list[i]
		if cursor != noCursorSentinel && n.ID >= cursor {
			continue
		}
		out = append(out, *n)
	}
	return out, nil
}
func (f *FakeStore) GetNotification(ctx context.Context, id, accountID string) (*domain.Notification, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, n := range f.notificationsByAccount[accountID] {
		if n != nil && n.ID == id {
			return n, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (f *FakeStore) ClearNotifications(ctx context.Context, accountID string) error {
	return nil
}
func (f *FakeStore) DismissNotification(ctx context.Context, id, accountID string) error {
	return nil
}
func (f *FakeStore) GetStatusAttachments(ctx context.Context, statusID string) ([]domain.MediaAttachment, error) {
	return nil, nil
}

func (f *FakeStore) GetMonsteraSettings(ctx context.Context) (*domain.MonsteraSettings, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.monsteraSettings != nil {
		return f.monsteraSettings, nil
	}
	return &domain.MonsteraSettings{RegistrationMode: domain.MonsteraRegistrationModeOpen}, nil
}

func (f *FakeStore) UpdateMonsteraSettings(ctx context.Context, in *domain.MonsteraSettings) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.monsteraSettings = &domain.MonsteraSettings{
		RegistrationMode:    in.RegistrationMode,
		InviteMaxUses:       in.InviteMaxUses,
		InviteExpiresInDays: in.InviteExpiresInDays,
		ServerName:          in.ServerName,
		ServerDescription:   in.ServerDescription,
		ServerRules:         append([]string(nil), in.ServerRules...),
	}
	return nil
}

func (f *FakeStore) GetMediaAttachment(ctx context.Context, id string) (*domain.MediaAttachment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.mediaByID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return a, nil
}

func (f *FakeStore) CountFollowers(ctx context.Context, accountID string) (int64, error) {
	return 0, nil
}

func (f *FakeStore) CountFollowing(ctx context.Context, accountID string) (int64, error) {
	return 0, nil
}
func (f *FakeStore) IncrementFollowersCount(ctx context.Context, accountID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if a, ok := f.accountsByID[accountID]; ok {
		a.FollowersCount++
	}
	return nil
}
func (f *FakeStore) DecrementFollowersCount(ctx context.Context, accountID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if a, ok := f.accountsByID[accountID]; ok && a.FollowersCount > 0 {
		a.FollowersCount--
	}
	return nil
}
func (f *FakeStore) IncrementFollowingCount(ctx context.Context, accountID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if a, ok := f.accountsByID[accountID]; ok {
		a.FollowingCount++
	}
	return nil
}
func (f *FakeStore) DecrementFollowingCount(ctx context.Context, accountID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if a, ok := f.accountsByID[accountID]; ok && a.FollowingCount > 0 {
		a.FollowingCount--
	}
	return nil
}

func followKey(accountID, targetID string) string { return accountID + ":" + targetID }

func (f *FakeStore) GetBlock(ctx context.Context, accountID, targetID string) (*domain.Block, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.blocksByKey[followKey(accountID, targetID)]; !ok {
		return nil, domain.ErrNotFound
	}
	for _, e := range f.blocksList {
		if e.AccountID == accountID && e.TargetID == targetID {
			return &domain.Block{ID: e.ID, AccountID: accountID, TargetID: targetID}, nil
		}
	}
	return &domain.Block{AccountID: accountID, TargetID: targetID}, nil
}

func (f *FakeStore) GetMute(ctx context.Context, accountID, targetID string) (*domain.Mute, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.mutesByKey[followKey(accountID, targetID)]; !ok {
		return nil, domain.ErrNotFound
	}
	for _, e := range f.mutesList {
		if e.AccountID == accountID && e.TargetID == targetID {
			return &domain.Mute{ID: e.ID, AccountID: accountID, TargetID: targetID, HideNotifications: true}, nil
		}
	}
	return &domain.Mute{AccountID: accountID, TargetID: targetID, HideNotifications: true}, nil
}

func (f *FakeStore) ListDomainBlocks(ctx context.Context) ([]domain.DomainBlock, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]domain.DomainBlock, 0, len(f.domainBlocksByDomain))
	for _, b := range f.domainBlocksByDomain {
		if b != nil {
			out = append(out, *b)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Domain < out[j].Domain })
	return out, nil
}

func (f *FakeStore) GetFollow(ctx context.Context, accountID, targetID string) (*domain.Follow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	follow, ok := f.followsByKey[followKey(accountID, targetID)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return follow, nil
}
func (f *FakeStore) GetFollowByID(ctx context.Context, id string) (*domain.Follow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, follow := range f.followsByKey {
		if follow.ID == id {
			return follow, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (f *FakeStore) GetFollowByAPID(ctx context.Context, apID string) (*domain.Follow, error) {
	return nil, domain.ErrNotFound
}
func (f *FakeStore) CreateFollow(ctx context.Context, in store.CreateFollowInput) (*domain.Follow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	follow := &domain.Follow{
		ID:        in.ID,
		AccountID: in.AccountID,
		TargetID:  in.TargetID,
		State:     in.State,
		APID:      in.APID,
		CreatedAt: time.Now(),
	}
	f.followsByKey[followKey(in.AccountID, in.TargetID)] = follow
	return follow, nil
}
func (f *FakeStore) AcceptFollow(ctx context.Context, followID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, follow := range f.followsByKey {
		if follow.ID == followID {
			follow.State = domain.FollowStateAccepted
			return nil
		}
	}
	return domain.ErrNotFound
}
func (f *FakeStore) DeleteFollow(ctx context.Context, accountID, targetID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.followsByKey, followKey(accountID, targetID))
	return nil
}
func (f *FakeStore) GetFollowers(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, error) {
	return nil, nil
}
func (f *FakeStore) GetFollowing(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, error) {
	return nil, nil
}

func (f *FakeStore) GetPendingFollowRequests(ctx context.Context, targetID string, maxID *string, limit int) ([]domain.Account, *string, error) {
	return nil, nil, nil
}

func (f *FakeStore) CreateBookmark(ctx context.Context, in store.CreateBookmarkInput) error {
	return nil
}

func (f *FakeStore) DeleteBookmark(ctx context.Context, accountID, statusID string) error {
	return nil
}

func (f *FakeStore) GetBookmarks(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, *string, error) {
	return nil, nil, nil
}

func (f *FakeStore) IsBookmarked(ctx context.Context, accountID, statusID string) (bool, error) {
	return false, nil
}

func (f *FakeStore) CreateAccountPin(ctx context.Context, accountID, statusID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.accountPins {
		if p.accountID == accountID && p.statusID == statusID {
			return nil
		}
	}
	f.accountPins = append(f.accountPins, pinEntry{accountID: accountID, statusID: statusID, createdAt: time.Now()})
	return nil
}

func (f *FakeStore) DeleteAccountPin(ctx context.Context, accountID, statusID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	var newPins []pinEntry
	for _, p := range f.accountPins {
		if p.accountID != accountID || p.statusID != statusID {
			newPins = append(newPins, p)
		}
	}
	f.accountPins = newPins
	return nil
}

func (f *FakeStore) ListPinnedStatusIDs(ctx context.Context, accountID string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var ids []string
	for _, p := range f.accountPins {
		if p.accountID == accountID {
			ids = append(ids, p.statusID)
		}
	}
	return ids, nil
}

func (f *FakeStore) CountAccountPins(ctx context.Context, accountID string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var n int64
	for _, p := range f.accountPins {
		if p.accountID == accountID {
			n++
		}
	}
	return n, nil
}

func copyList(l *domain.List) *domain.List {
	if l == nil {
		return nil
	}
	return &domain.List{
		ID:            l.ID,
		AccountID:     l.AccountID,
		Title:         l.Title,
		RepliesPolicy: l.RepliesPolicy,
		Exclusive:     l.Exclusive,
		CreatedAt:     l.CreatedAt,
	}
}

func (f *FakeStore) CreateList(ctx context.Context, in store.CreateListInput) (*domain.List, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	l := &domain.List{
		ID:            in.ID,
		AccountID:     in.AccountID,
		Title:         in.Title,
		RepliesPolicy: in.RepliesPolicy,
		Exclusive:     in.Exclusive,
		CreatedAt:     time.Now().UTC(),
	}
	f.listsByID[in.ID] = l
	f.listAccountIDs[in.ID] = nil
	return copyList(l), nil
}

func (f *FakeStore) GetListByID(ctx context.Context, id string) (*domain.List, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	l, ok := f.listsByID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return copyList(l), nil
}

func (f *FakeStore) ListLists(ctx context.Context, accountID string) ([]domain.List, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.List
	for _, l := range f.listsByID {
		if l.AccountID == accountID {
			out = append(out, *copyList(l))
		}
	}
	return out, nil
}

func (f *FakeStore) UpdateList(ctx context.Context, in store.UpdateListInput) (*domain.List, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	l, ok := f.listsByID[in.ID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	l.Title = in.Title
	l.RepliesPolicy = in.RepliesPolicy
	l.Exclusive = in.Exclusive
	return copyList(l), nil
}

func (f *FakeStore) DeleteList(ctx context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.listsByID, id)
	delete(f.listAccountIDs, id)
	return nil
}

func (f *FakeStore) ListListAccountIDs(ctx context.Context, listID string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.listsByID[listID]; !ok {
		return nil, domain.ErrNotFound
	}
	ids := f.listAccountIDs[listID]
	if ids == nil {
		return nil, nil
	}
	out := make([]string, len(ids))
	copy(out, ids)
	return out, nil
}

func (f *FakeStore) AddAccountToList(ctx context.Context, listID, accountID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.listsByID[listID]; !ok {
		return domain.ErrNotFound
	}
	ids := f.listAccountIDs[listID]
	for _, id := range ids {
		if id == accountID {
			return nil
		}
	}
	f.listAccountIDs[listID] = append(ids, accountID)
	return nil
}

func (f *FakeStore) RemoveAccountFromList(ctx context.Context, listID, accountID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	ids := f.listAccountIDs[listID]
	if ids == nil {
		return nil
	}
	var newIDs []string
	for _, id := range ids {
		if id != accountID {
			newIDs = append(newIDs, id)
		}
	}
	f.listAccountIDs[listID] = newIDs
	return nil
}

func (f *FakeStore) GetListTimeline(ctx context.Context, listID string, maxID *string, limit int) ([]domain.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	memberIDs := f.listAccountIDs[listID]
	if len(memberIDs) == 0 {
		return nil, nil
	}
	memberSet := make(map[string]struct{}, len(memberIDs))
	for _, id := range memberIDs {
		memberSet[id] = struct{}{}
	}
	var list []*domain.Status
	for _, s := range f.statusesByID {
		if s.DeletedAt != nil || s.ReblogOfID != nil {
			continue
		}
		if _, ok := memberSet[s.AccountID]; ok {
			list = append(list, s)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[j].ID < list[i].ID })
	cursor := noCursorSentinel
	if maxID != nil && *maxID != "" {
		cursor = *maxID
	}
	if limit <= 0 {
		limit = 20
	}
	out := make([]domain.Status, 0, limit)
	for _, s := range list {
		if cursor != noCursorSentinel && s.ID >= cursor {
			continue
		}
		if len(out) >= limit {
			break
		}
		out = append(out, *s)
	}
	return out, nil
}

func copyUserFilter(uf *domain.UserFilter) *domain.UserFilter {
	if uf == nil {
		return nil
	}
	ctxCopy := make([]string, len(uf.Context))
	copy(ctxCopy, uf.Context)
	var exp *time.Time
	if uf.ExpiresAt != nil {
		t := *uf.ExpiresAt
		exp = &t
	}
	return &domain.UserFilter{
		ID:           uf.ID,
		AccountID:    uf.AccountID,
		Phrase:       uf.Phrase,
		Context:      ctxCopy,
		WholeWord:    uf.WholeWord,
		ExpiresAt:    exp,
		Irreversible: uf.Irreversible,
		CreatedAt:    uf.CreatedAt,
	}
}

func (f *FakeStore) CreateUserFilter(ctx context.Context, in store.CreateUserFilterInput) (*domain.UserFilter, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	uf := &domain.UserFilter{
		ID:           in.ID,
		AccountID:    in.AccountID,
		Phrase:       in.Phrase,
		Context:      append([]string(nil), in.Context...),
		WholeWord:    in.WholeWord,
		ExpiresAt:    in.ExpiresAt,
		Irreversible: in.Irreversible,
		CreatedAt:    time.Now().UTC(),
	}
	f.userFiltersByID[in.ID] = uf
	return copyUserFilter(uf), nil
}

func (f *FakeStore) GetUserFilterByID(ctx context.Context, id string) (*domain.UserFilter, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	uf, ok := f.userFiltersByID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return copyUserFilter(uf), nil
}

func (f *FakeStore) ListUserFilters(ctx context.Context, accountID string) ([]domain.UserFilter, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.UserFilter
	for _, uf := range f.userFiltersByID {
		if uf.AccountID == accountID {
			out = append(out, *copyUserFilter(uf))
		}
	}
	return out, nil
}

func (f *FakeStore) UpdateUserFilter(ctx context.Context, in store.UpdateUserFilterInput) (*domain.UserFilter, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	uf, ok := f.userFiltersByID[in.ID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	uf.Phrase = in.Phrase
	uf.Context = append([]string(nil), in.Context...)
	uf.WholeWord = in.WholeWord
	uf.ExpiresAt = in.ExpiresAt
	uf.Irreversible = in.Irreversible
	return copyUserFilter(uf), nil
}

func (f *FakeStore) DeleteUserFilter(ctx context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.userFiltersByID, id)
	return nil
}

func (f *FakeStore) GetActiveUserFiltersByContext(ctx context.Context, accountID, filterContext string) ([]domain.UserFilter, error) {
	return nil, nil
}

func (f *FakeStore) GetMarkers(ctx context.Context, accountID string, timelines []string) (map[string]domain.Marker, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[string]domain.Marker, len(timelines))
	for _, t := range timelines {
		k := followKey(accountID, t)
		if m := f.markersByKey[k]; m != nil {
			out[t] = *m
		}
	}
	return out, nil
}

func (f *FakeStore) SetMarker(ctx context.Context, accountID, timeline, lastReadID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	k := followKey(accountID, timeline)
	m := f.markersByKey[k]
	if m == nil {
		f.markersByKey[k] = &domain.Marker{LastReadID: lastReadID, Version: 0, UpdatedAt: time.Now()}
		return nil
	}
	m.LastReadID = lastReadID
	m.Version++
	m.UpdatedAt = time.Now()
	return nil
}

func (f *FakeStore) UpdateAccountLastStatusAt(ctx context.Context, accountID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastStatusAtByAccount[accountID] = time.Now()
	return nil
}

func (f *FakeStore) ListDirectoryAccounts(ctx context.Context, order string, localOnly bool, offset, limit int) ([]domain.Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var candidates []*domain.Account
	for _, a := range f.accountsByID {
		if a.Suspended {
			continue
		}
		if localOnly && a.Domain != nil && *a.Domain != "" {
			continue
		}
		candidates = append(candidates, a)
	}
	if order == "active" {
		sort.Slice(candidates, func(i, j int) bool {
			a, b := candidates[i], candidates[j]
			ta := f.lastStatusAtByAccount[a.ID]
			tb := f.lastStatusAtByAccount[b.ID]
			if ta.IsZero() && tb.IsZero() {
				return a.CreatedAt.Before(b.CreatedAt)
			}
			if ta.IsZero() {
				return false
			}
			if tb.IsZero() {
				return true
			}
			return ta.After(tb)
		})
	} else {
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].CreatedAt.After(candidates[j].CreatedAt)
		})
	}
	start := offset
	if start > len(candidates) {
		start = len(candidates)
	}
	end := start + limit
	if end > len(candidates) {
		end = len(candidates)
	}
	slice := candidates[start:end]
	out := make([]domain.Account, 0, len(slice))
	for _, a := range slice {
		ac := *a
		out = append(out, ac)
	}
	return out, nil
}

func (f *FakeStore) SuspendAccount(ctx context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.suspendedAccountIDs[id] = struct{}{}
	return nil
}
func (f *FakeStore) CreateBlock(ctx context.Context, in store.CreateBlockInput) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.blocksByKey[followKey(in.AccountID, in.TargetID)] = struct{}{}
	f.blocksList = append(f.blocksList, blockEntry{ID: in.ID, AccountID: in.AccountID, TargetID: in.TargetID})
	return nil
}
func (f *FakeStore) DeleteBlock(ctx context.Context, accountID, targetID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.blocksByKey, followKey(accountID, targetID))
	for i, b := range f.blocksList {
		if b.AccountID == accountID && b.TargetID == targetID {
			f.blocksList = append(f.blocksList[:i], f.blocksList[i+1:]...)
			break
		}
	}
	return nil
}

// ListBlockedAccounts returns blocked accounts (cursor = block id; next page: id < max_id). Matches postgres semantics.
func (f *FakeStore) ListBlockedAccounts(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, *string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var entries []blockEntry
	for _, b := range f.blocksList {
		if b.AccountID == accountID {
			if maxID != nil && *maxID != "" && b.ID >= *maxID {
				continue
			}
			entries = append(entries, b)
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID > entries[j].ID })
	if limit <= 0 {
		limit = 40
	}
	var nextCursor *string
	if len(entries) > limit {
		nextCursor = &entries[limit-1].ID
		entries = entries[:limit]
	}
	out := make([]domain.Account, 0, len(entries))
	for _, e := range entries {
		acc, ok := f.accountsByID[e.TargetID]
		if ok && acc != nil {
			out = append(out, *acc)
		}
	}
	return out, nextCursor, nil
}

func (f *FakeStore) IsBlockedEitherDirection(ctx context.Context, accountID, targetID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, aBlocksB := f.blocksByKey[followKey(accountID, targetID)]
	_, bBlocksA := f.blocksByKey[followKey(targetID, accountID)]
	return aBlocksB || bBlocksA, nil
}
func (f *FakeStore) CreateMute(ctx context.Context, in store.CreateMuteInput) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mutesByKey[followKey(in.AccountID, in.TargetID)] = struct{}{}
	f.mutesList = append(f.mutesList, muteEntry{ID: in.ID, AccountID: in.AccountID, TargetID: in.TargetID})
	return nil
}
func (f *FakeStore) DeleteMute(ctx context.Context, accountID, targetID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.mutesByKey, followKey(accountID, targetID))
	for i, m := range f.mutesList {
		if m.AccountID == accountID && m.TargetID == targetID {
			f.mutesList = append(f.mutesList[:i], f.mutesList[i+1:]...)
			break
		}
	}
	return nil
}

// ListMutedAccounts returns muted accounts (cursor = mute id; next page: id < max_id). Matches postgres semantics.
func (f *FakeStore) ListMutedAccounts(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, *string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var entries []muteEntry
	for _, m := range f.mutesList {
		if m.AccountID == accountID {
			if maxID != nil && *maxID != "" && m.ID >= *maxID {
				continue
			}
			entries = append(entries, m)
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID > entries[j].ID })
	if limit <= 0 {
		limit = 40
	}
	var nextCursor *string
	if len(entries) > limit {
		nextCursor = &entries[limit-1].ID
		entries = entries[:limit]
	}
	out := make([]domain.Account, 0, len(entries))
	for _, e := range entries {
		acc, ok := f.accountsByID[e.TargetID]
		if ok && acc != nil {
			out = append(out, *acc)
		}
	}
	return out, nextCursor, nil
}
func favouriteAccountStatusKey(accountID, statusID string) string { return accountID + ":" + statusID }

func (f *FakeStore) CreateFavourite(ctx context.Context, in store.CreateFavouriteInput) (*domain.Favourite, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fav := &domain.Favourite{
		ID:        in.ID,
		AccountID: in.AccountID,
		StatusID:  in.StatusID,
		CreatedAt: time.Now(),
	}
	if in.APID != nil {
		fav.APID = *in.APID
	}
	f.favouritesByID[fav.ID] = fav
	if fav.APID != "" {
		f.favouritesByAPID[fav.APID] = fav
	}
	f.favouritesByAccountStatus[favouriteAccountStatusKey(fav.AccountID, fav.StatusID)] = fav
	return fav, nil
}
func (f *FakeStore) DeleteFavourite(ctx context.Context, accountID, statusID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := favouriteAccountStatusKey(accountID, statusID)
	if fav, ok := f.favouritesByAccountStatus[key]; ok {
		delete(f.favouritesByID, fav.ID)
		if fav.APID != "" {
			delete(f.favouritesByAPID, fav.APID)
		}
		delete(f.favouritesByAccountStatus, key)
	}
	return nil
}
func (f *FakeStore) GetFavouriteByAPID(ctx context.Context, apID string) (*domain.Favourite, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if fav, ok := f.favouritesByAPID[apID]; ok {
		copy := *fav
		return &copy, nil
	}
	return nil, domain.ErrNotFound
}
func (f *FakeStore) GetFavouriteByAccountAndStatus(ctx context.Context, accountID, statusID string) (*domain.Favourite, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := favouriteAccountStatusKey(accountID, statusID)
	if fav, ok := f.favouritesByAccountStatus[key]; ok {
		copy := *fav
		return &copy, nil
	}
	return nil, domain.ErrNotFound
}
func (f *FakeStore) IncrementFavouritesCount(ctx context.Context, statusID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s, ok := f.statusesByID[statusID]; ok {
		s.FavouritesCount++
	}
	return nil
}
func (f *FakeStore) DecrementFavouritesCount(ctx context.Context, statusID string) error {
	return nil
}
func (f *FakeStore) IncrementReblogsCount(ctx context.Context, statusID string) error {
	return nil
}
func (f *FakeStore) DecrementReblogsCount(ctx context.Context, statusID string) error {
	return nil
}
func (f *FakeStore) IncrementQuotesCount(ctx context.Context, quotedStatusID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s, ok := f.statusesByID[quotedStatusID]; ok {
		s.QuotesCount++
	}
	return nil
}
func (f *FakeStore) DecrementQuotesCount(ctx context.Context, quotedStatusID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s, ok := f.statusesByID[quotedStatusID]; ok && s.QuotesCount > 0 {
		s.QuotesCount--
	}
	return nil
}
func (f *FakeStore) CreateQuoteApproval(ctx context.Context, quotingStatusID, quotedStatusID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.quoteApprovalsByQuoting[quotingStatusID] = &quoteApprovalEntry{quotedStatusID: quotedStatusID}
	return nil
}
func (f *FakeStore) RevokeQuote(ctx context.Context, quotedStatusID, quotingStatusID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.quoteApprovalsByQuoting[quotingStatusID]
	if !ok || e.quotedStatusID != quotedStatusID {
		return domain.ErrNotFound
	}
	now := time.Now()
	e.revokedAt = &now
	return nil
}
func (f *FakeStore) ListQuotesOfStatus(ctx context.Context, quotedStatusID string, maxID *string, limit int) ([]domain.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.Status
	for quotingID, e := range f.quoteApprovalsByQuoting {
		if e.quotedStatusID != quotedStatusID || e.revokedAt != nil {
			continue
		}
		s, ok := f.statusesByID[quotingID]
		if !ok || s.DeletedAt != nil {
			continue
		}
		if maxID != nil && *maxID != "" && s.ID >= *maxID {
			continue
		}
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
func (f *FakeStore) GetQuoteApproval(ctx context.Context, quotingStatusID string) (*domain.QuoteApprovalRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.quoteApprovalsByQuoting[quotingStatusID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &domain.QuoteApprovalRecord{
		QuotingStatusID: quotingStatusID,
		QuotedStatusID:  e.quotedStatusID,
		RevokedAt:       e.revokedAt,
	}, nil
}
func (f *FakeStore) UpdateStatusQuoteApprovalPolicy(ctx context.Context, statusID, policy string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s, ok := f.statusesByID[statusID]; ok {
		s.QuoteApprovalPolicy = policy
		return nil
	}
	return domain.ErrNotFound
}
func (f *FakeStore) IncrementRepliesCount(ctx context.Context, statusID string) error {
	return nil
}
func (f *FakeStore) GetReblogByAccountAndTarget(ctx context.Context, accountID, statusID string) (*domain.Status, error) {
	return nil, domain.ErrNotFound
}
func (f *FakeStore) UpdateAccount(ctx context.Context, in store.UpdateAccountInput) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	acc := f.accountsByID[in.ID]
	if acc == nil {
		return domain.ErrNotFound
	}
	if in.DisplayName != nil {
		acc.DisplayName = in.DisplayName
	}
	if in.Note != nil {
		acc.Note = in.Note
	}
	if in.AvatarMediaID != nil {
		acc.AvatarMediaID = in.AvatarMediaID
	}
	if in.HeaderMediaID != nil {
		acc.HeaderMediaID = in.HeaderMediaID
	}
	if len(in.Fields) > 0 {
		acc.Fields = in.Fields
	}
	acc.Bot = in.Bot
	acc.Locked = in.Locked
	return nil
}
func (f *FakeStore) UpdateAccountKeys(ctx context.Context, id, publicKey string, apRaw []byte) error {
	return nil
}
func (f *FakeStore) AttachMediaToStatus(ctx context.Context, mediaID, statusID, accountID string) error {
	return nil
}
func (f *FakeStore) CreateMediaAttachment(ctx context.Context, in store.CreateMediaAttachmentInput) (*domain.MediaAttachment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	att := &domain.MediaAttachment{
		ID:          in.ID,
		AccountID:   in.AccountID,
		Type:        in.Type,
		StorageKey:  in.StorageKey,
		URL:         in.URL,
		PreviewURL:  in.PreviewURL,
		RemoteURL:   in.RemoteURL,
		Description: in.Description,
		Blurhash:    in.Blurhash,
		Meta:        json.RawMessage(in.Meta),
		CreatedAt:   time.Now(),
	}
	f.mediaByID[in.ID] = att
	return att, nil
}

func (f *FakeStore) UpdateMediaAttachment(ctx context.Context, in store.UpdateMediaAttachmentInput) (*domain.MediaAttachment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	att, ok := f.mediaByID[in.ID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	if att.AccountID != in.AccountID {
		return nil, domain.ErrNotFound
	}
	if in.Description != nil {
		att.Description = in.Description
	}
	if in.Meta != nil {
		att.Meta = json.RawMessage(in.Meta)
	}
	return att, nil
}

func (f *FakeStore) CreateStatusEdit(ctx context.Context, in store.CreateStatusEditInput) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statusEdits = append(f.statusEdits, domain.StatusEdit{
		ID:             in.ID,
		StatusID:       in.StatusID,
		AccountID:      in.AccountID,
		Text:           in.Text,
		Content:        in.Content,
		ContentWarning: in.ContentWarning,
		Sensitive:      in.Sensitive,
		CreatedAt:      time.Now(),
	})
	return nil
}

func (f *FakeStore) ListStatusEdits(ctx context.Context, statusID string) ([]domain.StatusEdit, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.StatusEdit
	for _, e := range f.statusEdits {
		if e.StatusID == statusID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *FakeStore) UpdateStatus(ctx context.Context, in store.UpdateStatusInput) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	st := f.statusesByID[in.ID]
	if st == nil {
		return domain.ErrNotFound
	}
	if in.Text != nil {
		st.Text = in.Text
	}
	if in.Content != nil {
		st.Content = in.Content
	}
	if in.ContentWarning != nil {
		st.ContentWarning = in.ContentWarning
	}
	st.Sensitive = in.Sensitive
	now := time.Now()
	st.EditedAt = &now
	st.UpdatedAt = now
	return nil
}

func (f *FakeStore) CreateScheduledStatus(ctx context.Context, in store.CreateScheduledStatusInput) (*domain.ScheduledStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := &domain.ScheduledStatus{
		ID:          in.ID,
		AccountID:   in.AccountID,
		Params:      in.Params,
		ScheduledAt: in.ScheduledAt,
		CreatedAt:   time.Now(),
	}
	f.scheduledStatuses[in.ID] = s
	return s, nil
}

func (f *FakeStore) GetScheduledStatusByID(ctx context.Context, id string) (*domain.ScheduledStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := f.scheduledStatuses[id]
	if s == nil {
		return nil, domain.ErrNotFound
	}
	cp := *s
	return &cp, nil
}

func (f *FakeStore) ListScheduledStatuses(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.ScheduledStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var list []*domain.ScheduledStatus
	for _, s := range f.scheduledStatuses {
		if s.AccountID != accountID {
			continue
		}
		list = append(list, s)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID > list[j].ID })
	var out []domain.ScheduledStatus
	for _, s := range list {
		if maxID != nil && *maxID != "" && s.ID >= *maxID {
			continue
		}
		out = append(out, *s)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (f *FakeStore) UpdateScheduledStatus(ctx context.Context, in store.UpdateScheduledStatusInput) (*domain.ScheduledStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := f.scheduledStatuses[in.ID]
	if s == nil {
		return nil, domain.ErrNotFound
	}
	s.Params = in.Params
	s.ScheduledAt = in.ScheduledAt
	cp := *s
	return &cp, nil
}

func (f *FakeStore) DeleteScheduledStatus(ctx context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.scheduledStatuses[id] == nil {
		return domain.ErrNotFound
	}
	delete(f.scheduledStatuses, id)
	return nil
}

func (f *FakeStore) ListScheduledStatusesDue(ctx context.Context, limit int) ([]domain.ScheduledStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := time.Now()
	var list []*domain.ScheduledStatus
	for _, s := range f.scheduledStatuses {
		if !s.ScheduledAt.After(now) {
			list = append(list, s)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ScheduledAt.Before(list[j].ScheduledAt) })
	var out []domain.ScheduledStatus
	for i := 0; i < limit && i < len(list); i++ {
		out = append(out, *list[i])
	}
	return out, nil
}

func (f *FakeStore) GetFollowerInboxURLs(ctx context.Context, accountID string) ([]string, error) {
	return f.getDistinctFollowerInboxURLsPaginated(ctx, accountID, "", 0)
}

func (f *FakeStore) GetDistinctFollowerInboxURLsPaginated(ctx context.Context, accountID string, cursor string, limit int) ([]string, error) {
	return f.getDistinctFollowerInboxURLsPaginated(ctx, accountID, cursor, limit)
}

func (f *FakeStore) GetLocalFollowerAccountIDs(ctx context.Context, targetID string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var ids []string
	seen := make(map[string]bool)
	for key, follow := range f.followsByKey {
		if follow.TargetID != targetID || follow.State != domain.FollowStateAccepted {
			continue
		}
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			continue
		}
		followerID := parts[0]
		if seen[followerID] {
			continue
		}
		acc, ok := f.accountsByID[followerID]
		if !ok || acc.Domain != nil && *acc.Domain != "" {
			continue
		}
		if _, suspended := f.suspendedAccountIDs[followerID]; suspended {
			continue
		}
		seen[followerID] = true
		ids = append(ids, followerID)
	}
	return ids, nil
}

func (f *FakeStore) getDistinctFollowerInboxURLsPaginated(_ context.Context, accountID string, cursor string, limit int) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var inboxURLs []string
	seen := make(map[string]bool)
	for key, follow := range f.followsByKey {
		if follow.TargetID != accountID || follow.State != domain.FollowStateAccepted {
			continue
		}
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			continue
		}
		followerID := parts[0]
		acc, ok := f.accountsByID[followerID]
		if !ok || acc.InboxURL == "" || acc.Domain == nil {
			continue
		}
		if _, suspended := f.suspendedAccountIDs[followerID]; suspended {
			continue
		}
		if !seen[acc.InboxURL] {
			seen[acc.InboxURL] = true
			inboxURLs = append(inboxURLs, acc.InboxURL)
		}
	}
	// Sort for stable pagination
	sort.Strings(inboxURLs)
	var result []string
	for _, u := range inboxURLs {
		if u > cursor {
			result = append(result, u)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (f *FakeStore) CreateReport(ctx context.Context, in store.CreateReportInput) (*domain.Report, error) {
	return &domain.Report{
		ID:        in.ID,
		AccountID: in.AccountID,
		TargetID:  in.TargetID,
		StatusIDs: in.StatusIDs,
		Comment:   in.Comment,
		Category:  in.Category,
		State:     domain.ReportStateOpen,
		CreatedAt: time.Now(),
	}, nil
}
func (f *FakeStore) GetReportByID(ctx context.Context, id string) (*domain.Report, error) {
	_ = id
	return nil, domain.ErrNotFound
}
func (f *FakeStore) ListReports(ctx context.Context, state string, limit, offset int) ([]domain.Report, error) {
	return nil, nil
}
func (f *FakeStore) AssignReport(ctx context.Context, reportID string, assigneeID *string) error {
	return nil
}
func (f *FakeStore) ResolveReport(ctx context.Context, reportID string, actionTaken *string) error {
	return nil
}
func (f *FakeStore) CreateDomainBlock(ctx context.Context, in store.CreateDomainBlockInput) (*domain.DomainBlock, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := strings.ToLower(strings.TrimSpace(in.Domain))
	b := &domain.DomainBlock{ID: in.ID, Domain: in.Domain, Severity: in.Severity, Reason: in.Reason, CreatedAt: time.Now()}
	f.domainBlocksByDomain[key] = b
	return b, nil
}
func (f *FakeStore) GetDomainBlock(ctx context.Context, domainName string) (*domain.DomainBlock, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := strings.ToLower(strings.TrimSpace(domainName))
	if b, ok := f.domainBlocksByDomain[key]; ok {
		copy := *b
		return &copy, nil
	}
	return nil, domain.ErrNotFound
}
func (f *FakeStore) UpdateDomainBlock(ctx context.Context, domainName string, severity string, reason *string) (*domain.DomainBlock, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := strings.ToLower(strings.TrimSpace(domainName))
	if b, ok := f.domainBlocksByDomain[key]; ok {
		b.Severity = severity
		b.Reason = reason
		copy := *b
		return &copy, nil
	}
	return nil, domain.ErrNotFound
}
func (f *FakeStore) DeleteDomainBlock(ctx context.Context, domain string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := strings.ToLower(strings.TrimSpace(domain))
	delete(f.domainBlocksByDomain, key)
	return nil
}
func (f *FakeStore) CreateAdminAction(ctx context.Context, in store.CreateAdminActionInput) error {
	return nil
}
func (f *FakeStore) ListAdminActionsByTarget(ctx context.Context, targetAccountID string) ([]domain.AdminAction, error) {
	return nil, nil
}
func (f *FakeStore) CreateInvite(ctx context.Context, in store.CreateInviteInput) (*domain.Invite, error) {
	return &domain.Invite{ID: in.ID, Code: in.Code, CreatedBy: in.CreatedBy, MaxUses: in.MaxUses, Uses: 0, ExpiresAt: in.ExpiresAt, CreatedAt: time.Now()}, nil
}
func (f *FakeStore) GetInviteByCode(ctx context.Context, code string) (*domain.Invite, error) {
	_ = code
	return nil, domain.ErrNotFound
}
func (f *FakeStore) ListInvitesByCreator(ctx context.Context, createdByUserID string) ([]domain.Invite, error) {
	return nil, nil
}
func (f *FakeStore) DeleteInvite(ctx context.Context, id string) error {
	return nil
}
func (f *FakeStore) IncrementInviteUses(ctx context.Context, code string) error {
	return nil
}
func (f *FakeStore) UpsertKnownInstance(ctx context.Context, id, domain string) error {
	return nil
}
func (f *FakeStore) ListKnownInstances(ctx context.Context, limit, offset int) ([]domain.KnownInstance, error) {
	return nil, nil
}
func (f *FakeStore) CountKnownInstances(ctx context.Context) (int64, error) {
	return 0, nil
}
func (f *FakeStore) CreateServerFilter(ctx context.Context, in store.CreateServerFilterInput) (*domain.ServerFilter, error) {
	return &domain.ServerFilter{ID: in.ID, Phrase: in.Phrase, Scope: in.Scope, Action: in.Action, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}
func (f *FakeStore) GetServerFilter(ctx context.Context, id string) (*domain.ServerFilter, error) {
	_ = id
	return nil, domain.ErrNotFound
}
func (f *FakeStore) ListServerFilters(ctx context.Context) ([]domain.ServerFilter, error) {
	return nil, nil
}
func (f *FakeStore) UpdateServerFilter(ctx context.Context, in store.UpdateServerFilterInput) (*domain.ServerFilter, error) {
	_ = in
	return nil, domain.ErrNotFound
}
func (f *FakeStore) DeleteServerFilter(ctx context.Context, id string) error {
	return nil
}
func (f *FakeStore) ListLocalUsers(ctx context.Context, limit, offset int) ([]domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.User
	for _, u := range f.usersByAccountID {
		acc, ok := f.accountsByID[u.AccountID]
		if !ok || acc.Domain != nil && *acc.Domain != "" {
			continue
		}
		out = append(out, *u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if offset > len(out) {
		return nil, nil
	}
	out = out[offset:]
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
func (f *FakeStore) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.usersByAccountID {
		if u.ID == id {
			u2 := *u
			return &u2, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (f *FakeStore) UpdateUserRole(ctx context.Context, userID string, role string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.usersByAccountID {
		if u.ID == userID {
			u.Role = role
			return nil
		}
	}
	return domain.ErrNotFound
}
func (f *FakeStore) UpdateUserDefaultQuotePolicy(ctx context.Context, accountID, policy string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u, ok := f.usersByAccountID[accountID]; ok {
		u.DefaultQuotePolicy = policy
		return nil
	}
	return domain.ErrNotFound
}
func (f *FakeStore) UpdateUserPreferences(ctx context.Context, in store.UpdateUserPreferencesInput) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.usersByAccountID {
		if u.ID == in.UserID {
			u.DefaultPrivacy = in.DefaultPrivacy
			u.DefaultSensitive = in.DefaultSensitive
			u.DefaultLanguage = in.DefaultLanguage
			u.DefaultQuotePolicy = in.DefaultQuotePolicy
			return nil
		}
	}
	return domain.ErrNotFound
}
func (f *FakeStore) UpdateUserEmail(ctx context.Context, userID, email string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.usersByAccountID {
		if u.ID == userID {
			u.Email = email
			return nil
		}
	}
	return domain.ErrNotFound
}
func (f *FakeStore) UpdateUserPassword(ctx context.Context, userID, passwordHash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.usersByAccountID {
		if u.ID == userID {
			u.PasswordHash = passwordHash
			return nil
		}
	}
	return domain.ErrNotFound
}
func (f *FakeStore) GetPendingRegistrations(ctx context.Context) ([]domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.User
	for _, u := range f.usersByAccountID {
		if u.ConfirmedAt != nil {
			continue
		}
		acc, ok := f.accountsByID[u.AccountID]
		if !ok || acc.Domain != nil && *acc.Domain != "" {
			continue
		}
		out = append(out, *u)
	}
	return out, nil
}
func (f *FakeStore) DeleteUser(ctx context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for accountID, u := range f.usersByAccountID {
		if u.ID == id {
			delete(f.usersByAccountID, accountID)
			return nil
		}
	}
	return nil
}
func (f *FakeStore) SilenceAccount(ctx context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if acc, ok := f.accountsByID[id]; ok {
		acc.Silenced = true
	}
	return nil
}
func (f *FakeStore) UnsuspendAccount(ctx context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.suspendedAccountIDs, id)
	if acc, ok := f.accountsByID[id]; ok {
		acc.Suspended = false
	}
	return nil
}
func (f *FakeStore) UnsilenceAccount(ctx context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if acc, ok := f.accountsByID[id]; ok {
		acc.Silenced = false
	}
	return nil
}
func (f *FakeStore) DeleteAccount(ctx context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.accountsByID, id)
	delete(f.suspendedAccountIDs, id)
	return nil
}
func (f *FakeStore) ListLocalAccounts(ctx context.Context, limit, offset int) ([]domain.Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.Account
	for _, acc := range f.accountsByID {
		if acc.Domain != nil && *acc.Domain != "" {
			continue
		}
		out = append(out, *acc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	if offset > len(out) {
		return nil, nil
	}
	out = out[offset:]
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
func (f *FakeStore) DeleteFollowsByDomain(ctx context.Context, domain string) error {
	return nil
}

func (f *FakeStore) CreatePoll(ctx context.Context, in store.CreatePollInput) (*domain.Poll, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := &domain.Poll{
		ID:        in.ID,
		StatusID:  in.StatusID,
		ExpiresAt: in.ExpiresAt,
		Multiple:  in.Multiple,
		CreatedAt: time.Now(),
	}
	f.pollsByID[p.ID] = p
	f.pollsByStatusID[p.StatusID] = p
	f.pollOptions[p.ID] = nil
	return p, nil
}

func (f *FakeStore) CreatePollOption(ctx context.Context, in store.CreatePollOptionInput) (*domain.PollOption, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	opt := domain.PollOption{ID: in.ID, PollID: in.PollID, Title: in.Title, Position: in.Position}
	opts := f.pollOptions[in.PollID]
	if opts == nil {
		opts = []domain.PollOption{}
	}
	f.pollOptions[in.PollID] = append(opts, opt)
	return &opt, nil
}

func (f *FakeStore) GetPollByID(ctx context.Context, id string) (*domain.Poll, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := f.pollsByID[id]
	if p == nil {
		return nil, domain.ErrNotFound
	}
	return p, nil
}

func (f *FakeStore) GetPollByStatusID(ctx context.Context, statusID string) (*domain.Poll, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := f.pollsByStatusID[statusID]
	if p == nil {
		return nil, domain.ErrNotFound
	}
	return p, nil
}

func (f *FakeStore) ListPollOptions(ctx context.Context, pollID string) ([]domain.PollOption, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	opts := f.pollOptions[pollID]
	if opts == nil {
		return nil, nil
	}
	cp := append([]domain.PollOption{}, opts...)
	sort.Slice(cp, func(i, j int) bool {
		if cp[i].Position != cp[j].Position {
			return cp[i].Position < cp[j].Position
		}
		return cp[i].ID < cp[j].ID
	})
	return cp, nil
}

func (f *FakeStore) DeletePollVotesByAccount(ctx context.Context, pollID, accountID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	var keep []pollVoteEntry
	for _, v := range f.pollVotes {
		if v.pollID != pollID || v.accountID != accountID {
			keep = append(keep, v)
		}
	}
	f.pollVotes = keep
	return nil
}

func (f *FakeStore) CreatePollVote(ctx context.Context, id, pollID, accountID, optionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pollVotes = append(f.pollVotes, pollVoteEntry{pollID: pollID, accountID: accountID, optionID: optionID})
	return nil
}

func (f *FakeStore) GetVoteCountsByPoll(ctx context.Context, pollID string) (map[string]int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	counts := make(map[string]int)
	for _, v := range f.pollVotes {
		if v.pollID == pollID {
			counts[v.optionID]++
		}
	}
	return counts, nil
}

func (f *FakeStore) HasVotedOnPoll(ctx context.Context, pollID, accountID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, v := range f.pollVotes {
		if v.pollID == pollID && v.accountID == accountID {
			return true, nil
		}
	}
	return false, nil
}

func (f *FakeStore) GetOwnVoteOptionIDs(ctx context.Context, pollID, accountID string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var ids []string
	for _, v := range f.pollVotes {
		if v.pollID == pollID && v.accountID == accountID {
			ids = append(ids, v.optionID)
		}
	}
	return ids, nil
}
