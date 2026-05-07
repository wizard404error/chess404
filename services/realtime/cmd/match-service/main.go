package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chess404/realtime/internal/contracts"
	"github.com/chess404/realtime/internal/match"
	"github.com/chess404/realtime/internal/platform"
	"github.com/gorilla/websocket"
)

func main() {
	mux := http.NewServeMux()
	archive, err := openArchiveStore()
	if err != nil {
		log.Fatalf("failed to initialize archive store: %v", err)
	}
	defer func() { _ = archive.Close() }()
	service := match.NewServiceWithArchive(archive)
	upgrader := websocket.Upgrader{
		CheckOrigin: func(_ *http.Request) bool { return true },
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":       "ok",
			"service":      "match-service",
			"rulesVersion": "v1-alpha-foundation",
			"checkedAt":    nowUTC(),
		})
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":       "ok",
			"service":      "match-service",
			"rulesVersion": "v1-alpha-foundation",
			"checkedAt":    nowUTC(),
		})
	})

	mux.HandleFunc("/api/system/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":       "ok",
			"service":      "match-service",
			"rulesVersion": "v1-alpha-foundation",
			"checkedAt":    nowUTC(),
			"stats":        service.Stats(),
		})
	})

	mux.HandleFunc("/api/matches", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var req contracts.CreateMatchRequest
			if r.Body != nil {
				_ = json.NewDecoder(r.Body).Decode(&req)
			}
			resp := service.CreateMatch(req, nowUTC())
			writeJSON(w, http.StatusCreated, resp)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	mux.HandleFunc("/api/matches/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/matches/")
		if path == "" {
			writeError(w, http.StatusNotFound, "match id required")
			return
		}

		parts := strings.Split(path, "/")
		matchID := parts[0]

		if len(parts) == 1 && r.Method == http.MethodGet {
			resp, err := service.GetMatch(matchID)
			if err != nil {
				writeMatchError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, resp)
			return
		}

		if len(parts) == 2 && parts[1] == "intents" && r.Method == http.MethodPost {
			var req contracts.ApplyIntentRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid request body")
				return
			}
			req.Intent.MatchID = matchID
			resp, err := service.ApplyIntent(req.Intent, nowUTC())
			if err != nil {
				writeMatchError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, resp)
			return
		}

		if len(parts) == 2 && parts[1] == "ws" && r.Method == http.MethodGet {
			handleMatchSocket(w, r, service, &upgrader, matchID)
			return
		}

		writeError(w, http.StatusNotFound, "route not found")
	})

	addr := listenAddr("MATCH_SERVICE_ADDR", 8081)
	log.Printf("match-service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, withCORS(mux)))
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

func archivePath() string {
	if value := os.Getenv("MATCH_ARCHIVE_PATH"); value != "" {
		return value
	}
	return filepath.Join("data", "match-archive.json")
}

func archiveSQLitePath() string {
	if value := os.Getenv("MATCH_ARCHIVE_SQLITE_PATH"); value != "" {
		return value
	}
	return filepath.Join("data", "match-archive.sqlite")
}

func archivePostgresURL() string {
	return envOrDefault("MATCH_ARCHIVE_POSTGRES_URL", "postgres://postgres:postgres@127.0.0.1:5432/chess404?sslmode=disable")
}

func openArchiveStore() (*platform.MatchArchiveStore, error) {
	switch strings.ToLower(envOrDefault("MATCH_ARCHIVE_BACKEND", "file")) {
	case "sqlite":
		return platform.NewSQLiteMatchArchiveStore(archiveSQLitePath())
	case "postgres":
		return platform.NewPostgresMatchArchiveStore(archivePostgresURL())
	default:
		return platform.NewMatchArchiveStore(archivePath())
	}
}

func nowUTC() time.Time {
	return time.Now().UTC()
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"error": message,
	})
}

func writeMatchError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, match.ErrMatchNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}

func handleMatchSocket(w http.ResponseWriter, r *http.Request, service *match.Service, upgrader *websocket.Upgrader, matchID string) {
	stream, unsubscribe, initial, err := service.Subscribe(matchID)
	if err != nil {
		writeMatchError(w, err)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		unsubscribe()
		return
	}
	defer func() {
		unsubscribe()
		_ = conn.Close()
	}()

	if err := writeEnvelope(conn, "match.snapshot", initial); err != nil {
		return
	}

	pingTicker := time.NewTicker(20 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case snapshot, ok := <-stream:
			if !ok {
				return
			}
			if err := writeEnvelope(conn, "match.snapshot", snapshot); err != nil {
				return
			}
		case <-pingTicker.C:
			if err := conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(5*time.Second)); err != nil {
				return
			}
		}
	}
}

func writeEnvelope(conn *websocket.Conn, messageType string, payload any) error {
	_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return conn.WriteJSON(contracts.Envelope{
		Type:    messageType,
		Payload: payload,
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
