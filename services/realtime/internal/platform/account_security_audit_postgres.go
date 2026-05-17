package platform

import (
	"database/sql"
	"os"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresAccountSecurityAuditStore struct {
	db *sql.DB
}

func NewPostgresAccountSecurityAuditStore(rawURL string) (*PostgresAccountSecurityAuditStore, error) {
	resolvedURL := strings.TrimSpace(rawURL)
	if resolvedURL == "" {
		return nil, os.ErrInvalid
	}
	db, err := sql.Open("pgx", resolvedURL)
	if err != nil {
		return nil, err
	}
	store := &PostgresAccountSecurityAuditStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *PostgresAccountSecurityAuditStore) init() error {
	if s == nil || s.db == nil {
		return os.ErrInvalid
	}
	_, err := s.db.Exec(`
		create table if not exists account_security_events (
			event_id text primary key,
			account_id text not null,
			kind text not null,
			detail text not null,
			created_at timestamptz not null
		);
		create index if not exists account_security_events_account_idx on account_security_events (account_id, created_at desc, event_id asc);
	`)
	return err
}

func (s *PostgresAccountSecurityAuditStore) Backend() string { return "postgres" }

func (s *PostgresAccountSecurityAuditStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *PostgresAccountSecurityAuditStore) RecordEvent(request AccountSecurityEventRequest) (AccountSecurityEvent, error) {
	event, err := normalizeAccountSecurityEventRequest(request)
	if err != nil {
		return AccountSecurityEvent{}, err
	}
	_, err = s.db.Exec(
		`insert into account_security_events(event_id, account_id, kind, detail, created_at) values($1, $2, $3, $4, $5)`,
		event.EventID,
		event.AccountID,
		event.Kind,
		event.Detail,
		event.CreatedAt,
	)
	if err != nil {
		return AccountSecurityEvent{}, err
	}
	return event, nil
}

func (s *PostgresAccountSecurityAuditStore) ListOverview(accountID string, limit int) AccountSecurityEventOverview {
	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return AccountSecurityEventOverview{Events: make([]AccountSecurityEvent, 0)}
	}
	resolvedLimit := limit
	if resolvedLimit <= 0 {
		resolvedLimit = 12
	}
	rows, err := s.db.Query(
		`select event_id, account_id, kind, detail, created_at from account_security_events where account_id = $1 order by created_at desc, event_id asc limit $2`,
		resolvedAccountID,
		resolvedLimit,
	)
	if err != nil {
		return AccountSecurityEventOverview{Events: make([]AccountSecurityEvent, 0)}
	}
	defer rows.Close()
	items := make([]AccountSecurityEvent, 0)
	for rows.Next() {
		event, scanErr := scanPostgresAccountSecurityEvent(rows)
		if scanErr != nil {
			continue
		}
		items = append(items, event)
	}
	return AccountSecurityEventOverview{Events: items}
}

func (s *PostgresAccountSecurityAuditStore) Stats() AccountSecurityAuditStats {
	var stats AccountSecurityAuditStats
	if err := s.db.QueryRow(`select count(*) from account_security_events`).Scan(&stats.EventCount); err != nil {
		return AccountSecurityAuditStats{}
	}
	return stats
}

type postgresAccountSecurityEventScanner interface {
	Scan(dest ...any) error
}

func scanPostgresAccountSecurityEvent(scanner postgresAccountSecurityEventScanner) (AccountSecurityEvent, error) {
	var event AccountSecurityEvent
	if err := scanner.Scan(&event.EventID, &event.AccountID, &event.Kind, &event.Detail, &event.CreatedAt); err != nil {
		return AccountSecurityEvent{}, err
	}
	event.CreatedAt = event.CreatedAt.UTC()
	return event, nil
}

var _ AccountSecurityAuditDirectory = (*PostgresAccountSecurityAuditStore)(nil)
