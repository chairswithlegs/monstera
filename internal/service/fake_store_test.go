package service

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/store"
)

const noCursorSentinel = "ZZZZZZZZZZZZZZZZZZZZZZZZZZ"

// fakeStore implements store.Store for unit tests using domain types only.
type fakeStore struct {
	mu sync.Mutex

	accountsByID       map[string]*domain.Account
	accountsByUsername map[string]*domain.Account
	usersByAccountID   map[string]*domain.User
	statusesByID       map[string]*domain.Status
	statusesCount      map[string]int
	homeTimeline       map[string][]*domain.Status
	publicTimeline     []*domain.Status
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		accountsByID:       make(map[string]*domain.Account),
		accountsByUsername: make(map[string]*domain.Account),
		usersByAccountID:   make(map[string]*domain.User),
		statusesByID:       make(map[string]*domain.Status),
		statusesCount:      make(map[string]int),
		homeTimeline:       make(map[string][]*domain.Status),
	}
}

// WithTx calls fn with the same store instance. It does not provide transaction
// isolation or rollback — partial failures within fn leave state changes applied.
// This is an intentional simplification for unit testing happy paths.
func (f *fakeStore) WithTx(ctx context.Context, fn func(store.Store) error) error {
	return fn(f)
}

func (f *fakeStore) CreateAccount(ctx context.Context, in store.CreateAccountInput) (*domain.Account, error) {
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

func (f *fakeStore) GetAccountByID(ctx context.Context, id string) (*domain.Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.accountsByID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return a, nil
}

func (f *fakeStore) GetLocalAccountByUsername(ctx context.Context, username string) (*domain.Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.accountsByUsername[username]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return a, nil
}

func (f *fakeStore) GetRemoteAccountByUsername(ctx context.Context, username string, accountDomain *string) (*domain.Account, error) {
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

func (f *fakeStore) CreateUser(ctx context.Context, in store.CreateUserInput) (*domain.User, error) {
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

func (f *fakeStore) CreateStatus(ctx context.Context, in store.CreateStatusInput) (*domain.Status, error) {
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

func (f *fakeStore) GetStatusByID(ctx context.Context, id string) (*domain.Status, error) {
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

func (f *fakeStore) DeleteStatus(ctx context.Context, id string) error {
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

func (f *fakeStore) IncrementStatusesCount(ctx context.Context, accountID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statusesCount[accountID]++
	return nil
}

func (f *fakeStore) DecrementStatusesCount(ctx context.Context, accountID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statusesCount[accountID]--
	return nil
}

func (f *fakeStore) GetHomeTimeline(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, error) {
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

func (f *fakeStore) GetPublicTimeline(ctx context.Context, localOnly bool, maxID *string, limit int) ([]domain.Status, error) {
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

func (f *fakeStore) CreateApplication(ctx context.Context, in store.CreateApplicationInput) (*domain.OAuthApplication, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeStore) GetApplicationByClientID(ctx context.Context, clientID string) (*domain.OAuthApplication, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeStore) CreateAuthorizationCode(ctx context.Context, in store.CreateAuthorizationCodeInput) (*domain.OAuthAuthorizationCode, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeStore) GetAuthorizationCode(ctx context.Context, code string) (*domain.OAuthAuthorizationCode, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeStore) DeleteAuthorizationCode(ctx context.Context, code string) error {
	return nil
}
func (f *fakeStore) CreateAccessToken(ctx context.Context, in store.CreateAccessTokenInput) (*domain.OAuthAccessToken, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeStore) GetAccessToken(ctx context.Context, token string) (*domain.OAuthAccessToken, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeStore) RevokeAccessToken(ctx context.Context, token string) error {
	return nil
}
func (f *fakeStore) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.usersByAccountID {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (f *fakeStore) GetUserByAccountID(ctx context.Context, accountID string) (*domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.usersByAccountID[accountID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return u, nil
}
