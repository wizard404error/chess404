package engine

import (
	"github.com/chess404/realtime/internal/contracts"
)

var pieceValues = map[string]int{
	"pawn":   100,
	"knight": 320,
	"bishop": 330,
	"rook":   500,
	"queen":  900,
	"king":   20000,
}

var pawnTable = [8][8]int{
	{0, 0, 0, 0, 0, 0, 0, 0},
	{50, 50, 50, 50, 50, 50, 50, 50},
	{10, 10, 20, 30, 30, 20, 10, 10},
	{5, 5, 10, 25, 25, 10, 5, 5},
	{0, 0, 0, 20, 20, 0, 0, 0},
	{5, -5, -10, 0, 0, -10, -5, 5},
	{5, 10, 10, -20, -20, 10, 10, 5},
	{0, 0, 0, 0, 0, 0, 0, 0},
}

var knightTable = [8][8]int{
	{-50, -40, -30, -30, -30, -30, -40, -50},
	{-40, -20, 0, 0, 0, 0, -20, -40},
	{-30, 0, 10, 15, 15, 10, 0, -30},
	{-30, 5, 15, 20, 20, 15, 5, -30},
	{-30, 0, 15, 20, 20, 15, 0, -30},
	{-30, 5, 10, 15, 15, 10, 5, -30},
	{-40, -20, 0, 5, 5, 0, -20, -40},
	{-50, -40, -30, -30, -30, -30, -40, -50},
}

var bishopTable = [8][8]int{
	{-20, -10, -10, -10, -10, -10, -10, -20},
	{-10, 0, 0, 0, 0, 0, 0, -10},
	{-10, 0, 10, 10, 10, 10, 0, -10},
	{-10, 5, 5, 10, 10, 5, 5, -10},
	{-10, 0, 10, 10, 10, 10, 0, -10},
	{-10, 10, 10, 10, 10, 10, 10, -10},
	{-10, 5, 0, 0, 0, 0, 5, -10},
	{-20, -10, -10, -10, -10, -10, -10, -20},
}

var rookTable = [8][8]int{
	{0, 0, 0, 0, 0, 0, 0, 0},
	{5, 10, 10, 10, 10, 10, 10, 5},
	{-5, 0, 0, 0, 0, 0, 0, -5},
	{-5, 0, 0, 0, 0, 0, 0, -5},
	{-5, 0, 0, 0, 0, 0, 0, -5},
	{-5, 0, 0, 0, 0, 0, 0, -5},
	{-5, 0, 0, 0, 0, 0, 0, -5},
	{0, 0, 0, 5, 5, 0, 0, 0},
}

var queenTable = [8][8]int{
	{-20, -10, -10, -5, -5, -10, -10, -20},
	{-10, 0, 0, 0, 0, 0, 0, -10},
	{-10, 0, 5, 5, 5, 5, 0, -10},
	{-5, 0, 5, 5, 5, 5, 0, -5},
	{0, 0, 5, 5, 5, 5, 0, -5},
	{-10, 5, 5, 5, 5, 5, 0, -10},
	{-10, 0, 5, 0, 0, 0, 0, -10},
	{-20, -10, -10, -5, -5, -10, -10, -20},
}

var kingMiddleTable = [8][8]int{
	{-30, -40, -40, -50, -50, -40, -40, -30},
	{-30, -40, -40, -50, -50, -40, -40, -30},
	{-30, -40, -40, -50, -50, -40, -40, -30},
	{-30, -40, -40, -50, -50, -40, -40, -30},
	{-20, -30, -30, -40, -40, -30, -30, -20},
	{-10, -20, -20, -20, -20, -20, -20, -10},
	{20, 20, 0, 0, 0, 0, 20, 20},
	{20, 30, 10, 0, 0, 10, 30, 20},
}

var kingEndTable = [8][8]int{
	{-50, -40, -30, -20, -20, -30, -40, -50},
	{-30, -20, -10, 0, 0, -10, -20, -30},
	{-30, -10, 20, 30, 30, 20, -10, -30},
	{-30, -10, 30, 40, 40, 30, -10, -30},
	{-30, -10, 30, 40, 40, 30, -10, -30},
	{-30, -10, 20, 30, 30, 20, -10, -30},
	{-30, -30, 0, 0, 0, 0, -30, -30},
	{-50, -30, -30, -30, -30, -30, -30, -50},
}

// isLavaSquare checks if a given square has an active lava trap.
func isLavaSquare(lavas []contracts.LavaSquare, row, col int) bool {
	for _, lava := range lavas {
		if lava.Row == row && lava.Col == col {
			return true
		}
	}
	return false
}

// inFriendlyFortress checks if a square is inside a fortress zone owned by color.
func inFriendlyFortress(zones []contracts.FortressZone, color string, row, col int) bool {
	for _, z := range zones {
		if z.OwnerColor != color {
			continue
		}
		if row >= z.TopRow && row <= z.TopRow+1 && col >= z.LeftCol && col <= z.LeftCol+1 {
			return true
		}
	}
	return false
}

func Evaluate(board [][]*contracts.Piece, turn string) int {
	return EvaluateWithModifiers(board, turn, nil, nil, nil)
}

// EvaluateWithModifiers extends Evaluate with board-modifier scoring (lava, fortress, bombs).
// Uses NNUE if weights are loaded, otherwise falls back to hand-crafted evaluation.
func EvaluateWithModifiers(board [][]*contracts.Piece, turn string, lavas []contracts.LavaSquare, fortresses []contracts.FortressZone, bombs []contracts.BombPiece) int {
	if defaultNNUE != nil && defaultNNUE.Loaded() {
		nnue := defaultNNUE.Evaluate(board, lavas, fortresses, bombs, nil, nil)
		if turn == "black" {
			nnue = -nnue
		}
		return nnue
	}
	score := 0
	whiteMaterial := 0
	blackMaterial := 0
	whitePieces := 0
	blackPieces := 0

	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			piece := board[r][c]
			if piece == nil {
				continue
			}

			value := pieceValue(piece.Type)
			if piece.FusedWith != "" {
				value = (value + pieceValue(piece.FusedWith)) / 2
			}

			if piece.Color == "white" {
				whiteMaterial += value
				whitePieces++
			} else {
				blackMaterial += value
				blackPieces++
			}
		}
	}

	isEndgame := whiteMaterial+blackMaterial < 2600

	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			piece := board[r][c]
			if piece == nil {
				continue
			}

			value := pieceValue(piece.Type)
			if piece.FusedWith != "" {
				value = (value + pieceValue(piece.FusedWith)) / 2
			}

			posBonus := positionalBonus(piece, r, c, isEndgame)

			mobility := 0

			total := value + posBonus + mobility

			if piece.Color == turn {
				score += total
			} else {
				score -= total
			}
		}
	}

	whiteKing := findKingPos(board, "white")
	blackKing := findKingPos(board, "black")

	if whiteKing != nil {
		whiteKingShield := kingShieldScore(board, whiteKing.Row, whiteKing.Col, "white")
		if isEndgame {
			whiteKingShield = 0
		}
		if turn == "white" {
			score += whiteKingShield
		} else {
			score -= whiteKingShield
		}
	}

	if blackKing != nil {
		blackKingShield := kingShieldScore(board, blackKing.Row, blackKing.Col, "black")
		if isEndgame {
			blackKingShield = 0
		}
		if turn == "white" {
			score -= blackKingShield
		} else {
			score += blackKingShield
		}
	}

	// --- Board-modifier scoring ---

	// Lava squares: penalize own pieces standing on lava, reward enemy pieces on lava.
	for _, lava := range lavas {
		if lava.Row < 0 || lava.Row > 7 || lava.Col < 0 || lava.Col > 7 {
			continue
		}
		piece := board[lava.Row][lava.Col]
		if piece == nil {
			continue
		}
		penalty := pieceValue(piece.Type) / 3
		if piece.Color == turn {
			score -= penalty
		} else {
			score += penalty
		}
	}

	// Fortress zones: bonus for own fortress control.
	for _, z := range fortresses {
		if z.OwnerColor == turn {
			score += 30
		} else {
			score -= 30
		}
	}

	// Bomb pieces: friendly-fire risk vs. enemy-bait upside.
	for _, bomb := range bombs {
		ownBomb := bomb.OwnerColor == turn
		for dr := -1; dr <= 1; dr++ {
			for dc := -1; dc <= 1; dc++ {
				r := bomb.Row + dr
				c := bomb.Col + dc
				if !inBounds(r, c) || (dr == 0 && dc == 0) {
					continue
				}
				p := board[r][c]
				if p == nil || p.Type == "king" {
					continue
				}
				if ownBomb && p.Color == turn {
					score -= pieceValue(p.Type) / 4
				} else if ownBomb && p.Color != turn {
					score += pieceValue(p.Type) / 4
				}
			}
		}
	}

	return score
}

func positionalBonus(piece *contracts.Piece, r, c int, isEndgame bool) int {
	blackRow := 7 - r
	switch piece.Type {
	case "pawn":
		if piece.Color == "white" {
			return pawnTable[r][c]
		}
		return pawnTable[blackRow][c]
	case "knight":
		if piece.Color == "white" {
			return knightTable[r][c]
		}
		return knightTable[blackRow][c]
	case "bishop":
		if piece.Color == "white" {
			return bishopTable[r][c]
		}
		return bishopTable[blackRow][c]
	case "rook":
		if piece.Color == "white" {
			return rookTable[r][c]
		}
		return rookTable[blackRow][c]
	case "queen":
		if piece.Color == "white" {
			return queenTable[r][c]
		}
		return queenTable[blackRow][c]
	case "king":
		if isEndgame {
			if piece.Color == "white" {
				return kingEndTable[r][c]
			}
			return kingEndTable[blackRow][c]
		}
		if piece.Color == "white" {
			return kingMiddleTable[r][c]
		}
		return kingMiddleTable[blackRow][c]
	}
	return 0
}

func kingShieldScore(board [][]*contracts.Piece, kingRow, kingCol int, color string) int {
	score := 0
	dir := 1
	if color == "black" {
		dir = -1
	}
	for dr := 0; dr <= 2; dr++ {
		for dc := -1; dc <= 1; dc++ {
			r := kingRow + dr*dir
			c := kingCol + dc
			if r < 0 || r > 7 || c < 0 || c > 7 {
				continue
			}
			piece := board[r][c]
			if piece != nil && piece.Color == color && piece.Type == "pawn" {
				score += 10
			}
			if piece != nil && piece.Color == color && (piece.Type == "knight" || piece.Type == "bishop") {
				score += 5
			}
		}
	}
	return score
}

func findKingPos(board [][]*contracts.Piece, color string) *contracts.Square {
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

func pieceValue(pieceType string) int {
	if v, ok := pieceValues[pieceType]; ok {
		return v
	}
	return 0
}
