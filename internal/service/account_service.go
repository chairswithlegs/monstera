package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	"github.com/chairswithlegs/monstera-fed/internal/uid"
)

type AccountService interface {
	GetByID(ctx context.Context, id string) (*domain.Account, error)
	GetByAPID(ctx context.Context, apID string) (*domain.Account, error)
	GetLocalByUsername(ctx context.Context, username string) (*domain.Account, error)
	Create(ctx context.Context, in CreateAccountInput) (*domain.Account, error)
	CreateOrUpdateRemoteAccount(ctx context.Context, in CreateOrUpdateRemoteInput) (*domain.Account, error)
	Suspend(ctx context.Context, accountID string) error
	GetByUsername(ctx context.Context, username string, accountDomain *string) (*domain.Account, error)
	GetLocalActorForFederation(ctx context.Context, username string) (*domain.Account, error)
	GetLocalActorWithMedia(ctx context.Context, username string) (*LocalActorWithMedia, error)
	CountFollowers(ctx context.Context, accountID string) (int64, error)
	CountFollowing(ctx context.Context, accountID string) (int64, error)
	GetRelationship(ctx context.Context, accountID, targetID string) (*domain.Relationship, error)
	GetAccountWithUser(ctx context.Context, accountID string) (*domain.Account, *domain.User, error)
	UpdateCredentials(ctx context.Context, in UpdateCredentialsInput) (*domain.Account, *domain.User, error)
	Register(ctx context.Context, in RegisterInput) (*domain.Account, error)
}

// AccountService handles account creation and lookup.
type accountService struct {
	store store.Store
	// instanceBaseURL is the scheme + host for the instance (e.g. "https://example.com").
	instanceBaseURL string
}

// NewAccountService returns an AccountService that uses the given store and instance base URL.
func NewAccountService(s store.Store, instanceBaseURL string) AccountService {
	base := strings.TrimSuffix(instanceBaseURL, "/")
	return &accountService{store: s, instanceBaseURL: base}
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
func (svc *accountService) Create(ctx context.Context, in CreateAccountInput) (*domain.Account, error) {
	return svc.createAccountWithStore(ctx, svc.store, in)
}

func (svc *accountService) createAccountWithStore(ctx context.Context, s store.Store, in CreateAccountInput) (*domain.Account, error) {
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
func (svc *accountService) Register(ctx context.Context, in RegisterInput) (*domain.Account, error) {
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
func (svc *accountService) GetByID(ctx context.Context, id string) (*domain.Account, error) {
	acc, err := svc.store.GetAccountByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("GetAccountByID(%s): %w", id, err)
	}
	return acc, nil
}

// GetByAPID returns the account by ActivityPub ID (actor IRI), or ErrNotFound.
func (svc *accountService) GetByAPID(ctx context.Context, apID string) (*domain.Account, error) {
	acc, err := svc.store.GetAccountByAPID(ctx, apID)
	if err != nil {
		return nil, fmt.Errorf("GetAccountByAPID(%s): %w", apID, err)
	}
	return acc, nil
}

// GetLocalByUsername returns the local account by username (no domain). Does not filter suspended accounts.
func (svc *accountService) GetLocalByUsername(ctx context.Context, username string) (*domain.Account, error) {
	acc, err := svc.store.GetLocalAccountByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("GetLocalByUsername(%s): %w", username, err)
	}
	return acc, nil
}

// CreateOrUpdateRemoteInput is the input for creating or updating a remote account from federation.
type CreateOrUpdateRemoteInput struct {
	APID         string
	Username     string
	Domain       string
	DisplayName  *string
	Note         *string
	PublicKey    string
	InboxURL     string
	OutboxURL    string
	FollowersURL string
	FollowingURL string
	Bot          bool
	Locked       bool
	ApRaw        []byte
}

// CreateOrUpdateRemoteAccount creates a remote account from federation input, or updates it if it already exists by APID.
func (svc *accountService) CreateOrUpdateRemoteAccount(ctx context.Context, in CreateOrUpdateRemoteInput) (*domain.Account, error) {
	existing, err := svc.store.GetAccountByAPID(ctx, in.APID)
	if err == nil {
		if err := svc.store.UpdateAccount(ctx, store.UpdateAccountInput{
			ID:          existing.ID,
			DisplayName: in.DisplayName,
			Note:        in.Note,
			APRaw:       in.ApRaw,
			Bot:         in.Bot,
			Locked:      in.Locked,
		}); err != nil {
			return nil, fmt.Errorf("CreateOrUpdateRemoteAccount UpdateAccount: %w", err)
		}
		if in.PublicKey != "" && in.PublicKey != existing.PublicKey {
			if err := svc.store.UpdateAccountKeys(ctx, existing.ID, in.PublicKey, in.ApRaw); err != nil {
				return nil, fmt.Errorf("CreateOrUpdateRemoteAccount UpdateAccountKeys: %w", err)
			}
		}
		acc, getErr := svc.store.GetAccountByAPID(ctx, in.APID)
		if getErr != nil {
			return nil, fmt.Errorf("CreateOrUpdateRemoteAccount GetAccountByAPID after update: %w", getErr)
		}
		return acc, nil
	}
	dom := in.Domain
	storeIn := store.CreateAccountInput{
		ID:           uid.New(),
		Username:     in.Username,
		Domain:       &dom,
		DisplayName:  in.DisplayName,
		Note:         in.Note,
		PublicKey:    in.PublicKey,
		InboxURL:     in.InboxURL,
		OutboxURL:    in.OutboxURL,
		FollowersURL: in.FollowersURL,
		FollowingURL: in.FollowingURL,
		APID:         in.APID,
		ApRaw:        in.ApRaw,
		Bot:          in.Bot,
		Locked:       in.Locked,
	}
	acc, createErr := svc.store.CreateAccount(ctx, storeIn)
	if createErr != nil {
		return nil, fmt.Errorf("CreateOrUpdateRemoteAccount CreateAccount: %w", createErr)
	}
	return acc, nil
}

// Suspend suspends the account by ID (e.g. for Delete{Person}).
func (svc *accountService) Suspend(ctx context.Context, accountID string) error {
	if err := svc.store.SuspendAccount(ctx, accountID); err != nil {
		return fmt.Errorf("Suspend(%s): %w", accountID, err)
	}
	return nil
}

// GetByUsername returns the account by username. If accountDomain is nil, looks up local account; otherwise remote.
func (svc *accountService) GetByUsername(ctx context.Context, username string, accountDomain *string) (*domain.Account, error) {
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
func (svc *accountService) GetLocalActorForFederation(ctx context.Context, username string) (*domain.Account, error) {
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
func (svc *accountService) GetLocalActorWithMedia(ctx context.Context, username string) (*LocalActorWithMedia, error) {
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
func (svc *accountService) CountFollowers(ctx context.Context, accountID string) (int64, error) {
	n, err := svc.store.CountFollowers(ctx, accountID)
	if err != nil {
		return 0, fmt.Errorf("CountFollowers(%s): %w", accountID, err)
	}
	return n, nil
}

// CountFollowing returns the number of accepted follows for the account.
func (svc *accountService) CountFollowing(ctx context.Context, accountID string) (int64, error) {
	n, err := svc.store.CountFollowing(ctx, accountID)
	if err != nil {
		return 0, fmt.Errorf("CountFollowing(%s): %w", accountID, err)
	}
	return n, nil
}

// GetRelationship returns the relationship between accountID (viewer) and targetID.
func (svc *accountService) GetRelationship(ctx context.Context, accountID, targetID string) (*domain.Relationship, error) {
	rel, err := svc.store.GetRelationship(ctx, accountID, targetID)
	if err != nil {
		return nil, fmt.Errorf("GetRelationship: %w", err)
	}
	return rel, nil
}

// GetAccountWithUser returns the account and its linked user by account ID.
// Returns ErrNotFound if the account or user does not exist.
func (svc *accountService) GetAccountWithUser(ctx context.Context, accountID string) (*domain.Account, *domain.User, error) {
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

// UpdateCredentialsInput is the input for updating the authenticated account's profile (PATCH update_credentials).
type UpdateCredentialsInput struct {
	AccountID     string
	DisplayName   *string
	Note          *string
	AvatarMediaID *string
	HeaderMediaID *string
	Locked        bool
	Bot           bool
	Fields        json.RawMessage // when nil or empty, existing account.Fields are preserved
}

// UpdateCredentials updates the account profile. Caller should pass current account.Fields when not updating fields.
// Returns the updated account and user for building the CredentialAccount response.
func (svc *accountService) UpdateCredentials(ctx context.Context, in UpdateCredentialsInput) (*domain.Account, *domain.User, error) {
	acc, err := svc.store.GetAccountByID(ctx, in.AccountID)
	if err != nil {
		return nil, nil, fmt.Errorf("UpdateCredentials GetAccountByID: %w", err)
	}
	fields := in.Fields
	if len(fields) == 0 {
		fields = acc.Fields
	}
	if err := svc.store.UpdateAccount(ctx, store.UpdateAccountInput{
		ID:            in.AccountID,
		DisplayName:   in.DisplayName,
		Note:          in.Note,
		AvatarMediaID: in.AvatarMediaID,
		HeaderMediaID: in.HeaderMediaID,
		Bot:           in.Bot,
		Locked:        in.Locked,
		Fields:        fields,
	}); err != nil {
		return nil, nil, fmt.Errorf("UpdateCredentials UpdateAccount: %w", err)
	}
	updated, err := svc.store.GetAccountByID(ctx, in.AccountID)
	if err != nil {
		return nil, nil, fmt.Errorf("UpdateCredentials GetAccountByID after: %w", err)
	}
	user, _ := svc.store.GetUserByAccountID(ctx, in.AccountID)
	return updated, user, nil
}
