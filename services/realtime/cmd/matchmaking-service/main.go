package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/chess404/realtime/internal/contracts"
	"github.com/chess404/realtime/internal/matchmaking"
)

func main() {
	mux := http.NewServeMux()
	service, err := openMatchmakingService()
	if err != nil {
		log.Fatalf("failed to initialize matchmaking service: %v", err)
	}
	defer func() { _ = service.Close() }()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":    "ok",
			"service":   "matchmaking-service",
			"checkedAt": time.Now().UTC(),
			"stats":     service.Stats(),
		})
	})

	mux.HandleFunc("/api/queues/default", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"queue":        "rated",
			"status":       "open",
			"architecture": "region-aware-authoritative",
		})
	})

	mux.HandleFunc("/api/queues/snapshots", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		queueFilter := parseOptionalQueueName(r.URL.Query().Get("queue"))
		modeFilter := parseOptionalModeID(r.URL.Query().Get("modeId"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"snapshots": queueSnapshots(service, queueFilter, modeFilter),
			"checkedAt": time.Now().UTC(),
		})
	})

	mux.HandleFunc("/api/queues/tickets", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			queue := parseQueueName(r.URL.Query().Get("queue"))
			modeID := parseModeID(r.URL.Query().Get("modeId"))
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tickets": service.List(queue, modeID),
			})
		case http.MethodPost:
			var payload struct {
				Queue       string `json:"queue"`
				ModeID      string `json:"modeId"`
				GuestID     string `json:"guestId"`
				AccountID   string `json:"accountId"`
				DisplayName string `json:"displayName"`
				Rating      int    `json:"rating"`
			}
			if r.Body != nil {
				defer r.Body.Close()
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					http.Error(w, `{"error":"invalid queue payload"}`, http.StatusBadRequest)
					return
				}
			}
			if payload.GuestID == "" {
				http.Error(w, `{"error":"guestId is required"}`, http.StatusBadRequest)
				return
			}
			if payload.Rating <= 0 {
				payload.Rating = 1200
			}
			modeID := parseModeID(payload.ModeID)
			ticket, err := service.EnqueueWithAccount(
				parseQueueName(payload.Queue),
				modeID,
				payload.GuestID,
				payload.Rating,
				payload.DisplayName,
				strings.TrimSpace(payload.AccountID),
			)
			if err != nil {
				var activeErr matchmaking.ActiveTicketError
				if errors.As(err, &activeErr) {
					w.Header().Set("Content-Type", "application/json")
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
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ticket":   ticket,
				"snapshot": service.Snapshot(ticket.Queue, ticket.ModeID),
			})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/queues/tickets/", func(w http.ResponseWriter, r *http.Request) {
		ticketID := strings.TrimPrefix(r.URL.Path, "/api/queues/tickets/")
		if ticketID == "" {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			ticket, ok := service.Get(ticketID)
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ticket":   ticket,
				"snapshot": service.Snapshot(ticket.Queue, ticket.ModeID),
			})
		case http.MethodDelete:
			ticket, ok, err := service.Cancel(ticketID)
			if !ok {
				http.NotFound(w, r)
				return
			}
			if err != nil {
				http.Error(w, `{"error":"failed to persist queue cancellation"}`, http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ticket":   ticket,
				"snapshot": service.Snapshot(ticket.Queue, ticket.ModeID),
			})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	addr := listenAddr("MATCHMAKING_ADDR", 8084)
	log.Printf("matchmaking-service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func listenAddr(key string, fallbackPort int) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	if value := os.Getenv("PORT"); value != "" {
		if strings.HasPrefix(value, ":") {
			return value
		}
		return ":" + value
	}
	return fmt.Sprintf(":%d", fallbackPort)
}

func matchmakingTicketStoreSQLitePath() string {
	return envOrDefault("MATCHMAKING_TICKET_STORE_SQLITE_PATH", "data/matchmaking-tickets.sqlite")
}

func matchmakingTicketStoreRedisURL() string {
	return envOrDefault("MATCHMAKING_TICKET_STORE_REDIS_URL", "redis://127.0.0.1:6379/0")
}

func matchmakingTicketStoreRedisKey() string {
	return envOrDefault("MATCHMAKING_TICKET_STORE_REDIS_KEY", "chess404:matchmaking:tickets")
}

func matchmakingMatchServiceURL() string {
	return strings.TrimRight(envOrDefault("MATCH_SERVICE_INTERNAL_URL", "http://127.0.0.1:8082"), "/")
}

func openMatchmakingService() (*matchmaking.Service, error) {
	serviceCreator := &httpMatchCreator{
		baseURL: matchmakingMatchServiceURL(),
		client:  &http.Client{Timeout: 3 * time.Second},
	}

	var (
		service *matchmaking.Service
		err     error
	)
	switch strings.ToLower(envOrDefault("MATCHMAKING_TICKET_STORE_BACKEND", "file")) {
	case "sqlite":
		service, err = matchmaking.NewSQLitePersistentService(matchmakingTicketStoreSQLitePath())
	case "redis":
		service, err = matchmaking.NewRedisPersistentService(matchmakingTicketStoreRedisURL(), matchmakingTicketStoreRedisKey())
	default:
		service, err = matchmaking.NewPersistentService(envOrDefault("MATCHMAKING_TICKET_STORE_PATH", "data/matchmaking-tickets.json"))
	}
	if err != nil {
		return nil, err
	}
	service.SetMatchCreator(serviceCreator)
	return service, nil
}

func parseQueueName(value string) matchmaking.QueueName {
	switch value {
	case string(matchmaking.QueueCasual):
		return matchmaking.QueueCasual
	default:
		return matchmaking.QueueRated
	}
}

func parseModeID(value string) contracts.MatchModeID {
	return contracts.NormalizeMatchModeID(value)
}

func parseOptionalQueueName(value string) matchmaking.QueueName {
	switch strings.TrimSpace(value) {
	case "":
		return ""
	default:
		return parseQueueName(value)
	}
}

func parseOptionalModeID(value string) contracts.MatchModeID {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return contracts.NormalizeMatchModeID(value)
}

func queueSnapshots(service *matchmaking.Service, queueFilter matchmaking.QueueName, modeFilter contracts.MatchModeID) []matchmaking.QueueSnapshot {
	queues := []matchmaking.QueueName{matchmaking.QueueCasual, matchmaking.QueueRated}
	if queueFilter != "" {
		queues = []matchmaking.QueueName{queueFilter}
	}

	modes := []contracts.MatchModeID{contracts.MatchModeOpenCards, contracts.MatchModeHiddenCards}
	if modeFilter != "" {
		modes = []contracts.MatchModeID{modeFilter}
	}

	snapshots := make([]matchmaking.QueueSnapshot, 0, len(queues)*len(modes))
	for _, queue := range queues {
		for _, modeID := range modes {
			snapshots = append(snapshots, service.Snapshot(queue, modeID))
		}
	}
	return snapshots
}

type httpMatchCreator struct {
	baseURL string
	client  *http.Client
}

func (c *httpMatchCreator) CreateMatch(assignment matchmaking.MatchAssignment) error {
	if c == nil || strings.TrimSpace(c.baseURL) == "" {
		return fmt.Errorf("match service URL is not configured")
	}
	client := c.client
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}

	payload := map[string]any{
		"matchId":           assignment.RoomID,
		"clockSeconds":      600,
		"starterHandMode":   "starter_three",
		"queue":             string(assignment.Queue),
		"modeId":            string(assignment.ModeID),
		"whiteGuestId":      assignment.WhiteGuestID,
		"blackGuestId":      assignment.BlackGuestID,
		"whiteAccountId":    assignment.WhiteAccountID,
		"blackAccountId":    assignment.BlackAccountID,
		"whiteName":         assignment.WhiteName,
		"blackName":         assignment.BlackName,
		"whitePlayerSecret": assignment.WhitePlayerSecret,
		"blackPlayerSecret": assignment.BlackPlayerSecret,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/matches", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("match service create failed with status %d", resp.StatusCode)
	}
	return nil
}
