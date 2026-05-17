package platform

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

const postgresFriendshipInitSQL = `
		create table if not exists friend_requests (
			request_id text primary key,
			requester_account_id text not null,
			target_account_id text not null,
			status text not null,
			created_at timestamptz not null,
			updated_at timestamptz not null
		);
		create index if not exists friend_requests_requester_idx on friend_requests (requester_account_id);
		create index if not exists friend_requests_target_idx on friend_requests (target_account_id);
		create table if not exists friendships (
			friendship_id text primary key,
			low_account_id text not null,
			high_account_id text not null,
			created_at timestamptz not null
		);
		create index if not exists friendships_low_idx on friendships (low_account_id);
		create index if not exists friendships_high_idx on friendships (high_account_id);
	`

func TestPostgresFriendshipStoreLoadAndPersist(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("expected sqlmock database, got %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta(postgresFriendshipInitSQL)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`select request_id, requester_account_id, target_account_id, status, created_at, updated_at from friend_requests`).
		WillReturnRows(sqlmock.NewRows([]string{"request_id", "requester_account_id", "target_account_id", "status", "created_at", "updated_at"}))
	mock.ExpectQuery(`select friendship_id, low_account_id, high_account_id, created_at from friendships`).
		WillReturnRows(sqlmock.NewRows([]string{"friendship_id", "low_account_id", "high_account_id", "created_at"}))

	persistence, err := newPostgresFriendshipPersistenceWithDB(db)
	if err != nil {
		t.Fatalf("expected postgres friendship store to initialize, got %v", err)
	}
	store, err := newFriendshipStore(persistence)
	if err != nil {
		t.Fatalf("expected wrapped postgres friendship store to initialize, got %v", err)
	}
	defer func() { _ = store.Close() }()

	mock.ExpectBegin()
	mock.ExpectExec(`delete from friend_requests`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`delete from friendships`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`insert into friend_requests\(request_id, requester_account_id, target_account_id, status, created_at, updated_at\) values\(\$1, \$2, \$3, \$4, \$5, \$6\)`).
		WithArgs(sqlmock.AnyArg(), "acct_one", "acct_two", FriendRequestStatusPending, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if _, err := store.SendRequest("acct_one", "acct_two"); err != nil {
		t.Fatalf("expected postgres request send to succeed, got %v", err)
	}

	now := time.Now().UTC()
	if now.IsZero() {
		t.Fatalf("expected current time to be non-zero")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet postgres friendship expectations: %v", err)
	}
}
