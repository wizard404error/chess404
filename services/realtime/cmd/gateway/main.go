package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/chess404/realtime/internal/contracts"
	"github.com/chess404/realtime/internal/envutil"
	"github.com/chess404/realtime/internal/httputil"
	"github.com/chess404/realtime/internal/matchmaking"
	"github.com/chess404/realtime/internal/metrics"
	"github.com/chess404/realtime/internal/platform"
	"github.com/chess404/realtime/internal/rate_limit"
)

type GatewayConfig struct {
	MatchServiceURL       string `json:"matchServiceUrl"`
	PlatformServiceURL    string `json:"platformServiceUrl"`
	MatchmakingServiceURL string `json:"matchmakingServiceUrl"`
}

type GatewayServiceHealth struct {
	URL        string `json:"url"`
	Healthy    bool   `json:"healthy"`
	StatusCode int    `json:"statusCode,omitempty"`
	Payload    any    `json:"payload,omitempty"`
	Error      string `json:"error,omitempty"`
}

type GatewaySystemStatus struct {
	Status    string                          `json:"status"`
	Service   string                          `json:"service"`
	CheckedAt time.Time                       `json:"checkedAt"`
	Services  map[string]GatewayServiceHealth `json:"services"`
}

type GatewayGuestIdentity struct {
	GuestID       string `json:"guestId,omitempty"`
	SessionSecret string `json:"sessionSecret,omitempty"`
	SessionToken  string `json:"sessionToken,omitempty"`
}

type GatewayAccountIdentity struct {
	AccountID    string `json:"accountId,omitempty"`
	SessionToken string `json:"sessionToken,omitempty"`
}

type GatewaySeatClaim struct {
	MatchID      string                `json:"matchId"`
	GuestID      string                `json:"guestId"`
	SeatColor    string                `json:"seatColor"`
	PlayerID     string                `json:"playerId"`
	PlayerSecret string                `json:"playerSecret"`
	ClaimToken   string                `json:"claimToken,omitempty"`
	ExpiresAt    time.Time             `json:"expiresAt,omitempty"`
	Queue        string                `json:"queue,omitempty"`
	ModeID       contracts.MatchModeID `json:"modeId,omitempty"`
	WhiteGuestID string                `json:"whiteGuestId,omitempty"`
	BlackGuestID string                `json:"blackGuestId,omitempty"`
	WhiteName    string                `json:"whiteName,omitempty"`
	BlackName    string                `json:"blackName,omitempty"`
	Status       string                `json:"status,omitempty"`
}

type GatewayBootstrapRequest struct {
	MatchID      string                  `json:"matchId,omitempty"`
	White        *GatewayGuestIdentity   `json:"white,omitempty"`
	Black        *GatewayGuestIdentity   `json:"black,omitempty"`
	WhiteAccount *GatewayAccountIdentity `json:"whiteAccount,omitempty"`
	BlackAccount *GatewayAccountIdentity `json:"blackAccount,omitempty"`
}

type GatewayBootstrapGuestSessions struct {
	White *platform.GuestSession `json:"white,omitempty"`
	Black *platform.GuestSession `json:"black,omitempty"`
}

type GatewayBootstrapMatchClaims struct {
	White *GatewaySeatClaim `json:"white,omitempty"`
	Black *GatewaySeatClaim `json:"black,omitempty"`
}

type GatewayBootstrapAccountSessions struct {
	White *platform.AccountSession `json:"white,omitempty"`
	Black *platform.AccountSession `json:"black,omitempty"`
}

type GatewayBootstrapQueueTickets struct {
	White *matchmaking.Ticket `json:"white,omitempty"`
	Black *matchmaking.Ticket `json:"black,omitempty"`
}

type GatewayBootstrapErrors struct {
	White string `json:"white,omitempty"`
	Black string `json:"black,omitempty"`
}

type GatewayBootstrapRecoveredMatch struct {
	MatchID      string                       `json:"matchId"`
	Queue        string                       `json:"queue,omitempty"`
	ModeID       contracts.MatchModeID        `json:"modeId,omitempty"`
	Status       string                       `json:"status,omitempty"`
	ViewerSeat   string                       `json:"viewerSeat,omitempty"`
	WhiteGuestID string                       `json:"whiteGuestId,omitempty"`
	BlackGuestID string                       `json:"blackGuestId,omitempty"`
	WhiteName    string                       `json:"whiteName,omitempty"`
	BlackName    string                       `json:"blackName,omitempty"`
	Claims       *GatewayBootstrapMatchClaims `json:"claims,omitempty"`
}

type GatewayBootstrapPayload struct {
	Status               string                           `json:"status"`
	RealtimeReady        bool                             `json:"realtimeReady"`
	PlatformReady        bool                             `json:"platformReady"`
	MatchmakingReady     bool                             `json:"matchmakingReady"`
	Authoritative        bool                             `json:"authoritative"`
	Services             map[string]GatewayServiceHealth  `json:"services"`
	ServiceEndpoints     GatewayConfig                    `json:"serviceEndpoints"`
	PlatformCaps         any                              `json:"platformCaps,omitempty"`
	DefaultQueue         any                              `json:"defaultQueue,omitempty"`
	GuestSessions        *GatewayBootstrapGuestSessions   `json:"guestSessions,omitempty"`
	MatchClaims          *GatewayBootstrapMatchClaims     `json:"matchClaims,omitempty"`
	AccountSessions      *GatewayBootstrapAccountSessions `json:"accountSessions,omitempty"`
	QueueTickets         *GatewayBootstrapQueueTickets    `json:"queueTickets,omitempty"`
	RecoveredMatch       *GatewayBootstrapRecoveredMatch  `json:"recoveredMatch,omitempty"`
	SessionErrors        *GatewayBootstrapErrors          `json:"sessionErrors,omitempty"`
	ClaimErrors          *GatewayBootstrapErrors          `json:"claimErrors,omitempty"`
	AccountErrors        *GatewayBootstrapErrors          `json:"accountErrors,omitempty"`
	QueueErrors          *GatewayBootstrapErrors          `json:"queueErrors,omitempty"`
	RecoveredMatchErrors *GatewayBootstrapErrors          `json:"recoveredMatchErrors,omitempty"`
	RequestedMatchID     string                           `json:"requestedMatchId,omitempty"`
	BootstrapCheckedAt   time.Time                        `json:"bootstrapCheckedAt"`
	Message              string                           `json:"message"`
}

type GatewayPrivateMatchRequest struct {
	Guest         GatewayGuestIdentity    `json:"guest"`
	Account       *GatewayAccountIdentity `json:"account,omitempty"`
	Queue         string                  `json:"queue,omitempty"`
	ModeID        contracts.MatchModeID   `json:"modeId,omitempty"`
	Difficulty    string                  `json:"difficulty,omitempty"`
	ClockSeconds  int64                   `json:"clockSeconds,omitempty"`
	PreferredSeat string                  `json:"preferredSeat,omitempty"`
}

type GatewayPrivateMatchResponse struct {
	MatchID            string                          `json:"matchId"`
	SeatColor          string                          `json:"seatColor"`
	WaitingForOpponent bool                            `json:"waitingForOpponent"`
	Snapshot           contracts.MatchSnapshotResponse `json:"snapshot"`
	Claim              *GatewaySeatClaim               `json:"claim,omitempty"`
}

type GatewayDirectChallengeRequest struct {
	Guest           GatewayGuestIdentity    `json:"guest"`
	Account         *GatewayAccountIdentity `json:"account,omitempty"`
	TargetAccountID string                  `json:"targetAccountId"`
	ModeID          contracts.MatchModeID   `json:"modeId,omitempty"`
	ClockSeconds    int64                   `json:"clockSeconds,omitempty"`
	PreferredSeat   string                  `json:"preferredSeat,omitempty"`
}

type GatewayDirectChallengeAcceptRequest struct {
	Guest   GatewayGuestIdentity    `json:"guest"`
	Account *GatewayAccountIdentity `json:"account,omitempty"`
}

type GatewayDirectChallengeView struct {
	ChallengeID    string                `json:"challengeId"`
	Status         string                `json:"status"`
	MatchID        string                `json:"matchId"`
	ModeID         contracts.MatchModeID `json:"modeId,omitempty"`
	ClockSeconds   int64                 `json:"clockSeconds,omitempty"`
	ChallengerSeat string                `json:"challengerSeat,omitempty"`
	ViewerSeat     string                `json:"viewerSeat,omitempty"`
}

type GatewayDirectChallengeLaunchResponse struct {
	ChallengeID string                      `json:"challengeId"`
	ModeID      contracts.MatchModeID       `json:"modeId,omitempty"`
	Match       GatewayPrivateMatchResponse `json:"match"`
}

const maxBodySize = 1 << 20 // 1 MB limit for request bodies

func isValidPathParam(param string) bool {
	return !strings.Contains(param, "/") && !strings.Contains(param, "..") && strings.TrimSpace(param) == param && param != ""
}

func main() {
	envutil.Require("MATCH_SERVICE_INTERNAL_URL", "PLATFORM_SERVICE_INTERNAL_URL", "MATCHMAKING_SERVICE_INTERNAL_URL", "ALLOWED_ORIGINS")
	config := gatewayConfigFromEnv()
	client := httputil.NewHTTPClient(3 * time.Second)
	mux := buildGatewayMux(config, client)
	rl := rate_limit.New()

	addr := httputil.ListenAddr("GATEWAY_ADDR", 8080)
	srv := &http.Server{
		Addr:              addr,
		Handler:           httputil.WithRecovery(httputil.WithLogging("gateway", rate_limit.CSRFMiddleware(rl.Middleware(rate_limit.DefaultAPIWindow, rate_limit.DefaultAPILimit)(rate_limit.ContentTypeMiddleware(mux)), httputil.ParseAllowedOrigins()))),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	go func() {
		certFile := os.Getenv("TLS_CERT_FILE")
		keyFile := os.Getenv("TLS_KEY_FILE")
		if certFile != "" && keyFile != "" {
			log.Printf("gateway listening with TLS on %s", addr)
			if err := srv.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
				log.Fatalf("listen: %v", err)
			}
		} else {
			log.Printf("gateway listening on %s", addr)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("listen: %v", err)
			}
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("gateway shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	rl.Close()
}

func buildGatewayMux(config GatewayConfig, client *http.Client) http.Handler {
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]any{
			"status":    "ok",
			"service":   "gateway",
			"checkedAt": time.Now().UTC(),
		})
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		httputil.WriteJSON(w, http.StatusOK, map[string]any{
			"status":    "ok",
			"service":   "gateway",
			"checkedAt": time.Now().UTC(),
		})
	})

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		httputil.WriteJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})

	mux.HandleFunc("/livez", func(w http.ResponseWriter, _ *http.Request) {
		httputil.WriteJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})

	mux.HandleFunc("/api/system/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			httputil.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, collectGatewayStatus(config, client, r))
	})

	mux.Handle("/metrics", metrics.Handler())

	mux.HandleFunc("/api/session/bootstrap", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			httputil.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var payload GatewayBootstrapPayload
		if r.Method == http.MethodPost {
			var request GatewayBootstrapRequest
			if r.Body != nil {
				defer r.Body.Close()
				r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					httputil.WriteError(w, http.StatusBadRequest, "invalid bootstrap payload")
					return
				}
			}
			payload = buildGatewayBootstrapPayload(config, client, request, r)
		} else {
			payload = buildGatewayBootstrapPayload(config, client, GatewayBootstrapRequest{}, r)
		}

		httputil.WriteJSON(w, http.StatusOK, contracts.Envelope{
			Type:    "gateway.bootstrap",
			Payload: payload,
		})
	})

	mux.HandleFunc("/api/private-matches", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httputil.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var payload GatewayPrivateMatchRequest
		if r.Body != nil {
			defer r.Body.Close()
			r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				httputil.WriteError(w, http.StatusBadRequest, "invalid private match payload")
				return
			}
		}
		response, statusCode, err := createGatewayPrivateMatch(config, client, payload, r)
		if err != nil {
			httputil.WriteError(w, statusCode, err.Error())
			return
		}
		httputil.WriteJSON(w, http.StatusCreated, response)
	})

	mux.HandleFunc("/api/matches/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httputil.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/matches/")
		if path == "" {
			httputil.WriteError(w, http.StatusNotFound, "match id required")
			return
		}
		parts := strings.Split(path, "/")
		if len(parts) != 2 || (parts[1] != "intents" && parts[1] != "presence") {
			httputil.WriteError(w, http.StatusNotFound, "route not found")
			return
		}
		if !isValidPathParam(parts[0]) {
			httputil.WriteError(w, http.StatusBadRequest, "invalid match id")
			return
		}
		if parts[1] == "intents" {
			proxyGatewayIntent(w, r, config, client, parts[0])
			return
		}
		proxyGatewayPresence(w, r, config, client, parts[0])
	})

	mux.HandleFunc("/api/private-matches/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httputil.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/private-matches/")
		if path == "" {
			httputil.WriteError(w, http.StatusNotFound, "match id required")
			return
		}
		parts := strings.Split(path, "/")
		if len(parts) != 2 || (parts[1] != "join" && parts[1] != "rematch") {
			httputil.WriteError(w, http.StatusNotFound, "route not found")
			return
		}
		var payload GatewayPrivateMatchRequest
		if r.Body != nil {
			defer r.Body.Close()
			r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				httputil.WriteError(w, http.StatusBadRequest, "invalid private match join payload")
				return
			}
		}
		if !isValidPathParam(parts[0]) {
			httputil.WriteError(w, http.StatusBadRequest, "invalid match id")
			return
		}
		var (
			response   GatewayPrivateMatchResponse
			statusCode int
			err        error
		)
		if parts[1] == "join" {
			response, statusCode, err = joinGatewayPrivateMatch(config, client, parts[0], payload, r)
		} else {
			response, statusCode, err = rematchGatewayPrivateMatch(config, client, parts[0], payload, r)
		}
		if err != nil {
			httputil.WriteError(w, statusCode, err.Error())
			return
		}
		httputil.WriteJSON(w, http.StatusOK, response)
	})

	mux.HandleFunc("/api/challenges", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httputil.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var payload GatewayDirectChallengeRequest
		if r.Body != nil {
			defer r.Body.Close()
			r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				httputil.WriteError(w, http.StatusBadRequest, "invalid direct challenge payload")
				return
			}
		}
		response, statusCode, err := createGatewayDirectChallenge(config, client, payload, r)
		if err != nil {
			httputil.WriteError(w, statusCode, err.Error())
			return
		}
		httputil.WriteJSON(w, http.StatusCreated, response)
	})

	mux.HandleFunc("/api/challenges/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httputil.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/challenges/")
		parts := strings.Split(path, "/")
		if len(parts) != 2 || parts[1] != "accept" || strings.TrimSpace(parts[0]) == "" {
			httputil.WriteError(w, http.StatusNotFound, "route not found")
			return
		}
		var payload GatewayDirectChallengeAcceptRequest
		if r.Body != nil {
			defer r.Body.Close()
			r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				httputil.WriteError(w, http.StatusBadRequest, "invalid challenge accept payload")
				return
			}
		}
		if !isValidPathParam(parts[0]) {
			httputil.WriteError(w, http.StatusBadRequest, "invalid challenge id")
			return
		}
		response, statusCode, err := acceptGatewayDirectChallenge(config, client, parts[0], payload, r)
		if err != nil {
			httputil.WriteError(w, statusCode, err.Error())
			return
		}
		httputil.WriteJSON(w, http.StatusOK, response)
	})

	return sourceRequestMiddleware(mux)
}

// sourceRequestKey is the context key under which the gateway stores the
// incoming *http.Request for the lifetime of a single request. Downstream
// helpers (e.g., fetchGatewayJSONRequestWithContext) read it back to forward
// the browser's Origin/Referer headers to backend services, so their CSRF
// middleware can validate the request against its allow-list.
type sourceRequestKey struct{}

func withSourceRequest(ctx context.Context, r *http.Request) context.Context {
	if r == nil {
		return ctx
	}
	return context.WithValue(ctx, sourceRequestKey{}, r)
}

func sourceRequestFromContext(ctx context.Context) *http.Request {
	if ctx == nil {
		return nil
	}
	r, _ := ctx.Value(sourceRequestKey{}).(*http.Request)
	return r
}

// sourceRequestMiddleware wraps a handler so every request's source is
// available in its context. This lets the gateway's outgoing HTTP calls
// (which use context.Background today) read back the original incoming
// request to forward headers like Origin/Referer.
func sourceRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := withSourceRequest(r.Context(), r)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// reconstructPublicOrigin returns the public-facing origin (scheme + host)
// for an incoming request, taking reverse-proxy headers into account. This
// mirrors the logic the CSRF middleware uses in
// internal/rate_limit.trustedSelfOrigin, applied to the source request
// rather than the current request.
//
// The browser sends an Origin header only for cross-origin requests, and
// the Referer it sends for same-origin POSTs includes the request path
// (e.g., https://example.com/play), which does not match a bare origin in
// a CSRF allow-list. Reconstructing the origin from the source's host
// information gives the gateway a clean origin to forward to backend
// services.
func reconstructPublicOrigin(r *http.Request) string {
	if r == nil {
		return ""
	}
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
		// Browser sent Origin (e.g., cross-origin POST). Parse it and
		// return just scheme://host[:port] so the destination's CSRF
		// allow-list (which contains bare origins) can match.
		if u, err := url.Parse(origin); err == nil && u.Scheme != "" && u.Host != "" {
			return u.Scheme + "://" + u.Host
		}
	}
	host := r.Host
	scheme := "https://"
	if r.TLS == nil {
		scheme = "http://"
		if forwardedProto := firstForwardedValue(r.Header, "X-Forwarded-Proto"); forwardedProto != "" {
			scheme = forwardedProto + "://"
		}
		if forwardedHost := firstForwardedValue(r.Header, "X-Forwarded-Host"); forwardedHost != "" {
			host = forwardedHost
		}
	}
	if host == "" {
		return ""
	}
	return scheme + host
}

func firstForwardedValue(h http.Header, name string) string {
	raw := h.Get(name)
	if raw == "" {
		return ""
	}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			return part
		}
	}
	return ""
}

func collectGatewayStatus(config GatewayConfig, client *http.Client, r *http.Request) GatewaySystemStatus {
	services := map[string]GatewayServiceHealth{
		"match":       fetchGatewayJSON(r, client, config.MatchServiceURL+"/api/system/status"),
		"platform":    fetchGatewayJSON(r, client, config.PlatformServiceURL+"/api/platform/status"),
		"matchmaking": fetchGatewayJSON(r, client, config.MatchmakingServiceURL+"/api/status"),
	}

	status := "ok"
	for name := range services {
		if !services[name].Healthy {
			status = "degraded"
		}
		service := services[name]
		service.URL = ""
		service.Payload = nil
		service.Error = ""
		services[name] = service
	}

	return GatewaySystemStatus{
		Status:    status,
		Service:   "gateway",
		CheckedAt: time.Now().UTC(),
		Services:  services,
	}
}

func buildGatewayBootstrapPayload(config GatewayConfig, client *http.Client, request GatewayBootstrapRequest, r *http.Request) GatewayBootstrapPayload {
	systemStatus := collectGatewayStatus(config, client, r)
	capabilities := fetchGatewayJSON(r, client, config.PlatformServiceURL+"/api/platform/capabilities")
	defaultQueue := fetchGatewayJSON(r, client, config.MatchmakingServiceURL+"/api/queues/default")
	guestSessions, sessionErrors := bootstrapGuestSessions(config, client, request, r)
	matchClaims, claimErrors := bootstrapMatchClaims(config, client, request.MatchID, guestSessions, r)
	accountSessions, accountErrors := bootstrapAccountSessions(config, client, request, guestSessions, r)
	queueTickets, queueErrors := bootstrapQueueTickets(config, client, guestSessions, accountSessions, r)
	recoveredMatch, recoveredMatchErrors := bootstrapRecoveredMatch(config, client, guestSessions, queueTickets, r)

	return GatewayBootstrapPayload{
		Status:               systemStatus.Status,
		RealtimeReady:        systemStatus.Services["match"].Healthy,
		PlatformReady:        systemStatus.Services["platform"].Healthy,
		MatchmakingReady:     systemStatus.Services["matchmaking"].Healthy,
		Authoritative:        systemStatus.Services["match"].Healthy,
		Services:             systemStatus.Services,
		ServiceEndpoints:     GatewayConfig{},
		PlatformCaps:         capabilities.Payload,
		DefaultQueue:         defaultQueue.Payload,
		GuestSessions:        guestSessions,
		MatchClaims:          matchClaims,
		AccountSessions:      accountSessions,
		QueueTickets:         queueTickets,
		RecoveredMatch:       recoveredMatch,
		SessionErrors:        sessionErrors,
		ClaimErrors:          claimErrors,
		AccountErrors:        accountErrors,
		QueueErrors:          queueErrors,
		RecoveredMatchErrors: recoveredMatchErrors,
		RequestedMatchID:     request.MatchID,
		BootstrapCheckedAt:   time.Now().UTC(),
		Message:              bootstrapMessage(systemStatus),
	}
}

func bootstrapGuestSessions(config GatewayConfig, client *http.Client, request GatewayBootstrapRequest, r *http.Request) (*GatewayBootstrapGuestSessions, *GatewayBootstrapErrors) {
	sessions := &GatewayBootstrapGuestSessions{}
	errors := &GatewayBootstrapErrors{}

	if session, errMessage := bootstrapGuestSessionForSide(config, client, request.White, r); session != nil {
		sessions.White = session
	} else if errMessage != "" {
		errors.White = errMessage
	}

	if session, errMessage := bootstrapGuestSessionForSide(config, client, request.Black, r); session != nil {
		sessions.Black = session
	} else if errMessage != "" {
		errors.Black = errMessage
	}

	if sessions.White == nil && sessions.Black == nil {
		sessions = nil
	}
	if errors.White == "" && errors.Black == "" {
		errors = nil
	}

	return sessions, errors
}

func bootstrapGuestSessionForSide(config GatewayConfig, client *http.Client, identity *GatewayGuestIdentity, r *http.Request) (*platform.GuestSession, string) {
	result := fetchGatewayJSONRequest(r, client, http.MethodPost, config.PlatformServiceURL+"/api/platform/guest-sessions", identity)
	if !result.Healthy && result.StatusCode == http.StatusUnauthorized && identity != nil && (identity.GuestID != "" || identity.SessionSecret != "") {
		result = fetchGatewayJSONRequest(r, client, http.MethodPost, config.PlatformServiceURL+"/api/platform/guest-sessions", GatewayGuestIdentity{})
	}
	if !result.Healthy {
		return nil, gatewayErrorMessage(result, "failed to bootstrap guest session")
	}

	session, err := decodeGatewayPayload[platform.GuestSession](result.Payload)
	if err != nil {
		return nil, fmt.Sprintf("failed to decode guest session: %v", err)
	}
	return &session, ""
}

func bootstrapMatchClaims(config GatewayConfig, client *http.Client, matchID string, sessions *GatewayBootstrapGuestSessions, r *http.Request) (*GatewayBootstrapMatchClaims, *GatewayBootstrapErrors) {
	if matchID == "" || sessions == nil {
		return nil, nil
	}

	claims := &GatewayBootstrapMatchClaims{}
	errors := &GatewayBootstrapErrors{}

	if claim, errMessage := bootstrapMatchClaimForSide(config, client, matchID, sessions.White, nil, r); claim != nil {
		claims.White = sanitizeSeatClaim(claim)
	} else if errMessage != "" {
		errors.White = errMessage
	}

	if claim, errMessage := bootstrapMatchClaimForSide(config, client, matchID, sessions.Black, nil, r); claim != nil {
		claims.Black = sanitizeSeatClaim(claim)
	} else if errMessage != "" {
		errors.Black = errMessage
	}

	if claims.White == nil && claims.Black == nil {
		claims = nil
	}
	if errors.White == "" && errors.Black == "" {
		errors = nil
	}

	return claims, errors
}

type matchClaimBootstrapFields struct {
	SeatColor    string
	PlayerSecret string
	WhiteGuestID string
	BlackGuestID string
	WhiteName    string
	BlackName    string
	Queue        string
	ModeID       string
	MatchStatus  string
}

func bootstrapMatchClaimForSide(config GatewayConfig, client *http.Client, matchID string, session *platform.GuestSession, matchFields *matchClaimBootstrapFields, r *http.Request) (*GatewaySeatClaim, string) {
	if session == nil || session.Guest.GuestID == "" || session.SessionSecret == "" {
		return nil, ""
	}

	body := map[string]string{
		"matchId":       matchID,
		"guestId":       session.Guest.GuestID,
		"sessionSecret": session.SessionSecret,
	}
	if matchFields != nil {
		body["seatColor"] = matchFields.SeatColor
		body["playerSecret"] = matchFields.PlayerSecret
		body["whiteGuestId"] = matchFields.WhiteGuestID
		body["blackGuestId"] = matchFields.BlackGuestID
		body["whiteName"] = matchFields.WhiteName
		body["blackName"] = matchFields.BlackName
		body["queue"] = matchFields.Queue
		body["modeId"] = matchFields.ModeID
		body["matchStatus"] = matchFields.MatchStatus
	}
	result := fetchGatewayJSONRequest(r, client, http.MethodPost, config.PlatformServiceURL+"/api/platform/match-claims", body)
	if !result.Healthy {
		return nil, gatewayErrorMessage(result, "failed to recover match claim")
	}

	claim, err := decodeGatewayPayload[GatewaySeatClaim](result.Payload)
	if err != nil {
		return nil, fmt.Sprintf("failed to decode match claim: %v", err)
	}
	return &claim, ""
}

func bootstrapAccountSessions(config GatewayConfig, client *http.Client, request GatewayBootstrapRequest, guestSessions *GatewayBootstrapGuestSessions, r *http.Request) (*GatewayBootstrapAccountSessions, *GatewayBootstrapErrors) {
	sessions := &GatewayBootstrapAccountSessions{}
	errors := &GatewayBootstrapErrors{}

	if session, errMessage := bootstrapAccountSessionForSide(config, client, request.WhiteAccount, guestSessionsSide(guestSessions, "white"), r); session != nil {
		sessions.White = session
	} else if errMessage != "" {
		errors.White = errMessage
	}

	if session, errMessage := bootstrapAccountSessionForSide(config, client, request.BlackAccount, guestSessionsSide(guestSessions, "black"), r); session != nil {
		sessions.Black = session
	} else if errMessage != "" {
		errors.Black = errMessage
	}

	if sessions.White == nil && sessions.Black == nil {
		sessions = nil
	}
	if errors.White == "" && errors.Black == "" {
		errors = nil
	}

	return sessions, errors
}

func bootstrapQueueTickets(
	config GatewayConfig,
	client *http.Client,
	guestSessions *GatewayBootstrapGuestSessions,
	accountSessions *GatewayBootstrapAccountSessions,
	r *http.Request,
) (*GatewayBootstrapQueueTickets, *GatewayBootstrapErrors) {
	tickets := &GatewayBootstrapQueueTickets{}
	errors := &GatewayBootstrapErrors{}

	if ticket, errMessage := bootstrapQueueTicketForSide(config, client, guestSessionsSide(guestSessions, "white"), accountSessionsSide(accountSessions, "white"), r); ticket != nil {
		tickets.White = ticket
	} else if errMessage != "" {
		errors.White = errMessage
	}

	if ticket, errMessage := bootstrapQueueTicketForSide(config, client, guestSessionsSide(guestSessions, "black"), accountSessionsSide(accountSessions, "black"), r); ticket != nil {
		tickets.Black = ticket
	} else if errMessage != "" {
		errors.Black = errMessage
	}

	if tickets.White == nil && tickets.Black == nil {
		tickets = nil
	}
	if errors.White == "" && errors.Black == "" {
		errors = nil
	}

	return tickets, errors
}

func bootstrapQueueTicketForSide(
	config GatewayConfig,
	client *http.Client,
	guestSession *platform.GuestSession,
	accountSession *platform.AccountSession,
	r *http.Request,
) (*matchmaking.Ticket, string) {
	guestID := ""
	if guestSession != nil {
		guestID = strings.TrimSpace(guestSession.Guest.GuestID)
	}
	accountID := ""
	if accountSession != nil {
		accountID = strings.TrimSpace(accountSession.Account.AccountID)
	}
	if guestID == "" && accountID == "" {
		return nil, ""
	}

	params := url.Values{}
	if guestID != "" {
		params.Set("guestId", guestID)
	}
	if accountID != "" {
		params.Set("accountId", accountID)
	}

	result := fetchGatewayJSON(r, client, config.MatchmakingServiceURL+"/api/queues/tickets?"+params.Encode())
	if result.StatusCode == http.StatusNotFound {
		return nil, ""
	}
	if !result.Healthy {
		return nil, gatewayErrorMessage(result, "failed to recover queue ticket")
	}

	payload, err := decodeGatewayPayload[struct {
		Ticket matchmaking.Ticket `json:"ticket"`
	}](result.Payload)
	if err != nil {
		return nil, fmt.Sprintf("failed to decode queue ticket: %v", err)
	}
	return &payload.Ticket, ""
}

func bootstrapRecoveredMatch(
	config GatewayConfig,
	client *http.Client,
	guestSessions *GatewayBootstrapGuestSessions,
	queueTickets *GatewayBootstrapQueueTickets,
	r *http.Request,
) (*GatewayBootstrapRecoveredMatch, *GatewayBootstrapErrors) {
	claims, errors := bootstrapActiveMatchClaims(config, client, guestSessions, r)
	if activeMatch := recoveredMatchFromClaims(claims); activeMatch != nil {
		return activeMatch, errors
	}

	if activeMatch := recoveredMatchFromQueueTickets(queueTickets, guestSessions); activeMatch != nil {
		return activeMatch, errors
	}

	if errors != nil && errors.White == "" && errors.Black == "" {
		errors = nil
	}
	return nil, errors
}

func bootstrapActiveMatchClaims(
	config GatewayConfig,
	client *http.Client,
	guestSessions *GatewayBootstrapGuestSessions,
	r *http.Request,
) (*GatewayBootstrapMatchClaims, *GatewayBootstrapErrors) {
	claims := &GatewayBootstrapMatchClaims{}
	errors := &GatewayBootstrapErrors{}

	if claim, errMessage := bootstrapActiveMatchClaimForSide(config, client, guestSessionsSide(guestSessions, "white"), r); claim != nil {
		claims.White = claim
	} else if errMessage != "" {
		errors.White = errMessage
	}

	if claim, errMessage := bootstrapActiveMatchClaimForSide(config, client, guestSessionsSide(guestSessions, "black"), r); claim != nil {
		claims.Black = claim
	} else if errMessage != "" {
		errors.Black = errMessage
	}

	if claims.White == nil && claims.Black == nil {
		claims = nil
	}
	if errors.White == "" && errors.Black == "" {
		errors = nil
	}

	return claims, errors
}

func bootstrapActiveMatchClaimForSide(
	config GatewayConfig,
	client *http.Client,
	session *platform.GuestSession,
	r *http.Request,
) (*GatewaySeatClaim, string) {
	if session == nil || strings.TrimSpace(session.Guest.GuestID) == "" {
		return nil, ""
	}

	payload := map[string]string{
		"guestId": session.Guest.GuestID,
	}
	if strings.TrimSpace(session.SessionToken) != "" {
		payload["sessionToken"] = strings.TrimSpace(session.SessionToken)
	} else if strings.TrimSpace(session.SessionSecret) != "" {
		payload["sessionSecret"] = strings.TrimSpace(session.SessionSecret)
	}

	result := fetchGatewayJSONRequest(r, client, http.MethodPost, config.PlatformServiceURL+"/api/platform/match-claims/active", payload)
	if result.StatusCode == http.StatusNotFound {
		return nil, ""
	}
	if !result.Healthy {
		return nil, gatewayErrorMessage(result, "failed to recover active match")
	}

	claim, err := decodeGatewayPayload[GatewaySeatClaim](result.Payload)
	if err != nil {
		return nil, fmt.Sprintf("failed to decode active match claim: %v", err)
	}
	return &claim, ""
}

func recoveredMatchFromClaims(claims *GatewayBootstrapMatchClaims) *GatewayBootstrapRecoveredMatch {
	if claims == nil {
		return nil
	}

	primary := claims.White
	if primary == nil || !isGatewayRecoverableClaimStatus(primary.Status) {
		primary = claims.Black
	}
	if primary == nil || !isGatewayRecoverableClaimStatus(primary.Status) || strings.TrimSpace(primary.MatchID) == "" {
		return nil
	}

	return &GatewayBootstrapRecoveredMatch{
		MatchID:      primary.MatchID,
		Queue:        primary.Queue,
		ModeID:       primary.ModeID,
		Status:       primary.Status,
		ViewerSeat:   primary.SeatColor,
		WhiteGuestID: primary.WhiteGuestID,
		BlackGuestID: primary.BlackGuestID,
		WhiteName:    primary.WhiteName,
		BlackName:    primary.BlackName,
		Claims:       claims,
	}
}

func recoveredMatchFromQueueTickets(
	tickets *GatewayBootstrapQueueTickets,
	guestSessions *GatewayBootstrapGuestSessions,
) *GatewayBootstrapRecoveredMatch {
	if tickets == nil {
		return nil
	}

	type ticketCandidate struct {
		side   string
		ticket *matchmaking.Ticket
	}
	for _, candidateEntry := range []ticketCandidate{
		{side: "white", ticket: tickets.White},
		{side: "black", ticket: tickets.Black},
	} {
		candidate := candidateEntry.ticket
		if candidate == nil || candidate.Status != matchmaking.StatusMatched || strings.TrimSpace(candidate.AssignedRoom) == "" {
			continue
		}

		viewerSeat := strings.TrimSpace(candidate.SeatColor)
		whiteGuestID := strings.TrimSpace(candidate.MatchedWith)
		blackGuestID := strings.TrimSpace(candidate.MatchedWith)
		whiteName := strings.TrimSpace(candidate.OpponentName)
		blackName := strings.TrimSpace(candidate.OpponentName)

		if viewerSeat == "white" {
			if guest := guestSessionsSide(guestSessions, candidateEntry.side); guest != nil {
				whiteGuestID = guest.Guest.GuestID
				whiteName = guest.Guest.DisplayName
			}
		} else if viewerSeat == "black" {
			if guest := guestSessionsSide(guestSessions, candidateEntry.side); guest != nil {
				blackGuestID = guest.Guest.GuestID
				blackName = guest.Guest.DisplayName
			}
		}

		return &GatewayBootstrapRecoveredMatch{
			MatchID:      candidate.AssignedRoom,
			Queue:        string(candidate.Queue),
			ModeID:       candidate.ModeID,
			Status:       string(candidate.Status),
			ViewerSeat:   viewerSeat,
			WhiteGuestID: whiteGuestID,
			BlackGuestID: blackGuestID,
			WhiteName:    whiteName,
			BlackName:    blackName,
		}
	}

	return nil
}

func isGatewayRecoverableClaimStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "waiting", "active":
		return true
	default:
		return false
	}
}

func guestSessionsSide(sessions *GatewayBootstrapGuestSessions, side string) *platform.GuestSession {
	if sessions == nil {
		return nil
	}
	if side == "white" {
		return sessions.White
	}
	return sessions.Black
}

func accountSessionsSide(sessions *GatewayBootstrapAccountSessions, side string) *platform.AccountSession {
	if sessions == nil {
		return nil
	}
	if side == "white" {
		return sessions.White
	}
	return sessions.Black
}

func bootstrapAccountSessionForSide(config GatewayConfig, client *http.Client, identity *GatewayAccountIdentity, guestSession *platform.GuestSession, r *http.Request) (*platform.AccountSession, string) {
	if identity == nil || strings.TrimSpace(identity.AccountID) == "" {
		return nil, ""
	}

	result := fetchGatewayJSONRequest(r, client, http.MethodPost, config.PlatformServiceURL+"/api/platform/account-sessions", map[string]string{
		"accountId":    strings.TrimSpace(identity.AccountID),
		"sessionToken": strings.TrimSpace(identity.SessionToken),
	})
	if result.Healthy {
		session, err := decodeGatewayPayload[platform.AccountSession](result.Payload)
		if err != nil {
			return nil, fmt.Sprintf("failed to decode account session: %v", err)
		}
		return &session, ""
	}

	if result.StatusCode == http.StatusUnauthorized && guestSession != nil && strings.TrimSpace(guestSession.Guest.GuestID) != "" {
		reclaimed, errMessage := reclaimGatewayAccountSession(config, client, strings.TrimSpace(identity.AccountID), guestSession, r)
		if reclaimed != nil || errMessage != "" {
			return reclaimed, errMessage
		}
	}

	return nil, gatewayErrorMessage(result, "failed to bootstrap account session")
}

func reclaimGatewayAccountSession(config GatewayConfig, client *http.Client, accountID string, guestSession *platform.GuestSession, r *http.Request) (*platform.AccountSession, string) {
	accountResult := fetchGatewayJSON(r, client, config.PlatformServiceURL+"/api/platform/accounts/"+accountID)
	if !accountResult.Healthy {
		return nil, gatewayErrorMessage(accountResult, "failed to fetch account profile")
	}
	accountEnvelope, err := decodeGatewayPayload[struct {
		Account platform.AccountProfile `json:"account"`
	}](accountResult.Payload)
	if err != nil {
		return nil, fmt.Sprintf("failed to decode account profile: %v", err)
	}
	if strings.TrimSpace(accountEnvelope.Account.Handle) == "" {
		return nil, "account profile is missing handle"
	}

	claimPayload := map[string]string{
		"guestId": guestSession.Guest.GuestID,
		"handle":  accountEnvelope.Account.Handle,
	}
	if strings.TrimSpace(guestSession.SessionToken) != "" {
		claimPayload["sessionToken"] = strings.TrimSpace(guestSession.SessionToken)
	} else {
		claimPayload["sessionSecret"] = strings.TrimSpace(guestSession.SessionSecret)
	}

	claimResult := fetchGatewayJSONRequest(r, client, http.MethodPost, config.PlatformServiceURL+"/api/platform/accounts/claim", claimPayload)
	if !claimResult.Healthy {
		return nil, gatewayErrorMessage(claimResult, "failed to reclaim account session")
	}
	session, err := decodeGatewayPayload[platform.AccountSession](claimResult.Payload)
	if err != nil {
		return nil, fmt.Sprintf("failed to decode reclaimed account session: %v", err)
	}
	return &session, ""
}

func resolveGatewayClaimByToken(config GatewayConfig, client *http.Client, matchID, claimToken string, r *http.Request) (*GatewaySeatClaim, string) {
	result := fetchGatewayJSONRequest(r, client, http.MethodPost, config.PlatformServiceURL+"/api/platform/match-claims/resolve", map[string]string{
		"matchId":    matchID,
		"claimToken": claimToken,
	})
	if !result.Healthy {
		return nil, gatewayErrorMessage(result, "failed to resolve room claim")
	}
	claim, err := decodeGatewayPayload[GatewaySeatClaim](result.Payload)
	if err != nil {
		return nil, fmt.Sprintf("failed to decode room claim: %v", err)
	}
	return &claim, ""
}

func createGatewayPrivateMatch(config GatewayConfig, client *http.Client, request GatewayPrivateMatchRequest, r *http.Request) (GatewayPrivateMatchResponse, int, error) {
	session, statusCode, err := ensureGatewayPrivateGuestSession(config, client, request.Guest, r)
	if err != nil {
		return GatewayPrivateMatchResponse{}, statusCode, err
	}
	accountSession, _, accountSessionErr := ensureGatewayPrivateAccountSession(config, client, request.Account, session, r)
	if accountSessionErr != nil {
		log.Printf("note: account session bootstrap skipped: %v", accountSessionErr)
	}
	return createGatewayPrivateMatchForSession(config, client, session, accountSession, request.Queue, request.ModeID, request.ClockSeconds, request.PreferredSeat, request.Difficulty, r)
}

func createGatewayPrivateMatchForSession(
	config GatewayConfig,
	client *http.Client,
	session *platform.GuestSession,
	accountSession *platform.AccountSession,
	queue string,
	modeID contracts.MatchModeID,
	clockSeconds int64,
	preferredSeat string,
	difficulty string,
	r *http.Request,
) (GatewayPrivateMatchResponse, int, error) {
	if session == nil {
		return GatewayPrivateMatchResponse{}, http.StatusBadRequest, errors.New("guest session is required")
	}
	preferredSeat = strings.ToLower(strings.TrimSpace(preferredSeat))
	if preferredSeat == "" {
		preferredSeat = "white"
	}

	matchQueue := strings.TrimSpace(queue)
	if matchQueue == "" {
		matchQueue = "direct"
	} else if matchQueue != "rated" && matchQueue != "casual" && matchQueue != "direct" {
		matchQueue = "direct"
	}

	createReq := contracts.CreateMatchRequest{
		ClockSeconds: clockSeconds,
		Queue:        matchQueue,
		ModeID:       contracts.NormalizeMatchModeID(string(modeID)),
		Difficulty:   strings.TrimSpace(difficulty),
	}
	if createReq.ModeID == "" {
		createReq.ModeID = contracts.MatchModeOpenCards
	}
	if preferredSeat == "black" {
		createReq.BlackGuestID = session.Guest.GuestID
		createReq.BlackName = session.Guest.DisplayName
		createReq.BlackPlayerSecret = session.SessionSecret
		if accountSession != nil {
			createReq.BlackAccountID = accountSession.Account.AccountID
		}
	} else {
		createReq.WhiteGuestID = session.Guest.GuestID
		createReq.WhiteName = session.Guest.DisplayName
		createReq.WhitePlayerSecret = session.SessionSecret
		if accountSession != nil {
			createReq.WhiteAccountID = accountSession.Account.AccountID
		}
	}

	result := fetchGatewayJSONRequest(r, client, http.MethodPost, config.MatchServiceURL+"/api/matches", createReq)
	if result.Error != "" && result.StatusCode == 0 {
		return GatewayPrivateMatchResponse{}, http.StatusBadGateway, errors.New(result.Error)
	}
	if !result.Healthy {
		return GatewayPrivateMatchResponse{}, statusOrDefault(result.StatusCode, http.StatusBadGateway), errors.New(gatewayErrorMessage(result, "failed to create private match"))
	}
	snapshot, err := decodeGatewayPayload[contracts.MatchSnapshotResponse](result.Payload)
	if err != nil {
		return GatewayPrivateMatchResponse{}, http.StatusBadGateway, fmt.Errorf("failed to decode private match snapshot: %v", err)
	}

	seatColor := strings.ToLower(strings.TrimSpace(preferredSeat))
	if seatColor != "black" {
		seatColor = "white"
	}
	matchFields := &matchClaimBootstrapFields{
		SeatColor:    seatColor,
		PlayerSecret: session.SessionSecret,
		WhiteGuestID: strings.TrimSpace(snapshot.Match.WhiteGuestID),
		BlackGuestID: strings.TrimSpace(snapshot.Match.BlackGuestID),
		WhiteName:    strings.TrimSpace(snapshot.Match.WhiteName),
		BlackName:    strings.TrimSpace(snapshot.Match.BlackName),
		Queue:        strings.TrimSpace(snapshot.Match.Queue),
		ModeID:       string(snapshot.Match.ModeID),
		MatchStatus:  strings.TrimSpace(snapshot.Match.Status),
	}
	claim, claimErr := bootstrapMatchClaimForSide(config, client, snapshot.Match.MatchID, session, matchFields, r)
	if claimErr != "" {
		return GatewayPrivateMatchResponse{}, http.StatusBadGateway, fmt.Errorf("failed to bootstrap match claim: %s", claimErr)
	}
	if claim != nil {
		seatColor = claim.SeatColor
	}

	return GatewayPrivateMatchResponse{
		MatchID:            snapshot.Match.MatchID,
		SeatColor:          seatColor,
		WaitingForOpponent: snapshot.Match.Status == "waiting",
		Snapshot:           snapshot,
		Claim:              sanitizeSeatClaim(claim),
	}, http.StatusCreated, nil
}

func joinGatewayPrivateMatch(config GatewayConfig, client *http.Client, matchID string, request GatewayPrivateMatchRequest, r *http.Request) (GatewayPrivateMatchResponse, int, error) {
	session, statusCode, err := ensureGatewayPrivateGuestSession(config, client, request.Guest, r)
	if err != nil {
		return GatewayPrivateMatchResponse{}, statusCode, err
	}
	accountSession, _, accountSessionErr := ensureGatewayPrivateAccountSession(config, client, request.Account, session, r)
	if accountSessionErr != nil {
		log.Printf("note: account session bootstrap skipped: %v", accountSessionErr)
	}

	joinReq := contracts.JoinMatchSeatRequest{
		GuestID:       session.Guest.GuestID,
		DisplayName:   session.Guest.DisplayName,
		PlayerSecret:  session.SessionSecret,
		PreferredSeat: strings.ToLower(strings.TrimSpace(request.PreferredSeat)),
	}
	if accountSession != nil {
		joinReq.AccountID = accountSession.Account.AccountID
	}

	result := fetchGatewayJSONRequest(r, client, http.MethodPost, config.MatchServiceURL+"/api/matches/"+matchID+"/join", joinReq)
	if result.Error != "" && result.StatusCode == 0 {
		return GatewayPrivateMatchResponse{}, http.StatusBadGateway, errors.New(result.Error)
	}
	if !result.Healthy {
		return GatewayPrivateMatchResponse{}, statusOrDefault(result.StatusCode, http.StatusBadGateway), errors.New(gatewayErrorMessage(result, "failed to join private match"))
	}
	joined, err := decodeGatewayPayload[contracts.JoinMatchSeatResponse](result.Payload)
	if err != nil {
		return GatewayPrivateMatchResponse{}, http.StatusBadGateway, fmt.Errorf("failed to decode private join response: %v", err)
	}

	joinSeatColor := strings.ToLower(strings.TrimSpace(request.PreferredSeat))
	if joinSeatColor != "black" {
		joinSeatColor = "white"
	}
	joinMatchFields := &matchClaimBootstrapFields{
		SeatColor:    joinSeatColor,
		PlayerSecret: session.SessionSecret,
		WhiteGuestID: strings.TrimSpace(joined.Match.Match.WhiteGuestID),
		BlackGuestID: strings.TrimSpace(joined.Match.Match.BlackGuestID),
		WhiteName:    strings.TrimSpace(joined.Match.Match.WhiteName),
		BlackName:    strings.TrimSpace(joined.Match.Match.BlackName),
		Queue:        strings.TrimSpace(joined.Match.Match.Queue),
		ModeID:       string(joined.Match.Match.ModeID),
		MatchStatus:  strings.TrimSpace(joined.Match.Match.Status),
	}
	claim, claimErr := bootstrapMatchClaimForSide(config, client, matchID, session, joinMatchFields, r)
	if claimErr != "" {
		return GatewayPrivateMatchResponse{}, http.StatusBadGateway, fmt.Errorf("failed to bootstrap match claim: %s", claimErr)
	}

	if claim != nil {
		joinSeatColor = claim.SeatColor
	}
	return GatewayPrivateMatchResponse{
		MatchID:            joined.Match.Match.MatchID,
		SeatColor:          joinSeatColor,
		WaitingForOpponent: joined.WaitingForOpponent,
		Snapshot:           joined.Match,
		Claim:              sanitizeSeatClaim(claim),
	}, http.StatusOK, nil
}

func rematchGatewayPrivateMatch(config GatewayConfig, client *http.Client, matchID string, request GatewayPrivateMatchRequest, r *http.Request) (GatewayPrivateMatchResponse, int, error) {
	session, statusCode, err := ensureGatewayPrivateGuestSession(config, client, request.Guest, r)
	if err != nil {
		return GatewayPrivateMatchResponse{}, statusCode, err
	}
	accountSession, _, accountSessionErr := ensureGatewayPrivateAccountSession(config, client, request.Account, session, r)
	if accountSessionErr != nil {
		log.Printf("note: account session bootstrap skipped: %v", accountSessionErr)
	}

	result := fetchGatewayJSONRequest(r, client, http.MethodGet, config.MatchServiceURL+"/api/matches/"+matchID, nil)
	if result.Error != "" && result.StatusCode == 0 {
		return GatewayPrivateMatchResponse{}, http.StatusBadGateway, errors.New(result.Error)
	}
	if !result.Healthy {
		return GatewayPrivateMatchResponse{}, statusOrDefault(result.StatusCode, http.StatusBadGateway), errors.New(gatewayErrorMessage(result, "failed to load private match for rematch"))
	}
	snapshot, err := decodeGatewayPayload[contracts.MatchSnapshotResponse](result.Payload)
	if err != nil {
		return GatewayPrivateMatchResponse{}, http.StatusBadGateway, fmt.Errorf("failed to decode private match snapshot: %v", err)
	}
	if snapshot.Match.Queue != "direct" {
		return GatewayPrivateMatchResponse{}, http.StatusConflict, errors.New("rematch rooms are only available for private direct matches")
	}
	if snapshot.Match.Status != "finished" {
		return GatewayPrivateMatchResponse{}, http.StatusConflict, errors.New("rematch is only available after the private match finishes")
	}

	requesterSeat := ""
	switch session.Guest.GuestID {
	case strings.TrimSpace(snapshot.Match.WhiteGuestID):
		requesterSeat = "white"
	case strings.TrimSpace(snapshot.Match.BlackGuestID):
		requesterSeat = "black"
	default:
		return GatewayPrivateMatchResponse{}, http.StatusForbidden, errors.New("only players from the original private match can create a rematch room")
	}

	clockSeconds := request.ClockSeconds
	if clockSeconds <= 0 {
		clockSeconds = 600
	}

	return createGatewayPrivateMatchForSession(
		config,
		client,
		session,
		accountSession,
		snapshot.Match.Queue,
		snapshot.Match.ModeID,
		clockSeconds,
		requesterSeat,
		"",
		r,
	)
}

func createGatewayDirectChallenge(config GatewayConfig, client *http.Client, request GatewayDirectChallengeRequest, r *http.Request) (GatewayDirectChallengeLaunchResponse, int, error) {
	session, statusCode, err := ensureGatewayPrivateGuestSession(config, client, request.Guest, r)
	if err != nil {
		return GatewayDirectChallengeLaunchResponse{}, statusCode, err
	}
	accountSession, statusCode, err := ensureGatewayPrivateAccountSession(config, client, request.Account, session, r)
	if err != nil {
		return GatewayDirectChallengeLaunchResponse{}, statusCode, err
	}
	if accountSession == nil {
		return GatewayDirectChallengeLaunchResponse{}, http.StatusUnauthorized, errors.New("direct challenges require a signed-in account session")
	}
	targetAccountID := strings.TrimSpace(request.TargetAccountID)
	if targetAccountID == "" {
		return GatewayDirectChallengeLaunchResponse{}, http.StatusBadRequest, errors.New("target account is required")
	}

	eligibility := fetchGatewayJSONRequest(r, client, http.MethodPost, config.PlatformServiceURL+"/api/platform/challenges/eligibility", map[string]string{
		"accountId":       accountSession.Account.AccountID,
		"sessionToken":    accountSession.SessionToken,
		"targetAccountId": targetAccountID,
	})
	if !eligibility.Healthy {
		return GatewayDirectChallengeLaunchResponse{}, statusOrDefault(eligibility.StatusCode, http.StatusBadGateway), errors.New(gatewayErrorMessage(eligibility, "failed to validate direct challenge"))
	}

	matchResponse, statusCode, err := createGatewayPrivateMatch(config, client, GatewayPrivateMatchRequest{
		Guest:         request.Guest,
		Account:       request.Account,
		ModeID:        request.ModeID,
		ClockSeconds:  request.ClockSeconds,
		PreferredSeat: request.PreferredSeat,
	}, r)
	if err != nil {
		return GatewayDirectChallengeLaunchResponse{}, statusCode, err
	}

	createResult := fetchGatewayJSONRequest(r, client, http.MethodPost, config.PlatformServiceURL+"/api/platform/challenges", map[string]any{
		"accountId":       accountSession.Account.AccountID,
		"sessionToken":    accountSession.SessionToken,
		"targetAccountId": targetAccountID,
		"matchId":         matchResponse.MatchID,
		"modeId":          matchResponse.Snapshot.Match.ModeID,
		"clockSeconds":    request.ClockSeconds,
		"challengerSeat":  matchResponse.SeatColor,
	})
	if !createResult.Healthy {
		return GatewayDirectChallengeLaunchResponse{}, statusOrDefault(createResult.StatusCode, http.StatusBadGateway), errors.New(gatewayErrorMessage(createResult, "failed to persist direct challenge"))
	}
	challenge, err := decodeGatewayPayload[GatewayDirectChallengeView](createResult.Payload)
	if err != nil {
		return GatewayDirectChallengeLaunchResponse{}, http.StatusBadGateway, fmt.Errorf("failed to decode direct challenge: %v", err)
	}

	return GatewayDirectChallengeLaunchResponse{
		ChallengeID: challenge.ChallengeID,
		ModeID:      challenge.ModeID,
		Match:       matchResponse,
	}, http.StatusCreated, nil
}

func acceptGatewayDirectChallenge(config GatewayConfig, client *http.Client, challengeID string, request GatewayDirectChallengeAcceptRequest, r *http.Request) (GatewayDirectChallengeLaunchResponse, int, error) {
	session, statusCode, err := ensureGatewayPrivateGuestSession(config, client, request.Guest, r)
	if err != nil {
		return GatewayDirectChallengeLaunchResponse{}, statusCode, err
	}
	accountSession, statusCode, err := ensureGatewayPrivateAccountSession(config, client, request.Account, session, r)
	if err != nil {
		return GatewayDirectChallengeLaunchResponse{}, statusCode, err
	}
	if accountSession == nil {
		return GatewayDirectChallengeLaunchResponse{}, http.StatusUnauthorized, errors.New("direct challenges require a signed-in account session")
	}

	viewResult := fetchGatewayJSONRequest(r, client, http.MethodPost, config.PlatformServiceURL+"/api/platform/challenges/"+challengeID+"/view", map[string]string{
		"accountId":    accountSession.Account.AccountID,
		"sessionToken": accountSession.SessionToken,
	})
	if !viewResult.Healthy {
		return GatewayDirectChallengeLaunchResponse{}, statusOrDefault(viewResult.StatusCode, http.StatusBadGateway), errors.New(gatewayErrorMessage(viewResult, "failed to load direct challenge"))
	}
	challenge, err := decodeGatewayPayload[GatewayDirectChallengeView](viewResult.Payload)
	if err != nil {
		return GatewayDirectChallengeLaunchResponse{}, http.StatusBadGateway, fmt.Errorf("failed to decode direct challenge: %v", err)
	}
	if challenge.Status != "pending" {
		return GatewayDirectChallengeLaunchResponse{}, http.StatusConflict, errors.New("direct challenge is no longer pending")
	}

	matchResponse, statusCode, err := joinGatewayPrivateMatch(config, client, challenge.MatchID, GatewayPrivateMatchRequest{
		Guest:   request.Guest,
		Account: request.Account,
	}, r)
	if err != nil {
		return GatewayDirectChallengeLaunchResponse{}, statusCode, err
	}

	respondResult := fetchGatewayJSONRequest(r, client, http.MethodPost, config.PlatformServiceURL+"/api/platform/challenges/"+challengeID+"/respond", map[string]any{
		"accountId":    accountSession.Account.AccountID,
		"sessionToken": accountSession.SessionToken,
		"accept":       true,
	})
	if !respondResult.Healthy {
		return GatewayDirectChallengeLaunchResponse{}, statusOrDefault(respondResult.StatusCode, http.StatusBadGateway), errors.New(gatewayErrorMessage(respondResult, "failed to accept direct challenge"))
	}

	return GatewayDirectChallengeLaunchResponse{
		ChallengeID: challenge.ChallengeID,
		ModeID:      challenge.ModeID,
		Match:       matchResponse,
	}, http.StatusOK, nil
}

func ensureGatewayPrivateGuestSession(config GatewayConfig, client *http.Client, identity GatewayGuestIdentity, r *http.Request) (*platform.GuestSession, int, error) {
	session, errMessage := bootstrapGuestSessionForSide(config, client, &identity, r)
	if session != nil {
		return session, http.StatusOK, nil
	}
	if errMessage == "" {
		return nil, http.StatusBadRequest, errors.New("failed to bootstrap guest session")
	}
	if strings.Contains(strings.ToLower(errMessage), "unauthorized") {
		return nil, http.StatusUnauthorized, errors.New(errMessage)
	}
	if strings.Contains(strings.ToLower(errMessage), "unknown guest") {
		return nil, http.StatusNotFound, errors.New(errMessage)
	}
	return nil, http.StatusBadGateway, errors.New(errMessage)
}

func ensureGatewayPrivateAccountSession(config GatewayConfig, client *http.Client, identity *GatewayAccountIdentity, guestSession *platform.GuestSession, r *http.Request) (*platform.AccountSession, int, error) {
	if identity == nil || strings.TrimSpace(identity.AccountID) == "" {
		return nil, http.StatusOK, nil
	}
	session, errMessage := bootstrapAccountSessionForSide(config, client, identity, guestSession, r)
	if session != nil {
		return session, http.StatusOK, nil
	}
	if errMessage == "" {
		return nil, http.StatusBadGateway, errors.New("failed to bootstrap account session")
	}
	if strings.Contains(strings.ToLower(errMessage), "unauthorized") {
		return nil, http.StatusUnauthorized, errors.New(errMessage)
	}
	return nil, http.StatusBadGateway, errors.New(errMessage)
}

func proxyGatewayIntent(w http.ResponseWriter, r *http.Request, config GatewayConfig, client *http.Client, matchID string) {
	if !isValidPathParam(matchID) {
		httputil.WriteError(w, http.StatusBadRequest, "invalid match id")
		return
	}
	var req contracts.ApplyIntentRequest
	if r.Body != nil {
		defer r.Body.Close()
		r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	req.Intent.MatchID = matchID
	if strings.TrimSpace(req.Intent.PlayerClaimToken) != "" {
		claim, errMessage := resolveGatewayClaimByToken(config, client, matchID, strings.TrimSpace(req.Intent.PlayerClaimToken), r)
		if errMessage != "" {
			httputil.WriteError(w, http.StatusUnauthorized, errMessage)
			return
		}
		req.Intent.PlayerID = claim.PlayerID
		req.Intent.PlayerSecret = claim.PlayerSecret
		req.Intent.PlayerClaimToken = ""
	}

	result := fetchGatewayJSONRequest(r, client, http.MethodPost, config.MatchServiceURL+"/api/matches/"+matchID+"/intents", req)
	if result.Error != "" && result.StatusCode == 0 {
		httputil.WriteError(w, http.StatusBadGateway, result.Error)
		return
	}
	httputil.WriteJSON(w, statusOrDefault(result.StatusCode, http.StatusBadGateway), result.Payload)
}

func proxyGatewayPresence(w http.ResponseWriter, r *http.Request, config GatewayConfig, client *http.Client, matchID string) {
	if !isValidPathParam(matchID) {
		httputil.WriteError(w, http.StatusBadRequest, "invalid match id")
		return
	}
	var req contracts.MatchPresenceRequest
	if r.Body != nil {
		defer r.Body.Close()
		r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	if strings.TrimSpace(req.PlayerClaimToken) != "" {
		claim, errMessage := resolveGatewayClaimByToken(config, client, matchID, strings.TrimSpace(req.PlayerClaimToken), r)
		if errMessage != "" {
			httputil.WriteError(w, http.StatusUnauthorized, errMessage)
			return
		}
		req.PlayerID = claim.PlayerID
		req.PlayerSecret = claim.PlayerSecret
		req.PlayerClaimToken = ""
	}

	result := fetchGatewayJSONRequest(r, client, http.MethodPost, config.MatchServiceURL+"/api/matches/"+matchID+"/presence", req)
	if result.Error != "" && result.StatusCode == 0 {
		httputil.WriteError(w, http.StatusBadGateway, result.Error)
		return
	}
	if result.StatusCode == http.StatusNoContent {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	httputil.WriteJSON(w, statusOrDefault(result.StatusCode, http.StatusBadGateway), result.Payload)
}

func fetchGatewayJSON(r *http.Request, client *http.Client, url string) GatewayServiceHealth {
	return fetchGatewayJSONRequest(r, client, http.MethodGet, url, nil)
}

func fetchGatewayJSONRequest(r *http.Request, client *http.Client, method, url string, payload any) GatewayServiceHealth {
	var ctx context.Context
	if r != nil {
		ctx = r.Context()
	} else {
		ctx = context.Background()
	}
	return fetchGatewayJSONRequestWithContext(ctx, client, method, url, payload)
}

func fetchGatewayJSONRequestWithContext(ctx context.Context, client *http.Client, method, url string, payload any) GatewayServiceHealth {
	var body *bytes.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return GatewayServiceHealth{URL: url, Error: err.Error()}
		}
		body = bytes.NewReader(encoded)
	} else {
		body = bytes.NewReader(nil)
	}

	request, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return GatewayServiceHealth{URL: url, Error: err.Error()}
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	// Set the Origin header on the outgoing request to match the public
	// origin of the incoming request. The destination service's CSRF
	// middleware compares the Origin against its allow-list, so it needs a
	// clean origin (no path) to match. Without this, server-to-server
	// POSTs from the gateway arrive with no Origin and are rejected with
	// 403 (CSRF check failed: origin header required). Note: the browser
	// does not send Origin for same-origin requests (only Referer with a
	// path), and Referer-with-path does not equal the bare origin in the
	// allow-list, so we always reconstruct the origin from the source
	// request's host information.
	if source := sourceRequestFromContext(ctx); source != nil {
		if origin := reconstructPublicOrigin(source); origin != "" {
			request.Header.Set("Origin", origin)
		}
	}

	response, err := client.Do(request)
	if err != nil {
		return GatewayServiceHealth{URL: url, Error: err.Error()}
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNoContent {
		return GatewayServiceHealth{
			URL:        url,
			Healthy:    true,
			StatusCode: response.StatusCode,
		}
	}

	var responsePayload any
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&responsePayload); err != nil {
		return GatewayServiceHealth{
			URL:        url,
			StatusCode: response.StatusCode,
			Error:      fmt.Sprintf("invalid json: %v", err),
		}
	}

	return GatewayServiceHealth{
		URL:        url,
		Healthy:    response.StatusCode >= 200 && response.StatusCode < 300,
		StatusCode: response.StatusCode,
		Payload:    responsePayload,
	}
}

func gatewayErrorMessage(status GatewayServiceHealth, fallback string) string {
	if payload, ok := status.Payload.(map[string]any); ok {
		if message, ok := payload["error"].(string); ok && message != "" {
			return message
		}
	}
	return fallback
}

func statusOrDefault(statusCode int, fallback int) int {
	if statusCode == 0 {
		return fallback
	}
	return statusCode
}

func decodeGatewayPayload[T any](payload any) (T, error) {
	var decoded T
	raw, err := json.Marshal(payload)
	if err != nil {
		return decoded, err
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return decoded, err
	}
	return decoded, nil
}

func bootstrapMessage(status GatewaySystemStatus) string {
	if status.Status == "ok" {
		return "Gateway online. Match, platform, and matchmaking services are ready."
	}

	problems := make([]string, 0, len(status.Services))
	for name, service := range status.Services {
		if !service.Healthy {
			problems = append(problems, name)
		}
	}

	if len(problems) == 0 {
		return "Gateway online."
	}
	return "Gateway online, but some backend services are degraded: " + strings.Join(problems, ", ")
}

func gatewayConfigFromEnv() GatewayConfig {
	return GatewayConfig{
		MatchServiceURL:       resolveInternalServiceURL("MATCH_SERVICE_INTERNAL_URL", "http://match-service:8080"),
		PlatformServiceURL:    resolveInternalServiceURL("PLATFORM_SERVICE_INTERNAL_URL", "http://platform-service:8080"),
		MatchmakingServiceURL: resolveInternalServiceURL("MATCHMAKING_SERVICE_INTERNAL_URL", "http://matchmaking-service:8080"),
	}
}

// resolveInternalServiceURL returns the value of envKey, falling back to
// defaultURL if the env var is missing, blank, or contains an unresolved
// Railway template (${{...}}). When the env var is set to a hostname-only
// Railway internal URL (e.g., "http://match-service.railway.internal:" with
// a trailing colon but no port), the function appends ":8080" so the
// resulting URL is valid. Services in this repo listen on port 8080.
func resolveInternalServiceURL(envKey string, defaultURL string) string {
	u := strings.TrimSpace(os.Getenv(envKey))
	if u == "" {
		return defaultURL
	}
	// Unresolved Railway template references (e.g., when the env var was
	// set to "${{match-service.RAILWAY_PRIVATE_DOMAIN}}" but the
	// referenced variable does not exist on the project). Using the literal
	// template as a URL would fail with a confusing connection error.
	if strings.Contains(u, "${{") {
		return defaultURL
	}
	// Hostname with no port (e.g., "http://match-service.railway.internal:"
	// from a misconfigured Railway variable). Append the default port so
	// the URL is valid.
	if strings.HasSuffix(u, ":") {
		u += "8080"
	}
	return u
}



func sanitizeSeatClaim(claim *GatewaySeatClaim) *GatewaySeatClaim {
	if claim == nil {
		return nil
	}
	return &GatewaySeatClaim{
		MatchID:      claim.MatchID,
		GuestID:      claim.GuestID,
		SeatColor:    claim.SeatColor,
		PlayerID:     claim.PlayerID,
		ExpiresAt:    claim.ExpiresAt,
		Queue:        claim.Queue,
		ModeID:       claim.ModeID,
		WhiteGuestID: claim.WhiteGuestID,
		BlackGuestID: claim.BlackGuestID,
		WhiteName:    claim.WhiteName,
		BlackName:    claim.BlackName,
		Status:       claim.Status,
	}
}
