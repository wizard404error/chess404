package platform

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/chess404/realtime/internal/contracts"
)

func TestAccountStoreClaimGuestPersistsAndReloads(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "accounts.json")
	store, err := NewAccountStore(storePath)
	if err != nil {
		t.Fatalf("expected account store to initialize, got %v", err)
	}
	defer func() { _ = store.Close() }()

	session, err := store.ClaimGuest(GuestProfile{GuestID: "guest_white"}, "aurora_fox")
	if err != nil {
		t.Fatalf("expected account claim to succeed, got %v", err)
	}
	if session.Account.AccountID == "" || session.Account.Handle != "aurora_fox" || session.SessionToken == "" || session.ExpiresAt.IsZero() {
		t.Fatalf("unexpected account session %#v", session)
	}

	reloaded, err := NewAccountStore(storePath)
	if err != nil {
		t.Fatalf("expected account store reload to succeed, got %v", err)
	}
	defer func() { _ = reloaded.Close() }()

	resumed, err := reloaded.ResumeAccount(session.Account.AccountID, session.SessionToken)
	if err != nil {
		t.Fatalf("expected account resume to succeed, got %v", err)
	}
	if resumed.Account.AccountID != session.Account.AccountID || resumed.SessionToken != session.SessionToken {
		t.Fatalf("expected persisted account session to round-trip, got %#v vs %#v", session, resumed)
	}
}

func TestAccountStoreRejectsDuplicateHandle(t *testing.T) {
	store, err := NewAccountStore("")
	if err != nil {
		t.Fatalf("expected in-memory account store to initialize, got %v", err)
	}

	if _, err := store.ClaimGuest(GuestProfile{GuestID: "guest_white"}, "aurora_fox"); err != nil {
		t.Fatalf("expected first account claim to succeed, got %v", err)
	}
	if _, err := store.ClaimGuest(GuestProfile{GuestID: "guest_black"}, "aurora_fox"); err != ErrAccountHandleTaken {
		t.Fatalf("expected duplicate handle to be rejected, got %v", err)
	}
}

func TestAccountStoreReturnsExistingAccountForLinkedGuest(t *testing.T) {
	store, err := NewAccountStore("")
	if err != nil {
		t.Fatalf("expected in-memory account store to initialize, got %v", err)
	}

	first, err := store.ClaimGuest(GuestProfile{GuestID: "guest_white"}, "aurora_fox")
	if err != nil {
		t.Fatalf("expected first account claim to succeed, got %v", err)
	}
	second, err := store.ClaimGuest(GuestProfile{GuestID: "guest_white"}, "aurora_fox")
	if err != nil {
		t.Fatalf("expected repeated account claim to succeed, got %v", err)
	}
	if first.Account.AccountID != second.Account.AccountID {
		t.Fatalf("expected linked guest to reuse the same account, got %#v vs %#v", first, second)
	}
}

func TestAccountStoreGetAccountByGuest(t *testing.T) {
	store, err := NewAccountStore("")
	if err != nil {
		t.Fatalf("expected in-memory account store to initialize, got %v", err)
	}

	session, err := store.ClaimGuest(GuestProfile{GuestID: "guest_white"}, "aurora_fox")
	if err != nil {
		t.Fatalf("expected account claim to succeed, got %v", err)
	}

	account, ok := store.GetAccountByGuest("guest_white")
	if !ok {
		t.Fatalf("expected linked guest lookup to succeed")
	}
	if account.AccountID != session.Account.AccountID || account.Handle != "aurora_fox" {
		t.Fatalf("unexpected account lookup result %#v", account)
	}
}

func TestAccountStoreListAccountsSortsByLastSeen(t *testing.T) {
	store, err := NewAccountStore("")
	if err != nil {
		t.Fatalf("expected in-memory account store to initialize, got %v", err)
	}

	first, err := store.ClaimGuest(GuestProfile{GuestID: "guest_first"}, "aurora_first")
	if err != nil {
		t.Fatalf("expected first account claim to succeed, got %v", err)
	}
	second, err := store.ClaimGuest(GuestProfile{GuestID: "guest_second"}, "aurora_second")
	if err != nil {
		t.Fatalf("expected second account claim to succeed, got %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if _, err := store.ResumeAccount(first.Account.AccountID, first.SessionToken); err != nil {
		t.Fatalf("expected first account resume to succeed, got %v", err)
	}

	accounts := store.ListAccounts(10)
	if len(accounts) != 2 {
		t.Fatalf("expected two accounts, got %#v", accounts)
	}
	if accounts[0].AccountID != first.Account.AccountID || accounts[1].AccountID != second.Account.AccountID {
		t.Fatalf("expected recently touched account first, got %#v", accounts)
	}
}

func TestAccountStoreFinalizeMatchUpdatesDirectStats(t *testing.T) {
	store, err := NewAccountStore("")
	if err != nil {
		t.Fatalf("expected in-memory account store to initialize, got %v", err)
	}

	whiteSession, err := store.ClaimGuest(GuestProfile{GuestID: "guest_white", Rating: 1230, MatchesPlayed: 5, Wins: 3, Losses: 1, Draws: 1}, "aurora_white")
	if err != nil {
		t.Fatalf("expected white account claim to succeed, got %v", err)
	}
	blackSession, err := store.ClaimGuest(GuestProfile{GuestID: "guest_black", Rating: 1190, MatchesPlayed: 4, Wins: 2, Losses: 2}, "aurora_black")
	if err != nil {
		t.Fatalf("expected black account claim to succeed, got %v", err)
	}
	if _, _, err := store.SyncGuestStats(GuestProfile{GuestID: "guest_white", Rating: 1230, MatchesPlayed: 5, Wins: 3, Losses: 1, Draws: 1}); err != nil {
		t.Fatalf("expected white account sync to succeed, got %v", err)
	}
	if _, _, err := store.SyncGuestStats(GuestProfile{GuestID: "guest_black", Rating: 1190, MatchesPlayed: 4, Wins: 2, Losses: 2}); err != nil {
		t.Fatalf("expected black account sync to succeed, got %v", err)
	}

	white, black, changed, err := store.FinalizeMatch("match_account_direct", whiteSession.Account.AccountID, blackSession.Account.AccountID, "white", "rated", contracts.MatchModeHiddenCards)
	if err != nil {
		t.Fatalf("expected account finalization to succeed, got %v", err)
	}
	if !changed {
		t.Fatalf("expected first account finalization to apply")
	}
	if white.Rating != 1246 || white.MatchesPlayed != 6 || white.Wins != 4 {
		t.Fatalf("unexpected white account after finalization %#v", white)
	}
	if black.Rating != 1174 || black.MatchesPlayed != 5 || black.Losses != 3 {
		t.Fatalf("unexpected black account after finalization %#v", black)
	}
	if len(white.RatingHistory) != 1 || len(black.RatingHistory) != 1 {
		t.Fatalf("expected direct account history entries, got %#v %#v", white.RatingHistory, black.RatingHistory)
	}
	if white.RatingHistory[0].MatchID != "match_account_direct" || white.RatingHistory[0].Result != "win" || white.RatingHistory[0].Delta != 16 || white.RatingHistory[0].RatingAfter != 1246 || white.RatingHistory[0].Queue != "rated" || white.RatingHistory[0].ModeID != contracts.MatchModeHiddenCards {
		t.Fatalf("unexpected white account history %#v", white.RatingHistory[0])
	}
	if black.RatingHistory[0].MatchID != "match_account_direct" || black.RatingHistory[0].Result != "loss" || black.RatingHistory[0].Delta != -16 || black.RatingHistory[0].RatingAfter != 1174 || black.RatingHistory[0].Queue != "rated" || black.RatingHistory[0].ModeID != contracts.MatchModeHiddenCards {
		t.Fatalf("unexpected black account history %#v", black.RatingHistory[0])
	}
}

func TestAccountStoreSyncGuestStatsSeedsLegacyAccounts(t *testing.T) {
	store, err := NewAccountStore("")
	if err != nil {
		t.Fatalf("expected in-memory account store to initialize, got %v", err)
	}

	session, err := store.ClaimGuest(GuestProfile{GuestID: "guest_legacy"}, "aurora_legacy")
	if err != nil {
		t.Fatalf("expected account claim to succeed, got %v", err)
	}
	account := store.accounts[session.Account.AccountID]
	account.Rating = 0
	account.MatchesPlayed = 0
	account.Wins = 0
	account.Losses = 0
	account.Draws = 0
	store.accounts[session.Account.AccountID] = account

	synced, ok, err := store.SyncGuestStats(GuestProfile{
		GuestID:       "guest_legacy",
		Rating:        1275,
		MatchesPlayed: 8,
		Wins:          5,
		Losses:        2,
		Draws:         1,
	})
	if err != nil {
		t.Fatalf("expected guest sync to succeed, got %v", err)
	}
	if !ok {
		t.Fatalf("expected linked legacy account to sync")
	}
	if synced.Rating != 1275 || synced.MatchesPlayed != 8 || synced.Wins != 5 || synced.Losses != 2 || synced.Draws != 1 {
		t.Fatalf("unexpected synced legacy account %#v", synced)
	}
}

func TestAccountStorePasswordLoginAndLogout(t *testing.T) {
	store, err := NewAccountStore("")
	if err != nil {
		t.Fatalf("expected in-memory account store to initialize, got %v", err)
	}

	claimed, err := store.ClaimGuest(GuestProfile{GuestID: "guest_white"}, "aurora_auth")
	if err != nil {
		t.Fatalf("expected account claim to succeed, got %v", err)
	}
	enabled, err := store.EnablePasswordLogin(claimed.Account.AccountID, claimed.SessionToken, "aurora@example.com", "Swordfish88")
	if err != nil {
		t.Fatalf("expected password login setup to succeed, got %v", err)
	}
	if enabled.Account.AccountID != claimed.Account.AccountID {
		t.Fatalf("expected enabled account session to match claim, got %#v", enabled)
	}

	byEmail, err := store.LoginWithPassword("aurora@example.com", "Swordfish88")
	if err != nil {
		t.Fatalf("expected email login to succeed, got %v", err)
	}
	if byEmail.Account.AccountID != claimed.Account.AccountID || byEmail.SessionToken == "" {
		t.Fatalf("unexpected email login session %#v", byEmail)
	}

	byHandle, err := store.LoginWithPassword("aurora_auth", "Swordfish88")
	if err != nil {
		t.Fatalf("expected handle login to succeed, got %v", err)
	}
	if byHandle.Account.AccountID != claimed.Account.AccountID {
		t.Fatalf("unexpected handle login session %#v", byHandle)
	}

	if err := store.LogoutAccount(byHandle.Account.AccountID, byHandle.SessionToken); err != nil {
		t.Fatalf("expected logout to succeed, got %v", err)
	}
	if _, err := store.ResumeAccount(byHandle.Account.AccountID, byHandle.SessionToken); err != ErrUnauthorizedAccountSession {
		t.Fatalf("expected logged-out session to be invalid, got %v", err)
	}
}

func TestAccountStoreRejectsDuplicateEmail(t *testing.T) {
	store, err := NewAccountStore("")
	if err != nil {
		t.Fatalf("expected in-memory account store to initialize, got %v", err)
	}

	first, err := store.ClaimGuest(GuestProfile{GuestID: "guest_one"}, "aurora_one")
	if err != nil {
		t.Fatalf("expected first claim to succeed, got %v", err)
	}
	if _, err := store.EnablePasswordLogin(first.Account.AccountID, first.SessionToken, "shared@example.com", "Swordfish88"); err != nil {
		t.Fatalf("expected first password setup to succeed, got %v", err)
	}

	second, err := store.ClaimGuest(GuestProfile{GuestID: "guest_two"}, "aurora_two")
	if err != nil {
		t.Fatalf("expected second claim to succeed, got %v", err)
	}
	if _, err := store.EnablePasswordLogin(second.Account.AccountID, second.SessionToken, "shared@example.com", "Swordfish99"); err != ErrAccountEmailTaken {
		t.Fatalf("expected duplicate email to be rejected, got %v", err)
	}
}

func TestAccountStoreManagesMultipleSessionsPerAccount(t *testing.T) {
	store, err := NewAccountStore("")
	if err != nil {
		t.Fatalf("expected in-memory account store to initialize, got %v", err)
	}

	claimed, err := store.ClaimGuest(GuestProfile{GuestID: "guest_multi"}, "aurora_multi")
	if err != nil {
		t.Fatalf("expected account claim to succeed, got %v", err)
	}
	if _, err := store.EnablePasswordLogin(claimed.Account.AccountID, claimed.SessionToken, "aurora_multi@example.com", "Swordfish88"); err != nil {
		t.Fatalf("expected password login setup to succeed, got %v", err)
	}

	byEmail, err := store.LoginWithPassword("aurora_multi@example.com", "Swordfish88")
	if err != nil {
		t.Fatalf("expected email login to succeed, got %v", err)
	}
	byHandle, err := store.LoginWithPassword("aurora_multi", "Swordfish88")
	if err != nil {
		t.Fatalf("expected handle login to succeed, got %v", err)
	}

	overview, err := store.ListAccountSessions(claimed.Account.AccountID, claimed.SessionToken)
	if err != nil {
		t.Fatalf("expected account session overview to succeed, got %v", err)
	}
	if len(overview.Sessions) != 3 {
		t.Fatalf("expected three active account sessions, got %#v", overview.Sessions)
	}

	if err := store.RevokeAccountSession(claimed.Account.AccountID, claimed.SessionToken, byEmail.SessionToken); err != nil {
		t.Fatalf("expected targeted account session revoke to succeed, got %v", err)
	}
	if _, err := store.ResumeAccount(byEmail.Account.AccountID, byEmail.SessionToken); err != ErrUnauthorizedAccountSession {
		t.Fatalf("expected revoked session to be invalid, got %v", err)
	}

	if err := store.RevokeOtherAccountSessions(claimed.Account.AccountID, claimed.SessionToken); err != nil {
		t.Fatalf("expected revoke-other-sessions to succeed, got %v", err)
	}
	if _, err := store.ResumeAccount(byHandle.Account.AccountID, byHandle.SessionToken); err != ErrUnauthorizedAccountSession {
		t.Fatalf("expected other session to be invalid after revoke-others, got %v", err)
	}

	overview, err = store.ListAccountSessions(claimed.Account.AccountID, claimed.SessionToken)
	if err != nil {
		t.Fatalf("expected account session overview after revoke-others to succeed, got %v", err)
	}
	if len(overview.Sessions) != 1 || overview.Sessions[0].SessionToken != claimed.SessionToken {
		t.Fatalf("expected only the current session to remain, got %#v", overview.Sessions)
	}
}
