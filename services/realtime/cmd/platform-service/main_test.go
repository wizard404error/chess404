package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chess404/realtime/internal/contracts"
	"github.com/chess404/realtime/internal/platform"
)

func buildTestPlatformMux(t *testing.T, archive *platform.MatchArchiveStore, guests platform.GuestDirectory, claims *platform.MatchClaimStore) http.Handler {
	t.Helper()
	accounts, err := platform.NewAccountStore("")
	if err != nil {
		t.Fatalf("expected account store to initialize, got %v", err)
	}
	return buildPlatformMux(archive, guests, accounts, claims)
}

func TestPlatformStatusIncludesGuestStoreBackend(t *testing.T) {
	tempDir := t.TempDir()
	archive, err := platform.NewSQLiteMatchArchiveStore(filepath.Join(tempDir, "archive.sqlite"))
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}
	defer func() { _ = archive.Close() }()
	guests, err := platform.NewSQLiteGuestStore(filepath.Join(tempDir, "guests.sqlite"))
	if err != nil {
		t.Fatalf("expected sqlite guest store to initialize, got %v", err)
	}
	defer func() { _ = guests.Close() }()
	accounts, err := platform.NewSQLiteAccountStore(filepath.Join(tempDir, "accounts.sqlite"))
	if err != nil {
		t.Fatalf("expected sqlite account store to initialize, got %v", err)
	}
	defer func() { _ = accounts.Close() }()
	claims := platform.NewMatchClaimStore()

	req := httptest.NewRequest(http.MethodGet, "/api/platform/status", nil)
	rec := httptest.NewRecorder()

	buildPlatformMux(archive, guests, accounts, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected platform status to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		ArchiveBackend      string `json:"archiveBackend"`
		GuestStoreBackend   string `json:"guestStoreBackend"`
		AccountStoreBackend string `json:"accountStoreBackend"`
		ClaimStoreBackend   string `json:"claimStoreBackend"`
		ClaimLeaseSeconds   int    `json:"claimLeaseSeconds"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected platform status to decode, got %v", err)
	}
	if response.ArchiveBackend != "sqlite" {
		t.Fatalf("expected sqlite archive backend in status, got %#v", response)
	}
	if response.GuestStoreBackend != "sqlite" {
		t.Fatalf("expected sqlite guest store backend in status, got %#v", response)
	}
	if response.AccountStoreBackend != "sqlite" {
		t.Fatalf("expected sqlite account store backend in status, got %#v", response)
	}
	if response.ClaimStoreBackend != "memory" {
		t.Fatalf("expected memory claim store backend in status, got %#v", response)
	}
	if response.ClaimLeaseSeconds <= 0 {
		t.Fatalf("expected positive claim lease seconds in status, got %#v", response)
	}
}

func TestGuestSessionsRejectUnauthorizedResume(t *testing.T) {
	tempDir := t.TempDir()
	archive, err := platform.NewMatchArchiveStore(filepath.Join(tempDir, "archive.json"))
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}
	defer func() { _ = archive.Close() }()
	guests, err := platform.NewGuestStore(filepath.Join(tempDir, "guests.json"))
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}
	defer func() { _ = guests.Close() }()
	claims := platform.NewMatchClaimStore()

	session, err := guests.EnsureGuest("guest_secure", "")
	if err != nil {
		t.Fatalf("expected guest session creation to succeed, got %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/platform/guest-sessions", strings.NewReader(`{"guestId":"`+session.Guest.GuestID+`","sessionSecret":"wrong-secret"}`))
	rec := httptest.NewRecorder()

	buildTestPlatformMux(t, archive, guests, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized guest resume to be rejected, got status %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGuestSessionsResumeByToken(t *testing.T) {
	tempDir := t.TempDir()
	archive, err := platform.NewMatchArchiveStore(filepath.Join(tempDir, "archive.json"))
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}
	defer func() { _ = archive.Close() }()
	guests, err := platform.NewGuestStore(filepath.Join(tempDir, "guests.json"))
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}
	defer func() { _ = guests.Close() }()
	claims := platform.NewMatchClaimStore()

	session, err := guests.EnsureGuest("guest_token_resume", "")
	if err != nil {
		t.Fatalf("expected guest session creation to succeed, got %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/platform/guest-sessions", strings.NewReader(`{"guestId":"`+session.Guest.GuestID+`","sessionToken":"`+session.SessionToken+`"}`))
	rec := httptest.NewRecorder()

	buildTestPlatformMux(t, archive, guests, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected token guest resume to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}

	var response platform.GuestSession
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected token guest resume response to decode, got %v", err)
	}
	if response.Guest.GuestID != session.Guest.GuestID || response.SessionToken != session.SessionToken || response.ExpiresAt.IsZero() {
		t.Fatalf("unexpected token guest resume response %#v", response)
	}
}

func TestAccountClaimCreatesAccountSession(t *testing.T) {
	tempDir := t.TempDir()
	archive, err := platform.NewMatchArchiveStore(filepath.Join(tempDir, "archive.json"))
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}
	defer func() { _ = archive.Close() }()
	guests, err := platform.NewGuestStore(filepath.Join(tempDir, "guests.json"))
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}
	defer func() { _ = guests.Close() }()
	accounts, err := platform.NewAccountStore(filepath.Join(tempDir, "accounts.json"))
	if err != nil {
		t.Fatalf("expected account store to initialize, got %v", err)
	}
	defer func() { _ = accounts.Close() }()
	claims := platform.NewMatchClaimStore()

	session, err := guests.EnsureGuest("guest_account_claim", "")
	if err != nil {
		t.Fatalf("expected guest session creation to succeed, got %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/platform/accounts/claim", strings.NewReader(`{"guestId":"`+session.Guest.GuestID+`","sessionToken":"`+session.SessionToken+`","handle":"Aurora_Fox"}`))
	rec := httptest.NewRecorder()

	buildPlatformMux(archive, guests, accounts, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected account claim to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}

	var response platform.AccountSession
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected account claim response to decode, got %v", err)
	}
	if response.Account.AccountID == "" || response.Account.Handle != "aurora_fox" || response.Account.PrimaryGuestID != session.Guest.GuestID || response.SessionToken == "" || response.ExpiresAt.IsZero() {
		t.Fatalf("unexpected account claim response %#v", response)
	}
}

func TestAccountSessionsResumeByToken(t *testing.T) {
	tempDir := t.TempDir()
	archive, err := platform.NewMatchArchiveStore(filepath.Join(tempDir, "archive.json"))
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}
	defer func() { _ = archive.Close() }()
	guests, err := platform.NewGuestStore(filepath.Join(tempDir, "guests.json"))
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}
	defer func() { _ = guests.Close() }()
	accounts, err := platform.NewAccountStore(filepath.Join(tempDir, "accounts.json"))
	if err != nil {
		t.Fatalf("expected account store to initialize, got %v", err)
	}
	defer func() { _ = accounts.Close() }()
	claims := platform.NewMatchClaimStore()

	accountSession, err := accounts.ClaimGuest(platform.GuestProfile{GuestID: "guest_account_resume"}, "aurora_resume")
	if err != nil {
		t.Fatalf("expected account claim to succeed, got %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/platform/account-sessions", strings.NewReader(`{"accountId":"`+accountSession.Account.AccountID+`","sessionToken":"`+accountSession.SessionToken+`"}`))
	rec := httptest.NewRecorder()

	buildPlatformMux(archive, guests, accounts, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected account resume to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}

	var response platform.AccountSession
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected account session response to decode, got %v", err)
	}
	if response.Account.AccountID != accountSession.Account.AccountID || response.SessionToken != accountSession.SessionToken || response.ExpiresAt.IsZero() {
		t.Fatalf("unexpected account session response %#v", response)
	}
}

func TestAccountsListReturnsRecentAccounts(t *testing.T) {
	tempDir := t.TempDir()
	archive, err := platform.NewMatchArchiveStore(filepath.Join(tempDir, "archive.json"))
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}
	defer func() { _ = archive.Close() }()
	guests, err := platform.NewGuestStore(filepath.Join(tempDir, "guests.json"))
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}
	defer func() { _ = guests.Close() }()
	accounts, err := platform.NewAccountStore(filepath.Join(tempDir, "accounts.json"))
	if err != nil {
		t.Fatalf("expected account store to initialize, got %v", err)
	}
	defer func() { _ = accounts.Close() }()
	claims := platform.NewMatchClaimStore()

	first, err := accounts.ClaimGuest(platform.GuestProfile{GuestID: "guest_first"}, "aurora_first")
	if err != nil {
		t.Fatalf("expected first account claim to succeed, got %v", err)
	}
	if _, err := accounts.ClaimGuest(platform.GuestProfile{GuestID: "guest_second"}, "aurora_second"); err != nil {
		t.Fatalf("expected second account claim to succeed, got %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if _, err := accounts.ResumeAccount(first.Account.AccountID, first.SessionToken); err != nil {
		t.Fatalf("expected first account resume to succeed, got %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/platform/accounts?limit=10", nil)
	rec := httptest.NewRecorder()

	buildPlatformMux(archive, guests, accounts, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected account list to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		Accounts []platform.AccountProfile `json:"accounts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected account list to decode, got %v", err)
	}
	if len(response.Accounts) != 2 || response.Accounts[0].AccountID != first.Account.AccountID {
		t.Fatalf("unexpected account list response %#v", response)
	}
}

func TestAccountByGuestLookupReturnsLinkedAccount(t *testing.T) {
	tempDir := t.TempDir()
	archive, err := platform.NewMatchArchiveStore(filepath.Join(tempDir, "archive.json"))
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}
	defer func() { _ = archive.Close() }()
	guests, err := platform.NewGuestStore(filepath.Join(tempDir, "guests.json"))
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}
	defer func() { _ = guests.Close() }()
	accounts, err := platform.NewAccountStore(filepath.Join(tempDir, "accounts.json"))
	if err != nil {
		t.Fatalf("expected account store to initialize, got %v", err)
	}
	defer func() { _ = accounts.Close() }()
	claims := platform.NewMatchClaimStore()

	session, err := accounts.ClaimGuest(platform.GuestProfile{GuestID: "guest_lookup"}, "aurora_lookup")
	if err != nil {
		t.Fatalf("expected account claim to succeed, got %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/platform/accounts/by-guest/guest_lookup", nil)
	rec := httptest.NewRecorder()

	buildPlatformMux(archive, guests, accounts, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected account lookup by guest to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		Account platform.AccountProfile `json:"account"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected account-by-guest response to decode, got %v", err)
	}
	if response.Account.AccountID != session.Account.AccountID || response.Account.Handle != "aurora_lookup" {
		t.Fatalf("unexpected account-by-guest response %#v", response)
	}
}

func TestAccountDetailIncludesDerivedGuestStats(t *testing.T) {
	tempDir := t.TempDir()
	archive, err := platform.NewMatchArchiveStore(filepath.Join(tempDir, "archive.json"))
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}
	defer func() { _ = archive.Close() }()
	guests, err := platform.NewGuestStore(filepath.Join(tempDir, "guests.json"))
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}
	defer func() { _ = guests.Close() }()
	accounts, err := platform.NewAccountStore(filepath.Join(tempDir, "accounts.json"))
	if err != nil {
		t.Fatalf("expected account store to initialize, got %v", err)
	}
	defer func() { _ = accounts.Close() }()
	claims := platform.NewMatchClaimStore()

	guestSession, err := guests.EnsureGuest("guest_account_stats", "")
	if err != nil {
		t.Fatalf("expected guest session creation to succeed, got %v", err)
	}
	accountSession, err := accounts.ClaimGuest(guestSession.Guest, "aurora_stats")
	if err != nil {
		t.Fatalf("expected account claim to succeed, got %v", err)
	}
	if _, _, _, err := guests.FinalizeMatch("stats_match_1", "guest_account_stats", "guest_other_1", "white"); err == nil {
		t.Fatalf("expected missing opponent to reject finalize before setup")
	}
	if _, err := guests.EnsureGuest("guest_other_1", ""); err != nil {
		t.Fatalf("expected first opponent creation to succeed, got %v", err)
	}
	if _, err := guests.EnsureGuest("guest_other_2", ""); err != nil {
		t.Fatalf("expected second opponent creation to succeed, got %v", err)
	}
	if _, _, changed, err := guests.FinalizeMatch("stats_match_1", "guest_account_stats", "guest_other_1", "white"); err != nil || !changed {
		t.Fatalf("expected first rated result to succeed, got changed=%v err=%v", changed, err)
	}
	if _, _, changed, err := guests.FinalizeMatch("stats_match_2", "guest_account_stats", "guest_other_2", "draw"); err != nil || !changed {
		t.Fatalf("expected second rated result to succeed, got changed=%v err=%v", changed, err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/platform/accounts/"+accountSession.Account.AccountID, nil)
	rec := httptest.NewRecorder()

	buildPlatformMux(archive, guests, accounts, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected account detail to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		Account struct {
			AccountID     string `json:"accountId"`
			Handle        string `json:"handle"`
			DisplayName   string `json:"displayName"`
			Rating        int    `json:"rating"`
			MatchesPlayed int    `json:"matchesPlayed"`
			Wins          int    `json:"wins"`
			Losses        int    `json:"losses"`
			Draws         int    `json:"draws"`
			GuestCount    int    `json:"guestCount"`
		} `json:"account"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected account detail response to decode, got %v", err)
	}
	if response.Account.AccountID != accountSession.Account.AccountID || response.Account.Handle != "aurora_stats" {
		t.Fatalf("unexpected account identity response %#v", response)
	}
	if response.Account.DisplayName != guestSession.Guest.DisplayName || response.Account.Rating != 1216 {
		t.Fatalf("expected derived account display/rating, got %#v", response)
	}
	if response.Account.MatchesPlayed != 2 || response.Account.Wins != 1 || response.Account.Losses != 0 || response.Account.Draws != 1 || response.Account.GuestCount != 1 {
		t.Fatalf("expected derived account stats, got %#v", response)
	}
}

func TestAccountDetailIncludesDirectRatingHistory(t *testing.T) {
	tempDir := t.TempDir()
	archive, err := platform.NewMatchArchiveStore(filepath.Join(tempDir, "archive.json"))
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}
	defer func() { _ = archive.Close() }()
	guests, err := platform.NewGuestStore(filepath.Join(tempDir, "guests.json"))
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}
	defer func() { _ = guests.Close() }()
	accounts, err := platform.NewAccountStore(filepath.Join(tempDir, "accounts.json"))
	if err != nil {
		t.Fatalf("expected account store to initialize, got %v", err)
	}
	defer func() { _ = accounts.Close() }()
	claims := platform.NewMatchClaimStore()

	whiteGuest, err := guests.EnsureGuest("guest_history_white", "")
	if err != nil {
		t.Fatalf("expected white guest creation to succeed, got %v", err)
	}
	blackGuest, err := guests.EnsureGuest("guest_history_black", "")
	if err != nil {
		t.Fatalf("expected black guest creation to succeed, got %v", err)
	}
	whiteAccount, err := accounts.ClaimGuest(whiteGuest.Guest, "history_white")
	if err != nil {
		t.Fatalf("expected white account claim to succeed, got %v", err)
	}
	blackAccount, err := accounts.ClaimGuest(blackGuest.Guest, "history_black")
	if err != nil {
		t.Fatalf("expected black account claim to succeed, got %v", err)
	}
	if _, _, err := accounts.SyncGuestStats(whiteGuest.Guest); err != nil {
		t.Fatalf("expected white account sync to succeed, got %v", err)
	}
	if _, _, err := accounts.SyncGuestStats(blackGuest.Guest); err != nil {
		t.Fatalf("expected black account sync to succeed, got %v", err)
	}
	if _, _, changed, err := accounts.FinalizeMatch("account_history_match", whiteAccount.Account.AccountID, blackAccount.Account.AccountID, "white"); err != nil || !changed {
		t.Fatalf("expected account finalization to succeed, got changed=%v err=%v", changed, err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/platform/accounts/"+whiteAccount.Account.AccountID, nil)
	rec := httptest.NewRecorder()

	buildPlatformMux(archive, guests, accounts, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected account detail to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		Account struct {
			AccountID     string `json:"accountId"`
			Handle        string `json:"handle"`
			Rating        int    `json:"rating"`
			MatchesPlayed int    `json:"matchesPlayed"`
			CurrentSeason *struct {
				SeasonID      string `json:"seasonId"`
				MatchesPlayed int    `json:"matchesPlayed"`
				NetDelta      int    `json:"netDelta"`
				RatingEnd     int    `json:"ratingEnd"`
			} `json:"currentSeason"`
			SeasonHistory []struct {
				SeasonID      string `json:"seasonId"`
				MatchesPlayed int    `json:"matchesPlayed"`
				NetDelta      int    `json:"netDelta"`
			} `json:"seasonHistory"`
			RatingHistory []struct {
				MatchID       string `json:"matchId"`
				Result        string `json:"result"`
				Delta         int    `json:"delta"`
				RatingAfter   int    `json:"ratingAfter"`
				MatchesPlayed int    `json:"matchesPlayed"`
			} `json:"ratingHistory"`
		} `json:"account"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected account detail response to decode, got %v", err)
	}
	if response.Account.AccountID != whiteAccount.Account.AccountID || response.Account.Handle != "history_white" {
		t.Fatalf("unexpected account identity response %#v", response)
	}
	if response.Account.Rating != 1216 || response.Account.MatchesPlayed != 1 {
		t.Fatalf("expected direct account stats after finalize, got %#v", response.Account)
	}
	if len(response.Account.RatingHistory) != 1 {
		t.Fatalf("expected direct rating history entry, got %#v", response.Account.RatingHistory)
	}
	if response.Account.CurrentSeason == nil || response.Account.CurrentSeason.SeasonID != "2026-05" || response.Account.CurrentSeason.MatchesPlayed != 1 || response.Account.CurrentSeason.NetDelta != 16 || response.Account.CurrentSeason.RatingEnd != 1216 {
		t.Fatalf("unexpected current season summary %#v", response.Account.CurrentSeason)
	}
	if len(response.Account.SeasonHistory) != 1 || response.Account.SeasonHistory[0].SeasonID != "2026-05" || response.Account.SeasonHistory[0].MatchesPlayed != 1 || response.Account.SeasonHistory[0].NetDelta != 16 {
		t.Fatalf("unexpected season history %#v", response.Account.SeasonHistory)
	}
	if response.Account.RatingHistory[0].MatchID != "account_history_match" || response.Account.RatingHistory[0].Result != "win" || response.Account.RatingHistory[0].Delta != 16 || response.Account.RatingHistory[0].RatingAfter != 1216 || response.Account.RatingHistory[0].MatchesPlayed != 1 {
		t.Fatalf("unexpected account rating history %#v", response.Account.RatingHistory[0])
	}
}

func TestAccountsListCanSortByDerivedRating(t *testing.T) {
	tempDir := t.TempDir()
	archive, err := platform.NewMatchArchiveStore(filepath.Join(tempDir, "archive.json"))
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}
	defer func() { _ = archive.Close() }()
	guests, err := platform.NewGuestStore(filepath.Join(tempDir, "guests.json"))
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}
	defer func() { _ = guests.Close() }()
	accounts, err := platform.NewAccountStore(filepath.Join(tempDir, "accounts.json"))
	if err != nil {
		t.Fatalf("expected account store to initialize, got %v", err)
	}
	defer func() { _ = accounts.Close() }()
	claims := platform.NewMatchClaimStore()

	whiteOne, err := guests.EnsureGuest("guest_rating_one", "")
	if err != nil {
		t.Fatalf("expected first guest creation to succeed, got %v", err)
	}
	whiteTwo, err := guests.EnsureGuest("guest_rating_two", "")
	if err != nil {
		t.Fatalf("expected second guest creation to succeed, got %v", err)
	}
	if _, err := guests.EnsureGuest("guest_loss_a", ""); err != nil {
		t.Fatalf("expected first opponent creation to succeed, got %v", err)
	}
	if _, err := guests.EnsureGuest("guest_loss_b", ""); err != nil {
		t.Fatalf("expected second opponent creation to succeed, got %v", err)
	}
	if _, err := accounts.ClaimGuest(whiteOne.Guest, "aurora_one"); err != nil {
		t.Fatalf("expected first account claim to succeed, got %v", err)
	}
	if _, err := accounts.ClaimGuest(whiteTwo.Guest, "aurora_two"); err != nil {
		t.Fatalf("expected second account claim to succeed, got %v", err)
	}
	if _, _, changed, err := guests.FinalizeMatch("rank_match_1", "guest_rating_one", "guest_loss_a", "white"); err != nil || !changed {
		t.Fatalf("expected first result finalize to succeed, got changed=%v err=%v", changed, err)
	}
	if _, _, changed, err := guests.FinalizeMatch("rank_match_2", "guest_rating_two", "guest_loss_b", "draw"); err != nil || !changed {
		t.Fatalf("expected second result finalize to succeed, got changed=%v err=%v", changed, err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/platform/accounts?limit=10&sort=rating", nil)
	rec := httptest.NewRecorder()

	buildPlatformMux(archive, guests, accounts, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected sorted account list to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		Accounts []struct {
			AccountID     string `json:"accountId"`
			Handle        string `json:"handle"`
			Rating        int    `json:"rating"`
			CurrentSeason *struct {
				SeasonID      string `json:"seasonId"`
				MatchesPlayed int    `json:"matchesPlayed"`
				NetDelta      int    `json:"netDelta"`
			} `json:"currentSeason"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected sorted account list response to decode, got %v", err)
	}
	if len(response.Accounts) != 2 {
		t.Fatalf("expected two accounts in sorted list, got %#v", response)
	}
	if response.Accounts[0].Handle != "aurora_one" || response.Accounts[0].Rating != 1216 {
		t.Fatalf("expected highest-rated account first, got %#v", response.Accounts)
	}
	if response.Accounts[1].Handle != "aurora_two" || response.Accounts[1].Rating != 1200 {
		t.Fatalf("expected second account to keep draw rating, got %#v", response.Accounts)
	}
}

func TestAccountsListIncludesCurrentSeasonForDirectAccountHistory(t *testing.T) {
	tempDir := t.TempDir()
	archive, err := platform.NewMatchArchiveStore(filepath.Join(tempDir, "archive.json"))
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}
	defer func() { _ = archive.Close() }()
	guests, err := platform.NewGuestStore(filepath.Join(tempDir, "guests.json"))
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}
	defer func() { _ = guests.Close() }()
	accounts, err := platform.NewAccountStore(filepath.Join(tempDir, "accounts.json"))
	if err != nil {
		t.Fatalf("expected account store to initialize, got %v", err)
	}
	defer func() { _ = accounts.Close() }()
	claims := platform.NewMatchClaimStore()

	whiteGuest, err := guests.EnsureGuest("guest_rank_history_white", "")
	if err != nil {
		t.Fatalf("expected white guest creation to succeed, got %v", err)
	}
	blackGuest, err := guests.EnsureGuest("guest_rank_history_black", "")
	if err != nil {
		t.Fatalf("expected black guest creation to succeed, got %v", err)
	}
	whiteAccount, err := accounts.ClaimGuest(whiteGuest.Guest, "rank_history_white")
	if err != nil {
		t.Fatalf("expected white account claim to succeed, got %v", err)
	}
	blackAccount, err := accounts.ClaimGuest(blackGuest.Guest, "rank_history_black")
	if err != nil {
		t.Fatalf("expected black account claim to succeed, got %v", err)
	}
	if _, _, err := accounts.SyncGuestStats(whiteGuest.Guest); err != nil {
		t.Fatalf("expected white account sync to succeed, got %v", err)
	}
	if _, _, err := accounts.SyncGuestStats(blackGuest.Guest); err != nil {
		t.Fatalf("expected black account sync to succeed, got %v", err)
	}
	if _, _, changed, err := accounts.FinalizeMatch("rank_history_match", whiteAccount.Account.AccountID, blackAccount.Account.AccountID, "white"); err != nil || !changed {
		t.Fatalf("expected direct account finalize to succeed, got changed=%v err=%v", changed, err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/platform/accounts?limit=10&sort=rating", nil)
	rec := httptest.NewRecorder()

	buildPlatformMux(archive, guests, accounts, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected sorted account list to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		Accounts []struct {
			AccountID     string `json:"accountId"`
			Handle        string `json:"handle"`
			Rating        int    `json:"rating"`
			CurrentSeason *struct {
				SeasonID      string `json:"seasonId"`
				MatchesPlayed int    `json:"matchesPlayed"`
				NetDelta      int    `json:"netDelta"`
			} `json:"currentSeason"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected sorted account list response to decode, got %v", err)
	}
	if len(response.Accounts) != 2 {
		t.Fatalf("expected two accounts in sorted list, got %#v", response)
	}
	if response.Accounts[0].Handle != "rank_history_white" || response.Accounts[0].Rating != 1216 {
		t.Fatalf("expected direct-history winner to rank first, got %#v", response.Accounts)
	}
	if response.Accounts[0].CurrentSeason == nil || response.Accounts[0].CurrentSeason.SeasonID != "2026-05" || response.Accounts[0].CurrentSeason.MatchesPlayed != 1 || response.Accounts[0].CurrentSeason.NetDelta != 16 {
		t.Fatalf("expected direct-history account to expose current season, got %#v", response.Accounts[0])
	}
}

func TestAccountsListCanFilterToRequestedSeason(t *testing.T) {
	tempDir := t.TempDir()
	archive, err := platform.NewMatchArchiveStore(filepath.Join(tempDir, "archive.json"))
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}
	defer func() { _ = archive.Close() }()
	guests, err := platform.NewGuestStore(filepath.Join(tempDir, "guests.json"))
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}
	defer func() { _ = guests.Close() }()
	accounts, err := platform.NewAccountStore(filepath.Join(tempDir, "accounts.json"))
	if err != nil {
		t.Fatalf("expected account store to initialize, got %v", err)
	}
	defer func() { _ = accounts.Close() }()
	claims := platform.NewMatchClaimStore()

	whiteGuest, _ := guests.EnsureGuest("guest_filter_white", "")
	blackGuest, _ := guests.EnsureGuest("guest_filter_black", "")
	whiteAccount, _ := accounts.ClaimGuest(whiteGuest.Guest, "season_filter_white")
	blackAccount, _ := accounts.ClaimGuest(blackGuest.Guest, "season_filter_black")
	if _, _, err := accounts.SyncGuestStats(whiteGuest.Guest); err != nil {
		t.Fatalf("expected white sync to succeed, got %v", err)
	}
	if _, _, err := accounts.SyncGuestStats(blackGuest.Guest); err != nil {
		t.Fatalf("expected black sync to succeed, got %v", err)
	}
	if _, _, changed, err := accounts.FinalizeMatch("season_filter_match", whiteAccount.Account.AccountID, blackAccount.Account.AccountID, "white"); err != nil || !changed {
		t.Fatalf("expected direct account finalize to succeed, got changed=%v err=%v", changed, err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/platform/accounts?limit=10&sort=rating&seasonId=2026-05", nil)
	rec := httptest.NewRecorder()

	buildPlatformMux(archive, guests, accounts, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected season-filtered account list to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		Accounts []struct {
			Handle         string `json:"handle"`
			SelectedSeason *struct {
				SeasonID string `json:"seasonId"`
				NetDelta int    `json:"netDelta"`
			} `json:"selectedSeason"`
		} `json:"accounts"`
		Seasons []struct {
			SeasonID string `json:"seasonId"`
		} `json:"seasons"`
		SelectedSeasonID string `json:"selectedSeasonId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected season-filtered account list to decode, got %v", err)
	}
	if response.SelectedSeasonID != "2026-05" {
		t.Fatalf("expected selected season id to round-trip, got %#v", response)
	}
	if len(response.Accounts) != 2 || response.Accounts[0].SelectedSeason == nil || response.Accounts[0].SelectedSeason.SeasonID != "2026-05" {
		t.Fatalf("expected only matching season accounts with selected season, got %#v", response)
	}
	if len(response.Seasons) != 1 || response.Seasons[0].SeasonID != "2026-05" {
		t.Fatalf("expected available season metadata, got %#v", response.Seasons)
	}
}

func TestArchivedMatchDetailIncludesLinkedAccountHandles(t *testing.T) {
	tempDir := t.TempDir()
	archive, err := platform.NewMatchArchiveStore(filepath.Join(tempDir, "archive.json"))
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}
	defer func() { _ = archive.Close() }()
	guests, err := platform.NewGuestStore(filepath.Join(tempDir, "guests.json"))
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}
	defer func() { _ = guests.Close() }()
	accounts, err := platform.NewAccountStore(filepath.Join(tempDir, "accounts.json"))
	if err != nil {
		t.Fatalf("expected account store to initialize, got %v", err)
	}
	defer func() { _ = accounts.Close() }()
	claims := platform.NewMatchClaimStore()

	if _, err := accounts.ClaimGuest(platform.GuestProfile{GuestID: "guest_archive_white"}, "aurora_archive"); err != nil {
		t.Fatalf("expected account claim to succeed, got %v", err)
	}
	now := time.Date(2026, 5, 7, 9, 0, 0, 0, time.UTC)
	if err := archive.Upsert(contracts.MatchSnapshotResponse{
		Match: contracts.MatchState{
			MatchID:      "archive_account_match",
			RulesVersion: "v1-alpha-foundation",
			WhiteGuestID: "guest_archive_white",
			BlackGuestID: "guest_archive_black",
			WhiteName:    "Aurora White",
			BlackName:    "Velvet Black",
			Status:       "finished",
			Winner:       "white",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}); err != nil {
		t.Fatalf("expected archive upsert to succeed, got %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/platform/matches/archive_account_match", nil)
	rec := httptest.NewRecorder()

	buildPlatformMux(archive, guests, accounts, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected archived match detail to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}

	var response platform.MatchArchiveEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected archived match detail to decode, got %v", err)
	}
	if response.WhiteAccountID == "" || response.WhiteAccountHandle != "aurora_archive" {
		t.Fatalf("expected white account identity to be enriched, got %#v", response)
	}
	if response.Snapshot.Match.WhiteAccountID != response.WhiteAccountID {
		t.Fatalf("expected snapshot match account id to be enriched, got %#v", response.Snapshot.Match)
	}
}

func TestArchivedMatchListCanFilterByAccount(t *testing.T) {
	tempDir := t.TempDir()
	archive, err := platform.NewMatchArchiveStore(filepath.Join(tempDir, "archive.json"))
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}
	defer func() { _ = archive.Close() }()
	guests, err := platform.NewGuestStore(filepath.Join(tempDir, "guests.json"))
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}
	defer func() { _ = guests.Close() }()
	accounts, err := platform.NewAccountStore(filepath.Join(tempDir, "accounts.json"))
	if err != nil {
		t.Fatalf("expected account store to initialize, got %v", err)
	}
	defer func() { _ = accounts.Close() }()
	claims := platform.NewMatchClaimStore()

	accountSession, err := accounts.ClaimGuest(platform.GuestProfile{GuestID: "guest_archive_white"}, "aurora_archive")
	if err != nil {
		t.Fatalf("expected account claim to succeed, got %v", err)
	}
	now := time.Date(2026, 5, 7, 9, 30, 0, 0, time.UTC)
	if err := archive.Upsert(contracts.MatchSnapshotResponse{
		Match: contracts.MatchState{
			MatchID:      "archive_account_filter_match",
			RulesVersion: "v1-alpha-foundation",
			WhiteGuestID: "guest_archive_white",
			BlackGuestID: "guest_archive_black",
			WhiteName:    "Aurora White",
			BlackName:    "Velvet Black",
			Status:       "finished",
			Winner:       "white",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}); err != nil {
		t.Fatalf("expected archive upsert to succeed, got %v", err)
	}
	if err := archive.Upsert(contracts.MatchSnapshotResponse{
		Match: contracts.MatchState{
			MatchID:      "archive_other_match",
			RulesVersion: "v1-alpha-foundation",
			WhiteGuestID: "guest_else",
			BlackGuestID: "guest_other",
			Status:       "finished",
			Winner:       "black",
			CreatedAt:    now,
			UpdatedAt:    now.Add(time.Minute),
		},
	}); err != nil {
		t.Fatalf("expected second archive upsert to succeed, got %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/platform/matches?accountId="+accountSession.Account.AccountID, nil)
	rec := httptest.NewRecorder()

	buildPlatformMux(archive, guests, accounts, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected archived account match list to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		Matches []platform.MatchArchiveEntry `json:"matches"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected archived account match list to decode, got %v", err)
	}
	if len(response.Matches) != 1 || response.Matches[0].MatchID != "archive_account_filter_match" {
		t.Fatalf("unexpected archived account match list response %#v", response)
	}
	if response.Matches[0].WhiteAccountID != accountSession.Account.AccountID {
		t.Fatalf("expected account filter response to be enriched with account id, got %#v", response.Matches[0])
	}
}

func TestMatchClaimsReturnSeatSecretForOwnedSeat(t *testing.T) {
	tempDir := t.TempDir()
	archive, err := platform.NewMatchArchiveStore(filepath.Join(tempDir, "archive.json"))
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}
	defer func() { _ = archive.Close() }()
	guests, err := platform.NewGuestStore(filepath.Join(tempDir, "guests.json"))
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}
	defer func() { _ = guests.Close() }()
	claims := platform.NewMatchClaimStore()

	session, err := guests.EnsureGuest("guest_white", "")
	if err != nil {
		t.Fatalf("expected guest session creation to succeed, got %v", err)
	}
	now := time.Date(2026, 5, 6, 18, 0, 0, 0, time.UTC)
	if err := archive.Upsert(contracts.MatchSnapshotResponse{
		Match: contracts.MatchState{
			MatchID:           "room_claim",
			RulesVersion:      "v1-alpha-foundation",
			Queue:             "rated",
			WhiteGuestID:      session.Guest.GuestID,
			WhiteName:         session.Guest.DisplayName,
			WhitePlayerSecret: "room_secret_white",
			Status:            "active",
			CreatedAt:         now,
			UpdatedAt:         now,
		},
	}); err != nil {
		t.Fatalf("expected archive upsert to succeed, got %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/platform/match-claims", strings.NewReader(`{"matchId":"room_claim","guestId":"`+session.Guest.GuestID+`","sessionSecret":"`+session.SessionSecret+`"}`))
	rec := httptest.NewRecorder()

	buildTestPlatformMux(t, archive, guests, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected match claim to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		SeatColor    string `json:"seatColor"`
		PlayerSecret string `json:"playerSecret"`
		PlayerID     string `json:"playerId"`
		ClaimToken   string `json:"claimToken"`
		ExpiresAt    string `json:"expiresAt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected match claim response to decode, got %v", err)
	}
	if response.SeatColor != "white" || response.PlayerSecret != "room_secret_white" || response.PlayerID != session.Guest.GuestID || response.ClaimToken == "" || response.ExpiresAt == "" {
		t.Fatalf("unexpected match claim response %#v", response)
	}
}

func TestMatchClaimsRejectUnauthorizedGuestSession(t *testing.T) {
	tempDir := t.TempDir()
	archive, err := platform.NewMatchArchiveStore(filepath.Join(tempDir, "archive.json"))
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}
	defer func() { _ = archive.Close() }()
	guests, err := platform.NewGuestStore(filepath.Join(tempDir, "guests.json"))
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}
	defer func() { _ = guests.Close() }()
	claims := platform.NewMatchClaimStore()

	session, err := guests.EnsureGuest("guest_white", "")
	if err != nil {
		t.Fatalf("expected guest session creation to succeed, got %v", err)
	}
	now := time.Date(2026, 5, 6, 18, 5, 0, 0, time.UTC)
	if err := archive.Upsert(contracts.MatchSnapshotResponse{
		Match: contracts.MatchState{
			MatchID:           "room_claim_secure",
			RulesVersion:      "v1-alpha-foundation",
			Queue:             "rated",
			WhiteGuestID:      session.Guest.GuestID,
			WhitePlayerSecret: "room_secret_white",
			Status:            "active",
			CreatedAt:         now,
			UpdatedAt:         now,
		},
	}); err != nil {
		t.Fatalf("expected archive upsert to succeed, got %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/platform/match-claims", strings.NewReader(`{"matchId":"room_claim_secure","guestId":"`+session.Guest.GuestID+`","sessionSecret":"wrong-secret"}`))
	rec := httptest.NewRecorder()

	buildTestPlatformMux(t, archive, guests, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized match claim to be rejected, got status %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMatchClaimsRecoverFromCachedClaimWithoutArchiveEntry(t *testing.T) {
	tempDir := t.TempDir()
	archive, err := platform.NewMatchArchiveStore(filepath.Join(tempDir, "archive.json"))
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}
	defer func() { _ = archive.Close() }()
	guests, err := platform.NewGuestStore(filepath.Join(tempDir, "guests.json"))
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}
	defer func() { _ = guests.Close() }()
	claims := platform.NewMatchClaimStore()

	session, err := guests.EnsureGuest("guest_white_cached", "")
	if err != nil {
		t.Fatalf("expected guest session creation to succeed, got %v", err)
	}
	if err := claims.Put(platform.MatchSeatClaim{
		MatchID:      "room_cached_claim",
		GuestID:      session.Guest.GuestID,
		SeatColor:    "white",
		PlayerID:     session.Guest.GuestID,
		PlayerSecret: "cached_room_secret",
		Queue:        "rated",
		Status:       "active",
	}); err != nil {
		t.Fatalf("expected cached claim write to succeed, got %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/platform/match-claims", strings.NewReader(`{"matchId":"room_cached_claim","guestId":"`+session.Guest.GuestID+`","sessionSecret":"`+session.SessionSecret+`"}`))
	rec := httptest.NewRecorder()

	buildTestPlatformMux(t, archive, guests, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected cached match claim to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		SeatColor    string `json:"seatColor"`
		PlayerSecret string `json:"playerSecret"`
		ClaimToken   string `json:"claimToken"`
		ExpiresAt    string `json:"expiresAt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected cached match claim response to decode, got %v", err)
	}
	if response.SeatColor != "white" || response.PlayerSecret != "cached_room_secret" || response.ClaimToken == "" || response.ExpiresAt == "" {
		t.Fatalf("expected cached claim to be returned even without archive, got %#v", response)
	}
}

func TestMatchClaimResolveReturnsCachedTokenClaim(t *testing.T) {
	tempDir := t.TempDir()
	archive, err := platform.NewMatchArchiveStore(filepath.Join(tempDir, "archive.json"))
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}
	defer func() { _ = archive.Close() }()
	guests, err := platform.NewGuestStore(filepath.Join(tempDir, "guests.json"))
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}
	defer func() { _ = guests.Close() }()
	claims := platform.NewMatchClaimStore()

	session, err := guests.EnsureGuest("guest_resolve", "")
	if err != nil {
		t.Fatalf("expected guest session creation to succeed, got %v", err)
	}
	if err := claims.Put(platform.MatchSeatClaim{
		MatchID:      "room_resolve",
		GuestID:      session.Guest.GuestID,
		SeatColor:    "white",
		PlayerID:     session.Guest.GuestID,
		PlayerSecret: "resolve_secret",
	}); err != nil {
		t.Fatalf("expected claim put to succeed, got %v", err)
	}
	stored, ok := claims.Get("room_resolve", session.Guest.GuestID)
	if !ok {
		t.Fatalf("expected stored claim to be available")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/platform/match-claims/resolve", strings.NewReader(`{"matchId":"room_resolve","claimToken":"`+stored.ClaimToken+`"}`))
	rec := httptest.NewRecorder()

	buildTestPlatformMux(t, archive, guests, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected claim resolve to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		GuestID      string `json:"guestId"`
		PlayerSecret string `json:"playerSecret"`
		ClaimToken   string `json:"claimToken"`
		ExpiresAt    string `json:"expiresAt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected claim resolve response to decode, got %v", err)
	}
	if response.GuestID != session.Guest.GuestID || response.PlayerSecret != "resolve_secret" || response.ClaimToken != stored.ClaimToken || response.ExpiresAt == "" {
		t.Fatalf("unexpected claim resolve response %#v", response)
	}
}

func TestGuestResultsRejectNonRatedMatches(t *testing.T) {
	tempDir := t.TempDir()
	archive, err := platform.NewMatchArchiveStore(filepath.Join(tempDir, "archive.json"))
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}
	guests, err := platform.NewGuestStore(filepath.Join(tempDir, "guests.json"))
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}
	claims := platform.NewMatchClaimStore()

	now := time.Date(2026, 5, 6, 16, 0, 0, 0, time.UTC)
	if err := archive.Upsert(contracts.MatchSnapshotResponse{
		Match: contracts.MatchState{
			MatchID:      "casual_room",
			RulesVersion: "v1-alpha-foundation",
			Queue:        "casual",
			WhiteGuestID: "guest_white",
			BlackGuestID: "guest_black",
			Status:       "finished",
			Winner:       "white",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}); err != nil {
		t.Fatalf("expected archive upsert to succeed, got %v", err)
	}

	white, err := guests.EnsureGuest("guest_white", "")
	if err != nil {
		t.Fatalf("expected white guest creation to succeed, got %v", err)
	}
	black, err := guests.EnsureGuest("guest_black", "")
	if err != nil {
		t.Fatalf("expected black guest creation to succeed, got %v", err)
	}

	body := `{"matchId":"casual_room","whiteGuestId":"guest_white","blackGuestId":"guest_black","winner":"white"}`
	req := httptest.NewRequest(http.MethodPost, "/api/platform/guest-results", strings.NewReader(body))
	rec := httptest.NewRecorder()

	buildTestPlatformMux(t, archive, guests, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected casual room result to be rejected, got status %d body=%s", rec.Code, rec.Body.String())
	}
	refreshedWhite, ok := guests.GetGuest("guest_white")
	if !ok {
		t.Fatalf("expected white guest to remain available")
	}
	refreshedBlack, ok := guests.GetGuest("guest_black")
	if !ok {
		t.Fatalf("expected black guest to remain available")
	}
	if refreshedWhite.Rating != white.Guest.Rating || refreshedBlack.Rating != black.Guest.Rating {
		t.Fatalf("expected non-rated result rejection to preserve ratings, got %#v %#v", refreshedWhite, refreshedBlack)
	}
}

func TestGuestResultsFinalizeRatedMatches(t *testing.T) {
	tempDir := t.TempDir()
	archive, err := platform.NewMatchArchiveStore(filepath.Join(tempDir, "archive.json"))
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}
	guests, err := platform.NewGuestStore(filepath.Join(tempDir, "guests.json"))
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}
	claims := platform.NewMatchClaimStore()

	now := time.Date(2026, 5, 6, 16, 30, 0, 0, time.UTC)
	if err := archive.Upsert(contracts.MatchSnapshotResponse{
		Match: contracts.MatchState{
			MatchID:      "rated_room",
			RulesVersion: "v1-alpha-foundation",
			Queue:        "rated",
			WhiteGuestID: "guest_white",
			BlackGuestID: "guest_black",
			Status:       "finished",
			Winner:       "white",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}); err != nil {
		t.Fatalf("expected archive upsert to succeed, got %v", err)
	}

	if _, err := guests.EnsureGuest("guest_white", ""); err != nil {
		t.Fatalf("expected white guest creation to succeed, got %v", err)
	}
	if _, err := guests.EnsureGuest("guest_black", ""); err != nil {
		t.Fatalf("expected black guest creation to succeed, got %v", err)
	}

	body := `{"matchId":"rated_room","whiteGuestId":"guest_white","blackGuestId":"guest_black","winner":"white"}`
	req := httptest.NewRequest(http.MethodPost, "/api/platform/guest-results", strings.NewReader(body))
	rec := httptest.NewRecorder()

	buildTestPlatformMux(t, archive, guests, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected rated room result to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		Changed bool                  `json:"changed"`
		White   platform.GuestProfile `json:"white"`
		Black   platform.GuestProfile `json:"black"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected successful guest result response to decode, got %v", err)
	}
	if !response.Changed || response.White.Rating != 1216 || response.Black.Rating != 1184 {
		t.Fatalf("expected rated room to update ratings, got %#v", response)
	}
}

func TestGuestResultsRejectArchiveWinnerMismatch(t *testing.T) {
	tempDir := t.TempDir()
	archive, err := platform.NewMatchArchiveStore(filepath.Join(tempDir, "archive.json"))
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}
	guests, err := platform.NewGuestStore(filepath.Join(tempDir, "guests.json"))
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}
	claims := platform.NewMatchClaimStore()

	now := time.Date(2026, 5, 6, 17, 0, 0, 0, time.UTC)
	if err := archive.Upsert(contracts.MatchSnapshotResponse{
		Match: contracts.MatchState{
			MatchID:      "rated_room",
			RulesVersion: "v1-alpha-foundation",
			Queue:        "rated",
			WhiteGuestID: "guest_white",
			BlackGuestID: "guest_black",
			Status:       "finished",
			Winner:       "black",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}); err != nil {
		t.Fatalf("expected archive upsert to succeed, got %v", err)
	}

	if _, err := guests.EnsureGuest("guest_white", ""); err != nil {
		t.Fatalf("expected white guest creation to succeed, got %v", err)
	}
	if _, err := guests.EnsureGuest("guest_black", ""); err != nil {
		t.Fatalf("expected black guest creation to succeed, got %v", err)
	}

	body := `{"matchId":"rated_room","whiteGuestId":"guest_white","blackGuestId":"guest_black","winner":"white"}`
	req := httptest.NewRequest(http.MethodPost, "/api/platform/guest-results", strings.NewReader(body))
	rec := httptest.NewRecorder()

	buildTestPlatformMux(t, archive, guests, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected mismatched winner to be rejected, got status %d body=%s", rec.Code, rec.Body.String())
	}
	refreshedWhite, _ := guests.GetGuest("guest_white")
	refreshedBlack, _ := guests.GetGuest("guest_black")
	if refreshedWhite.Rating != 1200 || refreshedBlack.Rating != 1200 {
		t.Fatalf("expected rejected winner mismatch to preserve ratings, got %#v %#v", refreshedWhite, refreshedBlack)
	}
}

func TestGuestResultsRejectUnfinishedRatedMatches(t *testing.T) {
	tempDir := t.TempDir()
	archive, err := platform.NewMatchArchiveStore(filepath.Join(tempDir, "archive.json"))
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}
	guests, err := platform.NewGuestStore(filepath.Join(tempDir, "guests.json"))
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}
	claims := platform.NewMatchClaimStore()

	now := time.Date(2026, 5, 6, 17, 30, 0, 0, time.UTC)
	if err := archive.Upsert(contracts.MatchSnapshotResponse{
		Match: contracts.MatchState{
			MatchID:      "rated_room_open",
			RulesVersion: "v1-alpha-foundation",
			Queue:        "rated",
			WhiteGuestID: "guest_white",
			BlackGuestID: "guest_black",
			Status:       "active",
			Winner:       "",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}); err != nil {
		t.Fatalf("expected archive upsert to succeed, got %v", err)
	}

	if _, err := guests.EnsureGuest("guest_white", ""); err != nil {
		t.Fatalf("expected white guest creation to succeed, got %v", err)
	}
	if _, err := guests.EnsureGuest("guest_black", ""); err != nil {
		t.Fatalf("expected black guest creation to succeed, got %v", err)
	}

	body := `{"matchId":"rated_room_open","whiteGuestId":"guest_white","blackGuestId":"guest_black","winner":"white"}`
	req := httptest.NewRequest(http.MethodPost, "/api/platform/guest-results", strings.NewReader(body))
	rec := httptest.NewRecorder()

	buildTestPlatformMux(t, archive, guests, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected unfinished rated room to be rejected, got status %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAccountResultsFinalizeRatedMatchesWithLinkedGuestFallback(t *testing.T) {
	tempDir := t.TempDir()
	archive, err := platform.NewMatchArchiveStore(filepath.Join(tempDir, "archive.json"))
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}
	guests, err := platform.NewGuestStore(filepath.Join(tempDir, "guests.json"))
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}
	accounts, err := platform.NewAccountStore(filepath.Join(tempDir, "accounts.json"))
	if err != nil {
		t.Fatalf("expected account store to initialize, got %v", err)
	}
	claims := platform.NewMatchClaimStore()

	whiteSession, err := guests.EnsureGuest("guest_white", "")
	if err != nil {
		t.Fatalf("expected white guest creation to succeed, got %v", err)
	}
	blackSession, err := guests.EnsureGuest("guest_black", "")
	if err != nil {
		t.Fatalf("expected black guest creation to succeed, got %v", err)
	}
	whiteAccountSession, err := accounts.ClaimGuest(whiteSession.Guest, "white_owner")
	if err != nil {
		t.Fatalf("expected white account claim to succeed, got %v", err)
	}
	blackAccountSession, err := accounts.ClaimGuest(blackSession.Guest, "black_owner")
	if err != nil {
		t.Fatalf("expected black account claim to succeed, got %v", err)
	}

	now := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	if err := archive.Upsert(contracts.MatchSnapshotResponse{
		Match: contracts.MatchState{
			MatchID:      "rated_account_room",
			RulesVersion: "v1-alpha-foundation",
			Queue:        "rated",
			WhiteGuestID: whiteSession.Guest.GuestID,
			BlackGuestID: blackSession.Guest.GuestID,
			Status:       "finished",
			Winner:       "white",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}); err != nil {
		t.Fatalf("expected archive upsert to succeed, got %v", err)
	}

	body := `{"matchId":"rated_account_room","whiteAccountId":"` + whiteAccountSession.Account.AccountID + `","blackAccountId":"` + blackAccountSession.Account.AccountID + `","winner":"white"}`
	req := httptest.NewRequest(http.MethodPost, "/api/platform/account-results", strings.NewReader(body))
	rec := httptest.NewRecorder()

	buildPlatformMux(archive, guests, accounts, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected rated account result to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		Changed      bool                          `json:"changed"`
		White        platform.GuestProfile         `json:"white"`
		Black        platform.GuestProfile         `json:"black"`
		WhiteAccount platform.PublicAccountProfile `json:"whiteAccount"`
		BlackAccount platform.PublicAccountProfile `json:"blackAccount"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected successful account result response to decode, got %v", err)
	}
	if !response.Changed || response.White.Rating != 1216 || response.Black.Rating != 1184 {
		t.Fatalf("expected account result to update guest ratings, got %#v", response)
	}
	if response.WhiteAccount.AccountID != whiteAccountSession.Account.AccountID || response.WhiteAccount.Rating != 1216 || response.WhiteAccount.Wins != 1 || response.WhiteAccount.MatchesPlayed != 1 {
		t.Fatalf("expected white account summary to reflect finalized result, got %#v", response.WhiteAccount)
	}
	if response.BlackAccount.AccountID != blackAccountSession.Account.AccountID || response.BlackAccount.Rating != 1184 || response.BlackAccount.Losses != 1 || response.BlackAccount.MatchesPlayed != 1 {
		t.Fatalf("expected black account summary to reflect finalized result, got %#v", response.BlackAccount)
	}
}

func TestAccountResultsRejectArchivedSeatMismatch(t *testing.T) {
	tempDir := t.TempDir()
	archive, err := platform.NewMatchArchiveStore(filepath.Join(tempDir, "archive.json"))
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}
	guests, err := platform.NewGuestStore(filepath.Join(tempDir, "guests.json"))
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}
	accounts, err := platform.NewAccountStore(filepath.Join(tempDir, "accounts.json"))
	if err != nil {
		t.Fatalf("expected account store to initialize, got %v", err)
	}
	claims := platform.NewMatchClaimStore()

	whiteSession, err := guests.EnsureGuest("guest_white", "")
	if err != nil {
		t.Fatalf("expected white guest creation to succeed, got %v", err)
	}
	blackSession, err := guests.EnsureGuest("guest_black", "")
	if err != nil {
		t.Fatalf("expected black guest creation to succeed, got %v", err)
	}
	whiteAccountSession, err := accounts.ClaimGuest(whiteSession.Guest, "white_owner")
	if err != nil {
		t.Fatalf("expected white account claim to succeed, got %v", err)
	}
	blackAccountSession, err := accounts.ClaimGuest(blackSession.Guest, "black_owner")
	if err != nil {
		t.Fatalf("expected black account claim to succeed, got %v", err)
	}

	now := time.Date(2026, 5, 7, 10, 30, 0, 0, time.UTC)
	if err := archive.Upsert(contracts.MatchSnapshotResponse{
		Match: contracts.MatchState{
			MatchID:        "rated_account_room_mismatch",
			RulesVersion:   "v1-alpha-foundation",
			Queue:          "rated",
			WhiteGuestID:   whiteSession.Guest.GuestID,
			BlackGuestID:   blackSession.Guest.GuestID,
			WhiteAccountID: whiteAccountSession.Account.AccountID,
			BlackAccountID: blackAccountSession.Account.AccountID,
			Status:         "finished",
			Winner:         "white",
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	}); err != nil {
		t.Fatalf("expected archive upsert to succeed, got %v", err)
	}

	body := `{"matchId":"rated_account_room_mismatch","whiteAccountId":"` + blackAccountSession.Account.AccountID + `","blackAccountId":"` + whiteAccountSession.Account.AccountID + `","winner":"white"}`
	req := httptest.NewRequest(http.MethodPost, "/api/platform/account-results", strings.NewReader(body))
	rec := httptest.NewRecorder()

	buildPlatformMux(archive, guests, accounts, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected mismatched account result to be rejected, got status %d body=%s", rec.Code, rec.Body.String())
	}
	refreshedWhite, _ := guests.GetGuest("guest_white")
	refreshedBlack, _ := guests.GetGuest("guest_black")
	if refreshedWhite.Rating != 1200 || refreshedBlack.Rating != 1200 {
		t.Fatalf("expected rejected account result to preserve ratings, got %#v %#v", refreshedWhite, refreshedBlack)
	}
}
