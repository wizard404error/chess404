package platform

import (
	"path/filepath"
	"testing"
)

func TestModerationStoreBlocksReportsAndOverview(t *testing.T) {
	store, err := NewSQLiteModerationStore(filepath.Join(t.TempDir(), "moderation.sqlite"))
	if err != nil {
		t.Fatalf("expected moderation store to initialize, got %v", err)
	}
	defer func() { _ = store.Close() }()

	if _, err := store.BlockAccount("acct_alpha", "acct_alpha", "self"); err != ErrInvalidAccountBlock {
		t.Fatalf("expected self block to fail, got %v", err)
	}

	block, err := store.BlockAccount("acct_alpha", "acct_beta", "spam challenge")
	if err != nil {
		t.Fatalf("expected block create to succeed, got %v", err)
	}
	if !store.IsBlocked("acct_alpha", "acct_beta") || !store.IsBlockedEitherDirection("acct_alpha", "acct_beta") {
		t.Fatalf("expected block to be visible in store")
	}

	updated, err := store.BlockAccount("acct_alpha", "acct_beta", "harassment")
	if err != nil {
		t.Fatalf("expected repeated block to update, got %v", err)
	}
	if updated.BlockID != block.BlockID || updated.Reason != "harassment" {
		t.Fatalf("expected existing block to update, got %#v", updated)
	}

	report, err := store.CreateReport("acct_alpha", "acct_beta", "spam", "kept sending bad invites")
	if err != nil {
		t.Fatalf("expected report create to succeed, got %v", err)
	}
	if report.Category != PlayerReportCategorySpam || report.Status != PlayerReportStatusOpen {
		t.Fatalf("unexpected player report %#v", report)
	}

	overview := store.ListOverview("acct_alpha")
	if len(overview.OutgoingBlocks) != 1 || len(overview.SubmittedReports) != 1 || len(overview.IncomingBlocks) != 0 {
		t.Fatalf("unexpected blocker overview %#v", overview)
	}
	incomingOverview := store.ListOverview("acct_beta")
	if len(incomingOverview.IncomingBlocks) != 1 {
		t.Fatalf("expected target to see incoming block, got %#v", incomingOverview)
	}

	if err := store.UnblockAccount("acct_alpha", "acct_beta"); err != nil {
		t.Fatalf("expected unblock to succeed, got %v", err)
	}
	if store.IsBlockedEitherDirection("acct_alpha", "acct_beta") {
		t.Fatalf("expected block to clear after unblock")
	}
	if err := store.UnblockAccount("acct_alpha", "acct_beta"); err != ErrAccountBlockNotFound {
		t.Fatalf("expected missing unblock to return not found, got %v", err)
	}

	if _, _, err := store.ResolveReport("acct_mod", report.ReportID, "review", "investigating"); err != nil {
		t.Fatalf("expected report review to succeed, got %v", err)
	}
	if _, _, err := store.ResolveReport("acct_mod", report.ReportID, "resolved_actioned", "warned account"); err != nil {
		t.Fatalf("expected report resolution to succeed, got %v", err)
	}

	adminOverview := store.ListAdminOverview(10, "")
	if len(adminOverview.Reports) != 1 {
		t.Fatalf("expected one report in admin overview, got %#v", adminOverview)
	}
	if adminOverview.Reports[0].Status != PlayerReportStatusResolvedActioned || adminOverview.Reports[0].ReviewedByAccountID != "acct_mod" || adminOverview.Reports[0].ResolutionNote != "warned account" {
		t.Fatalf("unexpected resolved report %#v", adminOverview.Reports[0])
	}
	if len(adminOverview.RecentActions) != 2 {
		t.Fatalf("expected two moderation actions, got %#v", adminOverview)
	}
	openOnly := store.ListAdminOverview(10, PlayerReportStatusOpen)
	if len(openOnly.Reports) != 0 {
		t.Fatalf("expected no open reports after resolution, got %#v", openOnly)
	}
}
