package platform

import (
	"database/sql"
	"encoding/json"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type postgresArchiveStore struct {
	db *sql.DB
}

func newPostgresArchiveStore(dsn string) (*postgresArchiveStore, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(3 * time.Minute)
	return newPostgresArchiveStoreWithDB(db)
}

func newPostgresArchiveStoreWithDB(db *sql.DB) (*postgresArchiveStore, error) {
	store := &postgresArchiveStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *postgresArchiveStore) backend() string {
	return "postgres"
}

func (s *postgresArchiveStore) load() (map[string]MatchArchiveEntry, map[string]MatchArchivePrivateEntry, error) {
	rows, err := s.db.Query(`
		select match_id, entry_json, private_json
		from archives
	`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	entries := make(map[string]MatchArchiveEntry)
	private := make(map[string]MatchArchivePrivateEntry)

	for rows.Next() {
		var (
			matchID     string
			entryJSON   []byte
			privateJSON []byte
		)
		if err := rows.Scan(&matchID, &entryJSON, &privateJSON); err != nil {
			return nil, nil, err
		}

		var entry MatchArchiveEntry
		if err := json.Unmarshal(entryJSON, &entry); err != nil {
			return nil, nil, err
		}
		entries[matchID] = entry

		if len(privateJSON) > 0 {
			var privateEntry MatchArchivePrivateEntry
			if err := json.Unmarshal(privateJSON, &privateEntry); err != nil {
				return nil, nil, err
			}
			private[matchID] = privateEntry
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return entries, private, nil
}

func (s *postgresArchiveStore) persist(entries map[string]MatchArchiveEntry, private map[string]MatchArchivePrivateEntry) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.Exec(`delete from archives`); err != nil {
		return err
	}

	for matchID, entry := range entries {
		entryJSON, err := json.Marshal(entry)
		if err != nil {
			return err
		}

		var privateJSON any
		if privateEntry, ok := private[matchID]; ok {
			encodedPrivate, err := json.Marshal(privateEntry)
			if err != nil {
				return err
			}
			privateJSON = encodedPrivate
		}

		if _, err := tx.Exec(`
			insert into archives(
				match_id,
				status,
				queue,
				white_guest_id,
				black_guest_id,
				updated_at,
				entry_json,
				private_json
			)
			values($1, $2, $3, $4, $5, $6, $7::jsonb, $8::jsonb)
		`,
			matchID,
			entry.Status,
			entry.Queue,
			nullIfEmpty(entry.WhiteGuestID),
			nullIfEmpty(entry.BlackGuestID),
			entry.UpdatedAt,
			entryJSON,
			privateJSON,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *postgresArchiveStore) close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *postgresArchiveStore) init() error {
	_, err := s.db.Exec(`
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
	`)
	return err
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
