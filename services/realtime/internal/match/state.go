package match

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chess404/realtime/internal/contracts"
	"github.com/chess404/realtime/internal/engine"
	"github.com/chess404/realtime/internal/logging"
	"github.com/chess404/realtime/internal/metrics"
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
	matchMapShards               = 32
)

type matchShard struct {
	mu      sync.RWMutex
	matches map[string]*matchContainer
}

var (
	ErrMatchNotFound     = errors.New("match not found")
	ErrMatchSeatFull     = errors.New("match has no open seats")
	ErrMatchJoinFinished = errors.New("match is finished")
	ErrStaleClientState  = errors.New("client state is stale; refresh from latest snapshot")
)

type matchContainer struct {
	mu       sync.Mutex
	state    *contracts.MatchState
	events   []contracts.ResolvedEvent
	presence *matchPresenceState
	subs     map[chan contracts.MatchSnapshotResponse]string
	seqNum   int64
	computer *engine.ComputerOpponent
}

func newMatchContainer(state *contracts.MatchState, events []contracts.ResolvedEvent, presence *matchPresenceState) *matchContainer {
	return &matchContainer{
		state:    state,
		events:   events,
		presence: presence,
		subs:     make(map[chan contracts.MatchSnapshotResponse]string),
	}
}

type computerMoveTask struct {
	c   *matchContainer
	now time.Time
}

type matchMap struct {
	shards [matchMapShards]*matchShard
}

func newMatchMap() *matchMap {
	mm := &matchMap{}
	for i := 0; i < matchMapShards; i++ {
		mm.shards[i] = &matchShard{matches: make(map[string]*matchContainer)}
	}
	return mm
}

func (mm *matchMap) shardFor(matchID string) *matchShard {
	h := sha256.Sum256([]byte(matchID))
	idx := int(h[0]) % matchMapShards
	return mm.shards[idx]
}

func (mm *matchMap) Load(matchID string) (*matchContainer, bool) {
	s := mm.shardFor(matchID)
	s.mu.RLock()
	c, ok := s.matches[matchID]
	s.mu.RUnlock()
	return c, ok
}

func (mm *matchMap) Store(matchID string, c *matchContainer) {
	s := mm.shardFor(matchID)
	s.mu.Lock()
	s.matches[matchID] = c
	s.mu.Unlock()
}

func (mm *matchMap) Delete(matchID string) {
	s := mm.shardFor(matchID)
	s.mu.Lock()
	delete(s.matches, matchID)
	s.mu.Unlock()
}

func (mm *matchMap) Len() int {
	total := 0
	for i := 0; i < matchMapShards; i++ {
		mm.shards[i].mu.RLock()
		total += len(mm.shards[i].matches)
		mm.shards[i].mu.RUnlock()
	}
	return total
}

func (mm *matchMap) Range(fn func(matchID string, c *matchContainer) bool) {
	for i := 0; i < matchMapShards; i++ {
		mm.shards[i].mu.RLock()
		for id, c := range mm.shards[i].matches {
			if !fn(id, c) {
				mm.shards[i].mu.RUnlock()
				return
			}
		}
		mm.shards[i].mu.RUnlock()
	}
}

func (mm *matchMap) RangeLocked(fn func(matchID string, c *matchContainer)) {
	for i := 0; i < matchMapShards; i++ {
		mm.shards[i].mu.Lock()
		for id, c := range mm.shards[i].matches {
			fn(id, c)
		}
		mm.shards[i].mu.Unlock()
	}
}

type Service struct {
	mu               sync.Mutex
	matches          *matchMap
	archive          MatchArchiver
	store            MatchStore
	broadcaster      Broadcaster
	stopCh           chan struct{}
	authTokens       map[string]authTokenEntry
	tokenStore       TokenStore
	Log              *logging.Logger
	computerCh       chan computerMoveTask
	computerWorkerWg sync.WaitGroup
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
	WhiteTokens             float64
	WhiteLastRefill         time.Time
	BlackTokens             float64
	BlackLastRefill         time.Time
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
	return NewServiceWithStoreBroadcasterAndTokenStore(archive, store, broadcaster, nil)
}

func NewServiceWithStoreBroadcasterAndTokenStore(archive MatchArchiver, store MatchStore, broadcaster Broadcaster, tokenStore TokenStore) *Service {
	service := &Service{
		matches:     newMatchMap(),
		archive:     archive,
		store:       store,
		broadcaster: broadcaster,
		stopCh:      make(chan struct{}),
		authTokens:  make(map[string]authTokenEntry),
		tokenStore:  tokenStore,
		Log:         logging.New("match-service"),
		computerCh:  make(chan computerMoveTask, 100),
	}
	if loader, ok := archive.(MatchArchiveBootstrapper); ok {
		service.restoreArchivedMatchesLocked(loader)
	}

	go service.startBroadcaster()
	go service.startGC()
	go service.cleanupAuthTokensLoop()
	numWorkers := runtime.NumCPU()
	if numWorkers < 2 {
		numWorkers = 2
	}
	for i := 0; i < numWorkers; i++ {
		service.computerWorkerWg.Add(1)
		go service.computerWorker()
	}
	service.Log.Info("computer worker pool started", "workers", numWorkers)

	return service
}

func (s *Service) getMatchContainer(matchID string) *matchContainer {
	c, _ := s.matches.Load(matchID)
	return c
}

func (s *Service) GetMatch(matchID string) (contracts.MatchSnapshotResponse, error) {
	s.mu.Lock()
	c, ok := s.ensureMatchLoadedLocked(matchID)
	s.mu.Unlock()
	if !ok {
		return contracts.MatchSnapshotResponse{}, ErrMatchNotFound
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now().UTC()
	return buildSnapshotWithPresence(c.state, s.ensurePresenceStateLocked(c, now), len(c.events), nil, now), nil
}

func (s *Service) HeartbeatPresence(matchID string, req contracts.MatchPresenceRequest, now time.Time) error {
	s.mu.Lock()
	c, ok := s.ensureMatchLoadedLocked(matchID)
	s.mu.Unlock()
	if !ok {
		return ErrMatchNotFound
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state.Status == "finished" {
		return nil
	}

	color, err := requireIntentColor(c.state, strings.TrimSpace(req.PlayerID), strings.TrimSpace(req.PlayerSecret))
	if err != nil {
		return err
	}

	presence := s.ensurePresenceStateLocked(c, now)
	presenceHeartbeat(presence, color, now)
	return nil
}

func (s *Service) MarkDisconnected(matchID string, playerID string, playerSecret string, now time.Time) error {
	s.mu.Lock()
	c, ok := s.ensureMatchLoadedLocked(matchID)
	s.mu.Unlock()
	if !ok || c.state.Status == "finished" {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	color, err := requireIntentColor(c.state, strings.TrimSpace(playerID), strings.TrimSpace(playerSecret))
	if err != nil {
		return err
	}

	presence := s.ensurePresenceStateLocked(c, now)
	if color == "white" {
		if !presence.WhiteConnected {
			return nil
		}
		presence.WhiteLastSeenAt = time.Time{}
		presence.WhiteConnected = false
	} else {
		if c.state.ModeID == contracts.MatchModeComputer {
			return nil
		}
		if !presence.BlackConnected {
			return nil
		}
		presence.BlackLastSeenAt = time.Time{}
		presence.BlackConnected = false
	}

	snapshot := buildSnapshotWithPresence(c.state, presence, len(c.events), nil, now)
	s.broadcastLocked(c, snapshot)
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
	c, ok := s.ensureMatchLoadedLocked(matchID)
	s.mu.Unlock()
	if !ok {
		return nil, nil, contracts.MatchSnapshotResponse{}, ErrMatchNotFound
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.subs == nil {
		c.subs = make(map[chan contracts.MatchSnapshotResponse]string)
	}

	const maxSubscribersPerMatch = 50
	if len(c.subs) >= maxSubscribersPerMatch {
		return nil, nil, contracts.MatchSnapshotResponse{}, errors.New("max subscribers reached for match")
	}

	playerColor := ""
	if (c.state.WhiteGuestID != "" && strings.EqualFold(c.state.WhiteGuestID, playerID)) || (c.state.WhiteAccountID != "" && strings.EqualFold(c.state.WhiteAccountID, playerID)) {
		playerColor = "white"
	} else if (c.state.BlackGuestID != "" && strings.EqualFold(c.state.BlackGuestID, playerID)) || (c.state.BlackAccountID != "" && strings.EqualFold(c.state.BlackAccountID, playerID)) {
		playerColor = "black"
	}

	ch := make(chan contracts.MatchSnapshotResponse, 128)
	c.subs[ch] = playerColor

	now := time.Now().UTC()
	baseInitial := buildSnapshotWithPresence(c.state, s.ensurePresenceStateLocked(c, now), len(c.events), c.events, now)
	initial := contracts.MatchSnapshotResponse{
		Match:      filterStateForColor(baseInitial.Match, playerColor),
		ReplayHead: baseInitial.ReplayHead,
		Events:     baseInitial.Events,
	}

	unsubscribe := func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if _, present := c.subs[ch]; present {
			delete(c.subs, ch)
			close(ch)
		}
	}

	return ch, unsubscribe, initial, nil
}

func (s *Service) ensureMatchLoadedLocked(matchID string) (*matchContainer, bool) {
	if c, ok := s.matches.Load(matchID); ok {
		return c, true
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
		if _, ok := s.matches.Load(matchID); ok {
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

func (s *Service) loadArchivedMatchLocked(matchID string, restored contracts.MatchState, events []contracts.ResolvedEvent) *matchContainer {
	// Restore SeenClientMoveIDs from Redis store if available
	if s.store != nil {
		if data, err := s.store.LoadSeenClientMoveIDs(matchID); err == nil && len(data) > 0 {
			var ids []string
			if json.Unmarshal(data, &ids) == nil {
				restored.SeenClientMoveIDs = ids
			}
		}
	}
	c := newMatchContainer(&restored, append([]contracts.ResolvedEvent{}, events...), newRecoveredMatchPresenceState(&restored))
	s.matches.Store(matchID, c)
	return c
}

const authTokenTTL = 5 * time.Minute

func (s *Service) CreateAuthToken(playerID, playerSecret string, now time.Time) string {
	raw := make([]byte, 16)
	var token string
	if _, err := rand.Read(raw); err != nil {
		h := sha256.Sum256([]byte(fmt.Sprintf("%s_%s_%d", playerID, playerSecret, now.UnixNano())))
		token = "at_" + hex.EncodeToString(h[:16])
	} else {
		token = "at_" + hex.EncodeToString(raw)
	}
	entry := authTokenEntry{
		PlayerID:     playerID,
		PlayerSecret: playerSecret,
		ExpiresAt:    now.Add(authTokenTTL),
	}
	if s.tokenStore != nil {
		if err := s.tokenStore.Create(token, entry, authTokenTTL); err != nil {
			s.Log.Error("failed to store auth token in redis, falling back to memory", "error", err)
		} else {
			return token
		}
	}
	s.mu.Lock()
	s.authTokens[token] = entry
	s.mu.Unlock()
	return token
}

func (s *Service) ResolveAuthToken(token string) (string, string, bool) {
	if token == "" {
		return "", "", false
	}
	if s.tokenStore != nil {
		entry, ok, err := s.tokenStore.Resolve(token)
		if err != nil {
			s.Log.Error("failed to resolve auth token from redis, falling back to memory", "error", err)
		} else if ok {
			return entry.PlayerID, entry.PlayerSecret, true
		} else if !ok && err == nil {
			return "", "", false
		}
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
	if s.tokenStore != nil {
		return
	}
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
	stats := ServiceStats{}
	s.matches.Range(func(_ string, c *matchContainer) bool {
		c.mu.Lock()
		stats.LoadedMatches++
		stats.BufferedEventSets++
		if c.state.Status == "finished" {
			stats.FinishedMatches++
		} else {
			stats.ActiveMatches++
		}
		stats.SubscriberCount += len(c.subs)
		c.mu.Unlock()
		return true
	})
	return stats
}

func (s *Service) Close() {
	close(s.stopCh)
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

	stateForRedis := snapshot
	stateForRedis.Match.WhitePlayerSecret = ""
	stateForRedis.Match.BlackPlayerSecret = ""
	if err := s.store.SaveState(matchID, stateForRedis); err != nil {
		s.Log.Error("failed to save state to redis", "matchId", matchID, "error", err)
	}

	if err := s.store.SaveSecrets(matchID, hashSecret(snapshot.Match.WhitePlayerSecret), hashSecret(snapshot.Match.BlackPlayerSecret)); err != nil {
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

	if len(snapshot.Match.SeenClientMoveIDs) > 0 {
		seenIDsData, err := json.Marshal(snapshot.Match.SeenClientMoveIDs)
		if err == nil {
			_ = s.store.SaveSeenClientMoveIDs(matchID, seenIDsData)
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

func (s *Service) broadcastLocked(c *matchContainer, snapshot contracts.MatchSnapshotResponse) {
	subscribers := c.subs

	c.seqNum++
	snapshot.SeqNum = c.seqNum

	// Strip replay frames from periodic broadcasts to reduce bandwidth.
	// Replay frames are still sent on initial Subscribe and via ApplyIntent
	// so clients can resync on reconnect.
	snapshot.ReplayFrames = nil

	s.publishToRedis(c.state.MatchID, snapshot)

	if len(subscribers) == 0 {
		return
	}

	cachedWhite := snapshot
	cachedWhite.Match = filterStateForColor(snapshot.Match, "white")
	cachedWhite.Events = filterEventsForColor(snapshot.Events, "white")
	cachedBlack := snapshot
	cachedBlack.Match = filterStateForColor(snapshot.Match, "black")
	cachedBlack.Events = filterEventsForColor(snapshot.Events, "black")
	cachedSpec := snapshot
	cachedSpec.Match = filterStateForColor(snapshot.Match, "")
	cachedSpec.Events = filterEventsForColor(snapshot.Events, "")

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
		if r := recover(); r != nil {
			log.Printf("pushSnapshot: recovered panic for seq=%d: %v", snapshot.SeqNum, r)
		}
	}()
	select {
	case ch <- snapshot:
	default:
		metrics.PushSnapshotDrops.Inc()
		log.Printf("pushSnapshot: dropping event seq=%d for channel %p (buffer full) — forcing client resync", snapshot.SeqNum, ch)
		close(ch)
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
			s.collectAndBroadcast(now.UTC())
		}
	}
}

const broadcastConcurrency = 20

func (s *Service) startGC() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case now, ok := <-ticker.C:
			if !ok {
				return
			}
			s.gcFinishedMatches(now.UTC())
		}
	}
}

func (s *Service) collectAndBroadcast(now time.Time) {
	sem := make(chan struct{}, broadcastConcurrency)
	var wg sync.WaitGroup

	s.matches.Range(func(_ string, c *matchContainer) bool {
		sem <- struct{}{}
		wg.Add(1)
		go func(mc *matchContainer) {
			defer func() {
				<-sem
				wg.Done()
			}()
			s.processMatchBroadcast(mc, now)
		}(c)
		return true
	})

	wg.Wait()
}

func (s *Service) processMatchBroadcast(c *matchContainer, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state.Status == "finished" {
		return
	}

	presence := s.ensurePresenceStateLocked(c, now)

	recentCutoff := now.Add(-presenceHeartbeatTimeout)
	hasRecentActivity := (!presence.WhiteLastSeenAt.IsZero() && presence.WhiteLastSeenAt.After(recentCutoff)) ||
		(!presence.BlackLastSeenAt.IsZero() && presence.BlackLastSeenAt.After(recentCutoff))
	if hasRecentActivity {
		timeoutEvents := syncClockForMutation(c.state, now)
		if len(timeoutEvents) > 0 {
			c.events = append(c.events, timeoutEvents...)
			s.broadcastLocked(c, buildSnapshotWithPresence(c.state, presence, len(c.events), timeoutEvents, now))
		}
		s.persistSnapshot(buildSnapshot(c.state, len(c.events), c.events, now))
		if len(timeoutEvents) > 0 {
			return
		}
	}

	runtimeEvents := evaluatePresenceRuntime(c.state, presence, now)
	if len(runtimeEvents) > 0 {
		c.events = append(c.events, runtimeEvents...)
		s.persistSnapshot(buildSnapshot(c.state, len(c.events), c.events, now))
		s.broadcastLocked(c, buildSnapshotWithPresence(c.state, presence, len(c.events), runtimeEvents, now))
		return
	}
	if len(c.subs) == 0 {
		return
	}
	s.broadcastLocked(c, buildSnapshotWithPresence(c.state, presence, len(c.events), nil, now))
}

func (s *Service) computerWorker() {
	defer s.computerWorkerWg.Done()
	for {
		select {
		case <-s.stopCh:
			return
		case task := <-s.computerCh:
			task.c.mu.Lock()
			s.autoPlayComputerDepthLimited(task.c, task.now, 0)
			task.c.mu.Unlock()
		}
	}
}

func (s *Service) gcFinishedMatches(now time.Time) {
	const finishedMatchTTL = 30 * time.Minute
	const waitingMatchTTL = 30 * time.Minute

	s.matches.Range(func(matchID string, c *matchContainer) bool {
		c.mu.Lock()
		status := c.state.Status
		updatedAt := c.state.UpdatedAt
		c.mu.Unlock()

		if status == "finished" {
			if now.Sub(updatedAt) >= finishedMatchTTL {
				s.matches.Delete(matchID)
			}
		} else if status == "waiting" {
			if now.Sub(updatedAt) >= waitingMatchTTL {
				s.matches.Delete(matchID)
			}
		}
		return true
	})
}


