package platform

import "time"

// AnticheatAnalysisRecord is a single post-game analysis result, computed
// by the analysis worker after a match finishes and stored for review.
type AnticheatAnalysisRecord struct {
	AnalysisID     string    `json:"analysisId"`
	MatchID        string    `json:"matchId"`
	PlayerID       string    `json:"playerId"`
	ModeID         string    `json:"modeId"`
	Accuracy       float64   `json:"accuracy"`
	AvgCPL         float64   `json:"avgCpl"`
	MaxCPL         int       `json:"maxCpl"`
	MoveCount      int       `json:"moveCount"`
	CardMoves      int       `json:"cardMoves"`
	FlagsJSON      string    `json:"flagsJson"`
	TimeProfileJSON string   `json:"timeProfileJson"`
	SuspicionScore float64   `json:"suspicionScore"`
	AnalyzedAt     time.Time `json:"analyzedAt"`
}

// AnticheatPlayerSummary is the rolled-up per-player view used by admin
// tooling and auto-action logic. SuspicionScore is the rolling composite
// of recent analyses; RecentAnalyses holds the last N analysis IDs.
type AnticheatPlayerSummary struct {
	PlayerID         string    `json:"playerId"`
	TotalGames       int       `json:"totalGames"`
	AvgAccuracy      float64   `json:"avgAccuracy"`
	AvgCPL           float64   `json:"avgCpl"`
	LongestWinStreak int       `json:"longestWinStreak"`
	CurrentWinStreak int       `json:"currentWinStreak"`
	RatingGain20     int       `json:"ratingGain20"`
	RatingGain50     int       `json:"ratingGain50"`
	SuspicionScore   float64   `json:"suspicionScore"`
	RecentAnalyses   []string  `json:"recentAnalyses"`
	ActionTaken      string    `json:"actionTaken"`
	LastAnalyzedAt   time.Time `json:"lastAnalyzedAt"`
}

// AnticheatStore persists per-match analyses and per-player summaries.
// The auto-action layer reads SuspicionScore from the summary to decide
// when to take action (rating reset, ban); admins read it via the
// platform-service endpoint.
type AnticheatStore interface {
	Backend() string
	Close() error
	RecordAnalysis(record AnticheatAnalysisRecord) (AnticheatAnalysisRecord, error)
	UpsertPlayerSummary(summary AnticheatPlayerSummary) (AnticheatPlayerSummary, error)
	ListFlaggedPlayers(minScore float64, limit int) []AnticheatPlayerSummary
	GetPlayerSummary(playerID string) (AnticheatPlayerSummary, bool)
	ListPlayerAnalyses(playerID string, limit int) []AnticheatAnalysisRecord
	Stats() AnticheatStats
	// PruneAnalysesOlderThan deletes analysis rows older than the given time
	// and returns the number of rows removed. Implementations that don't
	// support pruning should return 0 with no error.
	PruneAnalysesOlderThan(cutoff time.Time) (int64, error)
}

// AnticheatStats are aggregate counts for the admin overview.
type AnticheatStats struct {
	AnalysisCount    int `json:"analysisCount"`
	PlayerCount      int `json:"playerCount"`
	FlaggedPlayerCount int `json:"flaggedPlayerCount"`
}

const (
	// AnticheatActionNone is the default; no action has been taken yet.
	AnticheatActionNone = ""
	// AnticheatActionWarning means the player received a warning via
	// their account security event log; no rating change.
	AnticheatActionWarning = "warning_issued"
	// AnticheatActionRatingReset means the player's rating was reset to
	// the platform default after a confirmed cheating signal.
	AnticheatActionRatingReset = "rating_reset"
	// AnticheatActionBanned means the account is banned from ranked play.
	AnticheatActionBanned = "banned"
)

// AnticheatSuspicionThreshold is the SuspicionScore above which the
// platform-service auto-action layer escalates a player (warning or
// rating reset). Tuned conservatively: 80/100 only fires when several
// independent signals (accuracy, CPL, time profile, win streak) all
// line up. False positives are still possible but should be rare.
const AnticheatSuspicionThreshold = 80.0
