package anticheat

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// PositionSample is one position in a game with the move the player
// actually played. Card-move positions are tagged so the analysis can
// weight card moves and tactical moves separately.
type PositionSample struct {
	// FEN is the board position before the player's move.
	FEN string
	// PlayedMove is the move the player chose, in UCI notation.
	PlayedMove string
	// IsCardMove is true if the played move involved a card ability
	// (e.g., a recall, a teleport, a swap). Card moves are
	// intentionally NOT evaluated against the engine — a card move
	// is a meta-decision the engine has no signal for.
	IsCardMove bool
}

// AnalyzeIrwin computes the player's engine correlation across the
// given positions. For each non-card position it asks the engine for
// the top N moves, then records:
//   - top1Pct: % of positions where the player matched the engine's
//     top-1 move
//   - top3Pct: % of positions where the player matched any of the
//     engine's top-3 moves
//   - avgRank: average engine-rank of the player's move (1.0 = always
//     top-1, 3.0 = always bottom of top-3, etc.)
//   - cardMovePct: % of positions that were card moves (excluded from
//     the engine correlation)
//
// Returns a fully-populated Result with TotalPositions = len(samples)
// and EnginePositions = number of non-card positions. Empty samples
// yields a Result with all zeros.
//
// This is the lichess "Irwin" pattern: simple, no ML, no thresholds
// baked in. Suspicion comes from comparing the player's distribution
// against population norms after we've collected enough data.
func AnalyzeIrwin(ctx context.Context, engine Engine, samples []PositionSample, depth, multiPV int) (Result, error) {
	if engine == nil {
		return Result{}, errors.New("engine is required")
	}
	if multiPV <= 0 {
		multiPV = 3
	}
	if depth <= 0 {
		depth = 20
	}

	res := Result{
		TotalPositions: len(samples),
		EngineDepth:    depth,
		EngineMultiPV:  multiPV,
	}
	if len(samples) == 0 {
		return res, nil
	}

	for _, sample := range samples {
		if sample.IsCardMove {
			res.CardMoveCount++
			continue
		}
		res.EnginePositions++

		top, err := engine.TopNMoves(ctx, sample.FEN, depth, multiPV)
		if err != nil {
			// Don't fail the whole analysis for one bad position;
			// record the error and skip. This makes the system robust
			// to engine hiccups mid-game.
			res.EngineErrors++
			continue
		}
		if len(top) == 0 {
			res.EngineErrors++
			continue
		}

		rank := findUCIPlay(top, sample.PlayedMove)
		if rank == 0 {
			// Player's move is not in the engine's top-N. Could be a
			// genuinely creative choice or a card-related side
			// effect we missed. Count as "outside top-N".
			res.OutsideTopN++
			continue
		}
		res.RankSum += rank
		if rank == 1 {
			res.Top1Count++
		}
		if rank <= 3 {
			res.Top3Count++
		}
	}

	// Compute percentages over the engine-evaluated positions (not
	// over the full sample, since card moves are intentionally
	// excluded from engine comparison).
	evaluated := res.EnginePositions - res.EngineErrors
	if evaluated > 0 {
		res.Top1Pct = float64(res.Top1Count) / float64(evaluated) * 100
		res.Top3Pct = float64(res.Top3Count) / float64(evaluated) * 100
		res.AvgRank = float64(res.RankSum) / float64(evaluated)
	}
	return res, nil
}

// findUCIPlay returns the 1-based engine rank of playedMove in the
// engine's top-N list, or 0 if the move is not present. The match is
// exact (case-sensitive, no normalization). UCI notation is canonical,
// so "e2e4" and "E2E4" should not collide in practice; we still
// lowercase-compare to be safe.
func findUCIPlay(top []EngineMove, playedMove string) int {
	needle := strings.ToLower(strings.TrimSpace(playedMove))
	for _, m := range top {
		if strings.ToLower(m.Move) == needle {
			return m.Rank
		}
	}
	return 0
}

// Result is the output of AnalyzeIrwin: a per-player summary of how
// the player's moves correlated with the engine's top-N. All counts
// are absolute; percentages are computed from evaluated positions.
type Result struct {
	// TotalPositions is the number of position samples analyzed
	// (including card moves).
	TotalPositions int `json:"totalPositions"`
	// EnginePositions is the number of non-card positions sent to
	// the engine.
	EnginePositions int `json:"enginePositions"`
	// EngineErrors is the number of positions where the engine
	// failed (returned an error or empty top-N).
	EngineErrors int `json:"engineErrors"`
	// CardMoveCount is the number of positions excluded because
	// they were card moves.
	CardMoveCount int `json:"cardMoveCount"`

	// Top1Count is the number of positions where the played move
	// matched the engine's top-1.
	Top1Count int `json:"top1Count"`
	// Top3Count is the number of positions where the played move
	// matched the engine's top-3 (any rank 1..3).
	Top3Count int `json:"top3Count"`
	// OutsideTopN is the number of positions where the played move
	// was not in the engine's top-N at all.
	OutsideTopN int `json:"outsideTopN"`
	// RankSum is the sum of engine ranks for the played moves that
	// were in the top-N. Used to compute AvgRank.
	RankSum int `json:"-"`

	// Top1Pct is the percentage of engine-evaluated positions where
	// the player matched the engine's top-1 move.
	Top1Pct float64 `json:"top1Pct"`
	// Top3Pct is the percentage where the player matched top-3.
	Top3Pct float64 `json:"top3Pct"`
	// AvgRank is the average engine rank of the played moves
	// (1.0 = always top-1, 3.0 = always bottom of top-3).
	AvgRank float64 `json:"avgRank"`

	// EngineDepth is the depth that was used (recorded for
	// reproducibility).
	EngineDepth int `json:"engineDepth"`
	// EngineMultiPV is the multiPV that was used.
	EngineMultiPV int `json:"engineMultiPV"`
}

// Score converts the engine-correlation result to a 0-100 suspicion
// score. The constants are tuned for card chess: at our scale we want
// a low false-positive rate, so the threshold is conservative. Tune
// with real data.
//
// The formula:
//   - 70% weight on Top1Pct (the strongest signal)
//   - 30% weight on Top3Pct (secondary signal)
//   - Both are clamped to [0, 100].
//
// At Top1Pct=85%, Top3Pct=99% the score is around 70/100.
// At Top1Pct=92%, Top3Pct=99% the score is around 76/100.
//
// If Top1Pct and Top3Pct are zero (i.e., the result was constructed
// from counts without computing percentages), Score recomputes them
// from Top1Count/Top3Count/EnginePositions. This makes the helper
// usable both for live analysis (where percentages are computed by
// AnalyzeIrwin) and for tests that build Result values directly.
func (r Result) Score() float64 {
	evaluated := r.EnginePositions - r.EngineErrors
	if evaluated <= 0 {
		return 0
	}
	top1Pct := r.Top1Pct
	if top1Pct == 0 && r.Top1Count > 0 {
		top1Pct = float64(r.Top1Count) / float64(evaluated) * 100
	}
	top3Pct := r.Top3Pct
	if top3Pct == 0 && r.Top3Count > 0 {
		top3Pct = float64(r.Top3Count) / float64(evaluated) * 100
	}
	weighted := 0.7*top1Pct + 0.3*top3Pct
	if weighted > 100 {
		weighted = 100
	}
	return weighted
}

// String returns a compact one-line summary for logging.
func (r Result) String() string {
	evaluated := r.EnginePositions - r.EngineErrors
	return fmt.Sprintf("irwin: total=%d engine=%d card=%d err=%d top1=%.1f%% top3=%.1f%% avgRank=%.2f score=%.1f (n=%d)",
		r.TotalPositions, r.EnginePositions, r.CardMoveCount, r.EngineErrors,
		r.Top1Pct, r.Top3Pct, r.AvgRank, r.Score(), evaluated)
}
