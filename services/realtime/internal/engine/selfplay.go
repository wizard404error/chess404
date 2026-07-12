package engine

import (
	"math/rand"
	"time"

	"github.com/chess404/realtime/internal/contracts"
)

// SelfPlayConfig tunes the self-play generation.
type SelfPlayConfig struct {
	Games            int
	MaxPly           int
	SearchDepth      int
	TTEntryCount     int
	Temperature      float64 // 0 = deterministic, 1 = fully random
	RandomizeOpening bool
}

var DefaultSelfPlayConfig = SelfPlayConfig{
	Games:            100,
	MaxPly:           80,
	SearchDepth:      4,
	TTEntryCount:     1 << 16,
	Temperature:      0.3,
	RandomizeOpening: true,
}

// SelfPlayResult holds the outcome of a self-play game.
type SelfPlayResult struct {
	GameNum     int
	PlyCount    int
	Result      string // "1-0", "0-1", "1/2-1/2"
	Moves       []string
	FinalFEN    string
}

// RunSelfPlay runs a batch of self-play games.
func RunSelfPlay(cfg SelfPlayConfig) []SelfPlayResult {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	results := make([]SelfPlayResult, 0, cfg.Games)

	for g := 0; g < cfg.Games; g++ {
		result := playOneGame(g+1, cfg, rng)
		results = append(results, result)
	}

	return results
}

func playOneGame(gameNum int, cfg SelfPlayConfig, rng *rand.Rand) SelfPlayResult {
	state := MatchStateFromFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	tt := NewTranspositionTable(cfg.TTEntryCount)
	var moves []string

	for ply := 0; ply < cfg.MaxPly; ply++ {
		if state.Status != "active" {
			break
		}
		isWhite := state.Turn == "white"
		genMoves := generateAllMoves(state, isWhite)
		if len(genMoves) == 0 {
			break
		}

		var bestMove Move

		if cfg.Temperature > 0 && rng.Float64() < cfg.Temperature {
			bestMove = genMoves[rng.Intn(len(genMoves))]
		} else {
			result := Search(state, cfg.SearchDepth, tt)
			if result.BestMove.From.Row == 0 && result.BestMove.From.Col == 0 &&
				result.BestMove.To.Row == 0 && result.BestMove.To.Col == 0 {
				bestMove = genMoves[0]
			} else {
				bestMove = result.BestMove
			}
		}

		moves = append(moves, moveToUCI(&bestMove))
		state = applyMoveCopy(state, &bestMove)
	}

	// Determine result by checking final position.
	result := "1/2-1/2"
	if state.Status == "checkmate" {
		if state.Turn == "white" {
			result = "0-1"
		} else {
			result = "1-0"
		}
	}

	return SelfPlayResult{
		GameNum:  gameNum,
		PlyCount: len(moves),
		Result:   result,
		Moves:    moves,
		FinalFEN: boardToSimpleFEN(state),
	}
}

// boardToSimpleFEN generates a simple FEN from the state for diagnostic logging.
func BoardToSimpleFEN(state *contracts.MatchState) string {
	return boardToSimpleFEN(state)
}

func boardToSimpleFEN(state *contracts.MatchState) string {
	if state == nil {
		return ""
	}
	pieceChar := func(p *contracts.Piece) byte {
		ch := byte('?')
		switch p.Type {
		case "pawn":
			ch = 'p'
		case "knight":
			ch = 'n'
		case "bishop":
			ch = 'b'
		case "rook":
			ch = 'r'
		case "queen":
			ch = 'q'
		case "king":
			ch = 'k'
		}
		if p.Color == "white" {
			ch -= 32
		}
		return ch
	}
	var fen []byte
	for r := 7; r >= 0; r-- {
		empty := 0
		for c := 0; c < 8; c++ {
			p := state.Board[r][c]
			if p == nil {
				empty++
			} else {
				if empty > 0 {
					fen = append(fen, byte('0'+empty))
					empty = 0
				}
				fen = append(fen, pieceChar(p))
			}
		}
		if empty > 0 {
			fen = append(fen, byte('0'+empty))
		}
		if r > 0 {
			fen = append(fen, '/')
		}
	}
	fen = append(fen, ' ')
	if state.Turn == "white" {
		fen = append(fen, 'w')
	} else {
		fen = append(fen, 'b')
	}
	return string(fen)
}
