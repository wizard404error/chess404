package platform

import (
	"database/sql"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chess404/realtime/internal/contracts"
)

type MatchArchiveEntry struct {
	MatchID            string                          `json:"matchId"`
	Status             string                          `json:"status"`
	Winner             string                          `json:"winner,omitempty"`
	FinishReason       string                          `json:"finishReason,omitempty"`
	RulesVersion       string                          `json:"rulesVersion"`
	Queue              string                          `json:"queue,omitempty"`
	ModeID             contracts.MatchModeID           `json:"modeId,omitempty"`
	WhiteGuestID       string                          `json:"whiteGuestId,omitempty"`
	BlackGuestID       string                          `json:"blackGuestId,omitempty"`
	WhiteAccountID     string                          `json:"whiteAccountId,omitempty"`
	BlackAccountID     string                          `json:"blackAccountId,omitempty"`
	WhiteAccountHandle string                          `json:"whiteAccountHandle,omitempty"`
	BlackAccountHandle string                          `json:"blackAccountHandle,omitempty"`
	WhiteName          string                          `json:"whiteName,omitempty"`
	BlackName          string                          `json:"blackName,omitempty"`
	CreatedAt          time.Time                       `json:"createdAt"`
	UpdatedAt          time.Time                       `json:"updatedAt"`
	MoveCount          int                             `json:"moveCount"`
	LastMove           string                          `json:"lastMove,omitempty"`
	Snapshot           contracts.MatchSnapshotResponse `json:"snapshot"`
}

type MatchArchivePrivateEntry struct {
	WhitePlayerSecret string                    `json:"whitePlayerSecret,omitempty"`
	BlackPlayerSecret string                    `json:"blackPlayerSecret,omitempty"`
	History           []contracts.PositionState `json:"history,omitempty"`
}

type matchArchiveFile struct {
	Entries map[string]MatchArchiveEntry        `json:"entries"`
	Private map[string]MatchArchivePrivateEntry `json:"private,omitempty"`
}

type MatchArchiveStore struct {
	mu          sync.Mutex
	store       archivePersistence
	entries     map[string]MatchArchiveEntry
	private     map[string]MatchArchivePrivateEntry
	dirty       map[string]struct{}
	writeCh     chan struct{}
	closeCh     chan struct{}
	closed      bool
	persistMu   sync.Mutex
	useQueries  bool // when true, read ops query the DB instead of scanning in-memory maps
}

type MatchArchiveStats struct {
	TotalMatches    int `json:"totalMatches"`
	ActiveMatches   int `json:"activeMatches"`
	FinishedMatches int `json:"finishedMatches"`
	RatedMatches    int `json:"ratedMatches"`
	CasualMatches   int `json:"casualMatches"`
	DirectMatches   int `json:"directMatches"`
}

func NewMatchArchiveStore(path string) (*MatchArchiveStore, error) {
	return newMatchArchiveStore(newFileArchiveStore(path))
}

func NewSQLiteMatchArchiveStore(path string) (*MatchArchiveStore, error) {
	store, err := newSQLiteArchiveStore(path)
	if err != nil {
		return nil, err
	}
	return newMatchArchiveStore(store)
}

func NewPostgresMatchArchiveStore(dsn string) (*MatchArchiveStore, error) {
	store, err := newPostgresArchiveStore(dsn)
	if err != nil {
		return nil, err
	}
	return newMatchArchiveStore(store)
}

func NewPostgresMatchArchiveStoreWithDB(db *sql.DB) (*MatchArchiveStore, error) {
	store, err := NewPostgresArchiveStoreWithDB(db)
	if err != nil {
		return nil, err
	}
	return newMatchArchiveStore(store)
}

func newMatchArchiveStore(persistence archivePersistence) (*MatchArchiveStore, error) {
	store := &MatchArchiveStore{
		store:   persistence,
		entries: make(map[string]MatchArchiveEntry),
		private: make(map[string]MatchArchivePrivateEntry),
		dirty:   make(map[string]struct{}),
		writeCh: make(chan struct{}, 64),
		closeCh: make(chan struct{}),
	}
	// Postgres uses lazy-loaded DB queries; file/SQLite load everything.
	if persistence != nil && persistence.backend() == "postgres" {
		store.useQueries = true
	}
	if err := store.load(); err != nil {
		if store.store != nil {
			_ = store.store.close()
		}
		return nil, err
	}
	go store.writeLoop()
	return store, nil
}

func (s *MatchArchiveStore) writeLoop() {
	for {
		select {
		case <-s.writeCh:
		drainLoop:
			for {
				select {
				case <-s.writeCh:
				default:
					break drainLoop
				}
			}
			s.persistMu.Lock()
			s.mu.Lock()
			if s.closed {
				s.mu.Unlock()
				s.persistMu.Unlock()
				return
			}
			_ = s.persistLocked()
			s.mu.Unlock()
			s.persistMu.Unlock()
		case <-s.closeCh:
			s.persistMu.Lock()
			s.mu.Lock()
			_ = s.persistLocked()
			s.mu.Unlock()
			s.persistMu.Unlock()
			return
		}
	}
}

func (s *MatchArchiveStore) Backend() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store == nil {
		return "memory"
	}
	return s.store.backend()
}

func (s *MatchArchiveStore) Flush() error {
	s.persistMu.Lock()
	defer s.persistMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	// Flush always persists ALL entries (used for forced/manual saves).
	if s.store == nil {
		return nil
	}
	if err := s.store.persist(s.entries, s.private); err != nil {
		return err
	}
	s.dirty = make(map[string]struct{})
	return nil
}

func (s *MatchArchiveStore) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	close(s.closeCh)
	s.mu.Unlock()

	s.persistMu.Lock()
	s.mu.Lock()
	// On close, persist ALL entries (not just dirty) to ensure nothing is lost.
	if s.store != nil {
		_ = s.store.persist(s.entries, s.private)
		s.store.close()
	}
	s.store = nil
	s.entries = nil
	s.private = nil
	s.dirty = nil
	s.mu.Unlock()
	s.persistMu.Unlock()
	return nil
}

func (s *MatchArchiveStore) Upsert(snapshot contracts.MatchSnapshotResponse) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	match := snapshot.Match
	entry := MatchArchiveEntry{
		MatchID:        match.MatchID,
		Status:         match.Status,
		Winner:         match.Winner,
		FinishReason:   match.FinishReason,
		RulesVersion:   match.RulesVersion,
		Queue:          match.Queue,
		ModeID:         match.ModeID,
		WhiteGuestID:   match.WhiteGuestID,
		BlackGuestID:   match.BlackGuestID,
		WhiteAccountID: match.WhiteAccountID,
		BlackAccountID: match.BlackAccountID,
		WhiteName:      match.WhiteName,
		BlackName:      match.BlackName,
		CreatedAt:      match.CreatedAt,
		UpdatedAt:      match.UpdatedAt,
		MoveCount:      len(match.MoveHistory),
		Snapshot:       cloneSnapshot(snapshot),
	}
	if len(match.MoveHistory) > 0 {
		entry.LastMove = match.MoveHistory[len(match.MoveHistory)-1]
	}
	s.entries[match.MatchID] = entry
	s.private[match.MatchID] = MatchArchivePrivateEntry{
		WhitePlayerSecret: match.WhitePlayerSecret,
		BlackPlayerSecret: match.BlackPlayerSecret,
		History:           clonePositionHistory(match.History),
	}
	s.dirty[match.MatchID] = struct{}{}
	select {
	case s.writeCh <- struct{}{}:
	default:
	}
	return nil
}

func (s *MatchArchiveStore) Get(matchID string) (MatchArchiveEntry, bool) {
	s.mu.Lock()
	entry, ok := s.entries[matchID]
	if ok {
		s.mu.Unlock()
		return cloneArchiveEntry(entry), true
	}
	if s.useQueries && s.store != nil {
		queried, found, err := s.store.queryGet(matchID)
		if err == nil && found {
			s.entries[matchID] = queried
			s.mu.Unlock()
			return cloneArchiveEntry(queried), true
		}
	}
	s.mu.Unlock()
	return MatchArchiveEntry{}, false
}

func (s *MatchArchiveStore) LoadMatch(matchID string) (contracts.MatchState, []contracts.ResolvedEvent, bool) {
	s.mu.Lock()
	entry, ok := s.entries[matchID]
	if !ok && s.useQueries && s.store != nil {
		queried, found, err := s.store.queryGet(matchID)
		if err == nil && found {
			privateQ, _, _ := s.store.queryPrivate(matchID)
			s.entries[matchID] = queried
			s.private[matchID] = privateQ
			entry = queried
			ok = true
		}
	}
	if !ok {
		s.mu.Unlock()
		return contracts.MatchState{}, nil, false
	}

	// Ensure private data is cached when using lazy-load queries.
	privateEntry, privateExists := s.private[matchID]
	if !privateExists && s.useQueries && s.store != nil {
		queriedPrivate, _, _ := s.store.queryPrivate(matchID)
		s.private[matchID] = queriedPrivate
		privateEntry = queriedPrivate
	}

	snapshot := cloneSnapshot(entry.Snapshot)
	s.mu.Unlock()

	snapshot.Match.WhitePlayerSecret = privateEntry.WhitePlayerSecret
	snapshot.Match.BlackPlayerSecret = privateEntry.BlackPlayerSecret
	snapshot.Match.History = clonePositionHistory(privateEntry.History)

	return snapshot.Match, cloneEvents(snapshot.Events), true
}

func (s *MatchArchiveStore) Stats() MatchArchiveStats {
	if s.useQueries && s.store != nil {
		stats, err := s.store.queryStats()
		if err == nil {
			return stats
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	stats := MatchArchiveStats{
		TotalMatches: len(s.entries),
	}
	for _, entry := range s.entries {
		switch entry.Status {
		case "finished":
			stats.FinishedMatches++
		default:
			stats.ActiveMatches++
		}
		switch entry.Queue {
		case "rated":
			stats.RatedMatches++
		case "casual":
			stats.CasualMatches++
		default:
			stats.DirectMatches++
		}
	}
	return stats
}

func (s *MatchArchiveStore) List(limit int) []MatchArchiveEntry {
	if s.useQueries && s.store != nil {
		items, err := s.store.queryList(limit, 0)
		if err == nil {
			for i := range items {
				s.mu.Lock()
				s.entries[items[i].MatchID] = items[i]
				s.mu.Unlock()
			}
			return items
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	items := sortEntriesByUpdatedAt(s.entries)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	for i := range items {
		items[i] = cloneArchiveEntry(items[i])
	}
	return items
}

func (s *MatchArchiveStore) ListUnfinishedMatchIDs(limit int) []string {
	if s.useQueries && s.store != nil {
		ids, err := s.store.queryUnfinishedIDs(limit)
		if err == nil {
			return ids
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	ids := make([]string, 0, len(s.entries))
	for _, entry := range s.entries {
		if strings.EqualFold(strings.TrimSpace(entry.Status), "finished") {
			continue
		}
		ids = append(ids, entry.MatchID)
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := s.entries[ids[i]], s.entries[ids[j]]
		return a.UpdatedAt.After(b.UpdatedAt)
	})
	if limit > 0 && len(ids) > limit {
		ids = ids[:limit]
	}
	return ids
}

func (s *MatchArchiveStore) ListFinishedMatchIDs(limit int) []string {
	if s.useQueries && s.store != nil {
		ids, err := s.store.queryFinishedIDs(limit)
		if err == nil {
			return ids
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	ids := make([]string, 0, len(s.entries))
	for _, entry := range s.entries {
		if !strings.EqualFold(strings.TrimSpace(entry.Status), "finished") {
			continue
		}
		ids = append(ids, entry.MatchID)
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := s.entries[ids[i]], s.entries[ids[j]]
		return a.UpdatedAt.After(b.UpdatedAt)
	})
	if limit > 0 && len(ids) > limit {
		ids = ids[:limit]
	}
	return ids
}

func (s *MatchArchiveStore) ListByGuest(guestID string, limit int) []MatchArchiveEntry {
	if s.useQueries && s.store != nil {
		items, err := s.store.queryByGuest(guestID, limit, 0)
		if err == nil {
			return items
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	items := make([]MatchArchiveEntry, 0, len(s.entries))
	for _, entry := range s.entries {
		if entry.WhiteGuestID == guestID || entry.BlackGuestID == guestID {
			items = append(items, cloneArchiveEntry(entry))
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func (s *MatchArchiveStore) ListByAccount(accountID string, linkedGuestIDs []string, limit int) []MatchArchiveEntry {
	if strings.TrimSpace(accountID) == "" {
		return nil
	}

	if s.useQueries && s.store != nil {
		items, err := s.store.queryByAccount(accountID, linkedGuestIDs, limit, 0)
		if err == nil {
			return items
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	guestIDs := make(map[string]struct{}, len(linkedGuestIDs))
	for _, guestID := range linkedGuestIDs {
		if guestID == "" {
			continue
		}
		guestIDs[guestID] = struct{}{}
	}

	items := make([]MatchArchiveEntry, 0, len(s.entries))
	for _, entry := range s.entries {
		if entry.WhiteAccountID == accountID || entry.BlackAccountID == accountID {
			items = append(items, cloneArchiveEntry(entry))
			continue
		}
		if _, ok := guestIDs[entry.WhiteGuestID]; ok {
			items = append(items, cloneArchiveEntry(entry))
			continue
		}
		if _, ok := guestIDs[entry.BlackGuestID]; ok {
			items = append(items, cloneArchiveEntry(entry))
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func sortEntriesByUpdatedAt(source map[string]MatchArchiveEntry) []MatchArchiveEntry {
	items := make([]MatchArchiveEntry, 0, len(source))
	for _, entry := range source {
		items = append(items, entry)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return items
}

func ParseListLimit(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	if parsed > 100 {
		return 100
	}
	return parsed
}

func (s *MatchArchiveStore) load() error {
	if s.store == nil {
		return nil
	}
	entries, private, err := s.store.load()
	if err != nil {
		return err
	}
	if entries == nil {
		entries = make(map[string]MatchArchiveEntry)
	}
	s.entries = entries
	if private == nil {
		private = make(map[string]MatchArchivePrivateEntry)
	}
	s.private = private
	return nil
}

func (s *MatchArchiveStore) persistLocked() error {
	if s.store == nil {
		return nil
	}
	if len(s.dirty) == 0 {
		return nil
	}
	dirtyEntries := make(map[string]MatchArchiveEntry, len(s.dirty))
	dirtyPrivate := make(map[string]MatchArchivePrivateEntry, len(s.dirty))
	for matchID := range s.dirty {
		if entry, ok := s.entries[matchID]; ok {
			dirtyEntries[matchID] = entry
		}
		if privateEntry, ok := s.private[matchID]; ok {
			dirtyPrivate[matchID] = privateEntry
		}
	}
	if err := s.store.persist(dirtyEntries, dirtyPrivate); err != nil {
		return err
	}
	s.dirty = make(map[string]struct{})
	return nil
}

func cloneSnapshot(snapshot contracts.MatchSnapshotResponse) contracts.MatchSnapshotResponse {
	return contracts.MatchSnapshotResponse{
		Match:        cloneMatchState(snapshot.Match),
		ReplayHead:   snapshot.ReplayHead,
		ReplayFrames: cloneReplayFrames(snapshot.ReplayFrames),
		Events:       cloneEvents(snapshot.Events),
	}
}

func cloneMatchState(state contracts.MatchState) contracts.MatchState {
	clone := state
	clone.Board = cloneBoard(state.Board)
	clone.Moved = append([]string{}, state.Moved...)
	clone.MoveHistory = append([]string{}, state.MoveHistory...)
	clone.ChatMessages = append([]contracts.ChatMessage{}, state.ChatMessages...)
	clone.WhiteHand = append([]contracts.GameCard{}, state.WhiteHand...)
	clone.BlackHand = append([]contracts.GameCard{}, state.BlackHand...)
	clone.LavaSquares = append([]contracts.LavaSquare{}, state.LavaSquares...)
	clone.BombPieces = append([]contracts.BombPiece{}, state.BombPieces...)
	clone.BlackHoles = append([]contracts.BlackHoleZone{}, state.BlackHoles...)
	clone.FogZones = append([]contracts.FogZone{}, state.FogZones...)
	clone.FortressZones = append([]contracts.FortressZone{}, state.FortressZones...)
	clone.History = clonePositionHistory(state.History)

	if state.InvisiblePiece != nil {
		invisible := *state.InvisiblePiece
		clone.InvisiblePiece = &invisible
	}
	if state.CheaterState != nil {
		cheater := *state.CheaterState
		clone.CheaterState = &cheater
	}
	if state.DoubleMove != nil {
		doubleMove := *state.DoubleMove
		if state.DoubleMove.TrackedSq != nil {
			tracked := *state.DoubleMove.TrackedSq
			doubleMove.TrackedSq = &tracked
		}
		clone.DoubleMove = &doubleMove
	}
	if state.LastMove != nil {
		lastMove := *state.LastMove
		clone.LastMove = &lastMove
	}
	if state.PendingCard != nil {
		pending := *state.PendingCard
		clone.PendingCard = &pending
	}

	return clone
}

func clonePositionHistory(history []contracts.PositionState) []contracts.PositionState {
	if len(history) == 0 {
		return nil
	}
	cloned := make([]contracts.PositionState, 0, len(history))
	for _, position := range history {
		next := position
		next.Board = cloneBoard(position.Board)
		next.LavaSquares = append([]contracts.LavaSquare{}, position.LavaSquares...)
		next.BombPieces = append([]contracts.BombPiece{}, position.BombPieces...)
		next.BlackHoles = append([]contracts.BlackHoleZone{}, position.BlackHoles...)
		next.FogZones = append([]contracts.FogZone{}, position.FogZones...)
		next.FortressZones = append([]contracts.FortressZone{}, position.FortressZones...)
		next.Moved = append([]string{}, position.Moved...)
		next.MoveHistory = append([]string{}, position.MoveHistory...)
		if position.InvisiblePiece != nil {
			invisible := *position.InvisiblePiece
			next.InvisiblePiece = &invisible
		}
		if position.CheaterState != nil {
			cheater := *position.CheaterState
			next.CheaterState = &cheater
		}
		if position.LastMove != nil {
			lastMove := *position.LastMove
			next.LastMove = &lastMove
		}
		cloned = append(cloned, next)
	}
	return cloned
}

func cloneReplayFrames(frames []contracts.ReplayFrame) []contracts.ReplayFrame {
	if len(frames) == 0 {
		return nil
	}
	cloned := make([]contracts.ReplayFrame, 0, len(frames))
	for _, frame := range frames {
		next := frame
		next.Board = cloneBoard(frame.Board)
		next.MoveHistory = append([]string{}, frame.MoveHistory...)
		if frame.LastMove != nil {
			lastMove := *frame.LastMove
			next.LastMove = &lastMove
		}
		cloned = append(cloned, next)
	}
	return cloned
}

func cloneEvents(events []contracts.ResolvedEvent) []contracts.ResolvedEvent {
	if len(events) == 0 {
		return nil
	}
	cloned := make([]contracts.ResolvedEvent, 0, len(events))
	for _, event := range events {
		next := event
		next.Payload = clonePayload(event.Payload)
		cloned = append(cloned, next)
	}
	return cloned
}

func clonePayload(payload map[string]any) map[string]any {
	if len(payload) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(payload))
	for key, value := range payload {
		cloned[key] = deepCloneAny(value)
	}
	return cloned
}

func deepCloneAny(value any) any {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case map[string]any:
		return clonePayload(v)
	case []any:
		if v == nil {
			return nil
		}
		cloned := make([]any, len(v))
		for i, item := range v {
			cloned[i] = deepCloneAny(item)
		}
		return cloned
	default:
		return value
	}
}

func cloneArchiveEntry(entry MatchArchiveEntry) MatchArchiveEntry {
	entry.Snapshot = cloneSnapshot(entry.Snapshot)
	return entry
}

func cloneBoard(board [][]*contracts.Piece) [][]*contracts.Piece {
	if len(board) == 0 {
		return nil
	}
	cloned := make([][]*contracts.Piece, len(board))
	for row := range board {
		cloned[row] = make([]*contracts.Piece, len(board[row]))
		for col := range board[row] {
			if board[row][col] == nil {
				continue
			}
			piece := *board[row][col]
			if piece.ShieldTurn != nil {
				val := *piece.ShieldTurn
				piece.ShieldTurn = &val
			}
			if piece.InvisibleTurn != nil {
				val := *piece.InvisibleTurn
				piece.InvisibleTurn = &val
			}
			cloned[row][col] = &piece
		}
	}
	return cloned
}
