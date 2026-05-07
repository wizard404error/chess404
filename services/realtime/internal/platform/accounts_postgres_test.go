package platform

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

const postgresAccountInitSQL = `
		create table if not exists accounts (
			account_id text primary key,
			handle text not null unique,
			primary_guest_id text not null,
			linked_guest_ids jsonb not null,
			rating integer not null default 1200,
			matches_played integer not null default 0,
			wins integer not null default 0,
			losses integer not null default 0,
			draws integer not null default 0,
			rating_history jsonb not null default '[]'::jsonb,
			created_at timestamptz not null,
			last_seen_at timestamptz not null,
			session_token text,
			session_expires_at timestamptz
		);
		create table if not exists account_guest_links (
			guest_id text primary key,
			account_id text not null references accounts(account_id) on delete cascade
		);
		create table if not exists account_finalized_matches (
			match_id text primary key,
			winner text not null
		);
		alter table accounts add column if not exists rating integer not null default 1200;
		alter table accounts add column if not exists matches_played integer not null default 0;
		alter table accounts add column if not exists wins integer not null default 0;
		alter table accounts add column if not exists losses integer not null default 0;
		alter table accounts add column if not exists draws integer not null default 0;
		alter table accounts add column if not exists rating_history jsonb not null default '[]'::jsonb;
		create index if not exists accounts_last_seen_order_idx on accounts (last_seen_at desc, created_at desc, account_id asc);
	`

func TestPostgresAccountStoreClaimGuestTouchesExistingAccount(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("expected sqlmock database, got %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta(postgresAccountInitSQL)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	store, err := newPostgresAccountStoreWithDB(db)
	if err != nil {
		t.Fatalf("expected postgres account store to initialize, got %v", err)
	}
	defer func() { _ = store.Close() }()

	now := time.Date(2026, 5, 7, 1, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(`select a.account_id, a.handle, a.primary_guest_id, a.linked_guest_ids, a.rating, a.matches_played, a.wins, a.losses, a.draws, a.rating_history, a.created_at, a.last_seen_at, a.session_token, a.session_expires_at from accounts a join account_guest_links l on l.account_id = a.account_id where l.guest_id = \$1`).
		WithArgs("guest_existing").
		WillReturnRows(sqlmock.NewRows([]string{
			"account_id", "handle", "primary_guest_id", "linked_guest_ids", "rating", "matches_played", "wins", "losses", "draws", "rating_history", "created_at", "last_seen_at", "session_token", "session_expires_at",
		}).AddRow("acct_existing", "aurora_fox", "guest_existing", `["guest_existing"]`, 1250, 9, 6, 2, 1, `[]`, now, now, "accttok_existing", now.Add(6*time.Hour)))
	mock.ExpectExec(`update accounts set handle = \$1, primary_guest_id = \$2, linked_guest_ids = \$3, rating = \$4, matches_played = \$5, wins = \$6, losses = \$7, draws = \$8, rating_history = \$9, created_at = \$10, last_seen_at = \$11, session_token = \$12, session_expires_at = \$13 where account_id = \$14`).
		WithArgs("aurora_fox", "guest_existing", []byte(`["guest_existing"]`), 1250, 9, 6, 2, 1, []byte(`[]`), now, sqlmock.AnyArg(), "accttok_existing", sqlmock.AnyArg(), "acct_existing").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	session, err := store.ClaimGuest(GuestProfile{GuestID: "guest_existing"}, "aurora_fox")
	if err != nil {
		t.Fatalf("expected postgres account claim to succeed, got %v", err)
	}
	if session.Account.AccountID != "acct_existing" || session.Account.Handle != "aurora_fox" || session.SessionToken != "accttok_existing" || session.ExpiresAt.IsZero() {
		t.Fatalf("unexpected postgres account session %#v", session)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet postgres account expectations: %v", err)
	}
}

func TestPostgresAccountStoreResumeAccountByToken(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("expected sqlmock database, got %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta(postgresAccountInitSQL)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	store, err := newPostgresAccountStoreWithDB(db)
	if err != nil {
		t.Fatalf("expected postgres account store to initialize, got %v", err)
	}
	defer func() { _ = store.Close() }()

	now := time.Date(2026, 5, 7, 1, 15, 0, 0, time.UTC)
	mock.ExpectQuery(`select account_id, handle, primary_guest_id, linked_guest_ids, rating, matches_played, wins, losses, draws, rating_history, created_at, last_seen_at, session_token, session_expires_at from accounts where account_id = \$1`).
		WithArgs("acct_resume").
		WillReturnRows(sqlmock.NewRows([]string{
			"account_id", "handle", "primary_guest_id", "linked_guest_ids", "rating", "matches_played", "wins", "losses", "draws", "rating_history", "created_at", "last_seen_at", "session_token", "session_expires_at",
		}).AddRow("acct_resume", "aurora_resume", "guest_resume", `["guest_resume"]`, 1210, 3, 2, 1, 0, `[]`, now, now, "accttok_resume", now.Add(6*time.Hour)))
	mock.ExpectExec(`update accounts set last_seen_at = \$1, session_token = \$2, session_expires_at = \$3 where account_id = \$4`).
		WithArgs(sqlmock.AnyArg(), "accttok_resume", sqlmock.AnyArg(), "acct_resume").
		WillReturnResult(sqlmock.NewResult(0, 1))

	session, err := store.ResumeAccount("acct_resume", "accttok_resume")
	if err != nil {
		t.Fatalf("expected postgres account resume to succeed, got %v", err)
	}
	if session.Account.AccountID != "acct_resume" || session.SessionToken != "accttok_resume" || session.ExpiresAt.IsZero() {
		t.Fatalf("unexpected postgres account resume %#v", session)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet postgres account resume expectations: %v", err)
	}
}
