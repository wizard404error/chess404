package match

import (
	"fmt"
	"strings"

	"github.com/chess404/realtime/internal/contracts"
)

func makeBoard() [][]*contracts.Piece {
	board := make([][]*contracts.Piece, 8)
	for i := range board {
		board[i] = make([]*contracts.Piece, 8)
	}

	backRank := []string{"rook", "knight", "bishop", "queen", "king", "bishop", "knight", "rook"}
	for c, pieceType := range backRank {
		board[0][c] = &contracts.Piece{Type: pieceType, Color: "white"}
		board[7][c] = &contracts.Piece{Type: pieceType, Color: "black"}
	}
	for c := 0; c < 8; c++ {
		board[1][c] = &contracts.Piece{Type: "pawn", Color: "white"}
		board[6][c] = &contracts.Piece{Type: "pawn", Color: "black"}
	}

	return board
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
		if !isAttackedWithFusion(nextBoard, *king, opposite(piece.Color)) {
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
			key := keyForSquare(move)
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

func hasLegalMoveWithFusion(board [][]*contracts.Piece, color string, lastMove *contracts.LastMove, moved map[string]struct{}) bool {
	opponent := opposite(color)
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			piece := board[r][c]
			if piece == nil || piece.Color != color {
				continue
			}

			from := contracts.Square{Row: r, Col: c}
			moves := legalMovesWithFusion(board, from, lastMove, moved)
			for _, move := range moves {
				testBoard := cloneBoard(board)
				moving := testBoard[from.Row][from.Col]
				if moving == nil {
					continue
				}
				captureEmptyDiagonal := moving.Type == "pawn" && move.Col != from.Col && pieceAt(board, move) == nil
				movePiece(testBoard, from, move, moving, captureEmptyDiagonal)
				king := findKing(testBoard, color)
				if king != nil && !isAttackedWithFusion(testBoard, *king, opponent) {
					return true
				}
			}
		}
	}

	return false
}

func gameStatusWithFusion(board [][]*contracts.Piece, player string, lastMove *contracts.LastMove, moved map[string]struct{}) (bool, bool, bool) {
	king := findKing(board, player)
	if king == nil {
		return false, false, false
	}

	inCheck := isAttackedWithFusion(board, *king, opposite(player))
	hasLegal := hasLegalMoveWithFusion(board, player, lastMove, moved)
	return inCheck, inCheck && !hasLegal, !inCheck && !hasLegal
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
		if _, movedKing := moved[keyForSquare(from)]; !movedKing && !isAttackedWithFusion(board, from, opposite(piece.Color)) {
			if _, rookMoved := moved[keyForCoords(from.Row, 7)]; !rookMoved &&
				isRookPiece(pieceAt(board, contracts.Square{Row: from.Row, Col: 7}), piece.Color) &&
				pieceAt(board, contracts.Square{Row: from.Row, Col: 5}) == nil &&
				pieceAt(board, contracts.Square{Row: from.Row, Col: 6}) == nil &&
				!isAttackedWithFusion(board, contracts.Square{Row: from.Row, Col: 5}, opposite(piece.Color)) &&
				!isAttackedWithFusion(board, contracts.Square{Row: from.Row, Col: 6}, opposite(piece.Color)) {
				moves = append(moves, contracts.Square{Row: from.Row, Col: 6})
			}
			if _, rookMoved := moved[keyForCoords(from.Row, 0)]; !rookMoved &&
				isRookPiece(pieceAt(board, contracts.Square{Row: from.Row, Col: 0}), piece.Color) &&
				pieceAt(board, contracts.Square{Row: from.Row, Col: 1}) == nil &&
				pieceAt(board, contracts.Square{Row: from.Row, Col: 2}) == nil &&
				pieceAt(board, contracts.Square{Row: from.Row, Col: 3}) == nil &&
				!isAttackedWithFusion(board, contracts.Square{Row: from.Row, Col: 3}, opposite(piece.Color)) &&
				!isAttackedWithFusion(board, contracts.Square{Row: from.Row, Col: 2}, opposite(piece.Color)) {
				moves = append(moves, contracts.Square{Row: from.Row, Col: 2})
			}
		}
	}

	return moves
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

func pieceTypes(piece *contracts.Piece) []string {
	types := []string{piece.Type}
	if piece.FusedWith != "" {
		types = append(types, piece.FusedWith)
	}
	return types
}

func hasEffectiveType(piece *contracts.Piece, targetType string) bool {
	if piece.Type == targetType {
		return true
	}
	if piece.FusedWith == targetType {
		return true
	}
	return false
}

func insufficientMaterial(board [][]*contracts.Piece) bool {
	nonKings := make([]*contracts.Piece, 0, 8)
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			piece := board[r][c]
			if piece != nil && piece.Type != "king" {
				nonKings = append(nonKings, piece)
			}
		}
	}

	switch len(nonKings) {
	case 0:
		return true
	case 1:
		return hasEffectiveType(nonKings[0], "bishop") || hasEffectiveType(nonKings[0], "knight")
	case 2:
		// KBN vs K is a known forced mate; never draw. Two knights vs lone king is also
		// not a forced draw. The only true insufficient-material positions are:
		//   K vs K, KB vs K, KN vs K, KBB vs K (same-color bishops).
		// KBN vs K is decidable mate, so we must NOT declare it a draw.
		// We treat any bishop+knight combination on a single side as still winnable.
		// (Same-color bishops on KBB vs K are also decided; we do not need to enumerate
		//  every position, we just need to avoid false positives.)
		return (hasEffectiveType(nonKings[0], "bishop") && hasEffectiveType(nonKings[1], "bishop")) ||
			(hasEffectiveType(nonKings[0], "knight") && hasEffectiveType(nonKings[1], "knight"))
	default:
		return false
	}
}

func positionKey(board [][]*contracts.Piece, turn string, moved map[string]struct{}, lastMove *contracts.LastMove) string {
	castling := ""
	if _, movedWhiteKing := moved["0-4"]; !movedWhiteKing && pieceAt(board, contracts.Square{Row: 0, Col: 4}) != nil && pieceAt(board, contracts.Square{Row: 0, Col: 4}).Type == "king" {
		if _, movedWhiteRookKing := moved["0-7"]; !movedWhiteRookKing && pieceAt(board, contracts.Square{Row: 0, Col: 7}) != nil && pieceAt(board, contracts.Square{Row: 0, Col: 7}).Type == "rook" {
			castling += "K"
		}
		if _, movedWhiteRookQueen := moved["0-0"]; !movedWhiteRookQueen && pieceAt(board, contracts.Square{Row: 0, Col: 0}) != nil && pieceAt(board, contracts.Square{Row: 0, Col: 0}).Type == "rook" {
			castling += "Q"
		}
	}
	if _, movedBlackKing := moved["7-4"]; !movedBlackKing && pieceAt(board, contracts.Square{Row: 7, Col: 4}) != nil && pieceAt(board, contracts.Square{Row: 7, Col: 4}).Type == "king" {
		if _, movedBlackRookKing := moved["7-7"]; !movedBlackRookKing && pieceAt(board, contracts.Square{Row: 7, Col: 7}) != nil && pieceAt(board, contracts.Square{Row: 7, Col: 7}).Type == "rook" {
			castling += "k"
		}
		if _, movedBlackRookQueen := moved["7-0"]; !movedBlackRookQueen && pieceAt(board, contracts.Square{Row: 7, Col: 0}) != nil && pieceAt(board, contracts.Square{Row: 7, Col: 0}).Type == "rook" {
			castling += "q"
		}
	}

	enPassant := "-"
	if lastMove != nil {
		lastPiece := pieceAt(board, lastMove.To)
		if lastPiece != nil && lastPiece.Type == "pawn" && abs(lastMove.From.Row-lastMove.To.Row) == 2 {
			enPassant = fileLabels[lastMove.To.Col] + rankLabels[(lastMove.From.Row+lastMove.To.Row)/2]
		}
	}

	return fmt.Sprintf("%s|%c|%s|%s", boardPositionString(board), turn[0], fallbackCastling(castling), enPassant)
}

func threefold(history []string, current string) bool {
	count := 0
	for _, entry := range history {
		if entry == current {
			count++
		}
	}
	return count >= 3
}

func movePiece(board [][]*contracts.Piece, from, to contracts.Square, piece *contracts.Piece, captureEmptyDiagonal bool) {
	board[to.Row][to.Col] = piece
	board[from.Row][from.Col] = nil

	if piece != nil && piece.Type == "pawn" && from.Col != to.Col && captureEmptyDiagonal {
		board[from.Row][to.Col] = nil
	}

	if piece != nil && piece.Type == "king" && abs(to.Col-from.Col) == 2 {
		if to.Col == 6 {
			board[from.Row][5] = board[from.Row][7]
			board[from.Row][7] = nil
		} else if to.Col == 2 {
			board[from.Row][3] = board[from.Row][0]
			board[from.Row][0] = nil
		}
	}
}

func moveNotation(board [][]*contracts.Piece, from, to contracts.Square, piece *contracts.Piece, capture bool) string {
	if piece == nil {
		return ""
	}
	if piece.Type == "king" && abs(to.Col-from.Col) == 2 {
		if to.Col == 6 {
			return "O-O"
		}
		return "O-O-O"
	}

	files := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	ranks := []string{"1", "2", "3", "4", "5", "6", "7", "8"}
	toSq := files[to.Col] + ranks[to.Row]

	notation := ""
	if piece.Type != "pawn" {
		switch piece.Type {
		case "knight":
			notation = "N"
		case "bishop":
			notation = "B"
		case "rook":
			notation = "R"
		case "queen":
			notation = "Q"
		case "king":
			notation = "K"
		default:
			return notation + toSq
		}

		var ambiguous []contracts.Square
		for r := 0; r < 8; r++ {
			for c := 0; c < 8; c++ {
				other := board[r][c]
				if other != nil && other.Type == piece.Type && other.Color == piece.Color && !(r == from.Row && c == from.Col) {
					moves := pseudoMoves(board, contracts.Square{Row: r, Col: c}, nil, nil)
					for _, m := range moves {
						if m.Row == to.Row && m.Col == to.Col {
							ambiguous = append(ambiguous, contracts.Square{Row: r, Col: c})
						}
					}
				}
			}
		}
		if len(ambiguous) > 0 {
			sameFile := false
			sameRank := false
			for _, a := range ambiguous {
				if a.Col == from.Col {
					sameFile = true
				}
				if a.Row == from.Row {
					sameRank = true
				}
			}
			if !sameFile {
				notation += files[from.Col]
			} else if !sameRank {
				notation += ranks[from.Row]
			} else {
				notation += files[from.Col] + ranks[from.Row]
			}
		}
	}
	if piece.Type == "pawn" && capture {
		notation += files[from.Col]
	}
	if capture {
		notation += "x"
	}
	return notation + toSq
}

func pieceAt(board [][]*contracts.Piece, square contracts.Square) *contracts.Piece {
	if !inBounds(square.Row, square.Col) {
		return nil
	}
	return board[square.Row][square.Col]
}

func isRookPiece(piece *contracts.Piece, expectedColor string) bool {
	if piece == nil || piece.Type != "rook" || piece.Color != expectedColor {
		return false
	}
	return true
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
	}
}

func inBounds(r, c int) bool {
	return r >= 0 && r <= 7 && c >= 0 && c <= 7
}

func containsSquare(squares []contracts.Square, target contracts.Square) bool {
	for _, square := range squares {
		if square.Row == target.Row && square.Col == target.Col {
			return true
		}
	}
	return false
}

func keyForSquare(square contracts.Square) string {
	return keyForCoords(square.Row, square.Col)
}

func squareName(square contracts.Square) string {
	files := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	ranks := []string{"1", "2", "3", "4", "5", "6", "7", "8"}
	return files[square.Col] + ranks[square.Row]
}

func keyForCoords(row, col int) string {
	return fmt.Sprintf("%d-%d", row, col)
}

func sliceToSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func nextHalfMoveClock(current int, pieceType string, captured bool) int {
	if pieceType == "pawn" || captured {
		return 0
	}
	return current + 1
}

func boardPositionString(board [][]*contracts.Piece) string {
	var builder strings.Builder
	for r := 0; r < 8; r++ {
		if r > 0 {
			builder.WriteByte('|')
		}
		for c := 0; c < 8; c++ {
			piece := board[r][c]
			if piece == nil {
				builder.WriteByte('-')
				continue
			}
			builder.WriteByte(piece.Color[0])
			builder.WriteByte(piece.Type[0])
		}
	}
	return builder.String()
}

func fallbackCastling(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

var fileLabels = [...]string{"a", "b", "c", "d", "e", "f", "g", "h"}
var rankLabels = [...]string{"1", "2", "3", "4", "5", "6", "7", "8"}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
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
