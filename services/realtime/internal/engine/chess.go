package engine

import (
	"github.com/chess404/realtime/internal/contracts"
)

func pieceAt(board [][]*contracts.Piece, square contracts.Square) *contracts.Piece {
	if square.Row < 0 || square.Row > 7 || square.Col < 0 || square.Col > 7 {
		return nil
	}
	return board[square.Row][square.Col]
}

func inBounds(r, c int) bool {
	return r >= 0 && r <= 7 && c >= 0 && c <= 7
}

func clearPath(board [][]*contracts.Piece, from, to contracts.Square) bool {
	dr := sign(to.Row - from.Row)
	dc := sign(to.Col - from.Col)
	r, c := from.Row+dr, from.Col+dc
	for r != to.Row || c != to.Col {
		if board[r][c] != nil {
			return false
		}
		r += dr
		c += dc
	}
	return true
}

func sign(value int) int {
	switch {
	case value < 0:
		return -1
	case value > 0:
		return 1
	default:
		return 0
	}
}

func attacks(board [][]*contracts.Piece, from, to contracts.Square, piece *contracts.Piece) bool {
	dr := to.Row - from.Row
	dc := to.Col - from.Col
	switch piece.Type {
	case "pawn":
		dir := 1
		if piece.Color == "black" {
			dir = -1
		}
		return dr == dir && abs(dc) == 1
	case "knight":
		return (abs(dr) == 2 && abs(dc) == 1) || (abs(dr) == 1 && abs(dc) == 2)
	case "king":
		return abs(dr) <= 1 && abs(dc) <= 1 && (dr != 0 || dc != 0)
	case "rook":
		if dr != 0 && dc != 0 {
			return false
		}
		return clearPath(board, from, to)
	case "bishop":
		if abs(dr) != abs(dc) {
			return false
		}
		return clearPath(board, from, to)
	case "queen":
		if dr == 0 || dc == 0 || abs(dr) == abs(dc) {
			return clearPath(board, from, to)
		}
		return false
	}
	return false
}

func isAttacked(board [][]*contracts.Piece, target contracts.Square, by string) bool {
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			piece := board[r][c]
			if piece != nil && piece.Color == by && attacks(board, contracts.Square{Row: r, Col: c}, target, piece) {
				return true
			}
		}
	}
	return false
}

func isAttackedWithFusion(board [][]*contracts.Piece, target contracts.Square, by string) bool {
	if isAttacked(board, target, by) {
		return true
	}
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			piece := board[r][c]
			if piece == nil || piece.Color != by || piece.FusedWith == "" {
				continue
			}
			fused := clonePieceAsType(piece, piece.FusedWith)
			if attacks(board, contracts.Square{Row: r, Col: c}, target, fused) {
				return true
			}
		}
	}
	return false
}

func clonePieceAsType(piece *contracts.Piece, pieceType string) *contracts.Piece {
	if piece == nil {
		return nil
	}
	return &contracts.Piece{
		Type:           pieceType,
		Color:          piece.Color,
		Shielded:       piece.Shielded,
		ShieldTurn:     piece.ShieldTurn,
		Frozen:         piece.Frozen,
		Borrowed:       piece.Borrowed,
		ParasiteTarget: piece.ParasiteTarget,
		Bomb:           piece.Bomb,
		Invisible:      piece.Invisible,
		InvisibleTurn:  piece.InvisibleTurn,
		InvisibleOver:  piece.InvisibleOver,
		FusedWith:      piece.FusedWith,
		Fake:           piece.Fake,
	}
}

func legalMoves(board [][]*contracts.Piece, from contracts.Square, lastMove *contracts.LastMove, moved map[string]struct{}) []contracts.Square {
	piece := pieceAt(board, from)
	if piece == nil {
		return nil
	}

	candidates := pseudoMoves(board, from, lastMove, moved)
	legal := make([]contracts.Square, 0, len(candidates))

	for _, move := range candidates {
		nextBoard := cloneBoard(board)
		moving := nextBoard[from.Row][from.Col]
		captured := nextBoard[move.Row][move.Col]
		nextBoard[move.Row][move.Col] = moving
		nextBoard[from.Row][from.Col] = nil
		if moving != nil && moving.Type == "pawn" && move.Col != from.Col && captured == nil {
			nextBoard[from.Row][move.Col] = nil
		}
		king := findKing(nextBoard, piece.Color)
		if king == nil {
			continue
		}
		if !isAttackedWithFusion(nextBoard, *king, oppositeColor(piece.Color)) {
			legal = append(legal, move)
		}
	}

	return legal
}

func legalMovesWithFusion(board [][]*contracts.Piece, from contracts.Square, lastMove *contracts.LastMove, moved map[string]struct{}) []contracts.Square {
	piece := pieceAt(board, from)
	if piece == nil {
		return nil
	}
	if piece.FusedWith == "" {
		return legalMoves(board, from, lastMove, moved)
	}

	merged := make([]contracts.Square, 0, 16)
	seen := make(map[string]struct{})
	appendMoves := func(moves []contracts.Square) {
		for _, move := range moves {
			key := keyForSquareGo(move)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, move)
		}
	}

	appendMoves(legalMoves(board, from, lastMove, moved))

	transformedBoard := cloneBoard(board)
	transformedBoard[from.Row][from.Col] = clonePieceAsType(piece, piece.FusedWith)
	appendMoves(legalMoves(transformedBoard, from, lastMove, moved))

	return merged
}

func findKing(board [][]*contracts.Piece, color string) *contracts.Square {
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			piece := board[r][c]
			if piece != nil && piece.Type == "king" && piece.Color == color {
				return &contracts.Square{Row: r, Col: c}
			}
		}
	}
	return nil
}

func pseudoMoves(board [][]*contracts.Piece, from contracts.Square, lastMove *contracts.LastMove, moved map[string]struct{}) []contracts.Square {
	piece := pieceAt(board, from)
	if piece == nil {
		return nil
	}

	moves := []contracts.Square{}
	canTarget := func(r, c int) bool {
		return inBounds(r, c) && (board[r][c] == nil || board[r][c].Color != piece.Color)
	}
	slide := func(dirs [][2]int) {
		for _, dir := range dirs {
			for i := 1; i <= 7; i++ {
				r := from.Row + dir[0]*i
				c := from.Col + dir[1]*i
				if !inBounds(r, c) || (board[r][c] != nil && board[r][c].Color == piece.Color) {
					break
				}
				moves = append(moves, contracts.Square{Row: r, Col: c})
				if board[r][c] != nil {
					break
				}
			}
		}
	}

	switch piece.Type {
	case "pawn":
		dir := 1
		startRow := 1
		if piece.Color == "black" {
			dir = -1
			startRow = 6
		}
		if inBounds(from.Row+dir, from.Col) && board[from.Row+dir][from.Col] == nil {
			moves = append(moves, contracts.Square{Row: from.Row + dir, Col: from.Col})
			if from.Row == startRow && board[from.Row+2*dir][from.Col] == nil {
				moves = append(moves, contracts.Square{Row: from.Row + 2*dir, Col: from.Col})
			}
		}
		for _, dc := range []int{-1, 1} {
			nr, nc := from.Row+dir, from.Col+dc
			if inBounds(nr, nc) && board[nr][nc] != nil && board[nr][nc].Color != piece.Color {
				moves = append(moves, contracts.Square{Row: nr, Col: nc})
			}
		}
		if lastMove != nil {
			lastPiece := pieceAt(board, lastMove.To)
			if lastPiece != nil &&
				lastPiece.Type == "pawn" &&
				abs(lastMove.From.Row-lastMove.To.Row) == 2 &&
				lastMove.To.Row == from.Row &&
				abs(lastMove.To.Col-from.Col) == 1 {
				moves = append(moves, contracts.Square{Row: from.Row + dir, Col: lastMove.To.Col})
			}
		}
	case "knight":
		deltas := [][2]int{{-2, -1}, {-2, 1}, {-1, -2}, {-1, 2}, {1, -2}, {1, 2}, {2, -1}, {2, 1}}
		for _, delta := range deltas {
			r, c := from.Row+delta[0], from.Col+delta[1]
			if canTarget(r, c) {
				moves = append(moves, contracts.Square{Row: r, Col: c})
			}
		}
	case "bishop":
		slide([][2]int{{1, 1}, {1, -1}, {-1, 1}, {-1, -1}})
	case "rook":
		slide([][2]int{{0, 1}, {0, -1}, {1, 0}, {-1, 0}})
	case "queen":
		slide([][2]int{{0, 1}, {0, -1}, {1, 0}, {-1, 0}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}})
	case "king":
		deltas := [][2]int{{0, 1}, {0, -1}, {1, 0}, {-1, 0}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}}
		for _, delta := range deltas {
			r, c := from.Row+delta[0], from.Col+delta[1]
			if canTarget(r, c) {
				moves = append(moves, contracts.Square{Row: r, Col: c})
			}
		}
		if _, movedKing := moved[keyForSquareGo(from)]; !movedKing && !isAttackedWithFusion(board, from, oppositeColor(piece.Color)) {
			if _, rookMoved := moved[keyForCoordsGo(from.Row, 7)]; !rookMoved &&
				pieceAt(board, contracts.Square{Row: from.Row, Col: 7}) != nil &&
				pieceAt(board, contracts.Square{Row: from.Row, Col: 7}).Type == "rook" &&
				pieceAt(board, contracts.Square{Row: from.Row, Col: 7}).Color == piece.Color &&
				pieceAt(board, contracts.Square{Row: from.Row, Col: 5}) == nil &&
				pieceAt(board, contracts.Square{Row: from.Row, Col: 6}) == nil &&
				!isAttackedWithFusion(board, contracts.Square{Row: from.Row, Col: 5}, oppositeColor(piece.Color)) &&
				!isAttackedWithFusion(board, contracts.Square{Row: from.Row, Col: 6}, oppositeColor(piece.Color)) {
				moves = append(moves, contracts.Square{Row: from.Row, Col: 6})
			}
			if _, rookMoved := moved[keyForCoordsGo(from.Row, 0)]; !rookMoved &&
				pieceAt(board, contracts.Square{Row: from.Row, Col: 0}) != nil &&
				pieceAt(board, contracts.Square{Row: from.Row, Col: 0}).Type == "rook" &&
				pieceAt(board, contracts.Square{Row: from.Row, Col: 0}).Color == piece.Color &&
				pieceAt(board, contracts.Square{Row: from.Row, Col: 1}) == nil &&
				pieceAt(board, contracts.Square{Row: from.Row, Col: 2}) == nil &&
				pieceAt(board, contracts.Square{Row: from.Row, Col: 3}) == nil &&
				!isAttackedWithFusion(board, contracts.Square{Row: from.Row, Col: 3}, oppositeColor(piece.Color)) &&
				!isAttackedWithFusion(board, contracts.Square{Row: from.Row, Col: 2}, oppositeColor(piece.Color)) {
				moves = append(moves, contracts.Square{Row: from.Row, Col: 2})
			}
		}
	}

	return moves
}

func keyForSquareGo(sq contracts.Square) string {
	return keyForCoordsGo(sq.Row, sq.Col)
}

func keyForCoordsGo(row, col int) string {
	return string(rune('0'+row)) + "-" + string(rune('0'+col))
}
