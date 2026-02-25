package oauth

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/chairswithlegs/monstera-fed/internal/cache"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	"github.com/stretchr/testify/require"
)

type fakeOAuthStore struct {
	mu sync.Mutex

	applications     map[string]*domain.OAuthApplication
	applicationsByID map[string]*domain.OAuthApplication
	authCodes        map[string]*domain.OAuthAuthorizationCode
	tokens           map[string]*domain.OAuthAccessToken
}

func newFakeOAuthStore() *fakeOAuthStore {
	return &fakeOAuthStore{
		applications:     make(map[string]*domain.OAuthApplication),
		applicationsByID: make(map[string]*domain.OAuthApplication),
		authCodes:        make(map[string]*domain.OAuthAuthorizationCode),
		tokens:           make(map[string]*domain.OAuthAccessToken),
	}
}

func (f *fakeOAuthStore) CreateApplication(ctx context.Context, in store.CreateApplicationInput) (*domain.OAuthApplication, error) {
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

func (f *fakeOAuthStore) GetApplicationByClientID(ctx context.Context, clientID string) (*domain.OAuthApplication, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if app, ok := f.applications[clientID]; ok {
		return app, nil
	}
	return nil, domain.ErrNotFound
}

func (f *fakeOAuthStore) CreateAuthorizationCode(ctx context.Context, in store.CreateAuthorizationCodeInput) (*domain.OAuthAuthorizationCode, error) {
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

func (f *fakeOAuthStore) GetAuthorizationCode(ctx context.Context, code string) (*domain.OAuthAuthorizationCode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if ac, ok := f.authCodes[code]; ok && ac.ExpiresAt.After(time.Now()) {
		return ac, nil
	}
	return nil, domain.ErrNotFound
}

func (f *fakeOAuthStore) DeleteAuthorizationCode(ctx context.Context, code string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.authCodes, code)
	return nil
}

func (f *fakeOAuthStore) CreateAccessToken(ctx context.Context, in store.CreateAccessTokenInput) (*domain.OAuthAccessToken, error) {
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

func (f *fakeOAuthStore) GetAccessToken(ctx context.Context, token string) (*domain.OAuthAccessToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if tok, ok := f.tokens[token]; ok && tok.RevokedAt == nil {
		return tok, nil
	}
	return nil, domain.ErrNotFound
}

func (f *fakeOAuthStore) RevokeAccessToken(ctx context.Context, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if tok, ok := f.tokens[token]; ok {
		now := time.Now()
		tok.RevokedAt = &now
	}
	return nil
}

func (f *fakeOAuthStore) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeOAuthStore) GetUserByAccountID(ctx context.Context, accountID string) (*domain.User, error) {
	return nil, domain.ErrNotFound
}

func (f *fakeOAuthStore) CreateAccount(ctx context.Context, in store.CreateAccountInput) (*domain.Account, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeOAuthStore) GetAccountByID(ctx context.Context, id string) (*domain.Account, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeOAuthStore) GetLocalAccountByUsername(ctx context.Context, username string) (*domain.Account, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeOAuthStore) GetRemoteAccountByUsername(ctx context.Context, username string, d *string) (*domain.Account, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeOAuthStore) WithTx(ctx context.Context, fn func(store.Store) error) error {
	return fn(f)
}
func (f *fakeOAuthStore) CreateUser(ctx context.Context, in store.CreateUserInput) (*domain.User, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeOAuthStore) CreateStatus(ctx context.Context, in store.CreateStatusInput) (*domain.Status, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeOAuthStore) GetStatusByID(ctx context.Context, id string) (*domain.Status, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeOAuthStore) DeleteStatus(ctx context.Context, id string) error {
	return nil
}
func (f *fakeOAuthStore) IncrementStatusesCount(ctx context.Context, accountID string) error {
	return nil
}
func (f *fakeOAuthStore) DecrementStatusesCount(ctx context.Context, accountID string) error {
	return nil
}
func (f *fakeOAuthStore) GetHomeTimeline(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, error) {
	return nil, nil
}
func (f *fakeOAuthStore) GetPublicTimeline(ctx context.Context, localOnly bool, maxID *string, limit int) ([]domain.Status, error) {
	return nil, nil
}
func (f *fakeOAuthStore) CreateStatusMention(ctx context.Context, statusID, accountID string) error {
	return nil
}
func (f *fakeOAuthStore) GetStatusMentions(ctx context.Context, statusID string) ([]*domain.Account, error) {
	return nil, nil
}
func (f *fakeOAuthStore) GetOrCreateHashtag(ctx context.Context, name string) (*domain.Hashtag, error) {
	return &domain.Hashtag{ID: "tag-" + name, Name: name}, nil
}
func (f *fakeOAuthStore) AttachHashtagsToStatus(ctx context.Context, statusID string, hashtagIDs []string) error {
	return nil
}
func (f *fakeOAuthStore) GetStatusHashtags(ctx context.Context, statusID string) ([]domain.Hashtag, error) {
	return nil, nil
}
func (f *fakeOAuthStore) CreateNotification(ctx context.Context, in store.CreateNotificationInput) (*domain.Notification, error) {
	return &domain.Notification{
		ID:        in.ID,
		AccountID: in.AccountID,
		FromID:    in.FromID,
		Type:      in.Type,
		StatusID:  in.StatusID,
		CreatedAt: time.Now(),
	}, nil
}
func (f *fakeOAuthStore) GetStatusAttachments(ctx context.Context, statusID string) ([]domain.MediaAttachment, error) {
	return nil, nil
}

var _ store.Store = (*fakeOAuthStore)(nil)

func TestServer_RegisterApplication(t *testing.T) {
	ctx := context.Background()
	c, err := cache.New(cache.Config{Driver: "memory", Logger: slog.Default()})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	srv := NewServer(newFakeOAuthStore(), c, slog.Default())

	app, err := srv.RegisterApplication(ctx, "Test App", "https://app.example/cb", "read write", "https://app.example")
	require.NoError(t, err)
	require.NotNil(t, app)
	require.NotEmpty(t, app.ID)
	require.Equal(t, "Test App", app.Name)
	require.Len(t, app.ClientID, 64)
	require.Len(t, app.ClientSecret, 64)
	require.Equal(t, "https://app.example/cb", app.RedirectURI)
	require.Empty(t, app.VapidKey)
}

func TestServer_AuthorizeRequest_ExchangeCode(t *testing.T) {
	ctx := context.Background()
	st := newFakeOAuthStore()
	c, err := cache.New(cache.Config{Driver: "memory", Logger: slog.Default()})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	app, _ := st.CreateApplication(ctx, store.CreateApplicationInput{
		ID:           "app1",
		Name:         "App",
		ClientID:     "client123",
		ClientSecret: "secret456",
		RedirectURIs: "https://app.example/cb",
		Scopes:       "read write",
		Website:      nil,
	})
	require.NotNil(t, app)

	srv := NewServer(st, c, slog.Default())

	code, err := srv.AuthorizeRequest(ctx, AuthorizeRequest{
		ApplicationID:       app.ID,
		AccountID:           "acc1",
		RedirectURI:         "https://app.example/cb",
		Scopes:              "read write",
		CodeChallenge:       GenerateCodeChallenge("verifier123"),
		CodeChallengeMethod: "S256",
	})
	require.NoError(t, err)
	require.NotEmpty(t, code)

	resp, err := srv.ExchangeCode(ctx, TokenRequest{
		GrantType:    "authorization_code",
		Code:         code,
		RedirectURI:  "https://app.example/cb",
		ClientID:     "client123",
		ClientSecret: "secret456",
		CodeVerifier: "verifier123",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.AccessToken)
	require.Equal(t, "Bearer", resp.TokenType)
	require.Contains(t, resp.Scope, "read")

	claims, err := srv.LookupToken(ctx, resp.AccessToken)
	require.NoError(t, err)
	require.Equal(t, "acc1", claims.AccountID)
	require.Equal(t, app.ID, claims.ApplicationID)
	require.True(t, claims.Scopes.HasScope("read:statuses"))
}

func TestServer_ExchangeClientCredentials(t *testing.T) {
	ctx := context.Background()
	st := newFakeOAuthStore()
	c, err := cache.New(cache.Config{Driver: "memory", Logger: slog.Default()})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	app, _ := st.CreateApplication(ctx, store.CreateApplicationInput{
		ID:           "app1",
		Name:         "App",
		ClientID:     "client",
		ClientSecret: "secret",
		RedirectURIs: "https://app.example/cb",
		Scopes:       "read write",
		Website:      nil,
	})
	require.NotNil(t, app)

	srv := NewServer(st, c, slog.Default())

	resp, err := srv.ExchangeClientCredentials(ctx, TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     "client",
		ClientSecret: "secret",
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.AccessToken)

	claims, err := srv.LookupToken(ctx, resp.AccessToken)
	require.NoError(t, err)
	require.Empty(t, claims.AccountID)
	require.True(t, claims.Scopes.HasScope("read"))
	require.False(t, claims.Scopes.HasScope("write"))
}

func TestServer_RevokeToken(t *testing.T) {
	ctx := context.Background()
	st := newFakeOAuthStore()
	c, err := cache.New(cache.Config{Driver: "memory", Logger: slog.Default()})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	app, _ := st.CreateApplication(ctx, store.CreateApplicationInput{
		ID:           "app1",
		Name:         "App",
		ClientID:     "c",
		ClientSecret: "s",
		RedirectURIs: "https://x/cb",
		Scopes:       "read",
		Website:      nil,
	})
	require.NotNil(t, app)

	srv := NewServer(st, c, slog.Default())
	resp, err := srv.ExchangeClientCredentials(ctx, TokenRequest{
		GrantType: "client_credentials", ClientID: "c", ClientSecret: "s",
	})
	require.NoError(t, err)
	token := resp.AccessToken

	_, err = srv.LookupToken(ctx, token)
	require.NoError(t, err)

	err = srv.RevokeToken(ctx, token)
	require.NoError(t, err)

	// Cache eviction may be eventually consistent (e.g. ristretto), so retry
	// until LookupToken returns an error or we timeout.
	require.Eventually(t, func() bool {
		_, err := srv.LookupToken(ctx, token)
		return err != nil
	}, 500*time.Millisecond, 10*time.Millisecond,
		"LookupToken should return an error after RevokeToken (cache eviction may be eventual)")
}
