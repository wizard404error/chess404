package platform

import (
	"database/sql"
	"time"

	"github.com/chess404/realtime/internal/contracts"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type postgresAccountNotificationStore struct {
	db *sql.DB
}

func NewPostgresAccountNotificationStore(dsn string) (*AccountNotificationStore, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	store, err := newPostgresAccountNotificationPersistenceWithDB(db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return newAccountNotificationStore(store)
}

func newPostgresAccountNotificationPersistenceWithDB(db *sql.DB) (*postgresAccountNotificationStore, error) {
	store := &postgresAccountNotificationStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *postgresAccountNotificationStore) backend() string {
	return "postgres"
}

func (s *postgresAccountNotificationStore) load() (map[string]AccountNotification, error) {
	notifications := make(map[string]AccountNotification)
	rows, err := s.db.Query(`select notification_id, account_id, actor_account_id, kind, friend_request_id, challenge_id, match_id, mode_id, challenger_seat, created_at, updated_at, read_at from account_notifications`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			notification    AccountNotification
			friendRequestID sql.NullString
			challengeID     sql.NullString
			matchID         sql.NullString
			modeID          string
			challengerSeat  sql.NullString
			readAt          sql.NullTime
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
			&notification.CreatedAt,
			&notification.UpdatedAt,
			&readAt,
		); err != nil {
			return nil, err
		}
		notification.FriendRequestID = friendRequestID.String
		notification.ChallengeID = challengeID.String
		notification.MatchID = matchID.String
		notification.ModeID = contracts.NormalizeMatchModeID(modeID)
		notification.ChallengerSeat = normalizeChallengeSeat(challengerSeat.String)
		if readAt.Valid {
			value := readAt.Time.UTC()
			notification.ReadAt = &value
		}
		notifications[notification.NotificationID] = notification
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return notifications, nil
}

func (s *postgresAccountNotificationStore) persist(notifications map[string]AccountNotification) error {
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
			`insert into account_notifications(notification_id, account_id, actor_account_id, kind, friend_request_id, challenge_id, match_id, mode_id, challenger_seat, created_at, updated_at, read_at) values($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
			notification.NotificationID,
			notification.AccountID,
			notification.ActorAccountID,
			notification.Kind,
			nullableTrimmedString(notification.FriendRequestID),
			nullableTrimmedString(notification.ChallengeID),
			nullableTrimmedString(notification.MatchID),
			string(notification.ModeID),
			nullableTrimmedString(notification.ChallengerSeat),
			notification.CreatedAt.UTC(),
			notification.UpdatedAt.UTC(),
			nullableReadAtTime(notification.ReadAt),
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *postgresAccountNotificationStore) close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *postgresAccountNotificationStore) init() error {
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
			created_at timestamptz not null,
			updated_at timestamptz not null,
			read_at timestamptz
		);
		create index if not exists account_notifications_account_idx on account_notifications (account_id, updated_at desc, created_at desc);
		create index if not exists account_notifications_actor_idx on account_notifications (actor_account_id);
		create index if not exists account_notifications_read_idx on account_notifications (account_id, read_at);
	`)
	return err
}

func nullableReadAtTime(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.UTC()
}
