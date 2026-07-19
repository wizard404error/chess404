package match

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chess404/realtime/internal/contracts"
)

func TestCardFreeze(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_freeze"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "freeze")

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_freeze", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("play_card freeze: %v", err)
	}

	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_freeze", PlayerID: "white_player",
		Target: &contracts.Square{Row: 6, Col: 0},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("select_target freeze: %v", err)
	}

	if frozen := service.getMatchContainer("test_freeze").state.Board[6][0]; frozen == nil || !frozen.Frozen {
		t.Fatal("expected frozen piece")
	}
	if result.Match.PendingCard != nil {
		t.Fatal("expected pending card cleared")
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}

func TestCardBadsniper(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_badsniper"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "badsniper")

	state := service.getMatchContainer("test_badsniper").state
	state.Board = emptyBoard()
	state.Board[0][0] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[2][2] = &contracts.Piece{Type: "knight", Color: "white"}

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_badsniper", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("play_card: %v", err)
	}

	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_badsniper", PlayerID: "white_player",
		Target: &contracts.Square{Row: 2, Col: 2},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("select_target: %v", err)
	}

	if removed := service.getMatchContainer("test_badsniper").state.Board[2][2]; removed != nil {
		t.Fatal("expected own piece removed")
	}
	if result.Match.PendingCard != nil {
		t.Fatal("expected pending cleared")
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}

func TestCardDemote(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_demote"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "demote")

	state := service.getMatchContainer("test_demote").state
	state.Board = emptyBoard()
	state.Board[0][0] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[3][3] = &contracts.Piece{Type: "queen", Color: "white"}

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_demote", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("play_card: %v", err)
	}
	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_demote", PlayerID: "white_player",
		Target: &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("target step: %v", err)
	}
	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_demote", PlayerID: "white_player",
		SelectionID: "rook",
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("selection step: %v", err)
	}

	if piece := service.getMatchContainer("test_demote").state.Board[3][3]; piece == nil || piece.Type != "rook" {
		t.Fatalf("expected rook got %#v", piece)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}

func TestCardDemotehim(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_demotehim"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "demotehim")

	state := service.getMatchContainer("test_demotehim").state
	state.Board = emptyBoard()
	state.Board[0][0] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[3][3] = &contracts.Piece{Type: "queen", Color: "black"}

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_demotehim", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("play_card: %v", err)
	}
	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_demotehim", PlayerID: "white_player",
		Target: &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("target: %v", err)
	}
	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_demotehim", PlayerID: "white_player",
		SelectionID: "pawn",
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("selection: %v", err)
	}

	if piece := service.getMatchContainer("test_demotehim").state.Board[3][3]; piece == nil || piece.Type != "pawn" {
		t.Fatalf("expected pawn got %#v", piece)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}

func TestCardPromote(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_promote"}, now)
	// Advance the white pawn from (1,1) to (6,0) — row 6 is the promotion rank for white.
	// The final step captures the black pawn at (6,0) diagonally.
	moves := []contracts.PlayerIntent{
		{Type: "make_move", MatchID: "test_promote", PlayerID: "white_player",
			From: &contracts.Square{Row: 1, Col: 1}, To: &contracts.Square{Row: 3, Col: 1}},
		{Type: "make_move", MatchID: "test_promote", PlayerID: "black_player",
			From: &contracts.Square{Row: 6, Col: 2}, To: &contracts.Square{Row: 4, Col: 2}},
		{Type: "make_move", MatchID: "test_promote", PlayerID: "white_player",
			From: &contracts.Square{Row: 3, Col: 1}, To: &contracts.Square{Row: 4, Col: 1}},
		{Type: "make_move", MatchID: "test_promote", PlayerID: "black_player",
			From: &contracts.Square{Row: 6, Col: 3}, To: &contracts.Square{Row: 4, Col: 3}},
		{Type: "make_move", MatchID: "test_promote", PlayerID: "white_player",
			From: &contracts.Square{Row: 4, Col: 1}, To: &contracts.Square{Row: 5, Col: 1}},
		{Type: "make_move", MatchID: "test_promote", PlayerID: "black_player",
			From: &contracts.Square{Row: 6, Col: 4}, To: &contracts.Square{Row: 4, Col: 4}},
		{Type: "make_move", MatchID: "test_promote", PlayerID: "white_player",
			From: &contracts.Square{Row: 5, Col: 1}, To: &contracts.Square{Row: 6, Col: 0}},
		{Type: "make_move", MatchID: "test_promote", PlayerID: "black_player",
			From: &contracts.Square{Row: 6, Col: 5}, To: &contracts.Square{Row: 4, Col: 5}},
	}
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "promote")

	for idx, m := range moves {
		if _, err := applyTestIntent(service, m, now.Add(time.Duration(idx+1)*time.Second)); err != nil {
			t.Fatalf("setup move %d: %v", idx, err)
		}
	}

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_promote", PlayerID: "white_player", CardID: cardID,
	}, now.Add(9*time.Second)); err != nil {
		t.Fatalf("play_card: %v", err)
	}
	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_promote", PlayerID: "white_player",
		Target: &contracts.Square{Row: 6, Col: 0},
	}, now.Add(10*time.Second)); err != nil {
		t.Fatalf("target: %v", err)
	}
	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_promote", PlayerID: "white_player",
		SelectionID: "queen",
	}, now.Add(11*time.Second))
	if err != nil {
		t.Fatalf("selection: %v", err)
	}

	if piece := service.getMatchContainer("test_promote").state.Board[6][0]; piece == nil || piece.Type != "queen" {
		t.Fatalf("expected queen got %#v", piece)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}

func TestCardPromotehim(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_promotehim"}, now)
	// Advance black's pawn from (6,0) to (1,1) — row 1 is the promotion rank for black.
	// Black captures white's b-pawn at (3,1) to switch files and bypass the white a-pawn.
	moves := []contracts.PlayerIntent{
		{Type: "make_move", MatchID: "test_promotehim", PlayerID: "white_player",
			From: &contracts.Square{Row: 1, Col: 1}, To: &contracts.Square{Row: 3, Col: 1}},
		{Type: "make_move", MatchID: "test_promotehim", PlayerID: "black_player",
			From: &contracts.Square{Row: 6, Col: 0}, To: &contracts.Square{Row: 4, Col: 0}},
		{Type: "make_move", MatchID: "test_promotehim", PlayerID: "white_player",
			From: &contracts.Square{Row: 0, Col: 1}, To: &contracts.Square{Row: 2, Col: 2}},
		{Type: "make_move", MatchID: "test_promotehim", PlayerID: "black_player",
			From: &contracts.Square{Row: 4, Col: 0}, To: &contracts.Square{Row: 3, Col: 1}},
		{Type: "make_move", MatchID: "test_promotehim", PlayerID: "white_player",
			From: &contracts.Square{Row: 2, Col: 2}, To: &contracts.Square{Row: 0, Col: 1}},
		{Type: "make_move", MatchID: "test_promotehim", PlayerID: "black_player",
			From: &contracts.Square{Row: 3, Col: 1}, To: &contracts.Square{Row: 2, Col: 1}},
		{Type: "make_move", MatchID: "test_promotehim", PlayerID: "white_player",
			From: &contracts.Square{Row: 0, Col: 1}, To: &contracts.Square{Row: 2, Col: 2}},
		{Type: "make_move", MatchID: "test_promotehim", PlayerID: "black_player",
			From: &contracts.Square{Row: 2, Col: 1}, To: &contracts.Square{Row: 1, Col: 1}},
	}
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "promotehim")

	for idx, m := range moves {
		if _, err := applyTestIntent(service, m, now.Add(time.Duration(idx+1)*time.Second)); err != nil {
			t.Fatalf("setup move %d: %v", idx, err)
		}
	}

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_promotehim", PlayerID: "white_player", CardID: cardID,
	}, now.Add(9*time.Second)); err != nil {
		t.Fatalf("play_card: %v", err)
	}
	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_promotehim", PlayerID: "white_player",
		Target: &contracts.Square{Row: 1, Col: 1},
	}, now.Add(10*time.Second)); err != nil {
		t.Fatalf("target: %v", err)
	}
	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_promotehim", PlayerID: "white_player",
		SelectionID: "queen",
	}, now.Add(11*time.Second))
	if err != nil {
		t.Fatalf("selection: %v", err)
	}

	if piece := service.getMatchContainer("test_promotehim").state.Board[1][1]; piece == nil || piece.Type != "queen" || piece.Color != "black" {
		t.Fatalf("expected black queen at (1,1) got %#v", piece)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}

func TestCardShield(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_shield"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "shield")

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_shield", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("play_card: %v", err)
	}
	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_shield", PlayerID: "white_player",
		Target: &contracts.Square{Row: 1, Col: 0},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("select_target: %v", err)
	}

	shielded := service.getMatchContainer("test_shield").state.Board[1][0]
	if shielded == nil || !shielded.Shielded || shielded.ShieldTurn == nil {
		t.Fatal("expected shielded with expiry")
	}
	if result.Match.PendingCard != nil {
		t.Fatal("expected pending cleared")
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}

func TestCardSniper(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_sniper"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "sniper")

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_sniper", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("play_card: %v", err)
	}
	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_sniper", PlayerID: "white_player",
		Target: &contracts.Square{Row: 6, Col: 0},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("select_target: %v", err)
	}

	if removed := service.getMatchContainer("test_sniper").state.Board[6][0]; removed != nil {
		t.Fatal("expected sniper removed piece")
	}
	if result.Match.PendingCard != nil {
		t.Fatal("expected pending cleared")
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}

func TestCardSwapme(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_swapme"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "swapme")

	state := service.getMatchContainer("test_swapme").state
	state.Board = emptyBoard()
	state.Board[0][0] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[2][2] = &contracts.Piece{Type: "knight", Color: "white"}
	state.Board[4][4] = &contracts.Piece{Type: "rook", Color: "white"}

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_swapme", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("play_card: %v", err)
	}
	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_swapme", PlayerID: "white_player",
		Target: &contracts.Square{Row: 2, Col: 2},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("first target: %v", err)
	}
	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_swapme", PlayerID: "white_player",
		Target: &contracts.Square{Row: 4, Col: 4},
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("second target: %v", err)
	}

	if p1 := service.getMatchContainer("test_swapme").state.Board[2][2]; p1 == nil || p1.Type != "rook" {
		t.Fatalf("expected rook at (2,2) got %#v", p1)
	}
	if p2 := service.getMatchContainer("test_swapme").state.Board[4][4]; p2 == nil || p2.Type != "knight" {
		t.Fatalf("expected knight at (4,4) got %#v", p2)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}

func TestCardSwapus(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_swapus"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "swapus")

	state := service.getMatchContainer("test_swapus").state
	state.Board = emptyBoard()
	state.Board[0][0] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[2][2] = &contracts.Piece{Type: "rook", Color: "white"}
	state.Board[5][5] = &contracts.Piece{Type: "knight", Color: "black"}

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_swapus", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("play_card: %v", err)
	}
	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_swapus", PlayerID: "white_player",
		Target: &contracts.Square{Row: 2, Col: 2},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("first: %v", err)
	}
	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_swapus", PlayerID: "white_player",
		Target: &contracts.Square{Row: 5, Col: 5},
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("second: %v", err)
	}

	if p1 := service.getMatchContainer("test_swapus").state.Board[2][2]; p1 == nil || p1.Color != "black" {
		t.Fatalf("expected black piece at (2,2) got %#v", p1)
	}
	if p2 := service.getMatchContainer("test_swapus").state.Board[5][5]; p2 == nil || p2.Color != "white" {
		t.Fatalf("expected white piece at (5,5) got %#v", p2)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}

func TestCardSwaphim(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_swaphim"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "swaphim")

	state := service.getMatchContainer("test_swaphim").state
	state.Board = emptyBoard()
	state.Board[0][0] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[5][5] = &contracts.Piece{Type: "knight", Color: "black"}
	state.Board[6][4] = &contracts.Piece{Type: "rook", Color: "black"}

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_swaphim", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("play_card: %v", err)
	}
	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_swaphim", PlayerID: "white_player",
		Target: &contracts.Square{Row: 5, Col: 5},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("first: %v", err)
	}
	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_swaphim", PlayerID: "white_player",
		Target: &contracts.Square{Row: 6, Col: 4},
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("second: %v", err)
	}

	if p1 := service.getMatchContainer("test_swaphim").state.Board[5][5]; p1 == nil || p1.Type != "rook" {
		t.Fatalf("expected rook at (5,5) got %#v", p1)
	}
	if p2 := service.getMatchContainer("test_swaphim").state.Board[6][4]; p2 == nil || p2.Type != "knight" {
		t.Fatalf("expected knight at (6,4) got %#v", p2)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}

func TestCardJump(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_jump"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "jump")

	state := service.getMatchContainer("test_jump").state
	state.Board = emptyBoard()
	state.Board[0][0] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[3][3] = &contracts.Piece{Type: "rook", Color: "white"}
	state.Board[3][4] = &contracts.Piece{Type: "pawn", Color: "black"}

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_jump", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("play_card: %v", err)
	}
	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_jump", PlayerID: "white_player",
		Target: &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("source: %v", err)
	}
	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_jump", PlayerID: "white_player",
		Target: &contracts.Square{Row: 3, Col: 5},
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("destination: %v", err)
	}

	if p := service.getMatchContainer("test_jump").state.Board[3][5]; p == nil || p.Type != "rook" {
		t.Fatalf("expected rook at (3,5) got %#v", p)
	}
	if src := service.getMatchContainer("test_jump").state.Board[3][3]; src != nil {
		t.Fatal("expected source empty")
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}

func TestCardTeleport(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_teleport"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "teleport")

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_teleport", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("play_card: %v", err)
	}
	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_teleport", PlayerID: "white_player",
		Target: &contracts.Square{Row: 1, Col: 0},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("source: %v", err)
	}
	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_teleport", PlayerID: "white_player",
		Target: &contracts.Square{Row: 4, Col: 4},
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("dest: %v", err)
	}

	if p := service.getMatchContainer("test_teleport").state.Board[4][4]; p == nil || p.Type != "pawn" {
		t.Fatalf("expected pawn at (4,4) got %#v", p)
	}
	if src := service.getMatchContainer("test_teleport").state.Board[1][0]; src != nil {
		t.Fatal("expected source empty")
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}

func TestCardSmallsacrifice(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_smallsac"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "smallsacrifice")

	state := service.getMatchContainer("test_smallsac").state
	state.Board = emptyBoard()
	state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][4] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[3][3] = &contracts.Piece{Type: "rook", Color: "white"}
	state.Board[3][4] = &contracts.Piece{Type: "bishop", Color: "white"}

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_smallsac", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("play_card: %v", err)
	}
	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_smallsac", PlayerID: "white_player",
		Target: &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("target1: %v", err)
	}
	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_smallsac", PlayerID: "white_player",
		Target: &contracts.Square{Row: 3, Col: 4},
	}, now.Add(3*time.Second)); err != nil {
		t.Fatalf("target2: %v", err)
	}
	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_smallsac", PlayerID: "white_player",
		Target: &contracts.Square{Row: 4, Col: 4},
	}, now.Add(4*time.Second))
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}

	if state.Board[3][3] != nil || state.Board[3][4] != nil {
		t.Fatal("expected sacrificed pieces removed")
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)+1 {
		t.Fatalf("expected +1 net cards (consume 1, draw 2), got %d", len(result.Match.WhiteHand))
	}
	if result.Match.PendingCard != nil {
		t.Fatal("expected pending cleared")
	}
}

func TestCardBigsacrifice(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_bigsac"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "bigsacrifice")

	state := service.getMatchContainer("test_bigsac").state
	state.Board = emptyBoard()
	state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][4] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[3][3] = &contracts.Piece{Type: "queen", Color: "white"}
	state.Board[3][4] = &contracts.Piece{Type: "rook", Color: "white"}

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_bigsac", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("play_card: %v", err)
	}
	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_bigsac", PlayerID: "white_player",
		Target: &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("target1: %v", err)
	}
	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_bigsac", PlayerID: "white_player",
		Target: &contracts.Square{Row: 3, Col: 4},
	}, now.Add(3*time.Second)); err != nil {
		t.Fatalf("target2: %v", err)
	}
	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_bigsac", PlayerID: "white_player",
		Target: &contracts.Square{Row: 4, Col: 4},
	}, now.Add(4*time.Second))
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}

	if state.Board[3][3] != nil || state.Board[3][4] != nil {
		t.Fatal("expected sacrificed pieces removed")
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)+2 {
		t.Fatalf("expected +2 net cards (consume 1, draw 3), got %d", len(result.Match.WhiteHand))
	}
	if result.Match.PendingCard != nil {
		t.Fatal("expected pending cleared")
	}
}

func TestCardClone(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_clone"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "clone")

	state := service.getMatchContainer("test_clone").state
	state.Board = emptyBoard()
	state.Board[0][0] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[3][3] = &contracts.Piece{Type: "knight", Color: "white"}

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_clone", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("play_card: %v", err)
	}
	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_clone", PlayerID: "white_player",
		Target: &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("source: %v", err)
	}
	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_clone", PlayerID: "white_player",
		Target: &contracts.Square{Row: 4, Col: 4},
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("dest: %v", err)
	}

	if p := service.getMatchContainer("test_clone").state.Board[4][4]; p == nil || p.Type != "knight" {
		t.Fatalf("expected knight clone got %#v", p)
	}
	if src := service.getMatchContainer("test_clone").state.Board[3][3]; src == nil || src.Type != "knight" {
		t.Fatal("expected source preserved")
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}

func TestCardBorrow(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_borrow"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "borrow")

	state := service.getMatchContainer("test_borrow").state
	state.Board = emptyBoard()
	state.Board[0][0] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[3][3] = &contracts.Piece{Type: "rook", Color: "black"}

	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_borrow", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second))
	if err != nil {
		t.Fatalf("play_card: %v", err)
	}
	if result.Match.PendingCard == nil {
		t.Fatal("expected pending")
	}

	result, err = applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_borrow", PlayerID: "white_player",
		Target: &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("target: %v", err)
	}

	if p := service.getMatchContainer("test_borrow").state.Board[3][3]; p == nil || p.Color != "white" || !p.Borrowed {
		t.Fatal("expected borrowed white piece")
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}

func TestCardParasite(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_parasite"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "parasite")

	state := service.getMatchContainer("test_parasite").state
	state.Board = emptyBoard()
	state.Board[0][0] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[3][3] = &contracts.Piece{Type: "rook", Color: "white"}
	state.Board[5][5] = &contracts.Piece{Type: "rook", Color: "black"}

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_parasite", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("play_card: %v", err)
	}
	step1, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_parasite", PlayerID: "white_player",
		Target: &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("host: %v", err)
	}
	if step1.Match.PendingCard == nil || len(step1.Match.PendingCard.Options) != 1 || step1.Match.PendingCard.Options[0] != "5" {
		t.Fatalf("expected host value=5, got %#v", step1.Match.PendingCard)
	}

	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_parasite", PlayerID: "white_player",
		Target: &contracts.Square{Row: 5, Col: 5},
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("target: %v", err)
	}

	if p := service.getMatchContainer("test_parasite").state.Board[3][3]; p == nil || p.ParasiteTarget != "5,5" {
		t.Fatalf("expected parasite link, got %#v", p)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}
func TestCardBlackhole(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)

	// Work directly with state
	state := &contracts.MatchState{
		MatchID:           "test_blackhole",
		Turn:              "white",
		Status:            "active",
		Board:             emptyBoard(),
		WhiteHand:         []contracts.GameCard{cardTemplateByMechanic("blackhole")},
		BlackHand:         []contracts.GameCard{cardTemplateByMechanic("freeze")},
		History:           []contracts.PositionState{},
		RNGSeed:           42,
		WhitePlayerSecret: "white-secret",
	}
	startedAt := now.UnixMilli()
	state.Clock.RunningFor = "white"
	state.Clock.StartedAt = &startedAt
	state.Clock.WhiteMS = 120000
	state.Clock.BlackMS = 120000
	state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][4] = &contracts.Piece{Type: "king", Color: "black"}
	state.History = []contracts.PositionState{capturePositionState(state)}

	cardID := state.WhiteHand[0].ID

	if _, err := applyPlayCard(state, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_blackhole", PlayerID: "white_player", PlayerSecret: "white-secret", CardID: cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("play_card: %v", err)
	}
	if state.PendingCard == nil || state.PendingCard.Mechanic != "blackhole" {
		t.Fatal("expected pending blackhole after play_card")
	}

	if _, err := applySelectTarget(state, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_blackhole", PlayerID: "white_player", PlayerSecret: "white-secret",
		Target: &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("sq1: %v", err)
	}
	if state.PendingCard == nil || state.PendingCard.Target == nil {
		t.Fatal("expected pending target after first select")
	}

	if _, err := applySelectTarget(state, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_blackhole", PlayerID: "white_player", PlayerSecret: "white-secret",
		Target: &contracts.Square{Row: 5, Col: 3},
	}, now.Add(3*time.Second)); err != nil {
		t.Fatalf("sq2: %v", err)
	}

	if len(state.BlackHoles) != 1 || state.BlackHoles[0].TurnsLeft != 2 {
		t.Fatalf("expected blackhole with 2 turns, got %#v", state.BlackHoles)
	}
	if len(state.WhiteHand) != 0 {
		t.Fatal("expected card consumed")
	}
}

func TestCardLavaground(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_lava"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "lavaground")

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_lava", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("play_card: %v", err)
	}
	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_lava", PlayerID: "white_player",
		Target: &contracts.Square{Row: 4, Col: 4},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("target: %v", err)
	}

	if len(result.Match.LavaSquares) != 1 {
		t.Fatalf("expected 1 lava, got %#v", result.Match.LavaSquares)
	}
	lava := result.Match.LavaSquares[0]
	if lava.Row != 4 || lava.Col != 4 || lava.MovesLeft != 2 {
		t.Fatalf("unexpected lava %#v", lava)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}

func TestCardFogVillage(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_fog"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "fog_village")

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_fog", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("play_card: %v", err)
	}
	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_fog", PlayerID: "white_player",
		Target: &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("target: %v", err)
	}

	if len(result.Match.FogZones) != 1 {
		t.Fatalf("expected 1 fog zone, got %#v", result.Match.FogZones)
	}
	zone := result.Match.FogZones[0]
	if zone.TurnsLeft != 2 || zone.OwnerColor != "white" {
		t.Fatalf("unexpected fog %#v", zone)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}

func TestCardFortress(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_fortress"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "fortress")

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_fortress", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("play_card: %v", err)
	}
	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_fortress", PlayerID: "white_player",
		Target: &contracts.Square{Row: 4, Col: 3},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("target: %v", err)
	}

	if len(result.Match.FortressZones) != 1 {
		t.Fatalf("expected 1 fortress, got %#v", result.Match.FortressZones)
	}
	zone := result.Match.FortressZones[0]
	if zone.TurnsLeft != 2 || zone.OwnerColor != "white" {
		t.Fatalf("unexpected fortress %#v", zone)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}

func TestCardDoublemoveDiff(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_double_diff"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "doublemove_diff")

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_double_diff", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("play_card: %v", err)
	}

	first, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "make_move", MatchID: "test_double_diff", PlayerID: "white_player",
		From: &contracts.Square{Row: 1, Col: 4}, To: &contracts.Square{Row: 3, Col: 4},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("first move: %v", err)
	}
	if first.Match.Turn != "white" {
		t.Fatal("expected turn stays white")
	}
	if first.Match.DoubleMove == nil || first.Match.DoubleMove.MovesLeft != 1 {
		t.Fatal("expected 1 move left")
	}

	second, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "make_move", MatchID: "test_double_diff", PlayerID: "white_player",
		From: &contracts.Square{Row: 1, Col: 3}, To: &contracts.Square{Row: 3, Col: 3},
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("second move: %v", err)
	}
	if second.Match.Turn != "black" {
		t.Fatal("expected turn passes")
	}
	if second.Match.DoubleMove != nil {
		t.Fatal("expected double move cleared")
	}
	if len(first.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed on play")
	}
}

func TestCardDoublemoveSame(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_double_same"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "doublemove_same")

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_double_same", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("play_card: %v", err)
	}
	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "make_move", MatchID: "test_double_same", PlayerID: "white_player",
		From: &contracts.Square{Row: 1, Col: 4}, To: &contracts.Square{Row: 3, Col: 4},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("first move: %v", err)
	}
	second, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "make_move", MatchID: "test_double_same", PlayerID: "white_player",
		From: &contracts.Square{Row: 3, Col: 4}, To: &contracts.Square{Row: 4, Col: 4},
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("second move (same piece): %v", err)
	}
	if second.Match.Turn != "black" {
		t.Fatal("expected turn passes after solo double")
	}
	if second.Match.DoubleMove != nil {
		t.Fatal("expected double move cleared")
	}
}

func TestCardReverse(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_reverse"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "reverse")

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "make_move", MatchID: "test_reverse", PlayerID: "white_player",
		From: &contracts.Square{Row: 1, Col: 4}, To: &contracts.Square{Row: 3, Col: 4},
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("white move: %v", err)
	}
	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "make_move", MatchID: "test_reverse", PlayerID: "black_player",
		From: &contracts.Square{Row: 6, Col: 4}, To: &contracts.Square{Row: 4, Col: 4},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("black move: %v", err)
	}

	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_reverse", PlayerID: "white_player", CardID: cardID,
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("reverse: %v", err)
	}

	if p := service.getMatchContainer("test_reverse").state.Board[4][4]; p != nil {
		t.Fatal("expected reversed black pawn gone from e5")
	}
	if p := service.getMatchContainer("test_reverse").state.Board[6][4]; p == nil || p.Type != "pawn" || p.Color != "black" {
		t.Fatal("expected black pawn restored to e7")
	}
	if result.Match.Turn != "white" {
		t.Fatal("expected turn stays white")
	}
	if len(result.Match.MoveHistory) != 1 {
		t.Fatal("expected single move in history")
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}

func TestCardUndo(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_undo"}, now)
	undoCardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "undo")
	freezeCardID := cardIDByMechanic(t, snapshot.Match.BlackHand, "freeze")

	armed, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_undo", PlayerID: "white_player", CardID: undoCardID,
	}, now.Add(time.Second))
	if err != nil {
		t.Fatalf("play_card undo: %v", err)
	}
	if armed.Match.UndoAgainst != "black" {
		t.Fatal("expected undo armed against black")
	}

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "make_move", MatchID: "test_undo", PlayerID: "white_player",
		From: &contracts.Square{Row: 1, Col: 4}, To: &contracts.Square{Row: 3, Col: 4},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("white move: %v", err)
	}

	nullified, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_undo", PlayerID: "black_player", CardID: freezeCardID,
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("black card nullified: %v", err)
	}
	if nullified.Match.PendingCard != nil {
		t.Fatal("expected no pending after nullify")
	}
	if nullified.Match.UndoAgainst != "" {
		t.Fatal("expected undo cleared")
	}
	if len(nullified.Match.BlackHand) != len(snapshot.Match.BlackHand)-1 {
		t.Fatal("expected black card consumed")
	}
}

func TestCardGambler(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_gambler"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "gambler")

	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_gambler", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second))
	if err != nil {
		t.Fatalf("play_card: %v", err)
	}

	if len(result.Events) == 0 || result.Events[0].Type != "card_played" {
		t.Fatal("expected card_played event")
	}
	outcome, _ := 	result.Events[0].Payload["outcome"].(string)
	if outcome != "win" && outcome != "lose" && outcome != "none" {
		t.Fatalf("unexpected outcome %q", outcome)
	}
	if len(result.Match.WhiteHand) < len(snapshot.Match.WhiteHand)-2 || len(result.Match.WhiteHand) > len(snapshot.Match.WhiteHand) {
		t.Fatalf("unexpected hand size: got %d, expected between %d and %d",
			len(result.Match.WhiteHand), len(snapshot.Match.WhiteHand)-2, len(snapshot.Match.WhiteHand))
	}
}

func TestCardRadar(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_radar"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "radar")

	armed, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_radar", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second))
	if err != nil {
		t.Fatalf("play_card: %v", err)
	}
	if armed.Match.RadarRevealFor != "white" {
		t.Fatal("expected radarRevealFor white")
	}

	moved, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "make_move", MatchID: "test_radar", PlayerID: "white_player",
		From: &contracts.Square{Row: 1, Col: 4}, To: &contracts.Square{Row: 3, Col: 4},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	if moved.Match.RadarRevealFor != "" {
		t.Fatal("expected radar cleared after turn")
	}
	if len(moved.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}

func TestCardMirror(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_mirror"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "mirror")

	state := service.getMatchContainer("test_mirror").state
	state.Board = emptyBoard()
	state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][4] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[3][1] = &contracts.Piece{Type: "knight", Color: "white"}
	state.Board[5][2] = &contracts.Piece{Type: "knight", Color: "black"}
	state.LastMove = &contracts.LastMove{
		From: contracts.Square{Row: 7, Col: 1}, To: contracts.Square{Row: 5, Col: 2},
	}
	state.History = []contracts.PositionState{capturePositionState(state)}

	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_mirror", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second))
	if err != nil {
		t.Fatalf("mirror: %v", err)
	}

	if state.Board[3][1] != nil {
		t.Fatal("expected mirror source empty")
	}
	if p := state.Board[1][2]; p == nil || p.Type != "knight" || p.Color != "white" {
		t.Fatal("expected mirrored knight at c2")
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}

func TestCardCheater(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_cheater"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "cheater")

	armed, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_cheater", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second))
	if err != nil {
		t.Fatalf("play_card: %v", err)
	}
	if armed.Match.CheaterState == nil || armed.Match.CheaterState.TurnsLeft != 3 {
		t.Fatalf("expected cheater 3 turns, got %#v", armed.Match.CheaterState)
	}

	afterMove, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "make_move", MatchID: "test_cheater", PlayerID: "white_player",
		From: &contracts.Square{Row: 1, Col: 4}, To: &contracts.Square{Row: 3, Col: 4},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	if afterMove.Match.CheaterState == nil || afterMove.Match.CheaterState.TurnsLeft != 2 {
		t.Fatalf("expected cheater decrement to 2, got %#v", afterMove.Match.CheaterState)
	}
	if len(afterMove.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}

func TestCardInvisible(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_invisible"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "invisible")

	state := service.getMatchContainer("test_invisible").state
	state.Board = emptyBoard()
	state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][4] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[0][0] = &contracts.Piece{Type: "rook", Color: "white"}

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_invisible", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("play_card: %v", err)
	}
	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_invisible", PlayerID: "white_player",
		Target: &contracts.Square{Row: 0, Col: 0},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("target: %v", err)
	}

	if p := service.getMatchContainer("test_invisible").state.Board[0][0]; p != nil {
		t.Fatal("expected invisible piece removed from board")
	}
	if service.getMatchContainer("test_invisible").state.InvisiblePiece == nil {
		t.Fatal("expected invisible ghost state")
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}

func TestCardFakepiece(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_fake"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "fakepiece")

	state := service.getMatchContainer("test_fake").state
	state.Board = emptyBoard()
	state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][4] = &contracts.Piece{Type: "king", Color: "black"}
	state.History = []contracts.PositionState{capturePositionState(state)}

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_fake", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("play_card: %v", err)
	}
	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_fake", PlayerID: "white_player",
		Target: &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("target: %v", err)
	}

	if p := state.Board[3][3]; p == nil || p.Type != "pawn" || p.Color != "white" || !p.Fake {
		t.Fatalf("expected fake pawn, got %#v", p)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}

func TestCardHalffuse(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_halffuse"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "halffuse")

	state := service.getMatchContainer("test_halffuse").state
	state.Board = emptyBoard()
	state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][4] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[3][3] = &contracts.Piece{Type: "pawn", Color: "white"}
	state.Board[3][4] = &contracts.Piece{Type: "knight", Color: "white"}

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_halffuse", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("play_card: %v", err)
	}
	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_halffuse", PlayerID: "white_player",
		Target: &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("first: %v", err)
	}
	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_halffuse", PlayerID: "white_player",
		Target: &contracts.Square{Row: 3, Col: 4},
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("second: %v", err)
	}

	if src := state.Board[3][3]; src != nil {
		t.Fatal("expected first piece consumed")
	}
	if fused := state.Board[3][4]; fused == nil || fused.Type != "knight" || fused.FusedWith != "pawn" {
		t.Fatalf("expected fused knight+pawn, got %#v", fused)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}

func TestCardFullfusion(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_fullfusion"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "fullfusion")

	state := service.getMatchContainer("test_fullfusion").state
	state.Board = emptyBoard()
	state.Board[0][0] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[3][3] = &contracts.Piece{Type: "queen", Color: "white"}
	state.Board[3][4] = &contracts.Piece{Type: "knight", Color: "white"}

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_fullfusion", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("play_card: %v", err)
	}
	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_fullfusion", PlayerID: "white_player",
		Target: &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("first: %v", err)
	}
	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_fullfusion", PlayerID: "white_player",
		Target: &contracts.Square{Row: 3, Col: 4},
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("second: %v", err)
	}

	if src := state.Board[3][3]; src != nil {
		t.Fatal("expected first piece consumed")
	}
	if fused := state.Board[3][4]; fused == nil || fused.Type != "knight" || fused.FusedWith != "queen" {
		t.Fatalf("expected fused knight+queen, got %#v", fused)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}

func TestCardMindcontrol(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_mindcontrol"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "mindcontrol")

	state := service.getMatchContainer("test_mindcontrol").state
	state.Board = emptyBoard()
	state.Board[0][0] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[3][3] = &contracts.Piece{Type: "rook", Color: "black"}

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_mindcontrol", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("play_card: %v", err)
	}
	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_mindcontrol", PlayerID: "white_player",
		Target: &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("target: %v", err)
	}

	if p := service.getMatchContainer("test_mindcontrol").state.Board[3][3]; p == nil || p.Color != "white" || p.Borrowed {
		t.Fatal("expected permanently white piece (not borrowed)")
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}

func TestCardJoker(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_joker"}, now)
	state := service.getMatchContainer("test_joker").state
	state.WhiteHand = []contracts.GameCard{cardTemplateByMechanic("joker")}
	snapshot.Match.WhiteHand = state.WhiteHand
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "joker")

	armed, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_joker", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second))
	if err != nil {
		t.Fatalf("play_card: %v", err)
	}
	if armed.Match.PendingCard == nil || armed.Match.PendingCard.Mechanic != "joker" {
		t.Fatal("expected joker pending")
	}

	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_joker", PlayerID: "white_player",
		SelectionID: "freeze",
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("transform: %v", err)
	}

	if result.Match.PendingCard != nil {
		t.Fatal("expected pending cleared")
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand) {
		t.Fatal("expected hand size unchanged")
	}
	found := false
	for _, card := range result.Match.WhiteHand {
		if strings.HasPrefix(card.ID, "joker_freeze_white_") && card.Mechanic == "freeze" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected transformed freeze card, got %#v", result.Match.WhiteHand)
	}
}

func TestCardUnabomber(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := createTestMatch(service, contracts.CreateMatchRequest{MatchID: "test_unabomber"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "unabomber")

	state := service.getMatchContainer("test_unabomber").state
	state.Board = emptyBoard()
	state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][4] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[1][0] = &contracts.Piece{Type: "pawn", Color: "white"}

	if _, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "play_card", MatchID: "test_unabomber", PlayerID: "white_player", CardID: cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("play_card: %v", err)
	}
	result, err := applyTestIntent(service, contracts.PlayerIntent{
		Type: "select_target", MatchID: "test_unabomber", PlayerID: "white_player",
		Target: &contracts.Square{Row: 1, Col: 0},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("target: %v", err)
	}

	if p := state.Board[1][0]; p == nil || !p.Bomb {
		t.Fatal("expected bomb on piece")
	}
	if len(state.BombPieces) != 1 {
		t.Fatalf("expected 1 bomb tracker, got %#v", state.BombPieces)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatal("expected card consumed")
	}
}

// TestCardMechanicsGolden plays all 37 card mechanics in a single match and snapshots the board.
func TestCardMechanicsGolden(t *testing.T) {
	t.Parallel()
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)

	mechanics := []string{
		"badsniper", "demote", "gambler", "promotehim", "halffuse",
		"swapme", "jump", "smallsacrifice", "freeze", "promote",
		"shield", "fog_village", "fullfusion", "swapus", "swaphim",
		"doublemove_diff", "doublemove_same", "demotehim", "fakepiece",
		"teleport", "lavaground", "radar", "mirror", "cheater",
		"invisible", "sniper", "fortress", "clone", "borrow",
		"parasite", "blackhole", "bigsacrifice", "undo", "reverse",
		"unabomber", "mindcontrol", "joker",
	}

	createTestMatch(service, contracts.CreateMatchRequest{
		MatchID: "golden_all_37",
		Seed:    42,
	}, now)

	type goldenEntry struct {
		Index       int                      `json:"index"`
		Mechanic    string                   `json:"mechanic"`
		CardID      string                   `json:"cardId"`
		BeforeBoard [][]*contracts.Piece     `json:"beforeBoard,omitempty"`
		AfterBoard  [][]*contracts.Piece     `json:"afterBoard,omitempty"`
		State       *contracts.MatchState    `json:"state,omitempty"`
		Events      []contracts.ResolvedEvent `json:"events,omitempty"`
		Error       string                   `json:"error,omitempty"`
	}

	type goldenResult struct {
		MatchID string        `json:"matchId"`
		Entries []goldenEntry `json:"entries"`
	}

	result := goldenResult{MatchID: "golden_all_37"}

	state := service.getMatchContainer("golden_all_37").state

	for idx, mech := range mechanics {
		entry := goldenEntry{
			Index:    idx,
			Mechanic: mech,
		}

		cardID := ""
		for _, c := range state.WhiteHand {
			if c.Mechanic == mech {
				cardID = c.ID
				break
			}
		}
		for _, c := range state.BlackHand {
			if c.Mechanic == mech {
				cardID = c.ID
				break
			}
		}
		if cardID == "" && mech != "joker" {
			entry.Error = "card not found in either hand"
			result.Entries = append(result.Entries, entry)
			continue
		}

		if mech == "joker" {
			state.WhiteHand = []contracts.GameCard{cardTemplateByMechanic("joker")}
			cardID = state.WhiteHand[0].ID
		}

		// Set up board for mechanics that require specific piece configurations
		entry.BeforeBoard = cloneBoard(state.Board)
		var err error

		switch mech {
		case "badsniper", "demote", "promotehim", "demotehim", "swapme", "swapus", "swaphim",
			"jump", "smallsacrifice", "bigsacrifice", "clone", "borrow", "parasite",
			"mindcontrol", "invisible", "fakepiece", "fullfusion", "halffuse", "mirror",
			"unabomber", "blackhole":
			state.Board = emptyBoard()
			state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
			state.Board[7][4] = &contracts.Piece{Type: "king", Color: "black"}
			switch mech {
			case "badsniper":
				state.Board[2][2] = &contracts.Piece{Type: "knight", Color: "white"}
			case "demote":
				state.Board[3][3] = &contracts.Piece{Type: "queen", Color: "white"}
			case "promotehim":
				state.Board[6][0] = &contracts.Piece{Type: "pawn", Color: "black"}
			case "demotehim":
				state.Board[3][3] = &contracts.Piece{Type: "queen", Color: "black"}
			case "swapme":
				state.Board[2][2] = &contracts.Piece{Type: "knight", Color: "white"}
				state.Board[4][4] = &contracts.Piece{Type: "rook", Color: "white"}
			case "swapus":
				state.Board[2][2] = &contracts.Piece{Type: "rook", Color: "white"}
				state.Board[5][5] = &contracts.Piece{Type: "knight", Color: "black"}
			case "swaphim":
				state.Board[5][5] = &contracts.Piece{Type: "knight", Color: "black"}
				state.Board[6][4] = &contracts.Piece{Type: "rook", Color: "black"}
			case "jump":
				state.Board[3][3] = &contracts.Piece{Type: "rook", Color: "white"}
				state.Board[3][4] = &contracts.Piece{Type: "pawn", Color: "black"}
			case "smallsacrifice":
				state.Board[3][3] = &contracts.Piece{Type: "rook", Color: "white"}
				state.Board[3][4] = &contracts.Piece{Type: "bishop", Color: "white"}
			case "bigsacrifice":
				state.Board[3][3] = &contracts.Piece{Type: "queen", Color: "white"}
				state.Board[3][4] = &contracts.Piece{Type: "rook", Color: "white"}
			case "clone":
				state.Board[3][3] = &contracts.Piece{Type: "knight", Color: "white"}
			case "borrow":
				state.Board[3][3] = &contracts.Piece{Type: "rook", Color: "black"}
			case "parasite":
				state.Board[3][3] = &contracts.Piece{Type: "rook", Color: "white"}
				state.Board[5][5] = &contracts.Piece{Type: "rook", Color: "black"}
			case "mindcontrol":
				state.Board[3][3] = &contracts.Piece{Type: "rook", Color: "black"}
			case "invisible":
				state.Board[0][0] = &contracts.Piece{Type: "rook", Color: "white"}
			case "fakepiece":
			case "fullfusion":
				state.Board[3][3] = &contracts.Piece{Type: "queen", Color: "white"}
				state.Board[3][4] = &contracts.Piece{Type: "knight", Color: "white"}
			case "halffuse":
				state.Board[3][3] = &contracts.Piece{Type: "pawn", Color: "white"}
				state.Board[3][4] = &contracts.Piece{Type: "knight", Color: "white"}
			case "mirror":
				state.Board[3][1] = &contracts.Piece{Type: "knight", Color: "white"}
				state.Board[5][2] = &contracts.Piece{Type: "knight", Color: "black"}
				state.LastMove = &contracts.LastMove{
					From: contracts.Square{Row: 7, Col: 1}, To: contracts.Square{Row: 5, Col: 2},
				}
			case "unabomber":
				state.Board[1][0] = &contracts.Piece{Type: "pawn", Color: "white"}
			case "blackhole":
			}
			state.History = []contracts.PositionState{capturePositionState(state)}

		case "reverse":
			state.History = []contracts.PositionState{capturePositionState(state)}
			state.MoveHistory = []string{"e4", "e5"}
			// set up a position with history 2 entries deep
			state.Board[4][4] = &contracts.Piece{Type: "pawn", Color: "white"}
			state.Board[1][4] = nil
			state.Board[4][4] = &contracts.Piece{Type: "pawn", Color: "black"}
			state.Board[6][4] = nil
		}

		now = now.Add(time.Second)

		// Play the card
		if _, playErr := applyPlayCard(state, contracts.PlayerIntent{
			Type: "play_card", MatchID: "golden_all_37", PlayerID: "white_player", CardID: cardID,
		}, now); playErr != nil && mech != "gambler" {
			entry.Error = playErr.Error()
			result.Entries = append(result.Entries, entry)
			state.WhiteHand = nil
			state.BlackHand = nil
			state.PendingCard = nil
			continue
		}

		now = now.Add(time.Second)

		// Select targets based on mechanic
		switch mech {
		case "freeze":
			_, err = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 6, Col: 0},
			}, now)
		case "shield":
			_, err = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 1, Col: 0},
			}, now)
		case "sniper":
			_, err = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 6, Col: 0},
			}, now)
		case "badsniper":
			_, err = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 2, Col: 2},
			}, now)
		case "promote", "promotehim", "demote", "demotehim":
			selectTarget := contracts.Square{Row: 6, Col: 0}
			if mech == "demote" || mech == "demotehim" {
				selectTarget = contracts.Square{Row: 3, Col: 3}
			}
			_, selErr := applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &selectTarget,
			}, now)
			if selErr != nil {
				err = selErr
				break
			}
			now = now.Add(time.Second)
			_, err = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				SelectionID: "queen",
			}, now)
		case "teleport":
			_, selErr := applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 1, Col: 0},
			}, now)
			if selErr != nil {
				err = selErr
				break
			}
			now = now.Add(time.Second)
			_, err = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 4, Col: 4},
			}, now)
		case "jump":
			_, selErr := applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 3, Col: 3},
			}, now)
			if selErr != nil {
				err = selErr
				break
			}
			now = now.Add(time.Second)
			_, err = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 3, Col: 5},
			}, now)
		case "swapme":
			_, selErr := applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 2, Col: 2},
			}, now)
			if selErr != nil {
				err = selErr
				break
			}
			now = now.Add(time.Second)
			_, err = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 4, Col: 4},
			}, now)
		case "swapus":
			_, selErr := applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 2, Col: 2},
			}, now)
			if selErr != nil {
				err = selErr
				break
			}
			now = now.Add(time.Second)
			_, err = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 5, Col: 5},
			}, now)
		case "swaphim":
			_, selErr := applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 5, Col: 5},
			}, now)
			if selErr != nil {
				err = selErr
				break
			}
			now = now.Add(time.Second)
			_, err = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 6, Col: 4},
			}, now)
		case "clone":
			_, selErr := applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 3, Col: 3},
			}, now)
			if selErr != nil {
				err = selErr
				break
			}
			now = now.Add(time.Second)
			_, err = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 4, Col: 4},
			}, now)
		case "borrow":
			_, err = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 3, Col: 3},
			}, now)
		case "mindcontrol":
			_, err = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 3, Col: 3},
			}, now)
		case "parasite":
			_, selErr := applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 3, Col: 3},
			}, now)
			if selErr != nil {
				err = selErr
				break
			}
			now = now.Add(time.Second)
			_, err = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 5, Col: 5},
			}, now)
		case "fakepiece":
			_, err = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 3, Col: 3},
			}, now)
		case "lavaground":
			_, err = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 4, Col: 4},
			}, now)
		case "fog_village":
			_, err = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 3, Col: 3},
			}, now)
		case "fortress":
			_, err = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 4, Col: 3},
			}, now)
		case "invisible":
			_, err = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 0, Col: 0},
			}, now)
		case "unabomber":
			_, err = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 1, Col: 0},
			}, now)
		case "blackhole":
			_, selErr := applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 3, Col: 3},
			}, now)
			if selErr != nil {
				err = selErr
				break
			}
			now = now.Add(time.Second)
			_, err = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 5, Col: 3},
			}, now)
		case "smallsacrifice":
			_, selErr := applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 3, Col: 3},
			}, now)
			if selErr != nil {
				err = selErr
				break
			}
			now = now.Add(time.Second)
			_, selErr = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 3, Col: 4},
			}, now)
			if selErr != nil {
				err = selErr
				break
			}
			now = now.Add(time.Second)
			_, err = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 4, Col: 4},
			}, now)
		case "bigsacrifice":
			_, selErr := applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 3, Col: 3},
			}, now)
			if selErr != nil {
				err = selErr
				break
			}
			now = now.Add(time.Second)
			_, selErr = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 3, Col: 4},
			}, now)
			if selErr != nil {
				err = selErr
				break
			}
			now = now.Add(time.Second)
			_, err = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 4, Col: 4},
			}, now)
		case "halffuse":
			_, selErr := applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 3, Col: 3},
			}, now)
			if selErr != nil {
				err = selErr
				break
			}
			now = now.Add(time.Second)
			_, err = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 3, Col: 4},
			}, now)
		case "fullfusion":
			_, selErr := applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 3, Col: 3},
			}, now)
			if selErr != nil {
				err = selErr
				break
			}
			now = now.Add(time.Second)
			_, err = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				Target: &contracts.Square{Row: 3, Col: 4},
			}, now)
		case "reverse":
			// Reverse play_card already handled in applyPlayCard; no select_target needed
			err = nil
		case "undo", "doublemove_diff", "doublemove_same", "gambler", "radar", "cheater", "mirror":
			// These handle their target in play_card, no select_target needed
			err = nil
		case "joker":
			// Joker needs transform selection
			_, err = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "golden_all_37", PlayerID: "white_player",
				SelectionID: "freeze",
			}, now)
		}

		entry.AfterBoard = cloneBoard(state.Board)
		if err != nil {
			entry.Error = err.Error()
		}

		result.Entries = append(result.Entries, entry)

		// Reset hands and pending for next card
		state.WhiteHand = nil
		state.BlackHand = nil
		state.PendingCard = nil
		state.LastMove = nil
	}

	dir := filepath.Join("testdata", "golden")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	goldenPath := filepath.Join(dir, "mechanics_golden.json")
	if err := os.WriteFile(goldenPath, data, 0644); err != nil {
		t.Fatalf("write golden: %v", err)
	}

	t.Logf("wrote golden snapshot: %s (%d entries)", goldenPath, len(result.Entries))
}

func FuzzApplyCard(f *testing.F) {
	mechanics := []string{
		"freeze", "shield", "sniper", "badsniper", "promote", "demote",
		"promotehim", "demotehim", "teleport", "jump", "swapme", "swapus",
		"swaphim", "clone", "borrow", "mindcontrol", "parasite", "fakepiece",
		"lavaground", "fog_village", "fortress", "invisible", "unabomber",
		"blackhole", "smallsacrifice", "bigsacrifice", "halffuse", "fullfusion",
	}

	// Seed corpus with a few representative inputs
	for i := 0; i < len(mechanics); i++ {
		f.Add(i, 0, 0, 0, 0)
	}
	f.Add(0, 3, 4, 5, 5)

	f.Fuzz(func(t *testing.T, mechanicIdx int, targetRow int, targetCol int, targetRow2 int, targetCol2 int) {
		if mechanicIdx < 0 || mechanicIdx >= len(mechanics) {
			return
		}
		mech := mechanics[mechanicIdx]

		service := NewService()
		now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
		snapshot := createTestMatch(service, contracts.CreateMatchRequest{
			MatchID: "fuzz_" + mech,
			Seed:    int64(mechanicIdx)*1000 + int64(targetRow)*100 + int64(targetCol),
		}, now)

		state := service.getMatchContainer("fuzz_" + mech).state
		cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, mech)

		// Place random pieces on the board
		state.Board = emptyBoard()
		state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
		state.Board[7][4] = &contracts.Piece{Type: "king", Color: "black"}

		// Clamp target squares
		tRow := clampInt(targetRow%8, 0, 7)
		tCol := clampInt(targetCol%8, 0, 7)
		tRow2 := clampInt(targetRow2%8, 0, 7)
		tCol2 := clampInt(targetCol2%8, 0, 7)

		// Place some random pieces
		types := []string{"pawn", "knight", "bishop", "rook", "queen"}
		rng := rand.New(rand.NewSource(int64(targetRow)*1000 + int64(targetCol)))
		for i := 0; i < 3; i++ {
			r := rng.Intn(8)
			c := rng.Intn(8)
			if state.Board[r][c] == nil {
				state.Board[r][c] = &contracts.Piece{
					Type:  types[rng.Intn(len(types))],
					Color: []string{"white", "black"}[rng.Intn(2)],
				}
			}
		}
		state.History = []contracts.PositionState{capturePositionState(state)}

		// Try to play the card
		_, playErr := applyPlayCard(state, contracts.PlayerIntent{
			Type: "play_card", MatchID: "fuzz_" + mech, PlayerID: "white_player", CardID: cardID,
		}, now.Add(time.Second))

		if playErr != nil {
			return // expected for many configurations
		}

		// Try to select target (may fail for many reasons, that's OK)
		sq := contracts.Square{Row: tRow, Col: tCol}
		_, selErr := applySelectTarget(state, contracts.PlayerIntent{
			Type: "select_target", MatchID: "fuzz_" + mech, PlayerID: "white_player",
			Target: &sq,
		}, now.Add(2*time.Second))

		if selErr != nil && tRow != tRow2 && tCol != tCol2 {
			// Try second target for multi-step mechanics
			sq2 := contracts.Square{Row: tRow2, Col: tCol2}
			_, _ = applySelectTarget(state, contracts.PlayerIntent{
				Type: "select_target", MatchID: "fuzz_" + mech, PlayerID: "white_player",
				Target: &sq2,
			}, now.Add(3*time.Second))
		}

		// No panic = fuzz passed
	})
}
