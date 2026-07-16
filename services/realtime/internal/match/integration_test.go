package match_test

import (
	"testing"
	"time"

	"github.com/chess404/realtime/internal/contracts"
	"github.com/chess404/realtime/internal/match"
)

func TestIntegrationPlayAlternatingMoves(t *testing.T) {
	service := match.NewService()
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)

	snapshot := service.CreateMatch(contracts.CreateMatchRequest{
		MatchID:           "integration_test",
		WhitePlayerSecret: "white-secret",
		BlackPlayerSecret: "black-secret",
	}, now)

	if snapshot.Match.Status != "active" {
		t.Fatalf("expected active match, got %s", snapshot.Match.Status)
	}
	if snapshot.Match.Turn != "white" {
		t.Fatalf("expected white to move first, got %s", snapshot.Match.Turn)
	}

	moves := []struct {
		playerID string
		from     contracts.Square
		to       contracts.Square
	}{
		{"white_player", contracts.Square{Row: 1, Col: 4}, contracts.Square{Row: 3, Col: 4}},
		{"black_player", contracts.Square{Row: 6, Col: 4}, contracts.Square{Row: 4, Col: 4}},
		{"white_player", contracts.Square{Row: 0, Col: 6}, contracts.Square{Row: 2, Col: 5}},
		{"black_player", contracts.Square{Row: 7, Col: 1}, contracts.Square{Row: 5, Col: 2}},
		{"white_player", contracts.Square{Row: 0, Col: 5}, contracts.Square{Row: 3, Col: 2}},
	}

	for i, move := range moves {
		now = now.Add(10 * time.Second)
		result, err := service.ApplyIntent(contracts.PlayerIntent{
			Type:         "make_move",
			MatchID:      "integration_test",
			PlayerID:     move.playerID,
			PlayerSecret: testSecret(move.playerID),
			From:         &move.from,
			To:           &move.to,
		}, now)
		if err != nil {
			t.Fatalf("move %d (%s) failed: %v", i+1, move.playerID, err)
		}
		if result.Match.MatchID != "integration_test" {
			t.Fatalf("move %d: matchID changed", i+1)
		}
		nonNilPieces := 0
		for _, row := range result.Match.Board {
			for _, p := range row {
				if p != nil {
					nonNilPieces++
				}
			}
		}
		if nonNilPieces < 4 {
			t.Fatalf("move %d: expected at least 4 pieces on board, got %d", i+1, nonNilPieces)
		}
	}

	final, err := service.GetMatch("integration_test")
	if err != nil {
		t.Fatalf("failed to get match: %v", err)
	}

	if final.Match.Turn != "black" {
		t.Fatalf("expected black to move after 5 moves, got %s", final.Match.Turn)
	}
	if len(final.Match.MoveHistory) != 5 {
		t.Fatalf("expected 5 moves in history, got %d", len(final.Match.MoveHistory))
	}
	if len(final.Match.Board) == 0 {
		t.Fatalf("expected non-empty board")
	}
}

func testSecret(playerID string) string {
	switch {
	case playerID == "white_player" || playerID == "white":
		return "white-secret"
	default:
		return "black-secret"
	}
}
