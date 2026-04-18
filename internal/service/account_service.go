package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/events"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

type AccountService interface {
	GetByID(ctx context.Context, id string) (*domain.Account, error)
	GetByAPID(ctx context.Context, apID string) (*domain.Account, error)
	GetLocalByUsername(ctx context.Context, username string) (*domain.Account, error)
	GetActiveLocalAccount(ctx context.Context, username string) (*domain.Account, error)
	GetByUsername(ctx context.Context, username string, accountDomain *string) (*domain.Account, error)
	Create(ctx context.Context, in CreateAccountInput) (*domain.Account, error)
	CreateOrUpdateRemoteAccount(ctx context.Context, in CreateOrUpdateRemoteInput) (*domain.Account, error)
	SuspendRemote(ctx context.Context, accountID string) error
	// SetRemotePins replaces all pinned statuses for a remote account.
	// Clears existing pins then creates one pin per statusID.
	// Returns ErrForbidden if accountID refers to a local account.
	SetRemotePins(ctx context.Context, accountID string, statusIDs []string) error
	CountFollowers(ctx context.Context, accountID string) (int64, error)
	CountFollowing(ctx context.Context, accountID string) (int64, error)
	GetRelationship(ctx context.Context, accountID, targetID string) (*domain.Relationship, error)
	GetAccountWithUser(ctx context.Context, accountID string) (*domain.Account, *domain.User, error)
	UpdateCredentials(ctx context.Context, in UpdateCredentialsInput) (*domain.Account, *domain.User, error)
	Register(ctx context.Context, in RegisterInput) (*domain.Account, error)
	ListLocalUsers(ctx context.Context, limit, offset int) ([]domain.User, error)
	ListDirectory(ctx context.Context, order string, localOnly bool, offset, limit int) ([]domain.Account, error)
	UpdatePreferences(ctx context.Context, userID string, in UpdatePreferencesInput) (*domain.User, error)
	ChangeEmail(ctx context.Context, userID, newEmail string) (*domain.User, error)
	ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error
	// DeleteSelf verifies the user's current password then permanently deletes
	// their local account. Hard-deletes the account row (Postgres CASCADE
	// removes statuses, follows, oauth tokens, etc.) and emits
	// EventAccountDeleted in the same transaction so federation can fan out a
	// Delete{Actor} to remote followers using the snapshot in the payload.
	//
	// Returns domain.ErrForbidden if the password does not match or the
	// account is remote.
	DeleteSelf(ctx context.Context, userID, currentPassword string) error
	// DeleteLocalAccount is the admin/moderation entry point. It performs the
	// same hard-delete flow as DeleteSelf without a password check. Used by
	// ModerationService.DeleteAccount.
	DeleteLocalAccount(ctx context.Context, accountID string) error
}

// UpdatePreferencesInput is the input for updating a user's post preferences.
type UpdatePreferencesInput struct {
	DefaultPrivacy     string
	DefaultSensitive   bool
	DefaultLanguage    string
	DefaultQuotePolicy string
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
	if in.Username == "" {
		return nil, fmt.Errorf("CreateAccount: %w", domain.ErrValidation)
	}
	publicKey, privateKey, err := generateRSAKeyPair()
	if err != nil {
		return nil, fmt.Errorf("CreateAccount: %w", err)
	}
	id := uid.New()

	// Build AP IRIs from instanceBaseURL and username.
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
		Bot:          in.Bot,
		Locked:       in.Locked,
	}
	acc, err := svc.store.CreateAccount(ctx, storeIn)
	if err != nil {
		return nil, fmt.Errorf("CreateAccount: %w", err)
	}
	return acc, nil
}

// RegisterInput is the input for registering a user (account + user in one transaction).
type RegisterInput struct {
	Username           string
	DisplayName        *string
	Note               *string
	Email              string
	Password           string
	Role               string
	RegistrationReason *string // optional; used when registration_mode is approval
	InviteCode         *string // required when registration_mode is invite
}

// Register creates an account and a linked user in one transaction.
// Behaviour depends on instance setting registration_mode: "open" (or unset) confirms immediately;
// "approval" leaves user unconfirmed and stores RegistrationReason; "invite" requires a valid InviteCode and confirms on success.
func (svc *accountService) Register(ctx context.Context, in RegisterInput) (*domain.Account, error) {
	if in.Username == "" || in.Email == "" || in.Password == "" {
		return nil, fmt.Errorf("Register: %w", domain.ErrValidation)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("Register: hash password: %w", err)
	}
	passwordHash := string(hash)
	if in.Role == "" {
		in.Role = domain.RoleUser
	}
	switch in.Role {
	case domain.RoleUser, domain.RoleModerator, domain.RoleAdmin:
	default:
		return nil, fmt.Errorf("Register: %w", domain.ErrValidation)
	}
	settings, err := svc.store.GetMonsteraSettings(ctx)
	if err != nil {
		settings = &domain.MonsteraSettings{RegistrationMode: domain.MonsteraRegistrationModeOpen}
	}
	regMode := settings.RegistrationMode
	confirm := regMode != domain.MonsteraRegistrationModeApproval
	if regMode == domain.MonsteraRegistrationModeInvite {
		if in.InviteCode == nil || *in.InviteCode == "" {
			return nil, fmt.Errorf("Register: invite code required: %w", domain.ErrValidation)
		}
		inv, err := svc.store.GetInviteByCode(ctx, *in.InviteCode)
		if err != nil {
			return nil, fmt.Errorf("Register: invalid invite code: %w", err)
		}
		if inv.ExpiresAt != nil && inv.ExpiresAt.Before(time.Now()) {
			return nil, fmt.Errorf("Register: invite code expired: %w", domain.ErrValidation)
		}
		if inv.MaxUses != nil && inv.Uses >= *inv.MaxUses {
			return nil, fmt.Errorf("Register: invite code exhausted: %w", domain.ErrValidation)
		}
	}
	publicKey, privateKey, err := generateRSAKeyPair()
	if err != nil {
		return nil, fmt.Errorf("Register: generate key pair: %w", err)
	}
	accountID := uid.New()
	apID := fmt.Sprintf("%s/users/%s", svc.instanceBaseURL, in.Username)

	var created *domain.Account
	err = svc.store.WithTx(ctx, func(tx store.Store) error {
		acc, err := tx.CreateAccount(ctx, store.CreateAccountInput{
			ID:           accountID,
			Username:     in.Username,
			Domain:       nil,
			DisplayName:  in.DisplayName,
			Note:         in.Note,
			PublicKey:    publicKey,
			PrivateKey:   &privateKey,
			InboxURL:     fmt.Sprintf("%s/users/%s/inbox", svc.instanceBaseURL, in.Username),
			OutboxURL:    fmt.Sprintf("%s/users/%s/outbox", svc.instanceBaseURL, in.Username),
			FollowersURL: fmt.Sprintf("%s/users/%s/followers", svc.instanceBaseURL, in.Username),
			FollowingURL: fmt.Sprintf("%s/users/%s/following", svc.instanceBaseURL, in.Username),
			APID:         apID,
			Bot:          false,
			Locked:       false,
		})
		if err != nil {
			return fmt.Errorf("CreateAccount: %w", err)
		}
		userID := uid.New()
		_, err = tx.CreateUser(ctx, store.CreateUserInput{
			ID:                 userID,
			AccountID:          acc.ID,
			Email:              in.Email,
			PasswordHash:       passwordHash,
			Role:               in.Role,
			RegistrationReason: in.RegistrationReason,
		})
		if err != nil {
			return fmt.Errorf("CreateUser: %w", err)
		}
		if confirm {
			if err := tx.ConfirmUser(ctx, userID); err != nil {
				return fmt.Errorf("ConfirmUser: %w", err)
			}
		}
		if regMode == "invite" && in.InviteCode != nil {
			if err := tx.IncrementInviteUses(ctx, *in.InviteCode); err != nil {
				return fmt.Errorf("IncrementInviteUses: %w", err)
			}
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
	APID           string
	Username       string
	Domain         string
	DisplayName    *string
	Note           *string
	PublicKey      string
	InboxURL       string
	OutboxURL      string
	FollowersURL   string
	FollowingURL   string
	SharedInboxURL string
	AvatarURL      string
	HeaderURL      string
	FeaturedURL    string
	URL            string
	Fields         json.RawMessage
	Bot            bool
	Locked         bool
	FollowersCount int
	FollowingCount int
	StatusesCount  int
}

// CreateOrUpdateRemoteAccount creates a remote account from federation input, or updates it if it already exists by APID.
func (svc *accountService) CreateOrUpdateRemoteAccount(ctx context.Context, in CreateOrUpdateRemoteInput) (*domain.Account, error) {
	// Check if the account already exists.
	existing, err := svc.store.GetAccountByAPID(ctx, in.APID)
	if err == nil {
		if existing.IsLocal() {
			return nil, fmt.Errorf("CreateOrUpdateRemoteAccount: cannot update local account: %w", domain.ErrForbidden)
		}
		fields := in.Fields
		if len(fields) == 0 {
			fields = existing.Fields
		}
		var urlPtr *string
		if in.URL != "" {
			urlPtr = &in.URL
		}
		if err := svc.store.UpdateAccount(ctx, store.UpdateAccountInput{
			ID:          existing.ID,
			DisplayName: in.DisplayName,
			Note:        in.Note,
			Bot:         in.Bot,
			Locked:      in.Locked,
			Fields:      fields,
			URL:         urlPtr,
		}); err != nil {
			return nil, fmt.Errorf("CreateOrUpdateRemoteAccount UpdateAccount: %w", err)
		}
		if err := svc.store.UpdateAccountURLs(ctx, existing.ID, in.InboxURL, in.OutboxURL, in.FollowersURL, in.FollowingURL); err != nil {
			return nil, fmt.Errorf("CreateOrUpdateRemoteAccount UpdateAccountURLs: %w", err)
		}
		if in.PublicKey != "" && in.PublicKey != existing.PublicKey {
			if err := svc.store.UpdateAccountKeys(ctx, existing.ID, in.PublicKey); err != nil {
				return nil, fmt.Errorf("CreateOrUpdateRemoteAccount UpdateAccountKeys: %w", err)
			}
		}
		if err := svc.store.UpdateRemoteAccountMeta(ctx, existing.ID, in.AvatarURL, in.HeaderURL, in.FollowersCount, in.FollowingCount, in.StatusesCount, in.FeaturedURL); err != nil {
			return nil, fmt.Errorf("CreateOrUpdateRemoteAccount UpdateRemoteAccountMeta: %w", err)
		}
		acc, getErr := svc.store.GetAccountByAPID(ctx, in.APID)
		if getErr != nil {
			return nil, fmt.Errorf("CreateOrUpdateRemoteAccount GetAccountByAPID after update: %w", getErr)
		}
		return acc, nil
	}

	if !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("CreateOrUpdateRemoteAccount GetAccountByAPID: %w", err)
	}

	if in.Domain == "" {
		return nil, fmt.Errorf("CreateOrUpdateRemoteAccount: empty domain: %w", domain.ErrValidation)
	}

	dom := in.Domain
	var urlPtr *string
	if in.URL != "" {
		urlPtr = &in.URL
	}
	storeIn := store.CreateAccountInput{
		ID:             uid.New(),
		Username:       in.Username,
		Domain:         &dom,
		DisplayName:    in.DisplayName,
		Note:           in.Note,
		PublicKey:      in.PublicKey,
		InboxURL:       in.InboxURL,
		OutboxURL:      in.OutboxURL,
		FollowersURL:   in.FollowersURL,
		FollowingURL:   in.FollowingURL,
		APID:           in.APID,
		Bot:            in.Bot,
		Locked:         in.Locked,
		URL:            urlPtr,
		AvatarURL:      in.AvatarURL,
		HeaderURL:      in.HeaderURL,
		FeaturedURL:    in.FeaturedURL,
		FollowersCount: in.FollowersCount,
		FollowingCount: in.FollowingCount,
		StatusesCount:  in.StatusesCount,
	}
	acc, createErr := svc.store.CreateAccount(ctx, storeIn)
	if createErr != nil {
		return nil, fmt.Errorf("CreateOrUpdateRemoteAccount CreateAccount: %w", createErr)
	}
	return acc, nil
}

// SuspendRemote suspends a remote account by ID (e.g. from federation Delete{Person}).
// Returns ErrForbidden if the account is local. Does not emit domain events.
func (svc *accountService) SuspendRemote(ctx context.Context, accountID string) error {
	acc, err := svc.store.GetAccountByID(ctx, accountID)
	if err != nil {
		return fmt.Errorf("SuspendRemote(%s): %w", accountID, err)
	}
	if acc.IsLocal() {
		return fmt.Errorf("SuspendRemote(%s): %w", accountID, domain.ErrForbidden)
	}
	if err := svc.store.SuspendAccount(ctx, accountID); err != nil {
		return fmt.Errorf("SuspendRemote(%s): %w", accountID, err)
	}
	return nil
}

// SetRemotePins replaces all pinned statuses for a remote account.
// Returns ErrForbidden if accountID refers to a local account.
func (svc *accountService) SetRemotePins(ctx context.Context, accountID string, statusIDs []string) error {
	acc, err := svc.store.GetAccountByID(ctx, accountID)
	if err != nil {
		return fmt.Errorf("SetRemotePins(%s): %w", accountID, err)
	}
	if acc.IsLocal() {
		return fmt.Errorf("SetRemotePins(%s): %w", accountID, domain.ErrForbidden)
	}
	if err := svc.store.ReplaceAccountPins(ctx, accountID, statusIDs); err != nil {
		return fmt.Errorf("SetRemotePins(%s): %w", accountID, err)
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

// GetActiveLocalAccount returns the local account by username if it exists, is not suspended, and the user is confirmed.
// Returns ErrNotFound otherwise.
func (svc *accountService) GetActiveLocalAccount(ctx context.Context, username string) (*domain.Account, error) {
	acc, err := svc.store.GetLocalAccountByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("GetActiveLocalAccount(%s): %w", username, err)
	}
	if acc.Suspended {
		return nil, fmt.Errorf("GetActiveLocalAccount(%s): %w", username, domain.ErrNotFound)
	}
	user, err := svc.store.GetUserByAccountID(ctx, acc.ID)
	if err != nil || user == nil || user.ConfirmedAt == nil {
		return nil, fmt.Errorf("GetActiveLocalAccount(%s): %w", username, domain.ErrNotFound)
	}
	return acc, nil
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
	rel := &domain.Relationship{
		TargetID:       targetID,
		ShowingReblogs: true,
		Notifying:      false,
		Endorsed:       false,
		Note:           "",
	}
	fw, err := svc.store.GetFollow(ctx, accountID, targetID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("GetFollow(actor->target): %w", err)
	}
	if err == nil {
		switch fw.State {
		case domain.FollowStateAccepted:
			rel.Following = true
		case domain.FollowStatePending:
			rel.Following = true
			rel.Requested = true
		}
	}
	bw, err := svc.store.GetFollow(ctx, targetID, accountID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("GetFollow(target->actor): %w", err)
	}
	if err == nil && bw.State == domain.FollowStateAccepted {
		rel.FollowedBy = true
	}
	_, err = svc.store.GetBlock(ctx, accountID, targetID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("GetBlock(actor->target): %w", err)
	}
	if err == nil {
		rel.Blocking = true
	}
	_, err = svc.store.GetBlock(ctx, targetID, accountID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("GetBlock(target->actor): %w", err)
	}
	if err == nil {
		rel.BlockedBy = true
	}
	m, err := svc.store.GetMute(ctx, accountID, targetID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("GetMute: %w", err)
	}
	if err == nil {
		rel.Muting = true
		rel.MutingNotifications = m.HideNotifications
	}
	target, err := svc.store.GetAccountByID(ctx, targetID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("GetAccountByID(target): %w", err)
	}
	if target != nil && target.IsRemote() && target.Domain != nil {
		blocked, dbErr := svc.store.IsUserDomainBlocked(ctx, accountID, *target.Domain)
		if dbErr != nil {
			return nil, fmt.Errorf("IsUserDomainBlocked: %w", dbErr)
		}
		rel.DomainBlocking = blocked
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
	AccountID          string
	DisplayName        *string
	Note               *string
	AvatarMediaID      *string
	HeaderMediaID      *string
	AvatarURL          *string
	HeaderURL          *string
	Locked             bool
	Bot                bool
	DefaultQuotePolicy *string         // public | followers | nobody
	Fields             json.RawMessage // when nil or empty, existing account.Fields are preserved
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
		AvatarURL:     in.AvatarURL,
		HeaderURL:     in.HeaderURL,
		Bot:           in.Bot,
		Locked:        in.Locked,
		Fields:        fields,
	}); err != nil {
		return nil, nil, fmt.Errorf("UpdateCredentials UpdateAccount: %w", err)
	}
	if in.DefaultQuotePolicy != nil {
		policy := strings.TrimSpace(*in.DefaultQuotePolicy)
		switch policy {
		case domain.QuotePolicyPublic, domain.QuotePolicyFollowers, domain.QuotePolicyNobody:
			if err := svc.store.UpdateUserDefaultQuotePolicy(ctx, in.AccountID, policy); err != nil {
				return nil, nil, fmt.Errorf("UpdateCredentials UpdateUserDefaultQuotePolicy: %w", err)
			}
		default:
			return nil, nil, fmt.Errorf("UpdateCredentials default_quote_policy: %w", domain.ErrValidation)
		}
	}
	updated, err := svc.store.GetAccountByID(ctx, in.AccountID)
	if err != nil {
		return nil, nil, fmt.Errorf("UpdateCredentials GetAccountByID after: %w", err)
	}
	user, err := svc.store.GetUserByAccountID(ctx, in.AccountID)
	if err != nil {
		return nil, nil, fmt.Errorf("UpdateCredentials GetUserByAccountID after: %w", err)
	}
	_ = events.EmitEvent(ctx, svc.store, domain.EventAccountUpdated, "account", updated.ID, domain.AccountUpdatedPayload{
		Account: updated,
		Local:   updated.IsLocal(),
	})
	return updated, user, nil
}

// ListLocalUsers returns local users for admin listing.
func (svc *accountService) ListLocalUsers(ctx context.Context, limit, offset int) ([]domain.User, error) {
	users, err := svc.store.ListLocalUsers(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("ListLocalUsers: %w", err)
	}
	return users, nil
}

func (svc *accountService) ListDirectory(ctx context.Context, order string, localOnly bool, offset, limit int) ([]domain.Account, error) {
	limit = ClampLimit(limit, DefaultServiceListLimit, MaxServicePageLimit)
	if order != "active" && order != "new" {
		order = "active"
	}
	if offset < 0 {
		offset = 0
	}
	accounts, err := svc.store.ListDirectoryAccounts(ctx, order, localOnly, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("ListDirectoryAccounts: %w", err)
	}
	return accounts, nil
}

// UpdatePreferences updates the user's post preferences.
func (svc *accountService) UpdatePreferences(ctx context.Context, userID string, in UpdatePreferencesInput) (*domain.User, error) {
	switch in.DefaultPrivacy {
	case "public", "unlisted", "private", "direct":
	default:
		return nil, fmt.Errorf("UpdatePreferences: %w", domain.ErrValidation)
	}
	switch in.DefaultQuotePolicy {
	case domain.QuotePolicyPublic, domain.QuotePolicyFollowers, domain.QuotePolicyNobody:
	default:
		return nil, fmt.Errorf("UpdatePreferences: %w", domain.ErrValidation)
	}
	if err := svc.store.UpdateUserPreferences(ctx, store.UpdateUserPreferencesInput{
		UserID:             userID,
		DefaultPrivacy:     in.DefaultPrivacy,
		DefaultSensitive:   in.DefaultSensitive,
		DefaultLanguage:    in.DefaultLanguage,
		DefaultQuotePolicy: in.DefaultQuotePolicy,
	}); err != nil {
		return nil, fmt.Errorf("UpdatePreferences: %w", err)
	}
	u, err := svc.store.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("UpdatePreferences GetUserByID: %w", err)
	}
	return u, nil
}

// ChangeEmail updates the user's email address.
func (svc *accountService) ChangeEmail(ctx context.Context, userID, newEmail string) (*domain.User, error) {
	if err := svc.store.UpdateUserEmail(ctx, userID, newEmail); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return nil, fmt.Errorf("ChangeEmail: email already in use: %w", domain.ErrConflict)
		}
		return nil, fmt.Errorf("ChangeEmail: %w", err)
	}
	u, err := svc.store.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("ChangeEmail GetUserByID: %w", err)
	}
	return u, nil
}

// ChangePassword verifies the current password and replaces it with a new one.
func (svc *accountService) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	u, err := svc.store.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("ChangePassword GetUserByID: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(currentPassword)); err != nil {
		return fmt.Errorf("ChangePassword: %w", domain.ErrForbidden)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("ChangePassword GenerateFromPassword: %w", err)
	}
	if err := svc.store.UpdateUserPassword(ctx, userID, string(hash)); err != nil {
		return fmt.Errorf("ChangePassword: %w", err)
	}
	return nil
}

// DeleteSelf — see interface doc comment.
func (svc *accountService) DeleteSelf(ctx context.Context, userID, currentPassword string) error {
	u, err := svc.store.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("DeleteSelf GetUserByID(%s): %w", userID, err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(currentPassword)); err != nil {
		return fmt.Errorf("DeleteSelf: %w", domain.ErrForbidden)
	}
	if err := deleteLocalAccount(ctx, svc.store, u.AccountID, nil); err != nil {
		return fmt.Errorf("DeleteSelf: %w", err)
	}
	return nil
}

// DeleteLocalAccount — see interface doc comment.
func (svc *accountService) DeleteLocalAccount(ctx context.Context, accountID string) error {
	if err := deleteLocalAccount(ctx, svc.store, accountID, nil); err != nil {
		return fmt.Errorf("DeleteLocalAccount(%s): %w", accountID, err)
	}
	return nil
}

// deleteLocalAccount is the package-level hard-delete flow shared by
// AccountService.DeleteSelf and ModerationService.DeleteAccount. It snapshots
// the account, deletes the row (Postgres CASCADE removes dependent rows),
// emits EventAccountDeleted, and optionally runs a caller-supplied auditFn —
// all in one transaction. Moderation uses auditFn to record the admin action
// atomically; self-service passes nil.
func deleteLocalAccount(ctx context.Context, s store.Store, accountID string, auditFn func(ctx context.Context, tx store.Store) error) error {
	acc, err := s.GetAccountByID(ctx, accountID)
	if err != nil {
		return fmt.Errorf("GetAccountByID(%s): %w", accountID, err)
	}
	if err := requireLocal(acc.IsLocal(), "deleteLocalAccount"); err != nil {
		return err
	}
	return s.WithTx(ctx, func(tx store.Store) error {
		if err := tx.DeleteAccount(ctx, accountID); err != nil {
			return fmt.Errorf("DeleteAccount: %w", err)
		}
		if err := events.EmitEvent(ctx, tx, domain.EventAccountDeleted, "account", accountID, domain.AccountDeletedPayload{
			Account: acc,
			Local:   true,
		}); err != nil {
			return err
		}
		if auditFn != nil {
			if err := auditFn(ctx, tx); err != nil {
				return err
			}
		}
		return nil
	})
}
