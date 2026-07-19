package platform

import (
	"database/sql"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type postgresModerationStore struct {
	db *sql.DB
}

func NewPostgresModerationStore(dsn string) (*ModerationStore, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	configurePostgresPool(db, 25, 5)
	return NewPostgresModerationStoreWithDB(db)
}

func NewPostgresModerationStoreWithDB(db *sql.DB) (*ModerationStore, error) {
	store, err := newPostgresModerationStoreWithDB(db)
	if err != nil {
		return nil, err
	}
	return NewModerationStoreFromDB(store)
}

func newPostgresModerationStoreWithDB(db *sql.DB) (*postgresModerationStore, error) {
	store := &postgresModerationStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *postgresModerationStore) backend() string {
	return "postgres"
}

func (s *postgresModerationStore) load() (map[string]AccountBlock, map[string]PlayerReport, map[string]ModerationActionAudit, map[string]AccountRestriction, error) {
	blocks := make(map[string]AccountBlock)
	reports := make(map[string]PlayerReport)
	actions := make(map[string]ModerationActionAudit)
	restrictions := make(map[string]AccountRestriction)

	blockRows, err := s.db.Query(`select block_id, blocker_account_id, target_account_id, reason, created_at, updated_at from account_blocks`)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	defer blockRows.Close()
	for blockRows.Next() {
		var block AccountBlock
		if err := blockRows.Scan(&block.BlockID, &block.BlockerAccountID, &block.TargetAccountID, &block.Reason, &block.CreatedAt, &block.UpdatedAt); err != nil {
			return nil, nil, nil, nil, err
		}
		blocks[block.BlockID] = block
	}
	if err := blockRows.Err(); err != nil {
		return nil, nil, nil, nil, err
	}

	reportRows, err := s.db.Query(`select report_id, reporter_account_id, target_account_id, category, details, status, reviewed_by_account_id, reviewed_at, resolution_note, created_at, updated_at from player_reports`)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	defer reportRows.Close()
	for reportRows.Next() {
		var report PlayerReport
		var category string
		var reviewedByID sql.NullString
		var reviewedAt sql.NullTime
		var resolutionNote string
		if err := reportRows.Scan(&report.ReportID, &report.ReporterAccountID, &report.TargetAccountID, &category, &report.Details, &report.Status, &reviewedByID, &reviewedAt, &resolutionNote, &report.CreatedAt, &report.UpdatedAt); err != nil {
			return nil, nil, nil, nil, err
		}
		report.Category = PlayerReportCategory(category)
		report.ReviewedByAccountID = reviewedByID.String
		if reviewedAt.Valid {
			parsedReviewedAt := reviewedAt.Time.UTC()
			report.ReviewedAt = &parsedReviewedAt
		}
		report.ResolutionNote = resolutionNote
		reports[report.ReportID] = report
	}
	if err := reportRows.Err(); err != nil {
		return nil, nil, nil, nil, err
	}

	actionRows, err := s.db.Query(`select action_id, report_id, moderator_account_id, reporter_account_id, target_account_id, previous_status, next_status, action, note, created_at from moderation_actions`)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	defer actionRows.Close()
	for actionRows.Next() {
		var action ModerationActionAudit
		if err := actionRows.Scan(&action.ActionID, &action.ReportID, &action.ModeratorAccountID, &action.ReporterAccountID, &action.TargetAccountID, &action.PreviousStatus, &action.NextStatus, &action.Action, &action.Note, &action.CreatedAt); err != nil {
			return nil, nil, nil, nil, err
		}
		action.CreatedAt = action.CreatedAt.UTC()
		actions[action.ActionID] = action
	}
	if err := actionRows.Err(); err != nil {
		return nil, nil, nil, nil, err
	}

	restrictionRows, err := s.db.Query(`select account_id, restriction_id, kind, reason, report_id, applied_by_account_id, created_at, updated_at from account_restrictions`)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	defer restrictionRows.Close()
	for restrictionRows.Next() {
		var restriction AccountRestriction
		var reportID sql.NullString
		var appliedByAccountID sql.NullString
		if err := restrictionRows.Scan(&restriction.AccountID, &restriction.RestrictionID, &restriction.Kind, &restriction.Reason, &reportID, &appliedByAccountID, &restriction.CreatedAt, &restriction.UpdatedAt); err != nil {
			return nil, nil, nil, nil, err
		}
		restriction.CreatedAt = restriction.CreatedAt.UTC()
		restriction.UpdatedAt = restriction.UpdatedAt.UTC()
		if reportID.Valid {
			restriction.ReportID = reportID.String
		}
		if appliedByAccountID.Valid {
			restriction.AppliedByAccountID = appliedByAccountID.String
		}
		restrictions[restriction.AccountID] = restriction
	}
	if err := restrictionRows.Err(); err != nil {
		return nil, nil, nil, nil, err
	}

	return blocks, reports, actions, restrictions, nil
}

func (s *postgresModerationStore) persist(blocks map[string]AccountBlock, reports map[string]PlayerReport, actions map[string]ModerationActionAudit, restrictions map[string]AccountRestriction) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`delete from account_blocks`); err != nil {
		return err
	}
	if _, err := tx.Exec(`delete from player_reports`); err != nil {
		return err
	}
	if _, err := tx.Exec(`delete from moderation_actions`); err != nil {
		return err
	}
	if _, err := tx.Exec(`delete from account_restrictions`); err != nil {
		return err
	}

	for _, block := range blocks {
		if _, err := tx.Exec(
			`insert into account_blocks(block_id, blocker_account_id, target_account_id, reason, created_at, updated_at) values($1, $2, $3, $4, $5, $6)`,
			block.BlockID,
			block.BlockerAccountID,
			block.TargetAccountID,
			block.Reason,
			block.CreatedAt.UTC(),
			block.UpdatedAt.UTC(),
		); err != nil {
			return err
		}
	}
	for _, report := range reports {
		if _, err := tx.Exec(
			`insert into player_reports(report_id, reporter_account_id, target_account_id, category, details, status, reviewed_by_account_id, reviewed_at, resolution_note, created_at, updated_at) values($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			report.ReportID,
			report.ReporterAccountID,
			report.TargetAccountID,
			string(report.Category),
			report.Details,
			report.Status,
			nullString(report.ReviewedByAccountID),
			nullTime(report.ReviewedAt),
			report.ResolutionNote,
			report.CreatedAt.UTC(),
			report.UpdatedAt.UTC(),
		); err != nil {
			return err
		}
	}
	for _, action := range actions {
		if _, err := tx.Exec(
			`insert into moderation_actions(action_id, report_id, moderator_account_id, reporter_account_id, target_account_id, previous_status, next_status, action, note, created_at) values($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			action.ActionID,
			action.ReportID,
			action.ModeratorAccountID,
			action.ReporterAccountID,
			action.TargetAccountID,
			action.PreviousStatus,
			action.NextStatus,
			action.Action,
			action.Note,
			action.CreatedAt.UTC(),
		); err != nil {
			return err
		}
	}
	for _, restriction := range restrictions {
		if _, err := tx.Exec(
			`insert into account_restrictions(account_id, restriction_id, kind, reason, report_id, applied_by_account_id, created_at, updated_at) values($1, $2, $3, $4, $5, $6, $7, $8)`,
			restriction.AccountID,
			restriction.RestrictionID,
			restriction.Kind,
			restriction.Reason,
			nullString(restriction.ReportID),
			nullString(restriction.AppliedByAccountID),
			restriction.CreatedAt.UTC(),
			restriction.UpdatedAt.UTC(),
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *postgresModerationStore) close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *postgresModerationStore) init() error {
	_, err := s.db.Exec(`
		create table if not exists account_blocks (
			block_id text primary key,
			blocker_account_id text not null,
			target_account_id text not null,
			reason text not null,
			created_at timestamptz not null,
			updated_at timestamptz not null
		);
		create index if not exists account_blocks_blocker_idx on account_blocks (blocker_account_id);
		create index if not exists account_blocks_target_idx on account_blocks (target_account_id);
		create table if not exists player_reports (
			report_id text primary key,
			reporter_account_id text not null,
			target_account_id text not null,
			category text not null,
			details text not null,
			status text not null,
			reviewed_by_account_id text,
			reviewed_at timestamptz,
			resolution_note text not null default '',
			created_at timestamptz not null,
			updated_at timestamptz not null
		);
		create index if not exists player_reports_reporter_idx on player_reports (reporter_account_id);
		create index if not exists player_reports_target_idx on player_reports (target_account_id);
		create index if not exists player_reports_status_idx on player_reports (status);
		create table if not exists moderation_actions (
			action_id text primary key,
			report_id text not null,
			moderator_account_id text not null,
			reporter_account_id text not null,
			target_account_id text not null,
			previous_status text not null,
			next_status text not null,
			action text not null,
			note text not null default '',
			created_at timestamptz not null
		);
		create index if not exists moderation_actions_report_idx on moderation_actions (report_id);
		create index if not exists moderation_actions_moderator_idx on moderation_actions (moderator_account_id);
		create table if not exists account_restrictions (
			account_id text primary key,
			restriction_id text not null,
			kind text not null,
			reason text not null default '',
			report_id text,
			applied_by_account_id text,
			created_at timestamptz not null,
			updated_at timestamptz not null
		);
		create index if not exists account_restrictions_kind_idx on account_restrictions (kind);
	`)
	return err
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}
