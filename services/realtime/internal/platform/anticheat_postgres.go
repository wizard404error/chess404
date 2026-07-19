package platform

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresAnticheatStore struct {
	db *sql.DB
}

func NewPostgresAnticheatStore(rawURL string) (*PostgresAnticheatStore, error) {
	resolvedURL := strings.TrimSpace(rawURL)
	if resolvedURL == "" {
		return nil, os.ErrInvalid
	}
	db, err := sql.Open("pgx", resolvedURL)
	if err != nil {
		return nil, err
	}
	configurePostgresPool(db, 15, 3)
	return NewPostgresAnticheatStoreWithDB(db)
}

func NewPostgresAnticheatStoreWithDB(db *sql.DB) (*PostgresAnticheatStore, error) {
	store := &PostgresAnticheatStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *PostgresAnticheatStore) init() error {
	if s == nil || s.db == nil {
		return os.ErrInvalid
	}
	_, err := s.db.Exec(`
		create table if not exists anticheat_analyses (
			analysis_id text primary key,
			match_id text not null,
			player_id text not null,
			mode_id text not null,
			accuracy double precision not null,
			avg_cpl double precision not null,
			max_cpl integer not null,
			move_count integer not null,
			card_moves integer not null,
			flags_json text not null,
			time_profile_json text not null,
			suspicion_score double precision not null,
			analyzed_at timestamptz not null
		);
		create index if not exists anticheat_analyses_player_idx on anticheat_analyses (player_id, analyzed_at desc);
		create index if not exists anticheat_analyses_match_idx on anticheat_analyses (match_id);
		create index if not exists anticheat_analyses_score_idx on anticheat_analyses (suspicion_score desc);

		create table if not exists anticheat_player_summaries (
			player_id text primary key,
			total_games integer not null,
			avg_accuracy double precision not null,
			avg_cpl double precision not null,
			longest_win_streak integer not null,
			current_win_streak integer not null,
			rating_gain_20 integer not null,
			rating_gain_50 integer not null,
			suspicion_score double precision not null,
			recent_analyses_json text not null,
			action_taken text not null,
			last_analyzed_at timestamptz not null
		);
		create index if not exists anticheat_player_summaries_score_idx on anticheat_player_summaries (suspicion_score desc);
	`)
	return err
}

func (s *PostgresAnticheatStore) Backend() string { return "postgres" }

func (s *PostgresAnticheatStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *PostgresAnticheatStore) RecordAnalysis(record AnticheatAnalysisRecord) (AnticheatAnalysisRecord, error) {
	if strings.TrimSpace(record.AnalysisID) == "" {
		record.AnalysisID = "ach_" + randomToken(10)
	}
	if record.AnalyzedAt.IsZero() {
		record.AnalyzedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(
		`insert into anticheat_analyses (
			analysis_id, match_id, player_id, mode_id,
			accuracy, avg_cpl, max_cpl, move_count, card_moves,
			flags_json, time_profile_json, suspicion_score, analyzed_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		on conflict (analysis_id) do nothing`,
		record.AnalysisID, record.MatchID, record.PlayerID, record.ModeID,
		record.Accuracy, record.AvgCPL, record.MaxCPL, record.MoveCount, record.CardMoves,
		record.FlagsJSON, record.TimeProfileJSON, record.SuspicionScore, record.AnalyzedAt,
	)
	if err != nil {
		return AnticheatAnalysisRecord{}, err
	}
	return record, nil
}

func (s *PostgresAnticheatStore) UpsertPlayerSummary(summary AnticheatPlayerSummary) (AnticheatPlayerSummary, error) {
	if summary.LastAnalyzedAt.IsZero() {
		summary.LastAnalyzedAt = time.Now().UTC()
	}
	recentJSON, err := json.Marshal(summary.RecentAnalyses)
	if err != nil {
		return AnticheatPlayerSummary{}, err
	}
	_, err = s.db.Exec(
		`insert into anticheat_player_summaries (
			player_id, total_games, avg_accuracy, avg_cpl,
			longest_win_streak, current_win_streak,
			rating_gain_20, rating_gain_50,
			suspicion_score, recent_analyses_json, action_taken, last_analyzed_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		on conflict (player_id) do update set
			total_games = excluded.total_games,
			avg_accuracy = excluded.avg_accuracy,
			avg_cpl = excluded.avg_cpl,
			longest_win_streak = excluded.longest_win_streak,
			current_win_streak = excluded.current_win_streak,
			rating_gain_20 = excluded.rating_gain_20,
			rating_gain_50 = excluded.rating_gain_50,
			suspicion_score = excluded.suspicion_score,
			recent_analyses_json = excluded.recent_analyses_json,
			action_taken = excluded.action_taken,
			last_analyzed_at = excluded.last_analyzed_at`,
		summary.PlayerID, summary.TotalGames, summary.AvgAccuracy, summary.AvgCPL,
		summary.LongestWinStreak, summary.CurrentWinStreak,
		summary.RatingGain20, summary.RatingGain50,
		summary.SuspicionScore, string(recentJSON), summary.ActionTaken, summary.LastAnalyzedAt,
	)
	if err != nil {
		return AnticheatPlayerSummary{}, err
	}
	return summary, nil
}

func (s *PostgresAnticheatStore) ListFlaggedPlayers(minScore float64, limit int) []AnticheatPlayerSummary {
	rows, err := s.db.Query(
		`select player_id, total_games, avg_accuracy, avg_cpl,
				longest_win_streak, current_win_streak,
				rating_gain_20, rating_gain_50,
				suspicion_score, recent_analyses_json, action_taken, last_analyzed_at
		from anticheat_player_summaries
		where suspicion_score >= $1
		order by suspicion_score desc
		limit $2`,
		minScore, limit,
	)
	if err != nil {
		return make([]AnticheatPlayerSummary, 0)
	}
	defer rows.Close()
	out := make([]AnticheatPlayerSummary, 0)
	for rows.Next() {
		summary, err := scanPostgresAnticheatPlayerSummary(rows)
		if err != nil {
			continue
		}
		out = append(out, summary)
	}
	return out
}

func (s *PostgresAnticheatStore) GetPlayerSummary(playerID string) (AnticheatPlayerSummary, bool) {
	resolvedPlayerID := strings.TrimSpace(playerID)
	if resolvedPlayerID == "" {
		return AnticheatPlayerSummary{}, false
	}
	row := s.db.QueryRow(
		`select player_id, total_games, avg_accuracy, avg_cpl,
				longest_win_streak, current_win_streak,
				rating_gain_20, rating_gain_50,
				suspicion_score, recent_analyses_json, action_taken, last_analyzed_at
		from anticheat_player_summaries where player_id = $1`,
		resolvedPlayerID,
	)
	summary, err := scanPostgresAnticheatPlayerSummary(row)
	if err != nil {
		return AnticheatPlayerSummary{}, false
	}
	return summary, true
}

func (s *PostgresAnticheatStore) ListPlayerAnalyses(playerID string, limit int) []AnticheatAnalysisRecord {
	resolvedPlayerID := strings.TrimSpace(playerID)
	rows, err := s.db.Query(
		`select analysis_id, match_id, player_id, mode_id,
				accuracy, avg_cpl, max_cpl, move_count, card_moves,
				flags_json, time_profile_json, suspicion_score, analyzed_at
		from anticheat_analyses
		where player_id = $1
		order by analyzed_at desc
		limit $2`,
		resolvedPlayerID, limit,
	)
	if err != nil {
		return make([]AnticheatAnalysisRecord, 0)
	}
	defer rows.Close()
	out := make([]AnticheatAnalysisRecord, 0)
	for rows.Next() {
		record, err := scanPostgresAnticheatAnalysis(rows)
		if err != nil {
			continue
		}
		out = append(out, record)
	}
	return out
}

func (s *PostgresAnticheatStore) Stats() AnticheatStats {
	stats := AnticheatStats{}
	if err := s.db.QueryRow(`select count(*) from anticheat_analyses`).Scan(&stats.AnalysisCount); err != nil {
		stats.AnalysisCount = 0
	}
	if err := s.db.QueryRow(`select count(*) from anticheat_player_summaries`).Scan(&stats.PlayerCount); err != nil {
		stats.PlayerCount = 0
	}
	if err := s.db.QueryRow(
		`select count(*) from anticheat_player_summaries where suspicion_score >= $1`,
		AnticheatSuspicionThreshold,
	).Scan(&stats.FlaggedPlayerCount); err != nil {
		stats.FlaggedPlayerCount = 0
	}
	return stats
}

func (s *PostgresAnticheatStore) PruneAnalysesOlderThan(cutoff time.Time) (int64, error) {
	if s == nil || s.db == nil {
		return 0, os.ErrInvalid
	}
	result, err := s.db.Exec(`delete from anticheat_analyses where analyzed_at < $1`, cutoff.UTC())
	if err != nil {
		return 0, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return rows, nil
}

type postgresAnticheatScanner interface {
	Scan(dest ...any) error
}

func scanPostgresAnticheatAnalysis(scanner postgresAnticheatScanner) (AnticheatAnalysisRecord, error) {
	var record AnticheatAnalysisRecord
	if err := scanner.Scan(
		&record.AnalysisID, &record.MatchID, &record.PlayerID, &record.ModeID,
		&record.Accuracy, &record.AvgCPL, &record.MaxCPL, &record.MoveCount, &record.CardMoves,
		&record.FlagsJSON, &record.TimeProfileJSON, &record.SuspicionScore, &record.AnalyzedAt,
	); err != nil {
		return AnticheatAnalysisRecord{}, err
	}
	record.AnalyzedAt = record.AnalyzedAt.UTC()
	return record, nil
}

func scanPostgresAnticheatPlayerSummary(scanner postgresAnticheatScanner) (AnticheatPlayerSummary, error) {
	var (
		summary     AnticheatPlayerSummary
		recentJSON  string
	)
	if err := scanner.Scan(
		&summary.PlayerID, &summary.TotalGames, &summary.AvgAccuracy, &summary.AvgCPL,
		&summary.LongestWinStreak, &summary.CurrentWinStreak,
		&summary.RatingGain20, &summary.RatingGain50,
		&summary.SuspicionScore, &recentJSON, &summary.ActionTaken, &summary.LastAnalyzedAt,
	); err != nil {
		return AnticheatPlayerSummary{}, err
	}
	summary.LastAnalyzedAt = summary.LastAnalyzedAt.UTC()
	if strings.TrimSpace(recentJSON) != "" {
		_ = json.Unmarshal([]byte(recentJSON), &summary.RecentAnalyses)
	}
	if summary.RecentAnalyses == nil {
		summary.RecentAnalyses = make([]string, 0)
	}
	return summary, nil
}

var _ AnticheatStore = (*PostgresAnticheatStore)(nil)
var _ AnticheatStore = (*inMemoryAnticheatStore)(nil)

var ErrAnticheatInvalid = errors.New("invalid anticheat record")
