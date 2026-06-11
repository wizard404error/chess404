package platform

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chess404/realtime/internal/contracts"
)

var ErrInvalidAccountHandle = errors.New("invalid account handle")
var ErrAccountHandleTaken = errors.New("account handle already taken")
var ErrUnauthorizedAccountSession = errors.New("unauthorized account session")

const defaultAccountSessionTTL = 14 * 24 * time.Hour
const maxAccountRatingHistoryEntries = 100

var accountHandlePattern = regexp.MustCompile(`^[a-z0-9_-]{3,24}$`)

type AccountProfile struct {
	AccountID      string                      `json:"accountId"`
	Handle         string                      `json:"handle"`
	PrimaryGuestID string                      `json:"primaryGuestId"`
	LinkedGuestIDs []string                    `json:"linkedGuestIds"`
	Rating         int                         `json:"rating"`
	MatchesPlayed  int                         `json:"matchesPlayed"`
	Wins           int                         `json:"wins"`
	Losses         int                         `json:"losses"`
	Draws          int                         `json:"draws"`
	RatingHistory  []AccountRatingHistoryEntry `json:"ratingHistory,omitempty"`
	CreatedAt      time.Time                   `json:"createdAt"`
	LastSeenAt     time.Time                   `json:"lastSeenAt"`
	LastActiveAt   time.Time                   `json:"lastActiveAt,omitempty"`
}

type AccountRatingHistoryEntry struct {
	MatchID           string                `json:"matchId"`
	OpponentAccountID string                `json:"opponentAccountId,omitempty"`
	Queue             string                `json:"queue,omitempty"`
	ModeID            contracts.MatchModeID `json:"modeId,omitempty"`
	Result            string                `json:"result"`
	Winner            string                `json:"winner"`
	Delta             int                   `json:"delta"`
	RatingBefore      int                   `json:"ratingBefore"`
	RatingAfter       int                   `json:"ratingAfter"`
	MatchesPlayed     int                   `json:"matchesPlayed"`
	At                time.Time             `json:"at"`
}

type AccountSession struct {
	Account      AccountProfile `json:"account"`
	SessionToken string         `json:"sessionToken"`
	ExpiresAt    time.Time      `json:"expiresAt"`
}

type AccountPrivateState struct {
	SessionToken       string                           `json:"sessionToken,omitempty"`
	ExpiresAt          time.Time                        `json:"expiresAt,omitempty"`
	Sessions           []AccountSessionRecord           `json:"sessions,omitempty"`
	Email              string                           `json:"email,omitempty"`
	PasswordHash       string                           `json:"passwordHash,omitempty"`
	EmailVerifiedAt    time.Time                        `json:"emailVerifiedAt,omitempty"`
	EmailVerifications []AccountEmailVerificationRecord `json:"emailVerifications,omitempty"`
	PasswordResets     []AccountPasswordResetRecord     `json:"passwordResets,omitempty"`
}

type AccountStoreStats struct {
	AccountCount       int `json:"accountCount"`
	LinkedGuestCount   int `json:"linkedGuestCount"`
	ActiveSessionCount int `json:"activeSessionCount"`
}

type AccountStore struct {
	mu         sync.Mutex
	path       string
	accounts   map[string]AccountProfile
	guestLinks map[string]string
	private    map[string]AccountPrivateState
	finalized  map[string]string
}

type accountStoreFile struct {
	Accounts   map[string]AccountProfile      `json:"accounts"`
	GuestLinks map[string]string              `json:"guestLinks"`
	Private    map[string]AccountPrivateState `json:"private,omitempty"`
	Finalized  map[string]string              `json:"finalized,omitempty"`
}

func NewAccountStore(path string) (*AccountStore, error) {
	store := &AccountStore{
		path:       path,
		accounts:   make(map[string]AccountProfile),
		guestLinks: make(map[string]string),
		private:    make(map[string]AccountPrivateState),
		finalized:  make(map[string]string),
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *AccountStore) Backend() string {
	return "file"
}

func (s *AccountStore) Close() error {
	return nil
}

func (s *AccountStore) Stats() AccountStoreStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	activeSessions := 0
	for _, privateState := range s.private {
		activeSessions += countActiveAccountSessions(privateState, now)
	}
	return AccountStoreStats{
		AccountCount:       len(s.accounts),
		LinkedGuestCount:   len(s.guestLinks),
		ActiveSessionCount: activeSessions,
	}
}

func (s *AccountStore) ClaimGuest(guest GuestProfile, handle string) (AccountSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	if accountID, ok := s.guestLinks[guest.GuestID]; ok {
		account, ok := s.accounts[accountID]
		if !ok {
			delete(s.guestLinks, guest.GuestID)
		} else {
			normalizedHandle, err := normalizeAccountHandle(handle)
			if err == nil && normalizedHandle != "" && normalizedHandle != account.Handle {
				return AccountSession{}, ErrAccountHandleTaken
			}
			touchAccountPresence(&account, now)
			s.accounts[accountID] = account
			privateState, record := issueAccountPrivateSession(s.private[accountID], now)
			s.private[accountID] = privateState
			if err := s.persistLocked(); err != nil {
				return AccountSession{}, err
			}
			return buildAccountSessionFromRecord(account, record), nil
		}
	}

	normalizedHandle, err := normalizeAccountHandle(handle)
	if err != nil {
		return AccountSession{}, err
	}
	if normalizedHandle == "" {
		return AccountSession{}, ErrInvalidAccountHandle
	}
	for _, account := range s.accounts {
		if account.Handle == normalizedHandle {
			return AccountSession{}, ErrAccountHandleTaken
		}
	}

	accountID := "acct_" + randomToken(8)
	account := AccountProfile{
		AccountID:      accountID,
		Handle:         normalizedHandle,
		PrimaryGuestID: guest.GuestID,
		LinkedGuestIDs: []string{guest.GuestID},
		CreatedAt:      now,
		LastSeenAt:     now,
		LastActiveAt:   now,
	}
	privateState, record := issueAccountPrivateSession(AccountPrivateState{}, now)
	s.accounts[accountID] = account
	s.guestLinks[guest.GuestID] = accountID
	s.private[accountID] = privateState
	if err := s.persistLocked(); err != nil {
		return AccountSession{}, err
	}
	return buildAccountSessionFromRecord(account, record), nil
}

func (s *AccountStore) RegisterGuestAccount(guest GuestProfile, handle, email, password string) (AccountSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedGuestID := strings.TrimSpace(guest.GuestID)
	if resolvedGuestID == "" {
		return AccountSession{}, os.ErrInvalid
	}

	normalizedHandle, err := normalizeAccountHandle(handle)
	if err != nil || normalizedHandle == "" {
		if err == nil {
			err = ErrInvalidAccountHandle
		}
		return AccountSession{}, err
	}
	normalizedEmail, err := normalizeAccountEmail(email)
	if err != nil {
		return AccountSession{}, err
	}
	passwordHash, err := hashAccountPassword(password)
	if err != nil {
		return AccountSession{}, err
	}

	now := time.Now().UTC()
	if accountID, ok := s.guestLinks[resolvedGuestID]; ok {
		account, ok := s.accounts[accountID]
		if !ok {
			delete(s.guestLinks, resolvedGuestID)
		} else {
			if normalizedHandle != account.Handle {
				return AccountSession{}, ErrAccountHandleTaken
			}
			if existingAccountID, exists := s.lookupAccountIDByEmailLocked(normalizedEmail); exists && existingAccountID != accountID {
				return AccountSession{}, ErrAccountEmailTaken
			}
			touchAccountPresence(&account, now)
			privateState := s.private[accountID]
			emailChanged := !strings.EqualFold(strings.TrimSpace(privateState.Email), normalizedEmail)
			privateState.Email = normalizedEmail
			privateState.PasswordHash = passwordHash
			if emailChanged {
				privateState.EmailVerifiedAt = time.Time{}
				privateState.EmailVerifications = nil
				privateState.PasswordResets = nil
			}
			privateState, record := issueAccountPrivateSession(privateState, now)
			s.accounts[accountID] = account
			s.private[accountID] = privateState
			if err := s.persistLocked(); err != nil {
				return AccountSession{}, err
			}
			return buildAccountSessionFromRecord(account, record), nil
		}
	}

	for _, account := range s.accounts {
		if account.Handle == normalizedHandle {
			return AccountSession{}, ErrAccountHandleTaken
		}
	}
	if existingAccountID, exists := s.lookupAccountIDByEmailLocked(normalizedEmail); exists && existingAccountID != "" {
		return AccountSession{}, ErrAccountEmailTaken
	}

	accountID := "acct_" + randomToken(8)
	account := AccountProfile{
		AccountID:      accountID,
		Handle:         normalizedHandle,
		PrimaryGuestID: resolvedGuestID,
		LinkedGuestIDs: []string{resolvedGuestID},
		CreatedAt:      now,
		LastSeenAt:     now,
		LastActiveAt:   now,
	}
	privateState, record := issueAccountPrivateSession(AccountPrivateState{
		Email:        normalizedEmail,
		PasswordHash: passwordHash,
	}, now)
	s.accounts[accountID] = account
	s.guestLinks[resolvedGuestID] = accountID
	s.private[accountID] = privateState
	if err := s.persistLocked(); err != nil {
		return AccountSession{}, err
	}
	return buildAccountSessionFromRecord(account, record), nil
}

func (s *AccountStore) ResumeAccount(accountID, sessionToken string) (AccountSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return AccountSession{}, os.ErrInvalid
	}

	account, ok := s.accounts[accountID]
	if !ok {
		return AccountSession{}, os.ErrNotExist
	}
	privateState := s.private[accountID]
	privateState, record, ok := renewAccountPrivateSession(privateState, sessionToken, time.Now().UTC())
	if !ok {
		return AccountSession{}, ErrUnauthorizedAccountSession
	}

	now := time.Now().UTC()
	touchAccountPresence(&account, now)
	s.accounts[accountID] = account
	s.private[accountID] = privateState
	if err := s.persistLocked(); err != nil {
		return AccountSession{}, err
	}
	return buildAccountSessionFromRecord(account, record), nil
}

func (s *AccountStore) TouchPresence(accountID, sessionToken string) (AccountSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return AccountSession{}, os.ErrInvalid
	}

	account, ok := s.accounts[resolvedAccountID]
	if !ok {
		return AccountSession{}, os.ErrNotExist
	}
	now := time.Now().UTC()
	privateState, record, ok := renewAccountPrivateSession(s.private[resolvedAccountID], sessionToken, now)
	if !ok {
		return AccountSession{}, ErrUnauthorizedAccountSession
	}

	touchAccountPresence(&account, now)
	s.accounts[resolvedAccountID] = account
	s.private[resolvedAccountID] = privateState
	if err := s.persistLocked(); err != nil {
		return AccountSession{}, err
	}
	return buildAccountSessionFromRecord(account, record), nil
}

func (s *AccountStore) GetAccountAuthOverview(accountID, sessionToken string) (AccountAuthOverview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return AccountAuthOverview{}, os.ErrInvalid
	}
	account, ok := s.accounts[resolvedAccountID]
	if !ok {
		return AccountAuthOverview{}, os.ErrNotExist
	}
	privateState, record, ok := renewAccountPrivateSession(s.private[resolvedAccountID], sessionToken, time.Now().UTC())
	if !ok {
		return AccountAuthOverview{}, ErrUnauthorizedAccountSession
	}
	s.private[resolvedAccountID] = privateState
	s.accounts[resolvedAccountID] = account
	if err := s.persistLocked(); err != nil {
		return AccountAuthOverview{}, err
	}
	_ = record
	return buildAccountAuthOverview(account, privateState, time.Now().UTC()), nil
}

func (s *AccountStore) StartEmailVerification(accountID, sessionToken string) (AccountEmailVerificationChallenge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return AccountEmailVerificationChallenge{}, os.ErrInvalid
	}
	account, ok := s.accounts[resolvedAccountID]
	if !ok {
		return AccountEmailVerificationChallenge{}, os.ErrNotExist
	}
	now := time.Now().UTC()
	privateState, _, ok := renewAccountPrivateSession(s.private[resolvedAccountID], sessionToken, now)
	if !ok {
		return AccountEmailVerificationChallenge{}, ErrUnauthorizedAccountSession
	}
	if strings.TrimSpace(privateState.Email) == "" || strings.TrimSpace(privateState.PasswordHash) == "" {
		return AccountEmailVerificationChallenge{}, ErrAccountLoginUnavailable
	}
	if !privateState.EmailVerifiedAt.IsZero() {
		return AccountEmailVerificationChallenge{}, ErrAccountEmailAlreadyVerified
	}
	privateState, record := issueAccountEmailVerification(privateState, privateState.Email, now)
	s.private[resolvedAccountID] = privateState
	s.accounts[resolvedAccountID] = account
	if err := s.persistLocked(); err != nil {
		return AccountEmailVerificationChallenge{}, err
	}
	return AccountEmailVerificationChallenge{
		AccountID: resolvedAccountID,
		Email:     record.Email,
		Token:     record.Token,
		ExpiresAt: record.ExpiresAt,
		CreatedAt: record.CreatedAt,
	}, nil
}

func (s *AccountStore) VerifyEmail(accountID, token string) (AccountAuthOverview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return AccountAuthOverview{}, os.ErrInvalid
	}
	account, ok := s.accounts[resolvedAccountID]
	if !ok {
		return AccountAuthOverview{}, os.ErrNotExist
	}
	now := time.Now().UTC()
	privateState, record, ok := consumeAccountEmailVerification(s.private[resolvedAccountID], token, now)
	if !ok {
		return AccountAuthOverview{}, ErrUnauthorizedAccountEmailVerification
	}
	privateState.Email = record.Email
	privateState.PasswordResets = nil
	s.private[resolvedAccountID] = privateState
	s.accounts[resolvedAccountID] = account
	if err := s.persistLocked(); err != nil {
		return AccountAuthOverview{}, err
	}
	return buildAccountAuthOverview(account, privateState, now), nil
}

func (s *AccountStore) ListAccountSessions(accountID, sessionToken string) (AccountSessionOverview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return AccountSessionOverview{}, os.ErrInvalid
	}
	account, ok := s.accounts[resolvedAccountID]
	if !ok {
		return AccountSessionOverview{}, os.ErrNotExist
	}
	if !accountSessionTokenValid(s.private[resolvedAccountID], sessionToken, time.Now().UTC()) {
		return AccountSessionOverview{}, ErrUnauthorizedAccountSession
	}
	return buildAccountSessionOverview(account, activeAccountSessionRecords(s.private[resolvedAccountID], time.Now().UTC())), nil
}

func (s *AccountStore) RevokeAccountSession(accountID, sessionToken, revokeToken string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return os.ErrInvalid
	}
	if _, ok := s.accounts[resolvedAccountID]; !ok {
		return os.ErrNotExist
	}
	if !accountSessionTokenValid(s.private[resolvedAccountID], sessionToken, time.Now().UTC()) {
		return ErrUnauthorizedAccountSession
	}
	privateState, removed := removeAccountPrivateSession(s.private[resolvedAccountID], revokeToken)
	if !removed {
		return ErrUnauthorizedAccountSession
	}
	s.private[resolvedAccountID] = privateState
	return s.persistLocked()
}

func (s *AccountStore) RevokeOtherAccountSessions(accountID, sessionToken string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return os.ErrInvalid
	}
	if _, ok := s.accounts[resolvedAccountID]; !ok {
		return os.ErrNotExist
	}
	if !accountSessionTokenValid(s.private[resolvedAccountID], sessionToken, time.Now().UTC()) {
		return ErrUnauthorizedAccountSession
	}
	privateState, found := retainOnlyAccountPrivateSession(s.private[resolvedAccountID], sessionToken)
	if !found {
		return ErrUnauthorizedAccountSession
	}
	s.private[resolvedAccountID] = privateState
	return s.persistLocked()
}

func (s *AccountStore) EnablePasswordLogin(accountID, sessionToken, email, password string) (AccountSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return AccountSession{}, os.ErrInvalid
	}
	account, ok := s.accounts[resolvedAccountID]
	if !ok {
		return AccountSession{}, os.ErrNotExist
	}
	privateState := s.private[resolvedAccountID]
	privateState, record, ok := renewAccountPrivateSession(privateState, sessionToken, time.Now().UTC())
	if !ok {
		return AccountSession{}, ErrUnauthorizedAccountSession
	}

	normalizedEmail, err := normalizeAccountEmail(email)
	if err != nil {
		return AccountSession{}, err
	}
	passwordHash, err := hashAccountPassword(password)
	if err != nil {
		return AccountSession{}, err
	}
	if existingAccountID, exists := s.lookupAccountIDByEmailLocked(normalizedEmail); exists && existingAccountID != resolvedAccountID {
		return AccountSession{}, ErrAccountEmailTaken
	}

	now := time.Now().UTC()
	touchAccountPresence(&account, now)
	emailChanged := !strings.EqualFold(strings.TrimSpace(privateState.Email), normalizedEmail)
	privateState.Email = normalizedEmail
	privateState.PasswordHash = passwordHash
	if emailChanged {
		privateState.EmailVerifiedAt = time.Time{}
		privateState.EmailVerifications = nil
		privateState.PasswordResets = nil
	}
	s.accounts[resolvedAccountID] = account
	s.private[resolvedAccountID] = privateState
	if err := s.persistLocked(); err != nil {
		return AccountSession{}, err
	}
	return buildAccountSessionFromRecord(account, record), nil
}

func (s *AccountStore) LoginWithPassword(identifier, password string) (AccountSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	accountID, privateState, ok := s.lookupAccountCredentialsLocked(identifier)
	if !ok || !verifyAccountPassword(password, privateState.PasswordHash) {
		return AccountSession{}, ErrUnauthorizedAccountCredentials
	}

	account, ok := s.accounts[accountID]
	if !ok {
		return AccountSession{}, os.ErrNotExist
	}

	now := time.Now().UTC()
	touchAccountPresence(&account, now)
	privateState, record := issueAccountPrivateSession(privateState, now)
	s.accounts[accountID] = account
	s.private[accountID] = privateState
	if err := s.persistLocked(); err != nil {
		return AccountSession{}, err
	}
	return buildAccountSessionFromRecord(account, record), nil
}

func (s *AccountStore) StartPasswordReset(identifier string) (AccountPasswordResetChallenge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	accountID, privateState, ok := s.lookupAccountCredentialsLocked(identifier)
	if !ok {
		return AccountPasswordResetChallenge{Requested: true}, nil
	}
	if strings.TrimSpace(privateState.Email) == "" || strings.TrimSpace(privateState.PasswordHash) == "" || privateState.EmailVerifiedAt.IsZero() {
		return AccountPasswordResetChallenge{Requested: true}, nil
	}
	now := time.Now().UTC()
	privateState, record := issueAccountPasswordReset(privateState, now)
	s.private[accountID] = privateState
	if err := s.persistLocked(); err != nil {
		return AccountPasswordResetChallenge{}, err
	}
	return AccountPasswordResetChallenge{
		Requested: true,
		AccountID: accountID,
		Email:     privateState.Email,
		Token:     record.Token,
		ExpiresAt: record.ExpiresAt,
		CreatedAt: record.CreatedAt,
	}, nil
}

func (s *AccountStore) ResetPassword(accountID, token, password string) (AccountSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return AccountSession{}, os.ErrInvalid
	}
	account, ok := s.accounts[resolvedAccountID]
	if !ok {
		return AccountSession{}, os.ErrNotExist
	}
	passwordHash, err := hashAccountPassword(password)
	if err != nil {
		return AccountSession{}, err
	}
	now := time.Now().UTC()
	privateState := s.private[resolvedAccountID]
	if privateState.EmailVerifiedAt.IsZero() {
		return AccountSession{}, ErrAccountEmailNotVerified
	}
	privateState, _, ok = consumeAccountPasswordReset(privateState, token, now)
	if !ok {
		return AccountSession{}, ErrUnauthorizedAccountPasswordReset
	}
	privateState.PasswordHash = passwordHash
	privateState.PasswordResets = nil
	privateState = clearAccountPrivateSession(privateState)
	touchAccountPresence(&account, now)
	privateState, record := issueAccountPrivateSession(privateState, now)
	s.private[resolvedAccountID] = privateState
	s.accounts[resolvedAccountID] = account
	if err := s.persistLocked(); err != nil {
		return AccountSession{}, err
	}
	return buildAccountSessionFromRecord(account, record), nil
}

func (s *AccountStore) LogoutAccount(accountID, sessionToken string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return os.ErrInvalid
	}
	if _, ok := s.accounts[resolvedAccountID]; !ok {
		return os.ErrNotExist
	}
	privateState := s.private[resolvedAccountID]
	if !accountSessionTokenValid(privateState, sessionToken, time.Now().UTC()) {
		return ErrUnauthorizedAccountSession
	}
	privateState, removed := removeAccountPrivateSession(privateState, sessionToken)
	if !removed {
		return ErrUnauthorizedAccountSession
	}
	s.private[resolvedAccountID] = privateState
	return s.persistLocked()
}

func (s *AccountStore) SyncGuestStats(guest GuestProfile) (AccountProfile, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	accountID, ok := s.guestLinks[strings.TrimSpace(guest.GuestID)]
	if !ok {
		return AccountProfile{}, false, nil
	}
	account, ok := s.accounts[accountID]
	if !ok {
		return AccountProfile{}, false, nil
	}
	if accountHasDirectStats(account) {
		return account, true, nil
	}
	seeded := seedAccountStatsFromGuestIfNeeded(account, guest)
	s.accounts[accountID] = seeded
	if err := s.persistLocked(); err != nil {
		return AccountProfile{}, false, err
	}
	return seeded, true, nil
}

func (s *AccountStore) FinalizeMatch(matchID, whiteAccountID, blackAccountID, winner, queue string, modeID contracts.MatchModeID) (AccountProfile, AccountProfile, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(matchID) == "" {
		return AccountProfile{}, AccountProfile{}, false, os.ErrInvalid
	}

	white, ok := s.accounts[strings.TrimSpace(whiteAccountID)]
	if !ok {
		return AccountProfile{}, AccountProfile{}, false, os.ErrNotExist
	}
	black, ok := s.accounts[strings.TrimSpace(blackAccountID)]
	if !ok {
		return AccountProfile{}, AccountProfile{}, false, os.ErrNotExist
	}
	if _, finalized := s.finalized[matchID]; finalized {
		return white, black, false, nil
	}
	if white.Rating <= 0 {
		white.Rating = 1200
	}
	if black.Rating <= 0 {
		black.Rating = 1200
	}
	whiteBefore := white.Rating
	blackBefore := black.Rating

	now := time.Now().UTC()
	if err := applyAccountMatchResult(&white, &black, winner); err != nil {
		return AccountProfile{}, AccountProfile{}, false, err
	}
	white.MatchesPlayed++
	black.MatchesPlayed++
	touchAccountPresence(&white, now)
	touchAccountPresence(&black, now)
	modeID = contracts.NormalizeMatchModeID(string(modeID))
	white.RatingHistory = appendAccountRatingHistory(white.RatingHistory, buildAccountRatingHistoryEntry(matchID, black.AccountID, winner, queue, modeID, whiteBefore, white.Rating, white.MatchesPlayed, "white", now))
	black.RatingHistory = appendAccountRatingHistory(black.RatingHistory, buildAccountRatingHistoryEntry(matchID, white.AccountID, winner, queue, modeID, blackBefore, black.Rating, black.MatchesPlayed, "black", now))

	s.accounts[white.AccountID] = white
	s.accounts[black.AccountID] = black
	s.finalized[matchID] = winner
	return white, black, true, s.persistLocked()
}

func (s *AccountStore) GetAccount(accountID string) (AccountProfile, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	account, ok := s.accounts[strings.TrimSpace(accountID)]
	return account, ok
}

func (s *AccountStore) GetAccountByGuest(guestID string) (AccountProfile, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	accountID, ok := s.guestLinks[strings.TrimSpace(guestID)]
	if !ok {
		return AccountProfile{}, false
	}
	account, ok := s.accounts[accountID]
	return account, ok
}

func (s *AccountStore) ListAccounts(limit int) []AccountProfile {
	s.mu.Lock()
	defer s.mu.Unlock()

	items := make([]AccountProfile, 0, len(s.accounts))
	for _, account := range s.accounts {
		items = append(items, account)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].LastSeenAt.Equal(items[j].LastSeenAt) {
			return items[i].CreatedAt.After(items[j].CreatedAt)
		}
		return items[i].LastSeenAt.After(items[j].LastSeenAt)
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func (s *AccountStore) load() error {
	if s.path == "" {
		return nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var wrapped accountStoreFile
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return err
	}
	if wrapped.Accounts != nil {
		s.accounts = wrapped.Accounts
	}
	if wrapped.GuestLinks != nil {
		s.guestLinks = wrapped.GuestLinks
	}
	if wrapped.Private != nil {
		s.private = wrapped.Private
	}
	if wrapped.Finalized != nil {
		s.finalized = wrapped.Finalized
	}
	return nil
}

func (s *AccountStore) persistLocked() error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(accountStoreFile{
		Accounts:   s.accounts,
		GuestLinks: s.guestLinks,
		Private:    s.private,
		Finalized:  s.finalized,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

func normalizeAccountHandle(handle string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(handle))
	if normalized == "" {
		return "", nil
	}
	if !accountHandlePattern.MatchString(normalized) {
		return "", ErrInvalidAccountHandle
	}
	return normalized, nil
}

func defaultAccountRating(guest GuestProfile) int {
	if guest.Rating > 0 {
		return guest.Rating
	}
	return 1200
}

func accountHasDirectStats(account AccountProfile) bool {
	return account.Rating > 0 || account.MatchesPlayed > 0 || account.Wins > 0 || account.Losses > 0 || account.Draws > 0
}

func seedAccountStatsFromGuestIfNeeded(account AccountProfile, guest GuestProfile) AccountProfile {
	if accountHasDirectStats(account) {
		return account
	}
	account.Rating = defaultAccountRating(guest)
	account.MatchesPlayed = guest.MatchesPlayed
	account.Wins = guest.Wins
	account.Losses = guest.Losses
	account.Draws = guest.Draws
	return account
}

func (s *AccountStore) lookupAccountIDByEmailLocked(email string) (string, bool) {
	for accountID, privateState := range s.private {
		if strings.EqualFold(strings.TrimSpace(privateState.Email), strings.TrimSpace(email)) {
			return accountID, true
		}
	}
	return "", false
}

func (s *AccountStore) lookupAccountCredentialsLocked(identifier string) (string, AccountPrivateState, bool) {
	resolved := strings.ToLower(strings.TrimSpace(identifier))
	if resolved == "" {
		return "", AccountPrivateState{}, false
	}
	for accountID, privateState := range s.private {
		if strings.EqualFold(strings.TrimSpace(privateState.Email), resolved) && strings.TrimSpace(privateState.PasswordHash) != "" {
			return accountID, privateState, true
		}
	}
	for accountID, account := range s.accounts {
		if strings.EqualFold(strings.TrimSpace(account.Handle), resolved) {
			privateState := s.private[accountID]
			if strings.TrimSpace(privateState.PasswordHash) == "" {
				return "", AccountPrivateState{}, false
			}
			return accountID, privateState, true
		}
	}
	return "", AccountPrivateState{}, false
}

func appendAccountRatingHistory(history []AccountRatingHistoryEntry, entry AccountRatingHistoryEntry) []AccountRatingHistoryEntry {
	next := append(append([]AccountRatingHistoryEntry{}, history...), entry)
	if len(next) <= maxAccountRatingHistoryEntries {
		return next
	}
	return append([]AccountRatingHistoryEntry{}, next[len(next)-maxAccountRatingHistoryEntries:]...)
}

func buildAccountRatingHistoryEntry(matchID, opponentAccountID, winner, queue string, modeID contracts.MatchModeID, ratingBefore, ratingAfter, matchesPlayed int, perspective string, at time.Time) AccountRatingHistoryEntry {
	result := "draw"
	switch {
	case winner == "draw":
		result = "draw"
	case winner == perspective:
		result = "win"
	default:
		result = "loss"
	}

	return AccountRatingHistoryEntry{
		MatchID:           strings.TrimSpace(matchID),
		OpponentAccountID: strings.TrimSpace(opponentAccountID),
		Queue:             strings.TrimSpace(queue),
		ModeID:            contracts.NormalizeMatchModeID(string(modeID)),
		Result:            result,
		Winner:            strings.TrimSpace(winner),
		Delta:             ratingAfter - ratingBefore,
		RatingBefore:      ratingBefore,
		RatingAfter:       ratingAfter,
		MatchesPlayed:     matchesPlayed,
		At:                at.UTC(),
	}
}

func applyAccountMatchResult(white, black *AccountProfile, winner string) error {
	if white == nil || black == nil {
		return os.ErrInvalid
	}
	newWhite, newBlack := ApplyEloMatchResult(white.Rating, black.Rating, winner)
	switch winner {
	case "white":
		white.Rating = newWhite
		black.Rating = newBlack
		white.Wins++
		black.Losses++
	case "black":
		white.Rating = newWhite
		black.Rating = newBlack
		black.Wins++
		white.Losses++
	case "draw":
		white.Rating = newWhite
		black.Rating = newBlack
		white.Draws++
		black.Draws++
	default:
		return os.ErrInvalid
	}

	return nil
}

func accountSessionTokenValid(state AccountPrivateState, sessionToken string, now time.Time) bool {
	for _, record := range activeAccountSessionRecords(state, now) {
		if accountSessionTokenMatches(record.SessionToken, sessionToken) {
			return true
		}
	}
	return false
}
