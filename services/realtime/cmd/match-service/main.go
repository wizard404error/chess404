package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/chess404/realtime/internal/contracts"
	"github.com/chess404/realtime/internal/envutil"
	"github.com/chess404/realtime/internal/httputil"
	"github.com/chess404/realtime/internal/match"
	"github.com/chess404/realtime/internal/metrics"
	"github.com/chess404/realtime/internal/platform"
	"github.com/chess404/realtime/internal/rate_limit"
	"github.com/gorilla/websocket"
)

func main() {
	envutil.Require("PLATFORM_SERVICE_INTERNAL_URL", "ALLOWED_ORIGINS", "INTERNAL_SERVICE_TOKEN")
	mux := http.NewServeMux()
	archive, err := openArchiveStore()
	if err != nil {
		log.Fatalf("failed to initialize archive store: %v", err)
	}
	defer func() { _ = archive.Close() }()

	store, broadcaster := openMatchStore()
	service := match.NewServiceWithStoreAndBroadcaster(newFinalizingArchiveStore(archive), store, broadcaster)
	rl := rate_limit.New()
	allowed := httputil.ParseAllowedOrigins()
	upgrader := websocket.Upgrader{
		ReadBufferSize:    4096,
		WriteBufferSize:   4096,
		EnableCompression: false,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				log.Printf("match:ws: rejecting upgrade for matchID=%s: missing Origin header (allowed=%v)", r.URL.Path, allowed)
				return false
			}
			if !httputil.IsOriginAllowed(origin, allowed) {
				log.Printf("match:ws: rejecting upgrade for matchID=%s: origin=%q not in allowed list %v", r.URL.Path, origin, allowed)
				return false
			}
			return true
		},
	}

	// Hard cap on per-message size to prevent a single client from amplifying
	// memory pressure (gigantic payloads, slow-loris, or accidental floods).
	// 64 KiB is more than enough for any intent we accept (typical intents are
	// well under 1 KiB). Excess closes the connection.
	const wsReadLimit int64 = 64 * 1024

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]any{
			"status":       "ok",
			"service":      "match-service",
			"rulesVersion": "v1-alpha-foundation",
			"checkedAt":    httputil.NowUTC(),
		})
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		// Basic backend health check: ping the database if supported
		if pingable, ok := any(archive).(interface{ Ping() error }); ok {
			if err := pingable.Ping(); err != nil {
				httputil.WriteError(w, http.StatusServiceUnavailable, "database unavailable")
				return
			}
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]any{
			"status":       "ok",
			"service":      "match-service",
			"rulesVersion": "v1-alpha-foundation",
			"checkedAt":    httputil.NowUTC(),
		})
	})

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if pingable, ok := any(archive).(interface{ Ping() error }); ok {
			if err := pingable.Ping(); err != nil {
				httputil.WriteError(w, http.StatusServiceUnavailable, "database unavailable")
				return
			}
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})

	mux.HandleFunc("/livez", func(w http.ResponseWriter, _ *http.Request) {
		httputil.WriteJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})

	mux.Handle("/metrics", metrics.Handler())

	mux.HandleFunc("/api/system/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			httputil.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]any{
			"status":       "ok",
			"service":      "match-service",
			"rulesVersion": "v1-alpha-foundation",
			"checkedAt":    httputil.NowUTC(),
			"stats":        service.Stats(),
		})
	})

	mux.HandleFunc("/api/matches/internal/finished-jobs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			httputil.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if r.Header.Get("X-Internal-Service-Token") == "" || r.Header.Get("X-Internal-Service-Token") != internalServiceToken() {
			httputil.WriteError(w, http.StatusUnauthorized, "internal service token required")
			return
		}
		if internalServiceToken() == "" {
			httputil.WriteError(w, http.StatusServiceUnavailable, "internal service token not configured on server")
			return
		}
		limit := platform.ParseListLimit(r.URL.Query().Get("limit"), 10)
		ids := archive.ListFinishedMatchIDs(limit)
		httputil.WriteJSON(w, http.StatusOK, map[string]any{"matchIds": ids})
	})

	mux.HandleFunc("/api/matches", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var req contracts.CreateMatchRequest
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
					return
				}
			}
			resp := service.CreateMatch(req, httputil.NowUTC())
			httputil.WriteJSON(w, http.StatusCreated, resp)
		default:
			httputil.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	mux.HandleFunc("/api/matches/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/matches/")
		if path == "" {
			httputil.WriteError(w, http.StatusNotFound, "match id required")
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
			httputil.WriteJSON(w, http.StatusOK, resp)
			return
		}

		if len(parts) == 2 && parts[1] == "join" && r.Method == http.MethodPost {
			var req contracts.JoinMatchSeatRequest
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
				return
			}
			resp, err := service.JoinMatchSeat(matchID, req, httputil.NowUTC())
			if err != nil {
				writeMatchError(w, err)
				return
			}
			httputil.WriteJSON(w, http.StatusOK, resp)
			return
		}

		if len(parts) == 2 && parts[1] == "intents" && r.Method == http.MethodPost {
			var req contracts.ApplyIntentRequest
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
				return
			}
			req.Intent.MatchID = matchID
			resp, err := service.ApplyIntent(req.Intent, httputil.NowUTC())
			if err != nil {
				writeMatchError(w, err)
				return
			}
			httputil.WriteJSON(w, http.StatusOK, resp)
			return
		}

		if len(parts) == 2 && parts[1] == "token" && (r.Method == http.MethodPost || r.Method == http.MethodGet) {
			playerID := strings.TrimSpace(r.URL.Query().Get("i"))
			playerSecret := strings.TrimSpace(r.URL.Query().Get("s"))
			if playerID == "" {
				httputil.WriteError(w, http.StatusBadRequest, "playerId is required")
				return
			}
			token := service.CreateAuthToken(playerID, playerSecret, httputil.NowUTC())
			httputil.WriteJSON(w, http.StatusOK, map[string]string{"token": token})
			return
		}

		if len(parts) == 2 && parts[1] == "presence" && r.Method == http.MethodPost {
			var req contracts.MatchPresenceRequest
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
				return
			}
			if err := service.HeartbeatPresence(matchID, req, httputil.NowUTC()); err != nil {
				writeMatchError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if len(parts) == 2 && parts[1] == "ws" && r.Method == http.MethodGet {
			handleMatchSocket(w, r, service, &upgrader, matchID, wsReadLimit)
			return
		}

		httputil.WriteError(w, http.StatusNotFound, "route not found")
	})

	addr := httputil.ListenAddr("MATCH_SERVICE_ADDR", 8081)
	srv := &http.Server{
		Addr:              addr,
		// CORS middleware wraps CSRF so that even CSRF-rejected responses
		// carry the proper Access-Control-Allow-* headers. Otherwise the
		// browser reports "blocked by CORS policy" on legitimate cross-origin
		// POSTs whose Origin happens to mismatch the same-origin self check.
		Handler:           httputil.WithRecovery(httputil.WithLogging("match-service", httputil.LimitBody(withCORS(rate_limit.CSRFMiddleware(rl.Middleware(rate_limit.DefaultAPIWindow, rate_limit.DefaultAPILimit)(mux), httputil.ParseAllowedOrigins()))))),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	go func() {
		log.Printf("match-service listening on %s", addr)
		service.Log.Info("listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("match-service shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	rl.Close()
}

type finalizingArchiveStore struct {
	archive      *platform.MatchArchiveStore
	platformURL  string
	serviceToken string
	client       *http.Client
	mu           sync.Mutex
	inFlight     map[string]struct{}
	done         map[string]struct{}
}

func newFinalizingArchiveStore(archive *platform.MatchArchiveStore) *finalizingArchiveStore {
	return &finalizingArchiveStore{
		archive:      archive,
		platformURL:  platformServiceURL(),
		serviceToken: internalServiceToken(),
		client:       &http.Client{Timeout: 5 * time.Second},
		inFlight:     make(map[string]struct{}),
		done:         make(map[string]struct{}),
	}
}

func (s *finalizingArchiveStore) Upsert(snapshot contracts.MatchSnapshotResponse) error {
	if err := s.archive.Upsert(snapshot); err != nil {
		return err
	}
	s.maybeFinalizeRatedMatch(snapshot)
	return nil
}

func (s *finalizingArchiveStore) LoadMatch(matchID string) (contracts.MatchState, []contracts.ResolvedEvent, bool) {
	return s.archive.LoadMatch(matchID)
}

func (s *finalizingArchiveStore) ListUnfinishedMatchIDs(limit int) []string {
	return s.archive.ListUnfinishedMatchIDs(limit)
}

func (s *finalizingArchiveStore) ListFinishedMatchIDs(limit int) []string {
	return s.archive.ListFinishedMatchIDs(limit)
}

func (s *finalizingArchiveStore) maybeFinalizeRatedMatch(snapshot contracts.MatchSnapshotResponse) {
	matchState := snapshot.Match
	matchID := strings.TrimSpace(matchState.MatchID)
	if matchID == "" || strings.TrimSpace(s.serviceToken) == "" || strings.TrimSpace(s.platformURL) == "" {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(matchState.Queue), "rated") || !strings.EqualFold(strings.TrimSpace(matchState.Status), "finished") {
		return
	}
	if strings.TrimSpace(matchState.Winner) == "" {
		return
	}
	if !s.beginFinalization(matchID) {
		return
	}

	go func() {
		err := s.finalizeRatedMatch(matchID)
		s.finishFinalization(matchID, err == nil)
		if err != nil {
			log.Printf("trusted rated finalization failed for match %s: %v", matchID, err)
		}
	}()
}

func (s *finalizingArchiveStore) beginFinalization(matchID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.done[matchID]; ok {
		return false
	}
	if _, ok := s.inFlight[matchID]; ok {
		return false
	}
	s.inFlight[matchID] = struct{}{}
	return true
}

func (s *finalizingArchiveStore) finishFinalization(matchID string, succeeded bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.inFlight, matchID)
	if succeeded {
		s.done[matchID] = struct{}{}
	}
}

func (s *finalizingArchiveStore) finalizeRatedMatch(matchID string) error {
	body, err := json.Marshal(map[string]string{"matchId": matchID})
	if err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPost, strings.TrimRight(s.platformURL, "/")+"/api/platform/internal/finalize-rated-match", bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+s.serviceToken)

	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("platform finalizer returned %d", response.StatusCode)
	}
	return nil
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
	return httputil.EnvOrDefault("MATCH_ARCHIVE_POSTGRES_URL", "")
}

func openArchiveStore() (*platform.MatchArchiveStore, error) {
	switch strings.ToLower(httputil.EnvOrDefault("MATCH_ARCHIVE_BACKEND", "file")) {
	case "sqlite":
		return platform.NewSQLiteMatchArchiveStore(archiveSQLitePath())
	case "postgres":
		return platform.NewPostgresMatchArchiveStore(archivePostgresURL())
	default:
		return platform.NewMatchArchiveStore(archivePath())
	}
}

func openMatchStore() (match.MatchStore, match.Broadcaster) {
	backend := strings.ToLower(httputil.EnvOrDefault("MATCH_STATE_BACKEND", "redis"))
	redisURL := httputil.EnvOrDefault("MATCH_REDIS_URL", "")
	keyPrefix := httputil.EnvOrDefault("MATCH_REDIS_KEY_PREFIX", "chess404:match")

	if backend == "redis" && redisURL != "" {
		store, err := match.NewRedisMatchStore(redisURL, keyPrefix)
		if err != nil {
			log.Printf("WARNING: failed to connect to redis for match store, falling back to memory: %v", err)
			return match.NewMemoryMatchStore(), match.NoopBroadcaster{}
		}
		broadcaster, err := match.NewRedisBroadcaster(redisURL, keyPrefix)
		if err != nil {
			log.Printf("WARNING: failed to connect to redis for broadcaster, falling back to noop: %v", err)
			return store, match.NoopBroadcaster{}
		}
		log.Printf("match store: redis backend (prefix=%s)", keyPrefix)
		return store, broadcaster
	}

	if backend == "redis" {
		log.Printf("WARNING: MATCH_STATE_BACKEND=redis but MATCH_REDIS_URL is unset, falling back to memory (DATA LOSS ON RESTART)")
	} else {
		log.Printf("WARNING: MATCH_STATE_BACKEND=%s, match state will be lost on restart", backend)
	}
	return match.NewMemoryMatchStore(), match.NoopBroadcaster{}
}

func writeMatchError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, match.ErrMatchNotFound):
		httputil.WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, match.ErrMatchSeatFull):
		httputil.WriteError(w, http.StatusConflict, err.Error())
	case errors.Is(err, match.ErrMatchJoinFinished):
		httputil.WriteError(w, http.StatusConflict, err.Error())
	case errors.Is(err, match.ErrStaleClientState):
		httputil.WriteError(w, http.StatusConflict, err.Error())
	default:
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
	}
}

func handleMatchSocket(w http.ResponseWriter, r *http.Request, service *match.Service, upgrader *websocket.Upgrader, matchID string, wsReadLimit int64) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(wsReadLimit)
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	var authMsg struct {
		ClaimToken   string `json:"claimToken,omitempty"`
		PlayerID     string `json:"playerId,omitempty"`
		PlayerSecret string `json:"playerSecret,omitempty"`
	}
	if err := conn.ReadJSON(&authMsg); err != nil {
		_ = conn.Close()
		return
	}
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))

	playerClaimToken := strings.TrimSpace(authMsg.ClaimToken)
	var playerID, playerSecret string
	if playerClaimToken != "" {
		if pid, psec, ok := service.ResolveAuthToken(playerClaimToken); ok {
			playerID = pid
			playerSecret = psec
		} else {
			claim, err := resolveSocketClaim(matchID, playerClaimToken)
			if err != nil {
				_ = conn.WriteJSON(map[string]string{"type": "auth.error", "message": "unauthorized"})
				_ = conn.Close()
				return
			}
			playerID = strings.TrimSpace(claim.PlayerID)
			playerSecret = strings.TrimSpace(claim.PlayerSecret)
		}
	} else if authMsg.PlayerID != "" && authMsg.PlayerSecret != "" {
		playerID = strings.TrimSpace(authMsg.PlayerID)
		playerSecret = strings.TrimSpace(authMsg.PlayerSecret)
	}
	if playerID == "" || playerSecret == "" {
		_ = conn.WriteJSON(map[string]string{"type": "auth.error", "message": "unauthorized"})
		_ = conn.Close()
		return
	}

	stream, unsubscribe, initial, err := service.Subscribe(matchID, playerID)
	if err != nil {
		_ = conn.WriteJSON(map[string]string{"type": "auth.error", "message": err.Error()})
		_ = conn.Close()
		return
	}

	if err := writeEnvelope(conn, "auth.success", nil); err != nil {
		unsubscribe()
		_ = conn.Close()
		return
	}

	done := make(chan struct{})

	go func() {
		defer close(done)
		conn.SetPongHandler(func(string) error {
			_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			return nil
		})
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	defer func() {
		unsubscribe()
		_ = service.MarkDisconnected(matchID, playerID, playerSecret, httputil.NowUTC())
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
		case <-done:
			return
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
	allowed := httputil.ParseAllowedOrigins()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" || httputil.IsOriginAllowed(origin, allowed) {
			if origin == "" && len(allowed) > 0 {
				origin = allowed[0]
			}
			if origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
			w.Header().Set("Vary", "Origin")
		} else {
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		// Allow the custom identity headers used by the web client (x-chess404-{white|black}-{guest-id|session-token|session-secret}).
		// Safe to use "*" because no `credentials: include` is set on the client side.
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, X-Chess404-White-Guest-Id, X-Chess404-White-Session-Token, X-Chess404-White-Session-Secret, X-Chess404-Black-Guest-Id, X-Chess404-Black-Session-Token, X-Chess404-Black-Session-Secret")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Type, X-Request-Id")
		w.Header().Set("Access-Control-Max-Age", "600")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func resolveSocketClaim(matchID, claimToken string) (platform.MatchSeatClaim, error) {
	payload := map[string]string{
		"matchId":    strings.TrimSpace(matchID),
		"claimToken": strings.TrimSpace(claimToken),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return platform.MatchSeatClaim{}, err
	}

	request, err := http.NewRequest(http.MethodPost, platformServiceURL()+"/api/platform/match-claims/resolve", bytes.NewReader(body))
	if err != nil {
		return platform.MatchSeatClaim{}, err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := (&http.Client{Timeout: 3 * time.Second}).Do(request)
	if err != nil {
		return platform.MatchSeatClaim{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return platform.MatchSeatClaim{}, fmt.Errorf("claim resolve failed with %d", response.StatusCode)
	}

	var claim platform.MatchSeatClaim
	if err := json.NewDecoder(response.Body).Decode(&claim); err != nil {
		return platform.MatchSeatClaim{}, err
	}
	if strings.TrimSpace(claim.PlayerID) == "" || strings.TrimSpace(claim.PlayerSecret) == "" {
		return platform.MatchSeatClaim{}, errors.New("claim missing player credentials")
	}
	return claim, nil
}

func platformServiceURL() string {
	return httputil.EnvOrDefault("PLATFORM_SERVICE_INTERNAL_URL", "http://platform-service:8080")
}

func internalServiceToken() string {
	for _, name := range []string{"PLATFORM_INTERNAL_SERVICE_TOKEN", "CHESS404_INTERNAL_SERVICE_TOKEN", "INTERNAL_SERVICE_TOKEN"} {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}
