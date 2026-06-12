package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chess404/realtime/internal/anticheat"
)

// TestPostAnalysisSendsBodyAndServiceToken verifies the worker POSTs
// the Irwin engine-correlation result with the X-Chess404-Service-Token
// header (so the platform-service's internal-auth check passes) and
// the expected JSON body fields.
func TestPostAnalysisSendsBodyAndServiceToken(t *testing.T) {
	var (
		gotPath   string
		gotMethod string
		gotToken  string
		gotCT     string
		gotBody   []byte
		callCount int32
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotToken = r.Header.Get("X-Chess404-Service-Token")
		gotCT = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"analysisId":"ach_test"}`))
	}))
	defer srv.Close()

	w := &Worker{
		platformServiceURL: srv.URL,
		serviceToken:       "test_token_xyz",
		httpClient:         &http.Client{Timeout: 5 * time.Second},
	}
	w.postAnalysis(context.Background(), "match_1", "player_1", "open_cards", anticheat.Result{
		Top1Pct:        85.0,
		Top3Pct:        98.0,
		AvgRank:        1.2,
		OutsideTopN:    2,
		Top1Count:      17,
		Top3Count:      19,
		CardMoveCount:  0,
		EnginePositions: 20,
		EngineErrors:    0,
		EngineDepth:     20,
		EngineMultiPV:   3,
	})

	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/api/platform/anticheat/analyses" {
		t.Fatalf("expected path /api/platform/anticheat/analyses, got %s", gotPath)
	}
	if gotToken != "test_token_xyz" {
		t.Fatalf("expected service token forwarded, got %q", gotToken)
	}
	if !strings.HasPrefix(gotCT, "application/json") {
		t.Fatalf("expected content-type application/json, got %q", gotCT)
	}
	var payload map[string]any
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("decode body: %v body=%s", err, gotBody)
	}
	if payload["matchId"] != "match_1" {
		t.Fatalf("expected matchId, got %v", payload["matchId"])
	}
	if payload["playerId"] != "player_1" {
		t.Fatalf("expected playerId, got %v", payload["playerId"])
	}
	// Top1Pct=85, Top3Pct=98 -> weighted=0.7*85+0.3*98=59.5+29.4=88.9
	wantScore := 0.7*85.0 + 0.3*98.0
	if payload["suspicionScore"].(float64) < wantScore-0.01 || payload["suspicionScore"].(float64) > wantScore+0.01 {
		t.Fatalf("expected suspicionScore=%.1f, got %v", wantScore, payload["suspicionScore"])
	}
}

// TestPostAnalysisSkipsWhenPlatformURLMissing ensures no call is made
// when the platform-service URL is unconfigured.
func TestPostAnalysisSkipsWhenPlatformURLMissing(t *testing.T) {
	w := &Worker{
		platformServiceURL: "",
		serviceToken:       "irrelevant",
		httpClient:         &http.Client{Timeout: 1 * time.Second},
	}
	// No assertion needed; we just verify no panic / no call.
	w.postAnalysis(context.Background(), "match_1", "player_1", "open_cards", anticheat.Result{
		Top1Pct: 50,
	})
}

// TestRunIrwinProducesLogAndPostsResult verifies runIrwin calls the
// engine, logs the result, and posts to the platform-service. Uses a
// MockEngine so no Stockfish subprocess is needed.
func TestRunIrwinProducesLogAndPostsResult(t *testing.T) {
	var postCalled atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postCalled.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Engine that always returns the same top-3; the played move
	// "e2e4" will be top-1 for all positions.
	engine := anticheat.NewMockEngine([]anticheat.EngineMove{
		{Move: "e2e4", Rank: 1, ScoreCP: 30},
		{Move: "d2d4", Rank: 2, ScoreCP: 25},
		{Move: "g1f3", Rank: 3, ScoreCP: 20},
	})

	w := &Worker{
		platformServiceURL: srv.URL,
		serviceToken:       "test_token",
		httpClient:         &http.Client{Timeout: 5 * time.Second},
		engine:             engine,
		engineDepth:        20,
		engineMultiPV:      3,
	}

	samples := []anticheat.PositionSample{
		{FEN: "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w - - 0 1", PlayedMove: "e2e4"},
		{FEN: "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b - - 0 1", PlayedMove: "e7e5"},
		{FEN: "rnbqkbnr/pppp1ppp/8/4p3/4P3/8/PPPP1PPP/RNBQKBNR w - - 0 1", PlayedMove: "g1f3"},
	}
	w.runIrwin(context.Background(), "match_x", "player_1", "open_cards", samples)

	if !postCalled.Load() {
		t.Fatal("expected platform-service POST to be called")
	}
	// MockEngine should have been called once per position.
	if engine.CallCount != len(samples) {
		t.Fatalf("engine call count: want %d got %d", len(samples), engine.CallCount)
	}
}

// TestRunIrwinNoEngineSkips verifies the worker is a no-op (no panic,
// no post) when the engine isn't configured.
func TestRunIrwinNoEngineSkips(t *testing.T) {
	w := &Worker{
		platformServiceURL: "http://should.not.be.called",
		httpClient:         &http.Client{Timeout: 1 * time.Second},
		engine:             nil,
	}
	w.runIrwin(context.Background(), "match_x", "player_1", "open_cards", []anticheat.PositionSample{
		{FEN: "x", PlayedMove: "e2e4"},
	})
	// If we got here without panic, the test passes.
}
