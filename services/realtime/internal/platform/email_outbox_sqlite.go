package platform

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteAccountEmailOutboxStore struct {
	db *sql.DB
}

func NewSQLiteAccountEmailOutboxStore(path string) (*SQLiteAccountEmailOutboxStore, error) {
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
	store := &SQLiteAccountEmailOutboxStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteAccountEmailOutboxStore) init() error {
	if s == nil || s.db == nil {
		return os.ErrInvalid
	}
	_, _ = s.db.Exec(`PRAGMA journal_mode=WAL`)
	_, _ = s.db.Exec(`PRAGMA busy_timeout=5000`)
	if _, err := s.db.Exec(`
		create table if not exists account_email_outbox (
			delivery_id text primary key,
			account_id text not null,
			email text not null,
			kind text not null,
			subject text not null,
			text_body text not null,
			html_body text not null,
			action_url text not null,
			status text not null,
			provider text not null default '',
			provider_message_id text not null default '',
			attempt_count integer not null default 0,
			last_attempt_at text null,
			next_attempt_at text null,
			delivered_at text null,
			failed_at text null,
			failure_reason text not null default '',
			created_at text not null,
			updated_at text not null
		);
		create index if not exists account_email_outbox_account_idx on account_email_outbox (account_id, updated_at desc, created_at desc, delivery_id asc);
		create index if not exists account_email_outbox_status_idx on account_email_outbox (status, next_attempt_at asc, updated_at desc, created_at desc, delivery_id asc);
	`); err != nil {
		return err
	}
	for _, statement := range []string{
		`alter table account_email_outbox add column provider text not null default ''`,
		`alter table account_email_outbox add column provider_message_id text not null default ''`,
		`alter table account_email_outbox add column attempt_count integer not null default 0`,
		`alter table account_email_outbox add column last_attempt_at text null`,
		`alter table account_email_outbox add column next_attempt_at text null`,
		`alter table account_email_outbox add column delivered_at text null`,
		`alter table account_email_outbox add column failed_at text null`,
		`alter table account_email_outbox add column failure_reason text not null default ''`,
	} {
		if _, err := s.db.Exec(statement); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
			return err
		}
	}
	return nil
}

func (s *SQLiteAccountEmailOutboxStore) Backend() string { return "sqlite" }

func (s *SQLiteAccountEmailOutboxStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteAccountEmailOutboxStore) QueueDelivery(request AccountEmailDeliveryRequest) (AccountEmailDelivery, error) {
	delivery, err := normalizeAccountEmailDeliveryRequest(request)
	if err != nil {
		return AccountEmailDelivery{}, err
	}
	_, err = s.db.Exec(
		`insert into account_email_outbox(delivery_id, account_id, email, kind, subject, text_body, html_body, action_url, status, provider, provider_message_id, attempt_count, last_attempt_at, next_attempt_at, delivered_at, failed_at, failure_reason, created_at, updated_at) values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		delivery.DeliveryID,
		delivery.AccountID,
		delivery.Email,
		delivery.Kind,
		delivery.Subject,
		delivery.TextBody,
		delivery.HTMLBody,
		delivery.ActionURL,
		delivery.Status,
		delivery.Provider,
		delivery.ProviderMessageID,
		delivery.AttemptCount,
		nullableTimeString(delivery.LastAttemptAt),
		nullableTimeString(delivery.NextAttemptAt),
		nullableTimeString(delivery.DeliveredAt),
		nullableTimeString(delivery.FailedAt),
		delivery.FailureReason,
		timeString(delivery.CreatedAt),
		timeString(delivery.UpdatedAt),
	)
	if err != nil {
		return AccountEmailDelivery{}, err
	}
	return delivery, nil
}

func (s *SQLiteAccountEmailOutboxStore) ListOverview(accountID string, limit int) AccountEmailDeliveryOverview {
	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return AccountEmailDeliveryOverview{Deliveries: make([]AccountEmailDelivery, 0)}
	}
	resolvedLimit := normalizeAccountEmailDeliveryLimit(limit, 12)
	rows, err := s.db.Query(
		`select delivery_id, account_id, email, kind, subject, text_body, html_body, action_url, status, provider, provider_message_id, attempt_count, last_attempt_at, next_attempt_at, delivered_at, failed_at, failure_reason, created_at, updated_at
		from account_email_outbox where account_id = ? order by updated_at desc, created_at desc, delivery_id asc limit ?`,
		resolvedAccountID,
		resolvedLimit,
	)
	if err != nil {
		return AccountEmailDeliveryOverview{Deliveries: make([]AccountEmailDelivery, 0)}
	}
	defer rows.Close()
	items := make([]AccountEmailDelivery, 0)
	for rows.Next() {
		delivery, scanErr := scanSQLiteAccountEmailDelivery(rows)
		if scanErr != nil {
			continue
		}
		items = append(items, delivery)
	}
	return AccountEmailDeliveryOverview{Deliveries: items}
}

func (s *SQLiteAccountEmailOutboxStore) ListPendingDeliveries(limit int, now time.Time) []AccountEmailDelivery {
	resolvedLimit := normalizeAccountEmailDeliveryLimit(limit, 16)
	rows, err := s.db.Query(
		`select delivery_id, account_id, email, kind, subject, text_body, html_body, action_url, status, provider, provider_message_id, attempt_count, last_attempt_at, next_attempt_at, delivered_at, failed_at, failure_reason, created_at, updated_at
		from account_email_outbox
		where status = ? and (next_attempt_at is null or next_attempt_at = '' or next_attempt_at <= ?)
		order by coalesce(nullif(next_attempt_at, ''), created_at) asc, created_at asc, delivery_id asc
		limit ?`,
		AccountEmailDeliveryStatusQueued,
		timeString(now.UTC()),
		resolvedLimit,
	)
	if err != nil {
		return make([]AccountEmailDelivery, 0)
	}
	defer rows.Close()
	items := make([]AccountEmailDelivery, 0)
	for rows.Next() {
		delivery, scanErr := scanSQLiteAccountEmailDelivery(rows)
		if scanErr != nil {
			continue
		}
		items = append(items, delivery)
	}
	return items
}

func (s *SQLiteAccountEmailOutboxStore) RecordDeliveryResult(request AccountEmailDeliveryResultRequest) (AccountEmailDelivery, error) {
	resolved, err := normalizeAccountEmailDeliveryResultRequest(request)
	if err != nil {
		return AccountEmailDelivery{}, err
	}
	delivery, err := s.getDelivery(resolved.DeliveryID)
	if err != nil {
		return AccountEmailDelivery{}, err
	}
	delivery = normalizePersistedAccountEmailDelivery(delivery)
	delivery.AttemptCount++
	delivery.UpdatedAt = resolved.AttemptedAt.UTC()
	delivery.Provider = firstNonEmpty(resolved.Provider, delivery.Provider)
	lastAttemptAt := resolved.AttemptedAt.UTC()
	delivery.LastAttemptAt = &lastAttemptAt
	if resolved.Delivered {
		delivery.Status = AccountEmailDeliveryStatusDelivered
		delivery.ProviderMessageID = resolved.ProviderMessageID
		delivery.FailureReason = ""
		delivery.NextAttemptAt = nil
		delivery.FailedAt = nil
		deliveredAt := resolved.AttemptedAt.UTC()
		delivery.DeliveredAt = &deliveredAt
	} else if resolved.TerminalFailure {
		delivery.Status = AccountEmailDeliveryStatusFailed
		delivery.ProviderMessageID = ""
		delivery.FailureReason = resolved.FailureReason
		delivery.NextAttemptAt = nil
		delivery.DeliveredAt = nil
		failedAt := resolved.AttemptedAt.UTC()
		delivery.FailedAt = &failedAt
	} else {
		delivery.Status = AccountEmailDeliveryStatusQueued
		delivery.ProviderMessageID = ""
		delivery.FailureReason = resolved.FailureReason
		delivery.DeliveredAt = nil
		delivery.FailedAt = nil
		if resolved.NextAttemptAt != nil {
			nextAttemptAt := resolved.NextAttemptAt.UTC()
			delivery.NextAttemptAt = &nextAttemptAt
		} else {
			delivery.NextAttemptAt = nil
		}
	}
	if _, err := s.db.Exec(
		`update account_email_outbox set status = ?, provider = ?, provider_message_id = ?, attempt_count = ?, last_attempt_at = ?, next_attempt_at = ?, delivered_at = ?, failed_at = ?, failure_reason = ?, updated_at = ? where delivery_id = ?`,
		delivery.Status,
		delivery.Provider,
		delivery.ProviderMessageID,
		delivery.AttemptCount,
		nullableTimeString(delivery.LastAttemptAt),
		nullableTimeString(delivery.NextAttemptAt),
		nullableTimeString(delivery.DeliveredAt),
		nullableTimeString(delivery.FailedAt),
		delivery.FailureReason,
		timeString(delivery.UpdatedAt),
		delivery.DeliveryID,
	); err != nil {
		return AccountEmailDelivery{}, err
	}
	return delivery, nil
}

func (s *SQLiteAccountEmailOutboxStore) Stats() AccountEmailDeliveryStoreStats {
	var stats AccountEmailDeliveryStoreStats
	if err := s.db.QueryRow(`select count(*) from account_email_outbox`).Scan(&stats.DeliveryCount); err != nil {
		return AccountEmailDeliveryStoreStats{}
	}
	if err := s.db.QueryRow(`select count(*) from account_email_outbox where status = ?`, AccountEmailDeliveryStatusQueued).Scan(&stats.QueuedCount); err != nil {
		return AccountEmailDeliveryStoreStats{}
	}
	if err := s.db.QueryRow(`select count(*) from account_email_outbox where status = ?`, AccountEmailDeliveryStatusDelivered).Scan(&stats.DeliveredCount); err != nil {
		return AccountEmailDeliveryStoreStats{}
	}
	if err := s.db.QueryRow(`select count(*) from account_email_outbox where status = ?`, AccountEmailDeliveryStatusFailed).Scan(&stats.FailedCount); err != nil {
		return AccountEmailDeliveryStoreStats{}
	}
	return stats
}

func (s *SQLiteAccountEmailOutboxStore) getDelivery(deliveryID string) (AccountEmailDelivery, error) {
	row := s.db.QueryRow(
		`select delivery_id, account_id, email, kind, subject, text_body, html_body, action_url, status, provider, provider_message_id, attempt_count, last_attempt_at, next_attempt_at, delivered_at, failed_at, failure_reason, created_at, updated_at
		from account_email_outbox where delivery_id = ?`,
		strings.TrimSpace(deliveryID),
	)
	delivery, err := scanSQLiteAccountEmailDelivery(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AccountEmailDelivery{}, ErrAccountEmailDeliveryNotFound
		}
		return AccountEmailDelivery{}, err
	}
	return delivery, nil
}

type sqliteAccountEmailOutboxScanner interface {
	Scan(dest ...any) error
}

func scanSQLiteAccountEmailDelivery(scanner sqliteAccountEmailOutboxScanner) (AccountEmailDelivery, error) {
	var delivery AccountEmailDelivery
	var createdAt, updatedAt string
	var lastAttemptAt, nextAttemptAt, deliveredAt, failedAt sql.NullString
	if err := scanner.Scan(
		&delivery.DeliveryID,
		&delivery.AccountID,
		&delivery.Email,
		&delivery.Kind,
		&delivery.Subject,
		&delivery.TextBody,
		&delivery.HTMLBody,
		&delivery.ActionURL,
		&delivery.Status,
		&delivery.Provider,
		&delivery.ProviderMessageID,
		&delivery.AttemptCount,
		&lastAttemptAt,
		&nextAttemptAt,
		&deliveredAt,
		&failedAt,
		&delivery.FailureReason,
		&createdAt,
		&updatedAt,
	); err != nil {
		return AccountEmailDelivery{}, err
	}
	var err error
	delivery.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return AccountEmailDelivery{}, err
	}
	delivery.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return AccountEmailDelivery{}, err
	}
	delivery.LastAttemptAt, err = parseNullableTimeString(lastAttemptAt)
	if err != nil {
		return AccountEmailDelivery{}, err
	}
	delivery.NextAttemptAt, err = parseNullableTimeString(nextAttemptAt)
	if err != nil {
		return AccountEmailDelivery{}, err
	}
	delivery.DeliveredAt, err = parseNullableTimeString(deliveredAt)
	if err != nil {
		return AccountEmailDelivery{}, err
	}
	delivery.FailedAt, err = parseNullableTimeString(failedAt)
	if err != nil {
		return AccountEmailDelivery{}, err
	}
	return normalizePersistedAccountEmailDelivery(delivery), nil
}

func nullableTimeString(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return timeString(value.UTC())
}

func parseNullableTimeString(value sql.NullString) (*time.Time, error) {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

var _ AccountEmailOutboxDirectory = (*SQLiteAccountEmailOutboxStore)(nil)
