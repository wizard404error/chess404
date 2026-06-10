package engine

import (
	"math/rand"
	"testing"

	"github.com/chess404/realtime/internal/contracts"
)

func newTestBoard() [][]*contracts.Piece {
	b := make([][]*contracts.Piece, 8)
	for r := 0; r < 8; r++ {
		b[r] = make([]*contracts.Piece, 8)
	}
	return b
}

func place(b [][]*contracts.Piece, r, c int, pType, color string) {
	b[r][c] = &contracts.Piece{Type: pType, Color: color}
}

func placeFused(b [][]*contracts.Piece, r, c int, pType, color, fusedWith string) {
	b[r][c] = &contracts.Piece{Type: pType, Color: color, FusedWith: fusedWith}
}

func newMatchState(board [][]*contracts.Piece, turn string) *contracts.MatchState {
	return &contracts.MatchState{
		Status: "active",
		Board:  board,
		Turn:   turn,
		Moved:  []string{},
	}
}

func TestPieceValueStandard(t *testing.T) {
	if pieceValue("pawn") != 100 {
		t.Errorf("pawn=100, got %d", pieceValue("pawn"))
	}
	if pieceValue("knight") != 320 {
		t.Errorf("knight=320, got %d", pieceValue("knight"))
	}
	if pieceValue("bishop") != 330 {
		t.Errorf("bishop=330, got %d", pieceValue("bishop"))
	}
	if pieceValue("rook") != 500 {
		t.Errorf("rook=500, got %d", pieceValue("rook"))
	}
	if pieceValue("queen") != 900 {
		t.Errorf("queen=900, got %d", pieceValue("queen"))
	}
	if pieceValue("king") != 20000 {
		t.Errorf("king=20000, got %d", pieceValue("king"))
	}
}

func TestPieceValueFused(t *testing.T) {
	v := pieceValue("pawn+bishop")
	// Fused pieces not in map, value is 0 — that's expected behavior
	if v != 0 {
		t.Errorf("fused pieces not in standard map, expected 0, got %d", v)
	}
}

func TestIsAttackedSimple(t *testing.T) {
	b := newTestBoard()
	place(b, 4, 4, "king", "white")
	place(b, 2, 3, "queen", "black")
	if isAttacked(b, contracts.Square{Row: 4, Col: 4}, "black") {
		t.Error("king should not be attacked by distant queen")
	}
	place(b, 3, 4, "rook", "black")
	if !isAttacked(b, contracts.Square{Row: 4, Col: 4}, "black") {
		t.Error("king should be attacked by adjacent rook")
	}
}

func TestIsAttackedWithFusion(t *testing.T) {
	b := newTestBoard()
	place(b, 4, 4, "king", "white")
	placeFused(b, 2, 2, "pawn", "black", "bishop")
	if !isAttackedWithFusion(b, contracts.Square{Row: 4, Col: 4}, "black") {
		t.Error("fused pawn+bishop should attack diagonally")
	}
}

func TestCloneBoard(t *testing.T) {
	b := newTestBoard()
	place(b, 0, 0, "rook", "white")
	place(b, 7, 7, "king", "black")
	clone := cloneBoard(b)
	clone[0][0] = nil
	if b[0][0] == nil {
		t.Error("cloneBoard mutated original")
	}
}

func TestClonePieceAsType(t *testing.T) {
	p := &contracts.Piece{Type: "pawn", Color: "white", Shielded: true}
	fused := clonePieceAsType(p, "bishop")
	if fused.Type != "bishop" {
		t.Errorf("expected bishop, got %s", fused.Type)
	}
	if !fused.Shielded {
		t.Error("shielded not preserved")
	}
}

func TestEvalWhiteAdvantage(t *testing.T) {
	b := newTestBoard()
	place(b, 3, 3, "queen", "white")
	place(b, 4, 4, "pawn", "black")
	place(b, 7, 4, "king", "white")
	place(b, 0, 4, "king", "black")
	score := Evaluate(b, "white")
	if score < 100 {
		t.Errorf("expected white advantage > 100, got %d", score)
	}
}

func TestEvalBlackAdvantage(t *testing.T) {
	b := newTestBoard()
	place(b, 3, 3, "queen", "black")
	place(b, 4, 4, "pawn", "white")
	place(b, 7, 4, "king", "white")
	place(b, 0, 4, "king", "black")
	score := Evaluate(b, "white")
	if score > -100 {
		t.Errorf("expected black advantage < -100, got %d", score)
	}
}

func TestSearchFindsAttack(t *testing.T) {
	b := newTestBoard()
	place(b, 0, 4, "king", "black")
	place(b, 7, 4, "king", "white")
	place(b, 3, 5, "queen", "white")
	place(b, 4, 2, "bishop", "white")
	place(b, 1, 5, "pawn", "black")

	state := newMatchState(b, "white")
	result := Search(state, 3, nil)
	if result.BestMove.From == (contracts.Square{}) && result.BestMove.To == (contracts.Square{}) {
		t.Fatal("search returned no move")
	}
	if result.Score < 0 {
		t.Errorf("expected positive score for white advantage, got %d", result.Score)
	}
}

func TestComputerOpponentBeginner(t *testing.T) {
	opp := NewComputerOpponent(DifficultyBeginner, "black")
	b := newTestBoard()
	place(b, 0, 4, "king", "black")
	place(b, 7, 4, "king", "white")
	place(b, 1, 0, "pawn", "black")
	place(b, 1, 1, "pawn", "black")
	place(b, 6, 0, "pawn", "white")
	place(b, 6, 1, "pawn", "white")

	state := newMatchState(b, "black")
	intent := opp.MakeMove(state)
	if intent == nil {
		t.Fatal("computer returned nil intent")
	}
}

func TestShouldPlayCard(t *testing.T) {
	rng := newTestRng()
	ce := NewCardEvaluator(rng)
	b := newTestBoard()
	place(b, 0, 4, "king", "black")
	place(b, 7, 4, "king", "white")
	state := newMatchState(b, "black")
	played := ce.ShouldPlayCard(state, false)
	_ = played
}

func TestBestCardToPlay(t *testing.T) {
	rng := newTestRng()
	ce := NewCardEvaluator(rng)
	b := newTestBoard()
	place(b, 0, 4, "king", "black")
	place(b, 7, 4, "king", "white")
	place(b, 3, 3, "queen", "black")
	place(b, 4, 4, "pawn", "white")
	state := newMatchState(b, "black")
	_ = ce.BestCardToPlay(state, false)
}

func TestHandEvaluation(t *testing.T) {
	rng := newTestRng()
	ce := NewCardEvaluator(rng)
	cards := []contracts.GameCard{
		{ID: "freeze", Mechanic: "freeze"},
		{ID: "heal", Mechanic: "heal"},
	}
	b := newTestBoard()
	place(b, 0, 4, "king", "black")
	place(b, 7, 4, "king", "white")
	state := newMatchState(b, "black")
	plays := ce.EvaluateHand(cards, state, false)
	if len(plays) != 2 {
		t.Errorf("expected 2 plays, got %d", len(plays))
	}
	for _, play := range plays {
		if play.Card.ID == "" {
			t.Error("card ID should not be empty")
		}
	}
}

func newTestRng() *rand.Rand {
	return rand.New(rand.NewSource(42))
}
