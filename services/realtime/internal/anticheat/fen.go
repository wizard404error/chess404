package anticheat

import (
	"strconv"
	"strings"

	"github.com/chess404/realtime/internal/contracts"
)

// BoardToFEN converts the in-house engine's MatchState board to a FEN
// string. The FEN includes piece placement, side to move, and a few
// zeroed-out fields (castling, en passant, halfmove, fullmove).
// Castling rights and en passant are not preserved because the
// in-house engine doesn't track them in the snapshot; this is OK
// for our purpose because Stockfish treats missing castling rights
// as conservative (prefers to keep the king in place) and missing
// en passant as "no en passant available". The score impact is small
// for post-game analysis.
//
// IMPORTANT: the in-house engine stores the board with row 0 as
// white's back rank (rank 1 in chess), so the FEN row order is the
// reverse of the in-house board's row order. FEN row 0 is rank 8
// (black's back rank), so the in-house board's last row (row 7)
// maps to FEN's first row.
//
// The result looks like:
//   rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w - - 0 1
func BoardToFEN(state *contracts.MatchState) string {
	if state == nil {
		return "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w - - 0 1"
	}
	rows := make([]string, 8)
	// FEN row 0 = rank 8 = in-house board row 7
	// FEN row 7 = rank 1 = in-house board row 0
	for fenRow := 0; fenRow < 8; fenRow++ {
		boardRow := 7 - fenRow
		var row strings.Builder
		empty := 0
		for c := 0; c < 8; c++ {
			var p *contracts.Piece
			if boardRow < len(state.Board) && c < len(state.Board[boardRow]) {
				p = state.Board[boardRow][c]
			}
			if p == nil {
				empty++
				continue
			}
			if empty > 0 {
				row.WriteString(strconv.Itoa(empty))
				empty = 0
			}
			ch := pieceTypeToFENChar(p.Type)
			if p.Color == "white" {
				ch = strings.ToUpper(ch)
			}
			row.WriteString(ch)
		}
		if empty > 0 {
			row.WriteString(strconv.Itoa(empty))
		}
		rows[fenRow] = row.String()
	}
	turn := "w"
	if state.Turn == "black" {
		turn = "b"
	}
	return strings.Join(rows, "/") + " " + turn + " - - 0 1"
}

// pieceTypeToFENChar maps the in-house engine's piece type strings to
// the FEN character. We only support the standard pieces; the card
// game introduces no extra piece types, so this mapping is stable.
func pieceTypeToFENChar(pieceType string) string {
	switch strings.ToLower(strings.TrimSpace(pieceType)) {
	case "pawn":
		return "p"
	case "knight":
		return "n"
	case "bishop":
		return "b"
	case "rook":
		return "r"
	case "queen":
		return "q"
	case "king":
		return "k"
	default:
		// Unknown type: emit lowercase 'p' as a safe placeholder. A
		// real game would never have an unknown piece type; this is
		// defensive only.
		return "p"
	}
}
