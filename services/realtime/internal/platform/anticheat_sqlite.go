package platform

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type SqliteAnticheatStore struct {
	db *sql.DB
}

type sqliteAnticheatStoreFile struct {
	Analyses  map[string]AnticheatAnalysisRecord `json:"analyses"`
	Summaries map[string]AnticheatPlayerSummary   `json:"summaries"`
}

type fileSqliteAnticheatStore struct {
	mu   sync.Mutex
	path string
}

func NewSqliteAnticheatStore(path string) (*SqliteAnticheatStore, error) {
	resolved := strings.TrimSpace(path)
	if resolved == "" {
		return nil, os.ErrInvalid
	}
	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", resolved)
	if err != nil {
		return nil, err
	}
	store := &SqliteAnticheatStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SqliteAnticheatStore) init() error {
	if s == nil || s.db == nil {
		return os.ErrInvalid
	}
	_, err := s.db.Exec(`
		create table if not exists anticheat_analyses (
			analysis_id text primary key,
			match_id text not null,
			player_id text not null,
			mode_id text not null,
			accuracy real not null,
			avg_cpl real not null,
			max_cpl integer not null,
			move_count integer not null,
			card_moves integer not null,
			flags_json text not null,
			time_profile_json text not null,
			suspicion_score real not null,
			analyzed_at text not null
		);
		create index if not exists anticheat_analyses_player_idx on anticheat_analyses (player_id, analyzed_at desc);
		create index if not exists anticheat_analyses_match_idx on anticheat_analyses (match_id);
		create index if not exists anticheat_analyses_score_idx on anticheat_analyses (suspicion_score desc);

		create table if not exists anticheat_player_summaries (
			player_id text primary key,
			total_games integer not null,
			avg_accuracy real not null,
			avg_cpl real not null,
			longest_win_streak integer not null,
			current_win_streak integer not null,
			rating_gain_20 integer not null,
			rating_gain_50 integer not null,
			suspicion_score real not null,
			recent_analyses_json text not null,
			action_taken text not null,
			last_analyzed_at text not null
		);
		create index if not exists anticheat_player_summaries_score_idx on anticheat_player_summaries (suspicion_score desc);
	`)
	return err
}

func (s *SqliteAnticheatStore) Backend() string { return "sqlite" }

func (s *SqliteAnticheatStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SqliteAnticheatStore) RecordAnalysis(record AnticheatAnalysisRecord) (AnticheatAnalysisRecord, error) {
	if strings.TrimSpace(record.AnalysisID) == "" {
		record.AnalysisID = "ach_" + randomToken(10)
	}
	if record.AnalyzedAt.IsZero() {
		record.AnalyzedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(
		`insert or ignore into anticheat_analyses (
			analysis_id, match_id, player_id, mode_id,
			accuracy, avg_cpl, max_cpl, move_count, card_moves,
			flags_json, time_profile_json, suspicion_score, analyzed_at
		) values (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		record.AnalysisID, record.MatchID, record.PlayerID, record.ModeID,
		record.Accuracy, record.AvgCPL, record.MaxCPL, record.MoveCount, record.CardMoves,
		record.FlagsJSON, record.TimeProfileJSON, record.SuspicionScore, record.AnalyzedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return AnticheatAnalysisRecord{}, err
	}
	return record, nil
}

func (s *SqliteAnticheatStore) UpsertPlayerSummary(summary AnticheatPlayerSummary) (AnticheatPlayerSummary, error) {
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
		) values (?,?,?,?,?,?,?,?,?,?,?,?)
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
		summary.SuspicionScore, string(recentJSON), summary.ActionTaken, summary.LastAnalyzedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return AnticheatPlayerSummary{}, err
	}
	return summary, nil
}

func (s *SqliteAnticheatStore) ListFlaggedPlayers(minScore float64, limit int) []AnticheatPlayerSummary {
	rows, err := s.db.Query(
		`select player_id, total_games, avg_accuracy, avg_cpl,
				longest_win_streak, current_win_streak,
				rating_gain_20, rating_gain_50,
				suspicion_score, recent_analyses_json, action_taken, last_analyzed_at
		from anticheat_player_summaries
		where suspicion_score >= ?
		order by suspicion_score desc
		limit ?`,
		minScore, limit,
	)
	if err != nil {
		return make([]AnticheatPlayerSummary, 0)
	}
	defer rows.Close()
	out := make([]AnticheatPlayerSummary, 0)
	for rows.Next() {
		summary, err := scanSqliteAnticheatPlayerSummary(rows)
		if err != nil {
			continue
		}
		out = append(out, summary)
	}
	return out
}

func (s *SqliteAnticheatStore) GetPlayerSummary(playerID string) (AnticheatPlayerSummary, bool) {
	resolvedPlayerID := strings.TrimSpace(playerID)
	if resolvedPlayerID == "" {
		return AnticheatPlayerSummary{}, false
	}
	row := s.db.QueryRow(
		`select player_id, total_games, avg_accuracy, avg_cpl,
				longest_win_streak, current_win_streak,
				rating_gain_20, rating_gain_50,
				suspicion_score, recent_analyses_json, action_taken, last_analyzed_at
		from anticheat_player_summaries where player_id = ?`,
		resolvedPlayerID,
	)
	summary, err := scanSqliteAnticheatPlayerSummary(row)
	if err != nil {
		return AnticheatPlayerSummary{}, false
	}
	return summary, true
}

func (s *SqliteAnticheatStore) ListPlayerAnalyses(playerID string, limit int) []AnticheatAnalysisRecord {
	resolvedPlayerID := strings.TrimSpace(playerID)
	rows, err := s.db.Query(
		`select analysis_id, match_id, player_id, mode_id,
				accuracy, avg_cpl, max_cpl, move_count, card_moves,
				flags_json, time_profile_json, suspicion_score, analyzed_at
		from anticheat_analyses
		where player_id = ?
		order by analyzed_at desc
		limit ?`,
		resolvedPlayerID, limit,
	)
	if err != nil {
		return make([]AnticheatAnalysisRecord, 0)
	}
	defer rows.Close()
	out := make([]AnticheatAnalysisRecord, 0)
	for rows.Next() {
		record, err := scanSqliteAnticheatAnalysis(rows)
		if err != nil {
			continue
		}
		out = append(out, record)
	}
	return out
}

func (s *SqliteAnticheatStore) Stats() AnticheatStats {
	stats := AnticheatStats{}
	if err := s.db.QueryRow(`select count(*) from anticheat_analyses`).Scan(&stats.AnalysisCount); err != nil {
		stats.AnalysisCount = 0
	}
	if err := s.db.QueryRow(`select count(*) from anticheat_player_summaries`).Scan(&stats.PlayerCount); err != nil {
		stats.PlayerCount = 0
	}
	if err := s.db.QueryRow(
		`select count(*) from anticheat_player_summaries where suspicion_score >= ?`,
		AnticheatSuspicionThreshold,
	).Scan(&stats.FlaggedPlayerCount); err != nil {
		stats.FlaggedPlayerCount = 0
	}
	return stats
}

type sqliteAnticheatScanner interface {
	Scan(dest ...any) error
}

func scanSqliteAnticheatAnalysis(scanner sqliteAnticheatScanner) (AnticheatAnalysisRecord, error) {
	var (
		record     AnticheatAnalysisRecord
		analyzedAt string
	)
	if err := scanner.Scan(
		&record.AnalysisID, &record.MatchID, &record.PlayerID, &record.ModeID,
		&record.Accuracy, &record.AvgCPL, &record.MaxCPL, &record.MoveCount, &record.CardMoves,
		&record.FlagsJSON, &record.TimeProfileJSON, &record.SuspicionScore, &analyzedAt,
	); err != nil {
		return AnticheatAnalysisRecord{}, err
	}
	if t, err := time.Parse(time.RFC3339Nano, analyzedAt); err == nil {
		record.AnalyzedAt = t.UTC()
	}
	return record, nil
}

func scanSqliteAnticheatPlayerSummary(scanner sqliteAnticheatScanner) (AnticheatPlayerSummary, error) {
	var (
		summary     AnticheatPlayerSummary
		recentJSON  string
		lastAnalyzed string
	)
	if err := scanner.Scan(
		&summary.PlayerID, &summary.TotalGames, &summary.AvgAccuracy, &summary.AvgCPL,
		&summary.LongestWinStreak, &summary.CurrentWinStreak,
		&summary.RatingGain20, &summary.RatingGain50,
		&summary.SuspicionScore, &recentJSON, &summary.ActionTaken, &lastAnalyzed,
	); err != nil {
		return AnticheatPlayerSummary{}, err
	}
	if t, err := time.Parse(time.RFC3339Nano, lastAnalyzed); err == nil {
		summary.LastAnalyzedAt = t.UTC()
	}
	if strings.TrimSpace(recentJSON) != "" {
		_ = json.Unmarshal([]byte(recentJSON), &summary.RecentAnalyses)
	}
	if summary.RecentAnalyses == nil {
		summary.RecentAnalyses = make([]string, 0)
	}
	return summary, nil
}

var _ AnticheatStore = (*SqliteAnticheatStore)(nil)
