package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/chess404/realtime/internal/anticheat"
	"github.com/chess404/realtime/internal/contracts"
	"github.com/chess404/realtime/internal/match"
)

type Job struct {
	MatchID  string `json:"matchId"`
	WhiteID  string `json:"whiteId"`
	BlackID  string `json:"blackId"`
	GameJSON string `json:"gameJson"`
}

type Result struct {
	MatchID  string                    `json:"matchId"`
	PlayerID string                    `json:"playerId"`
	Result   *anticheat.AnalysisResult `json:"result"`
}

func main() {
	log.SetPrefix("[analysis-worker] ")
	log.Println("Starting analysis worker...")

	matchServiceURL := os.Getenv("MATCH_SERVICE_INTERNAL_URL")
	if matchServiceURL == "" {
		log.Println("WARN: MATCH_SERVICE_INTERNAL_URL not set, analysis disabled")
		return
	}
	serviceToken := os.Getenv("INTERNAL_SERVICE_TOKEN")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Shutting down...")
		cancel()
	}()

	worker := &Worker{
		matchServiceURL: matchServiceURL,
		serviceToken:    serviceToken,
		httpClient:      &http.Client{Timeout: 10 * time.Second},
	}
	log.Println("Worker ready, polling for jobs...")
	worker.Run(ctx)
}

type Worker struct {
	matchServiceURL string
	serviceToken    string
	httpClient      *http.Client
	analyzed        map[string]struct{}
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.pollAndProcess(ctx)
		}
	}
}

func (w *Worker) pollAndProcess(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	if w.analyzed == nil {
		w.analyzed = make(map[string]struct{})
	}

	ids, err := w.fetchFinishedMatchIDs(ctx)
	if err != nil {
		log.Printf("poll: fetch finished match ids failed: %v", err)
		return
	}

	processed := 0
	for _, matchID := range ids {
		if _, ok := w.analyzed[matchID]; ok {
			continue
		}
		if err := w.processMatch(ctx, matchID); err != nil {
			log.Printf("process: match %s failed: %v", matchID, err)
			continue
		}
		w.analyzed[matchID] = struct{}{}
		processed++
	}
	if processed > 0 {
		log.Printf("processed %d match(es)", processed)
	}
}

func (w *Worker) fetchFinishedMatchIDs(ctx context.Context) ([]string, error) {
	u, err := url.Parse(strings.TrimRight(w.matchServiceURL, "/") + "/api/matches/internal/finished-jobs")
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	q := u.Query()
	q.Set("limit", "10")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if w.serviceToken != "" {
		req.Header.Set("X-Internal-Service-Token", w.serviceToken)
	}

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var body struct {
		MatchIDs []string `json:"matchIds"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode body: %w", err)
	}
	return body.MatchIDs, nil
}

func (w *Worker) processMatch(ctx context.Context, matchID string) error {
	snapshot, err := w.fetchMatchSnapshot(ctx, matchID)
	if err != nil {
		return err
	}

	whiteID := snapshot.Match.WhiteGuestID
	if whiteID == "" {
		whiteID = snapshot.Match.WhiteAccountID
	}
	blackID := snapshot.Match.BlackGuestID
	if blackID == "" {
		blackID = snapshot.Match.BlackAccountID
	}

	record := &anticheat.GameRecord{
		MatchID:    matchID,
		WhiteID:    whiteID,
		BlackID:    blackID,
		Mode:       string(snapshot.Match.ModeID),
		StartedAt:  snapshot.Match.CreatedAt,
		FinishedAt: snapshot.Match.UpdatedAt,
		Result:     snapshot.Match.Winner,
		Moves:      buildMoveRecords(snapshot.Match.MoveHistory),
	}

	if whiteID != "" {
		result := anticheat.AnalyzeGame(record, whiteID)
		log.Printf("analysis: match=%s winner=%s moves=%d suspicion=%.1f flags=%v",
			matchID, snapshot.Match.Winner, len(record.Moves), result.SuspicionScore, result.Flags)
	} else {
		log.Printf("analysis: match=%s winner=%s moves=%d (skipped: no whiteID)",
			matchID, snapshot.Match.Winner, len(record.Moves))
	}
	return nil
}

func (w *Worker) fetchMatchSnapshot(ctx context.Context, matchID string) (contracts.MatchSnapshotResponse, error) {
	u := strings.TrimRight(w.matchServiceURL, "/") + "/api/matches/" + matchID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return contracts.MatchSnapshotResponse{}, fmt.Errorf("build request: %w", err)
	}
	resp, err := w.httpClient.Do(req)
	if err != nil {
		return contracts.MatchSnapshotResponse{}, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return contracts.MatchSnapshotResponse{}, fmt.Errorf("get match %s: status %d", matchID, resp.StatusCode)
	}

	var snapshot contracts.MatchSnapshotResponse
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		return contracts.MatchSnapshotResponse{}, fmt.Errorf("decode body: %w", err)
	}
	return snapshot, nil
}

func buildMoveRecords(history []string) []anticheat.MoveRecord {
	moves := make([]anticheat.MoveRecord, 0, len(history))
	index := 0
	for i, raw := range history {
		notation := strings.TrimSpace(raw)
		if notation == "" {
			continue
		}
		color := "white"
		if index%2 == 1 {
			color = "black"
		}
		from := ""
		to := ""
		isCapture := false
		isCastle := ""
		if parsed, ok := match.ParseAlgebraicMove(notation); ok {
			from = squareLabel(parsed.From)
			to = squareLabel(parsed.To)
			isCapture = parsed.IsCapture
			isCastle = parsed.IsCastle
		}
		moves = append(moves, anticheat.MoveRecord{
			MoveNumber: index + 1,
			Color:      color,
			From:       from,
			To:         to,
			Timestamp:  time.Time{},
			ThinkTime:  0,
			CardsPlayed: nil,
		})
		_ = isCapture
		_ = isCastle
		_ = i
		index++
	}
	return moves
}

func squareLabel(square contracts.Square) string {
	files := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	ranks := []string{"1", "2", "3", "4", "5", "6", "7", "8"}
	if square.Row < 0 || square.Row > 7 || square.Col < 0 || square.Col > 7 {
		return ""
	}
	return files[square.Col] + ranks[square.Row]
}
