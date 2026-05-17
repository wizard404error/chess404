package platform

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteAccountSecurityAuditStore struct {
	db *sql.DB
}

func NewSQLiteAccountSecurityAuditStore(path string) (*SQLiteAccountSecurityAuditStore, error) {
	resolvedPath := strings.TrimSpace(path)
	if resolvedPath == "" {
		return nil, os.ErrInvalid
	}
	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", resolvedPath)
	if err != nil {
		return nil, err
	}
	store := &SQLiteAccountSecurityAuditStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteAccountSecurityAuditStore) init() error {
	if s == nil || s.db == nil {
		return os.ErrInvalid
	}
	_, err := s.db.Exec(`
		create table if not exists account_security_events (
			event_id text primary key,
			account_id text not null,
			kind text not null,
			detail text not null,
			created_at text not null
		);
		create index if not exists account_security_events_account_idx on account_security_events (account_id, created_at desc, event_id asc);
	`)
	return err
}

func (s *SQLiteAccountSecurityAuditStore) Backend() string { return "sqlite" }

func (s *SQLiteAccountSecurityAuditStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteAccountSecurityAuditStore) RecordEvent(request AccountSecurityEventRequest) (AccountSecurityEvent, error) {
	event, err := normalizeAccountSecurityEventRequest(request)
	if err != nil {
		return AccountSecurityEvent{}, err
	}
	_, err = s.db.Exec(
		`insert into account_security_events(event_id, account_id, kind, detail, created_at) values(?, ?, ?, ?, ?)`,
		event.EventID,
		event.AccountID,
		event.Kind,
		event.Detail,
		timeString(event.CreatedAt),
	)
	if err != nil {
		return AccountSecurityEvent{}, err
	}
	return event, nil
}

func (s *SQLiteAccountSecurityAuditStore) ListOverview(accountID string, limit int) AccountSecurityEventOverview {
	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return AccountSecurityEventOverview{Events: make([]AccountSecurityEvent, 0)}
	}
	resolvedLimit := limit
	if resolvedLimit <= 0 {
		resolvedLimit = 12
	}
	rows, err := s.db.Query(
		`select event_id, account_id, kind, detail, created_at from account_security_events where account_id = ? order by created_at desc, event_id asc limit ?`,
		resolvedAccountID,
		resolvedLimit,
	)
	if err != nil {
		return AccountSecurityEventOverview{Events: make([]AccountSecurityEvent, 0)}
	}
	defer rows.Close()
	items := make([]AccountSecurityEvent, 0)
	for rows.Next() {
		event, scanErr := scanSQLiteAccountSecurityEvent(rows)
		if scanErr != nil {
			continue
		}
		items = append(items, event)
	}
	return AccountSecurityEventOverview{Events: items}
}

func (s *SQLiteAccountSecurityAuditStore) Stats() AccountSecurityAuditStats {
	var stats AccountSecurityAuditStats
	if err := s.db.QueryRow(`select count(*) from account_security_events`).Scan(&stats.EventCount); err != nil {
		return AccountSecurityAuditStats{}
	}
	return stats
}

type sqliteAccountSecurityEventScanner interface {
	Scan(dest ...any) error
}

func scanSQLiteAccountSecurityEvent(scanner sqliteAccountSecurityEventScanner) (AccountSecurityEvent, error) {
	var event AccountSecurityEvent
	var createdAt string
	if err := scanner.Scan(&event.EventID, &event.AccountID, &event.Kind, &event.Detail, &createdAt); err != nil {
		return AccountSecurityEvent{}, err
	}
	parsed, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return AccountSecurityEvent{}, err
	}
	event.CreatedAt = parsed.UTC()
	return event, nil
}

var _ AccountSecurityAuditDirectory = (*SQLiteAccountSecurityAuditStore)(nil)
