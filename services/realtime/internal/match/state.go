package match

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chess404/realtime/internal/contracts"
	"github.com/chess404/realtime/internal/engine"
	"github.com/chess404/realtime/internal/logging"
)

const (
	rulesVersion                 = "v1-alpha-foundation"
	defaultClock                 = int64(10 * 60 * 1000)
	maxHandSize                  = 10
	drawFromRound                = 8
	drawEveryRounds              = 3
	presenceHeartbeatTimeout     = 25 * time.Second
	disconnectGracePeriod        = 45 * time.Second
	disconnectGraceBothPeriod    = 2 * time.Minute
	disconnectGraceBoth          = "both"
	maxIntentsPerSecondPerPlayer = 10
)

var (
	ErrMatchNotFound     = errors.New("match not found")
	ErrMatchSeatFull     = errors.New("match has no open seats")
	ErrMatchJoinFinished = errors.New("match is finished")
	ErrStaleClientState  = errors.New("client state is stale; refresh from latest snapshot")
)

type Service struct {
	mu          sync.RWMutex
	matches     map[string]*contracts.MatchState
	events      map[string][]contracts.ResolvedEvent
	subs        map[string]map[chan contracts.MatchSnapshotResponse]string
	presence    map[string]*matchPresenceState
	matchMu     map[string]*sync.Mutex
	archive     MatchArchiver
	store       MatchStore
	broadcaster Broadcaster
	stopCh      chan struct{}
	matchSeqNum map[string]int64
	authTokens  map[string]authTokenEntry
	Log         *logging.Logger
	broadcastWG sync.WaitGroup
	computers   map[string]*engine.ComputerOpponent
}

type authTokenEntry struct {
	PlayerID     string
	PlayerSecret string
	ExpiresAt    time.Time
}

type matchPresenceState struct {
	WhiteLastSeenAt         time.Time
	BlackLastSeenAt         time.Time
	WhiteConnected          bool
	BlackConnected          bool
	DisconnectGraceFor      string
	DisconnectGraceDeadline *time.Time
	WhiteLastIntentAt       time.Time
	BlackLastIntentAt       time.Time
}

type MatchArchiver interface {
	Upsert(snapshot contracts.MatchSnapshotResponse) error
}

type MatchArchiveLoader interface {
	MatchArchiver
	LoadMatch(matchID string) (contracts.MatchState, []contracts.ResolvedEvent, bool)
}

type MatchArchiveBootstrapper interface {
	MatchArchiveLoader
	ListUnfinishedMatchIDs(limit int) []string
}

type ServiceStats struct {
	LoadedMatches     int `json:"loadedMatches"`
	ActiveMatches     int `json:"activeMatches"`
	FinishedMatches   int `json:"finishedMatches"`
	SubscriberCount   int `json:"subscriberCount"`
	BufferedEventSets int `json:"bufferedEventSets"`
}

func NewService() *Service {
	return NewServiceWithArchive(nil)
}

func NewServiceWithArchive(archive MatchArchiver) *Service {
	return NewServiceWithStoreAndBroadcaster(archive, NewMemoryMatchStore(), NoopBroadcaster{})
}

func NewServiceWithStoreAndBroadcaster(archive MatchArchiver, store MatchStore, broadcaster Broadcaster) *Service {
	service := &Service{
		matches:     make(map[string]*contracts.MatchState),
		events:      make(map[string][]contracts.ResolvedEvent),
		subs:        make(map[string]map[chan contracts.MatchSnapshotResponse]string),
		presence:    make(map[string]*matchPresenceState),
		matchMu:     make(map[string]*sync.Mutex),
		matchSeqNum: make(map[string]int64),
		archive:     archive,
		store:       store,
		broadcaster: broadcaster,
		stopCh:      make(chan struct{}),
		authTokens:  make(map[string]authTokenEntry),
		Log:         logging.New("match-service"),
		computers:   make(map[string]*engine.ComputerOpponent),
	}
	if loader, ok := archive.(MatchArchiveBootstrapper); ok {
		service.mu.Lock()
		service.restoreArchivedMatchesLocked(loader)
		service.mu.Unlock()
	}

	go service.startBroadcaster()
	go service.cleanupAuthTokensLoop()

	return service
}

func (s *Service) GetMatch(matchID string) (contracts.MatchSnapshotResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.ensureMatchLoadedLocked(matchID)
	if !ok {
		return contracts.MatchSnapshotResponse{}, ErrMatchNotFound
	}

	return buildSnapshotWithPresence(state, s.ensurePresenceStateLocked(matchID, state, time.Now().UTC()), 0, nil, time.Now().UTC()), nil
}

func (s *Service) HeartbeatPresence(matchID string, req contracts.MatchPresenceRequest, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.ensureMatchLoadedLocked(matchID)
	if !ok {
		return ErrMatchNotFound
	}
	if state.Status == "finished" {
		return nil
	}

	color, err := requireIntentColor(state, strings.TrimSpace(req.PlayerID), strings.TrimSpace(req.PlayerSecret))
	if err != nil {
		return err
	}

	presence := s.ensurePresenceStateLocked(matchID, state, now)
	changed := presenceHeartbeat(presence, color, now)
	if !changed {
		return nil
	}

	return nil
}

func (s *Service) MarkDisconnected(matchID string, playerID string, playerSecret string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.ensureMatchLoadedLocked(matchID)
	if !ok || state.Status == "finished" {
		return nil
	}

	color, err := requireIntentColor(state, strings.TrimSpace(playerID), strings.TrimSpace(playerSecret))
	if err != nil {
		return err
	}

	presence := s.ensurePresenceStateLocked(matchID, state, now)
	if color == "white" {
		if !presence.WhiteConnected {
			return nil
		}
		presence.WhiteLastSeenAt = time.Time{}
		presence.WhiteConnected = false
	} else {
		if state.ModeID == contracts.MatchModeComputer {
			return nil
		}
		if !presence.BlackConnected {
			return nil
		}
		presence.BlackLastSeenAt = time.Time{}
		presence.BlackConnected = false
	}

	snapshot := buildSnapshotWithPresence(state, presence, len(s.events[matchID]), nil, now)
	s.broadcastLocked(matchID, snapshot)
	return nil
}

// redactPlayerSecret returns a short fingerprint of a player
// secret so logs are useful for debugging without exposing the full
// secret. Empty string becomes "<empty>".
func redactPlayerSecret(s string) string {
	if s == "" {
		return "<empty>"
	}
	if len(s) <= 6 {
		return s[:1] + "***"
	}
	return s[:6] + "...len=" + strconv.Itoa(len(s))
}

func (s *Service) Subscribe(matchID string, playerID string) (<-chan contracts.MatchSnapshotResponse, func(), contracts.MatchSnapshotResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.ensureMatchLoadedLocked(matchID)
	if !ok {
		return nil, nil, contracts.MatchSnapshotResponse{}, ErrMatchNotFound
	}

	if s.subs[matchID] == nil {
		s.subs[matchID] = make(map[chan contracts.MatchSnapshotResponse]string)
	}

	const maxSubscribersPerMatch = 50
	if len(s.subs[matchID]) >= maxSubscribersPerMatch {
		return nil, nil, contracts.MatchSnapshotResponse{}, errors.New("max subscribers reached for match")
	}

	playerColor := ""
	if (state.WhiteGuestID != "" && strings.EqualFold(state.WhiteGuestID, playerID)) || (state.WhiteAccountID != "" && strings.EqualFold(state.WhiteAccountID, playerID)) {
		playerColor = "white"
	} else if (state.BlackGuestID != "" && strings.EqualFold(state.BlackGuestID, playerID)) || (state.BlackAccountID != "" && strings.EqualFold(state.BlackAccountID, playerID)) {
		playerColor = "black"
	}

	if playerColor == "" {
		return nil, nil, contracts.MatchSnapshotResponse{}, errors.New("unauthorized: must be a seated player to subscribe")
	}

	ch := make(chan contracts.MatchSnapshotResponse, 128)
	s.subs[matchID][ch] = playerColor

	now := time.Now().UTC()
	baseInitial := buildSnapshotWithPresence(state, s.ensurePresenceStateLocked(matchID, state, now), len(s.events[matchID]), s.events[matchID], now)
	initial := contracts.MatchSnapshotResponse{
		Match:      filterStateForColor(baseInitial.Match, playerColor),
		ReplayHead: baseInitial.ReplayHead,
		Events:     baseInitial.Events,
	}

	unsubscribe := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if subs, exists := s.subs[matchID]; exists {
			if _, present := subs[ch]; present {
				delete(subs, ch)
				close(ch)
			}
		}
	}

	return ch, unsubscribe, initial, nil
}

func (s *Service) ensureMatchLoadedLocked(matchID string) (*contracts.MatchState, bool) {
	if state, ok := s.matches[matchID]; ok {
		return state, true
	}

	loader, ok := s.archive.(MatchArchiveLoader)
	if !ok {
		return nil, false
	}

	restored, events, ok := loader.LoadMatch(matchID)
	if !ok {
		return nil, false
	}

	if len(restored.History) == 0 {
		restored.History = []contracts.PositionState{capturePositionState(&restored)}
	}

	return s.loadArchivedMatchLocked(matchID, restored, events), true
}

func (s *Service) restoreArchivedMatchesLocked(loader MatchArchiveBootstrapper) {
	for _, matchID := range loader.ListUnfinishedMatchIDs(0) {
		if _, ok := s.matches[matchID]; ok {
			continue
		}
		restored, events, ok := loader.LoadMatch(matchID)
		if !ok {
			continue
		}
		if len(restored.History) == 0 {
			restored.History = []contracts.PositionState{capturePositionState(&restored)}
		}
		s.loadArchivedMatchLocked(matchID, restored, events)
	}
}

func (s *Service) loadArchivedMatchLocked(matchID string, restored contracts.MatchState, events []contracts.ResolvedEvent) *contracts.MatchState {
	state := &restored
	s.matches[matchID] = state
	s.events[matchID] = append([]contracts.ResolvedEvent{}, events...)
	if s.subs[matchID] == nil {
		s.subs[matchID] = make(map[chan contracts.MatchSnapshotResponse]string)
	}
	s.presence[matchID] = newRecoveredMatchPresenceState(state)
	return state
}

const authTokenTTL = 5 * time.Minute

func (s *Service) CreateAuthToken(playerID, playerSecret string, now time.Time) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		h := sha256.Sum256([]byte(fmt.Sprintf("%s_%s_%d", playerID, playerSecret, now.UnixNano())))
		token := "at_" + hex.EncodeToString(h[:16])
		s.authTokens[token] = authTokenEntry{
			PlayerID:     playerID,
			PlayerSecret: playerSecret,
			ExpiresAt:    now.Add(authTokenTTL),
		}
		return token
	}
	token := "at_" + hex.EncodeToString(raw)
	s.authTokens[token] = authTokenEntry{
		PlayerID:     playerID,
		PlayerSecret: playerSecret,
		ExpiresAt:    now.Add(authTokenTTL),
	}
	return token
}

func (s *Service) ResolveAuthToken(token string) (string, string, bool) {
	if token == "" {
		return "", "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.authTokens[token]
	if !ok {
		return "", "", false
	}
	if time.Now().After(entry.ExpiresAt) {
		delete(s.authTokens, token)
		return "", "", false
	}
	delete(s.authTokens, token)
	return entry.PlayerID, entry.PlayerSecret, true
}

func (s *Service) cleanupAuthTokensLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case now := <-ticker.C:
			s.mu.Lock()
			for token, entry := range s.authTokens {
				if now.After(entry.ExpiresAt) {
					delete(s.authTokens, token)
				}
			}
			s.mu.Unlock()
		}
	}
}

func (s *Service) Stats() ServiceStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := ServiceStats{
		LoadedMatches:     len(s.matches),
		BufferedEventSets: len(s.events),
	}
	for matchID, state := range s.matches {
		if state.Status == "finished" {
			stats.FinishedMatches++
		} else {
			stats.ActiveMatches++
		}
		stats.SubscriberCount += len(s.subs[matchID])
	}
	return stats
}

func (s *Service) Close() {
	close(s.stopCh)
	s.broadcastWG.Wait()
}

func (s *Service) persistSnapshot(snapshot contracts.MatchSnapshotResponse) {
	if s.archive == nil {
		return
	}
	persisted := snapshot
	persisted.Match.WhiteConnected = false
	persisted.Match.BlackConnected = false
	persisted.Match.DisconnectGraceFor = ""
	persisted.Match.DisconnectGraceDeadline = nil
	if err := s.archive.Upsert(persisted); err != nil {
		s.Log.Error("failed to persist snapshot", "matchId", snapshot.Match.MatchID, "error", err)
	}
}

func (s *Service) saveToRedis(snapshot contracts.MatchSnapshotResponse, presence *matchPresenceState) {
	if s.store == nil {
		return
	}
	matchID := snapshot.Match.MatchID

	if err := s.store.SaveState(matchID, snapshot); err != nil {
		s.Log.Error("failed to save state to redis", "matchId", matchID, "error", err)
	}

	if err := s.store.SaveSecrets(matchID, snapshot.Match.WhitePlayerSecret, snapshot.Match.BlackPlayerSecret); err != nil {
		s.Log.Error("failed to save secrets to redis", "matchId", matchID, "error", err)
	}

	historyData, err := json.Marshal(snapshot.Match.History)
	if err == nil {
		_ = s.store.SaveHistory(matchID, historyData)
	}

	eventsData, err := json.Marshal(snapshot.Events)
	if err == nil {
		_ = s.store.SaveEvents(matchID, eventsData)
	}

	if presence != nil {
		presenceData, err := json.Marshal(presence)
		if err == nil {
			_ = s.store.SavePresence(matchID, presenceData)
		}
	}
}

func (s *Service) publishToRedis(matchID string, snapshot contracts.MatchSnapshotResponse) {
	if s.broadcaster == nil {
		return
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		s.Log.Error("failed to marshal snapshot for broadcast", "matchId", matchID, "error", err)
		return
	}
	if err := s.broadcaster.Publish(matchID, data); err != nil {
		s.Log.Error("failed to publish to redis", "matchId", matchID, "error", err)
	}
}

func (s *Service) broadcastLocked(matchID string, snapshot contracts.MatchSnapshotResponse) {
	subscribers := s.subs[matchID]

	s.matchSeqNum[matchID]++
	snapshot.SeqNum = s.matchSeqNum[matchID]

	s.publishToRedis(matchID, snapshot)

	if len(subscribers) == 0 {
		return
	}

	cachedWhite := snapshot
	cachedWhite.Match = filterStateForColor(snapshot.Match, "white")
	cachedBlack := snapshot
	cachedBlack.Match = filterStateForColor(snapshot.Match, "black")
	cachedSpec := snapshot
	cachedSpec.Match = filterStateForColor(snapshot.Match, "")

	for ch, color := range subscribers {
		if color == "white" {
			pushSnapshot(ch, cachedWhite)
		} else if color == "black" {
			pushSnapshot(ch, cachedBlack)
		} else {
			pushSnapshot(ch, cachedSpec)
		}
	}
}

func pushSnapshot(ch chan contracts.MatchSnapshotResponse, snapshot contracts.MatchSnapshotResponse) {
	defer func() {
		_ = recover()
	}()
	select {
	case ch <- snapshot:
	default:
		log.Printf("pushSnapshot: dropping event seq=%d for channel %p (buffer full)", snapshot.SeqNum, ch)
	}
}

func (s *Service) startBroadcaster() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case now, ok := <-ticker.C:
			if !ok {
				return
			}
			s.broadcastWG.Add(1)
			s.collectAndBroadcast(now.UTC())
			s.broadcastWG.Done()
			s.gcFinishedMatches(now.UTC())
		}
	}
}

func (s *Service) collectAndBroadcast(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for matchID, state := range s.matches {
		if state.Status == "finished" {
			continue
		}

		presence := s.ensurePresenceStateLocked(matchID, state, now)

		recentCutoff := now.Add(-presenceHeartbeatTimeout)
		hasRecentActivity := (!presence.WhiteLastSeenAt.IsZero() && presence.WhiteLastSeenAt.After(recentCutoff)) ||
			(!presence.BlackLastSeenAt.IsZero() && presence.BlackLastSeenAt.After(recentCutoff))
		if hasRecentActivity {
			timeoutEvents := syncClockForMutation(state, now)
			if len(timeoutEvents) > 0 {
				s.events[matchID] = append(s.events[matchID], timeoutEvents...)
				s.broadcastLocked(matchID, buildSnapshotWithPresence(state, presence, len(s.events[matchID]), timeoutEvents, now))
			}
			s.persistSnapshot(buildSnapshot(state, len(s.events[matchID]), s.events[matchID], now))
			if len(timeoutEvents) > 0 {
				continue
			}
		}

		runtimeEvents := evaluatePresenceRuntime(state, presence, now)
		if len(runtimeEvents) > 0 {
			s.events[matchID] = append(s.events[matchID], runtimeEvents...)
			s.persistSnapshot(buildSnapshot(state, len(s.events[matchID]), s.events[matchID], now))
			s.broadcastLocked(matchID, buildSnapshotWithPresence(state, presence, len(s.events[matchID]), runtimeEvents, now))
			continue
		}
		if len(s.subs[matchID]) == 0 {
			continue
		}
		s.broadcastLocked(matchID, buildSnapshotWithPresence(state, presence, len(s.events[matchID]), nil, now))
	}
}

func (s *Service) gcFinishedMatches(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	const finishedMatchTTL = 5 * time.Minute
	const waitingMatchTTL = 5 * time.Minute

	for matchID, state := range s.matches {
		if state.Status == "finished" {
			if now.Sub(state.UpdatedAt) >= finishedMatchTTL {
				delete(s.matches, matchID)
				delete(s.events, matchID)
				delete(s.subs, matchID)
				delete(s.matchSeqNum, matchID)
				delete(s.presence, matchID)
				delete(s.matchMu, matchID)
				delete(s.computers, matchID)
			}
		} else if state.Status == "waiting" {
			if now.Sub(state.UpdatedAt) >= waitingMatchTTL {
				delete(s.matches, matchID)
				delete(s.events, matchID)
				delete(s.subs, matchID)
				delete(s.matchSeqNum, matchID)
				delete(s.presence, matchID)
				delete(s.matchMu, matchID)
				delete(s.computers, matchID)
			}
		}
	}
}

func rateLimitIntent(presence *matchPresenceState, actorColor string, now time.Time) error {
	if presence == nil || actorColor == "" {
		return nil
	}
	var lastIntentAt *time.Time
	if actorColor == "white" {
		lastIntentAt = &presence.WhiteLastIntentAt
	} else if actorColor == "black" {
		lastIntentAt = &presence.BlackLastIntentAt
	} else {
		return nil
	}
	if !lastIntentAt.IsZero() && now.Sub(*lastIntentAt) < time.Second/time.Duration(maxIntentsPerSecondPerPlayer) {
		return fmt.Errorf("rate limited: too many intents (max %d/sec)", maxIntentsPerSecondPerPlayer)
	}
	return nil
}

func trackIntentTime(presence *matchPresenceState, actorColor string, now time.Time) {
	if presence == nil || actorColor == "" {
		return
	}
	if actorColor == "white" {
		presence.WhiteLastIntentAt = now
	} else if actorColor == "black" {
		presence.BlackLastIntentAt = now
	}
}
