package platform

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/chess404/realtime/internal/contracts"
)

func TestMatchArchiveStoreUpsertAndReload(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "match-archive.json")
	store, err := NewMatchArchiveStore(storePath)
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}

	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	snapshot := contracts.MatchSnapshotResponse{
		Match: contracts.MatchState{
			MatchID:      "archive_test",
			RulesVersion: "v1-alpha-foundation",
			Status:       "finished",
			Winner:       "white",
			FinishReason: "checkmate",
			MoveHistory:  []string{"e4", "e5"},
			CreatedAt:    now,
			UpdatedAt:    now.Add(time.Minute),
		},
		ReplayHead: 2,
	}
	if err := store.Upsert(snapshot); err != nil {
		t.Fatalf("expected upsert to succeed, got %v", err)
	}
	if err := store.Flush(); err != nil {
		t.Fatalf("expected archive flush to succeed, got %v", err)
	}

	reloaded, err := NewMatchArchiveStore(storePath)
	if err != nil {
		t.Fatalf("expected archive store reload to succeed, got %v", err)
	}
	entry, ok := reloaded.Get("archive_test")
	if !ok {
		t.Fatalf("expected archive entry to be reloadable")
	}
	if entry.MatchID != "archive_test" || entry.MoveCount != 2 || entry.LastMove != "e5" || entry.Winner != "white" || entry.FinishReason != "checkmate" {
		t.Fatalf("unexpected archive entry %#v", entry)
	}
}

func TestMatchArchiveStorePreservesPlayerMetadata(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "match-archive.json")
	store, err := NewMatchArchiveStore(storePath)
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}

	now := time.Date(2026, 5, 6, 11, 0, 0, 0, time.UTC)
	snapshot := contracts.MatchSnapshotResponse{
		Match: contracts.MatchState{
			MatchID:        "archive_players",
			RulesVersion:   "v1-alpha-foundation",
			Queue:          "rated",
			ModeID:         contracts.MatchModeHiddenCards,
			WhiteGuestID:   "guest_white",
			BlackGuestID:   "guest_black",
			WhiteAccountID: "acct_white",
			BlackAccountID: "acct_black",
			WhiteName:      "Aurora Bishop 101",
			BlackName:      "Velvet Queen 202",
			Status:         "active",
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	}
	if err := store.Upsert(snapshot); err != nil {
		t.Fatalf("expected upsert to succeed, got %v", err)
	}

	entry, ok := store.Get("archive_players")
	if !ok {
		t.Fatalf("expected archive entry lookup to succeed")
	}
	if entry.WhiteGuestID != "guest_white" || entry.BlackGuestID != "guest_black" {
		t.Fatalf("expected guest ids to persist, got %#v", entry)
	}
	if entry.WhiteAccountID != "acct_white" || entry.BlackAccountID != "acct_black" {
		t.Fatalf("expected account ids to persist, got %#v", entry)
	}
	if entry.WhiteName != "Aurora Bishop 101" || entry.BlackName != "Velvet Queen 202" {
		t.Fatalf("expected guest names to persist, got %#v", entry)
	}
	if entry.Queue != "rated" {
		t.Fatalf("expected queue metadata to persist, got %#v", entry)
	}
	if entry.ModeID != contracts.MatchModeHiddenCards {
		t.Fatalf("expected mode metadata to persist, got %#v", entry)
	}
}

func TestMatchArchiveStoreListByGuest(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "match-archive.json")
	store, err := NewMatchArchiveStore(storePath)
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}

	base := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	snapshots := []contracts.MatchSnapshotResponse{
		{
			Match: contracts.MatchState{
				MatchID:      "guest_match_1",
				RulesVersion: "v1-alpha-foundation",
				WhiteGuestID: "guest_focus",
				BlackGuestID: "guest_other",
				CreatedAt:    base,
				UpdatedAt:    base.Add(2 * time.Minute),
			},
		},
		{
			Match: contracts.MatchState{
				MatchID:      "guest_match_2",
				RulesVersion: "v1-alpha-foundation",
				WhiteGuestID: "guest_else",
				BlackGuestID: "guest_focus",
				CreatedAt:    base,
				UpdatedAt:    base.Add(5 * time.Minute),
			},
		},
		{
			Match: contracts.MatchState{
				MatchID:      "guest_match_3",
				RulesVersion: "v1-alpha-foundation",
				WhiteGuestID: "guest_else",
				BlackGuestID: "guest_other",
				CreatedAt:    base,
				UpdatedAt:    base.Add(8 * time.Minute),
			},
		},
	}

	for _, snapshot := range snapshots {
		if err := store.Upsert(snapshot); err != nil {
			t.Fatalf("expected upsert to succeed, got %v", err)
		}
	}

	matches := store.ListByGuest("guest_focus", 10)
	if len(matches) != 2 {
		t.Fatalf("expected 2 guest matches, got %d", len(matches))
	}
	if matches[0].MatchID != "guest_match_2" || matches[1].MatchID != "guest_match_1" {
		t.Fatalf("expected guest matches sorted by recency, got %#v", matches)
	}
}

func TestMatchArchiveStoreListByAccountIncludesLinkedGuestFallback(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "match-archive.json")
	store, err := NewMatchArchiveStore(storePath)
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}

	base := time.Date(2026, 5, 6, 12, 30, 0, 0, time.UTC)
	snapshots := []contracts.MatchSnapshotResponse{
		{
			Match: contracts.MatchState{
				MatchID:        "account_match_direct",
				RulesVersion:   "v1-alpha-foundation",
				WhiteGuestID:   "guest_focus",
				BlackGuestID:   "guest_other",
				WhiteAccountID: "acct_focus",
				CreatedAt:      base,
				UpdatedAt:      base.Add(2 * time.Minute),
			},
		},
		{
			Match: contracts.MatchState{
				MatchID:      "account_match_legacy",
				RulesVersion: "v1-alpha-foundation",
				WhiteGuestID: "guest_else",
				BlackGuestID: "guest_focus",
				CreatedAt:    base,
				UpdatedAt:    base.Add(5 * time.Minute),
			},
		},
		{
			Match: contracts.MatchState{
				MatchID:      "account_match_other",
				RulesVersion: "v1-alpha-foundation",
				WhiteGuestID: "guest_else",
				BlackGuestID: "guest_other",
				CreatedAt:    base,
				UpdatedAt:    base.Add(8 * time.Minute),
			},
		},
	}

	for _, snapshot := range snapshots {
		if err := store.Upsert(snapshot); err != nil {
			t.Fatalf("expected upsert to succeed, got %v", err)
		}
	}

	matches := store.ListByAccount("acct_focus", []string{"guest_focus"}, 10)
	if len(matches) != 2 {
		t.Fatalf("expected 2 account matches, got %d", len(matches))
	}
	if matches[0].MatchID != "account_match_legacy" || matches[1].MatchID != "account_match_direct" {
		t.Fatalf("expected account matches sorted by recency, got %#v", matches)
	}
}

func TestMatchArchiveStorePreservesReplayFrames(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "match-archive.json")
	store, err := NewMatchArchiveStore(storePath)
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}

	now := time.Date(2026, 5, 6, 13, 0, 0, 0, time.UTC)
	snapshot := contracts.MatchSnapshotResponse{
		Match: contracts.MatchState{
			MatchID:      "replay_archive",
			RulesVersion: "v1-alpha-foundation",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		ReplayHead: 1,
		ReplayFrames: []contracts.ReplayFrame{
			{Index: 0, Turn: "white", Board: make([][]*contracts.Piece, 8)},
			{Index: 1, Turn: "black", Board: make([][]*contracts.Piece, 8), MoveHistory: []string{"e4"}},
		},
	}

	if err := store.Upsert(snapshot); err != nil {
		t.Fatalf("expected upsert to succeed, got %v", err)
	}
	if err := store.Flush(); err != nil {
		t.Fatalf("expected archive flush to succeed, got %v", err)
	}

	reloaded, err := NewMatchArchiveStore(storePath)
	if err != nil {
		t.Fatalf("expected archive store reload to succeed, got %v", err)
	}
	entry, ok := reloaded.Get("replay_archive")
	if !ok {
		t.Fatalf("expected replay archive entry to exist after reload")
	}
	if len(entry.Snapshot.ReplayFrames) != 2 || entry.Snapshot.ReplayFrames[1].Turn != "black" {
		t.Fatalf("expected replay frames to persist, got %#v", entry.Snapshot.ReplayFrames)
	}
}

func TestMatchArchiveStoreLoadMatchRestoresPrivateState(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "match-archive.json")
	store, err := NewMatchArchiveStore(storePath)
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}

	now := time.Date(2026, 5, 6, 14, 0, 0, 0, time.UTC)
	snapshot := contracts.MatchSnapshotResponse{
		Match: contracts.MatchState{
			MatchID:           "private_restore",
			RulesVersion:      "v1-alpha-foundation",
			WhitePlayerSecret: "white-secret",
			BlackPlayerSecret: "black-secret",
			Status:            "active",
			Turn:              "black",
			Board:             make([][]*contracts.Piece, 8),
			Moved:             []string{"1-4"},
			MoveHistory:       []string{"e4"},
			CreatedAt:         now,
			UpdatedAt:         now,
			History: []contracts.PositionState{
				{Board: make([][]*contracts.Piece, 8), Turn: "white", MoveHistory: []string{}},
				{Board: make([][]*contracts.Piece, 8), Turn: "black", MoveHistory: []string{"e4"}},
			},
		},
		ReplayHead: 2,
		Events: []contracts.ResolvedEvent{
			{ID: "evt_1", MatchID: "private_restore", Type: "match_started", At: now, Payload: map[string]any{"turn": "white"}},
			{ID: "evt_2", MatchID: "private_restore", Type: "move_applied", At: now, Payload: map[string]any{"notation": "e4"}},
		},
	}
	for i := range snapshot.Match.Board {
		snapshot.Match.Board[i] = make([]*contracts.Piece, 8)
	}
	for i := range snapshot.Match.History {
		for row := range snapshot.Match.History[i].Board {
			snapshot.Match.History[i].Board[row] = make([]*contracts.Piece, 8)
		}
	}

	if err := store.Upsert(snapshot); err != nil {
		t.Fatalf("expected archive upsert to succeed, got %v", err)
	}
	if err := store.Flush(); err != nil {
		t.Fatalf("expected archive flush to succeed, got %v", err)
	}

	reloaded, err := NewMatchArchiveStore(storePath)
	if err != nil {
		t.Fatalf("expected archive store reload to succeed, got %v", err)
	}
	match, events, ok := reloaded.LoadMatch("private_restore")
	if !ok {
		t.Fatalf("expected private restore entry to be loadable")
	}
	if match.WhitePlayerSecret != "white-secret" || match.BlackPlayerSecret != "black-secret" {
		t.Fatalf("expected seat secrets to persist privately, got %#v", match)
	}
	if len(match.History) != 2 || len(events) != 2 {
		t.Fatalf("expected private history and events to persist, got history=%d events=%d", len(match.History), len(events))
	}
}

func TestMatchArchiveStoreStatsReflectQueuesAndStatuses(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "match-archive.json")
	store, err := NewMatchArchiveStore(storePath)
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}

	now := time.Date(2026, 5, 6, 15, 0, 0, 0, time.UTC)
	snapshots := []contracts.MatchSnapshotResponse{
		{Match: contracts.MatchState{MatchID: "rated_finished", RulesVersion: "v1", Queue: "rated", Status: "finished", CreatedAt: now, UpdatedAt: now}},
		{Match: contracts.MatchState{MatchID: "casual_active", RulesVersion: "v1", Queue: "casual", Status: "active", CreatedAt: now, UpdatedAt: now}},
		{Match: contracts.MatchState{MatchID: "direct_active", RulesVersion: "v1", Status: "active", CreatedAt: now, UpdatedAt: now}},
	}
	for _, snapshot := range snapshots {
		if err := store.Upsert(snapshot); err != nil {
			t.Fatalf("expected archive upsert to succeed, got %v", err)
		}
	}

	stats := store.Stats()
	if stats.TotalMatches != 3 || stats.FinishedMatches != 1 || stats.ActiveMatches != 2 {
		t.Fatalf("unexpected archive status counts %#v", stats)
	}
	if stats.RatedMatches != 1 || stats.CasualMatches != 1 || stats.DirectMatches != 1 {
		t.Fatalf("unexpected archive queue counts %#v", stats)
	}
}
