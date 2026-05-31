package platform

import (
	"database/sql"
	"errors"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresAccountEmailOutboxStore struct {
	db *sql.DB
}

func NewPostgresAccountEmailOutboxStore(rawURL string) (*PostgresAccountEmailOutboxStore, error) {
	resolvedURL := strings.TrimSpace(rawURL)
	if resolvedURL == "" {
		return nil, os.ErrInvalid
	}
	db, err := sql.Open("pgx", resolvedURL)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	store := &PostgresAccountEmailOutboxStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *PostgresAccountEmailOutboxStore) init() error {
	if s == nil || s.db == nil {
		return os.ErrInvalid
	}
	_, err := s.db.Exec(`
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
			last_attempt_at timestamptz null,
			next_attempt_at timestamptz null,
			delivered_at timestamptz null,
			failed_at timestamptz null,
			failure_reason text not null default '',
			created_at timestamptz not null,
			updated_at timestamptz not null
		);
		alter table account_email_outbox add column if not exists provider text not null default '';
		alter table account_email_outbox add column if not exists provider_message_id text not null default '';
		alter table account_email_outbox add column if not exists attempt_count integer not null default 0;
		alter table account_email_outbox add column if not exists last_attempt_at timestamptz null;
		alter table account_email_outbox add column if not exists next_attempt_at timestamptz null;
		alter table account_email_outbox add column if not exists delivered_at timestamptz null;
		alter table account_email_outbox add column if not exists failed_at timestamptz null;
		alter table account_email_outbox add column if not exists failure_reason text not null default '';
		create index if not exists account_email_outbox_account_idx on account_email_outbox (account_id, updated_at desc, created_at desc, delivery_id asc);
		create index if not exists account_email_outbox_status_idx on account_email_outbox (status, next_attempt_at asc, updated_at desc, created_at desc, delivery_id asc);
	`)
	return err
}

func (s *PostgresAccountEmailOutboxStore) Backend() string { return "postgres" }

func (s *PostgresAccountEmailOutboxStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *PostgresAccountEmailOutboxStore) QueueDelivery(request AccountEmailDeliveryRequest) (AccountEmailDelivery, error) {
	delivery, err := normalizeAccountEmailDeliveryRequest(request)
	if err != nil {
		return AccountEmailDelivery{}, err
	}
	_, err = s.db.Exec(
		`insert into account_email_outbox(delivery_id, account_id, email, kind, subject, text_body, html_body, action_url, status, provider, provider_message_id, attempt_count, last_attempt_at, next_attempt_at, delivered_at, failed_at, failure_reason, created_at, updated_at) values($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)`,
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
		nil,
		nil,
		nil,
		nil,
		delivery.FailureReason,
		delivery.CreatedAt,
		delivery.UpdatedAt,
	)
	if err != nil {
		return AccountEmailDelivery{}, err
	}
	return delivery, nil
}

func (s *PostgresAccountEmailOutboxStore) ListOverview(accountID string, limit int) AccountEmailDeliveryOverview {
	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return AccountEmailDeliveryOverview{Deliveries: make([]AccountEmailDelivery, 0)}
	}
	resolvedLimit := normalizeAccountEmailDeliveryLimit(limit, 12)
	rows, err := s.db.Query(
		`select delivery_id, account_id, email, kind, subject, text_body, html_body, action_url, status, provider, provider_message_id, attempt_count, last_attempt_at, next_attempt_at, delivered_at, failed_at, failure_reason, created_at, updated_at
		from account_email_outbox where account_id = $1 order by updated_at desc, created_at desc, delivery_id asc limit $2`,
		resolvedAccountID,
		resolvedLimit,
	)
	if err != nil {
		return AccountEmailDeliveryOverview{Deliveries: make([]AccountEmailDelivery, 0)}
	}
	defer rows.Close()
	items := make([]AccountEmailDelivery, 0)
	for rows.Next() {
		delivery, scanErr := scanPostgresAccountEmailDelivery(rows)
		if scanErr != nil {
			continue
		}
		items = append(items, delivery)
	}
	return AccountEmailDeliveryOverview{Deliveries: items}
}

func (s *PostgresAccountEmailOutboxStore) ListPendingDeliveries(limit int, now time.Time) []AccountEmailDelivery {
	resolvedLimit := normalizeAccountEmailDeliveryLimit(limit, 16)
	rows, err := s.db.Query(
		`select delivery_id, account_id, email, kind, subject, text_body, html_body, action_url, status, provider, provider_message_id, attempt_count, last_attempt_at, next_attempt_at, delivered_at, failed_at, failure_reason, created_at, updated_at
		from account_email_outbox
		where status = $1 and (next_attempt_at is null or next_attempt_at <= $2)
		order by coalesce(next_attempt_at, created_at) asc, created_at asc, delivery_id asc
		limit $3`,
		AccountEmailDeliveryStatusQueued,
		now.UTC(),
		resolvedLimit,
	)
	if err != nil {
		return make([]AccountEmailDelivery, 0)
	}
	defer rows.Close()
	items := make([]AccountEmailDelivery, 0)
	for rows.Next() {
		delivery, scanErr := scanPostgresAccountEmailDelivery(rows)
		if scanErr != nil {
			continue
		}
		items = append(items, delivery)
	}
	return items
}

func (s *PostgresAccountEmailOutboxStore) RecordDeliveryResult(request AccountEmailDeliveryResultRequest) (AccountEmailDelivery, error) {
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
		`update account_email_outbox set status = $1, provider = $2, provider_message_id = $3, attempt_count = $4, last_attempt_at = $5, next_attempt_at = $6, delivered_at = $7, failed_at = $8, failure_reason = $9, updated_at = $10 where delivery_id = $11`,
		delivery.Status,
		delivery.Provider,
		delivery.ProviderMessageID,
		delivery.AttemptCount,
		delivery.LastAttemptAt,
		delivery.NextAttemptAt,
		delivery.DeliveredAt,
		delivery.FailedAt,
		delivery.FailureReason,
		delivery.UpdatedAt,
		delivery.DeliveryID,
	); err != nil {
		return AccountEmailDelivery{}, err
	}
	return delivery, nil
}

func (s *PostgresAccountEmailOutboxStore) Stats() AccountEmailDeliveryStoreStats {
	var stats AccountEmailDeliveryStoreStats
	if err := s.db.QueryRow(`select count(*) from account_email_outbox`).Scan(&stats.DeliveryCount); err != nil {
		return AccountEmailDeliveryStoreStats{}
	}
	if err := s.db.QueryRow(`select count(*) from account_email_outbox where status = $1`, AccountEmailDeliveryStatusQueued).Scan(&stats.QueuedCount); err != nil {
		return AccountEmailDeliveryStoreStats{}
	}
	if err := s.db.QueryRow(`select count(*) from account_email_outbox where status = $1`, AccountEmailDeliveryStatusDelivered).Scan(&stats.DeliveredCount); err != nil {
		return AccountEmailDeliveryStoreStats{}
	}
	if err := s.db.QueryRow(`select count(*) from account_email_outbox where status = $1`, AccountEmailDeliveryStatusFailed).Scan(&stats.FailedCount); err != nil {
		return AccountEmailDeliveryStoreStats{}
	}
	return stats
}

func (s *PostgresAccountEmailOutboxStore) getDelivery(deliveryID string) (AccountEmailDelivery, error) {
	row := s.db.QueryRow(
		`select delivery_id, account_id, email, kind, subject, text_body, html_body, action_url, status, provider, provider_message_id, attempt_count, last_attempt_at, next_attempt_at, delivered_at, failed_at, failure_reason, created_at, updated_at
		from account_email_outbox where delivery_id = $1`,
		strings.TrimSpace(deliveryID),
	)
	delivery, err := scanPostgresAccountEmailDelivery(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AccountEmailDelivery{}, ErrAccountEmailDeliveryNotFound
		}
		return AccountEmailDelivery{}, err
	}
	return delivery, nil
}

type postgresAccountEmailOutboxScanner interface {
	Scan(dest ...any) error
}

func scanPostgresAccountEmailDelivery(scanner postgresAccountEmailOutboxScanner) (AccountEmailDelivery, error) {
	var delivery AccountEmailDelivery
	var lastAttemptAt, nextAttemptAt, deliveredAt, failedAt sql.NullTime
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
		&delivery.CreatedAt,
		&delivery.UpdatedAt,
	); err != nil {
		return AccountEmailDelivery{}, err
	}
	delivery.CreatedAt = delivery.CreatedAt.UTC()
	delivery.UpdatedAt = delivery.UpdatedAt.UTC()
	delivery.LastAttemptAt = nullTimePointer(lastAttemptAt)
	delivery.NextAttemptAt = nullTimePointer(nextAttemptAt)
	delivery.DeliveredAt = nullTimePointer(deliveredAt)
	delivery.FailedAt = nullTimePointer(failedAt)
	return normalizePersistedAccountEmailDelivery(delivery), nil
}

func nullTimePointer(value sql.NullTime) *time.Time {
	if !value.Valid || value.Time.IsZero() {
		return nil
	}
	resolved := value.Time.UTC()
	return &resolved
}

var _ AccountEmailOutboxDirectory = (*PostgresAccountEmailOutboxStore)(nil)
