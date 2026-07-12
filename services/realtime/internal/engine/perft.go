package engine

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/chess404/realtime/internal/contracts"
)

// MatchStateFromFEN parses a standard chess FEN string into a MatchState
// suitable for perft testing. Supports castling rights and en passant.
func MatchStateFromFEN(fen string) *contracts.MatchState {
	parts := strings.Fields(fen)
	if len(parts) < 2 {
		return nil
	}

	board := make([][]*contracts.Piece, 8)
	for r := 0; r < 8; r++ {
		board[r] = make([]*contracts.Piece, 8)
	}

	rows := strings.Split(parts[0], "/")
	if len(rows) != 8 {
		return nil
	}
	for fenRow, rowStr := range rows {
		boardRow := 7 - fenRow
		col := 0
		for _, ch := range rowStr {
			if ch >= '1' && ch <= '8' {
				col += int(ch - '0')
			} else {
				pieceType := fenCharToPieceType(ch)
				color := "white"
				if ch >= 'a' && ch <= 'z' {
					color = "black"
				}
				board[boardRow][col] = &contracts.Piece{Type: pieceType, Color: color}
				col++
			}
		}
	}

	turn := "white"
	if len(parts) > 1 && parts[1] == "b" {
		turn = "black"
	}

	var moved []string
	if len(parts) > 2 {
		castling := parts[2]
		if castling != "-" {
			if !strings.Contains(castling, "K") {
				moved = append(moved, "0-7")
			}
			if !strings.Contains(castling, "Q") {
				moved = append(moved, "0-0")
			}
			if !strings.Contains(castling, "k") {
				moved = append(moved, "7-7")
			}
			if !strings.Contains(castling, "q") {
				moved = append(moved, "7-0")
			}
		} else {
			// No castling rights: all rooks are considered moved
			moved = append(moved, "0-7", "0-0", "7-7", "7-0")
		}
	}

	state := &contracts.MatchState{
		Board:  board,
		Turn:   turn,
		Moved:  moved,
		Status: "active",
	}

	if len(parts) > 3 && parts[3] != "-" {
		ep := parts[3]
		epCol := int(ep[0] - 'a')
		epRow := int(ep[1] - '1')
		// The en-passant target square is midway between the double-push pawn's
		// from/to squares. If it's white's turn, black just double-pushed
		// (board rows decrease for black, so epRow is between from-1 and to+1).
		if turn == "white" {
			// Black double-pushed: epRow is the square the pawn passed.
			// Black moves from epRow+1 (higher row) to epRow-1 (lower row).
			state.LastMove = &contracts.LastMove{
				From: contracts.Square{Row: epRow + 1, Col: epCol},
				To:   contracts.Square{Row: epRow - 1, Col: epCol},
			}
		} else {
			// White double-pushed: moves from epRow-1 (lower row) to epRow+1.
			state.LastMove = &contracts.LastMove{
				From: contracts.Square{Row: epRow - 1, Col: epCol},
				To:   contracts.Square{Row: epRow + 1, Col: epCol},
			}
		}
	}

	if len(parts) > 4 {
		state.HalfMoveClock, _ = strconv.Atoi(parts[4])
	}
	if len(parts) > 5 {
		state.FullMoveNum, _ = strconv.Atoi(parts[5])
	}

	return state
}

func fenCharToPieceType(ch rune) string {
	switch ch {
	case 'P', 'p':
		return "pawn"
	case 'N', 'n':
		return "knight"
	case 'B', 'b':
		return "bishop"
	case 'R', 'r':
		return "rook"
	case 'Q', 'q':
		return "queen"
	case 'K', 'k':
		return "king"
	}
	return "pawn"
}

// Perft counts the number of legal positions reachable from state at the
// given depth. depth=0 returns 1 (the current position).
func Perft(state *contracts.MatchState, depth int) int {
	if depth == 0 {
		return 1
	}
	nodes := 0
	isWhite := state.Turn == "white"
	moves := generateAllMoves(state, isWhite)
	for i := range moves {
		next := applyMoveCopy(state, &moves[i])
		if depth == 1 {
			nodes++
		} else {
			nodes += Perft(next, depth-1)
		}
	}
	return nodes
}

// PerftDivide runs perft at the given depth and returns a map of move
// notation → node count for all root moves.
func PerftDivide(state *contracts.MatchState, depth int) map[string]int {
	result := make(map[string]int)
	isWhite := state.Turn == "white"
	moves := generateAllMoves(state, isWhite)
	for i := range moves {
		next := applyMoveCopy(state, &moves[i])
		uci := moveToUCI(&moves[i])
		count := 1
		if depth > 1 {
			count = Perft(next, depth-1)
		}
		result[uci] = count
	}
	return result
}

func MoveToUCI(m *Move) string {
	return moveToUCI(m)
}

func moveToUCI(m *Move) string {
	var fromBuf, toBuf [2]byte
	fromBuf[0] = byte('a' + m.From.Col)
	fromBuf[1] = byte('1' + m.From.Row)
	toBuf[0] = byte('a' + m.To.Col)
	toBuf[1] = byte('1' + m.To.Row)
	return string(fromBuf[:]) + string(toBuf[:])
}

// PrintPerftDivide formats perft divide output as a table.
func PrintPerftDivide(divide map[string]int) string {
	total := 0
	var lines []string
	for move, count := range divide {
		lines = append(lines, fmt.Sprintf("%s: %d", move, count))
		total += count
	}
	lines = append(lines, "", fmt.Sprintf("Total: %d", total))
	return strings.Join(lines, "\n")
}


