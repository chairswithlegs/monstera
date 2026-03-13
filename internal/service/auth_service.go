package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"golang.org/x/crypto/bcrypt"
)

// ErrUnconfirmed is returned by Authenticate when the user has not confirmed their email.
var ErrUnconfirmed = errors.New("unconfirmed")

// ErrSuspended is returned by Authenticate when the account is suspended.
var ErrSuspended = errors.New("suspended")

// AuthService handles authentication and OAuth app lookup for the login/authorize flow.
type AuthService interface {
	// Authenticate validates email+password, checks confirmation and suspension.
	// Returns the account ID on success.
	Authenticate(ctx context.Context, email, password string) (accountID string, err error)
	// ValidateRedirectURI checks if the URI is valid for the given app.
	ValidateRedirectURI(uri string, app *domain.OAuthApplication) bool
	// GetApplicationByClientID looks up an OAuth app.
	GetApplicationByClientID(ctx context.Context, clientID string) (*domain.OAuthApplication, error)
}

type authService struct {
	store              store.Store
	monsteraUIHost     string
	monsteraUIClientID string
}

// NewAuthService returns an AuthService. monsteraUIHost and monsteraUIClientID are used to
// allow the built-in Monstera UI redirect URI; pass empty strings to disable.
func NewAuthService(s store.Store, monsteraUIHost, monsteraUIClientID string) AuthService {
	return &authService{
		store:              s,
		monsteraUIHost:     monsteraUIHost,
		monsteraUIClientID: monsteraUIClientID,
	}
}

const outOfBandRedirectURI = "urn:ietf:wg:oauth:2.0:oob"

func (svc *authService) Authenticate(ctx context.Context, email, password string) (string, error) {
	user, err := svc.store.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return "", domain.ErrNotFound
		}
		return "", fmt.Errorf("GetUserByEmail: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", domain.ErrNotFound
	}
	if user.ConfirmedAt == nil {
		return "", ErrUnconfirmed
	}
	account, err := svc.store.GetAccountByID(ctx, user.AccountID)
	if err != nil {
		return "", fmt.Errorf("GetAccountByID(%s): %w", user.AccountID, err)
	}
	if account.Suspended {
		return "", ErrSuspended
	}
	return account.ID, nil
}

func (svc *authService) ValidateRedirectURI(uri string, app *domain.OAuthApplication) bool {
	if app == nil {
		slog.Error("application is nil")
		return false
	}
	if uri == outOfBandRedirectURI {
		return true
	}
	parsedURI, err := url.Parse(uri)
	if err != nil {
		slog.Error("failed to parse redirect URI", slog.Any("error", err))
		return false
	}
	if svc.monsteraUIClientID != "" && svc.monsteraUIHost != "" &&
		app.ClientID == svc.monsteraUIClientID && parsedURI.Host == svc.monsteraUIHost {
		slog.Info("valid internal redirect URI", slog.String("uri", uri))
		return true
	}
	for _, r := range strings.Split(app.RedirectURIs, "\n") {
		if strings.TrimSpace(r) == uri {
			return true
		}
	}
	return false
}

func (svc *authService) GetApplicationByClientID(ctx context.Context, clientID string) (*domain.OAuthApplication, error) {
	app, err := svc.store.GetApplicationByClientID(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("GetApplicationByClientID(%s): %w", clientID, err)
	}
	return app, nil
}
