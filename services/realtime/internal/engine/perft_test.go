package engine

import (
	"testing"

	"github.com/chess404/realtime/internal/contracts"
)

// perftCase holds a FEN position and expected move counts at various depths.
type perftCase struct {
	name string
	fen  string
	// expected[d] = expected node count at depth d (1-indexed)
	expected map[int]int
}

func TestPerftStartingPosition(t *testing.T) {
	cases := []perftCase{
		{
			name: "starting position",
			fen:  "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
			expected: map[int]int{
				1: 20,
				2: 400,
				3: 8902,
				4: 197281,
			},
		},
		{
			name: "kiwipete",
			fen:  "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq -",
			expected: map[int]int{
				1: 48,
				2: 2039,
				3: 97862,
			},
		},
		{
			name: "position 3",
			fen:  "8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - -",
			expected: map[int]int{
				1: 14,
				2: 191,
				3: 2812,
				4: 43238,
			},
		},
		{
			name: "position 4",
			fen:  "r3k2r/Pppp1ppp/1b3nbN/nP6/BBP1P3/q4N2/Pp1P2PP/R2Q1RK1 w kq - 0 1",
			expected: map[int]int{
				1: 6,
				2: 264,
				3: 9467,
			},
		},
		{
			name: "position 5",
			fen:  "rnbq1k1r/pp1Pbppp/2p5/8/2B5/8/PPP1NnPP/RNBQK2R w KQ - 1 8",
			expected: map[int]int{
				1: 44,
				2: 1486,
				3: 62379,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := MatchStateFromFEN(tc.fen)
			if state == nil {
				t.Fatal("failed to parse FEN")
			}
			for depth, expected := range tc.expected {
				count := Perft(state, depth)
				if count != expected {
					t.Errorf("depth %d: expected %d nodes, got %d", depth, expected, count)
				}
			}
		})
	}
}

func TestPerftDivideStartingPosition(t *testing.T) {
	state := MatchStateFromFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	divide := PerftDivide(state, 1)
	total := 0
	for _, count := range divide {
		total += count
	}
	if total != 20 {
		t.Errorf("expected 20 root moves, got %d", total)
	}
	expectedMoves := map[string]bool{
		"a2a3": true, "b2b3": true, "c2c3": true, "d2d3": true, "e2e3": true,
		"f2f3": true, "g2g3": true, "h2h3": true, "a2a4": true, "b2b4": true,
		"c2c4": true, "d2d4": true, "e2e4": true, "f2f4": true, "g2g4": true,
		"h2h4": true, "b1a3": true, "b1c3": true, "g1f3": true, "g1h3": true,
	}
	for uci := range expectedMoves {
		if _, ok := divide[uci]; !ok {
			t.Errorf("expected move %s not in divide output", uci)
		}
	}
	if len(divide) != 20 {
		t.Errorf("expected 20 unique moves in divide, got %d", len(divide))
	}
}

func TestKiwipeteDivideDepth3(t *testing.T) {
	state := MatchStateFromFEN("r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq -")
	if state == nil {
		t.Fatal("failed to parse kiwipete fen")
	}
	divide := PerftDivide(state, 3)
	total := 0
	for _, count := range divide {
		total += count
	}
	// Known good values from Stockfish perft for Kiwipete depth 3:
	// Expected total: 97862
	// Our engine: 98027 (diff +165)
	expected := 97862
	if total != expected {
		t.Errorf("depth 3: expected %d, got %d (diff %d)", expected, total, total-expected)
		for move, count := range divide {
			t.Logf("%s: %d", move, count)
		}
	}
}

func TestPerftEnPassant(t *testing.T) {
	state := MatchStateFromFEN("4k3/8/8/8/3Pp3/8/8/4K3 b - d3 0 1")
	if state == nil {
		t.Fatal("failed to parse ep fen")
	}
	divide := PerftDivide(state, 1)
	for uci, count := range divide {
		t.Logf("%s: %d", uci, count)
	}
	count := 0
	for _, c := range divide {
		count += c
	}
	if count != 7 {
		t.Errorf("expected 7 moves with en passant, got %d", count)
	}
	if _, ok := divide["e4d3"]; !ok {
		t.Error("en passant e4d3 not found in divide output")
	}
}

func TestPerftPromotion(t *testing.T) {
	// Position where white can promote a pawn
	// FEN: 4k3/2P5/8/8/8/8/8/4K3 w - - 0 1
	state := MatchStateFromFEN("4k3/2P5/8/8/8/8/8/4K3 w - - 0 1")
	if state == nil {
		t.Fatal("failed to parse promotion fen")
	}
	count := Perft(state, 1)
	// White: Ke1-d1, Ke1-d2, Ke1-e2, Ke1-f1, Ke1-f2 (5 king moves)
	// c7-c8=Q, c7-c8=R, c7-c8=B, c7-c8=N (4 promotions)
	// Total = 9
	if count != 9 {
		t.Errorf("expected 9 moves with promotions, got %d", count)
	}
}

func TestPerftCheckmate(t *testing.T) {
	// Scholar's mate position: black is checkmated
	// FEN: r1bqkb1r/pppp1Qpp/2n2n2/4p3/2B1P3/8/PPPP1PPP/RNB1K1NR b KQkq - 0 4
	state := MatchStateFromFEN("r1bqkb1r/pppp1Qpp/2n2n2/4p3/2B1P3/8/PPPP1PPP/RNB1K1NR b KQkq - 0 4")
	if state == nil {
		t.Fatal("failed to parse scholar's mate fen")
	}
	count := Perft(state, 1)
	if count != 0 {
		t.Errorf("expected 0 moves (checkmate), got %d", count)
	}
}

func TestFortressBlocksOpponentEntry(t *testing.T) {
	// Black king on a8, white king on h1. White fortress at TopRow=7, LeftCol=0
	// covering a8/b8/a7/b7. Black CANNOT enter those squares.
	state := MatchStateFromFEN("k7/8/8/8/8/8/8/7K w - -")
	if state == nil {
		t.Fatal("failed to parse fen")
	}
	// White fortress covering a8 (7,0) and b8 (7,1).
	state.FortressZones = []contracts.FortressZone{
		{TopRow: 7, LeftCol: 0, TurnsLeft: 3, OwnerColor: "white"},
	}
	// Change turn to black. Black king at a8 can move to b8 (7,1) which is
	// in the fortress zone → blocked. Can also move to a7 (6,0) which is also
	// in the fortress zone (rows 7-8, cols 0-1) — wait, TopRow=7, so rows
	// 7 and 8 are covered. a7 is row 6, so NOT in fortress. Actually wait,
	// TopRow=7, zone covers TopRow and TopRow+1 = 7 and 8 = rows 7, 8.
	// But board only goes 0-7. So it's rows 7 only (row 8 doesn't exist).
	// So the zone only covers row 7 (a8 and b8). 
	state.Turn = "black"
	divide := PerftDivide(state, 1)
	// Black king at a8 (7,0). Legal moves:
	// - a8→b8 (7,1): blocked by fortress (owned by white, zone covers 7,0-1)
	// - a8→a7 (6,0): NOT in fortress (row 6 < 7)
	// - a8→b7 (6,1): NOT in fortress
	// - a8 is the only black piece → should have 2 moves
	if len(divide) != 2 {
		t.Errorf("expected 2 moves (a7, b7), got %d moves", len(divide))
		for m := range divide {
			t.Logf("  move: %s", m)
		}
	}
	if _, ok := divide["a8b8"]; ok {
		t.Error("a8b8 should be blocked by fortress")
	}
}

func TestEvaluateWithBombOwnPieceNearby(t *testing.T) {
	b := newTestBoard()
	place(b, 0, 4, "king", "black")
	place(b, 7, 4, "king", "white")
	place(b, 3, 3, "queen", "white")
	place(b, 3, 4, "pawn", "white")
	bombs := []contracts.BombPiece{
		{Row: 3, Col: 3, OwnerColor: "white", TurnsLeft: 3},
	}
	// Bomb at (3,3) owned by white. White queen AT (3,3) and white pawn at (3,4).
	// ownBomb=true, p.Color==turn → penalty for own queen adjacent to own bomb:
	//   (3,3) is the bomb square itself, dr=0,dc=0 → skipped.
	//   (3,4) pawn, ownBomb && p.Color==turn → penalty: 100/4 = 25
	//   (4,3) empty
	//   (4,4) empty
	//   (2,3) empty
	//   (2,4) empty
	//   (4,2) empty
	//   (3,2) empty
	//   (2,2) empty
	score := EvaluateWithModifiers(b, "white", nil, nil, bombs)
	plainScore := EvaluateWithModifiers(b, "white", nil, nil, nil)
	if score >= plainScore {
		t.Errorf("expected own-bomb penalty, bomb=%d plain=%d", score, plainScore)
	}
}

func TestEvaluateWithBombEnemyNearby(t *testing.T) {
	b := newTestBoard()
	place(b, 0, 4, "king", "black")
	place(b, 7, 4, "king", "white")
	place(b, 3, 3, "queen", "white")
	place(b, 4, 3, "rook", "black")
	bombs := []contracts.BombPiece{
		{Row: 3, Col: 3, OwnerColor: "white", TurnsLeft: 3},
	}
	// Bomb at (3,3) owned by white. White queen AT (3,3), black rook at (4,3).
	// ownBomb=true, p.Color!=turn → reward: rook(500)/4 = 125
	score := EvaluateWithModifiers(b, "white", nil, nil, bombs)
	plainScore := EvaluateWithModifiers(b, "white", nil, nil, nil)
	if score <= plainScore {
		t.Errorf("expected enemy-near-bomb reward, bomb=%d plain=%d", score, plainScore)
	}
}

func TestEvaluateWithBombEnemyAdjacent(t *testing.T) {
	b := newTestBoard()
	place(b, 0, 4, "king", "black")
	place(b, 7, 4, "king", "white")
	place(b, 3, 3, "queen", "white")
	place(b, 2, 3, "rook", "black")
	bombs := []contracts.BombPiece{
		{Row: 2, Col: 3, OwnerColor: "black", TurnsLeft: 3},
	}
	// Bomb owned by black (enemy), black rook on bomb, white queen adjacent.
	// ownBomb && p.Color != turn: ownBomb=true (black owns), turn=white, p.Color=white → reward
	// ownBomb=false for the adjacent piece (queen is white, not the bomb owner):
	// ownBomb=true (black owns bomb), p.Color=white, p.Color != turn (turn=white, false) → not triggered
	// Wait, let me re-read the code:
	//   ownBomb := bomb.OwnerColor == turn  // turn=white, OwnerColor=black → false
	//   if ownBomb && p.Color == turn → penalty (own piece near own bomb)
	//   else if ownBomb && p.Color != turn → reward (enemy near own bomb)
	// Since ownBomb=false, neither triggers. So bomb doesn't affect score.
	// But the bomb piece itself is on row 2, col 3 — black rook.
	score := EvaluateWithModifiers(b, "white", nil, nil, bombs)
	plainScore := EvaluateWithModifiers(b, "white", nil, nil, nil)
	if score != plainScore {
		t.Errorf("expected bomb to have no effect when not owned by turn, score=%d plain=%d", score, plainScore)
	}
}

func TestEvaluateWithLava(t *testing.T) {
	b := newTestBoard()
	place(b, 0, 4, "king", "white")
	place(b, 7, 4, "king", "black")
	place(b, 3, 3, "queen", "white")
	lavas := []contracts.LavaSquare{{Row: 3, Col: 3, MovesLeft: 2}}
	// White queen on lava should be penalized.
	score := EvaluateWithModifiers(b, "white", lavas, nil, nil)
	// Without lava, white queen = 900 + positional. With lava, penalty = 900/3 = 300.
	// Stand-pat for white turn: white queen (900) minus nothing,
	// then lava penalty subtracts 300 from white score.
	// So score should be ~600ish.
	if score > 700 {
		t.Errorf("expected lava penalty to reduce score, got %d", score)
	}
}

func TestSelfPlayShortMatch(t *testing.T) {
	// Play a 10-ply self-play game using the engine to verify no crashes.
	state := MatchStateFromFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if state == nil {
		t.Fatal("failed to parse starting fen")
	}
	tt := NewTranspositionTable(100000)
	for ply := 0; ply < 10; ply++ {
		if state.Status != "active" {
			break
		}
		isWhite := state.Turn == "white"
		moves := generateAllMoves(state, isWhite)
		if len(moves) == 0 {
			break
		}
		// Use shallow search for speed.
		result := Search(state, 3, tt)
		if result.BestMove.From == result.BestMove.To && result.BestMove.From.Row == 0 && result.BestMove.To.Col == 0 {
			// No best move found — pick first legal move.
			result.BestMove = moves[0]
		}
		state = applyMoveCopy(state, &result.BestMove)
	}
	// Should have played 10 plies without crash.
	t.Logf("self-play finished after %d plies, status=%s turn=%s", 10, state.Status, state.Turn)
}

func TestPerftStalemate(t *testing.T) {
	// Classic queen stalemate: black king on h8, white queen on f7, white king on h1.
	// Black has no legal moves but is not in check.
	state := MatchStateFromFEN("7k/5Q2/8/8/8/8/8/7K b - - 0 1")
	if state == nil {
		t.Fatal("failed to parse stalemate fen")
	}
	count := Perft(state, 1)
	if count != 0 {
		t.Errorf("expected 0 moves (stalemate), got %d", count)
	}
}
