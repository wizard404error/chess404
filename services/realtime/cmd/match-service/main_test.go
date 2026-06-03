package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/chess404/realtime/internal/contracts"
	"github.com/chess404/realtime/internal/platform"
)

func TestFinalizingArchiveStoreCallsPlatformForFinishedRatedMatch(t *testing.T) {
	tempDir := t.TempDir()
	archive, err := platform.NewMatchArchiveStore(filepath.Join(tempDir, "archive.json"))
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}
	defer func() { _ = archive.Close() }()

	called := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/platform/internal/finalize-rated-match" {
			t.Errorf("unexpected finalizer path %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer service-secret" {
			t.Errorf("expected service token authorization, got %q", got)
		}
		var payload struct {
			MatchID string `json:"matchId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("expected finalizer payload to decode, got %v", err)
		}
		called <- payload.MatchID
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"changed":true}`))
	}))
	defer server.Close()

	store := &finalizingArchiveStore{
		archive:      archive,
		platformURL:  server.URL,
		serviceToken: "service-secret",
		client:       server.Client(),
		inFlight:     make(map[string]struct{}),
		done:         make(map[string]struct{}),
	}
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	if err := store.Upsert(contracts.MatchSnapshotResponse{
		Match: contracts.MatchState{
			MatchID:      "rated_finish",
			RulesVersion: "v1-alpha-foundation",
			Queue:        "rated",
			Status:       "finished",
			Winner:       "white",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}); err != nil {
		t.Fatalf("expected archive upsert to succeed, got %v", err)
	}

	select {
	case matchID := <-called:
		if matchID != "rated_finish" {
			t.Fatalf("expected finalizer to receive match id, got %q", matchID)
		}
	case <-time.After(time.Second):
		t.Fatalf("expected platform finalizer to be called")
	}
}
