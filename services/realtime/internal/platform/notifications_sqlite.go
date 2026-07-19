package platform

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chess404/realtime/internal/contracts"
	_ "modernc.org/sqlite"
)

type sqliteAccountNotificationStore struct {
	db *sql.DB
}

func NewSQLiteAccountNotificationStore(path string) (*AccountNotificationStore, error) {
	store, err := newSQLiteAccountNotificationPersistence(path)
	if err != nil {
		return nil, err
	}
	return NewAccountNotificationStoreFromDB(store)
}

func newSQLiteAccountNotificationPersistence(path string) (*sqliteAccountNotificationStore, error) {
	if path != "" && path != ":memory:" && !strings.HasPrefix(path, "file:") {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	store := &sqliteAccountNotificationStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *sqliteAccountNotificationStore) backend() string {
	return "sqlite"
}

func (s *sqliteAccountNotificationStore) load() (map[string]AccountNotification, error) {
	notifications := make(map[string]AccountNotification)
	rows, err := s.db.Query(`select notification_id, account_id, actor_account_id, kind, friend_request_id, challenge_id, match_id, mode_id, challenger_seat, created_at, updated_at, read_at from account_notifications`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			notification    AccountNotification
			modeID          string
			friendRequestID sql.NullString
			challengeID     sql.NullString
			matchID         sql.NullString
			challengerSeat  sql.NullString
			createdAt       string
			updatedAt       string
			readAt          sql.NullString
		)
		if err := rows.Scan(
			&notification.NotificationID,
			&notification.AccountID,
			&notification.ActorAccountID,
			&notification.Kind,
			&friendRequestID,
			&challengeID,
			&matchID,
			&modeID,
			&challengerSeat,
			&createdAt,
			&updatedAt,
			&readAt,
		); err != nil {
			return nil, err
		}
		parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, err
		}
		parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			return nil, err
		}
		notification.CreatedAt = parsedCreatedAt
		notification.UpdatedAt = parsedUpdatedAt
		notification.ModeID = contracts.NormalizeMatchModeID(modeID)
		notification.FriendRequestID = strings.TrimSpace(friendRequestID.String)
		notification.ChallengeID = strings.TrimSpace(challengeID.String)
		notification.MatchID = strings.TrimSpace(matchID.String)
		notification.ChallengerSeat = normalizeChallengeSeat(challengerSeat.String)
		if readAt.Valid && strings.TrimSpace(readAt.String) != "" {
			parsedReadAt, err := time.Parse(time.RFC3339Nano, readAt.String)
			if err != nil {
				return nil, err
			}
			notification.ReadAt = &parsedReadAt
		}
		notifications[notification.NotificationID] = notification
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return notifications, nil
}

func (s *sqliteAccountNotificationStore) persist(notifications map[string]AccountNotification) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`delete from account_notifications`); err != nil {
		return err
	}
	for _, notification := range notifications {
		if _, err := tx.Exec(
			`insert into account_notifications(notification_id, account_id, actor_account_id, kind, friend_request_id, challenge_id, match_id, mode_id, challenger_seat, created_at, updated_at, read_at) values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			notification.NotificationID,
			notification.AccountID,
			notification.ActorAccountID,
			notification.Kind,
			nullableTrimmedString(notification.FriendRequestID),
			nullableTrimmedString(notification.ChallengeID),
			nullableTrimmedString(notification.MatchID),
			string(notification.ModeID),
			nullableTrimmedString(notification.ChallengerSeat),
			timeString(notification.CreatedAt),
			timeString(notification.UpdatedAt),
			nullTimePointerString(notification.ReadAt),
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *sqliteAccountNotificationStore) close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *sqliteAccountNotificationStore) init() error {
	_, _ = s.db.Exec(`PRAGMA journal_mode=WAL`)
	_, _ = s.db.Exec(`PRAGMA busy_timeout=5000`)
	_, err := s.db.Exec(`
		create table if not exists account_notifications (
			notification_id text primary key,
			account_id text not null,
			actor_account_id text not null,
			kind text not null,
			friend_request_id text,
			challenge_id text,
			match_id text,
			mode_id text not null,
			challenger_seat text,
			created_at text not null,
			updated_at text not null,
			read_at text
		);
		create index if not exists account_notifications_account_idx on account_notifications (account_id, updated_at desc, created_at desc);
		create index if not exists account_notifications_actor_idx on account_notifications (actor_account_id);
		create index if not exists account_notifications_read_idx on account_notifications (account_id, read_at);
	`)
	return err
}

func nullableTrimmedString(value string) any {
	resolved := strings.TrimSpace(value)
	if resolved == "" {
		return nil
	}
	return resolved
}

func nullTimePointerString(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return timeString(value.UTC())
}
