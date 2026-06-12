package anticheat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// Engine abstracts a chess engine used to compute the player's engine
// correlation. Production uses a Stockfish subprocess; tests use a
// MockEngine that returns canned responses.
//
// The interface returns the engine's top N moves for a position, with
// centipawn scores from the side-to-move's perspective. Rank is
// 1-based: rank=1 is the engine's best move.
//
// Implementation note: this runs per-position and is the hot path of
// the analysis-worker. A subprocess-based Stockfish engine keeps state
// across calls (transposition table, etc.) so depth-20 here is roughly
// 50x faster than depth-20 from a cold start.
type Engine interface {
	// TopNMoves returns the engine's top N moves for the given FEN.
	// depth and multiPV are the engine search parameters. The returned
	// slice is ordered best-first (rank=1 first). An error is returned
	// only for non-recoverable problems (e.g., context cancelled); a
	// position the engine cannot evaluate (illegal FEN, etc.) returns
	// an empty slice and no error.
	TopNMoves(ctx context.Context, fen string, depth, multiPV int) ([]EngineMove, error)
	// Close releases any engine resources (subprocess, etc.). Idempotent.
	Close() error
}

// EngineMove is a single line in the engine's principal variation.
type EngineMove struct {
	// Move is the engine's move in UCI notation ("e2e4", "g1f3", etc.).
	Move string
	// ScoreCP is the centipawn evaluation from the side-to-move's
	// perspective. Positive = good for the player to move.
	ScoreCP int
	// Rank is the 1-based position in the engine's ranking (1 = best).
	Rank int
}

// ErrEngineClosed is returned by Engine methods after Close has been
// called. Callers should treat this as a fatal error for the engine
// instance and not retry.
var ErrEngineClosed = errors.New("anticheat engine is closed")

// parseUCIInfoLine extracts the multipv rank, the centipawn score, and
// the first move of the principal variation from a Stockfish
// "info ..." line. Returns ok=false if the line isn't a usable info
// line (e.g., a "info depth 0" before search starts).
func parseUCIInfoLine(line string) (rank int, scoreCP int, move string, ok bool) {
	if !strings.HasPrefix(line, "info ") {
		return 0, 0, "", false
	}
	fields := strings.Fields(line)
	var (
		haveRank  bool
		haveScore bool
		havePV    bool
		firstMove string
	)
	for i := 0; i < len(fields); i++ {
		switch fields[i] {
		case "multipv":
			if i+1 < len(fields) {
				if v, err := parseIntSafe(fields[i+1]); err == nil {
					rank = v
					haveRank = true
				}
			}
		case "score":
			if i+2 < len(fields) {
				switch fields[i+1] {
				case "cp":
					if v, err := parseIntSafe(fields[i+2]); err == nil {
						scoreCP = v
						haveScore = true
					}
				case "mate":
					// mate in N: treat as a very high centipawn score
					// (sign-preserved) so the rank ordering still works
					// but flagged as a forced win/loss.
					if v, err := parseIntSafe(fields[i+2]); err == nil {
						if v > 0 {
							scoreCP = 10000 - v
						} else {
							scoreCP = -10000 + v
						}
						haveScore = true
					}
				}
			}
		case "pv":
			havePV = true
			if i+1 < len(fields) {
				firstMove = fields[i+1]
			}
		}
	}
	if !haveRank || !haveScore || !havePV || firstMove == "" {
		return 0, 0, "", false
	}
	return rank, scoreCP, firstMove, true
}

func parseIntSafe(s string) (int, error) {
	var n int
	if s == "" {
		return 0, errors.New("empty int")
	}
	neg := false
	if s[0] == '-' {
		neg = true
		s = s[1:]
	} else if s[0] == '+' {
		s = s[1:]
	}
	if s == "" {
		return 0, errors.New("sign only")
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("non-digit %q", string(r))
		}
		n = n*10 + int(r-'0')
	}
	if neg {
		n = -n
	}
	return n, nil
}

// MockEngine is a deterministic engine for tests. It returns the
// pre-configured top-N moves for any position. Use it to drive the
// Irwin analysis in unit tests without spawning Stockfish.
type MockEngine struct {
	mu      sync.Mutex
	closed  bool
	TopNByFEN map[string][]EngineMove
	// DefaultTopN is returned for any FEN not in the map. Use it to
	// simulate a "perfect" engine (engine always picks the right move).
	DefaultTopN []EngineMove
	// CallCount counts every TopNMoves call (for test assertions).
	CallCount int
}

// NewMockEngine returns a MockEngine that uses the given default top-N
// for every position. Convenient for tests that don't care about
// per-position behavior.
func NewMockEngine(defaultTopN []EngineMove) *MockEngine {
	return &MockEngine{
		DefaultTopN: defaultTopN,
		TopNByFEN:   make(map[string][]EngineMove),
	}
}

// SetForFEN configures the top-N for a specific FEN. Use this to
// simulate an engine that picks a specific move for a known position.
func (e *MockEngine) SetForFEN(fen string, topN []EngineMove) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.TopNByFEN[fen] = topN
}

func (e *MockEngine) TopNMoves(ctx context.Context, fen string, depth, multiPV int) ([]EngineMove, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil, ErrEngineClosed
	}
	e.CallCount++
	var top []EngineMove
	if per, ok := e.TopNByFEN[fen]; ok {
		top = per
	} else if e.DefaultTopN != nil {
		top = e.DefaultTopN
	} else {
		return []EngineMove{}, nil
	}
	// Respect multiPV: only return up to that many moves (Stockfish
	// would also reject a multiPV larger than its configured value,
	// but for tests we just cap the slice).
	if multiPV > 0 && len(top) > multiPV {
		top = top[:multiPV]
	}
	return top, nil
}

func (e *MockEngine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.closed = true
	return nil
}
