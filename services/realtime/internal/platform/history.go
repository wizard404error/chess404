package platform

import (
	"sort"
	"strconv"
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
	mu      sync.Mutex
	store   archivePersistence
	entries map[string]MatchArchiveEntry
	private map[string]MatchArchivePrivateEntry
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

func newMatchArchiveStore(persistence archivePersistence) (*MatchArchiveStore, error) {
	store := &MatchArchiveStore{
		store:   persistence,
		entries: make(map[string]MatchArchiveEntry),
		private: make(map[string]MatchArchivePrivateEntry),
	}
	if err := store.load(); err != nil {
		if store.store != nil {
			_ = store.store.close()
		}
		return nil, err
	}
	return store, nil
}

func (s *MatchArchiveStore) Backend() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store == nil {
		return "memory"
	}
	return s.store.backend()
}

func (s *MatchArchiveStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store == nil {
		return nil
	}
	return s.store.close()
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
		Snapshot:       snapshot,
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
	return s.persistLocked()
}

func (s *MatchArchiveStore) Get(matchID string) (MatchArchiveEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[matchID]
	return entry, ok
}

func (s *MatchArchiveStore) LoadMatch(matchID string) (contracts.MatchState, []contracts.ResolvedEvent, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[matchID]
	if !ok {
		return contracts.MatchState{}, nil, false
	}

	snapshot := cloneSnapshot(entry.Snapshot)
	privateEntry := s.private[matchID]
	snapshot.Match.WhitePlayerSecret = privateEntry.WhitePlayerSecret
	snapshot.Match.BlackPlayerSecret = privateEntry.BlackPlayerSecret
	snapshot.Match.History = clonePositionHistory(privateEntry.History)

	return snapshot.Match, cloneEvents(snapshot.Events), true
}

func (s *MatchArchiveStore) Stats() MatchArchiveStats {
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
	s.mu.Lock()
	defer s.mu.Unlock()

	items := sortEntriesByUpdatedAt(s.entries)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func (s *MatchArchiveStore) ListByGuest(guestID string, limit int) []MatchArchiveEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	filtered := make(map[string]MatchArchiveEntry)
	for matchID, entry := range s.entries {
		if entry.WhiteGuestID == guestID || entry.BlackGuestID == guestID {
			filtered[matchID] = entry
		}
	}

	items := sortEntriesByUpdatedAt(filtered)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func (s *MatchArchiveStore) ListByAccount(accountID string, linkedGuestIDs []string, limit int) []MatchArchiveEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	guestIDs := make(map[string]struct{}, len(linkedGuestIDs))
	for _, guestID := range linkedGuestIDs {
		if guestID == "" {
			continue
		}
		guestIDs[guestID] = struct{}{}
	}

	filtered := make(map[string]MatchArchiveEntry)
	for matchID, entry := range s.entries {
		if entry.WhiteAccountID == accountID || entry.BlackAccountID == accountID {
			filtered[matchID] = entry
			continue
		}
		if _, ok := guestIDs[entry.WhiteGuestID]; ok {
			filtered[matchID] = entry
			continue
		}
		if _, ok := guestIDs[entry.BlackGuestID]; ok {
			filtered[matchID] = entry
		}
	}

	items := sortEntriesByUpdatedAt(filtered)
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
	return s.store.persist(s.entries, s.private)
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
		cloned[key] = value
	}
	return cloned
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
			cloned[row][col] = &piece
		}
	}
	return cloned
}
