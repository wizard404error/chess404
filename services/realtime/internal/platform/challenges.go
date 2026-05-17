package platform

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chess404/realtime/internal/contracts"
)

var ErrInvalidDirectChallenge = errors.New("invalid direct challenge")
var ErrDirectChallengeAlreadyExists = errors.New("direct challenge already exists")
var ErrDirectChallengeNotFound = errors.New("direct challenge not found")
var ErrUnauthorizedDirectChallenge = errors.New("unauthorized direct challenge")
var ErrDirectChallengeNotPending = errors.New("direct challenge is not pending")

const (
	DirectChallengeStatusPending   = "pending"
	DirectChallengeStatusAccepted  = "accepted"
	DirectChallengeStatusDeclined  = "declined"
	DirectChallengeStatusCancelled = "cancelled"
)

type DirectChallenge struct {
	ChallengeID         string                `json:"challengeId"`
	ChallengerAccountID string                `json:"challengerAccountId"`
	TargetAccountID     string                `json:"targetAccountId"`
	MatchID             string                `json:"matchId"`
	ModeID              contracts.MatchModeID `json:"modeId,omitempty"`
	ClockSeconds        int64                 `json:"clockSeconds,omitempty"`
	ChallengerSeat      string                `json:"challengerSeat,omitempty"`
	Status              string                `json:"status"`
	CreatedAt           time.Time             `json:"createdAt"`
	UpdatedAt           time.Time             `json:"updatedAt"`
}

type DirectChallengeOverview struct {
	Incoming []DirectChallenge `json:"incoming"`
	Outgoing []DirectChallenge `json:"outgoing"`
}

type DirectChallengeStoreStats struct {
	ChallengeCount        int `json:"challengeCount"`
	PendingChallengeCount int `json:"pendingChallengeCount"`
}

type directChallengePersistence interface {
	backend() string
	load() (map[string]DirectChallenge, error)
	persist(map[string]DirectChallenge) error
	close() error
}

type DirectChallengeStore struct {
	mu         sync.Mutex
	store      directChallengePersistence
	challenges map[string]DirectChallenge
}

type directChallengeStoreFile struct {
	Challenges map[string]DirectChallenge `json:"challenges"`
}

type fileDirectChallengeStore struct {
	path string
}

func NewDirectChallengeStore(path string) (*DirectChallengeStore, error) {
	return newDirectChallengeStore(&fileDirectChallengeStore{path: path})
}

func newDirectChallengeStore(store directChallengePersistence) (*DirectChallengeStore, error) {
	challenges, err := store.load()
	if err != nil {
		return nil, err
	}
	if challenges == nil {
		challenges = make(map[string]DirectChallenge)
	}
	return &DirectChallengeStore{
		store:      store,
		challenges: challenges,
	}, nil
}

func (s *DirectChallengeStore) Backend() string {
	if s == nil || s.store == nil {
		return "memory"
	}
	return s.store.backend()
}

func (s *DirectChallengeStore) Close() error {
	if s == nil || s.store == nil {
		return nil
	}
	return s.store.close()
}

func (s *DirectChallengeStore) Stats() DirectChallengeStoreStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	pending := 0
	for _, challenge := range s.challenges {
		if challenge.Status == DirectChallengeStatusPending {
			pending++
		}
	}
	return DirectChallengeStoreStats{
		ChallengeCount:        len(s.challenges),
		PendingChallengeCount: pending,
	}
}

func (s *DirectChallengeStore) CanCreateChallenge(challengerAccountID, targetAccountID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.canCreateChallengeLocked(challengerAccountID, targetAccountID)
}

func (s *DirectChallengeStore) CreateChallenge(challengerAccountID, targetAccountID, matchID string, modeID contracts.MatchModeID, clockSeconds int64, challengerSeat string) (DirectChallenge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.canCreateChallengeLocked(challengerAccountID, targetAccountID); err != nil {
		return DirectChallenge{}, err
	}
	resolvedMatchID := strings.TrimSpace(matchID)
	if resolvedMatchID == "" {
		return DirectChallenge{}, ErrInvalidDirectChallenge
	}
	resolvedSeat := normalizeChallengeSeat(challengerSeat)
	if resolvedSeat == "" {
		return DirectChallenge{}, ErrInvalidDirectChallenge
	}
	if clockSeconds <= 0 {
		clockSeconds = 600
	}

	now := time.Now().UTC()
	challenge := DirectChallenge{
		ChallengeID:         "challenge_" + randomToken(8),
		ChallengerAccountID: strings.TrimSpace(challengerAccountID),
		TargetAccountID:     strings.TrimSpace(targetAccountID),
		MatchID:             resolvedMatchID,
		ModeID:              contracts.NormalizeMatchModeID(string(modeID)),
		ClockSeconds:        clockSeconds,
		ChallengerSeat:      resolvedSeat,
		Status:              DirectChallengeStatusPending,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	s.challenges[challenge.ChallengeID] = challenge
	if err := s.persistLocked(); err != nil {
		return DirectChallenge{}, err
	}
	return challenge, nil
}

func (s *DirectChallengeStore) GetChallenge(challengeID string) (DirectChallenge, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	challenge, ok := s.challenges[strings.TrimSpace(challengeID)]
	return challenge, ok
}

func (s *DirectChallengeStore) RespondToChallenge(targetAccountID, challengeID string, accept bool) (DirectChallenge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedTarget := strings.TrimSpace(targetAccountID)
	resolvedChallengeID := strings.TrimSpace(challengeID)
	if resolvedTarget == "" || resolvedChallengeID == "" {
		return DirectChallenge{}, ErrInvalidDirectChallenge
	}
	challenge, ok := s.challenges[resolvedChallengeID]
	if !ok {
		return DirectChallenge{}, ErrDirectChallengeNotFound
	}
	if challenge.TargetAccountID != resolvedTarget {
		return DirectChallenge{}, ErrUnauthorizedDirectChallenge
	}
	if challenge.Status != DirectChallengeStatusPending {
		return DirectChallenge{}, ErrDirectChallengeNotPending
	}
	challenge.UpdatedAt = time.Now().UTC()
	if accept {
		challenge.Status = DirectChallengeStatusAccepted
	} else {
		challenge.Status = DirectChallengeStatusDeclined
	}
	s.challenges[resolvedChallengeID] = challenge
	if err := s.persistLocked(); err != nil {
		return DirectChallenge{}, err
	}
	return challenge, nil
}

func (s *DirectChallengeStore) CancelChallenge(challengerAccountID, challengeID string) (DirectChallenge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedChallenger := strings.TrimSpace(challengerAccountID)
	resolvedChallengeID := strings.TrimSpace(challengeID)
	if resolvedChallenger == "" || resolvedChallengeID == "" {
		return DirectChallenge{}, ErrInvalidDirectChallenge
	}
	challenge, ok := s.challenges[resolvedChallengeID]
	if !ok {
		return DirectChallenge{}, ErrDirectChallengeNotFound
	}
	if challenge.ChallengerAccountID != resolvedChallenger {
		return DirectChallenge{}, ErrUnauthorizedDirectChallenge
	}
	if challenge.Status != DirectChallengeStatusPending {
		return DirectChallenge{}, ErrDirectChallengeNotPending
	}
	challenge.Status = DirectChallengeStatusCancelled
	challenge.UpdatedAt = time.Now().UTC()
	s.challenges[resolvedChallengeID] = challenge
	if err := s.persistLocked(); err != nil {
		return DirectChallenge{}, err
	}
	return challenge, nil
}

func (s *DirectChallengeStore) PurgePair(accountID, otherAccountID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	resolvedOtherAccountID := strings.TrimSpace(otherAccountID)
	if resolvedAccountID == "" || resolvedOtherAccountID == "" || resolvedAccountID == resolvedOtherAccountID {
		return ErrInvalidDirectChallenge
	}

	for challengeID, challenge := range s.challenges {
		if challenge.Status != DirectChallengeStatusPending {
			continue
		}
		if (challenge.ChallengerAccountID == resolvedAccountID && challenge.TargetAccountID == resolvedOtherAccountID) ||
			(challenge.ChallengerAccountID == resolvedOtherAccountID && challenge.TargetAccountID == resolvedAccountID) {
			delete(s.challenges, challengeID)
		}
	}
	return s.persistLocked()
}

func (s *DirectChallengeStore) ListOverview(accountID string) DirectChallengeOverview {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return DirectChallengeOverview{}
	}
	overview := DirectChallengeOverview{
		Incoming: make([]DirectChallenge, 0),
		Outgoing: make([]DirectChallenge, 0),
	}
	for _, challenge := range s.challenges {
		if challenge.Status != DirectChallengeStatusPending {
			continue
		}
		switch {
		case challenge.TargetAccountID == resolvedAccountID:
			overview.Incoming = append(overview.Incoming, challenge)
		case challenge.ChallengerAccountID == resolvedAccountID:
			overview.Outgoing = append(overview.Outgoing, challenge)
		}
	}
	sort.Slice(overview.Incoming, func(i, j int) bool {
		if overview.Incoming[i].UpdatedAt.Equal(overview.Incoming[j].UpdatedAt) {
			return overview.Incoming[i].ChallengeID < overview.Incoming[j].ChallengeID
		}
		return overview.Incoming[i].UpdatedAt.After(overview.Incoming[j].UpdatedAt)
	})
	sort.Slice(overview.Outgoing, func(i, j int) bool {
		if overview.Outgoing[i].UpdatedAt.Equal(overview.Outgoing[j].UpdatedAt) {
			return overview.Outgoing[i].ChallengeID < overview.Outgoing[j].ChallengeID
		}
		return overview.Outgoing[i].UpdatedAt.After(overview.Outgoing[j].UpdatedAt)
	})
	return overview
}

func (s *DirectChallengeStore) canCreateChallengeLocked(challengerAccountID, targetAccountID string) error {
	resolvedChallenger := strings.TrimSpace(challengerAccountID)
	resolvedTarget := strings.TrimSpace(targetAccountID)
	if resolvedChallenger == "" || resolvedTarget == "" || resolvedChallenger == resolvedTarget {
		return ErrInvalidDirectChallenge
	}
	for _, challenge := range s.challenges {
		if challenge.Status != DirectChallengeStatusPending {
			continue
		}
		if (challenge.ChallengerAccountID == resolvedChallenger && challenge.TargetAccountID == resolvedTarget) ||
			(challenge.ChallengerAccountID == resolvedTarget && challenge.TargetAccountID == resolvedChallenger) {
			return ErrDirectChallengeAlreadyExists
		}
	}
	return nil
}

func (s *DirectChallengeStore) persistLocked() error {
	if s.store == nil {
		return nil
	}
	return s.store.persist(s.challenges)
}

func (s *fileDirectChallengeStore) backend() string {
	return "file"
}

func (s *fileDirectChallengeStore) load() (map[string]DirectChallenge, error) {
	if strings.TrimSpace(s.path) == "" {
		return make(map[string]DirectChallenge), nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]DirectChallenge), nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return make(map[string]DirectChallenge), nil
	}
	var payload directChallengeStoreFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	if payload.Challenges == nil {
		payload.Challenges = make(map[string]DirectChallenge)
	}
	return payload.Challenges, nil
}

func (s *fileDirectChallengeStore) persist(challenges map[string]DirectChallenge) error {
	if strings.TrimSpace(s.path) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(directChallengeStoreFile{Challenges: challenges}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, encoded, 0o644)
}

func (s *fileDirectChallengeStore) close() error {
	return nil
}

func normalizeChallengeSeat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "white":
		return "white"
	case "black":
		return "black"
	default:
		return ""
	}
}

func ChallengeOpponentAccountID(challenge DirectChallenge, viewerAccountID string) string {
	if strings.TrimSpace(challenge.ChallengerAccountID) == strings.TrimSpace(viewerAccountID) {
		return challenge.TargetAccountID
	}
	return challenge.ChallengerAccountID
}

func ChallengeViewerSeat(challenge DirectChallenge, viewerAccountID string) string {
	if strings.TrimSpace(challenge.ChallengerAccountID) == strings.TrimSpace(viewerAccountID) {
		return challenge.ChallengerSeat
	}
	if challenge.ChallengerSeat == "white" {
		return "black"
	}
	if challenge.ChallengerSeat == "black" {
		return "white"
	}
	return ""
}
