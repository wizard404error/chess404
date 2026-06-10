package anticheat

import (
	"testing"
	"time"
)

func TestCalculateAccuracy(t *testing.T) {
	tests := []struct {
		name     string
		cpl      []int
		expected float64
	}{
		{"empty", []int{}, 0},
		{"perfect", []int{0, 0, 0}, 100},
		{"good", []int{10, 15, 20}, 98.5},
		{"average", []int{50, 60, 70}, 94.0},
		{"poor", []int{100, 150, 200}, 85.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateAccuracy(tt.cpl)
			if result != tt.expected {
				t.Errorf("CalculateAccuracy(%v) = %f, want %f", tt.cpl, result, tt.expected)
			}
		})
	}
}

func TestCalculateSuspicion(t *testing.T) {
	tests := []struct {
		name     string
		result   *AnalysisResult
		minScore float64
	}{
		{
			name: "clean player",
			result: &AnalysisResult{
				Accuracy:  75,
				AvgCPL:    45,
				MoveCount: 20,
				TimeProfile: TimeProfile{
					FlatTimeRatio:  0.3,
					ComplexityCorr: 0.6,
				},
			},
			minScore: 0,
		},
		{
			name: "suspicious player",
			result: &AnalysisResult{
				Accuracy:  95,
				AvgCPL:    8,
				MoveCount: 40,
				TimeProfile: TimeProfile{
					FlatTimeRatio:  0.85,
					ComplexityCorr: 0.1,
				},
			},
			minScore: 50,
		},
		{
			name: "extremely suspicious",
			result: &AnalysisResult{
				Accuracy:  98,
				AvgCPL:    3,
				MoveCount: 50,
				TimeProfile: TimeProfile{
					FlatTimeRatio:  0.9,
					ComplexityCorr: 0.05,
				},
			},
			minScore: 80,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := CalculateSuspicion(tt.result)
			if score < tt.minScore {
				t.Errorf("CalculateSuspicion() = %f, want >= %f", score, tt.minScore)
			}
		})
	}
}

func TestDetectFlags(t *testing.T) {
	result := &AnalysisResult{
		Accuracy: 95,
		AvgCPL:   10,
		TimeProfile: TimeProfile{
			FlatTimeRatio:  0.8,
			ComplexityCorr: 0.1,
		},
	}

	flags := DetectFlags(result)

	if len(flags) < 3 {
		t.Errorf("expected at least 3 flags, got %d: %v", len(flags), flags)
	}

	flagSet := make(map[string]bool)
	for _, f := range flags {
		flagSet[f] = true
	}

	if !flagSet[string(FlagHighAccuracy)] {
		t.Error("expected high_accuracy flag")
	}
	if !flagSet[string(FlagLowCPL)] {
		t.Error("expected low_cpl flag")
	}
	if !flagSet[string(FlagFlatTimePattern)] {
		t.Error("expected flat_time_pattern flag")
	}
}

func TestAnalyzeTimeProfile(t *testing.T) {
	moves := []MoveRecord{
		{ThinkTime: 1000},
		{ThinkTime: 1000},
		{ThinkTime: 1000},
		{ThinkTime: 1000},
		{ThinkTime: 1000},
	}

	profile := AnalyzeTimeProfile(moves)

	if profile.StdDev != 0 {
		t.Errorf("expected stddev 0 for identical times, got %f", profile.StdDev)
	}
	if profile.AvgThinkTime != 1000 {
		t.Errorf("expected avg 1000, got %f", profile.AvgThinkTime)
	}
}

func TestAnalyzeStreaks(t *testing.T) {
	games := make([]GameResult, 25)
	for i := range games {
		games[i] = GameResult{
			Result:   "win",
			Rating:   1000 + i*20,
			Timestamp: time.Now().Add(-time.Duration(25-i) * 24 * time.Hour),
		}
	}

	history := &PlayerHistory{
		PlayerID: "test-player",
		Games:    games,
		Rating:   1500,
	}

	analysis := AnalyzeStreaks(history)

	if !analysis.SuspiciousStreak {
		t.Error("expected suspicious streak for 25 consecutive wins")
	}
	if !analysis.SuspiciousRating {
		t.Error("expected suspicious rating for +500 gain")
	}
	if analysis.LongestWinStreak != 25 {
		t.Errorf("expected longest win streak 25, got %d", analysis.LongestWinStreak)
	}
}
