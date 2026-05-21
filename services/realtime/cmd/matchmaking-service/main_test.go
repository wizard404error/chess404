package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chess404/realtime/internal/contracts"
	"github.com/chess404/realtime/internal/matchmaking"
)

func TestResolveInternalServiceURLAddsRailwayPortFallback(t *testing.T) {
	resolved := resolveInternalServiceURL("http://match-service.railway.internal", "http://match-service.railway.internal:8080")
	if resolved != "http://match-service.railway.internal:8080" {
		t.Fatalf("expected railway internal host to gain :8080, got %q", resolved)
	}
}

func TestMatchmakingStatsExposeSQLiteBackend(t *testing.T) {
	service, err := matchmaking.NewSQLitePersistentService(filepath.Join(t.TempDir(), "tickets.sqlite"))
	if err != nil {
		t.Fatalf("expected sqlite matchmaking service to initialize, got %v", err)
	}
	defer func() { _ = service.Close() }()

	if _, err := service.Enqueue(matchmaking.QueueRated, contracts.MatchModeOpenCards, "guest_a", 1200, "Alpha"); err != nil {
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

func TestMatchmakingRejectsSecondActiveQueueAcrossQueues(t *testing.T) {
	service := matchmaking.NewService()
	if _, err := service.Enqueue(matchmaking.QueueCasual, contracts.MatchModeOpenCards, "guest_a", 1200, "Alpha"); err != nil {
		t.Fatalf("seed casual ticket: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/queues/tickets", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var payload struct {
			Queue       string `json:"queue"`
			ModeID      string `json:"modeId"`
			GuestID     string `json:"guestId"`
			DisplayName string `json:"displayName"`
			Rating      int    `json:"rating"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, `{"error":"invalid queue payload"}`, http.StatusBadRequest)
			return
		}

		ticket, err := service.Enqueue(parseQueueName(payload.Queue), parseModeID(payload.ModeID), payload.GuestID, payload.Rating, payload.DisplayName)
		if err != nil {
			var activeErr matchmaking.ActiveTicketError
			if errors.As(err, &activeErr) {
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error":    err.Error(),
					"ticket":   activeErr.Ticket,
					"snapshot": service.Snapshot(activeErr.Ticket.Queue, activeErr.Ticket.ModeID),
				})
				return
			}
			http.Error(w, `{"error":"failed to persist queue ticket"}`, http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ticket":   ticket,
			"snapshot": service.Snapshot(ticket.Queue, ticket.ModeID),
		})
	})

	body := `{"guestId":"guest_a","queue":"rated","modeId":"open_cards","rating":1200,"displayName":"Alpha"}`
	req := httptest.NewRequest(http.MethodPost, "/api/queues/tickets", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected duplicate active queue join to return 409, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMatchmakingListEndpointFiltersByMode(t *testing.T) {
	service := matchmaking.NewService()
	if _, err := service.Enqueue(matchmaking.QueueRated, contracts.MatchModeOpenCards, "guest_a", 1200, "Alpha"); err != nil {
		t.Fatalf("seed open-cards ticket: %v", err)
	}
	if _, err := service.Enqueue(matchmaking.QueueRated, contracts.MatchModeHiddenCards, "guest_b", 1210, "Bravo"); err != nil {
		t.Fatalf("seed hidden-cards ticket: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/queues/tickets", func(w http.ResponseWriter, r *http.Request) {
		queue := parseQueueName(r.URL.Query().Get("queue"))
		modeID := parseModeID(r.URL.Query().Get("modeId"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tickets": service.List(queue, modeID),
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/queues/tickets?queue=rated&modeId=hidden_cards", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected filtered ticket list to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		Tickets []matchmaking.Ticket `json:"tickets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Tickets) != 1 || response.Tickets[0].ModeID != contracts.MatchModeHiddenCards {
		t.Fatalf("expected only hidden-cards tickets in filtered response, got %#v", response.Tickets)
	}
}

func TestMatchmakingSnapshotsEndpointReturnsQueueModeMatrix(t *testing.T) {
	service := matchmaking.NewService()
	if _, err := service.Enqueue(matchmaking.QueueCasual, contracts.MatchModeOpenCards, "guest_a", 1200, "Alpha"); err != nil {
		t.Fatalf("seed casual open-cards ticket: %v", err)
	}
	if _, err := service.Enqueue(matchmaking.QueueRated, contracts.MatchModeHiddenCards, "guest_b", 1210, "Bravo"); err != nil {
		t.Fatalf("seed rated hidden-cards ticket: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/queues/snapshots", func(w http.ResponseWriter, r *http.Request) {
		queueFilter := parseOptionalQueueName(r.URL.Query().Get("queue"))
		modeFilter := parseOptionalModeID(r.URL.Query().Get("modeId"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"snapshots": queueSnapshots(service, queueFilter, modeFilter),
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/queues/snapshots", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected queue snapshots to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		Snapshots []matchmaking.QueueSnapshot `json:"snapshots"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Snapshots) != 4 {
		t.Fatalf("expected full queue/mode matrix, got %#v", response.Snapshots)
	}

	var casualOpen, ratedHidden *matchmaking.QueueSnapshot
	for i := range response.Snapshots {
		snapshot := &response.Snapshots[i]
		if snapshot.Queue == matchmaking.QueueCasual && snapshot.ModeID == contracts.MatchModeOpenCards {
			casualOpen = snapshot
		}
		if snapshot.Queue == matchmaking.QueueRated && snapshot.ModeID == contracts.MatchModeHiddenCards {
			ratedHidden = snapshot
		}
	}
	if casualOpen == nil || casualOpen.QueuedCount != 1 {
		t.Fatalf("expected casual open-cards queued count, got %#v", response.Snapshots)
	}
	if ratedHidden == nil || ratedHidden.QueuedCount != 1 {
		t.Fatalf("expected rated hidden-cards queued count, got %#v", response.Snapshots)
	}
}
