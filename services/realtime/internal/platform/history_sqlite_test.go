package platform

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/chess404/realtime/internal/contracts"
)

func TestSQLiteMatchArchiveStoreUpsertAndReload(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "match-archive.sqlite")
	store, err := NewSQLiteMatchArchiveStore(storePath)
	if err != nil {
		t.Fatalf("expected sqlite archive store to initialize, got %v", err)
	}
	defer func() { _ = store.Close() }()

	now := time.Date(2026, 5, 6, 18, 0, 0, 0, time.UTC)
	snapshot := contracts.MatchSnapshotResponse{
		Match: contracts.MatchState{
			MatchID:      "sqlite_archive_test",
			RulesVersion: "v1-alpha-foundation",
			Status:       "finished",
			Winner:       "white",
			MoveHistory:  []string{"e4", "e5"},
			CreatedAt:    now,
			UpdatedAt:    now.Add(time.Minute),
		},
		ReplayHead: 2,
	}
	if err := store.Upsert(snapshot); err != nil {
		t.Fatalf("expected sqlite archive upsert to succeed, got %v", err)
	}

	reloaded, err := NewSQLiteMatchArchiveStore(storePath)
	if err != nil {
		t.Fatalf("expected sqlite archive store reload to succeed, got %v", err)
	}
	defer func() { _ = reloaded.Close() }()

	entry, ok := reloaded.Get("sqlite_archive_test")
	if !ok {
		t.Fatalf("expected sqlite archive entry to be reloadable")
	}
	if entry.MatchID != "sqlite_archive_test" || entry.MoveCount != 2 || entry.LastMove != "e5" || entry.Winner != "white" {
		t.Fatalf("unexpected sqlite archive entry %#v", entry)
	}
	if reloaded.Backend() != "sqlite" {
		t.Fatalf("expected sqlite archive backend, got %s", reloaded.Backend())
	}
}

func TestSQLiteMatchArchiveStoreLoadMatchRestoresPrivateState(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "match-archive.sqlite")
	store, err := NewSQLiteMatchArchiveStore(storePath)
	if err != nil {
		t.Fatalf("expected sqlite archive store to initialize, got %v", err)
	}
	defer func() { _ = store.Close() }()

	now := time.Date(2026, 5, 6, 18, 30, 0, 0, time.UTC)
	snapshot := contracts.MatchSnapshotResponse{
		Match: contracts.MatchState{
			MatchID:           "sqlite_private_restore",
			RulesVersion:      "v1-alpha-foundation",
			WhitePlayerSecret: "white-secret",
			BlackPlayerSecret: "black-secret",
			Status:            "active",
			Turn:              "black",
			Board:             make([][]*contracts.Piece, 8),
			MoveHistory:       []string{"e4"},
			CreatedAt:         now,
			UpdatedAt:         now,
			History: []contracts.PositionState{
				{Board: make([][]*contracts.Piece, 8), Turn: "white", MoveHistory: []string{}},
				{Board: make([][]*contracts.Piece, 8), Turn: "black", MoveHistory: []string{"e4"}},
			},
		},
		Events: []contracts.ResolvedEvent{
			{ID: "evt_1", MatchID: "sqlite_private_restore", Type: "match_started", At: now, Payload: map[string]any{"turn": "white"}},
			{ID: "evt_2", MatchID: "sqlite_private_restore", Type: "move_applied", At: now, Payload: map[string]any{"notation": "e4"}},
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
		t.Fatalf("expected sqlite archive upsert to succeed, got %v", err)
	}

	reloaded, err := NewSQLiteMatchArchiveStore(storePath)
	if err != nil {
		t.Fatalf("expected sqlite archive store reload to succeed, got %v", err)
	}
	defer func() { _ = reloaded.Close() }()

	match, events, ok := reloaded.LoadMatch("sqlite_private_restore")
	if !ok {
		t.Fatalf("expected sqlite private restore entry to be loadable")
	}
	if match.WhitePlayerSecret != "white-secret" || match.BlackPlayerSecret != "black-secret" {
		t.Fatalf("expected sqlite private secrets to persist, got %#v", match)
	}
	if len(match.History) != 2 || len(events) != 2 {
		t.Fatalf("expected sqlite private history and events to persist, got history=%d events=%d", len(match.History), len(events))
	}
}
