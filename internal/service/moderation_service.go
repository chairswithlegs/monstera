package service

import (
	"context"
	"fmt"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

const (
	AdminActionSuspend             = "suspend"
	AdminActionUnsuspend           = "unsuspend"
	AdminActionSilence             = "silence"
	AdminActionUnsilence           = "unsilence"
	AdminActionDeleteAccount       = "delete_account"
	AdminActionSetRole             = "set_role"
	AdminActionApproveRegistration = "approve_registration"
	AdminActionRejectRegistration  = "reject_registration"
	AdminActionResolveReport       = "resolve_report"
	AdminActionAssignReport        = "assign_report"
	AdminActionCreateDomainBlock   = "create_domain_block"
	AdminActionRemoveDomainBlock   = "remove_domain_block"
)

// ModerationService provides moderation operations with audit logging.
type ModerationService interface {
	SuspendAccount(ctx context.Context, moderatorID, targetID string) error
	UnsuspendAccount(ctx context.Context, moderatorID, targetID string) error
	SilenceAccount(ctx context.Context, moderatorID, targetID string) error
	UnsilenceAccount(ctx context.Context, moderatorID, targetID string) error
	SetUserRole(ctx context.Context, moderatorID, targetUserID, role string) error
	DeleteAccount(ctx context.Context, moderatorID, targetID string) error
	CreateReport(ctx context.Context, in CreateReportInput) (*domain.Report, error)
	ListReports(ctx context.Context, state string, limit, offset int) ([]domain.Report, error)
	CountReportsByState(ctx context.Context, state string) (int64, error)
	GetReport(ctx context.Context, id string) (*domain.Report, error)
	AssignReport(ctx context.Context, moderatorID, reportID string, assigneeID *string) error
	ResolveReport(ctx context.Context, moderatorID, reportID string, resolution string) error
	CreateDomainBlock(ctx context.Context, moderatorID string, in CreateDomainBlockInput) (*domain.DomainBlock, error)
	DeleteDomainBlock(ctx context.Context, moderatorID, domain string) error
	ListDomainBlocks(ctx context.Context) ([]domain.DomainBlock, error)
}

type moderationService struct {
	store store.Store
}

// NewModerationService returns a ModerationService that uses the given store.
func NewModerationService(s store.Store) ModerationService {
	return &moderationService{store: s}
}

// CreateReportInput is the input for creating a report.
type CreateReportInput struct {
	AccountID string
	TargetID  string
	StatusIDs []string
	Comment   *string
	Category  string
}

// CreateDomainBlockInput is the input for creating a domain block (service layer).
type CreateDomainBlockInput struct {
	Domain   string
	Severity string
	Reason   *string
}

//nolint:unparam // comment is part of the audit API for future use
func (svc *moderationService) writeAdminAction(ctx context.Context, moderatorID string, targetAccountID *string, action string, comment *string, metadata []byte) error {
	if err := svc.store.CreateAdminAction(ctx, store.CreateAdminActionInput{
		ID:              uid.New(),
		ModeratorID:     moderatorID,
		TargetAccountID: targetAccountID,
		Action:          action,
		Comment:         comment,
		Metadata:        metadata,
	}); err != nil {
		return fmt.Errorf("CreateAdminAction: %w", err)
	}
	return nil
}

func (svc *moderationService) SuspendAccount(ctx context.Context, moderatorID, targetID string) error {
	acc, err := svc.store.GetAccountByID(ctx, targetID)
	if err != nil {
		return fmt.Errorf("GetAccountByID(%s): %w", targetID, err)
	}
	if acc.IsRemote() {
		return fmt.Errorf("SuspendAccount: cannot suspend remote account, use domain blocks: %w", domain.ErrForbidden)
	}
	if err := svc.store.SuspendAccount(ctx, targetID); err != nil {
		return fmt.Errorf("SuspendAccount(%s): %w", targetID, err)
	}
	t := targetID
	if err := svc.writeAdminAction(ctx, moderatorID, &t, AdminActionSuspend, nil, nil); err != nil {
		return fmt.Errorf("CreateAdminAction(suspend): %w", err)
	}
	return nil
}

func (svc *moderationService) UnsuspendAccount(ctx context.Context, moderatorID, targetID string) error {
	if err := svc.store.UnsuspendAccount(ctx, targetID); err != nil {
		return fmt.Errorf("UnsuspendAccount(%s): %w", targetID, err)
	}
	t := targetID
	if err := svc.writeAdminAction(ctx, moderatorID, &t, AdminActionUnsuspend, nil, nil); err != nil {
		return fmt.Errorf("CreateAdminAction(unsuspend): %w", err)
	}
	return nil
}

func (svc *moderationService) SilenceAccount(ctx context.Context, moderatorID, targetID string) error {
	acc, err := svc.store.GetAccountByID(ctx, targetID)
	if err != nil {
		return fmt.Errorf("GetAccountByID(%s): %w", targetID, err)
	}
	if acc.IsRemote() {
		return fmt.Errorf("SilenceAccount: cannot silence remote account, use domain blocks: %w", domain.ErrForbidden)
	}
	if err := svc.store.SilenceAccount(ctx, targetID); err != nil {
		return fmt.Errorf("SilenceAccount(%s): %w", targetID, err)
	}
	t := targetID
	if err := svc.writeAdminAction(ctx, moderatorID, &t, AdminActionSilence, nil, nil); err != nil {
		return fmt.Errorf("CreateAdminAction(silence): %w", err)
	}
	return nil
}

func (svc *moderationService) UnsilenceAccount(ctx context.Context, moderatorID, targetID string) error {
	if err := svc.store.UnsilenceAccount(ctx, targetID); err != nil {
		return fmt.Errorf("UnsilenceAccount(%s): %w", targetID, err)
	}
	t := targetID
	if err := svc.writeAdminAction(ctx, moderatorID, &t, AdminActionUnsilence, nil, nil); err != nil {
		return fmt.Errorf("CreateAdminAction(unsilence): %w", err)
	}
	return nil
}

func (svc *moderationService) SetUserRole(ctx context.Context, moderatorID, targetUserID, role string) error {
	u, err := svc.store.GetUserByID(ctx, targetUserID)
	if err != nil {
		return fmt.Errorf("GetUserByID(%s): %w", targetUserID, err)
	}
	oldRole := u.Role
	if err := svc.store.UpdateUserRole(ctx, targetUserID, role); err != nil {
		return fmt.Errorf("UpdateUserRole(%s): %w", targetUserID, err)
	}
	meta := encodeMetadata(map[string]string{"old_role": oldRole, "new_role": role})
	t := u.AccountID
	if err := svc.writeAdminAction(ctx, moderatorID, &t, AdminActionSetRole, nil, meta); err != nil {
		return fmt.Errorf("CreateAdminAction(set_role): %w", err)
	}
	return nil
}

func (svc *moderationService) DeleteAccount(ctx context.Context, moderatorID, targetID string) error {
	t := targetID
	if err := svc.writeAdminAction(ctx, moderatorID, &t, AdminActionDeleteAccount, nil, nil); err != nil {
		return fmt.Errorf("CreateAdminAction(delete_account): %w", err)
	}
	user, err := svc.store.GetUserByAccountID(ctx, targetID)
	if err == nil {
		if err := svc.store.DeleteUser(ctx, user.ID); err != nil {
			return fmt.Errorf("DeleteUser(%s): %w", user.ID, err)
		}
	}
	if err := svc.store.DeleteAccount(ctx, targetID); err != nil {
		return fmt.Errorf("DeleteAccount(%s): %w", targetID, err)
	}
	return nil
}

func (svc *moderationService) CreateReport(ctx context.Context, in CreateReportInput) (*domain.Report, error) {
	r, err := svc.store.CreateReport(ctx, store.CreateReportInput{
		ID:        uid.New(),
		AccountID: in.AccountID,
		TargetID:  in.TargetID,
		StatusIDs: in.StatusIDs,
		Comment:   in.Comment,
		Category:  in.Category,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateReport: %w", err)
	}
	return r, nil
}

func (svc *moderationService) ListReports(ctx context.Context, state string, limit, offset int) ([]domain.Report, error) {
	reports, err := svc.store.ListReports(ctx, state, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("ListReports: %w", err)
	}
	return reports, nil
}

func (svc *moderationService) CountReportsByState(ctx context.Context, state string) (int64, error) {
	n, err := svc.store.CountReportsByState(ctx, state)
	if err != nil {
		return 0, fmt.Errorf("CountReportsByState(%s): %w", state, err)
	}
	return n, nil
}

func (svc *moderationService) GetReport(ctx context.Context, id string) (*domain.Report, error) {
	r, err := svc.store.GetReportByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("GetReportByID(%s): %w", id, err)
	}
	return r, nil
}

func (svc *moderationService) AssignReport(ctx context.Context, moderatorID, reportID string, assigneeID *string) error {
	if err := svc.store.AssignReport(ctx, reportID, assigneeID); err != nil {
		return fmt.Errorf("AssignReport(%s): %w", reportID, err)
	}
	meta := encodeMetadata(map[string]string{"report_id": reportID})
	if err := svc.writeAdminAction(ctx, moderatorID, nil, AdminActionAssignReport, nil, meta); err != nil {
		return fmt.Errorf("CreateAdminAction(assign_report): %w", err)
	}
	return nil
}

func (svc *moderationService) ResolveReport(ctx context.Context, moderatorID, reportID string, resolution string) error {
	if err := svc.store.ResolveReport(ctx, reportID, &resolution); err != nil {
		return fmt.Errorf("ResolveReport(%s): %w", reportID, err)
	}
	meta := encodeMetadata(map[string]string{"report_id": reportID, "resolution": resolution})
	if err := svc.writeAdminAction(ctx, moderatorID, nil, AdminActionResolveReport, nil, meta); err != nil {
		return fmt.Errorf("CreateAdminAction(resolve_report): %w", err)
	}
	return nil
}

func (svc *moderationService) CreateDomainBlock(ctx context.Context, moderatorID string, in CreateDomainBlockInput) (*domain.DomainBlock, error) {
	if in.Severity == domain.DomainBlockSeveritySuspend {
		if err := svc.store.DeleteFollowsByDomain(ctx, in.Domain); err != nil {
			return nil, fmt.Errorf("DeleteFollowsByDomain(%s): %w", in.Domain, err)
		}
	}
	block, err := svc.store.CreateDomainBlock(ctx, store.CreateDomainBlockInput{
		ID:       uid.New(),
		Domain:   in.Domain,
		Severity: in.Severity,
		Reason:   in.Reason,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateDomainBlock(%s): %w", in.Domain, err)
	}
	meta := encodeMetadata(map[string]string{"domain": in.Domain, "severity": in.Severity})
	if err := svc.writeAdminAction(ctx, moderatorID, nil, AdminActionCreateDomainBlock, nil, meta); err != nil {
		return nil, fmt.Errorf("CreateAdminAction(create_domain_block): %w", err)
	}
	return block, nil
}

func (svc *moderationService) DeleteDomainBlock(ctx context.Context, moderatorID, domain string) error {
	if err := svc.store.DeleteDomainBlock(ctx, domain); err != nil {
		return fmt.Errorf("DeleteDomainBlock(%s): %w", domain, err)
	}
	meta := encodeMetadata(map[string]string{"domain": domain})
	if err := svc.writeAdminAction(ctx, moderatorID, nil, AdminActionRemoveDomainBlock, nil, meta); err != nil {
		return fmt.Errorf("CreateAdminAction(remove_domain_block): %w", err)
	}
	return nil
}

func (svc *moderationService) ListDomainBlocks(ctx context.Context) ([]domain.DomainBlock, error) {
	blocks, err := svc.store.ListDomainBlocks(ctx)
	if err != nil {
		return nil, fmt.Errorf("ListDomainBlocks: %w", err)
	}
	return blocks, nil
}
