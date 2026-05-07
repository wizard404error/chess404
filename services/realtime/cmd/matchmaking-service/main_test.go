package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/chess404/realtime/internal/matchmaking"
)

func TestMatchmakingStatsExposeSQLiteBackend(t *testing.T) {
	service, err := matchmaking.NewSQLitePersistentService(filepath.Join(t.TempDir(), "tickets.sqlite"))
	if err != nil {
		t.Fatalf("expected sqlite matchmaking service to initialize, got %v", err)
	}
	defer func() { _ = service.Close() }()

	if _, err := service.Enqueue(matchmaking.QueueRated, "guest_a", 1200); err != nil {
		t.Fatalf("expected ticket enqueue to succeed, got %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":    "ok",
			"service":   "matchmaking-service",
			"checkedAt": "2026-05-06T00:00:00Z",
			"stats":     service.Stats(),
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected matchmaking status to succeed, got status %d body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		Stats struct {
			Backend string `json:"backend"`
		} `json:"stats"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected matchmaking status response to decode, got %v", err)
	}
	if response.Stats.Backend != "sqlite" {
		t.Fatalf("expected sqlite backend in matchmaking status, got %#v", response)
	}
}
