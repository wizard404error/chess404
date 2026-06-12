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
// the analysis result with the X-Chess404-Service-Token header (so the
// platform-service's internal-auth check passes) and the expected JSON
// body fields.
func TestPostAnalysisSendsBodyAndServiceToken(t *testing.T) {
	var (
		gotPath    string
		gotMethod  string
		gotToken   string
		gotCT      string
		gotBody    []byte
		callCount  int32
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
	w.postAnalysis(context.Background(), "match_1", "player_1", "open_cards", &anticheat.AnalysisResult{
		Accuracy:       95.0,
		AvgCPL:         10.0,
		MaxCPL:         50,
		MoveCount:      42,
		SuspicionScore: 88.0,
		Flags:          []string{"high_accuracy", "low_cpl"},
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
	if payload["suspicionScore"].(float64) != 88.0 {
		t.Fatalf("expected suspicionScore=88, got %v", payload["suspicionScore"])
	}
}

func TestPostAnalysisSkipsWhenPlatformURLMissing(t *testing.T) {
	w := &Worker{
		platformServiceURL: "",
		serviceToken:       "irrelevant",
		httpClient:         &http.Client{Timeout: 1 * time.Second},
	}
	// No assertion needed; we just verify no panic / no call.
	w.postAnalysis(context.Background(), "match_1", "player_1", "open_cards", &anticheat.AnalysisResult{
		SuspicionScore: 50,
	})
}
