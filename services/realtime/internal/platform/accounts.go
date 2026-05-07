package platform

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrInvalidAccountHandle = errors.New("invalid account handle")
var ErrAccountHandleTaken = errors.New("account handle already taken")
var ErrUnauthorizedAccountSession = errors.New("unauthorized account session")

const defaultAccountSessionTTL = 30 * 24 * time.Hour
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
}

type AccountRatingHistoryEntry struct {
	MatchID           string    `json:"matchId"`
	OpponentAccountID string    `json:"opponentAccountId,omitempty"`
	Result            string    `json:"result"`
	Winner            string    `json:"winner"`
	Delta             int       `json:"delta"`
	RatingBefore      int       `json:"ratingBefore"`
	RatingAfter       int       `json:"ratingAfter"`
	MatchesPlayed     int       `json:"matchesPlayed"`
	At                time.Time `json:"at"`
}

type AccountSession struct {
	Account      AccountProfile `json:"account"`
	SessionToken string         `json:"sessionToken"`
	ExpiresAt    time.Time      `json:"expiresAt"`
}

type AccountPrivateState struct {
	SessionToken string    `json:"sessionToken,omitempty"`
	ExpiresAt    time.Time `json:"expiresAt,omitempty"`
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
		if !privateState.ExpiresAt.IsZero() && privateState.ExpiresAt.After(now) {
			activeSessions++
		}
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
			account.LastSeenAt = now
			s.accounts[accountID] = account
			privateState := renewAccountPrivateState(s.private[accountID], now)
			s.private[accountID] = privateState
			if err := s.persistLocked(); err != nil {
				return AccountSession{}, err
			}
			return buildAccountSession(account, privateState), nil
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
	}
	privateState := renewAccountPrivateState(AccountPrivateState{}, now)
	s.accounts[accountID] = account
	s.guestLinks[guest.GuestID] = accountID
	s.private[accountID] = privateState
	if err := s.persistLocked(); err != nil {
		return AccountSession{}, err
	}
	return buildAccountSession(account, privateState), nil
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
	if !accountSessionTokenValid(privateState, sessionToken, time.Now().UTC()) {
		return AccountSession{}, ErrUnauthorizedAccountSession
	}

	now := time.Now().UTC()
	account.LastSeenAt = now
	s.accounts[accountID] = account
	privateState = renewAccountPrivateState(privateState, now)
	s.private[accountID] = privateState
	if err := s.persistLocked(); err != nil {
		return AccountSession{}, err
	}
	return buildAccountSession(account, privateState), nil
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

func (s *AccountStore) FinalizeMatch(matchID, whiteAccountID, blackAccountID, winner string) (AccountProfile, AccountProfile, bool, error) {
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
	switch winner {
	case "white":
		white.Rating += 16
		black.Rating = maxInt(100, black.Rating-16)
		white.Wins++
		black.Losses++
	case "black":
		black.Rating += 16
		white.Rating = maxInt(100, white.Rating-16)
		black.Wins++
		white.Losses++
	case "draw":
		white.Draws++
		black.Draws++
	default:
		return AccountProfile{}, AccountProfile{}, false, os.ErrInvalid
	}
	white.MatchesPlayed++
	black.MatchesPlayed++
	white.LastSeenAt = now
	black.LastSeenAt = now
	white.RatingHistory = appendAccountRatingHistory(white.RatingHistory, buildAccountRatingHistoryEntry(matchID, black.AccountID, winner, whiteBefore, white.Rating, white.MatchesPlayed, "white", now))
	black.RatingHistory = appendAccountRatingHistory(black.RatingHistory, buildAccountRatingHistoryEntry(matchID, white.AccountID, winner, blackBefore, black.Rating, black.MatchesPlayed, "black", now))

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

func renewAccountPrivateState(state AccountPrivateState, now time.Time) AccountPrivateState {
	state.SessionToken = strings.TrimSpace(state.SessionToken)
	if state.SessionToken == "" {
		state.SessionToken = "accttok_" + randomToken(18)
	}
	state.ExpiresAt = now.Add(defaultAccountSessionTTL).UTC()
	return state
}

func buildAccountSession(account AccountProfile, privateState AccountPrivateState) AccountSession {
	return AccountSession{
		Account:      account,
		SessionToken: strings.TrimSpace(privateState.SessionToken),
		ExpiresAt:    privateState.ExpiresAt.UTC(),
	}
}

func appendAccountRatingHistory(history []AccountRatingHistoryEntry, entry AccountRatingHistoryEntry) []AccountRatingHistoryEntry {
	next := append(append([]AccountRatingHistoryEntry{}, history...), entry)
	if len(next) <= maxAccountRatingHistoryEntries {
		return next
	}
	return append([]AccountRatingHistoryEntry{}, next[len(next)-maxAccountRatingHistoryEntries:]...)
}

func buildAccountRatingHistoryEntry(matchID, opponentAccountID, winner string, ratingBefore, ratingAfter, matchesPlayed int, perspective string, at time.Time) AccountRatingHistoryEntry {
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
		Result:            result,
		Winner:            strings.TrimSpace(winner),
		Delta:             ratingAfter - ratingBefore,
		RatingBefore:      ratingBefore,
		RatingAfter:       ratingAfter,
		MatchesPlayed:     matchesPlayed,
		At:                at.UTC(),
	}
}

func accountSessionTokenValid(state AccountPrivateState, sessionToken string, now time.Time) bool {
	resolved := strings.TrimSpace(state.SessionToken)
	requested := strings.TrimSpace(sessionToken)
	if resolved == "" || requested == "" {
		return false
	}
	if subtle.ConstantTimeCompare([]byte(resolved), []byte(requested)) != 1 {
		return false
	}
	if state.ExpiresAt.IsZero() || !state.ExpiresAt.After(now) {
		return false
	}
	return true
}
