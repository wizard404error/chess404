package platform

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type sqliteFriendshipStore struct {
	db *sql.DB
}

func NewSQLiteFriendshipStore(path string) (*FriendshipStore, error) {
	store, err := newSQLiteFriendshipPersistence(path)
	if err != nil {
		return nil, err
	}
	return newFriendshipStore(store)
}

func newSQLiteFriendshipPersistence(path string) (*sqliteFriendshipStore, error) {
	if path != "" && path != ":memory:" && !strings.HasPrefix(path, "file:") {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	store := &sqliteFriendshipStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *sqliteFriendshipStore) backend() string {
	return "sqlite"
}

func (s *sqliteFriendshipStore) load() (map[string]FriendRequest, map[string]Friendship, error) {
	requests := make(map[string]FriendRequest)
	friendships := make(map[string]Friendship)

	requestRows, err := s.db.Query(`select request_id, requester_account_id, target_account_id, status, created_at, updated_at from friend_requests`)
	if err != nil {
		return nil, nil, err
	}
	defer requestRows.Close()
	for requestRows.Next() {
		var (
			request   FriendRequest
			createdAt string
			updatedAt string
		)
		if err := requestRows.Scan(&request.RequestID, &request.RequesterAccountID, &request.TargetAccountID, &request.Status, &createdAt, &updatedAt); err != nil {
			return nil, nil, err
		}
		parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, nil, err
		}
		parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			return nil, nil, err
		}
		request.CreatedAt = parsedCreatedAt
		request.UpdatedAt = parsedUpdatedAt
		requests[request.RequestID] = request
	}
	if err := requestRows.Err(); err != nil {
		return nil, nil, err
	}

	friendRows, err := s.db.Query(`select friendship_id, low_account_id, high_account_id, created_at from friendships`)
	if err != nil {
		return nil, nil, err
	}
	defer friendRows.Close()
	for friendRows.Next() {
		var (
			friendship Friendship
			createdAt  string
		)
		if err := friendRows.Scan(&friendship.FriendshipID, &friendship.LowAccountID, &friendship.HighAccountID, &createdAt); err != nil {
			return nil, nil, err
		}
		parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, nil, err
		}
		friendship.CreatedAt = parsedCreatedAt
		friendships[friendship.FriendshipID] = friendship
	}
	if err := friendRows.Err(); err != nil {
		return nil, nil, err
	}

	return requests, friendships, nil
}

func (s *sqliteFriendshipStore) persist(requests map[string]FriendRequest, friendships map[string]Friendship) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`delete from friend_requests`); err != nil {
		return err
	}
	if _, err := tx.Exec(`delete from friendships`); err != nil {
		return err
	}

	for _, request := range requests {
		if _, err := tx.Exec(
			`insert into friend_requests(request_id, requester_account_id, target_account_id, status, created_at, updated_at) values(?, ?, ?, ?, ?, ?)`,
			request.RequestID,
			request.RequesterAccountID,
			request.TargetAccountID,
			request.Status,
			timeString(request.CreatedAt),
			timeString(request.UpdatedAt),
		); err != nil {
			return err
		}
	}
	for _, friendship := range friendships {
		if _, err := tx.Exec(
			`insert into friendships(friendship_id, low_account_id, high_account_id, created_at) values(?, ?, ?, ?)`,
			friendship.FriendshipID,
			friendship.LowAccountID,
			friendship.HighAccountID,
			timeString(friendship.CreatedAt),
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *sqliteFriendshipStore) close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *sqliteFriendshipStore) init() error {
	_, err := s.db.Exec(`
		create table if not exists friend_requests (
			request_id text primary key,
			requester_account_id text not null,
			target_account_id text not null,
			status text not null,
			created_at text not null,
			updated_at text not null
		);
		create index if not exists friend_requests_requester_idx on friend_requests (requester_account_id);
		create index if not exists friend_requests_target_idx on friend_requests (target_account_id);
		create table if not exists friendships (
			friendship_id text primary key,
			low_account_id text not null,
			high_account_id text not null,
			created_at text not null
		);
		create index if not exists friendships_low_idx on friendships (low_account_id);
		create index if not exists friendships_high_idx on friendships (high_account_id);
	`)
	return err
}
