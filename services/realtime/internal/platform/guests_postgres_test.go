package platform

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

const postgresGuestInitSQL = `
		create table if not exists guests (
			guest_id text primary key,
			display_name text not null,
			rating integer not null,
			matches_played integer not null,
			wins integer not null,
			losses integer not null,
			draws integer not null,
			created_at timestamptz not null,
			last_seen_at timestamptz not null,
			session_secret text,
			session_token text,
			session_expires_at timestamptz
		);
		create table if not exists finalized_matches (
			match_id text primary key,
			winner text not null,
			finalized_at timestamptz not null
		);
		create index if not exists guests_rating_order_idx on guests (rating desc, created_at asc, guest_id asc);
		create index if not exists guests_last_seen_order_idx on guests (last_seen_at desc, guest_id asc);
	`

func TestPostgresGuestStoreEnsureGuestTouchesExistingGuest(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("expected sqlmock database, got %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta(postgresGuestInitSQL)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`alter table guests add column if not exists session_secret text`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`alter table guests add column if not exists session_token text`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`alter table guests add column if not exists session_expires_at timestamptz`)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	store, err := newPostgresGuestStoreWithDB(db)
	if err != nil {
		t.Fatalf("expected postgres guest store to initialize, got %v", err)
	}
	defer func() { _ = store.Close() }()

	now := time.Date(2026, 5, 6, 19, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(`select guest_id, display_name, rating, matches_played, wins, losses, draws, created_at, last_seen_at, session_secret, session_token, session_expires_at from guests where guest_id = \$1`).
		WithArgs("guest_existing").
		WillReturnRows(sqlmock.NewRows([]string{
			"guest_id", "display_name", "rating", "matches_played", "wins", "losses", "draws", "created_at", "last_seen_at", "session_secret", "session_token", "session_expires_at",
		}).AddRow("guest_existing", "Aurora Bishop 101", 1200, 0, 0, 0, 0, now, now, "secret_existing", "guesttok_existing", now.Add(6*time.Hour)))
	mock.ExpectExec(`update guests set last_seen_at = \$1, session_secret = \$2, session_token = \$3, session_expires_at = \$4 where guest_id = \$5`).
		WithArgs(sqlmock.AnyArg(), "secret_existing", "guesttok_existing", sqlmock.AnyArg(), "guest_existing").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	guest, err := store.EnsureGuest("guest_existing", "secret_existing")
	if err != nil {
		t.Fatalf("expected postgres guest ensure to succeed, got %v", err)
	}
	if guest.Guest.GuestID != "guest_existing" || guest.Guest.DisplayName != "Aurora Bishop 101" || guest.SessionSecret != "secret_existing" || guest.SessionToken != "guesttok_existing" || guest.ExpiresAt.IsZero() {
		t.Fatalf("unexpected postgres guest %#v", guest)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet postgres guest expectations: %v", err)
	}
}

func TestPostgresGuestStoreFinalizeMatchIsIdempotent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("expected sqlmock database, got %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta(postgresGuestInitSQL)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`alter table guests add column if not exists session_secret text`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`alter table guests add column if not exists session_token text`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`alter table guests add column if not exists session_expires_at timestamptz`)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	store, err := newPostgresGuestStoreWithDB(db)
	if err != nil {
		t.Fatalf("expected postgres guest store to initialize, got %v", err)
	}
	defer func() { _ = store.Close() }()

	now := time.Date(2026, 5, 6, 19, 15, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(`select guest_id, display_name, rating, matches_played, wins, losses, draws, created_at, last_seen_at, session_secret, session_token, session_expires_at from guests where guest_id = \$1`).
		WithArgs("guest_white").
		WillReturnRows(sqlmock.NewRows([]string{
			"guest_id", "display_name", "rating", "matches_played", "wins", "losses", "draws", "created_at", "last_seen_at", "session_secret", "session_token", "session_expires_at",
		}).AddRow("guest_white", "White", 1216, 1, 1, 0, 0, now, now, "secret_white", "guesttok_white", now.Add(6*time.Hour)))
	mock.ExpectQuery(`select guest_id, display_name, rating, matches_played, wins, losses, draws, created_at, last_seen_at, session_secret, session_token, session_expires_at from guests where guest_id = \$1`).
		WithArgs("guest_black").
		WillReturnRows(sqlmock.NewRows([]string{
			"guest_id", "display_name", "rating", "matches_played", "wins", "losses", "draws", "created_at", "last_seen_at", "session_secret", "session_token", "session_expires_at",
		}).AddRow("guest_black", "Black", 1184, 1, 0, 1, 0, now, now, "secret_black", "guesttok_black", now.Add(6*time.Hour)))
	mock.ExpectQuery(`select winner from finalized_matches where match_id = \$1`).
		WithArgs("room_123").
		WillReturnRows(sqlmock.NewRows([]string{"winner"}).AddRow("white"))
	mock.ExpectRollback()

	white, black, changed, err := store.FinalizeMatch("room_123", "guest_white", "guest_black", "white")
	if err != nil {
		t.Fatalf("expected idempotent postgres finalize to succeed, got %v", err)
	}
	if changed {
		t.Fatalf("expected repeated postgres finalize to be unchanged")
	}
	if white.Rating != 1216 || black.Rating != 1184 {
		t.Fatalf("unexpected postgres guest ratings %#v %#v", white, black)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet postgres finalize expectations: %v", err)
	}
}

func TestPostgresGuestStoreResumeGuestByToken(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("expected sqlmock database, got %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta(postgresGuestInitSQL)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`alter table guests add column if not exists session_secret text`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`alter table guests add column if not exists session_token text`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`alter table guests add column if not exists session_expires_at timestamptz`)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	store, err := newPostgresGuestStoreWithDB(db)
	if err != nil {
		t.Fatalf("expected postgres guest store to initialize, got %v", err)
	}
	defer func() { _ = store.Close() }()

	now := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)
	mock.ExpectQuery(`select guest_id, display_name, rating, matches_played, wins, losses, draws, created_at, last_seen_at, session_secret, session_token, session_expires_at from guests where guest_id = \$1`).
		WithArgs("guest_token").
		WillReturnRows(sqlmock.NewRows([]string{
			"guest_id", "display_name", "rating", "matches_played", "wins", "losses", "draws", "created_at", "last_seen_at", "session_secret", "session_token", "session_expires_at",
		}).AddRow("guest_token", "Token Guest", 1200, 0, 0, 0, 0, now, now, "secret_token", "guesttok_resume", now.Add(6*time.Hour)))
	mock.ExpectExec(`update guests set last_seen_at = \$1, session_token = \$2, session_expires_at = \$3 where guest_id = \$4`).
		WithArgs(sqlmock.AnyArg(), "guesttok_resume", sqlmock.AnyArg(), "guest_token").
		WillReturnResult(sqlmock.NewResult(0, 1))

	session, err := store.ResumeGuestByToken("guest_token", "guesttok_resume")
	if err != nil {
		t.Fatalf("expected token resume to succeed, got %v", err)
	}
	if session.Guest.GuestID != "guest_token" || session.SessionToken != "guesttok_resume" || session.ExpiresAt.IsZero() {
		t.Fatalf("unexpected postgres token session %#v", session)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet postgres token resume expectations: %v", err)
	}
}
