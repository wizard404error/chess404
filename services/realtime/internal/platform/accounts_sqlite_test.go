package platform

import (
	"path/filepath"
	"testing"

	"github.com/chess404/realtime/internal/contracts"
)

func TestSQLiteAccountStoreClaimGuestPersistsAndReloads(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "accounts.sqlite")
	store, err := NewSQLiteAccountStore(storePath)
	if err != nil {
		t.Fatalf("expected sqlite account store to initialize, got %v", err)
	}
	defer func() { _ = store.Close() }()

	session, err := store.ClaimGuest(GuestProfile{GuestID: "guest_white"}, "aurora_fox")
	if err != nil {
		t.Fatalf("expected sqlite account claim to succeed, got %v", err)
	}
	if session.Account.AccountID == "" || session.Account.Handle != "aurora_fox" || session.SessionToken == "" || session.ExpiresAt.IsZero() {
		t.Fatalf("unexpected sqlite account session %#v", session)
	}

	reloaded, err := NewSQLiteAccountStore(storePath)
	if err != nil {
		t.Fatalf("expected sqlite account store reload to succeed, got %v", err)
	}
	defer func() { _ = reloaded.Close() }()

	resumed, err := reloaded.ResumeAccount(session.Account.AccountID, session.SessionToken)
	if err != nil {
		t.Fatalf("expected sqlite account resume to succeed, got %v", err)
	}
	if resumed.Account.AccountID != session.Account.AccountID || resumed.SessionToken != session.SessionToken {
		t.Fatalf("expected sqlite account round-trip, got %#v vs %#v", session, resumed)
	}
}

func TestSQLiteAccountStoreStatsReflectSessionsAndLinks(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "accounts.sqlite")
	store, err := NewSQLiteAccountStore(storePath)
	if err != nil {
		t.Fatalf("expected sqlite account store to initialize, got %v", err)
	}
	defer func() { _ = store.Close() }()

	if _, err := store.ClaimGuest(GuestProfile{GuestID: "guest_white"}, "aurora_fox"); err != nil {
		t.Fatalf("expected first sqlite account claim to succeed, got %v", err)
	}
	if _, err := store.ClaimGuest(GuestProfile{GuestID: "guest_black"}, "night_owl"); err != nil {
		t.Fatalf("expected second sqlite account claim to succeed, got %v", err)
	}

	stats := store.Stats()
	if stats.AccountCount != 2 || stats.LinkedGuestCount != 2 || stats.ActiveSessionCount != 2 {
		t.Fatalf("unexpected sqlite account stats %#v", stats)
	}
}

func TestSQLiteAccountStoreFinalizeMatchPersistsDirectStats(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "accounts.sqlite")
	store, err := NewSQLiteAccountStore(storePath)
	if err != nil {
		t.Fatalf("expected sqlite account store to initialize, got %v", err)
	}
	defer func() { _ = store.Close() }()

	whiteSession, err := store.ClaimGuest(GuestProfile{GuestID: "guest_white", Rating: 1220}, "aurora_white")
	if err != nil {
		t.Fatalf("expected white sqlite account claim to succeed, got %v", err)
	}
	blackSession, err := store.ClaimGuest(GuestProfile{GuestID: "guest_black", Rating: 1180}, "aurora_black")
	if err != nil {
		t.Fatalf("expected black sqlite account claim to succeed, got %v", err)
	}
	if _, _, err := store.SyncGuestStats(GuestProfile{GuestID: "guest_white", Rating: 1220}); err != nil {
		t.Fatalf("expected white sqlite account sync to succeed, got %v", err)
	}
	if _, _, err := store.SyncGuestStats(GuestProfile{GuestID: "guest_black", Rating: 1180}); err != nil {
		t.Fatalf("expected black sqlite account sync to succeed, got %v", err)
	}

	white, black, changed, err := store.FinalizeMatch("sqlite_account_match", whiteSession.Account.AccountID, blackSession.Account.AccountID, "draw", "casual", contracts.MatchModeHiddenCards)
	if err != nil {
		t.Fatalf("expected sqlite account finalization to succeed, got %v", err)
	}
	if !changed || white.Rating != 1220 || black.Rating != 1180 || white.Draws != 1 || black.Draws != 1 {
		t.Fatalf("unexpected sqlite account finalization result %#v %#v changed=%v", white, black, changed)
	}

	reloaded, err := NewSQLiteAccountStore(storePath)
	if err != nil {
		t.Fatalf("expected sqlite account store reload to succeed, got %v", err)
	}
	defer func() { _ = reloaded.Close() }()

	whiteReloaded, ok := reloaded.GetAccount(whiteSession.Account.AccountID)
	if !ok {
		t.Fatalf("expected reloaded white account to exist")
	}
	blackReloaded, ok := reloaded.GetAccount(blackSession.Account.AccountID)
	if !ok {
		t.Fatalf("expected reloaded black account to exist")
	}
	if whiteReloaded.Draws != 1 || blackReloaded.Draws != 1 || whiteReloaded.MatchesPlayed != 1 || blackReloaded.MatchesPlayed != 1 {
		t.Fatalf("expected sqlite account stats to persist, got %#v %#v", whiteReloaded, blackReloaded)
	}
	if len(whiteReloaded.RatingHistory) != 1 || len(blackReloaded.RatingHistory) != 1 {
		t.Fatalf("expected sqlite account history to persist, got %#v %#v", whiteReloaded.RatingHistory, blackReloaded.RatingHistory)
	}
	if whiteReloaded.RatingHistory[0].MatchID != "sqlite_account_match" || whiteReloaded.RatingHistory[0].Result != "draw" || whiteReloaded.RatingHistory[0].Delta != 0 || whiteReloaded.RatingHistory[0].Queue != "casual" || whiteReloaded.RatingHistory[0].ModeID != contracts.MatchModeHiddenCards {
		t.Fatalf("unexpected persisted white sqlite history %#v", whiteReloaded.RatingHistory[0])
	}
}

func TestSQLiteAccountStorePasswordLoginAndLogout(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "accounts.sqlite")
	store, err := NewSQLiteAccountStore(storePath)
	if err != nil {
		t.Fatalf("expected sqlite account store to initialize, got %v", err)
	}
	defer func() { _ = store.Close() }()

	claimed, err := store.ClaimGuest(GuestProfile{GuestID: "guest_auth"}, "aurora_auth")
	if err != nil {
		t.Fatalf("expected sqlite account claim to succeed, got %v", err)
	}
	enabled, err := store.EnablePasswordLogin(claimed.Account.AccountID, claimed.SessionToken, "aurora@example.com", "Swordfish88")
	if err != nil {
		t.Fatalf("expected sqlite password setup to succeed, got %v", err)
	}
	if enabled.Account.AccountID != claimed.Account.AccountID {
		t.Fatalf("unexpected sqlite enabled session %#v", enabled)
	}

	loggedIn, err := store.LoginWithPassword("aurora@example.com", "Swordfish88")
	if err != nil {
		t.Fatalf("expected sqlite email login to succeed, got %v", err)
	}
	if loggedIn.Account.AccountID != claimed.Account.AccountID {
		t.Fatalf("unexpected sqlite login session %#v", loggedIn)
	}

	if err := store.LogoutAccount(loggedIn.Account.AccountID, loggedIn.SessionToken); err != nil {
		t.Fatalf("expected sqlite logout to succeed, got %v", err)
	}
	if _, err := store.ResumeAccount(loggedIn.Account.AccountID, loggedIn.SessionToken); err != ErrUnauthorizedAccountSession {
		t.Fatalf("expected sqlite logged-out session to be invalid, got %v", err)
	}
}
