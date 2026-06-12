package anticheat

import (
	"testing"

	"github.com/chess404/realtime/internal/contracts"
)

func TestBoardToFEN_StartingPosition(t *testing.T) {
	state := &contracts.MatchState{
		Board: EmptyBoard(),
		Turn:  "white",
	}
	fen := BoardToFEN(state)
	want := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w - - 0 1"
	if fen != want {
		t.Fatalf("starting position FEN: want %q got %q", want, fen)
	}
}

func TestApplyMoveUCIToBoard_E4(t *testing.T) {
	board := EmptyBoard()
	ApplyMoveUCIToBoard(board, "e2e4", "white")
	// After e2e4: e4 has a white pawn, e2 is empty
	if board[1][4] != nil {
		t.Fatalf("e2 should be empty, got %+v", board[1][4])
	}
	if board[3][4] == nil || board[3][4].Type != "pawn" || board[3][4].Color != "white" {
		t.Fatalf("e4 should have white pawn, got %+v", board[3][4])
	}
}

func TestApplyMoveUCIToBoard_Promotion(t *testing.T) {
	board := EmptyBoard()
	// Remove black pieces from row 7 to give the white pawn a path
	// to e8 (we don't need to worry about check legality here).
	board[7] = make([]*contracts.Piece, 8)
	// Put a white pawn on e7
	board[6][4] = &contracts.Piece{Type: "pawn", Color: "white"}
	ApplyMoveUCIToBoard(board, "e7e8", "white")
	if board[7][4] == nil || board[7][4].Type != "queen" || board[7][4].Color != "white" {
		t.Fatalf("e8 should have white queen after promotion, got %+v", board[7][4])
	}
}

func TestReplayGame_E4E5Nf3(t *testing.T) {
	moves := []string{"e4", "e5", "Nf3"}
	// Simple parser that converts the most common moves.
	parseMove := func(n string) (string, bool) {
		lookup := map[string]string{
			"e4":  "e2e4",
			"e5":  "e7e5",
			"Nf3": "g1f3",
		}
		uci, ok := lookup[n]
		return uci, ok
	}
	samples := ReplayGame(moves, parseMove)
	if len(samples) != 3 {
		t.Fatalf("expected 3 samples, got %d", len(samples))
	}
	// Sample 0: FEN before e2e4, played e2e4
	if samples[0].PlayedMove != "e2e4" {
		t.Fatalf("sample 0 played: want e2e4, got %s", samples[0].PlayedMove)
	}
	if samples[0].FEN != "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w - - 0 1" {
		t.Fatalf("sample 0 FEN: %s", samples[0].FEN)
	}
	// Sample 1: FEN after e2e4, before e7e5
	if samples[1].PlayedMove != "e7e5" {
		t.Fatalf("sample 1 played: want e7e5, got %s", samples[1].PlayedMove)
	}
	// Should have a white pawn on e4 (file 4, rank 4)
	if !containsPiece(samples[1].FEN, 'P', 4, 3) {
		t.Fatalf("sample 1 FEN should have white pawn on e4: %s", samples[1].FEN)
	}
}

func TestContainsPiece_RankIndexing(t *testing.T) {
	fen := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w - - 0 1"
	// White pawn on a2: file 0, rank 2
	if !containsPiece(fen, 'P', 0, 1) {
		t.Fatalf("expected white pawn on a2")
	}
	// Black pawn on a7: file 0, rank 7
	if !containsPiece(fen, 'p', 0, 6) {
		t.Fatalf("expected black pawn on a7")
	}
	// a1 should have white rook
	if !containsPiece(fen, 'R', 0, 0) {
		t.Fatalf("expected white rook on a1")
	}
}

func containsPiece(fen string, piece byte, file, rank int) bool {
	// Parse the placement portion of the FEN and check the
	// (file, rank) square for the given piece. file=0..7 is
	// a..h; rank=0..7 is 1..8. The FEN lists ranks from 8
	// down to 1, so we flip the rank to get the FEN row.
	parts := splitFEN(fen)
	if len(parts) < 1 {
		return false
	}
	rows := splitRanks(parts[0])
	if rank < 0 || rank >= len(rows) {
		return false
	}
	fenRow := len(rows) - 1 - rank // flip: rank 1 (rank=0) -> fenRow=7
	row := rows[fenRow]
	col := 0
	for _, ch := range row {
		if col == file {
			return byte(ch) == piece
		}
		if ch >= '1' && ch <= '8' {
			col += int(ch - '0')
		} else {
			col++
		}
		if col > file {
			return false
		}
	}
	return false
}

func splitFEN(fen string) []string {
	out := []string{}
	start := 0
	for i := 0; i < len(fen); i++ {
		if fen[i] == ' ' {
			out = append(out, fen[start:i])
			start = i + 1
		}
	}
	out = append(out, fen[start:])
	return out
}

func splitRanks(placement string) []string {
	out := []string{}
	start := 0
	for i := 0; i < len(placement); i++ {
		if placement[i] == '/' {
			out = append(out, placement[start:i])
			start = i + 1
		}
	}
	out = append(out, placement[start:])
	return out
}
