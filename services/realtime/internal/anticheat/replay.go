package anticheat

import (
	"github.com/chess404/realtime/internal/contracts"
)

// EmptyBoard returns a fresh standard-chess starting position as the
// in-house engine's MatchState.Board. Used as the seed for replay
// when the match-service doesn't expose an initial FEN.
func EmptyBoard() [][]*contracts.Piece {
	board := make([][]*contracts.Piece, 8)
	place := func(row int, side string, types []string) {
		board[row] = make([]*contracts.Piece, 8)
		for c, t := range types {
			board[row][c] = &contracts.Piece{Type: t, Color: side}
		}
	}
	place(0, "white", []string{"rook", "knight", "bishop", "queen", "king", "bishop", "knight", "rook"})
	place(1, "white", []string{"pawn", "pawn", "pawn", "pawn", "pawn", "pawn", "pawn", "pawn"})
	for r := 2; r < 6; r++ {
		board[r] = make([]*contracts.Piece, 8)
	}
	place(6, "black", []string{"pawn", "pawn", "pawn", "pawn", "pawn", "pawn", "pawn", "pawn"})
	place(7, "black", []string{"rook", "knight", "bishop", "queen", "king", "bishop", "knight", "rook"})
	return board
}

// ApplyMoveUCIToBoard applies a UCI move (e.g., "e2e4") to the in-house
// engine's board representation. Pawn promotions default to queen
// (the analysis pipeline only cares about top-N correlation, not
// exact underpromotion analysis). Castling is not specially handled
// here: castling is just a king move from e1 to g1 (or c1 to f1 for
// queenside), and we don't track rook participation. This is a
// simplification acceptable for cheater detection.
func ApplyMoveUCIToBoard(board [][]*contracts.Piece, uciMove, color string) {
	if len(uciMove) < 4 {
		return
	}
	fromFile, fromRank := uciMove[0], uciMove[1]
	toFile, toRank := uciMove[2], uciMove[3]
	fromCol := int(fromFile - 'a')
	fromRow := int(fromRank - '1')
	toCol := int(toFile - 'a')
	toRow := int(toRank - '1')
	if fromCol < 0 || fromCol > 7 || fromRow < 0 || fromRow > 7 ||
		toCol < 0 || toCol > 7 || toRow < 0 || toRow > 7 {
		return
	}
	if fromRow >= len(board) || fromCol >= len(board[fromRow]) {
		return
	}
	piece := board[fromRow][fromCol]
	if piece == nil {
		return
	}
	// Standard move: pick up from source, drop on destination. If
	// there's a piece at the destination, this is a capture.
	if toRow < len(board) {
		if toCol < len(board[toRow]) {
			board[toRow][toCol] = piece
		}
	}
	board[fromRow][fromCol] = nil

	// Pawn promotion: if a pawn reaches the last rank, replace with
	// a queen. (We don't track underpromotion in this simplified
	// model.)
	if piece.Type == "pawn" {
		if (color == "white" && toRow == 7) || (color == "black" && toRow == 0) {
			board[toRow][toCol] = &contracts.Piece{Type: "queen", Color: color}
		}
	}
}

// ReplayGame walks the move history of a game, starting from the
// initial position, and returns one PositionSample per move. The FEN
// in each sample is the position BEFORE the move; the played move is
// the move in UCI notation. Card moves are not currently flagged
// (IsCardMove is always false) — this is a limitation of the move
// history format the match-service exposes; a future Phase 2 will
// add per-move card metadata so we can exclude card moves from
// engine comparison.
func ReplayGame(moveHistory []string, parseMove func(string) (string, bool)) []PositionSample {
	board := EmptyBoard()
	color := "white"
	samples := make([]PositionSample, 0, len(moveHistory))
	for _, notation := range moveHistory {
		uci, ok := parseMove(notation)
		if !ok {
			// Skip unparseable moves; the next FEN won't reflect
			// them, but downstream analysis just records a
			// TopNMoves error and continues.
			continue
		}
		fen := boardToFENFromSlices(board, color)
		samples = append(samples, PositionSample{
			FEN:        fen,
			PlayedMove: uci,
			IsCardMove:  false,
		})
		ApplyMoveUCIToBoard(board, uci, color)
		if color == "white" {
			color = "black"
		} else {
			color = "white"
		}
	}
	return samples
}

// boardToFENFromSlices is the loop-optimized version of BoardToFEN
// that takes a raw [][]*Piece rather than a MatchState. This avoids
// the MatchState struct construction overhead in the replay loop.
//
// As with BoardToFEN, we flip the row order: in-house row 0 is white's
// back rank, FEN row 0 is black's back rank.
func boardToFENFromSlices(board [][]*contracts.Piece, turn string) string {
	rows := make([]string, 8)
	for fenRow := 0; fenRow < 8; fenRow++ {
		boardRow := 7 - fenRow
		var row []byte
		empty := 0
		for c := 0; c < 8; c++ {
			var p *contracts.Piece
			if boardRow < len(board) && c < len(board[boardRow]) {
				p = board[boardRow][c]
			}
			if p == nil {
				empty++
				continue
			}
			if empty > 0 {
				row = append(row, byte('0'+empty))
				empty = 0
			}
			ch := pieceTypeToFENChar(p.Type)
			if p.Color == "white" && ch[0] >= 'a' && ch[0] <= 'z' {
				ch = string(ch[0] - 32)
			}
			row = append(row, ch[0])
		}
		if empty > 0 {
			row = append(row, byte('0'+empty))
		}
		rows[fenRow] = string(row)
	}
	t := "w"
	if turn == "black" {
		t = "b"
	}
	// join with '/'
	out := ""
	for i, r := range rows {
		if i > 0 {
			out += "/"
		}
		out += r
	}
	return out + " " + t + " - - 0 1"
}
