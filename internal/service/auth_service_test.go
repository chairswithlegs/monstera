package service

import (
	"context"
	"testing"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func createTestUserAndAccount(t *testing.T, fake *testutil.FakeStore, email, password string, confirmed, suspended bool) (user *domain.User, account *domain.Account) {
	t.Helper()
	ctx := context.Background()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)

	acc, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID:       "acc-" + email,
		Username: "user-" + email,
	})
	require.NoError(t, err)

	u, err := fake.CreateUser(ctx, store.CreateUserInput{
		ID:           "usr-" + email,
		AccountID:    acc.ID,
		Email:        email,
		PasswordHash: string(hash),
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	if confirmed {
		require.NoError(t, fake.ConfirmUser(ctx, u.ID))
	}
	if suspended {
		require.NoError(t, fake.SuspendAccount(ctx, acc.ID))
	}

	return u, acc
}

func TestAuthService_Authenticate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		email     string
		password  string
		setup     func(t *testing.T, fake *testutil.FakeStore) string // returns expected account ID
		wantErr   error
		wantAccID bool
	}{
		{
			name:     "valid credentials",
			email:    "valid@example.com",
			password: "password",
			setup: func(t *testing.T, fake *testutil.FakeStore) string {
				t.Helper()
				_, acc := createTestUserAndAccount(t, fake, "valid@example.com", "password", true, false)
				return acc.ID
			},
			wantAccID: true,
		},
		{
			name:     "wrong password",
			email:    "wrong@example.com",
			password: "wrongpassword",
			setup: func(t *testing.T, fake *testutil.FakeStore) string {
				t.Helper()
				createTestUserAndAccount(t, fake, "wrong@example.com", "correctpassword", true, false)
				return ""
			},
			wantErr: domain.ErrNotFound,
		},
		{
			name:     "unknown email",
			email:    "unknown@example.com",
			password: "password",
			setup: func(t *testing.T, fake *testutil.FakeStore) string {
				t.Helper()
				return ""
			},
			wantErr: domain.ErrNotFound,
		},
		{
			name:     "unconfirmed user",
			email:    "unconfirmed@example.com",
			password: "password",
			setup: func(t *testing.T, fake *testutil.FakeStore) string {
				t.Helper()
				createTestUserAndAccount(t, fake, "unconfirmed@example.com", "password", false, false)
				return ""
			},
			wantErr: ErrUnconfirmed,
		},
		{
			name:     "suspended account",
			email:    "suspended@example.com",
			password: "password",
			setup: func(t *testing.T, fake *testutil.FakeStore) string {
				t.Helper()
				createTestUserAndAccount(t, fake, "suspended@example.com", "password", true, true)
				return ""
			},
			wantErr: ErrSuspended,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			fake := testutil.NewFakeStore()
			svc := NewAuthService(fake, "", "")

			expectedID := tc.setup(t, fake)

			accountID, err := svc.Authenticate(ctx, tc.email, tc.password)

			if tc.wantErr != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tc.wantErr)
				assert.Empty(t, accountID)
			} else {
				require.NoError(t, err)
				assert.Equal(t, expectedID, accountID)
			}
		})
	}
}

func TestAuthService_ValidateRedirectURI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		uri              string
		app              *domain.OAuthApplication
		monsteraUIHost   string
		monsteraClientID string
		want             bool
	}{
		{
			name: "oob redirect",
			uri:  "urn:ietf:wg:oauth:2.0:oob",
			app:  &domain.OAuthApplication{ClientID: "c1", RedirectURIs: "https://app.example.com/callback"},
			want: true,
		},
		{
			name: "registered URI matches",
			uri:  "https://app.example.com/callback",
			app:  &domain.OAuthApplication{ClientID: "c1", RedirectURIs: "https://other.example.com/cb\nhttps://app.example.com/callback"},
			want: true,
		},
		{
			name: "unregistered URI",
			uri:  "https://evil.example.com/callback",
			app:  &domain.OAuthApplication{ClientID: "c1", RedirectURIs: "https://app.example.com/callback"},
			want: false,
		},
		{
			name:             "monstera UI host",
			uri:              "https://ui.monstera.social/auth/callback",
			app:              &domain.OAuthApplication{ClientID: "monstera-ui"},
			monsteraUIHost:   "ui.monstera.social",
			monsteraClientID: "monstera-ui",
			want:             true,
		},
		{
			name: "nil app",
			uri:  "https://app.example.com/callback",
			app:  nil,
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := NewAuthService(testutil.NewFakeStore(), tc.monsteraUIHost, tc.monsteraClientID)
			got := svc.ValidateRedirectURI(tc.uri, tc.app)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestAuthService_GetApplicationByClientID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		clientID string
		seed     bool
		wantErr  error
	}{
		{
			name:     "existing app",
			clientID: "client-abc",
			seed:     true,
		},
		{
			name:     "not found",
			clientID: "nonexistent",
			seed:     false,
			wantErr:  domain.ErrNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			fake := testutil.NewFakeStore()
			svc := NewAuthService(fake, "", "")

			if tc.seed {
				_, err := fake.CreateApplication(ctx, store.CreateApplicationInput{
					ID:           "app-1",
					Name:         "Test App",
					ClientID:     tc.clientID,
					ClientSecret: "secret",
					RedirectURIs: "https://example.com/cb",
					Scopes:       "read",
				})
				require.NoError(t, err)
			}

			app, err := svc.GetApplicationByClientID(ctx, tc.clientID)

			if tc.wantErr != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, app)
			} else {
				require.NoError(t, err)
				require.NotNil(t, app)
				assert.Equal(t, tc.clientID, app.ClientID)
			}
		})
	}
}
