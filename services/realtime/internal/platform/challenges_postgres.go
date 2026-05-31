package platform

import (
	"database/sql"
	"time"

	"github.com/chess404/realtime/internal/contracts"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type postgresDirectChallengeStore struct {
	db *sql.DB
}

func NewPostgresDirectChallengeStore(dsn string) (*DirectChallengeStore, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	store, err := newPostgresDirectChallengePersistenceWithDB(db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return newDirectChallengeStore(store)
}

func newPostgresDirectChallengePersistenceWithDB(db *sql.DB) (*postgresDirectChallengeStore, error) {
	store := &postgresDirectChallengeStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *postgresDirectChallengeStore) backend() string {
	return "postgres"
}

func (s *postgresDirectChallengeStore) load() (map[string]DirectChallenge, error) {
	challenges := make(map[string]DirectChallenge)
	rows, err := s.db.Query(`select challenge_id, challenger_account_id, target_account_id, match_id, mode_id, clock_seconds, challenger_seat, status, created_at, updated_at from direct_challenges`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var challenge DirectChallenge
		var modeID string
		if err := rows.Scan(&challenge.ChallengeID, &challenge.ChallengerAccountID, &challenge.TargetAccountID, &challenge.MatchID, &modeID, &challenge.ClockSeconds, &challenge.ChallengerSeat, &challenge.Status, &challenge.CreatedAt, &challenge.UpdatedAt); err != nil {
			return nil, err
		}
		challenge.ModeID = contracts.NormalizeMatchModeID(modeID)
		challenges[challenge.ChallengeID] = challenge
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return challenges, nil
}

func (s *postgresDirectChallengeStore) persist(challenges map[string]DirectChallenge) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`delete from direct_challenges`); err != nil {
		return err
	}
	for _, challenge := range challenges {
		if _, err := tx.Exec(
			`insert into direct_challenges(challenge_id, challenger_account_id, target_account_id, match_id, mode_id, clock_seconds, challenger_seat, status, created_at, updated_at) values($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			challenge.ChallengeID,
			challenge.ChallengerAccountID,
			challenge.TargetAccountID,
			challenge.MatchID,
			string(challenge.ModeID),
			challenge.ClockSeconds,
			challenge.ChallengerSeat,
			challenge.Status,
			challenge.CreatedAt.UTC(),
			challenge.UpdatedAt.UTC(),
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *postgresDirectChallengeStore) close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *postgresDirectChallengeStore) init() error {
	_, err := s.db.Exec(`
		create table if not exists direct_challenges (
			challenge_id text primary key,
			challenger_account_id text not null,
			target_account_id text not null,
			match_id text not null,
			mode_id text not null,
			clock_seconds bigint not null,
			challenger_seat text not null,
			status text not null,
			created_at timestamptz not null,
			updated_at timestamptz not null
		);
		create index if not exists direct_challenges_challenger_idx on direct_challenges (challenger_account_id);
		create index if not exists direct_challenges_target_idx on direct_challenges (target_account_id);
		create index if not exists direct_challenges_status_idx on direct_challenges (status);
	`)
	return err
}
