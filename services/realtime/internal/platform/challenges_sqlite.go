package platform

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chess404/realtime/internal/contracts"
	_ "modernc.org/sqlite"
)

type sqliteDirectChallengeStore struct {
	db *sql.DB
}

func NewSQLiteDirectChallengeStore(path string) (*DirectChallengeStore, error) {
	store, err := newSQLiteDirectChallengePersistence(path)
	if err != nil {
		return nil, err
	}
	return newDirectChallengeStore(store)
}

func newSQLiteDirectChallengePersistence(path string) (*sqliteDirectChallengeStore, error) {
	if path != "" && path != ":memory:" && !strings.HasPrefix(path, "file:") {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	store := &sqliteDirectChallengeStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *sqliteDirectChallengeStore) backend() string {
	return "sqlite"
}

func (s *sqliteDirectChallengeStore) load() (map[string]DirectChallenge, error) {
	challenges := make(map[string]DirectChallenge)
	rows, err := s.db.Query(`select challenge_id, challenger_account_id, target_account_id, match_id, mode_id, clock_seconds, challenger_seat, status, created_at, updated_at from direct_challenges`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			challenge DirectChallenge
			modeID    string
			createdAt string
			updatedAt string
		)
		if err := rows.Scan(&challenge.ChallengeID, &challenge.ChallengerAccountID, &challenge.TargetAccountID, &challenge.MatchID, &modeID, &challenge.ClockSeconds, &challenge.ChallengerSeat, &challenge.Status, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, err
		}
		parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			return nil, err
		}
		challenge.ModeID = contracts.NormalizeMatchModeID(modeID)
		challenge.CreatedAt = parsedCreatedAt
		challenge.UpdatedAt = parsedUpdatedAt
		challenges[challenge.ChallengeID] = challenge
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return challenges, nil
}

func (s *sqliteDirectChallengeStore) persist(challenges map[string]DirectChallenge) error {
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
			`insert into direct_challenges(challenge_id, challenger_account_id, target_account_id, match_id, mode_id, clock_seconds, challenger_seat, status, created_at, updated_at) values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			challenge.ChallengeID,
			challenge.ChallengerAccountID,
			challenge.TargetAccountID,
			challenge.MatchID,
			string(challenge.ModeID),
			challenge.ClockSeconds,
			challenge.ChallengerSeat,
			challenge.Status,
			timeString(challenge.CreatedAt),
			timeString(challenge.UpdatedAt),
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *sqliteDirectChallengeStore) close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *sqliteDirectChallengeStore) init() error {
	_, err := s.db.Exec(`
		create table if not exists direct_challenges (
			challenge_id text primary key,
			challenger_account_id text not null,
			target_account_id text not null,
			match_id text not null,
			mode_id text not null,
			clock_seconds integer not null,
			challenger_seat text not null,
			status text not null,
			created_at text not null,
			updated_at text not null
		);
		create index if not exists direct_challenges_challenger_idx on direct_challenges (challenger_account_id);
		create index if not exists direct_challenges_target_idx on direct_challenges (target_account_id);
		create index if not exists direct_challenges_status_idx on direct_challenges (status);
	`)
	return err
}
