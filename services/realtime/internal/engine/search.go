package engine

import (
	"fmt"
	"math"
	"math/rand"
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
	From      contracts.Square
	To        contracts.Square
	Score     int
	Promotion string // "queen", "rook", "bishop", "knight", or empty for non-promotion
}

type SearchResult struct {
	BestMove Move
	Score    int
	Nodes    int
	Depth    int
}

var defaultHasher *ZobristHasher

func init() {
	defaultHasher = NewZobristHasher(rand.New(rand.NewSource(0)))
}

func Search(state *contracts.MatchState, maxDepth int, tt *TranspositionTable) SearchResult {
	turn := state.Turn
	bestMove := Move{}
	nodes := 0
	prevScore := 0

	for depth := 1; depth <= maxDepth; depth++ {
		alpha := math.MinInt + 1
		beta := math.MaxInt - 1

		// Aspiration window: narrow the search window around the previous
		// iteration's score for tighter alpha-beta bounds.
		if depth >= 3 {
			alpha = prevScore - aspirationDelta
			beta = prevScore + aspirationDelta
		}

		score, move := alphaBeta(state, depth, alpha, beta, turn == "white", tt, &nodes, 0)

		// If the score fell outside the aspiration window, re-search with full
		// window (research is uncommon, ~5-10% of iterations).
		if score <= alpha || score >= beta {
			score, move = alphaBeta(state, depth, math.MinInt+1, math.MaxInt-1, turn == "white", tt, &nodes, 0)
		}

		prevScore = score

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

const (
	lmrMinDepth    = 3       // Don't LMR at very shallow depths
	lmrReduction   = 1       // Base reduction amount
	nullMoveDepth  = 4       // Minimum depth for null-move pruning (avoids overhead at low depths)
	nullMoveR      = 2       // Null-move reduction factor
	aspirationDelta = 50     // Aspiration window delta around previous score
	checkExtension = 0       // Check extension (0=disabled; enables deeper tactical search)
)

func alphaBeta(state *contracts.MatchState, depth, alpha, beta int, maximizing bool, tt *TranspositionTable, nodes *int, ply int) (int, *Move) {
	*nodes++

	// TT probe: use Zobrist hash to look up this position.
	hash := defaultHasher.Hash(state)
	if tt != nil {
		if ok, ttScore := tt.Lookup(hash, depth, alpha, beta); ok {
			return ttScore, nil
		}
	}

	if depth <= 0 {
		return quiescence(state, alpha, beta, maximizing, tt, nodes, ply, hash), nil
	}

	// Null-move pruning: skip a move (forfeit the turn) and see if we're still
	// above beta. If so, the position is too good to need searching.
	if ply > 0 && depth >= nullMoveDepth && !isKingInCheck(state) {
		nullState := cloneMatchState(state)
		nullState.Turn = oppositeColor(state.Turn)
		nullScore, _ := alphaBeta(nullState, depth-nullMoveR-1, -beta, -beta+1, !maximizing, tt, nodes, ply+1)
		nullScore = -nullScore
		if nullScore >= beta {
			return beta, nil
		}
	}

	inCheck := isKingInCheck(state)

	moves := generateAllMoves(state, maximizing)
	if len(moves) == 0 {
		if inCheck {
			if maximizing {
				return -20000 + ply, nil
			}
			return 20000 - ply, nil
		}
		return 0, nil
	}

	orderMoves(moves, state, ply)

	bestMove := &moves[0]
	improving := true
	if ply >= 2 {
		// Simple improving detection: non-pawn material is roughly equal →
		// assume improving unless proven otherwise (simplified).
		improving = true
	}

	if maximizing {
		maxEval := math.MinInt + 1
		for i := range moves {
			newDepth := depth - 1

			// Check extension (expensive per-move, only enabled if >0).
			isCheck := false
			if checkExtension > 0 {
				if piece := state.Board[moves[i].From.Row][moves[i].From.Col]; piece != nil {
					testState := applyMoveCopy(state, &moves[i])
					oppKing := findKingPos(testState.Board, oppositeColor(piece.Color))
					if oppKing != nil && isAttacked(testState.Board, *oppKing, piece.Color) {
						isCheck = true
					}
				}
				if isCheck {
					newDepth += checkExtension
				}
			}

			captured := state.Board[moves[i].To.Row][moves[i].To.Col]
			isCapture := captured != nil
			isPromo := moves[i].Promotion != ""
			if i >= 3 && depth >= lmrMinDepth && !isCapture && !isPromo && !isCheck {
				reduction := lmrReduction
				if i >= 6 {
					reduction = 2
				}
				if !improving {
					reduction++
				}
				newDepth -= reduction
				if newDepth < 0 {
					newDepth = 0
				}
			}

			newState := applyMoveCopy(state, &moves[i])
			eval, _ := alphaBeta(newState, newDepth, alpha, beta, false, tt, nodes, ply+1)

			// If LMR was used and result > alpha, re-search at full depth.
			if newDepth < depth-1 && eval > alpha {
				eval, _ = alphaBeta(newState, depth-1, alpha, beta, false, tt, nodes, ply+1)
			}

			if eval > maxEval {
				maxEval = eval
				bestMove = &moves[i]
			}
			alpha = max(alpha, eval)
			if beta <= alpha {
				storeKillerMove(ply, keyForSquare(moves[i].From)+keyForSquare(moves[i].To))
				attacker := state.Board[moves[i].From.Row][moves[i].From.Col]
				if attacker != nil && !isCapture {
					updateHistory(attacker.Type, moves[i].To.Row*8+moves[i].To.Col, depth)
				}
				break
			}
		}
		if tt != nil {
			flag := ExactScore
			if maxEval <= alpha {
				flag = UpperBound
			} else if maxEval >= beta {
				flag = LowerBound
			}
			tt.Store(hash, depth, maxEval, flag, keyForSquare(bestMove.From)+keyForSquare(bestMove.To))
		}
		return maxEval, bestMove
	}

	minEval := math.MaxInt - 1
	for i := range moves {
		newDepth := depth - 1

		isCheck := false
		if checkExtension > 0 {
			if piece := state.Board[moves[i].From.Row][moves[i].From.Col]; piece != nil {
				testState := applyMoveCopy(state, &moves[i])
				oppKing := findKingPos(testState.Board, oppositeColor(piece.Color))
				if oppKing != nil && isAttacked(testState.Board, *oppKing, piece.Color) {
					isCheck = true
				}
			}
			if isCheck {
				newDepth += checkExtension
			}
		}

		captured := state.Board[moves[i].To.Row][moves[i].To.Col]
		isCapture := captured != nil
		isPromo := moves[i].Promotion != ""
		if i >= 3 && depth >= lmrMinDepth && !isCapture && !isPromo && !isCheck {
			reduction := lmrReduction
			if i >= 6 {
				reduction = 2
			}
			if !improving {
				reduction++
			}
			newDepth -= reduction
			if newDepth < 0 {
				newDepth = 0
			}
		}

		newState := applyMoveCopy(state, &moves[i])
		eval, _ := alphaBeta(newState, newDepth, alpha, beta, true, tt, nodes, ply+1)

		if newDepth < depth-1 && eval < beta {
			eval, _ = alphaBeta(newState, depth-1, alpha, beta, true, tt, nodes, ply+1)
		}

		if eval < minEval {
			minEval = eval
			bestMove = &moves[i]
		}
		beta = min(beta, eval)
		if beta <= alpha {
			storeKillerMove(ply, keyForSquare(moves[i].From)+keyForSquare(moves[i].To))
			attacker := state.Board[moves[i].From.Row][moves[i].From.Col]
			if attacker != nil && !isCapture {
				updateHistory(attacker.Type, moves[i].To.Row*8+moves[i].To.Col, depth)
			}
			break
		}
	}
	if tt != nil {
		flag := ExactScore
		if minEval <= alpha {
			flag = UpperBound
		} else if minEval >= beta {
			flag = LowerBound
		}
		tt.Store(hash, depth, minEval, flag, keyForSquare(bestMove.From)+keyForSquare(bestMove.To))
	}
	return minEval, bestMove
}

// quiescence searches captures at depth 0 (stand-pat) to reduce the horizon
// effect. Returns a score from the perspective of the side to move.
func quiescence(state *contracts.MatchState, alpha, beta int, maximizing bool, tt *TranspositionTable, nodes *int, ply int, hash uint64) int {
	*nodes++

	// Stand-pat: evaluate the current position.
	standPat := EvaluateWithModifiers(state.Board, state.Turn, state.LavaSquares, state.FortressZones, state.BombPieces)
	if !maximizing {
		standPat = -standPat
	}

	if standPat >= beta {
		return beta
	}
	if standPat > alpha {
		alpha = standPat
	}

	// Generate only capture moves for quiescence search.
	captures := generateCaptureMoves(state, maximizing)
	for i := range captures {
		newState := applyMoveCopy(state, &captures[i])
		score := -quiescence(newState, -beta, -alpha, !maximizing, tt, nodes, ply+1, 0)
		if score >= beta {
			return beta
		}
		if score > alpha {
			alpha = score
		}
	}

	return alpha
}

// generateCaptureMoves returns only moves that capture an enemy piece.
func generateCaptureMoves(state *contracts.MatchState, forWhite bool) []Move {
	color := "black"
	if forWhite {
		color = "white"
	}

	var captures []Move
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			piece := state.Board[r][c]
			if piece == nil || piece.Color != color || piece.Frozen {
				continue
			}
			from := contracts.Square{Row: r, Col: c}
			candidates := legalMovesWithFusion(state.Board, from, state.LastMove, sliceToSet(state.Moved))
			for _, to := range candidates {
				if fortressEntryBlocked(state.FortressZones, piece.Color, to) {
					continue
				}
				target := state.Board[to.Row][to.Col]
				if target == nil || target.Color == color {
					continue
				}
				if piece.Type == "pawn" && (to.Row == 0 || to.Row == 7) {
					for _, promo := range []string{"queen", "rook", "bishop", "knight"} {
						captures = append(captures, Move{From: from, To: to, Promotion: promo})
					}
				} else {
					captures = append(captures, Move{From: from, To: to})
				}
			}
		}
	}
	return captures
}

func GenerateAllMoves(state *contracts.MatchState, forWhite bool) []Move {
	return generateAllMoves(state, forWhite)
}

func generateAllMoves(state *contracts.MatchState, forWhite bool) []Move {
	color := "black"
	if forWhite {
		color = "white"
	}

	if state.DoubleMove != nil && state.DoubleMove.MovesLeft > 0 && state.DoubleMove.TrackedSq != nil && state.DoubleMove.Type == "same" {
		tracked := state.DoubleMove.TrackedSq
		piece := state.Board[tracked.Row][tracked.Col]
		if piece != nil && piece.Color == color && !piece.Frozen {
			candidates := legalMovesWithFusion(state.Board, *tracked, state.LastMove, sliceToSet(state.Moved))
			var moves []Move
			for _, to := range candidates {
				if fortressEntryBlocked(state.FortressZones, piece.Color, to) {
					continue
				}
				if piece.Type == "pawn" && (to.Row == 0 || to.Row == 7) {
					for _, promo := range []string{"queen", "rook", "bishop", "knight"} {
						moves = append(moves, Move{From: *tracked, To: to, Promotion: promo})
					}
				} else {
					moves = append(moves, Move{From: *tracked, To: to})
				}
			}
			if state.InvisiblePiece != nil && state.InvisiblePiece.OwnerColor == color && state.InvisiblePiece.RoundsLeft > 0 && !state.InvisiblePiece.Piece.Frozen {
				from := contracts.Square{Row: state.InvisiblePiece.Row, Col: state.InvisiblePiece.Col}
				ghostCandidates := legalMovesWithFusion(state.Board, from, state.LastMove, sliceToSet(state.Moved))
				for _, to := range ghostCandidates {
					if fortressEntryBlocked(state.FortressZones, color, to) {
						continue
					}
					if state.InvisiblePiece.Piece.Type == "pawn" && (to.Row == 0 || to.Row == 7) {
						for _, promo := range []string{"queen", "rook", "bishop", "knight"} {
							moves = append(moves, Move{From: from, To: to, Promotion: promo})
						}
					} else {
						moves = append(moves, Move{From: from, To: to})
					}
				}
			}
			return moves
		}
		return nil
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

			// Double-move "diff": skip the tracked square (only other pieces may move).
			if state.DoubleMove != nil && state.DoubleMove.MovesLeft > 0 &&
				state.DoubleMove.TrackedSq != nil && state.DoubleMove.Type == "diff" {
				if from.Row == state.DoubleMove.TrackedSq.Row && from.Col == state.DoubleMove.TrackedSq.Col {
					continue
				}
			}

			candidates := legalMovesWithFusion(state.Board, from, state.LastMove, sliceToSet(state.Moved))

			for _, to := range candidates {
				if fortressEntryBlocked(state.FortressZones, piece.Color, to) {
					continue
				}
				if piece.Type == "pawn" && (to.Row == 0 || to.Row == 7) {
					for _, promo := range []string{"queen", "rook", "bishop", "knight"} {
						moves = append(moves, Move{From: from, To: to, Promotion: promo})
					}
				} else {
					moves = append(moves, Move{From: from, To: to})
				}
			}
		}
	}

	// Generate moves for the invisible piece if it belongs to the current player.
	if state.InvisiblePiece != nil && state.InvisiblePiece.OwnerColor == color && state.InvisiblePiece.RoundsLeft > 0 {
		from := contracts.Square{Row: state.InvisiblePiece.Row, Col: state.InvisiblePiece.Col}
		invisiblePiece := state.InvisiblePiece.Piece
		if !invisiblePiece.Frozen {
			candidates := legalMovesWithFusion(state.Board, from, state.LastMove, sliceToSet(state.Moved))
			for _, to := range candidates {
				if fortressEntryBlocked(state.FortressZones, color, to) {
					continue
				}
				if invisiblePiece.Type == "pawn" && (to.Row == 0 || to.Row == 7) {
					for _, promo := range []string{"queen", "rook", "bishop", "knight"} {
						moves = append(moves, Move{From: from, To: to, Promotion: promo})
					}
				} else {
					moves = append(moves, Move{From: from, To: to})
				}
			}
		}
	}

	return moves
}

// killerMoves tracks two best non-capture moves per ply as candidates for
// move ordering (they caused a beta cutoff in a sibling node).
var killerMoves [64][2]string // [ply][slot] → "from-to" key

// historyHeuristic tracks how often a move (from→to for a given piece type)
// causes a beta cutoff, indexed by [pieceTypeIndex][toSquare].
var historyHeuristic [6][64]int

func resetKillersAndHistory() {
	for i := range killerMoves {
		killerMoves[i] = [2]string{}
	}
	for i := range historyHeuristic {
		for j := range historyHeuristic[i] {
			historyHeuristic[i][j] = 0
		}
	}
}

func pieceTypeIndex(pieceType string) int {
	switch pieceType {
	case "pawn":
		return 0
	case "knight":
		return 1
	case "bishop":
		return 2
	case "rook":
		return 3
	case "queen":
		return 4
	case "king":
		return 5
	}
	return 0
}

func orderMoves(moves []Move, state *contracts.MatchState, ply int) {
	for i := range moves {
		score := 0
		captured := state.Board[moves[i].To.Row][moves[i].To.Col]
		if captured != nil {
			attacker := state.Board[moves[i].From.Row][moves[i].From.Col]
			if attacker != nil {
				// MVV-LVA: most-valuable victim, least-valuable attacker
				score += 1000 + 10*pieceValue(captured.Type) - pieceValue(attacker.Type)
			}
		}
		if moves[i].Score > 0 {
			score += 100 // TT best move priority
		}
		if moves[i].To.Row == 3 || moves[i].To.Row == 4 {
			score += 10
		}
		// Promotion bonus
		if moves[i].Promotion != "" {
			if moves[i].Promotion == "queen" {
				score += 900
			} else {
				score += 200
			}
		}
		moves[i].Score = score
	}

	// Apply killer-move bonus and history-heuristic bonus.
	for i := range moves {
		key := keyForSquare(moves[i].From) + keyForSquare(moves[i].To)
		// Check killer slots for the current ply
		kp := ply
		if kp >= len(killerMoves) {
			kp = 0
		}
		for _, k := range killerMoves[kp] {
			if k == key && k != "" {
				moves[i].Score += 500
				break
			}
		}
		// History heuristic bonus
		attacker := state.Board[moves[i].From.Row][moves[i].From.Col]
		if attacker != nil {
			idx := pieceTypeIndex(attacker.Type)
			moves[i].Score += historyHeuristic[idx][moves[i].To.Row*8+moves[i].To.Col]
		}
	}

	sort.SliceStable(moves, func(i, j int) bool {
		return moves[i].Score > moves[j].Score
	})
}

// storeKillerMove records a move that caused a beta cutoff at the given ply.
func storeKillerMove(ply int, moveKey string) {
	if ply >= len(killerMoves) {
		return
	}
	// Shift and store in the first slot (most recent).
	killerMoves[ply][1] = killerMoves[ply][0]
	killerMoves[ply][0] = moveKey
}

// updateHistory increments the history counter for a move that caused a cutoff.
func updateHistory(pieceType string, toSquare int, depth int) {
	idx := pieceTypeIndex(pieceType)
	historyHeuristic[idx][toSquare] += depth * depth
}

func ApplyMoveCopy(state *contracts.MatchState, move *Move) *contracts.MatchState {
	return applyMoveCopy(state, move)
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
			newState.Moved = append(newState.Moved, keyForCoords(move.From.Row, 7))
		} else if move.To.Col == 2 {
			newState.Board[move.From.Row][3] = newState.Board[move.From.Row][0]
			newState.Board[move.From.Row][0] = nil
			newState.Moved = append(newState.Moved, keyForCoords(move.From.Row, 0))
		}
	}

	if piece.Type == "pawn" && (move.To.Row == 0 || move.To.Row == 7) {
		promoType := move.Promotion
		if promoType == "" {
			promoType = "queen"
		}
		newState.Board[move.To.Row][move.To.Col] = &contracts.Piece{
			Type:  promoType,
			Color: piece.Color,
		}
	}

	justMovedColor := newState.Turn
	newState.Turn = oppositeColor(piece.Color)
	newState.Moved = append(newState.Moved, keyForSquare(move.From))
	newState.LastMove = &contracts.LastMove{From: move.From, To: move.To}

	if newState.DoubleMove != nil {
		newMovesLeft := newState.DoubleMove.MovesLeft - 1
		if newMovesLeft > 0 {
			tracked := contracts.Square{Row: move.To.Row, Col: move.To.Col}
			newState.DoubleMove = &contracts.DoubleMoveState{
				Type:      newState.DoubleMove.Type,
				MovesLeft: newMovesLeft,
				TrackedSq: &tracked,
			}
		} else {
			newState.DoubleMove = nil
		}
	}

	if newState.InvisiblePiece != nil && newState.InvisiblePiece.OwnerColor == justMovedColor {
		from := contracts.Square{Row: newState.InvisiblePiece.Row, Col: newState.InvisiblePiece.Col}
		if from.Row == move.From.Row && from.Col == move.From.Col {
			ghostBoard := cloneBoard(newState.Board)
			ghostBoard[move.To.Row][move.To.Col] = &contracts.Piece{
				Type:  newState.InvisiblePiece.Piece.Type,
				Color: newState.InvisiblePiece.Piece.Color,
			}
			oppKing := findKingPos(ghostBoard, newState.Turn)
			givesCheck := oppKing != nil && isAttacked(ghostBoard, *oppKing, justMovedColor)
			isMove2 := newState.InvisiblePiece.RoundsLeft <= 0
			if givesCheck || isMove2 {
				newState.InvisiblePiece = nil
			} else {
				newState.InvisiblePiece.Row = move.To.Row
				newState.InvisiblePiece.Col = move.To.Col
				newState.InvisiblePiece.RoundsLeft--
			}
		} else {
			if newState.InvisiblePiece.RoundsLeft > 0 {
				newState.InvisiblePiece.RoundsLeft--
			}
			if newState.InvisiblePiece.RoundsLeft <= 0 {
				newState.InvisiblePiece = nil
			}
		}
	} else if newState.InvisiblePiece != nil && newState.InvisiblePiece.OwnerColor != justMovedColor {
		if newState.InvisiblePiece.RoundsLeft > 0 {
			newState.InvisiblePiece.RoundsLeft--
		}
		if newState.InvisiblePiece.RoundsLeft <= 0 {
			newState.InvisiblePiece = nil
		}
	}

	return newState
}

func IsKingInCheck(state *contracts.MatchState) bool {
	return isKingInCheck(state)
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
	if state.DoubleMove != nil {
		dm := *state.DoubleMove
		if state.DoubleMove.TrackedSq != nil {
			tracked := *state.DoubleMove.TrackedSq
			dm.TrackedSq = &tracked
		}
		newState.DoubleMove = &dm
	}
	if state.InvisiblePiece != nil {
		ip := *state.InvisiblePiece
		newState.InvisiblePiece = &ip
	}
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
