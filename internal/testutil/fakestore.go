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
	mutesByKey             map[string]struct{}       // "accountID:targetID"
	suspendedAccountIDs    map[string]struct{}
	mediaByID              map[string]*domain.MediaAttachment
	notificationsByAccount map[string][]*domain.Notification
	hashtagsByName         map[string]*domain.Hashtag
	Settings               map[string]string // optional; GetSetting reads from here

	applications     map[string]*domain.OAuthApplication
	applicationsByID map[string]*domain.OAuthApplication
	authCodes        map[string]*domain.OAuthAuthorizationCode
	tokens           map[string]*domain.OAuthAccessToken
}

// NewFakeStore returns a new FakeStore for use in tests.
func NewFakeStore() *FakeStore {
	return &FakeStore{
		accountsByID:           make(map[string]*domain.Account),
		accountsByUsername:     make(map[string]*domain.Account),
		usersByAccountID:       make(map[string]*domain.User),
		statusesByID:           make(map[string]*domain.Status),
		statusesCount:          make(map[string]int),
		homeTimeline:           make(map[string][]*domain.Status),
		followsByKey:           make(map[string]*domain.Follow),
		blocksByKey:            make(map[string]struct{}),
		mutesByKey:             make(map[string]struct{}),
		suspendedAccountIDs:    make(map[string]struct{}),
		mediaByID:              make(map[string]*domain.MediaAttachment),
		notificationsByAccount: make(map[string][]*domain.Notification),
		hashtagsByName:         make(map[string]*domain.Hashtag),
		applications:           make(map[string]*domain.OAuthApplication),
		applicationsByID:       make(map[string]*domain.OAuthApplication),
		authCodes:              make(map[string]*domain.OAuthAuthorizationCode),
		tokens:                 make(map[string]*domain.OAuthAccessToken),
	}
}

// Ensure FakeStore implements store.Store.
var _ store.Store = (*FakeStore)(nil)

const noCursorSentinel = "ZZZZZZZZZZZZZZZZZZZZZZZZZZ"

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
		ID:           in.ID,
		AccountID:    in.AccountID,
		Email:        in.Email,
		PasswordHash: in.PasswordHash,
		Role:         in.Role,
		CreatedAt:    now,
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
	s := &domain.Status{
		ID:             in.ID,
		URI:            in.URI,
		AccountID:      in.AccountID,
		Text:           in.Text,
		Content:        in.Content,
		ContentWarning: in.ContentWarning,
		Visibility:     in.Visibility,
		Language:       in.Language,
		InReplyToID:    in.InReplyToID,
		ReblogOfID:     in.ReblogOfID,
		APID:           in.APID,
		APRaw:          apRaw,
		Sensitive:      in.Sensitive,
		Local:          in.Local,
		CreatedAt:      now,
		UpdatedAt:      now,
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

func (f *FakeStore) DeleteStatus(ctx context.Context, id string) error {
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
	return nil
}
func (f *FakeStore) GetStatusMentions(ctx context.Context, statusID string) ([]*domain.Account, error) {
	return nil, nil
}

func (f *FakeStore) GetStatusMentionAccountIDs(ctx context.Context, statusID string) ([]string, error) {
	return nil, nil
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
	return h, nil
}
func (f *FakeStore) AttachHashtagsToStatus(ctx context.Context, statusID string, hashtagIDs []string) error {
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

func (f *FakeStore) GetSetting(ctx context.Context, key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Settings != nil {
		if v, ok := f.Settings[key]; ok {
			return v, nil
		}
	}
	return "", nil
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

func (f *FakeStore) GetRelationship(ctx context.Context, accountID, targetID string) (*domain.Relationship, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rel := &domain.Relationship{TargetID: targetID, ShowingReblogs: true}
	if follow, ok := f.followsByKey[followKey(accountID, targetID)]; ok {
		rel.Following = true
		rel.Requested = follow.State == domain.FollowStatePending
	}
	if _, ok := f.followsByKey[followKey(targetID, accountID)]; ok {
		rel.FollowedBy = true
	}
	if _, ok := f.blocksByKey[followKey(accountID, targetID)]; ok {
		rel.Blocking = true
	}
	if _, ok := f.blocksByKey[followKey(targetID, accountID)]; ok {
		rel.BlockedBy = true
	}
	if _, ok := f.mutesByKey[followKey(accountID, targetID)]; ok {
		rel.Muting = true
		rel.MutingNotifications = true
	}
	return rel, nil
}
func (f *FakeStore) ListDomainBlocks(ctx context.Context) ([]domain.DomainBlock, error) {
	return nil, nil
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

func (f *FakeStore) CreateList(ctx context.Context, in store.CreateListInput) (*domain.List, error) {
	l := &domain.List{
		ID:            in.ID,
		AccountID:     in.AccountID,
		Title:         in.Title,
		RepliesPolicy: in.RepliesPolicy,
		Exclusive:     in.Exclusive,
		CreatedAt:     time.Now().UTC(),
	}
	return l, nil
}

func (f *FakeStore) GetListByID(ctx context.Context, id string) (*domain.List, error) {
	return nil, domain.ErrNotFound
}

func (f *FakeStore) ListLists(ctx context.Context, accountID string) ([]domain.List, error) {
	return nil, nil
}

func (f *FakeStore) UpdateList(ctx context.Context, in store.UpdateListInput) (*domain.List, error) {
	return nil, domain.ErrNotFound
}

func (f *FakeStore) DeleteList(ctx context.Context, id string) error {
	return nil
}

func (f *FakeStore) ListListAccountIDs(ctx context.Context, listID string) ([]string, error) {
	return nil, nil
}

func (f *FakeStore) AddAccountToList(ctx context.Context, listID, accountID string) error {
	return nil
}

func (f *FakeStore) RemoveAccountFromList(ctx context.Context, listID, accountID string) error {
	return nil
}

func (f *FakeStore) GetListTimeline(ctx context.Context, listID string, maxID *string, limit int) ([]domain.Status, error) {
	return nil, nil
}

func (f *FakeStore) CreateUserFilter(ctx context.Context, in store.CreateUserFilterInput) (*domain.UserFilter, error) {
	uf := &domain.UserFilter{
		ID:           in.ID,
		AccountID:    in.AccountID,
		Phrase:       in.Phrase,
		Context:      in.Context,
		WholeWord:    in.WholeWord,
		ExpiresAt:    in.ExpiresAt,
		Irreversible: in.Irreversible,
		CreatedAt:    time.Now().UTC(),
	}
	return uf, nil
}

func (f *FakeStore) GetUserFilterByID(ctx context.Context, id string) (*domain.UserFilter, error) {
	return nil, domain.ErrNotFound
}

func (f *FakeStore) ListUserFilters(ctx context.Context, accountID string) ([]domain.UserFilter, error) {
	return nil, nil
}

func (f *FakeStore) UpdateUserFilter(ctx context.Context, in store.UpdateUserFilterInput) (*domain.UserFilter, error) {
	return nil, domain.ErrNotFound
}

func (f *FakeStore) DeleteUserFilter(ctx context.Context, id string) error {
	return nil
}

func (f *FakeStore) GetActiveUserFiltersByContext(ctx context.Context, accountID, filterContext string) ([]domain.UserFilter, error) {
	return nil, nil
}

func (f *FakeStore) SoftDeleteStatus(ctx context.Context, id string) error {
	return f.DeleteStatus(ctx, id)
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
	return nil
}
func (f *FakeStore) DeleteBlock(ctx context.Context, accountID, targetID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.blocksByKey, followKey(accountID, targetID))
	return nil
}
func (f *FakeStore) CreateMute(ctx context.Context, in store.CreateMuteInput) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mutesByKey[followKey(in.AccountID, in.TargetID)] = struct{}{}
	return nil
}
func (f *FakeStore) DeleteMute(ctx context.Context, accountID, targetID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.mutesByKey, followKey(accountID, targetID))
	return nil
}
func (f *FakeStore) CreateFavourite(ctx context.Context, in store.CreateFavouriteInput) (*domain.Favourite, error) {
	return nil, domain.ErrNotFound
}
func (f *FakeStore) DeleteFavourite(ctx context.Context, accountID, statusID string) error {
	return nil
}
func (f *FakeStore) GetFavouriteByAPID(ctx context.Context, apID string) (*domain.Favourite, error) {
	return nil, domain.ErrNotFound
}
func (f *FakeStore) GetFavouriteByAccountAndStatus(ctx context.Context, accountID, statusID string) (*domain.Favourite, error) {
	return nil, domain.ErrNotFound
}
func (f *FakeStore) IncrementFavouritesCount(ctx context.Context, statusID string) error {
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
	return nil
}
func (f *FakeStore) UpdateStatus(ctx context.Context, in store.UpdateStatusInput) error {
	return nil
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
	return &domain.DomainBlock{ID: in.ID, Domain: in.Domain, Severity: in.Severity, Reason: in.Reason, CreatedAt: time.Now()}, nil
}
func (f *FakeStore) GetDomainBlock(ctx context.Context, domainName string) (*domain.DomainBlock, error) {
	_ = domainName
	return nil, domain.ErrNotFound
}
func (f *FakeStore) UpdateDomainBlock(ctx context.Context, domainName string, severity string, reason *string) (*domain.DomainBlock, error) {
	_ = domainName
	_ = severity
	_ = reason
	return nil, domain.ErrNotFound
}
func (f *FakeStore) DeleteDomainBlock(ctx context.Context, domain string) error {
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
func (f *FakeStore) SetSetting(ctx context.Context, key, value string) error {
	return nil
}
func (f *FakeStore) ListSettings(ctx context.Context) (map[string]string, error) {
	return nil, nil
}
func (f *FakeStore) UpsertKnownInstance(ctx context.Context, id, domain string) error {
	return nil
}
func (f *FakeStore) ListKnownInstances(ctx context.Context, limit, offset int) ([]domain.KnownInstance, error) {
	return nil, nil
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
