package engine

import (
	"fmt"
	"math"
	"sort"
	"sync"

	"github.com/chess404/realtime/internal/contracts"
)

const (
	ExactScore  = 0
	LowerBound  = 1
	UpperBound  = 2
)

type TTEntry struct {
	Depth  int
	Score  int
	Flag   int
	BestMove string
}

type TranspositionTable struct {
	entries map[uint64]TTEntry
	mu      sync.RWMutex
	maxSize int
}

func NewTranspositionTable(maxSize int) *TranspositionTable {
	return &TranspositionTable{
		entries: make(map[uint64]TTEntry, maxSize),
		maxSize: maxSize,
	}
}

func (tt *TranspositionTable) Lookup(key uint64, depth int, alpha, beta int) (bool, int) {
	tt.mu.RLock()
	entry, ok := tt.entries[key]
	tt.mu.RUnlock()

	if !ok || entry.Depth < depth {
		return false, 0
	}

	if entry.Flag == ExactScore {
		return true, entry.Score
	}
	if entry.Flag == LowerBound && entry.Score >= beta {
		return true, entry.Score
	}
	if entry.Flag == UpperBound && entry.Score <= alpha {
		return true, entry.Score
	}
	return false, 0
}

func (tt *TranspositionTable) Store(key uint64, depth, score, flag int, bestMove string) {
	tt.mu.Lock()
	defer tt.mu.Unlock()

	if len(tt.entries) >= tt.maxSize {
		tt.entries = make(map[uint64]TTEntry, tt.maxSize/2)
	}
	tt.entries[key] = TTEntry{Depth: depth, Score: score, Flag: flag, BestMove: bestMove}
}

type Move struct {
	From  contracts.Square
	To    contracts.Square
	Score int
}

type SearchResult struct {
	BestMove Move
	Score    int
	Nodes    int
	Depth    int
}

func Search(state *contracts.MatchState, maxDepth int, tt *TranspositionTable) SearchResult {
	turn := state.Turn
	bestMove := Move{}
	nodes := 0

	for depth := 1; depth <= maxDepth; depth++ {
		score, move := alphaBeta(state, depth, math.MinInt+1, math.MaxInt-1, turn == "white", tt, &nodes, 0)
		if move != nil {
			bestMove = *move
			bestMove.Score = score
		}
	}

	return SearchResult{
		BestMove: bestMove,
		Score:    bestMove.Score,
		Nodes:    nodes,
		Depth:    maxDepth,
	}
}

func alphaBeta(state *contracts.MatchState, depth, alpha, beta int, maximizing bool, tt *TranspositionTable, nodes *int, ply int) (int, *Move) {
	*nodes++

	if depth == 0 {
		score := Evaluate(state.Board, state.Turn)
		if !maximizing {
			score = -score
		}
		return score, nil
	}

	moves := generateAllMoves(state, maximizing)
	if len(moves) == 0 {
		if isKingInCheck(state) {
			if maximizing {
				return -20000 + ply, nil
			}
			return 20000 - ply, nil
		}
		return 0, nil
	}

	orderMoves(moves, state)

	bestMove := &moves[0]

	if maximizing {
		maxEval := math.MinInt + 1
		for i := range moves {
			newState := applyMoveCopy(state, &moves[i])
			eval, _ := alphaBeta(newState, depth-1, alpha, beta, false, tt, nodes, ply+1)
			if eval > maxEval {
				maxEval = eval
				bestMove = &moves[i]
			}
			alpha = max(alpha, eval)
			if beta <= alpha {
				break
			}
		}
		return maxEval, bestMove
	}

	minEval := math.MaxInt - 1
	for i := range moves {
		newState := applyMoveCopy(state, &moves[i])
		eval, _ := alphaBeta(newState, depth-1, alpha, beta, true, tt, nodes, ply+1)
		if eval < minEval {
			minEval = eval
			bestMove = &moves[i]
		}
		beta = min(beta, eval)
		if beta <= alpha {
			break
		}
	}
	return minEval, bestMove
}

func generateAllMoves(state *contracts.MatchState, forWhite bool) []Move {
	color := "black"
	if forWhite {
		color = "white"
	}

	var moves []Move
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			piece := state.Board[r][c]
			if piece == nil || piece.Color != color {
				continue
			}
			if piece.Frozen {
				continue
			}

			from := contracts.Square{Row: r, Col: c}
			candidates := legalMovesWithFusion(state.Board, from, state.LastMove, sliceToSet(state.Moved))

			for _, to := range candidates {
				moves = append(moves, Move{From: from, To: to})
			}
		}
	}
	return moves
}

func orderMoves(moves []Move, state *contracts.MatchState) {
	for i := range moves {
		score := 0
		captured := state.Board[moves[i].To.Row][moves[i].To.Col]
		if captured != nil {
			attacker := state.Board[moves[i].From.Row][moves[i].From.Col]
			if attacker != nil {
				score += 10 * pieceValue(captured.Type) - pieceValue(attacker.Type)
			}
		}
		if moves[i].To.Row == 3 || moves[i].To.Row == 4 {
			score += 10
		}
		moves[i].Score = score
	}

	sort.SliceStable(moves, func(i, j int) bool {
		return moves[i].Score > moves[j].Score
	})
}

func applyMoveCopy(state *contracts.MatchState, move *Move) *contracts.MatchState {
	newState := cloneMatchState(state)
	piece := newState.Board[move.From.Row][move.From.Col]
	if piece == nil {
		return newState
	}

	captured := newState.Board[move.To.Row][move.To.Col]
	newState.Board[move.To.Row][move.To.Col] = piece
	newState.Board[move.From.Row][move.From.Col] = nil

	if piece.Type == "pawn" && move.From.Col != move.To.Col && captured == nil {
		newState.Board[move.From.Row][move.To.Col] = nil
	}

	if piece.Type == "king" && abs(move.To.Col-move.From.Col) == 2 {
		if move.To.Col == 6 {
			newState.Board[move.From.Row][5] = newState.Board[move.From.Row][7]
			newState.Board[move.From.Row][7] = nil
		} else if move.To.Col == 2 {
			newState.Board[move.From.Row][3] = newState.Board[move.From.Row][0]
			newState.Board[move.From.Row][0] = nil
		}
	}

	if piece.Type == "pawn" && (move.To.Row == 0 || move.To.Row == 7) {
		newState.Board[move.To.Row][move.To.Col] = &contracts.Piece{
			Type:  "queen",
			Color: piece.Color,
		}
	}

	newState.Turn = oppositeColor(piece.Color)
	newState.Moved = append(newState.Moved, keyForSquare(move.From))

	return newState
}

func isKingInCheck(state *contracts.MatchState) bool {
	king := findKingPos(state.Board, state.Turn)
	if king == nil {
		return false
	}
	return isAttackedWithFusion(state.Board, *king, oppositeColor(state.Turn))
}

func cloneMatchState(state *contracts.MatchState) *contracts.MatchState {
	newState := &contracts.MatchState{
		MatchID:     state.MatchID,
		Turn:        state.Turn,
		Status:      state.Status,
		HalfMoveClock: state.HalfMoveClock,
		FullMoveNum: state.FullMoveNum,
		WhiteHand:   append([]contracts.GameCard{}, state.WhiteHand...),
		BlackHand:   append([]contracts.GameCard{}, state.BlackHand...),
		Moved:       append([]string{}, state.Moved...),
		LastMove:    state.LastMove,
	}
	newState.Board = cloneBoard(state.Board)
	return newState
}

func cloneBoard(board [][]*contracts.Piece) [][]*contracts.Piece {
	newBoard := make([][]*contracts.Piece, 8)
	for r := 0; r < 8; r++ {
		newBoard[r] = make([]*contracts.Piece, 8)
		for c := 0; c < 8; c++ {
			if board[r][c] != nil {
				pieceCopy := *board[r][c]
				newBoard[r][c] = &pieceCopy
			}
		}
	}
	return newBoard
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func oppositeColor(color string) string {
	if color == "white" {
		return "black"
	}
	return "white"
}

func keyForSquare(sq contracts.Square) string {
	return keyForCoords(sq.Row, sq.Col)
}

func keyForCoords(row, col int) string {
	return fmt.Sprintf("%d-%d", row, col)
}

func sliceToSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, v := range values {
		out[v] = struct{}{}
	}
	return out
}
