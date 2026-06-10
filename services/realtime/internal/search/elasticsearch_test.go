package search

import (
	"encoding/json"
	"testing"
)

func TestGameRecordJSON(t *testing.T) {
	game := &GameRecord{
		MatchID:     "match-123",
		ModeID:      "open_cards",
		WhiteID:     "player-1",
		BlackID:     "player-2",
		WhiteName:   "Alice",
		BlackName:   "Bob",
		Winner:      "white",
		TotalMoves:  25,
		Duration:    300,
		CardsPlayed: 5,
		CardTypes:   []string{"freeze", "swap", "clone"},
	}

	data, err := json.Marshal(game)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded GameRecord
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.MatchID != game.MatchID {
		t.Errorf("matchId = %s, want %s", decoded.MatchID, game.MatchID)
	}
	if decoded.TotalMoves != game.TotalMoves {
		t.Errorf("totalMoves = %d, want %d", decoded.TotalMoves, game.TotalMoves)
	}
	if len(decoded.CardTypes) != 3 {
		t.Errorf("cardTypes len = %d, want 3", len(decoded.CardTypes))
	}
}

func TestSearchQueryDefaults(t *testing.T) {
	q := SearchQuery{
		Query: "alice",
		Page:  1,
	}

	if q.PageSize == 0 {
		q.PageSize = 20
	}
	if q.SortBy == "" {
		q.SortBy = "createdAt"
	}
	if q.SortOrder == "" {
		q.SortOrder = "desc"
	}

	if q.PageSize != 20 {
		t.Errorf("pageSize = %d, want 20", q.PageSize)
	}
	if q.SortBy != "createdAt" {
		t.Errorf("sortBy = %s, want createdAt", q.SortBy)
	}
	if q.SortOrder != "desc" {
		t.Errorf("sortOrder = %s, want desc", q.SortOrder)
	}
}

func TestNewClient(t *testing.T) {
	c := NewClient("http://localhost:9200")
	if c.baseURL != "http://localhost:9200" {
		t.Errorf("baseURL = %s, want http://localhost:9200", c.baseURL)
	}
	if c.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}
