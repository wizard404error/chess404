package platform

import (
	"database/sql"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type postgresFriendshipStore struct {
	db *sql.DB
}

func NewPostgresFriendshipStore(dsn string) (*FriendshipStore, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(3 * time.Minute)
	store, err := newPostgresFriendshipPersistenceWithDB(db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return newFriendshipStore(store)
}

func newPostgresFriendshipPersistenceWithDB(db *sql.DB) (*postgresFriendshipStore, error) {
	store := &postgresFriendshipStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *postgresFriendshipStore) backend() string {
	return "postgres"
}

func (s *postgresFriendshipStore) load() (map[string]FriendRequest, map[string]Friendship, error) {
	requests := make(map[string]FriendRequest)
	friendships := make(map[string]Friendship)

	requestRows, err := s.db.Query(`select request_id, requester_account_id, target_account_id, status, created_at, updated_at from friend_requests`)
	if err != nil {
		return nil, nil, err
	}
	defer requestRows.Close()
	for requestRows.Next() {
		var request FriendRequest
		if err := requestRows.Scan(&request.RequestID, &request.RequesterAccountID, &request.TargetAccountID, &request.Status, &request.CreatedAt, &request.UpdatedAt); err != nil {
			return nil, nil, err
		}
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
		var friendship Friendship
		if err := friendRows.Scan(&friendship.FriendshipID, &friendship.LowAccountID, &friendship.HighAccountID, &friendship.CreatedAt); err != nil {
			return nil, nil, err
		}
		friendships[friendship.FriendshipID] = friendship
	}
	if err := friendRows.Err(); err != nil {
		return nil, nil, err
	}

	return requests, friendships, nil
}

func (s *postgresFriendshipStore) persist(requests map[string]FriendRequest, friendships map[string]Friendship) error {
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
			`insert into friend_requests(request_id, requester_account_id, target_account_id, status, created_at, updated_at) values($1, $2, $3, $4, $5, $6)`,
			request.RequestID,
			request.RequesterAccountID,
			request.TargetAccountID,
			request.Status,
			request.CreatedAt.UTC(),
			request.UpdatedAt.UTC(),
		); err != nil {
			return err
		}
	}
	for _, friendship := range friendships {
		if _, err := tx.Exec(
			`insert into friendships(friendship_id, low_account_id, high_account_id, created_at) values($1, $2, $3, $4)`,
			friendship.FriendshipID,
			friendship.LowAccountID,
			friendship.HighAccountID,
			friendship.CreatedAt.UTC(),
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *postgresFriendshipStore) close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *postgresFriendshipStore) init() error {
	_, err := s.db.Exec(`
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
	`)
	return err
}
