package platform

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

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
	configurePostgresPool(db, 25, 5)
	return NewPostgresArchiveStoreWithDB(db)
}

func NewPostgresArchiveStoreWithDB(db *sql.DB) (*postgresArchiveStore, error) {
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
	// Postgres backend uses lazy-loading and DB-backed queries.
	// Return empty maps; data is fetched on demand via query* methods.
	return make(map[string]MatchArchiveEntry), make(map[string]MatchArchivePrivateEntry), nil
}

func scanSingleEntry(row interface{ Scan(...any) error }) (MatchArchiveEntry, bool, error) {
	var matchID string
	var entryJSON, privateJSON []byte
	if err := row.Scan(&matchID, &entryJSON, &privateJSON); err != nil {
		if err == sql.ErrNoRows {
			return MatchArchiveEntry{}, false, nil
		}
		return MatchArchiveEntry{}, false, err
	}
	var entry MatchArchiveEntry
	if err := json.Unmarshal(entryJSON, &entry); err != nil {
		return MatchArchiveEntry{}, false, err
	}
	return entry, true, nil
}

func (s *postgresArchiveStore) queryGet(matchID string) (MatchArchiveEntry, bool, error) {
	return scanSingleEntry(s.db.QueryRow(`
		select match_id, entry_json, private_json
		from archives
		where match_id = $1
	`, matchID))
}

func (s *postgresArchiveStore) queryPrivate(matchID string) (MatchArchivePrivateEntry, bool, error) {
	var privateJSON []byte
	err := s.db.QueryRow(`select private_json from archives where match_id = $1`, matchID).Scan(&privateJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return MatchArchivePrivateEntry{}, false, nil
		}
		return MatchArchivePrivateEntry{}, false, err
	}
	if len(privateJSON) == 0 {
		return MatchArchivePrivateEntry{}, true, nil
	}
	var privateEntry MatchArchivePrivateEntry
	if err := json.Unmarshal(privateJSON, &privateEntry); err != nil {
		return MatchArchivePrivateEntry{}, false, err
	}
	return privateEntry, true, nil
}

func (s *postgresArchiveStore) scanEntryRows(rows *sql.Rows) ([]MatchArchiveEntry, error) {
	defer rows.Close()
	var items []MatchArchiveEntry
	for rows.Next() {
		var matchID string
		var entryJSON, privateJSON []byte
		if err := rows.Scan(&matchID, &entryJSON, &privateJSON); err != nil {
			return nil, err
		}
		var entry MatchArchiveEntry
		if err := json.Unmarshal(entryJSON, &entry); err != nil {
			return nil, err
		}
		items = append(items, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *postgresArchiveStore) queryList(limit, offset int) ([]MatchArchiveEntry, error) {
	rows, err := s.db.Query(`
		select match_id, entry_json, private_json
		from archives
		order by updated_at desc
		limit $1 offset $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	return s.scanEntryRows(rows)
}

func (s *postgresArchiveStore) queryUnfinishedIDs(limit int) ([]string, error) {
	rows, err := s.db.Query(`
		select match_id
		from archives
		where status != 'finished'
		order by updated_at desc
		limit $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *postgresArchiveStore) queryFinishedIDs(limit int) ([]string, error) {
	rows, err := s.db.Query(`
		select match_id
		from archives
		where status = 'finished'
		order by updated_at desc
		limit $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *postgresArchiveStore) queryByGuest(guestID string, limit, offset int) ([]MatchArchiveEntry, error) {
	rows, err := s.db.Query(`
		select match_id, entry_json, private_json
		from archives
		where white_guest_id = $1 or black_guest_id = $1
		order by updated_at desc
		limit $2 offset $3
	`, guestID, limit, offset)
	if err != nil {
		return nil, err
	}
	return s.scanEntryRows(rows)
}

func (s *postgresArchiveStore) queryByAccount(accountID string, linkedGuestIDs []string, limit, offset int) ([]MatchArchiveEntry, error) {
	if strings.TrimSpace(accountID) == "" {
		return nil, nil
	}

	guestIDSet := make(map[string]struct{}, len(linkedGuestIDs))
	for _, gid := range linkedGuestIDs {
		if gid != "" {
			guestIDSet[gid] = struct{}{}
		}
	}

	args := []any{accountID, limit, offset}
	where := fmt.Sprintf("(white_account_id = $1 or black_account_id = $1)")

	if len(guestIDSet) > 0 {
		placeholders := make([]string, 0, len(guestIDSet))
		for gid := range guestIDSet {
			args = append(args, gid)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
		}
		guestIDs := strings.Join(placeholders, ", ")
		where = fmt.Sprintf("((%s) or white_guest_id in (%s) or black_guest_id in (%s))",
			where, guestIDs, guestIDs)
	}

	query := fmt.Sprintf(`
		select match_id, entry_json, private_json
		from archives
		where %s
		order by updated_at desc
		limit $2 offset $3
	`, where)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	return s.scanEntryRows(rows)
}

func (s *postgresArchiveStore) queryStats() (MatchArchiveStats, error) {
	var stats MatchArchiveStats
	row := s.db.QueryRow(`select count(*) from archives`)
	if err := row.Scan(&stats.TotalMatches); err != nil {
		return stats, err
	}

	row = s.db.QueryRow(`select count(*) from archives where status = 'finished'`)
	if err := row.Scan(&stats.FinishedMatches); err != nil {
		return stats, err
	}

	stats.ActiveMatches = stats.TotalMatches - stats.FinishedMatches

	row = s.db.QueryRow(`select count(*) from archives where queue = 'rated'`)
	if err := row.Scan(&stats.RatedMatches); err != nil {
		return stats, err
	}

	row = s.db.QueryRow(`select count(*) from archives where queue = 'casual'`)
	if err := row.Scan(&stats.CasualMatches); err != nil {
		return stats, err
	}

	stats.DirectMatches = stats.TotalMatches - stats.RatedMatches - stats.CasualMatches
	return stats, nil
}

func (s *postgresArchiveStore) persist(entries map[string]MatchArchiveEntry, private map[string]MatchArchivePrivateEntry) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	upsertStmt, err := tx.Prepare(`
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
		on conflict (match_id) do update set
			status = excluded.status,
			queue = excluded.queue,
			white_guest_id = excluded.white_guest_id,
			black_guest_id = excluded.black_guest_id,
			updated_at = excluded.updated_at,
			entry_json = excluded.entry_json,
			private_json = excluded.private_json
	`)
	if err != nil {
		return err
	}
	defer upsertStmt.Close()

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

		if _, err := upsertStmt.Exec(
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
