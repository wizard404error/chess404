package anticheat

import (
	"math"
	"time"
)

type PlayerHistory struct {
	PlayerID  string        `json:"player_id"`
	Games     []GameResult  `json:"games"`
	Rating    int           `json:"rating"`
	TotalGames int          `json:"total_games"`
}

type GameResult struct {
	MatchID   string    `json:"match_id"`
	Result    string    `json:"result"`
	Opponent  string    `json:"opponent_id"`
	Rating    int       `json:"rating_at_time"`
	Timestamp time.Time `json:"timestamp"`
}

type StreakAnalysis struct {
	LongestWinStreak    int     `json:"longest_win_streak"`
	CurrentWinStreak    int     `json:"current_win_streak"`
	RatingGainLast20    int     `json:"rating_gain_last_20"`
	RatingGainLast50    int     `json:"rating_gain_last_50"`
	SuspiciousStreak    bool    `json:"suspicious_streak"`
	SuspiciousRating    bool    `json:"suspicious_rating"`
	OverallScore        float64 `json:"overall_score"`
}

func AnalyzeStreaks(history *PlayerHistory) StreakAnalysis {
	if len(history.Games) == 0 {
		return StreakAnalysis{}
	}

	longest := 0
	current := 0
	for _, g := range history.Games {
		if g.Result == "win" {
			current++
			if current > longest {
				longest = current
			}
		} else {
			current = 0
		}
	}

	recent20 := history.Games
	if len(recent20) > 20 {
		recent20 = recent20[len(recent20)-20:]
	}
	ratingGain20 := calculateRatingGain(recent20)

	recent50 := history.Games
	if len(recent50) > 50 {
		recent50 = recent50[len(recent50)-50:]
	}
	ratingGain50 := calculateRatingGain(recent50)

	suspiciousStreak := longest >= 15
	suspiciousRating := ratingGain20 > 300 || ratingGain50 > 500

	score := 0.0
	if suspiciousStreak {
		score += 40
	}
	if suspiciousRating {
		score += 30
	}
	if longest >= 20 {
		score += 20
	}
	if ratingGain20 > 400 {
		score += 10
	}

	return StreakAnalysis{
		LongestWinStreak: longest,
		CurrentWinStreak: current,
		RatingGainLast20: ratingGain20,
		RatingGainLast50: ratingGain50,
		SuspiciousStreak: suspiciousStreak,
		SuspiciousRating: suspiciousRating,
		OverallScore:     math.Min(100, score),
	}
}

func calculateRatingGain(games []GameResult) int {
	if len(games) < 2 {
		return 0
	}
	return games[len(games)-1].Rating - games[0].Rating
}
