package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type GameRecord struct {
	MatchID      string    `json:"matchId"`
	ModeID       string    `json:"modeId"`
	WhiteID      string    `json:"whiteId"`
	BlackID      string    `json:"blackId"`
	WhiteName    string    `json:"whiteName"`
	BlackName    string    `json:"blackName"`
	Winner       string    `json:"winner"`
	TotalMoves   int       `json:"totalMoves"`
	Duration     int       `json:"durationSeconds"`
	CardsPlayed  int       `json:"cardsPlayed"`
	CardTypes    []string  `json:"cardTypes"`
	CreatedAt    time.Time `json:"createdAt"`
	FinishedAt   time.Time `json:"finishedAt"`
}

type SearchResult struct {
	Total  int          `json:"total"`
	Hits   []GameRecord `json:"hits"`
}

type SearchQuery struct {
	Query     string `json:"query,omitempty"`
	PlayerID  string `json:"playerId,omitempty"`
	ModeID    string `json:"modeId,omitempty"`
	Winner    string `json:"winner,omitempty"`
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
	SortBy    string `json:"sortBy"`
	SortOrder string `json:"sortOrder"`
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) CreateIndex(ctx context.Context) error {
	mapping := `{
  "mappings": {
    "properties": {
      "matchId":     { "type": "keyword" },
      "modeId":      { "type": "keyword" },
      "whiteId":     { "type": "keyword" },
      "blackId":     { "type": "keyword" },
      "whiteName":   { "type": "text", "fields": { "keyword": { "type": "keyword" } } },
      "blackName":   { "type": "text", "fields": { "keyword": { "type": "keyword" } } },
      "winner":      { "type": "keyword" },
      "totalMoves":  { "type": "integer" },
      "duration":    { "type": "integer" },
      "cardsPlayed": { "type": "integer" },
      "cardTypes":   { "type": "keyword" },
      "createdAt":   { "type": "date" },
      "finishedAt":  { "type": "date" }
    }
  }
}`

	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/games", bytes.NewReader([]byte(mapping)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("create index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create index: status %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (c *Client) IndexGame(ctx context.Context, game *GameRecord) error {
	body, _ := json.Marshal(game)
	url := fmt.Sprintf("%s/games/_doc/%s", c.baseURL, game.MatchID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("index game: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("index game: status %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (c *Client) SearchGames(ctx context.Context, query SearchQuery) (*SearchResult, error) {
	must := []interface{}{}

	if query.Query != "" {
		must = append(must, map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  query.Query,
				"fields": []string{"whiteName", "blackName", "matchId"},
			},
		})
	}

	if query.PlayerID != "" {
		must = append(must, map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []interface{}{
					map[string]interface{}{"term": map[string]interface{}{"whiteId": query.PlayerID}},
					map[string]interface{}{"term": map[string]interface{}{"blackId": query.PlayerID}},
				},
			},
		})
	}

	if query.ModeID != "" {
		must = append(must, map[string]interface{}{"term": map[string]interface{}{"modeId": query.ModeID}})
	}

	if query.Winner != "" {
		must = append(must, map[string]interface{}{"term": map[string]interface{}{"winner": query.Winner}})
	}

	bodyMap := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": must,
			},
		},
		"from": (query.Page - 1) * query.PageSize,
		"size": query.PageSize,
		"sort": []interface{}{
			map[string]interface{}{
				query.SortBy: map[string]interface{}{
					"order": query.SortOrder,
				},
			},
		},
	}

	body, _ := json.Marshal(bodyMap)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/games/_search", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search games: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search games: status %d: %s", resp.StatusCode, string(b))
	}

	var esResp struct {
		Hits struct {
			Total struct {
				Value int `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source GameRecord `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&esResp); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}

	hits := make([]GameRecord, len(esResp.Hits.Hits))
	for i, h := range esResp.Hits.Hits {
		hits[i] = h.Source
	}

	return &SearchResult{
		Total: esResp.Hits.Total.Value,
		Hits:  hits,
	}, nil
}
