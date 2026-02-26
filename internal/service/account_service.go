package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	"github.com/chairswithlegs/monstera-fed/internal/uid"
)

// AccountService handles account creation and lookup.
type AccountService struct {
	store store.Store
	// instanceBaseURL is the scheme + host for the instance (e.g. "https://example.com").
	instanceBaseURL string
}

// NewAccountService returns an AccountService that uses the given store and instance base URL.
func NewAccountService(s store.Store, instanceBaseURL string) *AccountService {
	base := strings.TrimSuffix(instanceBaseURL, "/")
	return &AccountService{store: s, instanceBaseURL: base}
}

// CreateAccountInput is the input for creating a local account (no user record).
type CreateAccountInput struct {
	Username    string
	DisplayName *string
	Note        *string
	Bot         bool
	Locked      bool
}

// Create creates a local account. For local accounts (no domain), generates an RSA key pair and builds AP URLs from instanceBaseURL.
func (svc *AccountService) Create(ctx context.Context, in CreateAccountInput) (*domain.Account, error) {
	return svc.createAccountWithStore(ctx, svc.store, in)
}

func (svc *AccountService) createAccountWithStore(ctx context.Context, s store.Store, in CreateAccountInput) (*domain.Account, error) {
	if in.Username == "" {
		return nil, fmt.Errorf("CreateAccount: %w", domain.ErrValidation)
	}
	publicKey, privateKey, err := generateRSAKeyPair()
	if err != nil {
		return nil, fmt.Errorf("CreateAccount: %w", err)
	}
	id := uid.New()
	apID := fmt.Sprintf("%s/users/%s", svc.instanceBaseURL, in.Username)
	inboxURL := fmt.Sprintf("%s/users/%s/inbox", svc.instanceBaseURL, in.Username)
	outboxURL := fmt.Sprintf("%s/users/%s/outbox", svc.instanceBaseURL, in.Username)
	followersURL := fmt.Sprintf("%s/users/%s/followers", svc.instanceBaseURL, in.Username)
	followingURL := fmt.Sprintf("%s/users/%s/following", svc.instanceBaseURL, in.Username)

	storeIn := store.CreateAccountInput{
		ID:           id,
		Username:     in.Username,
		Domain:       nil,
		DisplayName:  in.DisplayName,
		Note:         in.Note,
		PublicKey:    publicKey,
		PrivateKey:   &privateKey,
		InboxURL:     inboxURL,
		OutboxURL:    outboxURL,
		FollowersURL: followersURL,
		FollowingURL: followingURL,
		APID:         apID,
		ApRaw:        nil,
		Bot:          in.Bot,
		Locked:       in.Locked,
	}
	acc, err := s.CreateAccount(ctx, storeIn)
	if err != nil {
		return nil, fmt.Errorf("CreateAccount: %w", err)
	}
	return acc, nil
}

// RegisterInput is the input for registering a user (account + user in one transaction).
type RegisterInput struct {
	Username     string
	DisplayName  *string
	Note         *string
	Email        string
	PasswordHash string
	Role         string
}

// Register creates an account and a linked user in one transaction.
func (svc *AccountService) Register(ctx context.Context, in RegisterInput) (*domain.Account, error) {
	if in.Username == "" || in.Email == "" || in.PasswordHash == "" {
		return nil, fmt.Errorf("Register: %w", domain.ErrValidation)
	}
	// TODO: ensure this is a safe assumption
	if in.Role == "" {
		in.Role = domain.RoleUser
	}
	switch in.Role {
	case domain.RoleUser, domain.RoleModerator, domain.RoleAdmin:
	default:
		return nil, fmt.Errorf("Register: %w", domain.ErrValidation)
	}
	var created *domain.Account
	err := svc.store.WithTx(ctx, func(tx store.Store) error {
		acc, err := svc.createAccountWithStore(ctx, tx, CreateAccountInput{
			Username:    in.Username,
			DisplayName: in.DisplayName,
			Note:        in.Note,
			Bot:         false,
			Locked:      false,
		})
		if err != nil {
			return err
		}
		userID := uid.New()
		_, err = tx.CreateUser(ctx, store.CreateUserInput{
			ID:           userID,
			AccountID:    acc.ID,
			Email:        in.Email,
			PasswordHash: in.PasswordHash,
			Role:         in.Role,
		})
		if err != nil {
			return fmt.Errorf("CreateUser: %w", err)
		}
		created = acc
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("Register: %w", err)
	}
	return created, nil
}

// GetByID returns the account by ID, or ErrNotFound.
func (svc *AccountService) GetByID(ctx context.Context, id string) (*domain.Account, error) {
	acc, err := svc.store.GetAccountByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("GetAccountByID(%s): %w", id, err)
	}
	return acc, nil
}

// GetByUsername returns the account by username. If accountDomain is nil, looks up local account; otherwise remote.
func (svc *AccountService) GetByUsername(ctx context.Context, username string, accountDomain *string) (*domain.Account, error) {
	var acc *domain.Account
	var err error
	if accountDomain == nil {
		acc, err = svc.store.GetLocalAccountByUsername(ctx, username)
	} else {
		acc, err = svc.store.GetRemoteAccountByUsername(ctx, username, accountDomain)
	}
	if err != nil {
		return nil, fmt.Errorf("GetByUsername(%s): %w", username, err)
	}
	return acc, nil
}

// GetLocalActorForFederation returns the local account by username for federation (WebFinger, Actor, collections).
// Returns ErrNotFound if the account does not exist or is suspended (callers treat as 404).
func (svc *AccountService) GetLocalActorForFederation(ctx context.Context, username string) (*domain.Account, error) {
	acc, err := svc.store.GetLocalAccountByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("GetLocalActorForFederation(%s): %w", username, err)
	}
	if acc.Suspended {
		return nil, fmt.Errorf("GetLocalActorForFederation(%s): %w", username, domain.ErrNotFound)
	}
	return acc, nil
}

// LocalActorWithMedia is the result of resolving a local actor with avatar/header URLs.
type LocalActorWithMedia struct {
	Account   *domain.Account
	AvatarURL string
	HeaderURL string
}

// GetLocalActorWithMedia returns the local account and resolved avatar/header URLs for federation (Actor document).
// Returns ErrNotFound if the account does not exist or is suspended.
func (svc *AccountService) GetLocalActorWithMedia(ctx context.Context, username string) (*LocalActorWithMedia, error) {
	acc, err := svc.GetLocalActorForFederation(ctx, username)
	if err != nil {
		return nil, err
	}
	out := &LocalActorWithMedia{Account: acc}
	if acc.AvatarMediaID != nil {
		if att, err := svc.store.GetMediaAttachment(ctx, *acc.AvatarMediaID); err == nil && att.URL != "" {
			out.AvatarURL = att.URL
		}
	}
	if acc.HeaderMediaID != nil {
		if att, err := svc.store.GetMediaAttachment(ctx, *acc.HeaderMediaID); err == nil && att.URL != "" {
			out.HeaderURL = att.URL
		}
	}
	return out, nil
}

// CountFollowers returns the number of accepted followers for the account.
func (svc *AccountService) CountFollowers(ctx context.Context, accountID string) (int64, error) {
	n, err := svc.store.CountFollowers(ctx, accountID)
	if err != nil {
		return 0, fmt.Errorf("CountFollowers(%s): %w", accountID, err)
	}
	return n, nil
}

// CountFollowing returns the number of accepted follows for the account.
func (svc *AccountService) CountFollowing(ctx context.Context, accountID string) (int64, error) {
	n, err := svc.store.CountFollowing(ctx, accountID)
	if err != nil {
		return 0, fmt.Errorf("CountFollowing(%s): %w", accountID, err)
	}
	return n, nil
}

// GetRelationship returns the relationship between accountID (viewer) and targetID.
func (svc *AccountService) GetRelationship(ctx context.Context, accountID, targetID string) (*domain.Relationship, error) {
	rel, err := svc.store.GetRelationship(ctx, accountID, targetID)
	if err != nil {
		return nil, fmt.Errorf("GetRelationship: %w", err)
	}
	return rel, nil
}

// GetAccountWithUser returns the account and its linked user by account ID.
// Returns ErrNotFound if the account or user does not exist.
func (svc *AccountService) GetAccountWithUser(ctx context.Context, accountID string) (*domain.Account, *domain.User, error) {
	acc, err := svc.store.GetAccountByID(ctx, accountID)
	if err != nil {
		return nil, nil, fmt.Errorf("GetAccountWithUser: %w", err)
	}
	user, err := svc.store.GetUserByAccountID(ctx, accountID)
	if err != nil {
		return nil, nil, fmt.Errorf("GetAccountWithUser: %w", err)
	}
	return acc, user, nil
}
