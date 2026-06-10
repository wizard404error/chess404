package anticheat

import (
	"math"
	"time"
)

type GameRecord struct {
	MatchID     string       `json:"match_id"`
	WhiteID     string       `json:"white_id"`
	BlackID     string       `json:"black_id"`
	Moves       []MoveRecord `json:"moves"`
	Mode        string       `json:"mode"`
	StartedAt   time.Time    `json:"started_at"`
	FinishedAt  time.Time    `json:"finished_at"`
	Result      string       `json:"result"`
}

type MoveRecord struct {
	MoveNumber int       `json:"move_number"`
	Color      string    `json:"color"`
	From       string    `json:"from"`
	To         string    `json:"to"`
	CardsPlayed []string `json:"cards_played,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	ThinkTime  float64   `json:"think_time_ms"`
}

type AnalysisResult struct {
	MatchID       string         `json:"match_id"`
	PlayerID      string         `json:"player_id"`
	Color         string         `json:"color"`
	Accuracy      float64        `json:"accuracy"`
	CPL           []int          `json:"cpl"`
	AvgCPL        float64        `json:"avg_cpl"`
	MaxCPL        int            `json:"max_cpl"`
	MoveCount     int            `json:"move_count"`
	CardMoves     int            `json:"card_moves"`
	TimeProfile   TimeProfile    `json:"time_profile"`
	SuspicionScore float64       `json:"suspicion_score"`
	Flags         []string       `json:"flags"`
	AnalyzedAt    time.Time      `json:"analyzed_at"`
}

type TimeProfile struct {
	AvgThinkTime   float64 `json:"avg_think_time_ms"`
	StdDev         float64 `json:"std_dev_ms"`
	FlatTimeRatio  float64 `json:"flat_time_ratio"`
	ComplexityCorr float64 `json:"complexity_correlation"`
}

type CheatFlag string

const (
	FlagHighAccuracy     CheatFlag = "high_accuracy"
	FlagLowCPL           CheatFlag = "low_cpl"
	FlagFlatTimePattern  CheatFlag = "flat_time_pattern"
	FlagWinStreak        CheatFlag = "win_streak"
	FlagRatingAnomaly    CheatFlag = "rating_anomaly"
	FlagComplexityBlind  CheatFlag = "complexity_blindness"
)

func CalculateAccuracy(cpl []int) float64 {
	if len(cpl) == 0 {
		return 0
	}
	total := 0
	for _, v := range cpl {
		total += v
	}
	avg := float64(total) / float64(len(cpl))
	return math.Max(0, 100-avg/10)
}

func CalculateSuspicion(result *AnalysisResult) float64 {
	score := 0.0

	if result.Accuracy > 92 {
		score += 30
	}
	if result.AvgCPL < 15 {
		score += 25
	}
	if result.TimeProfile.FlatTimeRatio > 0.7 {
		score += 20
	}
	if result.TimeProfile.ComplexityCorr < 0.2 {
		score += 15
	}
	if result.Accuracy > 95 && result.MoveCount > 30 {
		score += 10
	}

	return math.Min(100, score)
}

func DetectFlags(result *AnalysisResult) []string {
	var flags []string

	if result.Accuracy > 92 {
		flags = append(flags, string(FlagHighAccuracy))
	}
	if result.AvgCPL < 15 {
		flags = append(flags, string(FlagLowCPL))
	}
	if result.TimeProfile.FlatTimeRatio > 0.7 {
		flags = append(flags, string(FlagFlatTimePattern))
	}
	if result.TimeProfile.ComplexityCorr < 0.2 {
		flags = append(flags, string(FlagComplexityBlind))
	}

	return flags
}

func AnalyzeTimeProfile(moves []MoveRecord) TimeProfile {
	if len(moves) == 0 {
		return TimeProfile{}
	}

	times := make([]float64, len(moves))
	for i, m := range moves {
		times[i] = m.ThinkTime
	}

	avg := mean(times)
	std := stddev(times)

	flatCount := 0
	for _, t := range times {
		if std > 0 && math.Abs(t-avg) < std*0.1 {
			flatCount++
		}
	}

	return TimeProfile{
		AvgThinkTime:  avg,
		StdDev:        std,
		FlatTimeRatio: float64(flatCount) / float64(len(times)),
	}
}

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func stddev(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	avg := mean(vals)
	sum := 0.0
	for _, v := range vals {
		d := v - avg
		sum += d * d
	}
	return math.Sqrt(sum / float64(len(vals)))
}
