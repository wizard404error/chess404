package platform

import (
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/chess404/realtime/internal/contracts"
)

const postgresArchiveInitSQL = `
		create table if not exists archives (
			match_id text primary key,
			status text not null,
			queue text,
			white_guest_id text,
			black_guest_id text,
			updated_at timestamptz not null,
			entry_json jsonb not null,
			private_json jsonb
		);
		create index if not exists archives_updated_at_idx on archives (updated_at desc);
		create index if not exists archives_queue_idx on archives (queue);
		create index if not exists archives_status_idx on archives (status);
		create index if not exists archives_white_guest_idx on archives (white_guest_id);
		create index if not exists archives_black_guest_idx on archives (black_guest_id);
	`

func TestPostgresArchiveStoreLoadRestoresPrivateState(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("expected sqlmock database, got %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta(postgresArchiveInitSQL)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	now := time.Date(2026, 5, 6, 20, 0, 0, 0, time.UTC)
	entry := MatchArchiveEntry{
		MatchID:      "pg_archive_test",
		Status:       "finished",
		Winner:       "white",
		RulesVersion: "v1-alpha-foundation",
		UpdatedAt:    now,
		MoveCount:    2,
		LastMove:     "e5",
		Snapshot: contracts.MatchSnapshotResponse{
			Match: contracts.MatchState{
				MatchID:      "pg_archive_test",
				RulesVersion: "v1-alpha-foundation",
				Status:       "finished",
				Winner:       "white",
				MoveHistory:  []string{"e4", "e5"},
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			Events: []contracts.ResolvedEvent{
				{ID: "evt_1", MatchID: "pg_archive_test", Type: "move_applied", At: now, Payload: map[string]any{"notation": "e4"}},
			},
		},
	}
	privateEntry := MatchArchivePrivateEntry{
		WhitePlayerSecret: "white-secret",
		BlackPlayerSecret: "black-secret",
		History: []contracts.PositionState{
			{Board: make([][]*contracts.Piece, 8), Turn: "white"},
			{Board: make([][]*contracts.Piece, 8), Turn: "black", MoveHistory: []string{"e4"}},
		},
	}
	for i := range privateEntry.History {
		for row := range privateEntry.History[i].Board {
			privateEntry.History[i].Board[row] = make([]*contracts.Piece, 8)
		}
	}
	entryJSON, _ := json.Marshal(entry)
	privateJSON, _ := json.Marshal(privateEntry)

	mock.ExpectQuery(`select match_id, entry_json, private_json from archives`).
		WillReturnRows(sqlmock.NewRows([]string{"match_id", "entry_json", "private_json"}).
			AddRow("pg_archive_test", entryJSON, privateJSON))

	archiveStore, err := newPostgresArchiveStoreWithDB(db)
	if err != nil {
		t.Fatalf("expected postgres archive store to initialize, got %v", err)
	}
	matchStore, err := newMatchArchiveStore(archiveStore)
	if err != nil {
		t.Fatalf("expected wrapped archive store to initialize, got %v", err)
	}
	defer func() { _ = matchStore.Close() }()

	entryOut, ok := matchStore.Get("pg_archive_test")
	if !ok || entryOut.LastMove != "e5" {
		t.Fatalf("expected postgres archive entry to load, got %#v ok=%v", entryOut, ok)
	}
	match, events, ok := matchStore.LoadMatch("pg_archive_test")
	if !ok {
		t.Fatalf("expected postgres archive match to load")
	}
	if match.WhitePlayerSecret != "white-secret" || match.BlackPlayerSecret != "black-secret" {
		t.Fatalf("expected postgres private secrets to persist, got %#v", match)
	}
	if len(match.History) != 2 || len(events) != 1 {
		t.Fatalf("expected postgres history/events to persist, got history=%d events=%d", len(match.History), len(events))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet postgres archive expectations: %v", err)
	}
}

func TestPostgresArchiveStoreUpsertPersistsEntry(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("expected sqlmock database, got %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta(postgresArchiveInitSQL)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`select match_id, entry_json, private_json from archives`).
		WillReturnRows(sqlmock.NewRows([]string{"match_id", "entry_json", "private_json"}))

	archiveStore, err := newPostgresArchiveStoreWithDB(db)
	if err != nil {
		t.Fatalf("expected postgres archive store to initialize, got %v", err)
	}
	matchStore, err := newMatchArchiveStore(archiveStore)
	if err != nil {
		t.Fatalf("expected wrapped archive store to initialize, got %v", err)
	}
	defer func() { _ = matchStore.Close() }()

	now := time.Date(2026, 5, 6, 20, 30, 0, 0, time.UTC)
	snapshot := contracts.MatchSnapshotResponse{
		Match: contracts.MatchState{
			MatchID:           "pg_persist_test",
			RulesVersion:      "v1-alpha-foundation",
			Status:            "active",
			Queue:             "rated",
			WhiteGuestID:      "guest_white",
			BlackGuestID:      "guest_black",
			WhitePlayerSecret: "white-secret",
			BlackPlayerSecret: "black-secret",
			MoveHistory:       []string{"e4"},
			CreatedAt:         now,
			UpdatedAt:         now,
			History: []contracts.PositionState{
				{Board: make([][]*contracts.Piece, 8), Turn: "white"},
			},
		},
	}
	for i := range snapshot.Match.History {
		for row := range snapshot.Match.History[i].Board {
			snapshot.Match.History[i].Board[row] = make([]*contracts.Piece, 8)
		}
	}

	mock.ExpectBegin()
	mock.ExpectExec(`delete from archives`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`insert into archives\(`).
		WithArgs(
			"pg_persist_test",
			"active",
			"rated",
			"guest_white",
			"guest_black",
			now,
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := matchStore.Upsert(snapshot); err != nil {
		t.Fatalf("expected postgres archive upsert to succeed, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet postgres archive upsert expectations: %v", err)
	}
}
