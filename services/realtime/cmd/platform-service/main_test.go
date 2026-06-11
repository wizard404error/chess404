package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chess404/realtime/internal/contracts"
	"github.com/chess404/realtime/internal/platform"
)

func TestMain(m *testing.M) {
	if os.Getenv("ACCOUNT_AUTH_PUBLIC_BASE_URL") == "" {
		_ = os.Setenv("ACCOUNT_AUTH_PUBLIC_BASE_URL", "https://example.test")
	}
	os.Exit(m.Run())
}

func buildTestPlatformMux(t *testing.T, archive *platform.MatchArchiveStore, guests platform.GuestDirectory, claims *platform.MatchClaimStore) http.Handler {
	t.Helper()
	accounts, err := platform.NewAccountStore("")
	if err != nil {
		t.Fatalf("expected account store to initialize, got %v", err)
	}
	return buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims)
}

func buildTestPlatformMuxWithAccounts(t *testing.T, archive *platform.MatchArchiveStore, guests platform.GuestDirectory, accounts platform.AccountDirectory, claims *platform.MatchClaimStore) http.Handler {
	t.Helper()
	friends, err := platform.NewFriendshipStore("")
	if err != nil {
		t.Fatalf("expected friendship store to initialize, got %v", err)
	}
	moderation, err := platform.NewModerationStore("")
	if err != nil {
		t.Fatalf("expected moderation store to initialize, got %v", err)
	}
	challenges, err := platform.NewDirectChallengeStore("")
	if err != nil {
		t.Fatalf("expected direct challenge store to initialize, got %v", err)
	}
	notifications, err := platform.NewAccountNotificationStore("")
	if err != nil {
		t.Fatalf("expected notification store to initialize, got %v", err)
	}
	emailOutbox, err := platform.NewAccountEmailOutboxStore("")
	if err != nil {
		t.Fatalf("expected account email outbox to initialize, got %v", err)
	}
	securityAudit, err := platform.NewAccountSecurityAuditStore("")
	if err != nil {
		t.Fatalf("expected account security audit store to initialize, got %v", err)
	}
	return buildPlatformMux(archive, guests, accounts, friends, moderation, challenges, notifications, emailOutbox, securityAudit, claims)
}

const testInternalServiceToken = "test-internal-token"

func authorizeInternalPlatformRequest(t *testing.T, req *http.Request) {
	t.Helper()
	t.Setenv("PLATFORM_INTERNAL_SERVICE_TOKEN", testInternalServiceToken)
	req.Header.Set("Authorization", "Bearer "+testInternalServiceToken)
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

	buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims).ServeHTTP(rec, req)

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

	buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims).ServeHTTP(rec, req)

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

	buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims).ServeHTTP(rec, req)

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

func TestWriteAccountRestrictionErrorIncludesRestrictionDetails(t *testing.T) {
	rec := httptest.NewRecorder()

	writeAccountRestrictionError(rec, platform.AccountRestriction{
		AccountID: "acct_restricted",
		Kind:      platform.AccountRestrictionKindSuspended,
		Reason:    "Repeated harassment reports",
	})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden restriction response, got %d", rec.Code)
	}

	var payload struct {
		Error             string `json:"error"`
		RestrictionKind   string `json:"restrictionKind"`
		RestrictionReason string `json:"restrictionReason"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected restriction response to decode, got %v", err)
	}
	if payload.Error != "account suspended" {
		t.Fatalf("expected suspended error message, got %#v", payload)
	}
	if payload.RestrictionKind != platform.AccountRestrictionKindSuspended {
		t.Fatalf("expected suspended restriction kind, got %#v", payload)
	}
	if payload.RestrictionReason != "Repeated harassment reports" {
		t.Fatalf("expected restriction reason to round-trip, got %#v", payload)
	}
}

func TestAccountAuthLoginRestoresGuestSession(t *testing.T) {
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

	guestSession, err := guests.EnsureGuest("guest_login", "")
	if err != nil {
		t.Fatalf("expected guest session creation to succeed, got %v", err)
	}
	accountSession, err := accounts.ClaimGuest(guestSession.Guest, "aurora_login")
	if err != nil {
		t.Fatalf("expected account claim to succeed, got %v", err)
	}
	if _, err := accounts.EnablePasswordLogin(accountSession.Account.AccountID, accountSession.SessionToken, "aurora@example.com", "Swordfish88"); err != nil {
		t.Fatalf("expected password login setup to succeed, got %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/platform/account-auth/login", strings.NewReader(`{"identifier":"aurora@example.com","password":"Swordfish88"}`))
	rec := httptest.NewRecorder()

	buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected account login to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		Account platform.AccountSession `json:"account"`
		Guest   platform.GuestSession   `json:"guest"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected account login response to decode, got %v", err)
	}
	if response.Account.Account.AccountID != accountSession.Account.AccountID || response.Guest.Guest.GuestID != guestSession.Guest.GuestID || response.Guest.SessionSecret == "" {
		t.Fatalf("unexpected account login response %#v", response)
	}
}

func TestAccountAuthRegisterCreatesAccountAndQueuesVerification(t *testing.T) {
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

	guestSession, err := guests.EnsureGuest("guest_register", "")
	if err != nil {
		t.Fatalf("expected guest session creation to succeed, got %v", err)
	}

	body := `{"guestId":"` + guestSession.Guest.GuestID + `","sessionToken":"` + guestSession.SessionToken + `","handle":"Aurora_Register","email":"register@example.com","password":"Swordfish88"}`
	req := httptest.NewRequest(http.MethodPost, "/api/platform/account-auth/register", strings.NewReader(body))
	rec := httptest.NewRecorder()

	buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected account registration to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		Account               platform.AccountSession       `json:"account"`
		Guest                 platform.GuestSession         `json:"guest"`
		Overview              platform.AccountAuthOverview  `json:"overview"`
		Delivery              platform.AccountEmailDelivery `json:"delivery"`
		RequestedVerification bool                          `json:"requestedVerification"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected registration response to decode, got %v", err)
	}
	if response.Account.Account.Handle != "aurora_register" || response.Account.Account.PrimaryGuestID != guestSession.Guest.GuestID {
		t.Fatalf("unexpected account registration response %#v", response)
	}
	if response.Guest.Guest.GuestID != guestSession.Guest.GuestID || strings.TrimSpace(response.Guest.SessionSecret) == "" {
		t.Fatalf("expected guest bridge in registration response, got %#v", response.Guest)
	}
	if !response.Overview.PasswordLoginEnabled || response.Overview.Email != "register@example.com" {
		t.Fatalf("expected password login enabled overview, got %#v", response.Overview)
	}
	if !response.RequestedVerification || response.Delivery.Kind != platform.AccountEmailDeliveryKindEmailVerification || response.Delivery.Email != "register@example.com" {
		t.Fatalf("expected queued verification delivery, got %#v", response)
	}
}

func TestAccountAuthRegisterRejectsDuplicateEmail(t *testing.T) {
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
	handler := buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims)

	firstGuest, err := guests.EnsureGuest("guest_register_alpha", "")
	if err != nil {
		t.Fatalf("expected first guest session creation to succeed, got %v", err)
	}
	secondGuest, err := guests.EnsureGuest("guest_register_beta", "")
	if err != nil {
		t.Fatalf("expected second guest session creation to succeed, got %v", err)
	}

	firstReq := httptest.NewRequest(http.MethodPost, "/api/platform/account-auth/register", strings.NewReader(`{"guestId":"`+firstGuest.Guest.GuestID+`","sessionToken":"`+firstGuest.SessionToken+`","handle":"alpha_register","email":"duplicate@example.com","password":"Swordfish88"}`))
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("expected first registration to succeed, got status %d body=%s", firstRec.Code, firstRec.Body.String())
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/api/platform/account-auth/register", strings.NewReader(`{"guestId":"`+secondGuest.Guest.GuestID+`","sessionToken":"`+secondGuest.SessionToken+`","handle":"beta_register","email":"duplicate@example.com","password":"Swordfish99"}`))
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, secondReq)

	if secondRec.Code != http.StatusConflict {
		t.Fatalf("expected duplicate email registration to be rejected, got status %d body=%s", secondRec.Code, secondRec.Body.String())
	}
}

func TestAccountAuthLoginRateLimitReturnsTooManyRequests(t *testing.T) {
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
	handler := buildTestPlatformMux(t, archive, guests, claims)

	body := `{"identifier":"aurora@example.com","password":"wrong-password"}`
	for attempt := 0; attempt < loginRateLimitPerIdentifier; attempt++ {
		req := httptest.NewRequest(http.MethodPost, "/api/platform/account-auth/login", strings.NewReader(body))
		req.RemoteAddr = "203.0.113.55:4000"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected unauthorized login before throttling, got status %d body=%s", rec.Code, rec.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/platform/account-auth/login", strings.NewReader(body))
	req.RemoteAddr = "203.0.113.55:4000"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected login throttling to return 429, got status %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.TrimSpace(rec.Header().Get("Retry-After")) == "" {
		t.Fatalf("expected login throttling to include Retry-After header")
	}
}

func TestAccountAuthVerificationRequestQueuesEmailDelivery(t *testing.T) {
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

	accountSession, err := accounts.ClaimGuest(platform.GuestProfile{GuestID: "guest_verify_delivery"}, "aurora_verify")
	if err != nil {
		t.Fatalf("expected account claim to succeed, got %v", err)
	}
	if _, err := accounts.EnablePasswordLogin(accountSession.Account.AccountID, accountSession.SessionToken, "verify@example.com", "Swordfish88"); err != nil {
		t.Fatalf("expected password login setup to succeed, got %v", err)
	}
	handler := buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims)

	req := httptest.NewRequest(http.MethodPost, "/api/platform/account-auth/email-verification/request", strings.NewReader(`{"accountId":"`+accountSession.Account.AccountID+`","sessionToken":"`+accountSession.SessionToken+`"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected verification request to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		Requested bool                          `json:"requested"`
		Delivery  platform.AccountEmailDelivery `json:"delivery"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected verification request response to decode, got %v", err)
	}
	if !response.Requested || response.Delivery.Kind != platform.AccountEmailDeliveryKindEmailVerification || response.Delivery.Email != "verify@example.com" {
		t.Fatalf("unexpected verification delivery response %#v", response)
	}

	overviewReq := httptest.NewRequest(http.MethodPost, "/api/platform/email-outbox/overview", strings.NewReader(`{"accountId":"`+accountSession.Account.AccountID+`","sessionToken":"`+accountSession.SessionToken+`"}`))
	overviewRec := httptest.NewRecorder()
	handler.ServeHTTP(overviewRec, overviewReq)
	if overviewRec.Code != http.StatusOK {
		t.Fatalf("expected email outbox overview to succeed, got status %d body=%s", overviewRec.Code, overviewRec.Body.String())
	}
	var overview platform.AccountEmailDeliveryOverview
	if err := json.Unmarshal(overviewRec.Body.Bytes(), &overview); err != nil {
		t.Fatalf("expected email outbox overview to decode, got %v", err)
	}
	if len(overview.Deliveries) == 0 || overview.Deliveries[0].Kind != platform.AccountEmailDeliveryKindEmailVerification {
		t.Fatalf("expected verification email delivery in outbox, got %#v", overview)
	}
}

func TestAccountAuthPasswordResetRequestRateLimitReturnsTooManyRequests(t *testing.T) {
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
	handler := buildTestPlatformMux(t, archive, guests, claims)

	body := `{"identifier":"recover@example.com"}`
	for attempt := 0; attempt < passwordResetRateLimitPerID; attempt++ {
		req := httptest.NewRequest(http.MethodPost, "/api/platform/account-auth/password-reset/request", strings.NewReader(body))
		req.RemoteAddr = "203.0.113.77:4010"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected password reset preview request before throttling to succeed, got status %d body=%s", rec.Code, rec.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/platform/account-auth/password-reset/request", strings.NewReader(body))
	req.RemoteAddr = "203.0.113.77:4010"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected password reset throttling to return 429, got status %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.TrimSpace(rec.Header().Get("Retry-After")) == "" {
		t.Fatalf("expected password reset throttling to include Retry-After header")
	}
}

func TestAccountAuthPasswordResetRequestQueuesEmailDelivery(t *testing.T) {
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

	accountSession, err := accounts.ClaimGuest(platform.GuestProfile{GuestID: "guest_reset_delivery"}, "aurora_reset")
	if err != nil {
		t.Fatalf("expected account claim to succeed, got %v", err)
	}
	if _, err := accounts.EnablePasswordLogin(accountSession.Account.AccountID, accountSession.SessionToken, "reset@example.com", "Swordfish88"); err != nil {
		t.Fatalf("expected password login setup to succeed, got %v", err)
	}
	verifyChallenge, err := accounts.StartEmailVerification(accountSession.Account.AccountID, accountSession.SessionToken)
	if err != nil {
		t.Fatalf("expected direct verification challenge to succeed, got %v", err)
	}
	if _, err := accounts.VerifyEmail(accountSession.Account.AccountID, verifyChallenge.Token); err != nil {
		t.Fatalf("expected direct verification confirmation to succeed, got %v", err)
	}
	handler := buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims)

	req := httptest.NewRequest(http.MethodPost, "/api/platform/account-auth/password-reset/request", strings.NewReader(`{"identifier":"reset@example.com"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected password reset request to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		Requested bool                           `json:"requested"`
		Delivery  *platform.AccountEmailDelivery `json:"delivery"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected password reset response to decode, got %v", err)
	}
	if !response.Requested || response.Delivery == nil || response.Delivery.Kind != platform.AccountEmailDeliveryKindPasswordReset || response.Delivery.Email != "reset@example.com" {
		t.Fatalf("unexpected password reset delivery response %#v", response)
	}
}

func TestAccountAuthLogoutInvalidatesSession(t *testing.T) {
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

	accountSession, err := accounts.ClaimGuest(platform.GuestProfile{GuestID: "guest_logout"}, "aurora_logout")
	if err != nil {
		t.Fatalf("expected account claim to succeed, got %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/platform/account-auth/logout", strings.NewReader(`{"accountId":"`+accountSession.Account.AccountID+`","sessionToken":"`+accountSession.SessionToken+`"}`))
	rec := httptest.NewRecorder()

	buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected account logout to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}

	if _, err := accounts.ResumeAccount(accountSession.Account.AccountID, accountSession.SessionToken); err != platform.ErrUnauthorizedAccountSession {
		t.Fatalf("expected logged-out account session to be invalid, got %v", err)
	}
}

func TestAccountSessionOverviewAndRevokeOthers(t *testing.T) {
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

	accountSession, err := accounts.ClaimGuest(platform.GuestProfile{GuestID: "guest_account_overview"}, "aurora_overview")
	if err != nil {
		t.Fatalf("expected account claim to succeed, got %v", err)
	}
	if _, err := accounts.EnablePasswordLogin(accountSession.Account.AccountID, accountSession.SessionToken, "overview@example.com", "Swordfish88"); err != nil {
		t.Fatalf("expected password login setup to succeed, got %v", err)
	}
	otherSession, err := accounts.LoginWithPassword("overview@example.com", "Swordfish88")
	if err != nil {
		t.Fatalf("expected second account session to succeed, got %v", err)
	}

	overviewReq := httptest.NewRequest(http.MethodPost, "/api/platform/account-sessions/overview", strings.NewReader(`{"accountId":"`+accountSession.Account.AccountID+`","sessionToken":"`+accountSession.SessionToken+`"}`))
	overviewRec := httptest.NewRecorder()
	buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims).ServeHTTP(overviewRec, overviewReq)
	if overviewRec.Code != http.StatusOK {
		t.Fatalf("expected session overview to succeed, got status %d body=%s", overviewRec.Code, overviewRec.Body.String())
	}
	var overview platform.AccountSessionOverview
	if err := json.Unmarshal(overviewRec.Body.Bytes(), &overview); err != nil {
		t.Fatalf("expected session overview response to decode, got %v", err)
	}
	if len(overview.Sessions) != 2 {
		t.Fatalf("expected two active account sessions, got %#v", overview.Sessions)
	}

	revokeReq := httptest.NewRequest(http.MethodPost, "/api/platform/account-sessions/revoke-others", strings.NewReader(`{"accountId":"`+accountSession.Account.AccountID+`","sessionToken":"`+accountSession.SessionToken+`"}`))
	revokeRec := httptest.NewRecorder()
	buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims).ServeHTTP(revokeRec, revokeReq)
	if revokeRec.Code != http.StatusNoContent {
		t.Fatalf("expected revoke-others to succeed, got status %d body=%s", revokeRec.Code, revokeRec.Body.String())
	}
	if _, err := accounts.ResumeAccount(otherSession.Account.AccountID, otherSession.SessionToken); err != platform.ErrUnauthorizedAccountSession {
		t.Fatalf("expected other account session to be invalid after revoke-others, got %v", err)
	}
}

func TestAccountPresenceTouchReturnsFreshPresence(t *testing.T) {
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

	accountSession, err := accounts.ClaimGuest(platform.GuestProfile{GuestID: "guest_presence"}, "aurora_presence")
	if err != nil {
		t.Fatalf("expected account claim to succeed, got %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/platform/account-presence", strings.NewReader(`{"accountId":"`+accountSession.Account.AccountID+`","sessionToken":"`+accountSession.SessionToken+`"}`))
	rec := httptest.NewRecorder()

	buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected account presence touch to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}

	var response platform.AccountSession
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected account presence response to decode, got %v", err)
	}
	if response.Account.AccountID != accountSession.Account.AccountID {
		t.Fatalf("expected account presence response for %s, got %#v", accountSession.Account.AccountID, response)
	}
	if response.Account.LastActiveAt.IsZero() {
		t.Fatalf("expected account presence response to include lastActiveAt, got %#v", response)
	}
	if time.Since(response.Account.LastActiveAt) > time.Minute {
		t.Fatalf("expected fresh lastActiveAt, got %#v", response.Account.LastActiveAt)
	}
}

func TestFriendRequestsCreateAndAcceptFriendship(t *testing.T) {
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
	friends, err := platform.NewFriendshipStore(filepath.Join(tempDir, "friends.json"))
	if err != nil {
		t.Fatalf("expected friendship store to initialize, got %v", err)
	}
	defer func() { _ = friends.Close() }()
	moderation, err := platform.NewModerationStore(filepath.Join(tempDir, "moderation.json"))
	if err != nil {
		t.Fatalf("expected moderation store to initialize, got %v", err)
	}
	defer func() { _ = moderation.Close() }()
	challenges, err := platform.NewDirectChallengeStore(filepath.Join(tempDir, "challenges.json"))
	if err != nil {
		t.Fatalf("expected direct challenge store to initialize, got %v", err)
	}
	defer func() { _ = challenges.Close() }()
	notifications, err := platform.NewAccountNotificationStore(filepath.Join(tempDir, "notifications.json"))
	if err != nil {
		t.Fatalf("expected notification store to initialize, got %v", err)
	}
	defer func() { _ = notifications.Close() }()
	emailOutbox, err := platform.NewAccountEmailOutboxStore(filepath.Join(tempDir, "email-outbox.json"))
	if err != nil {
		t.Fatalf("expected account email outbox to initialize, got %v", err)
	}
	defer func() { _ = emailOutbox.Close() }()
	securityAudit, err := platform.NewAccountSecurityAuditStore(filepath.Join(tempDir, "account-security-audit.json"))
	if err != nil {
		t.Fatalf("expected account security audit store to initialize, got %v", err)
	}
	defer func() { _ = securityAudit.Close() }()
	claims := platform.NewMatchClaimStore()

	whiteGuest, err := guests.EnsureGuest("guest_friend_white", "")
	if err != nil {
		t.Fatalf("expected white guest session, got %v", err)
	}
	blackGuest, err := guests.EnsureGuest("guest_friend_black", "")
	if err != nil {
		t.Fatalf("expected black guest session, got %v", err)
	}
	whiteAccount, err := accounts.ClaimGuest(whiteGuest.Guest, "aurora_friend")
	if err != nil {
		t.Fatalf("expected white account claim, got %v", err)
	}
	blackAccount, err := accounts.ClaimGuest(blackGuest.Guest, "nova_friend")
	if err != nil {
		t.Fatalf("expected black account claim, got %v", err)
	}

	sendReq := httptest.NewRequest(http.MethodPost, "/api/platform/friends/requests", strings.NewReader(`{"accountId":"`+whiteAccount.Account.AccountID+`","sessionToken":"`+whiteAccount.SessionToken+`","targetHandle":"nova_friend"}`))
	sendRec := httptest.NewRecorder()

	buildPlatformMux(archive, guests, accounts, friends, moderation, challenges, notifications, emailOutbox, securityAudit, claims).ServeHTTP(sendRec, sendReq)

	if sendRec.Code != http.StatusOK {
		t.Fatalf("expected friend request send to succeed, got status %d body=%s", sendRec.Code, sendRec.Body.String())
	}

	var sendResponse struct {
		Outgoing []struct {
			RequestID string `json:"requestId"`
		} `json:"outgoing"`
	}
	if err := json.Unmarshal(sendRec.Body.Bytes(), &sendResponse); err != nil {
		t.Fatalf("expected friend request response to decode, got %v", err)
	}
	if len(sendResponse.Outgoing) != 1 || strings.TrimSpace(sendResponse.Outgoing[0].RequestID) == "" {
		t.Fatalf("expected outgoing friend request, got %#v", sendResponse)
	}

	acceptReq := httptest.NewRequest(http.MethodPost, "/api/platform/friends/requests/"+sendResponse.Outgoing[0].RequestID+"/respond", strings.NewReader(`{"accountId":"`+blackAccount.Account.AccountID+`","sessionToken":"`+blackAccount.SessionToken+`","accept":true}`))
	acceptRec := httptest.NewRecorder()

	buildPlatformMux(archive, guests, accounts, friends, moderation, challenges, notifications, emailOutbox, securityAudit, claims).ServeHTTP(acceptRec, acceptReq)

	if acceptRec.Code != http.StatusOK {
		t.Fatalf("expected friend request acceptance to succeed, got status %d body=%s", acceptRec.Code, acceptRec.Body.String())
	}

	var acceptResponse struct {
		Friends []struct {
			Account struct {
				Handle string `json:"handle"`
			} `json:"account"`
		} `json:"friends"`
		Incoming []any `json:"incoming"`
		Outgoing []any `json:"outgoing"`
	}
	if err := json.Unmarshal(acceptRec.Body.Bytes(), &acceptResponse); err != nil {
		t.Fatalf("expected accepted friend response to decode, got %v", err)
	}
	if len(acceptResponse.Friends) != 1 || acceptResponse.Friends[0].Account.Handle != "aurora_friend" {
		t.Fatalf("expected accepted friend response to include aurora_friend, got %#v", acceptResponse)
	}
	if len(acceptResponse.Incoming) != 0 || len(acceptResponse.Outgoing) != 0 {
		t.Fatalf("expected pending requests cleared after acceptance, got %#v", acceptResponse)
	}

	inboxReq := httptest.NewRequest(http.MethodPost, "/api/platform/inbox/overview", strings.NewReader(`{"accountId":"`+blackAccount.Account.AccountID+`","sessionToken":"`+blackAccount.SessionToken+`","limit":24}`))
	inboxRec := httptest.NewRecorder()
	buildPlatformMux(archive, guests, accounts, friends, moderation, challenges, notifications, emailOutbox, securityAudit, claims).ServeHTTP(inboxRec, inboxReq)
	if inboxRec.Code != http.StatusOK {
		t.Fatalf("expected inbox overview to succeed, got status %d body=%s", inboxRec.Code, inboxRec.Body.String())
	}
	var inboxResponse struct {
		UnreadCount   int `json:"unreadCount"`
		Notifications []struct {
			Kind string `json:"kind"`
		} `json:"notifications"`
	}
	if err := json.Unmarshal(inboxRec.Body.Bytes(), &inboxResponse); err != nil {
		t.Fatalf("expected inbox overview to decode, got %v", err)
	}
	if inboxResponse.UnreadCount != 1 || len(inboxResponse.Notifications) != 1 || inboxResponse.Notifications[0].Kind != platform.AccountNotificationKindFriendRequestReceived {
		t.Fatalf("unexpected inbox response after incoming friend request %#v", inboxResponse)
	}
}

func TestDirectChallengesCreateAndDeclineThroughPlatformAPI(t *testing.T) {
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
	friends, err := platform.NewFriendshipStore(filepath.Join(tempDir, "friends.json"))
	if err != nil {
		t.Fatalf("expected friendship store to initialize, got %v", err)
	}
	defer func() { _ = friends.Close() }()
	moderation, err := platform.NewModerationStore(filepath.Join(tempDir, "moderation.json"))
	if err != nil {
		t.Fatalf("expected moderation store to initialize, got %v", err)
	}
	defer func() { _ = moderation.Close() }()
	challenges, err := platform.NewDirectChallengeStore(filepath.Join(tempDir, "challenges.json"))
	if err != nil {
		t.Fatalf("expected direct challenge store to initialize, got %v", err)
	}
	defer func() { _ = challenges.Close() }()
	notifications, err := platform.NewAccountNotificationStore(filepath.Join(tempDir, "notifications.json"))
	if err != nil {
		t.Fatalf("expected notification store to initialize, got %v", err)
	}
	defer func() { _ = notifications.Close() }()
	emailOutbox, err := platform.NewAccountEmailOutboxStore(filepath.Join(tempDir, "email-outbox.json"))
	if err != nil {
		t.Fatalf("expected account email outbox to initialize, got %v", err)
	}
	defer func() { _ = emailOutbox.Close() }()
	securityAudit, err := platform.NewAccountSecurityAuditStore(filepath.Join(tempDir, "account-security-audit.json"))
	if err != nil {
		t.Fatalf("expected account security audit store to initialize, got %v", err)
	}
	defer func() { _ = securityAudit.Close() }()
	claims := platform.NewMatchClaimStore()

	alphaGuest, err := guests.EnsureGuest("guest_challenge_alpha", "")
	if err != nil {
		t.Fatalf("expected alpha guest session, got %v", err)
	}
	betaGuest, err := guests.EnsureGuest("guest_challenge_beta", "")
	if err != nil {
		t.Fatalf("expected beta guest session, got %v", err)
	}
	alphaAccount, err := accounts.ClaimGuest(alphaGuest.Guest, "alpha_challenge")
	if err != nil {
		t.Fatalf("expected alpha account claim, got %v", err)
	}
	betaAccount, err := accounts.ClaimGuest(betaGuest.Guest, "beta_challenge")
	if err != nil {
		t.Fatalf("expected beta account claim, got %v", err)
	}
	if _, err := friends.SendRequest(alphaAccount.Account.AccountID, betaAccount.Account.AccountID); err != nil {
		t.Fatalf("expected friend request send, got %v", err)
	}
	if _, err := friends.RespondToRequest(betaAccount.Account.AccountID, friends.ListOverview(alphaAccount.Account.AccountID).Outgoing[0].RequestID, true); err != nil {
		t.Fatalf("expected friend request acceptance, got %v", err)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/platform/challenges", strings.NewReader(`{"accountId":"`+alphaAccount.Account.AccountID+`","sessionToken":"`+alphaAccount.SessionToken+`","targetAccountId":"`+betaAccount.Account.AccountID+`","matchId":"room-challenge-1","modeId":"hidden_cards","clockSeconds":900,"challengerSeat":"black"}`))
	createRec := httptest.NewRecorder()
	buildPlatformMux(archive, guests, accounts, friends, moderation, challenges, notifications, emailOutbox, securityAudit, claims).ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusOK {
		t.Fatalf("expected direct challenge create to succeed, got status %d body=%s", createRec.Code, createRec.Body.String())
	}

	var createResponse struct {
		ChallengeID string `json:"challengeId"`
		Status      string `json:"status"`
		ModeID      string `json:"modeId"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &createResponse); err != nil {
		t.Fatalf("expected direct challenge response to decode, got %v", err)
	}
	if createResponse.ChallengeID == "" || createResponse.Status != platform.DirectChallengeStatusPending || createResponse.ModeID != string(contracts.MatchModeHiddenCards) {
		t.Fatalf("unexpected direct challenge create response %#v", createResponse)
	}

	declineReq := httptest.NewRequest(http.MethodPost, "/api/platform/challenges/"+createResponse.ChallengeID+"/respond", strings.NewReader(`{"accountId":"`+betaAccount.Account.AccountID+`","sessionToken":"`+betaAccount.SessionToken+`","accept":false}`))
	declineRec := httptest.NewRecorder()
	buildPlatformMux(archive, guests, accounts, friends, moderation, challenges, notifications, emailOutbox, securityAudit, claims).ServeHTTP(declineRec, declineReq)

	if declineRec.Code != http.StatusOK {
		t.Fatalf("expected direct challenge decline to succeed, got status %d body=%s", declineRec.Code, declineRec.Body.String())
	}

	var overviewResponse struct {
		Incoming []any `json:"incoming"`
		Outgoing []any `json:"outgoing"`
	}
	overviewReq := httptest.NewRequest(http.MethodPost, "/api/platform/challenges/overview", strings.NewReader(`{"accountId":"`+betaAccount.Account.AccountID+`","sessionToken":"`+betaAccount.SessionToken+`"}`))
	overviewRec := httptest.NewRecorder()
	buildPlatformMux(archive, guests, accounts, friends, moderation, challenges, notifications, emailOutbox, securityAudit, claims).ServeHTTP(overviewRec, overviewReq)
	if overviewRec.Code != http.StatusOK {
		t.Fatalf("expected direct challenge overview to succeed, got status %d body=%s", overviewRec.Code, overviewRec.Body.String())
	}
	if err := json.Unmarshal(overviewRec.Body.Bytes(), &overviewResponse); err != nil {
		t.Fatalf("expected direct challenge overview to decode, got %v", err)
	}
	if len(overviewResponse.Incoming) != 0 || len(overviewResponse.Outgoing) != 0 {
		t.Fatalf("expected declined challenge to disappear from pending overview, got %#v", overviewResponse)
	}

	inboxReq := httptest.NewRequest(http.MethodPost, "/api/platform/inbox/overview", strings.NewReader(`{"accountId":"`+alphaAccount.Account.AccountID+`","sessionToken":"`+alphaAccount.SessionToken+`","limit":24}`))
	inboxRec := httptest.NewRecorder()
	buildPlatformMux(archive, guests, accounts, friends, moderation, challenges, notifications, emailOutbox, securityAudit, claims).ServeHTTP(inboxRec, inboxReq)
	if inboxRec.Code != http.StatusOK {
		t.Fatalf("expected challenger inbox overview to succeed, got status %d body=%s", inboxRec.Code, inboxRec.Body.String())
	}
	var inboxResponse struct {
		UnreadCount   int `json:"unreadCount"`
		Notifications []struct {
			Kind string `json:"kind"`
		} `json:"notifications"`
	}
	if err := json.Unmarshal(inboxRec.Body.Bytes(), &inboxResponse); err != nil {
		t.Fatalf("expected challenger inbox overview to decode, got %v", err)
	}
	if inboxResponse.UnreadCount != 1 || len(inboxResponse.Notifications) == 0 || inboxResponse.Notifications[0].Kind != platform.AccountNotificationKindDirectChallengeDeclined {
		t.Fatalf("unexpected challenger inbox response %#v", inboxResponse)
	}
}

func TestModerationBlocksFriendRequestsAndDirectChallengesThroughPlatformAPI(t *testing.T) {
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
	friends, err := platform.NewFriendshipStore(filepath.Join(tempDir, "friends.json"))
	if err != nil {
		t.Fatalf("expected friendship store to initialize, got %v", err)
	}
	defer func() { _ = friends.Close() }()
	moderation, err := platform.NewModerationStore(filepath.Join(tempDir, "moderation.json"))
	if err != nil {
		t.Fatalf("expected moderation store to initialize, got %v", err)
	}
	defer func() { _ = moderation.Close() }()
	challenges, err := platform.NewDirectChallengeStore(filepath.Join(tempDir, "challenges.json"))
	if err != nil {
		t.Fatalf("expected direct challenge store to initialize, got %v", err)
	}
	defer func() { _ = challenges.Close() }()
	notifications, err := platform.NewAccountNotificationStore(filepath.Join(tempDir, "notifications.json"))
	if err != nil {
		t.Fatalf("expected notification store to initialize, got %v", err)
	}
	defer func() { _ = notifications.Close() }()
	emailOutbox, err := platform.NewAccountEmailOutboxStore(filepath.Join(tempDir, "email-outbox.json"))
	if err != nil {
		t.Fatalf("expected account email outbox to initialize, got %v", err)
	}
	defer func() { _ = emailOutbox.Close() }()
	securityAudit, err := platform.NewAccountSecurityAuditStore(filepath.Join(tempDir, "account-security-audit.json"))
	if err != nil {
		t.Fatalf("expected account security audit store to initialize, got %v", err)
	}
	defer func() { _ = securityAudit.Close() }()
	claims := platform.NewMatchClaimStore()
	mux := buildPlatformMux(archive, guests, accounts, friends, moderation, challenges, notifications, emailOutbox, securityAudit, claims)

	alphaGuest, err := guests.EnsureGuest("guest_moderation_alpha", "")
	if err != nil {
		t.Fatalf("expected alpha guest session, got %v", err)
	}
	betaGuest, err := guests.EnsureGuest("guest_moderation_beta", "")
	if err != nil {
		t.Fatalf("expected beta guest session, got %v", err)
	}
	alphaAccount, err := accounts.ClaimGuest(alphaGuest.Guest, "alpha_moderation")
	if err != nil {
		t.Fatalf("expected alpha account claim, got %v", err)
	}
	betaAccount, err := accounts.ClaimGuest(betaGuest.Guest, "beta_moderation")
	if err != nil {
		t.Fatalf("expected beta account claim, got %v", err)
	}
	if _, err := friends.SendRequest(alphaAccount.Account.AccountID, betaAccount.Account.AccountID); err != nil {
		t.Fatalf("expected friend request send, got %v", err)
	}
	if _, err := friends.RespondToRequest(betaAccount.Account.AccountID, friends.ListOverview(alphaAccount.Account.AccountID).Outgoing[0].RequestID, true); err != nil {
		t.Fatalf("expected friend request acceptance, got %v", err)
	}

	blockReq := httptest.NewRequest(http.MethodPost, "/api/platform/moderation/blocks", strings.NewReader(`{"accountId":"`+alphaAccount.Account.AccountID+`","sessionToken":"`+alphaAccount.SessionToken+`","targetAccountId":"`+betaAccount.Account.AccountID+`","reason":"spam invites"}`))
	blockRec := httptest.NewRecorder()
	mux.ServeHTTP(blockRec, blockReq)
	if blockRec.Code != http.StatusOK {
		t.Fatalf("expected block request to succeed, got status %d body=%s", blockRec.Code, blockRec.Body.String())
	}

	friendReq := httptest.NewRequest(http.MethodPost, "/api/platform/friends/requests", strings.NewReader(`{"accountId":"`+betaAccount.Account.AccountID+`","sessionToken":"`+betaAccount.SessionToken+`","targetHandle":"alpha_moderation"}`))
	friendRec := httptest.NewRecorder()
	mux.ServeHTTP(friendRec, friendReq)
	if friendRec.Code != http.StatusForbidden {
		t.Fatalf("expected blocked friend request to be rejected, got status %d body=%s", friendRec.Code, friendRec.Body.String())
	}

	challengeReq := httptest.NewRequest(http.MethodPost, "/api/platform/challenges", strings.NewReader(`{"accountId":"`+betaAccount.Account.AccountID+`","sessionToken":"`+betaAccount.SessionToken+`","targetAccountId":"`+alphaAccount.Account.AccountID+`","matchId":"room-blocked-challenge","modeId":"open_cards","clockSeconds":600,"challengerSeat":"white"}`))
	challengeRec := httptest.NewRecorder()
	mux.ServeHTTP(challengeRec, challengeReq)
	if challengeRec.Code != http.StatusForbidden {
		t.Fatalf("expected blocked challenge create to be rejected, got status %d body=%s", challengeRec.Code, challengeRec.Body.String())
	}
}

func TestModerationOverviewIncludesReportsThroughPlatformAPI(t *testing.T) {
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
	friends, err := platform.NewFriendshipStore(filepath.Join(tempDir, "friends.json"))
	if err != nil {
		t.Fatalf("expected friendship store to initialize, got %v", err)
	}
	defer func() { _ = friends.Close() }()
	moderation, err := platform.NewModerationStore(filepath.Join(tempDir, "moderation.json"))
	if err != nil {
		t.Fatalf("expected moderation store to initialize, got %v", err)
	}
	defer func() { _ = moderation.Close() }()
	challenges, err := platform.NewDirectChallengeStore(filepath.Join(tempDir, "challenges.json"))
	if err != nil {
		t.Fatalf("expected direct challenge store to initialize, got %v", err)
	}
	defer func() { _ = challenges.Close() }()
	notifications, err := platform.NewAccountNotificationStore(filepath.Join(tempDir, "notifications.json"))
	if err != nil {
		t.Fatalf("expected notification store to initialize, got %v", err)
	}
	defer func() { _ = notifications.Close() }()
	emailOutbox, err := platform.NewAccountEmailOutboxStore(filepath.Join(tempDir, "email-outbox.json"))
	if err != nil {
		t.Fatalf("expected account email outbox to initialize, got %v", err)
	}
	defer func() { _ = emailOutbox.Close() }()
	securityAudit, err := platform.NewAccountSecurityAuditStore(filepath.Join(tempDir, "account-security-audit.json"))
	if err != nil {
		t.Fatalf("expected account security audit store to initialize, got %v", err)
	}
	defer func() { _ = securityAudit.Close() }()
	claims := platform.NewMatchClaimStore()
	mux := buildPlatformMux(archive, guests, accounts, friends, moderation, challenges, notifications, emailOutbox, securityAudit, claims)

	reporterGuest, err := guests.EnsureGuest("guest_reporter", "")
	if err != nil {
		t.Fatalf("expected reporter guest session, got %v", err)
	}
	targetGuest, err := guests.EnsureGuest("guest_target", "")
	if err != nil {
		t.Fatalf("expected target guest session, got %v", err)
	}
	reporterAccount, err := accounts.ClaimGuest(reporterGuest.Guest, "reporter_alpha")
	if err != nil {
		t.Fatalf("expected reporter account claim, got %v", err)
	}
	targetAccount, err := accounts.ClaimGuest(targetGuest.Guest, "target_beta")
	if err != nil {
		t.Fatalf("expected target account claim, got %v", err)
	}

	reportReq := httptest.NewRequest(http.MethodPost, "/api/platform/moderation/reports", strings.NewReader(`{"accountId":"`+reporterAccount.Account.AccountID+`","sessionToken":"`+reporterAccount.SessionToken+`","targetAccountId":"`+targetAccount.Account.AccountID+`","category":"harassment","details":"kept spamming abuse in match chat"}`))
	reportRec := httptest.NewRecorder()
	mux.ServeHTTP(reportRec, reportReq)
	if reportRec.Code != http.StatusOK {
		t.Fatalf("expected report request to succeed, got status %d body=%s", reportRec.Code, reportRec.Body.String())
	}

	var reportResponse struct {
		SubmittedReports []struct {
			ReportID string `json:"reportId"`
			Category string `json:"category"`
			Target   struct {
				Handle string `json:"handle"`
			} `json:"target"`
		} `json:"submittedReports"`
	}
	if err := json.Unmarshal(reportRec.Body.Bytes(), &reportResponse); err != nil {
		t.Fatalf("expected report response to decode, got %v", err)
	}
	if len(reportResponse.SubmittedReports) != 1 || reportResponse.SubmittedReports[0].Category != "harassment" || reportResponse.SubmittedReports[0].Target.Handle != "target_beta" {
		t.Fatalf("unexpected report overview %#v", reportResponse)
	}
}

func TestModerationAdminOverviewRequiresAdminAccess(t *testing.T) {
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
	mux := buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims)

	adminGuest, err := guests.EnsureGuest("guest_mod_admin", "")
	if err != nil {
		t.Fatalf("expected admin guest session, got %v", err)
	}
	adminAccount, err := accounts.ClaimGuest(adminGuest.Guest, "mod_admin")
	if err != nil {
		t.Fatalf("expected admin account claim, got %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/platform/moderation/admin/overview", strings.NewReader(`{"accountId":"`+adminAccount.Account.AccountID+`","sessionToken":"`+adminAccount.SessionToken+`"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected non-admin moderation overview request to be rejected, got status %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestModerationAdminCanResolveReportsThroughPlatformAPI(t *testing.T) {
	t.Setenv("PLATFORM_ADMIN_HANDLES", "mod_admin")

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
	friends, err := platform.NewFriendshipStore(filepath.Join(tempDir, "friends.json"))
	if err != nil {
		t.Fatalf("expected friendship store to initialize, got %v", err)
	}
	defer func() { _ = friends.Close() }()
	moderation, err := platform.NewModerationStore(filepath.Join(tempDir, "moderation.json"))
	if err != nil {
		t.Fatalf("expected moderation store to initialize, got %v", err)
	}
	defer func() { _ = moderation.Close() }()
	challenges, err := platform.NewDirectChallengeStore(filepath.Join(tempDir, "challenges.json"))
	if err != nil {
		t.Fatalf("expected direct challenge store to initialize, got %v", err)
	}
	defer func() { _ = challenges.Close() }()
	notifications, err := platform.NewAccountNotificationStore(filepath.Join(tempDir, "notifications.json"))
	if err != nil {
		t.Fatalf("expected notification store to initialize, got %v", err)
	}
	defer func() { _ = notifications.Close() }()
	emailOutbox, err := platform.NewAccountEmailOutboxStore(filepath.Join(tempDir, "email-outbox.json"))
	if err != nil {
		t.Fatalf("expected account email outbox to initialize, got %v", err)
	}
	defer func() { _ = emailOutbox.Close() }()
	securityAudit, err := platform.NewAccountSecurityAuditStore(filepath.Join(tempDir, "account-security-audit.json"))
	if err != nil {
		t.Fatalf("expected account security audit store to initialize, got %v", err)
	}
	defer func() { _ = securityAudit.Close() }()
	claims := platform.NewMatchClaimStore()
	mux := buildPlatformMux(archive, guests, accounts, friends, moderation, challenges, notifications, emailOutbox, securityAudit, claims)

	adminGuest, err := guests.EnsureGuest("guest_mod_admin", "")
	if err != nil {
		t.Fatalf("expected admin guest session, got %v", err)
	}
	reporterGuest, err := guests.EnsureGuest("guest_mod_reporter", "")
	if err != nil {
		t.Fatalf("expected reporter guest session, got %v", err)
	}
	targetGuest, err := guests.EnsureGuest("guest_mod_target", "")
	if err != nil {
		t.Fatalf("expected target guest session, got %v", err)
	}
	adminAccount, err := accounts.ClaimGuest(adminGuest.Guest, "mod_admin")
	if err != nil {
		t.Fatalf("expected admin account claim, got %v", err)
	}
	t.Setenv("PLATFORM_ADMIN_ACCOUNT_IDS", adminAccount.Account.AccountID)
	reporterAccount, err := accounts.ClaimGuest(reporterGuest.Guest, "reporter_mod")
	if err != nil {
		t.Fatalf("expected reporter account claim, got %v", err)
	}
	targetAccount, err := accounts.ClaimGuest(targetGuest.Guest, "target_mod")
	if err != nil {
		t.Fatalf("expected target account claim, got %v", err)
	}

	reportReq := httptest.NewRequest(http.MethodPost, "/api/platform/moderation/reports", strings.NewReader(`{"accountId":"`+reporterAccount.Account.AccountID+`","sessionToken":"`+reporterAccount.SessionToken+`","targetAccountId":"`+targetAccount.Account.AccountID+`","category":"spam","details":"kept sending griefing messages"}`))
	reportRec := httptest.NewRecorder()
	mux.ServeHTTP(reportRec, reportReq)
	if reportRec.Code != http.StatusOK {
		t.Fatalf("expected report request to succeed, got status %d body=%s", reportRec.Code, reportRec.Body.String())
	}

	adminOverviewReq := httptest.NewRequest(http.MethodPost, "/api/platform/moderation/admin/overview", strings.NewReader(`{"accountId":"`+adminAccount.Account.AccountID+`","sessionToken":"`+adminAccount.SessionToken+`","limit":10,"status":"open"}`))
	adminOverviewRec := httptest.NewRecorder()
	mux.ServeHTTP(adminOverviewRec, adminOverviewReq)
	if adminOverviewRec.Code != http.StatusOK {
		t.Fatalf("expected admin moderation overview to succeed, got status %d body=%s", adminOverviewRec.Code, adminOverviewRec.Body.String())
	}

	var adminOverview struct {
		Reports []struct {
			ReportID string `json:"reportId"`
			Status   string `json:"status"`
			Reporter struct {
				Handle string `json:"handle"`
			} `json:"reporter"`
			Target struct {
				Handle string `json:"handle"`
			} `json:"target"`
		} `json:"reports"`
	}
	if err := json.Unmarshal(adminOverviewRec.Body.Bytes(), &adminOverview); err != nil {
		t.Fatalf("expected admin moderation overview to decode, got %v", err)
	}
	if len(adminOverview.Reports) != 1 || adminOverview.Reports[0].Status != platform.PlayerReportStatusOpen || adminOverview.Reports[0].Reporter.Handle != "reporter_mod" || adminOverview.Reports[0].Target.Handle != "target_mod" {
		t.Fatalf("unexpected admin moderation overview %#v", adminOverview)
	}

	resolveReq := httptest.NewRequest(http.MethodPost, "/api/platform/moderation/admin/reports/resolve", strings.NewReader(`{"accountId":"`+adminAccount.Account.AccountID+`","sessionToken":"`+adminAccount.SessionToken+`","reportId":"`+adminOverview.Reports[0].ReportID+`","action":"resolved_actioned","note":"warning issued","limit":10}`))
	resolveRec := httptest.NewRecorder()
	mux.ServeHTTP(resolveRec, resolveReq)
	if resolveRec.Code != http.StatusOK {
		t.Fatalf("expected moderation resolution to succeed, got status %d body=%s", resolveRec.Code, resolveRec.Body.String())
	}

	var resolved struct {
		Reports []struct {
			ReportID       string `json:"reportId"`
			Status         string `json:"status"`
			ResolutionNote string `json:"resolutionNote"`
			ReviewedBy     *struct {
				Handle string `json:"handle"`
			} `json:"reviewedBy"`
		} `json:"reports"`
		RecentActions []struct {
			Action    string `json:"action"`
			Moderator struct {
				Handle string `json:"handle"`
			} `json:"moderator"`
			PreviousStatus string `json:"previousStatus"`
			NextStatus     string `json:"nextStatus"`
		} `json:"recentActions"`
	}
	if err := json.Unmarshal(resolveRec.Body.Bytes(), &resolved); err != nil {
		t.Fatalf("expected moderation resolution response to decode, got %v", err)
	}
	if len(resolved.Reports) != 1 || resolved.Reports[0].Status != platform.PlayerReportStatusResolvedActioned || resolved.Reports[0].ResolutionNote != "warning issued" || resolved.Reports[0].ReviewedBy == nil || resolved.Reports[0].ReviewedBy.Handle != "mod_admin" {
		t.Fatalf("unexpected resolved moderation response %#v", resolved)
	}
	if len(resolved.RecentActions) == 0 || resolved.RecentActions[0].Action != platform.PlayerReportStatusResolvedActioned || resolved.RecentActions[0].Moderator.Handle != "mod_admin" || resolved.RecentActions[0].PreviousStatus != platform.PlayerReportStatusOpen || resolved.RecentActions[0].NextStatus != platform.PlayerReportStatusResolvedActioned {
		t.Fatalf("unexpected moderation action audit %#v", resolved)
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

	buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims).ServeHTTP(rec, req)

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

func TestAccountsListCanFilterByHandleQuery(t *testing.T) {
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
	if _, err := accounts.ClaimGuest(platform.GuestProfile{GuestID: "guest_second"}, "nova_second"); err != nil {
		t.Fatalf("expected second account claim to succeed, got %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/platform/accounts?limit=10&query=aurora", nil)
	rec := httptest.NewRecorder()

	buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected filtered account list to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		Accounts      []platform.AccountProfile `json:"accounts"`
		SelectedQuery string                    `json:"selectedQuery"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected filtered account list to decode, got %v", err)
	}
	if response.SelectedQuery != "aurora" {
		t.Fatalf("expected selected query to round-trip, got %#v", response)
	}
	if len(response.Accounts) != 1 || response.Accounts[0].AccountID != first.Account.AccountID {
		t.Fatalf("unexpected filtered account list response %#v", response)
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

	buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims).ServeHTTP(rec, req)

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

func TestAccountByHandleLookupReturnsLinkedAccount(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/api/platform/accounts/by-handle/Aurora_Lookup", nil)
	rec := httptest.NewRecorder()

	buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected account lookup by handle to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		Account platform.AccountProfile `json:"account"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected account-by-handle response to decode, got %v", err)
	}
	if response.Account.AccountID != session.Account.AccountID || response.Account.Handle != "aurora_lookup" {
		t.Fatalf("unexpected account-by-handle response %#v", response)
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

	buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims).ServeHTTP(rec, req)

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
	if response.Account.DisplayName != guestSession.Guest.DisplayName || response.Account.Rating != 1215 {
		t.Fatalf("expected derived account display/rating=1215, got %#v", response)
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
	if _, _, changed, err := accounts.FinalizeMatch("account_history_match", whiteAccount.Account.AccountID, blackAccount.Account.AccountID, "white", "rated", contracts.MatchModeHiddenCards); err != nil || !changed {
		t.Fatalf("expected account finalization to succeed, got changed=%v err=%v", changed, err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/platform/accounts/"+whiteAccount.Account.AccountID, nil)
	rec := httptest.NewRecorder()

	buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims).ServeHTTP(rec, req)

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
				Queue         string `json:"queue"`
				ModeID        string `json:"modeId"`
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
	expectedSeasonID := time.Now().UTC().Format("2006-01")
	if response.Account.CurrentSeason == nil || response.Account.CurrentSeason.SeasonID != expectedSeasonID || response.Account.CurrentSeason.MatchesPlayed != 1 || response.Account.CurrentSeason.NetDelta != 16 || response.Account.CurrentSeason.RatingEnd != 1216 {
		t.Fatalf("unexpected current season summary %#v", response.Account.CurrentSeason)
	}
	if len(response.Account.SeasonHistory) != 1 || response.Account.SeasonHistory[0].SeasonID != expectedSeasonID || response.Account.SeasonHistory[0].MatchesPlayed != 1 || response.Account.SeasonHistory[0].NetDelta != 16 {
		t.Fatalf("unexpected season history %#v", response.Account.SeasonHistory)
	}
	if response.Account.RatingHistory[0].MatchID != "account_history_match" || response.Account.RatingHistory[0].Queue != "rated" || response.Account.RatingHistory[0].ModeID != string(contracts.MatchModeHiddenCards) || response.Account.RatingHistory[0].Result != "win" || response.Account.RatingHistory[0].Delta != 16 || response.Account.RatingHistory[0].RatingAfter != 1216 || response.Account.RatingHistory[0].MatchesPlayed != 1 {
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

	buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims).ServeHTTP(rec, req)

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
	if _, _, changed, err := accounts.FinalizeMatch("rank_history_match", whiteAccount.Account.AccountID, blackAccount.Account.AccountID, "white", "rated", contracts.MatchModeOpenCards); err != nil || !changed {
		t.Fatalf("expected direct account finalize to succeed, got changed=%v err=%v", changed, err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/platform/accounts?limit=10&sort=rating", nil)
	rec := httptest.NewRecorder()

	buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims).ServeHTTP(rec, req)

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
	expectedSeasonID := time.Now().UTC().Format("2006-01")
	if response.Accounts[0].CurrentSeason == nil || response.Accounts[0].CurrentSeason.SeasonID != expectedSeasonID || response.Accounts[0].CurrentSeason.MatchesPlayed != 1 || response.Accounts[0].CurrentSeason.NetDelta != 16 {
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
	if _, _, changed, err := accounts.FinalizeMatch("season_filter_match", whiteAccount.Account.AccountID, blackAccount.Account.AccountID, "white", "rated", contracts.MatchModeOpenCards); err != nil || !changed {
		t.Fatalf("expected direct account finalize to succeed, got changed=%v err=%v", changed, err)
	}

	expectedSeasonID := time.Now().UTC().Format("2006-01")
	req := httptest.NewRequest(http.MethodGet, "/api/platform/accounts?limit=10&sort=rating&seasonId="+expectedSeasonID, nil)
	rec := httptest.NewRecorder()

	buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims).ServeHTTP(rec, req)

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
		Summary *struct {
			SeasonID    string `json:"seasonId"`
			SeasonLabel string `json:"seasonLabel"`
			PlayerCount int    `json:"playerCount"`
			MatchCount  int    `json:"matchCount"`
			Leader      *struct {
				Handle   string `json:"handle"`
				NetDelta int    `json:"netDelta"`
			} `json:"leader"`
			BiggestClimber *struct {
				Handle   string `json:"handle"`
				NetDelta int    `json:"netDelta"`
			} `json:"biggestClimber"`
		} `json:"summary"`
		SelectedSeasonID string `json:"selectedSeasonId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected season-filtered account list to decode, got %v", err)
	}
	if response.SelectedSeasonID != expectedSeasonID {
		t.Fatalf("expected selected season id to round-trip, got %#v", response)
	}
	if len(response.Accounts) != 2 || response.Accounts[0].SelectedSeason == nil || response.Accounts[0].SelectedSeason.SeasonID != expectedSeasonID {
		t.Fatalf("expected only matching season accounts with selected season, got %#v", response)
	}
	if len(response.Seasons) != 1 || response.Seasons[0].SeasonID != expectedSeasonID {
		t.Fatalf("expected available season metadata, got %#v", response.Seasons)
	}
	if response.Summary == nil || response.Summary.SeasonID != expectedSeasonID || response.Summary.PlayerCount != 2 || response.Summary.MatchCount != 2 {
		t.Fatalf("expected season summary metadata, got %#v", response.Summary)
	}
	if response.Summary.Leader == nil || response.Summary.BiggestClimber == nil || response.Summary.Leader.Handle == "" || response.Summary.BiggestClimber.Handle == "" {
		t.Fatalf("expected season summary spotlights, got %#v", response.Summary)
	}
}

func TestAccountsListCanFilterToRequestedMode(t *testing.T) {
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

	openWhiteGuest, _ := guests.EnsureGuest("guest_mode_open_white", "")
	openBlackGuest, _ := guests.EnsureGuest("guest_mode_open_black", "")
	hiddenWhiteGuest, _ := guests.EnsureGuest("guest_mode_hidden_white", "")
	hiddenBlackGuest, _ := guests.EnsureGuest("guest_mode_hidden_black", "")
	openWhiteAccount, _ := accounts.ClaimGuest(openWhiteGuest.Guest, "mode_open_white")
	openBlackAccount, _ := accounts.ClaimGuest(openBlackGuest.Guest, "mode_open_black")
	hiddenWhiteAccount, _ := accounts.ClaimGuest(hiddenWhiteGuest.Guest, "mode_hidden_white")
	hiddenBlackAccount, _ := accounts.ClaimGuest(hiddenBlackGuest.Guest, "mode_hidden_black")
	if _, _, err := accounts.SyncGuestStats(openWhiteGuest.Guest); err != nil {
		t.Fatalf("expected open white sync to succeed, got %v", err)
	}
	if _, _, err := accounts.SyncGuestStats(openBlackGuest.Guest); err != nil {
		t.Fatalf("expected open black sync to succeed, got %v", err)
	}
	if _, _, err := accounts.SyncGuestStats(hiddenWhiteGuest.Guest); err != nil {
		t.Fatalf("expected hidden white sync to succeed, got %v", err)
	}
	if _, _, err := accounts.SyncGuestStats(hiddenBlackGuest.Guest); err != nil {
		t.Fatalf("expected hidden black sync to succeed, got %v", err)
	}
	if _, _, changed, err := accounts.FinalizeMatch("mode_open_match", openWhiteAccount.Account.AccountID, openBlackAccount.Account.AccountID, "white", "rated", contracts.MatchModeOpenCards); err != nil || !changed {
		t.Fatalf("expected open-card finalize to succeed, got changed=%v err=%v", changed, err)
	}
	if _, _, changed, err := accounts.FinalizeMatch("mode_hidden_match", hiddenWhiteAccount.Account.AccountID, hiddenBlackAccount.Account.AccountID, "black", "rated", contracts.MatchModeHiddenCards); err != nil || !changed {
		t.Fatalf("expected hidden-card finalize to succeed, got changed=%v err=%v", changed, err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/platform/accounts?limit=10&sort=rating&modeId=hidden_cards", nil)
	rec := httptest.NewRecorder()

	buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected mode-filtered account list to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		Accounts []struct {
			Handle        string `json:"handle"`
			MatchesPlayed int    `json:"matchesPlayed"`
		} `json:"accounts"`
		Summary *struct {
			ModeID string `json:"modeId"`
			Leader *struct {
				Handle string `json:"handle"`
			} `json:"leader"`
			HighestPeak *struct {
				Handle     string `json:"handle"`
				PeakRating int    `json:"peakRating"`
			} `json:"highestPeak"`
			MostActive *struct {
				Handle        string `json:"handle"`
				MatchesPlayed int    `json:"matchesPlayed"`
			} `json:"mostActive"`
		} `json:"summary"`
		SelectedModeID string `json:"selectedModeId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected mode-filtered account list to decode, got %v", err)
	}
	if response.SelectedModeID != string(contracts.MatchModeHiddenCards) {
		t.Fatalf("expected selected mode id to round-trip, got %#v", response)
	}
	if len(response.Accounts) != 2 {
		t.Fatalf("expected only hidden-card accounts in filtered list, got %#v", response.Accounts)
	}
	for _, account := range response.Accounts {
		if account.Handle != "mode_hidden_white" && account.Handle != "mode_hidden_black" {
			t.Fatalf("expected hidden-card handles only, got %#v", response.Accounts)
		}
		if account.MatchesPlayed != 1 {
			t.Fatalf("expected filtered mode stats to scope matches played, got %#v", response.Accounts)
		}
	}
	if response.Summary == nil || response.Summary.ModeID != string(contracts.MatchModeHiddenCards) {
		t.Fatalf("expected hidden-card summary metadata, got %#v", response.Summary)
	}
	if response.Summary.Leader == nil || (response.Summary.Leader.Handle != "mode_hidden_black" && response.Summary.Leader.Handle != "mode_hidden_white") {
		t.Fatalf("expected mode leader spotlight, got %#v", response.Summary)
	}
	if response.Summary.HighestPeak == nil || response.Summary.HighestPeak.PeakRating < 1200 {
		t.Fatalf("expected peak spotlight metadata, got %#v", response.Summary)
	}
	if response.Summary.MostActive == nil || response.Summary.MostActive.MatchesPlayed != 1 {
		t.Fatalf("expected activity spotlight metadata, got %#v", response.Summary)
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

	buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected archived match detail to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}

	var response platform.MatchArchiveEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected archived match detail to decode, got %v", err)
	}
	if response.WhiteAccountID != "" || response.WhiteAccountHandle != "aurora_archive" {
		t.Fatalf("expected public archive detail to expose handle but hide account id, got %#v", response)
	}
	if response.Snapshot.Match.WhiteAccountID != "" || response.Snapshot.Match.WhiteGuestID != "" {
		t.Fatalf("expected public snapshot to hide internal seat ids, got %#v", response.Snapshot.Match)
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

	buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims).ServeHTTP(rec, req)

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
	if response.Matches[0].WhiteAccountID != "" || response.Matches[0].WhiteAccountHandle != "aurora_archive" {
		t.Fatalf("expected account filter response to expose handle but hide account id, got %#v", response.Matches[0])
	}
}

func TestArchivedMatchListCanFilterByStatusAndMode(t *testing.T) {
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

	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	if err := archive.Upsert(contracts.MatchSnapshotResponse{
		Match: contracts.MatchState{
			MatchID:      "watch_active_open",
			RulesVersion: "v1-alpha-foundation",
			Queue:        "rated",
			ModeID:       contracts.MatchModeOpenCards,
			WhiteGuestID: "guest_live_white",
			BlackGuestID: "guest_live_black",
			WhiteName:    "Live White",
			BlackName:    "Live Black",
			Status:       "active",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}); err != nil {
		t.Fatalf("expected active archive upsert to succeed, got %v", err)
	}
	if err := archive.Upsert(contracts.MatchSnapshotResponse{
		Match: contracts.MatchState{
			MatchID:      "watch_finished_open",
			RulesVersion: "v1-alpha-foundation",
			Queue:        "rated",
			ModeID:       contracts.MatchModeOpenCards,
			WhiteGuestID: "guest_done_white",
			BlackGuestID: "guest_done_black",
			Status:       "finished",
			Winner:       "white",
			CreatedAt:    now,
			UpdatedAt:    now.Add(time.Minute),
		},
	}); err != nil {
		t.Fatalf("expected finished archive upsert to succeed, got %v", err)
	}
	if err := archive.Upsert(contracts.MatchSnapshotResponse{
		Match: contracts.MatchState{
			MatchID:      "watch_active_hidden",
			RulesVersion: "v1-alpha-foundation",
			Queue:        "casual",
			ModeID:       contracts.MatchModeHiddenCards,
			WhiteGuestID: "guest_hidden_white",
			BlackGuestID: "guest_hidden_black",
			Status:       "active",
			CreatedAt:    now,
			UpdatedAt:    now.Add(2 * time.Minute),
		},
	}); err != nil {
		t.Fatalf("expected hidden active archive upsert to succeed, got %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/platform/matches?status=active&modeId=open_cards", nil)
	rec := httptest.NewRecorder()

	buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected filtered public match feed to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		Matches        []platform.MatchArchiveEntry `json:"matches"`
		SelectedModeID string                       `json:"selectedModeId"`
		SelectedStatus string                       `json:"selectedStatus"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected filtered public match feed to decode, got %v", err)
	}
	if response.SelectedModeID != string(contracts.MatchModeOpenCards) || response.SelectedStatus != "active" {
		t.Fatalf("expected filters to round-trip, got %#v", response)
	}
	if len(response.Matches) != 1 || response.Matches[0].MatchID != "watch_active_open" {
		t.Fatalf("expected only active open-card match in feed, got %#v", response.Matches)
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

func TestMatchClaimsRejectCachedClaimWithoutArchiveEntry(t *testing.T) {
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
	}); err != nil {
		t.Fatalf("expected cached claim write to succeed, got %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/platform/match-claims", strings.NewReader(`{"matchId":"room_cached_claim","guestId":"`+session.Guest.GuestID+`","sessionSecret":"`+session.SessionSecret+`"}`))
	rec := httptest.NewRecorder()

	buildTestPlatformMux(t, archive, guests, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected cached match claim without archive to be rejected, got status %d body=%s", rec.Code, rec.Body.String())
	}
	if _, ok := claims.Get("room_cached_claim", session.Guest.GuestID); ok {
		t.Fatalf("expected stale cached claim to be deleted after validation failure")
	}
}

func TestActiveMatchClaimsRejectFinishedRooms(t *testing.T) {
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

	session, err := guests.EnsureGuest("guest_finished_claim", "")
	if err != nil {
		t.Fatalf("expected guest session creation to succeed, got %v", err)
	}
	if err := archive.Upsert(contracts.MatchSnapshotResponse{
		Match: contracts.MatchState{
			MatchID:           "room_finished_claim",
			Status:            "finished",
			Queue:             "rated",
			ModeID:            contracts.MatchModeOpenCards,
			WhiteGuestID:      session.Guest.GuestID,
			WhitePlayerSecret: "white-seat-secret",
			WhiteName:         "White Guest",
			BlackGuestID:      "guest_other",
			BlackName:         "Other Guest",
		},
	}); err != nil {
		t.Fatalf("expected archived finished match to persist, got %v", err)
	}
	if err := claims.Put(platform.MatchSeatClaim{
		MatchID:      "room_finished_claim",
		GuestID:      session.Guest.GuestID,
		SeatColor:    "white",
		PlayerID:     session.Guest.GuestID,
		PlayerSecret: "cached_room_secret",
		Queue:        "rated",
	}); err != nil {
		t.Fatalf("expected cached claim write to succeed, got %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/platform/match-claims/active", strings.NewReader(`{"guestId":"`+session.Guest.GuestID+`","sessionSecret":"`+session.SessionSecret+`"}`))
	rec := httptest.NewRecorder()

	buildTestPlatformMux(t, archive, guests, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected finished-room active claim to be rejected, got status %d body=%s", rec.Code, rec.Body.String())
	}
	if _, ok := claims.Get("room_finished_claim", session.Guest.GuestID); ok {
		t.Fatalf("expected finished-room claim to be deleted after validation failure")
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
	if err := archive.Upsert(contracts.MatchSnapshotResponse{
		Match: contracts.MatchState{
			MatchID:           "room_resolve",
			Status:            "active",
			Queue:             "rated",
			ModeID:            contracts.MatchModeOpenCards,
			WhiteGuestID:      session.Guest.GuestID,
			WhitePlayerSecret: "resolve_secret",
			WhiteName:         "Resolve Guest",
			BlackGuestID:      "guest_other",
			BlackName:         "Other Guest",
		},
	}); err != nil {
		t.Fatalf("expected archived match to persist, got %v", err)
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
	if _, err := guests.EnsureGuest("guest_black", ""); err != nil {
		t.Fatalf("expected black guest creation to succeed, got %v", err)
	}

	body := fmt.Sprintf(`{"matchId":"casual_room","guestId":"%s","sessionSecret":"%s"}`, white.Guest.GuestID, white.SessionSecret)
	req := httptest.NewRequest(http.MethodPost, "/api/platform/guest-results", strings.NewReader(body))
	authorizeInternalPlatformRequest(t, req)
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
	if refreshedWhite.Rating != 1200 || refreshedBlack.Rating != 1200 {
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

	whiteSession, err := guests.EnsureGuest("guest_white", "")
	if err != nil {
		t.Fatalf("expected white guest creation to succeed, got %v", err)
	}
	if _, err := guests.EnsureGuest("guest_black", ""); err != nil {
		t.Fatalf("expected black guest creation to succeed, got %v", err)
	}

	body := fmt.Sprintf(`{"matchId":"rated_room","guestId":"%s","sessionSecret":"%s"}`, whiteSession.Guest.GuestID, whiteSession.SessionSecret)
	req := httptest.NewRequest(http.MethodPost, "/api/platform/guest-results", strings.NewReader(body))
	authorizeInternalPlatformRequest(t, req)
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

func TestGuestResultsRejectUnauthenticated(t *testing.T) {
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

	// A guest who is not a participant should be rejected
	intruder, err := guests.EnsureGuest("guest_intruder", "")
	if err != nil {
		t.Fatalf("expected intruder guest creation to succeed, got %v", err)
	}

	body := fmt.Sprintf(`{"matchId":"rated_room","guestId":"%s","sessionSecret":"%s"}`, intruder.Guest.GuestID, intruder.SessionSecret)
	req := httptest.NewRequest(http.MethodPost, "/api/platform/guest-results", strings.NewReader(body))
	authorizeInternalPlatformRequest(t, req)
	rec := httptest.NewRecorder()

	buildTestPlatformMux(t, archive, guests, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected non-participant guest to be rejected with 403, got status %d body=%s", rec.Code, rec.Body.String())
	}
	refreshedWhite, _ := guests.GetGuest("guest_white")
	refreshedBlack, _ := guests.GetGuest("guest_black")
	if refreshedWhite.Rating != 1200 || refreshedBlack.Rating != 1200 {
		t.Fatalf("expected rejected guest result to preserve ratings, got %#v %#v", refreshedWhite, refreshedBlack)
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

	white, err := guests.EnsureGuest("guest_white", "")
	if err != nil {
		t.Fatalf("expected white guest creation to succeed, got %v", err)
	}
	if _, err := guests.EnsureGuest("guest_black", ""); err != nil {
		t.Fatalf("expected black guest creation to succeed, got %v", err)
	}

	body := fmt.Sprintf(`{"matchId":"rated_room_open","guestId":"%s","sessionSecret":"%s"}`, white.Guest.GuestID, white.SessionSecret)
	req := httptest.NewRequest(http.MethodPost, "/api/platform/guest-results", strings.NewReader(body))
	authorizeInternalPlatformRequest(t, req)
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

	body := fmt.Sprintf(`{"matchId":"rated_account_room","accountId":"%s","sessionToken":"%s"}`, whiteAccountSession.Account.AccountID, whiteAccountSession.SessionToken)
	req := httptest.NewRequest(http.MethodPost, "/api/platform/account-results", strings.NewReader(body))
	authorizeInternalPlatformRequest(t, req)
	rec := httptest.NewRecorder()

	buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims).ServeHTTP(rec, req)

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

func TestAccountResultsRejectUnauthorized(t *testing.T) {
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
	if _, err := accounts.ClaimGuest(blackSession.Guest, "black_owner"); err != nil {
		t.Fatalf("expected black account claim to succeed, got %v", err)
	}

	now := time.Date(2026, 5, 7, 10, 30, 0, 0, time.UTC)
	if err := archive.Upsert(contracts.MatchSnapshotResponse{
		Match: contracts.MatchState{
			MatchID:        "rated_account_room",
			RulesVersion:   "v1-alpha-foundation",
			Queue:          "rated",
			WhiteGuestID:   whiteSession.Guest.GuestID,
			BlackGuestID:   blackSession.Guest.GuestID,
			WhiteAccountID: whiteAccountSession.Account.AccountID,
			Status:         "finished",
			Winner:         "white",
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	}); err != nil {
		t.Fatalf("expected archive upsert to succeed, got %v", err)
	}

	// An intruder account that owns no seat in the match should be rejected
	intruderGuest, err := guests.EnsureGuest("guest_intruder", "")
	if err != nil {
		t.Fatalf("expected intruder guest creation to succeed, got %v", err)
	}
	intruderSession, err := accounts.ClaimGuest(intruderGuest.Guest, "intruder_owner")
	if err != nil {
		t.Fatalf("expected intruder account claim to succeed, got %v", err)
	}

	body := fmt.Sprintf(`{"matchId":"rated_account_room","accountId":"%s","sessionToken":"%s"}`, intruderSession.Account.AccountID, intruderSession.SessionToken)
	req := httptest.NewRequest(http.MethodPost, "/api/platform/account-results", strings.NewReader(body))
	authorizeInternalPlatformRequest(t, req)
	rec := httptest.NewRecorder()

	buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected unauthorized account result to be rejected with 403, got status %d body=%s", rec.Code, rec.Body.String())
	}
	refreshedWhite, _ := guests.GetGuest("guest_white")
	refreshedBlack, _ := guests.GetGuest("guest_black")
	if refreshedWhite.Rating != 1200 || refreshedBlack.Rating != 1200 {
		t.Fatalf("expected rejected account result to preserve ratings, got %#v %#v", refreshedWhite, refreshedBlack)
	}
}

func TestInternalFinalizeRatedMatchRequiresServiceToken(t *testing.T) {
	t.Setenv("PLATFORM_INTERNAL_SERVICE_TOKEN", testInternalServiceToken)
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

	req := httptest.NewRequest(http.MethodPost, "/api/platform/internal/finalize-rated-match", strings.NewReader(`{"matchId":"rated_room"}`))
	rec := httptest.NewRecorder()

	buildTestPlatformMux(t, archive, guests, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected internal finalizer to require service token, got status %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestInternalFinalizeRatedMatchFinalizesArchivedRatedMatch(t *testing.T) {
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
	whiteAccountSession, err := accounts.ClaimGuest(whiteSession.Guest, "white_internal")
	if err != nil {
		t.Fatalf("expected white account claim to succeed, got %v", err)
	}
	blackAccountSession, err := accounts.ClaimGuest(blackSession.Guest, "black_internal")
	if err != nil {
		t.Fatalf("expected black account claim to succeed, got %v", err)
	}

	now := time.Date(2026, 5, 7, 11, 0, 0, 0, time.UTC)
	if err := archive.Upsert(contracts.MatchSnapshotResponse{
		Match: contracts.MatchState{
			MatchID:        "rated_internal_room",
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

	req := httptest.NewRequest(http.MethodPost, "/api/platform/internal/finalize-rated-match", strings.NewReader(`{"matchId":"rated_internal_room"}`))
	authorizeInternalPlatformRequest(t, req)
	rec := httptest.NewRecorder()

	buildTestPlatformMuxWithAccounts(t, archive, guests, accounts, claims).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected internal finalizer to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		Changed      bool                          `json:"changed"`
		White        platform.GuestProfile         `json:"white"`
		Black        platform.GuestProfile         `json:"black"`
		WhiteAccount platform.PublicAccountProfile `json:"whiteAccount"`
		BlackAccount platform.PublicAccountProfile `json:"blackAccount"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected internal finalizer response to decode, got %v", err)
	}
	if !response.Changed || response.White.Rating != 1216 || response.Black.Rating != 1184 {
		t.Fatalf("expected internal finalizer to update guest ratings, got %#v", response)
	}
	if response.WhiteAccount.AccountID != whiteAccountSession.Account.AccountID || response.WhiteAccount.Rating != 1216 {
		t.Fatalf("expected internal finalizer to update white account, got %#v", response.WhiteAccount)
	}
	if response.BlackAccount.AccountID != blackAccountSession.Account.AccountID || response.BlackAccount.Rating != 1184 {
		t.Fatalf("expected internal finalizer to update black account, got %#v", response.BlackAccount)
	}
}
