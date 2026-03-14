package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// PendingRegistration is a user awaiting approval (confirmed_at is NULL).
type PendingRegistration struct {
	User    domain.User
	Account domain.Account
}

// RegistrationService handles pending registrations and invite codes.
type RegistrationService interface {
	Confirm(ctx context.Context, userID string) error
	ListPending(ctx context.Context) ([]PendingRegistration, error)
	Approve(ctx context.Context, moderatorID, userID string) error
	Reject(ctx context.Context, moderatorID, userID, reason string) error
	CreateInvite(ctx context.Context, createdByUserID string, maxUses *int, expiresAt *time.Time) (*domain.Invite, error)
	ListInvites(ctx context.Context, createdByUserID string) ([]domain.Invite, error)
	RevokeInvite(ctx context.Context, inviteID string) error
	ValidateInviteCode(ctx context.Context, code string) (*domain.Invite, error)
}

// AccountApprovedMailer sends the account-approved email (optional; can be nil for tests).
type AccountApprovedMailer interface {
	SendAccountApproved(ctx context.Context, to, username, instanceName, instanceURL string) error
}

// RegistrationRejectedMailer sends the registration-rejected email (optional; can be nil for tests).
type RegistrationRejectedMailer interface {
	SendRegistrationRejected(ctx context.Context, to, username, instanceName, reason string) error
}

type registrationService struct {
	store        store.Store
	approvedMail AccountApprovedMailer
	rejectedMail RegistrationRejectedMailer
	instanceURL  string
	instanceName string
}

// NewRegistrationService returns a RegistrationService that uses the given store.
// approvedMail and rejectedMail can be nil to skip sending emails.
func NewRegistrationService(s store.Store, approvedMail AccountApprovedMailer, rejectedMail RegistrationRejectedMailer, instanceURL, instanceName string) RegistrationService {
	return &registrationService{
		store:        s,
		approvedMail: approvedMail,
		rejectedMail: rejectedMail,
		instanceURL:  instanceURL,
		instanceName: instanceName,
	}
}

// Confirm marks a user as confirmed.
func (svc *registrationService) Confirm(ctx context.Context, userID string) error {
	err := svc.store.ConfirmUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("ConfirmUser(%s): %w", userID, err)
	}
	return nil
}

// ListPending returns a list of pending registrations.
func (svc *registrationService) ListPending(ctx context.Context) ([]PendingRegistration, error) {
	users, err := svc.store.GetPendingRegistrations(ctx)
	if err != nil {
		return nil, fmt.Errorf("GetPendingRegistrations: %w", err)
	}
	out := make([]PendingRegistration, 0, len(users))
	for _, u := range users {
		acc, err := svc.store.GetAccountByID(ctx, u.AccountID)
		if err != nil {
			return nil, fmt.Errorf("GetAccountByID(%s): %w", u.AccountID, err)
		}
		out = append(out, PendingRegistration{User: u, Account: *acc})
	}
	return out, nil
}

// Approve confirms a user, records the action, and sends an email notifying the user.
func (svc *registrationService) Approve(ctx context.Context, moderatorID, userID string) error {
	u, err := svc.store.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("GetUserByID(%s): %w", userID, err)
	}
	acc, err := svc.store.GetAccountByID(ctx, u.AccountID)
	if err != nil {
		return fmt.Errorf("GetAccountByID(%s): %w", u.AccountID, err)
	}
	if err := svc.store.ConfirmUser(ctx, userID); err != nil {
		return fmt.Errorf("ConfirmUser(%s): %w", userID, err)
	}
	t := u.AccountID
	if err := svc.store.CreateAdminAction(ctx, store.CreateAdminActionInput{
		ID:              uid.New(),
		ModeratorID:     moderatorID,
		TargetAccountID: &t,
		Action:          AdminActionApproveRegistration,
		Comment:         nil,
		Metadata:        nil,
	}); err != nil {
		return fmt.Errorf("CreateAdminAction: %w", err)
	}
	if svc.approvedMail != nil {
		if err := svc.approvedMail.SendAccountApproved(ctx, u.Email, acc.Username, svc.instanceName, svc.instanceURL); err != nil {
			return fmt.Errorf("SendAccountApproved: %w", err)
		}
	}
	return nil
}

// Reject rejects a user, records the action, and sends an email notifying the user.
func (svc *registrationService) Reject(ctx context.Context, moderatorID, userID, reason string) error {
	u, err := svc.store.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("GetUserByID(%s): %w", userID, err)
	}
	acc, err := svc.store.GetAccountByID(ctx, u.AccountID)
	if err != nil {
		return fmt.Errorf("GetAccountByID(%s): %w", u.AccountID, err)
	}
	if svc.rejectedMail != nil {
		if err := svc.rejectedMail.SendRegistrationRejected(ctx, u.Email, acc.Username, svc.instanceName, reason); err != nil {
			return fmt.Errorf("SendRegistrationRejected: %w", err)
		}
	}
	meta := encodeMetadata(map[string]string{"email_reason": reason})
	if err := svc.store.CreateAdminAction(ctx, store.CreateAdminActionInput{
		ID:              uid.New(),
		ModeratorID:     moderatorID,
		TargetAccountID: &u.AccountID,
		Action:          AdminActionRejectRegistration,
		Comment:         &reason,
		Metadata:        meta,
	}); err != nil {
		return fmt.Errorf("CreateAdminAction: %w", err)
	}
	if err := svc.store.DeleteUser(ctx, userID); err != nil {
		return fmt.Errorf("DeleteUser(%s): %w", userID, err)
	}
	if err := svc.store.DeleteAccount(ctx, u.AccountID); err != nil {
		return fmt.Errorf("DeleteAccount(%s): %w", u.AccountID, err)
	}
	return nil
}

// CreateInvite creates a new invite code.
func (svc *registrationService) CreateInvite(ctx context.Context, createdByUserID string, maxUses *int, expiresAt *time.Time) (*domain.Invite, error) {
	code, err := generateInviteCode()
	if err != nil {
		return nil, fmt.Errorf("generateInviteCode: %w", err)
	}
	var max *int
	if maxUses != nil {
		m := *maxUses
		max = &m
	}
	inv, err := svc.store.CreateInvite(ctx, store.CreateInviteInput{
		ID:        uid.New(),
		Code:      code,
		CreatedBy: createdByUserID,
		MaxUses:   max,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateInvite: %w", err)
	}
	return inv, nil
}

func generateInviteCode() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("rand.Read: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// ListInvites returns a list of invites created by the given user.
func (svc *registrationService) ListInvites(ctx context.Context, createdByUserID string) ([]domain.Invite, error) {
	invites, err := svc.store.ListInvitesByCreator(ctx, createdByUserID)
	if err != nil {
		return nil, fmt.Errorf("ListInvitesByCreator: %w", err)
	}
	return invites, nil
}

// RevokeInvite revokes an invite.
func (svc *registrationService) RevokeInvite(ctx context.Context, inviteID string) error {
	if err := svc.store.DeleteInvite(ctx, inviteID); err != nil {
		return fmt.Errorf("DeleteInvite: %w", err)
	}
	return nil
}

// ValidateInviteCode validates an invite code.
func (svc *registrationService) ValidateInviteCode(ctx context.Context, code string) (*domain.Invite, error) {
	inv, err := svc.store.GetInviteByCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("GetInviteByCode: %w", err)
	}
	if inv.ExpiresAt != nil && inv.ExpiresAt.Before(time.Now()) {
		return nil, domain.ErrNotFound
	}
	if inv.MaxUses != nil && inv.Uses >= *inv.MaxUses {
		return nil, domain.ErrNotFound
	}
	return inv, nil
}
