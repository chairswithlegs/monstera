//go:build integration

package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
)

func TestIntegration_ModerationStore_Reports(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("CreateReport_GetByID", func(t *testing.T) {
		reporter := createTestLocalAccount(t, s, ctx)
		target := createTestLocalAccount(t, s, ctx)
		reportID := uid.New()

		rpt, err := s.CreateReport(ctx, store.CreateReportInput{
			ID:        reportID,
			AccountID: reporter.ID,
			TargetID:  target.ID,
			Comment:   testutil.StrPtr("spammy behavior"),
			Category:  domain.ReportCategorySpam,
		})
		require.NoError(t, err)
		assert.Equal(t, reportID, rpt.ID)
		assert.Equal(t, domain.ReportStateOpen, rpt.State)

		got, err := s.GetReportByID(ctx, reportID)
		require.NoError(t, err)
		assert.Equal(t, reporter.ID, got.AccountID)
		assert.Equal(t, target.ID, got.TargetID)
		assert.Equal(t, domain.ReportCategorySpam, got.Category)
	})

	t.Run("GetReportByID_not_found", func(t *testing.T) {
		_, err := s.GetReportByID(ctx, "nonexistent_"+uid.New())
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("ListReports", func(t *testing.T) {
		reporter := createTestLocalAccount(t, s, ctx)
		target := createTestLocalAccount(t, s, ctx)
		_, err := s.CreateReport(ctx, store.CreateReportInput{
			ID:        uid.New(),
			AccountID: reporter.ID,
			TargetID:  target.ID,
			Category:  domain.ReportCategoryOther,
		})
		require.NoError(t, err)

		reports, err := s.ListReports(ctx, "", 10, 0)
		require.NoError(t, err)
		assert.NotEmpty(t, reports)
	})

	t.Run("AssignReport", func(t *testing.T) {
		reporter := createTestLocalAccount(t, s, ctx)
		target := createTestLocalAccount(t, s, ctx)
		_, modUser := createTestLocalAccountWithUser(t, s, ctx)
		reportID := uid.New()

		_, err := s.CreateReport(ctx, store.CreateReportInput{
			ID:        reportID,
			AccountID: reporter.ID,
			TargetID:  target.ID,
			Category:  domain.ReportCategoryViolation,
		})
		require.NoError(t, err)

		err = s.AssignReport(ctx, reportID, &modUser.ID)
		require.NoError(t, err)

		got, err := s.GetReportByID(ctx, reportID)
		require.NoError(t, err)
		require.NotNil(t, got.AssignedToID)
		assert.Equal(t, modUser.ID, *got.AssignedToID)
	})

	t.Run("ResolveReport", func(t *testing.T) {
		reporter := createTestLocalAccount(t, s, ctx)
		target := createTestLocalAccount(t, s, ctx)
		reportID := uid.New()

		_, err := s.CreateReport(ctx, store.CreateReportInput{
			ID:        reportID,
			AccountID: reporter.ID,
			TargetID:  target.ID,
			Category:  domain.ReportCategorySpam,
		})
		require.NoError(t, err)

		action := "warned user"
		err = s.ResolveReport(ctx, reportID, &action)
		require.NoError(t, err)

		got, err := s.GetReportByID(ctx, reportID)
		require.NoError(t, err)
		assert.Equal(t, domain.ReportStateResolved, got.State)
		require.NotNil(t, got.ActionTaken)
		assert.Equal(t, "warned user", *got.ActionTaken)
	})

	t.Run("CreateReport_with_statusIDs", func(t *testing.T) {
		reporter := createTestLocalAccount(t, s, ctx)
		target := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, target.ID)
		reportID := uid.New()

		rpt, err := s.CreateReport(ctx, store.CreateReportInput{
			ID:        reportID,
			AccountID: reporter.ID,
			TargetID:  target.ID,
			StatusIDs: []string{st.ID},
			Category:  domain.ReportCategoryViolation,
		})
		require.NoError(t, err)
		assert.Contains(t, rpt.StatusIDs, st.ID)
	})
}

func TestIntegration_ModerationStore_DomainBlocks(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("CreateDomainBlock_ListDomainBlocks_DeleteDomainBlock", func(t *testing.T) {
		blockedDomain := "blocked_" + uid.New()[:8] + ".example"
		dbID := uid.New()

		db, err := s.CreateDomainBlock(ctx, store.CreateDomainBlockInput{
			ID:       dbID,
			Domain:   blockedDomain,
			Severity: domain.DomainBlockSeveritySuspend,
			Reason:   testutil.StrPtr("spam instance"),
		})
		require.NoError(t, err)
		assert.Equal(t, dbID, db.ID)
		assert.Equal(t, blockedDomain, db.Domain)

		blocks, err := s.ListDomainBlocks(ctx)
		require.NoError(t, err)
		found := false
		for _, b := range blocks {
			if b.Domain == blockedDomain {
				found = true
			}
		}
		assert.True(t, found, "domain block not in list")

		err = s.DeleteDomainBlock(ctx, blockedDomain)
		require.NoError(t, err)

		blocks, err = s.ListDomainBlocks(ctx)
		require.NoError(t, err)
		for _, b := range blocks {
			assert.NotEqual(t, blockedDomain, b.Domain, "deleted domain block still in list")
		}
	})
}

func TestIntegration_ModerationStore_ServerFilters(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("CRUD", func(t *testing.T) {
		sfID := uid.New()
		sf, err := s.CreateServerFilter(ctx, store.CreateServerFilterInput{
			ID:        sfID,
			Phrase:    "bad_word_" + uid.New()[:8],
			Scope:     domain.ServerFilterScopePublicTimeline,
			Action:    domain.ServerFilterActionHide,
			WholeWord: true,
		})
		require.NoError(t, err)
		assert.Equal(t, sfID, sf.ID)

		filters, err := s.ListServerFilters(ctx)
		require.NoError(t, err)
		found := false
		for _, f := range filters {
			if f.ID == sfID {
				found = true
			}
		}
		assert.True(t, found, "server filter not in list")

		newPhrase := "updated_" + uid.New()[:8]
		updated, err := s.UpdateServerFilter(ctx, store.UpdateServerFilterInput{
			ID:        sfID,
			Phrase:    newPhrase,
			Scope:     domain.ServerFilterScopeAll,
			Action:    domain.ServerFilterActionWarn,
			WholeWord: false,
		})
		require.NoError(t, err)
		assert.Equal(t, newPhrase, updated.Phrase)
		assert.False(t, updated.WholeWord)

		err = s.DeleteServerFilter(ctx, sfID)
		require.NoError(t, err)
	})
}

func TestIntegration_ModerationStore_Invites(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("CreateInvite_GetByCode_ListByCreator", func(t *testing.T) {
		_, user := createTestLocalAccountWithUser(t, s, ctx)
		inviteID := uid.New()
		code := "invite_" + uid.New()[:8]

		inv, err := s.CreateInvite(ctx, store.CreateInviteInput{
			ID:        inviteID,
			Code:      code,
			CreatedBy: user.ID,
			MaxUses:   intPtr(5),
		})
		require.NoError(t, err)
		assert.Equal(t, inviteID, inv.ID)
		assert.Equal(t, code, inv.Code)
		assert.Equal(t, 0, inv.Uses)

		got, err := s.GetInviteByCode(ctx, code)
		require.NoError(t, err)
		assert.Equal(t, inviteID, got.ID)

		list, err := s.ListInvitesByCreator(ctx, user.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, list)
	})

	t.Run("GetInviteByCode_not_found", func(t *testing.T) {
		_, err := s.GetInviteByCode(ctx, "nonexistent_"+uid.New())
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("IncrementInviteUses", func(t *testing.T) {
		_, user := createTestLocalAccountWithUser(t, s, ctx)
		code := "inc_" + uid.New()[:8]

		_, err := s.CreateInvite(ctx, store.CreateInviteInput{
			ID:        uid.New(),
			Code:      code,
			CreatedBy: user.ID,
		})
		require.NoError(t, err)

		err = s.IncrementInviteUses(ctx, code)
		require.NoError(t, err)

		got, err := s.GetInviteByCode(ctx, code)
		require.NoError(t, err)
		assert.Equal(t, 1, got.Uses)
	})

	t.Run("DeleteInvite", func(t *testing.T) {
		_, user := createTestLocalAccountWithUser(t, s, ctx)
		inviteID := uid.New()
		code := "del_" + uid.New()[:8]

		_, err := s.CreateInvite(ctx, store.CreateInviteInput{
			ID:        inviteID,
			Code:      code,
			CreatedBy: user.ID,
		})
		require.NoError(t, err)

		err = s.DeleteInvite(ctx, inviteID)
		require.NoError(t, err)

		_, err = s.GetInviteByCode(ctx, code)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestIntegration_ModerationStore_AdminActions(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("CreateAdminAction", func(t *testing.T) {
		_, modUser := createTestLocalAccountWithUser(t, s, ctx)
		target := createTestLocalAccount(t, s, ctx)

		err := s.CreateAdminAction(ctx, store.CreateAdminActionInput{
			ID:              uid.New(),
			ModeratorID:     modUser.ID,
			TargetAccountID: &target.ID,
			Action:          "suspend",
			Comment:         testutil.StrPtr("repeated violations"),
		})
		require.NoError(t, err)
	})
}

func TestIntegration_ModerationStore_KnownInstances(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("UpsertKnownInstance_List_Count", func(t *testing.T) {
		instanceDomain := "known_" + uid.New()[:8] + ".example"
		err := s.UpsertKnownInstance(ctx, uid.New(), instanceDomain)
		require.NoError(t, err)

		err = s.UpsertKnownInstance(ctx, uid.New(), instanceDomain)
		require.NoError(t, err)

		instances, err := s.ListKnownInstances(ctx, 100, 0)
		require.NoError(t, err)
		found := false
		for _, inst := range instances {
			if inst.Domain == instanceDomain {
				found = true
			}
		}
		assert.True(t, found, "known instance not in list")

		count, err := s.CountKnownInstances(ctx)
		require.NoError(t, err)
		assert.Greater(t, count, int64(0))
	})
}
