package platform

import (
	"database/sql"
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
			last_active_at timestamptz,
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
		create table if not exists account_credentials (
			account_id text primary key references accounts(account_id) on delete cascade,
			email text not null unique,
			password_hash text not null,
			email_verified_at timestamptz
		);
		create table if not exists account_email_verifications (
			account_id text not null references accounts(account_id) on delete cascade,
			token text primary key,
			email text not null,
			expires_at timestamptz not null,
			created_at timestamptz not null,
			used_at timestamptz,
			updated_at timestamptz not null
		);
		create table if not exists account_password_resets (
			account_id text not null references accounts(account_id) on delete cascade,
			token text primary key,
			expires_at timestamptz not null,
			created_at timestamptz not null,
			used_at timestamptz,
			updated_at timestamptz not null
		);
		create table if not exists account_sessions (
			account_id text not null references accounts(account_id) on delete cascade,
			session_token text primary key,
			expires_at timestamptz not null,
			created_at timestamptz not null,
			last_seen_at timestamptz not null
		);
		alter table accounts add column if not exists rating integer not null default 1200;
		alter table accounts add column if not exists matches_played integer not null default 0;
		alter table accounts add column if not exists wins integer not null default 0;
		alter table accounts add column if not exists losses integer not null default 0;
		alter table accounts add column if not exists draws integer not null default 0;
		alter table accounts add column if not exists rating_history jsonb not null default '[]'::jsonb;
		alter table accounts add column if not exists last_active_at timestamptz;
		alter table account_credentials add column if not exists email_verified_at timestamptz;
		create index if not exists accounts_last_seen_order_idx on accounts (last_seen_at desc, created_at desc, account_id asc);
		create index if not exists account_sessions_account_idx on account_sessions (account_id, last_seen_at desc, created_at desc, session_token asc);
		create index if not exists account_sessions_expires_idx on account_sessions (expires_at);
		create index if not exists account_email_verifications_account_idx on account_email_verifications (account_id, created_at desc);
		create index if not exists account_password_resets_account_idx on account_password_resets (account_id, created_at desc);
		insert into account_sessions(account_id, session_token, expires_at, created_at, last_seen_at)
		select account_id, session_token, session_expires_at, coalesce(last_active_at, last_seen_at, created_at), coalesce(last_active_at, last_seen_at, created_at)
		from accounts
		where session_token is not null
			and session_expires_at is not null
			and not exists (
				select 1 from account_sessions s where s.session_token = accounts.session_token
			)
		on conflict (session_token) do nothing;
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

	now := time.Now().UTC().Add(-30 * time.Minute)
	mock.ExpectBegin()
	mock.ExpectQuery(`select a.account_id, a.handle, a.primary_guest_id, a.linked_guest_ids, a.rating, a.matches_played, a.wins, a.losses, a.draws, a.rating_history, a.created_at, a.last_seen_at, a.last_active_at, a.session_token, a.session_expires_at from accounts a join account_guest_links l on l.account_id = a.account_id where l.guest_id = \$1`).
		WithArgs("guest_existing").
		WillReturnRows(sqlmock.NewRows([]string{
			"account_id", "handle", "primary_guest_id", "linked_guest_ids", "rating", "matches_played", "wins", "losses", "draws", "rating_history", "created_at", "last_seen_at", "last_active_at", "session_token", "session_expires_at",
		}).AddRow("acct_existing", "aurora_fox", "guest_existing", `["guest_existing"]`, 1250, 9, 6, 2, 1, `[]`, now, now, now, "accttok_existing", now.Add(6*time.Hour)))
	mock.ExpectExec(`update accounts set handle = \$1, primary_guest_id = \$2, linked_guest_ids = \$3, rating = \$4, matches_played = \$5, wins = \$6, losses = \$7, draws = \$8, rating_history = \$9, created_at = \$10, last_seen_at = \$11, last_active_at = \$12, session_token = \$13, session_expires_at = \$14 where account_id = \$15`).
		WithArgs("aurora_fox", "guest_existing", []byte(`["guest_existing"]`), 1250, 9, 6, 2, 1, []byte(`[]`), now, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "acct_existing").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`insert into account_sessions\(account_id, session_token, expires_at, created_at, last_seen_at\) values\(\$1, \$2, \$3, \$4, \$5\)`).
		WithArgs("acct_existing", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	session, err := store.ClaimGuest(GuestProfile{GuestID: "guest_existing"}, "aurora_fox")
	if err != nil {
		t.Fatalf("expected postgres account claim to succeed, got %v", err)
	}
	if session.Account.AccountID != "acct_existing" || session.Account.Handle != "aurora_fox" || session.SessionToken == "" || session.ExpiresAt.IsZero() {
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

	now := time.Now().UTC().Add(-15 * time.Minute)
	mock.ExpectBegin()
	mock.ExpectQuery(`select account_id, handle, primary_guest_id, linked_guest_ids, rating, matches_played, wins, losses, draws, rating_history, created_at, last_seen_at, last_active_at, session_token, session_expires_at from accounts where account_id = \$1`).
		WithArgs("acct_resume").
		WillReturnRows(sqlmock.NewRows([]string{
			"account_id", "handle", "primary_guest_id", "linked_guest_ids", "rating", "matches_played", "wins", "losses", "draws", "rating_history", "created_at", "last_seen_at", "last_active_at", "session_token", "session_expires_at",
		}).AddRow("acct_resume", "aurora_resume", "guest_resume", `["guest_resume"]`, 1210, 3, 2, 1, 0, `[]`, now, now, now, "accttok_resume", now.Add(6*time.Hour)))
	mock.ExpectQuery(`select session_token, expires_at, created_at, last_seen_at from account_sessions where account_id = \$1 and session_token = \$2 and expires_at > \$3`).
		WithArgs("acct_resume", "accttok_resume", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"session_token", "expires_at", "created_at", "last_seen_at"}).AddRow("accttok_resume", now.Add(6*time.Hour), now, now))
	mock.ExpectExec(`insert into account_sessions\(account_id, session_token, expires_at, created_at, last_seen_at\) values\(\$1, \$2, \$3, \$4, \$5\) on conflict \(session_token\) do update set account_id = excluded.account_id, expires_at = excluded.expires_at, created_at = excluded.created_at, last_seen_at = excluded.last_seen_at`).
		WithArgs("acct_resume", "accttok_resume", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`update accounts set last_seen_at = \$1, last_active_at = \$2, session_token = \$3, session_expires_at = \$4 where account_id = \$5`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "accttok_resume", sqlmock.AnyArg(), "acct_resume").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

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

func TestPostgresAccountStoreEnablePasswordLoginAndLoginByEmail(t *testing.T) {
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

	now := time.Now().UTC().Add(-10 * time.Minute)
	mock.ExpectBegin()
	mock.ExpectQuery(`select account_id, handle, primary_guest_id, linked_guest_ids, rating, matches_played, wins, losses, draws, rating_history, created_at, last_seen_at, last_active_at, session_token, session_expires_at from accounts where account_id = \$1`).
		WithArgs("acct_auth").
		WillReturnRows(sqlmock.NewRows([]string{
			"account_id", "handle", "primary_guest_id", "linked_guest_ids", "rating", "matches_played", "wins", "losses", "draws", "rating_history", "created_at", "last_seen_at", "last_active_at", "session_token", "session_expires_at",
		}).AddRow("acct_auth", "aurora_auth", "guest_auth", `["guest_auth"]`, 1215, 4, 3, 1, 0, `[]`, now, now, now, "accttok_auth", now.Add(6*time.Hour)))
	mock.ExpectQuery(`select session_token, expires_at, created_at, last_seen_at from account_sessions where account_id = \$1 and session_token = \$2 and expires_at > \$3`).
		WithArgs("acct_auth", "accttok_auth", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"session_token", "expires_at", "created_at", "last_seen_at"}).AddRow("accttok_auth", now.Add(6*time.Hour), now, now))
	mock.ExpectQuery(`select account_id from account_credentials where email = \$1`).
		WithArgs("aurora@example.com").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(`update accounts set handle = \$1, primary_guest_id = \$2, linked_guest_ids = \$3, rating = \$4, matches_played = \$5, wins = \$6, losses = \$7, draws = \$8, rating_history = \$9, created_at = \$10, last_seen_at = \$11, last_active_at = \$12, session_token = \$13, session_expires_at = \$14 where account_id = \$15`).
		WithArgs("aurora_auth", "guest_auth", []byte(`["guest_auth"]`), 1215, 4, 3, 1, 0, []byte(`[]`), now, sqlmock.AnyArg(), sqlmock.AnyArg(), "accttok_auth", sqlmock.AnyArg(), "acct_auth").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`insert into account_sessions\(account_id, session_token, expires_at, created_at, last_seen_at\) values\(\$1, \$2, \$3, \$4, \$5\) on conflict \(session_token\) do update set account_id = excluded.account_id, expires_at = excluded.expires_at, created_at = excluded.created_at, last_seen_at = excluded.last_seen_at`).
		WithArgs("acct_auth", "accttok_auth", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`select email, password_hash, email_verified_at from account_credentials where account_id = \$1`).
		WithArgs("acct_auth").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(`insert into account_credentials\(account_id, email, password_hash\) values\(\$1, \$2, \$3\) on conflict\(account_id\) do update set email = excluded.email, password_hash = excluded.password_hash`).
		WithArgs("acct_auth", "aurora@example.com", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`update account_credentials set email_verified_at = null where account_id = \$1`).
		WithArgs("acct_auth").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`delete from account_email_verifications where account_id = \$1`).
		WithArgs("acct_auth").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`delete from account_password_resets where account_id = \$1`).
		WithArgs("acct_auth").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	enabled, err := store.EnablePasswordLogin("acct_auth", "accttok_auth", "aurora@example.com", "Swordfish88")
	if err != nil {
		t.Fatalf("expected postgres password setup to succeed, got %v", err)
	}
	if enabled.Account.AccountID != "acct_auth" {
		t.Fatalf("unexpected postgres enabled session %#v", enabled)
	}

	passwordHash, err := hashAccountPassword("Swordfish88")
	if err != nil {
		t.Fatalf("expected password hash to build, got %v", err)
	}
	mock.ExpectQuery(`select account_id, password_hash from account_credentials where email = \$1`).
		WithArgs("aurora@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"account_id", "password_hash"}).AddRow("acct_auth", passwordHash))
	mock.ExpectQuery(`select account_id, handle, primary_guest_id, linked_guest_ids, rating, matches_played, wins, losses, draws, rating_history, created_at, last_seen_at, last_active_at, session_token, session_expires_at from accounts where account_id = \$1`).
		WithArgs("acct_auth").
		WillReturnRows(sqlmock.NewRows([]string{
			"account_id", "handle", "primary_guest_id", "linked_guest_ids", "rating", "matches_played", "wins", "losses", "draws", "rating_history", "created_at", "last_seen_at", "last_active_at", "session_token", "session_expires_at",
		}).AddRow("acct_auth", "aurora_auth", "guest_auth", `["guest_auth"]`, 1215, 4, 3, 1, 0, `[]`, now, now, now, "accttok_auth", now.Add(6*time.Hour)))
	mock.ExpectBegin()
	mock.ExpectExec(`insert into account_sessions\(account_id, session_token, expires_at, created_at, last_seen_at\) values\(\$1, \$2, \$3, \$4, \$5\)`).
		WithArgs("acct_auth", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`update accounts set last_seen_at = \$1, last_active_at = \$2, session_token = \$3, session_expires_at = \$4 where account_id = \$5`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "acct_auth").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	loggedIn, err := store.LoginWithPassword("aurora@example.com", "Swordfish88")
	if err != nil {
		t.Fatalf("expected postgres email login to succeed, got %v", err)
	}
	if loggedIn.Account.AccountID != "acct_auth" {
		t.Fatalf("unexpected postgres login session %#v", loggedIn)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet postgres auth expectations: %v", err)
	}
}
