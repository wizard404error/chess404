package platform

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

type sqliteArchiveStore struct {
	db *sql.DB
}

func newSQLiteArchiveStore(path string) (*sqliteArchiveStore, error) {
	if path != "" && path != ":memory:" && !strings.HasPrefix(path, "file:") {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	store := &sqliteArchiveStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *sqliteArchiveStore) backend() string {
	return "sqlite"
}

func (s *sqliteArchiveStore) load() (map[string]MatchArchiveEntry, map[string]MatchArchivePrivateEntry, error) {
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
			entryJSON   string
			privateJSON sql.NullString
		)
		if err := rows.Scan(&matchID, &entryJSON, &privateJSON); err != nil {
			return nil, nil, err
		}
		var entry MatchArchiveEntry
		if err := json.Unmarshal([]byte(entryJSON), &entry); err != nil {
			return nil, nil, err
		}
		entries[matchID] = entry

		if privateJSON.Valid && privateJSON.String != "" {
			var privateEntry MatchArchivePrivateEntry
			if err := json.Unmarshal([]byte(privateJSON.String), &privateEntry); err != nil {
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

func (s *sqliteArchiveStore) persist(entries map[string]MatchArchiveEntry, private map[string]MatchArchivePrivateEntry) error {
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
			privateJSON = string(encodedPrivate)
		}

		if _, err := tx.Exec(`
			insert into archives(match_id, entry_json, private_json)
			values(?, ?, ?)
		`, matchID, string(entryJSON), privateJSON); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *sqliteArchiveStore) close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *sqliteArchiveStore) init() error {
	_, err := s.db.Exec(`
		create table if not exists archives (
			match_id text primary key,
			entry_json text not null,
			private_json text
		)
	`)
	return err
}
