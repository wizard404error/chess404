package platform

import "math"

const (
	defaultEloKFactor    = 32.0
	defaultEloStartRating = 1200
	defaultEloMinRating  = 100
)

// ApplyEloMatchResult updates the two ratings according to the standard
// Elo formula with a 32-point K-factor, returning the new ratings.
//
//	winner: "white" | "black" | "draw"
//
// The K-factor and minimum rating are exposed for callers that need to vary
// them (account finalization uses a stricter floor).
func ApplyEloMatchResult(whiteRating, blackRating int, winner string) (int, int) {
	return applyEloMatchResultWithK(whiteRating, blackRating, winner, defaultEloKFactor, defaultEloMinRating)
}

func applyEloMatchResultWithK(whiteRating, blackRating int, winner string, kFactor float64, minRating int) (int, int) {
	if whiteRating <= 0 {
		whiteRating = defaultEloStartRating
	}
	if blackRating <= 0 {
		blackRating = defaultEloStartRating
	}
	whiteR := float64(whiteRating)
	blackR := float64(blackRating)
	whiteExpected := 1.0 / (1.0 + math.Pow(10, (blackR-whiteR)/400.0))
	blackExpected := 1.0 - whiteExpected

	var newWhite, newBlack int
	switch winner {
	case "white":
		newWhite = int(math.Round(whiteR + kFactor*(1.0-whiteExpected)))
		newBlack = int(math.Round(blackR + kFactor*(0.0-blackExpected)))
	case "black":
		newBlack = int(math.Round(blackR + kFactor*(1.0-blackExpected)))
		newWhite = int(math.Round(whiteR + kFactor*(0.0-whiteExpected)))
	case "draw":
		newWhite = int(math.Round(whiteR + kFactor*(0.5-whiteExpected)))
		newBlack = int(math.Round(blackR + kFactor*(0.5-blackExpected)))
	default:
		return whiteRating, blackRating
	}

	if newBlack < minRating {
		newBlack = minRating
	}
	if newWhite < minRating {
		newWhite = minRating
	}
	return newWhite, newBlack
}
