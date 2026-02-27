package testutil

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/store"
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
	suspendedAccountIDs    map[string]struct{}
	mediaByID              map[string]*domain.MediaAttachment
	notificationsByAccount map[string][]*domain.Notification
	hashtagsByName         map[string]*domain.Hashtag
	Settings               map[string]string // optional; GetSetting reads from here
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
		suspendedAccountIDs:    make(map[string]struct{}),
		mediaByID:              make(map[string]*domain.MediaAttachment),
		notificationsByAccount: make(map[string][]*domain.Notification),
		hashtagsByName:         make(map[string]*domain.Hashtag),
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

func (f *FakeStore) CreateApplication(ctx context.Context, in store.CreateApplicationInput) (*domain.OAuthApplication, error) {
	return nil, domain.ErrNotFound
}
func (f *FakeStore) GetApplicationByClientID(ctx context.Context, clientID string) (*domain.OAuthApplication, error) {
	return nil, domain.ErrNotFound
}
func (f *FakeStore) CreateAuthorizationCode(ctx context.Context, in store.CreateAuthorizationCodeInput) (*domain.OAuthAuthorizationCode, error) {
	return nil, domain.ErrNotFound
}
func (f *FakeStore) GetAuthorizationCode(ctx context.Context, code string) (*domain.OAuthAuthorizationCode, error) {
	return nil, domain.ErrNotFound
}
func (f *FakeStore) DeleteAuthorizationCode(ctx context.Context, code string) error {
	return nil
}
func (f *FakeStore) CreateAccessToken(ctx context.Context, in store.CreateAccessTokenInput) (*domain.OAuthAccessToken, error) {
	return nil, domain.ErrNotFound
}
func (f *FakeStore) GetAccessToken(ctx context.Context, token string) (*domain.OAuthAccessToken, error) {
	return nil, domain.ErrNotFound
}
func (f *FakeStore) RevokeAccessToken(ctx context.Context, token string) error {
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
	return nil
}
func (f *FakeStore) DecrementFollowersCount(ctx context.Context, accountID string) error {
	return nil
}
func (f *FakeStore) IncrementFollowingCount(ctx context.Context, accountID string) error {
	return nil
}
func (f *FakeStore) DecrementFollowingCount(ctx context.Context, accountID string) error {
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
	return nil
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
func (f *FakeStore) CreateStatusEdit(ctx context.Context, in store.CreateStatusEditInput) error {
	return nil
}
func (f *FakeStore) UpdateStatus(ctx context.Context, in store.UpdateStatusInput) error {
	return nil
}
func (f *FakeStore) GetFollowerInboxURLs(ctx context.Context, accountID string) ([]string, error) {
	return nil, nil
}
