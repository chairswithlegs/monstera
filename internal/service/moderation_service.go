package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/events"
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
	// DeleteAccount permanently removes a local account and all its data
	// (CASCADE). Emits EventAccountDeleted so federation fans out a
	// Delete{Actor} to remote followers, and records an admin action entry in
	// the same transaction.
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
	// ListDomainBlocksWithPurge returns every block joined with its async
	// purge progress (issue #104). For in-progress purges it also computes
	// AccountsRemaining so the admin UI can render a live countdown without
	// an extra round trip per row.
	ListDomainBlocksWithPurge(ctx context.Context) ([]DomainBlockWithPurgeResult, error)
}

// DomainBlockWithPurgeResult bundles a domain block with its purge row and,
// for in-progress purges, the count of remote accounts still to be
// processed. Mirrors store.DomainBlockWithPurge but carries the computed
// AccountsRemaining so the handler doesn't have to do store math.
type DomainBlockWithPurgeResult struct {
	Block             domain.DomainBlock
	Purge             *domain.DomainBlockPurge
	AccountsRemaining *int64
}

// BlocklistRefresher refreshes the in-memory blocklist cache.
type BlocklistRefresher interface {
	Refresh(ctx context.Context) error
}

type moderationService struct {
	store     store.Store
	blocklist BlocklistRefresher
}

// NewModerationService returns a ModerationService that uses the given store.
func NewModerationService(s store.Store, bl BlocklistRefresher) ModerationService {
	return &moderationService{store: s, blocklist: bl}
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

// SuspendAccount marks a local account as suspended, records an admin audit
// entry, and emits EventAccountSuspended so the federation subscriber fans out
// a Delete{Actor} to remote followers (matching Mastodon's de-facto behaviour
// for moderator suspensions). Remote accounts are rejected; their suspension
// flows through domain blocks instead. The flag flip, audit row, and event
// commit atomically.
func (svc *moderationService) SuspendAccount(ctx context.Context, moderatorID, targetID string) error {
	acc, err := svc.store.GetAccountByID(ctx, targetID)
	if err != nil {
		return fmt.Errorf("GetAccountByID(%s): %w", targetID, err)
	}
	if acc.IsRemote() {
		return fmt.Errorf("SuspendAccount: cannot suspend remote account, use domain blocks: %w", domain.ErrForbidden)
	}
	t := targetID
	if err := svc.store.WithTx(ctx, func(tx store.Store) error {
		if err := tx.SuspendAccount(ctx, targetID); err != nil {
			return fmt.Errorf("SuspendAccount(%s): %w", targetID, err)
		}
		if err := tx.CreateAdminAction(ctx, store.CreateAdminActionInput{
			ID:              uid.New(),
			ModeratorID:     moderatorID,
			TargetAccountID: &t,
			Action:          AdminActionSuspend,
		}); err != nil {
			return fmt.Errorf("CreateAdminAction(suspend): %w", err)
		}
		return events.EmitEvent(ctx, tx, domain.EventAccountSuspended, "account", targetID, domain.AccountSuspendedPayload{
			AccountID: acc.ID,
			APID:      acc.APID,
			Local:     true,
		})
	}); err != nil {
		return err
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
	auditFn := func(ctx context.Context, tx store.Store) error {
		if err := tx.CreateAdminAction(ctx, store.CreateAdminActionInput{
			ID:              uid.New(),
			ModeratorID:     moderatorID,
			TargetAccountID: &t,
			Action:          AdminActionDeleteAccount,
		}); err != nil {
			return fmt.Errorf("CreateAdminAction(delete_account): %w", err)
		}
		return nil
	}
	if err := deleteLocalAccount(ctx, svc.store, targetID, auditFn); err != nil {
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
	// Everything up to blocklist.Refresh runs in one tx so either all of
	// {follows-removed, block row, purge tracker, audit, EventDomainBlockSuspended}
	// commit together, or none do. The event lands in the outbox inside the
	// tx; the subscriber can't race the tracker row because both are
	// committed atomically.
	var block *domain.DomainBlock
	err := svc.store.WithTx(ctx, func(tx store.Store) error {
		if in.Severity == domain.DomainBlockSeveritySuspend {
			if err := tx.DeleteFollowsByDomain(ctx, in.Domain); err != nil {
				return fmt.Errorf("DeleteFollowsByDomain(%s): %w", in.Domain, err)
			}
		}
		b, err := tx.CreateDomainBlock(ctx, store.CreateDomainBlockInput{
			ID:       uid.New(),
			Domain:   in.Domain,
			Severity: in.Severity,
			Reason:   in.Reason,
		})
		if err != nil {
			return fmt.Errorf("CreateDomainBlock(%s): %w", in.Domain, err)
		}
		if in.Severity == domain.DomainBlockSeveritySuspend {
			// Flip domain_suspended=true for every account on the domain
			// atomically with the block row so lookups return 404
			// immediately — no race window between commit and the
			// subscriber catching up.
			if _, err := tx.SetAccountsDomainSuspendedByDomain(ctx, in.Domain, true); err != nil {
				return fmt.Errorf("SetAccountsDomainSuspendedByDomain(%s, true): %w", in.Domain, err)
			}
			if err := tx.CreateDomainBlockPurge(ctx, b.ID, in.Domain); err != nil {
				return fmt.Errorf("CreateDomainBlockPurge(%s): %w", b.ID, err)
			}
			if err := events.EmitEvent(ctx, tx, domain.EventDomainBlockSuspended, "domain_block", b.ID, domain.DomainBlockSuspendedPayload{
				BlockID: b.ID,
				Domain:  in.Domain,
			}); err != nil {
				return fmt.Errorf("EmitEvent(domain_block.suspended): %w", err)
			}
		}
		meta := encodeMetadata(map[string]string{"domain": in.Domain, "severity": in.Severity})
		if err := tx.CreateAdminAction(ctx, store.CreateAdminActionInput{
			ID:              uid.New(),
			ModeratorID:     moderatorID,
			TargetAccountID: nil,
			Action:          AdminActionCreateDomainBlock,
			Comment:         nil,
			Metadata:        meta,
		}); err != nil {
			return fmt.Errorf("CreateAdminAction(create_domain_block): %w", err)
		}
		block = b
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Refresh outside the tx: it touches an in-memory cache, not the DB,
	// and a best-effort retry on failure is already the existing semantic.
	if err := svc.blocklist.Refresh(ctx); err != nil {
		slog.WarnContext(ctx, "blocklist refresh after create domain block failed", slog.Any("error", err))
	}
	return block, nil
}

func (svc *moderationService) DeleteDomainBlock(ctx context.Context, moderatorID, domainName string) error {
	// Wrap in a tx so the block removal, the domain-suspend reversal, and
	// the audit entry all commit together. accounts.suspended is NEVER
	// touched — that flag reflects individual suspensions (moderator
	// action or federation Delete{Person}) and must survive the unblock.
	err := svc.store.WithTx(ctx, func(tx store.Store) error {
		if err := tx.DeleteDomainBlock(ctx, domainName); err != nil {
			return fmt.Errorf("DeleteDomainBlock(%s): %w", domainName, err)
		}
		if _, err := tx.SetAccountsDomainSuspendedByDomain(ctx, domainName, false); err != nil {
			return fmt.Errorf("SetAccountsDomainSuspendedByDomain(%s, false): %w", domainName, err)
		}
		meta := encodeMetadata(map[string]string{"domain": domainName})
		if err := tx.CreateAdminAction(ctx, store.CreateAdminActionInput{
			ID:              uid.New(),
			ModeratorID:     moderatorID,
			TargetAccountID: nil,
			Action:          AdminActionRemoveDomainBlock,
			Comment:         nil,
			Metadata:        meta,
		}); err != nil {
			return fmt.Errorf("CreateAdminAction(remove_domain_block): %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := svc.blocklist.Refresh(ctx); err != nil {
		slog.WarnContext(ctx, "blocklist refresh after delete domain block failed", slog.Any("error", err))
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

func (svc *moderationService) ListDomainBlocksWithPurge(ctx context.Context) ([]DomainBlockWithPurgeResult, error) {
	rows, err := svc.store.ListDomainBlocksWithPurge(ctx)
	if err != nil {
		return nil, fmt.Errorf("ListDomainBlocksWithPurge: %w", err)
	}
	out := make([]DomainBlockWithPurgeResult, 0, len(rows))
	for i := range rows {
		item := DomainBlockWithPurgeResult{
			Block: rows[i].Block,
			Purge: rows[i].Purge,
		}
		if rows[i].Purge != nil && rows[i].Purge.CompletedAt == nil {
			cursor := ""
			if rows[i].Purge.Cursor != nil {
				cursor = *rows[i].Purge.Cursor
			}
			n, err := svc.store.CountRemoteAccountsByDomainAfterCursor(ctx, rows[i].Block.Domain, cursor)
			if err != nil {
				return nil, fmt.Errorf("CountRemoteAccountsByDomainAfterCursor(%s): %w", rows[i].Block.Domain, err)
			}
			remaining := n
			item.AccountsRemaining = &remaining
		}
		out = append(out, item)
	}
	return out, nil
}
