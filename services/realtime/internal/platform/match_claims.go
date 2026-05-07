package platform

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

const defaultMatchClaimTTL = 12 * time.Hour

type MatchSeatClaim struct {
	MatchID      string    `json:"matchId"`
	GuestID      string    `json:"guestId"`
	SeatColor    string    `json:"seatColor"`
	PlayerID     string    `json:"playerId"`
	PlayerSecret string    `json:"playerSecret"`
	ClaimToken   string    `json:"claimToken,omitempty"`
	ExpiresAt    time.Time `json:"expiresAt,omitempty"`
	Queue        string    `json:"queue,omitempty"`
	WhiteGuestID string    `json:"whiteGuestId,omitempty"`
	BlackGuestID string    `json:"blackGuestId,omitempty"`
	WhiteName    string    `json:"whiteName,omitempty"`
	BlackName    string    `json:"blackName,omitempty"`
	Status       string    `json:"status,omitempty"`
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
	for _, claim := range s.claims {
		if claim.MatchID == matchID && claim.ClaimToken == claimToken {
			return claim, true
		}
	}
	return MatchSeatClaim{}, false
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
