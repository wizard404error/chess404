package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/chess404/realtime/internal/platform"
)

func newAnticheatMuxForTest(t *testing.T) (*http.ServeMux, platform.AnticheatStore) {
	t.Setenv("INTERNAL_SERVICE_TOKEN", "test_internal_token")
	t.Setenv("PLATFORM_INTERNAL_SERVICE_TOKEN", "test_internal_token")
	t.Setenv("CHESS404_INTERNAL_SERVICE_TOKEN", "test_internal_token")
	t.Setenv("ACCOUNT_SECURITY_AUDIT_BACKEND", "file")
	t.Setenv("MATCH_CLAIM_STORE_BACKEND", "memory")
	store := platform.NewInMemoryAnticheatStore()
	mux := http.NewServeMux()
	registerAnticheatRoutes(mux, store)
	return mux, store
}

func anticheatAuthedRequest(t *testing.T, mux http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		rdr = bytes.NewReader(buf)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("X-Chess404-Service-Token", "test_internal_token")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestAnticheatRouteRejectsMissingToken(t *testing.T) {
	mux, _ := newAnticheatMuxForTest(t)
	_ = os.Setenv("INTERNAL_SERVICE_TOKEN", "test_internal_token")
	defer os.Unsetenv("INTERNAL_SERVICE_TOKEN")

	req := httptest.NewRequest(http.MethodPost, "/api/platform/anticheat/analyses", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without service token, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAnticheatRouteRecordsAnalysisAndReachesThreshold(t *testing.T) {
	mux, store := newAnticheatMuxForTest(t)

	// Post 5 analyses with high suspicion. The rolling average should
	// climb past 80 and trigger the auto-action on the last one.
	for i := 0; i < 5; i++ {
		rec := anticheatAuthedRequest(t, mux, http.MethodPost, "/api/platform/anticheat/analyses", map[string]any{
			"matchId":        "match_" + string(rune('a'+i)),
			"playerId":       "player_evil",
			"modeId":         "open_cards",
			"accuracy":       95.0,
			"avgCpl":         10.0,
			"maxCpl":         40,
			"moveCount":      42,
			"cardMoves":      1,
			"flags":          []string{"high_accuracy", "low_cpl"},
			"suspicionScore": 95.0,
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("post #%d: expected 200, got %d body=%s", i, rec.Code, rec.Body.String())
		}
	}

	resp := anticheatAuthedRequest(t, mux, http.MethodGet, "/api/platform/anticheat/players?minScore=80", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 listing, got %d", resp.Code)
	}
	var list anticheatFlaggedPlayerListResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v body=%s", err, resp.Body.String())
	}
	if len(list.Players) != 1 || list.Players[0].PlayerID != "player_evil" {
		t.Fatalf("expected 1 flagged player, got %+v", list.Players)
	}
	if list.Players[0].ActionTaken != platform.AnticheatActionWarning {
		t.Fatalf("expected action_taken=warning_issued, got %q", list.Players[0].ActionTaken)
	}

	// Per-player detail endpoint
	detail := anticheatAuthedRequest(t, mux, http.MethodGet, "/api/platform/anticheat/players/player_evil", nil)
	if detail.Code != http.StatusOK {
		t.Fatalf("detail: expected 200, got %d body=%s", detail.Code, detail.Body.String())
	}
	var det anticheatPlayerDetailResponse
	if err := json.Unmarshal(detail.Body.Bytes(), &det); err != nil {
		t.Fatalf("decode detail: %v body=%s", err, detail.Body.String())
	}
	if len(det.Analyses) != 5 {
		t.Fatalf("expected 5 analyses, got %d", len(det.Analyses))
	}

	// Manual action endpoint
	action := anticheatAuthedRequest(t, mux, http.MethodPost, "/api/platform/anticheat/players/player_evil/action", map[string]any{
		"action": platform.AnticheatActionRatingReset,
		"detail": "verified cheating by human review",
	})
	if action.Code != http.StatusOK {
		t.Fatalf("action: expected 200, got %d body=%s", action.Code, action.Body.String())
	}
	summary, _ := store.GetPlayerSummary("player_evil")
	if summary.ActionTaken != platform.AnticheatActionRatingReset {
		t.Fatalf("expected action_taken=rating_reset, got %q", summary.ActionTaken)
	}
}

func TestAnticheatRouteRejectsInvalidPayload(t *testing.T) {
	mux, _ := newAnticheatMuxForTest(t)
	rec := anticheatAuthedRequest(t, mux, http.MethodPost, "/api/platform/anticheat/analyses", map[string]any{
		"playerId": "p1",
		// matchId missing
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing matchId, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAnticheatRoutePlayerNotFound(t *testing.T) {
	mux, _ := newAnticheatMuxForTest(t)
	rec := anticheatAuthedRequest(t, mux, http.MethodPost, "/api/platform/anticheat/players/ghost/action", map[string]any{
		"action": "warning_issued",
	})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing player, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// Sanity check: a brand new player with no analyses should appear in
// the detail endpoint with zero-count fields rather than 404.
func TestAnticheatRoutePlayerDetailMissing(t *testing.T) {
	mux, _ := newAnticheatMuxForTest(t)
	rec := anticheatAuthedRequest(t, mux, http.MethodGet, "/api/platform/anticheat/players/never_seen", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with empty summary, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"playerId":"never_seen"`) {
		t.Fatalf("expected playerId in body, got %s", rec.Body.String())
	}
}
