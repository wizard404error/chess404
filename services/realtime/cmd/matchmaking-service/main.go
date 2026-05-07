package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

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

	mux.HandleFunc("/api/queues/tickets", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			queue := parseQueueName(r.URL.Query().Get("queue"))
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tickets": service.List(queue),
			})
		case http.MethodPost:
			var payload struct {
				Queue   string `json:"queue"`
				GuestID string `json:"guestId"`
				Rating  int    `json:"rating"`
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
			ticket, err := service.Enqueue(parseQueueName(payload.Queue), payload.GuestID, payload.Rating)
			if err != nil {
				http.Error(w, `{"error":"failed to persist queue ticket"}`, http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ticket":   ticket,
				"snapshot": service.Snapshot(ticket.Queue),
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
				"snapshot": service.Snapshot(ticket.Queue),
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
				"snapshot": service.Snapshot(ticket.Queue),
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

func openMatchmakingService() (*matchmaking.Service, error) {
	switch strings.ToLower(envOrDefault("MATCHMAKING_TICKET_STORE_BACKEND", "file")) {
	case "sqlite":
		return matchmaking.NewSQLitePersistentService(matchmakingTicketStoreSQLitePath())
	case "redis":
		return matchmaking.NewRedisPersistentService(matchmakingTicketStoreRedisURL(), matchmakingTicketStoreRedisKey())
	default:
		return matchmaking.NewPersistentService(envOrDefault("MATCHMAKING_TICKET_STORE_PATH", "data/matchmaking-tickets.json"))
	}
}

func parseQueueName(value string) matchmaking.QueueName {
	switch value {
	case string(matchmaking.QueueCasual):
		return matchmaking.QueueCasual
	default:
		return matchmaking.QueueRated
	}
}
