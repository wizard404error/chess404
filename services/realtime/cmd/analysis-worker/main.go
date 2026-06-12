package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	platformServiceURL := os.Getenv("PLATFORM_SERVICE_INTERNAL_URL")
	if platformServiceURL == "" {
		log.Println("WARN: PLATFORM_SERVICE_INTERNAL_URL not set, anticheat results will only be logged")
	}

	// Try to launch Stockfish. If it fails (binary missing, broken
	// install, etc.) we fall back to "log only" mode: the worker
	// still polls for finished matches and logs the basic metadata
	// but doesn't run Irwin analysis. Better than refusing to
	// start at all.
	var engine anticheat.Engine
	engineDepth := envInt("ANTICHEAT_ENGINE_DEPTH", 20)
	engineMultiPV := envInt("ANTICHEAT_ENGINE_MULTIPV", 3)
	stockfishPath := strings.TrimSpace(os.Getenv("STOCKFISH_PATH"))
	if stockfishPath == "" {
		stockfishPath = "stockfish" // PATH lookup
	}
	sf, sfErr := anticheat.NewStockfishEngine(anticheat.StockfishConfig{
		Binary: stockfishPath,
	})
	if sfErr != nil {
		log.Printf("WARN: Stockfish unavailable (%v); anticheat analysis disabled, worker will still log match metadata", sfErr)
	} else {
		engine = sf
		log.Printf("Stockfish engine ready (depth=%d, multipv=%d)", engineDepth, engineMultiPV)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		if engine != nil {
			_ = engine.Close()
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Shutting down...")
		cancel()
	}()

	worker := &Worker{
		matchServiceURL:    matchServiceURL,
		platformServiceURL: platformServiceURL,
		serviceToken:       serviceToken,
		httpClient:         &http.Client{Timeout: 10 * time.Second},
		engine:             engine,
		engineDepth:        engineDepth,
		engineMultiPV:      engineMultiPV,
	}
	log.Println("Worker ready, polling for jobs...")
	worker.Run(ctx)
}

// envInt reads an integer env var, falling back to the default if
// unset or unparseable.
func envInt(name string, def int) int {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}
	var n int
	for _, c := range v {
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int(c-'0')
	}
	return n
}

type Worker struct {
	matchServiceURL    string
	platformServiceURL string
	serviceToken       string
	httpClient         *http.Client
	analyzed           map[string]struct{}
	// engine is the chess engine used for Irwin analysis. In
	// production this is a StockfishEngine; in tests it's a
	// MockEngine. nil means the worker runs in "log only" mode
	// (no engine analysis, no platform-service post).
	engine        anticheat.Engine
	engineDepth   int
	engineMultiPV int
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

	if whiteID == "" && blackID == "" {
		log.Printf("analysis: match=%s winner=%s (skipped: no player ids)",
			matchID, snapshot.Match.Winner)
		return nil
	}

	// Replay the game to produce position samples (FEN + UCI move
	// for each ply). All moves are currently treated as non-card;
	// card-move exclusion is a Phase 2 refinement once the
	// match-service exposes card-move metadata in the snapshot.
	samples := anticheat.ReplayGame(snapshot.Match.MoveHistory, parseMoveNotation)
	if len(samples) == 0 {
		log.Printf("analysis: match=%s (no parseable moves)", matchID)
		return nil
	}

	// Run Irwin analysis once per player; the engine produces the
	// same per-position top-N for both players, so we could share
	// the engine calls. The optimization is small (a few ms per
	// game at depth 20) and not worth the complexity now.
	for _, entry := range []struct {
		playerID string
		color    string
	}{
		{whiteID, "white"},
		{blackID, "black"},
	} {
		if entry.playerID == "" {
			continue
		}
		w.runIrwin(ctx, matchID, entry.playerID, string(snapshot.Match.ModeID), samples)
	}
	return nil
}

// runIrwin runs the Irwin engine-correlation analysis on the given
// samples and posts the result to the platform-service. The engine
// is the worker-wide shared StockfishEngine (or a MockEngine in
// tests). Engine errors are logged but do not stop the per-player
// analysis: a hiccup on one position shouldn't drop the player's
// entire report.
func (w *Worker) runIrwin(ctx context.Context, matchID, playerID, modeID string, samples []anticheat.PositionSample) {
	if w.engine == nil {
		log.Printf("analysis: match=%s player=%s skipped: no engine configured", matchID, playerID)
		return
	}
	result, err := anticheat.AnalyzeIrwin(ctx, w.engine, samples, w.engineDepth, w.engineMultiPV)
	if err != nil {
		log.Printf("analysis: match=%s player=%s irwin failed: %v", matchID, playerID, err)
		return
	}
	log.Printf("analysis: match=%s player=%s %s", matchID, playerID, result.String())
	w.postAnalysis(ctx, matchID, playerID, modeID, result)
}

// postAnalysis sends the Irwin result to the platform-service. If
// PLATFORM_SERVICE_INTERNAL_URL is not set, the function returns
// immediately; the analysis was already logged by the caller, so
// nothing is lost.
func (w *Worker) postAnalysis(ctx context.Context, matchID, playerID, modeID string, result anticheat.Result) {
	if strings.TrimSpace(w.platformServiceURL) == "" {
		return
	}
	u := strings.TrimRight(w.platformServiceURL, "/") + "/api/platform/anticheat/analyses"
	body, err := json.Marshal(map[string]any{
		"matchId":         matchID,
		"playerId":        playerID,
		"modeId":          modeID,
		"top1Pct":         result.Top1Pct,
		"top3Pct":         result.Top3Pct,
		"avgRank":         result.AvgRank,
		"outsideTopN":     result.OutsideTopN,
		"top1Count":       result.Top1Count,
		"top3Count":       result.Top3Count,
		"cardMoveCount":   result.CardMoveCount,
		"enginePositions": result.EnginePositions,
		"engineErrors":    result.EngineErrors,
		"engineDepth":     result.EngineDepth,
		"engineMultiPV":   result.EngineMultiPV,
		"suspicionScore":  result.Score(),
	})
	if err != nil {
		log.Printf("analysis: failed to marshal body for match=%s player=%s: %v", matchID, playerID, err)
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		log.Printf("analysis: failed to build request for match=%s player=%s: %v", matchID, playerID, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if w.serviceToken != "" {
		req.Header.Set("X-Chess404-Service-Token", w.serviceToken)
	}
	resp, err := w.httpClient.Do(req)
	if err != nil {
		log.Printf("analysis: POST to platform-service failed for match=%s player=%s: %v", matchID, playerID, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		log.Printf("analysis: platform-service returned %d for match=%s player=%s body=%s", resp.StatusCode, matchID, playerID, string(body))
		return
	}
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

// parseMoveNotation converts PGN-style algebraic notation to UCI form
// (e.g., "e4" -> "e2e4", "Nf3" -> "g1f3", "Bxc4" -> "f1c4"). It uses
// the in-house engine's parser and then concatenates from+to. If
// the notation can't be parsed, returns "" + false so the caller
// skips the position (rather than crashing).
func parseMoveNotation(notation string) (string, bool) {
	parsed, ok := match.ParseAlgebraicMove(notation)
	if !ok {
		return "", false
	}
	from := squareLabel(parsed.From)
	to := squareLabel(parsed.To)
	if from == "" || to == "" {
		return "", false
	}
	// For promotions, append the promotion piece (e.g., "e7e8q").
	if parsed.PromotionPiece != "" {
		promotion := strings.ToLower(parsed.PromotionPiece)
		if promotion != "q" && promotion != "r" && promotion != "b" && promotion != "n" {
			promotion = "q" // default to queen on unknown piece
		}
		return from + to + promotion, true
	}
	return from + to, true
}
