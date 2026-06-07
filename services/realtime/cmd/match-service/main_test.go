package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
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

// withCORSPreflightHeaders is the set of headers the browser sends in an Access-Control-Request-Headers
// preflight when calling a match endpoint with identity credentials.
var withCORSPreflightHeaders = []string{
	"X-Chess404-White-Guest-Id",
	"X-Chess404-White-Session-Token",
	"X-Chess404-White-Session-Secret",
	"X-Chess404-Black-Guest-Id",
	"X-Chess404-Black-Session-Token",
	"X-Chess404-Black-Session-Secret",
}

func TestWithCORSPreflightAllowsChess404Headers(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "https://web-production-9a697.up.railway.app")

	called := false
	handler := withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	for _, hdr := range withCORSPreflightHeaders {
		req := httptest.NewRequest(http.MethodOptions, "/api/matches/room_test", nil)
		req.Header.Set("Origin", "https://web-production-9a697.up.railway.app")
		req.Header.Set("Access-Control-Request-Method", "GET")
		req.Header.Set("Access-Control-Request-Headers", hdr)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("header %s: expected 204, got %d", hdr, rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://web-production-9a697.up.railway.app" {
			t.Fatalf("header %s: expected allowed origin to echo, got %q", hdr, got)
		}
		allow := rec.Header().Get("Access-Control-Allow-Headers")
		if !strings.Contains(strings.ToLower(allow), strings.ToLower(hdr)) {
			t.Fatalf("header %s: expected Allow-Headers to include %s, got %q", hdr, strings.ToLower(hdr), allow)
		}
	}
	if called {
		t.Fatalf("OPTIONS request must not invoke next handler")
	}
}

func TestWithCORSRejectsUnknownOriginWithoutAllowOrigin(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "https://web-production-9a697.up.railway.app")

	called := false
	handler := withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/matches/room_test", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatalf("CORS middleware does not block server-side; the browser is what enforces it. The next handler should still run.")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 (passthrough), got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no Allow-Origin for disallowed origin, got %q", got)
	}
	if rec.Header().Get("Vary") != "Origin" {
		t.Fatalf("expected Vary: Origin to be set")
	}
}

func TestWithCORSAcceptsEmptyAllowlist(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "")

	handler := withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/matches/room_test", nil)
	req.Header.Set("Origin", "https://anywhere.example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://anywhere.example.com" {
		t.Fatalf("empty allowlist should be permissive, got Allow-Origin=%q", got)
	}
}
