package platform

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"sync"
	"time"

	"github.com/chess404/realtime/internal/contracts"
)

const defaultMatchClaimTTL = 12 * time.Hour

type MatchSeatClaim struct {
	MatchID      string                `json:"matchId"`
	GuestID      string                `json:"guestId"`
	SeatColor    string                `json:"seatColor"`
	PlayerID     string                `json:"playerId"`
	PlayerSecret string                `json:"-"`
	ClaimToken   string                `json:"claimToken,omitempty"`
	ExpiresAt    time.Time             `json:"expiresAt,omitempty"`
	Queue        string                `json:"queue,omitempty"`
	ModeID       contracts.MatchModeID `json:"modeId,omitempty"`
	WhiteGuestID string                `json:"whiteGuestId,omitempty"`
	BlackGuestID string                `json:"blackGuestId,omitempty"`
	WhiteName    string                `json:"whiteName,omitempty"`
	BlackName    string                `json:"blackName,omitempty"`
	CreatedAt    time.Time             `json:"createdAt,omitempty"`
	UpdatedAt    time.Time             `json:"updatedAt,omitempty"`
}

// IssuedMatchSeatClaim is the credential-bearing envelope returned at
// claim-issue / claim-renewal time. The raw PlayerSecret is exposed only in
// this view; the regular MatchSeatClaim JSON serialization omits the secret.
type IssuedMatchSeatClaim struct {
	MatchSeatClaim
	PlayerSecret string `json:"playerSecret"`
}

// PublicView returns the JSON-safe view of the claim (PlayerSecret omitted).
func (c MatchSeatClaim) PublicView() MatchSeatClaim {
	public := c
	public.PlayerSecret = ""
	return public
}

// IssuedView returns the credential-bearing view (PlayerSecret included).
// This must only be used at claim-issue / claim-renewal endpoints.
func (c MatchSeatClaim) IssuedView() IssuedMatchSeatClaim {
	return IssuedMatchSeatClaim{MatchSeatClaim: c, PlayerSecret: c.PlayerSecret}
}

type MatchClaimStats struct {
	CachedClaims int `json:"cachedClaims"`
}

type claimPersistence interface {
	backend() string
	load() (map[string]MatchSeatClaim, error)
	persist(map[string]MatchSeatClaim) error
	close() error
}

type MatchClaimStore struct {
	mu     sync.Mutex
	store  claimPersistence
	claims map[string]MatchSeatClaim
	ttl    time.Duration
}

func NewMatchClaimStore() *MatchClaimStore {
	return NewMatchClaimStoreWithTTL(defaultMatchClaimTTL)
}

func NewMatchClaimStoreWithTTL(ttl time.Duration) *MatchClaimStore {
	return &MatchClaimStore{
		claims: make(map[string]MatchSeatClaim),
		ttl:    normalizeMatchClaimTTL(ttl),
	}
}

func NewRedisMatchClaimStore(redisURL, key string) (*MatchClaimStore, error) {
	return NewRedisMatchClaimStoreWithTTL(redisURL, key, defaultMatchClaimTTL)
}

func NewRedisMatchClaimStoreWithTTL(redisURL, key string, ttl time.Duration) (*MatchClaimStore, error) {
	store, err := newRedisClaimStore(redisURL, key)
	if err != nil {
		return nil, err
	}
	claims, err := store.load()
	if err != nil {
		_ = store.close()
		return nil, err
	}
	return &MatchClaimStore{
		store:  store,
		claims: claims,
		ttl:    normalizeMatchClaimTTL(ttl),
	}, nil
}

func (s *MatchClaimStore) Backend() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store == nil {
		return "memory"
	}
	return s.store.backend()
}

func (s *MatchClaimStore) TTLSeconds() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return int(s.ttl / time.Second)
}

func (s *MatchClaimStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store == nil {
		return nil
	}
	return s.store.close()
}

func (s *MatchClaimStore) Stats() MatchClaimStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneExpiredLocked(time.Now().UTC())
	return MatchClaimStats{
		CachedClaims: len(s.claims),
	}
}

func (s *MatchClaimStore) Get(matchID, guestID string) (MatchSeatClaim, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	s.pruneExpiredLocked(now)
	claim, ok := s.claims[matchClaimKey(matchID, guestID)]
	return claim, ok
}

func (s *MatchClaimStore) GetByToken(matchID, claimToken string) (MatchSeatClaim, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	s.pruneExpiredLocked(now)
	for key, claim := range s.claims {
		if claim.MatchID == matchID && subtle.ConstantTimeCompare([]byte(claim.ClaimToken), []byte(claimToken)) == 1 {
			delete(s.claims, key)
			_ = s.persistLocked()
			return claim, true
		}
	}
	return MatchSeatClaim{}, false
}

func (s *MatchClaimStore) FindByGuest(guestID string) (MatchSeatClaim, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	s.pruneExpiredLocked(now)

	var latest MatchSeatClaim
	found := false
	for _, claim := range s.claims {
		if claim.GuestID != guestID {
			continue
		}
		if !found || claim.ExpiresAt.After(latest.ExpiresAt) {
			latest = claim
			found = true
		}
	}
	return latest, found
}

func (s *MatchClaimStore) Put(claim MatchSeatClaim) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	s.pruneExpiredLocked(now)
	key := matchClaimKey(claim.MatchID, claim.GuestID)
	existing := s.claims[key]
	if claim.ClaimToken == "" {
		if existing.ClaimToken != "" {
			claim.ClaimToken = existing.ClaimToken
		} else {
			token, err := newClaimToken()
			if err != nil {
				return err
			}
			claim.ClaimToken = token
		}
	}
	claim.ExpiresAt = now.Add(s.ttl)
	s.claims[key] = claim
	return s.persistLocked()
}

func (s *MatchClaimStore) Delete(matchID, guestID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	s.pruneExpiredLocked(now)
	key := matchClaimKey(matchID, guestID)
	if _, ok := s.claims[key]; !ok {
		return nil
	}
	delete(s.claims, key)
	return s.persistLocked()
}

func matchClaimKey(matchID, guestID string) string {
	return matchID + "::" + guestID
}

func newClaimToken() (string, error) {
	raw := make([]byte, 18)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "claimtok_" + hex.EncodeToString(raw), nil
}

func normalizeMatchClaimTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return defaultMatchClaimTTL
	}
	return ttl
}

func (s *MatchClaimStore) pruneExpiredLocked(now time.Time) {
	changed := false
	for key, claim := range s.claims {
		if !claim.ExpiresAt.IsZero() && !claim.ExpiresAt.After(now) {
			delete(s.claims, key)
			changed = true
		}
	}
	if changed {
		_ = s.persistLocked()
	}
}

func (s *MatchClaimStore) persistLocked() error {
	if s.store == nil {
		return nil
	}
	return s.store.persist(s.claims)
}
