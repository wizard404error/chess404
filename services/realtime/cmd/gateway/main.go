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
	envutil.Require("MATCH_SERVICE_INTERNAL_URL", "PLATFORM_SERVICE_INTERNAL_URL", "MATCHMAKING_SERVICE_INTERNAL_URL")
	config := gatewayConfigFromEnv()
	client := &http.Client{Timeout: 3 * time.Second}
	mux := buildGatewayMux(config, client)
	rl := rate_limit.New()

	addr := httputil.ListenAddr("GATEWAY_ADDR", 8080)
	srv := &http.Server{
		Addr:              addr,
		Handler:           rate_limit.CSRFMiddleware(rl.Middleware(rate_limit.DefaultAPIWindow, rate_limit.DefaultAPILimit)(mux), httputil.ParseAllowedOrigins()),
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

	mux.HandleFunc("/api/system/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			httputil.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, collectGatewayStatus(config, client))
	})

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
			payload = buildGatewayBootstrapPayload(config, client, request)
		} else {
			payload = buildGatewayBootstrapPayload(config, client, GatewayBootstrapRequest{})
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
		response, statusCode, err := createGatewayPrivateMatch(config, client, payload)
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
			response, statusCode, err = joinGatewayPrivateMatch(config, client, parts[0], payload)
		} else {
			response, statusCode, err = rematchGatewayPrivateMatch(config, client, parts[0], payload)
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
		response, statusCode, err := createGatewayDirectChallenge(config, client, payload)
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
		response, statusCode, err := acceptGatewayDirectChallenge(config, client, parts[0], payload)
		if err != nil {
			httputil.WriteError(w, statusCode, err.Error())
			return
		}
		httputil.WriteJSON(w, http.StatusOK, response)
	})

	return mux
}

func collectGatewayStatus(config GatewayConfig, client *http.Client) GatewaySystemStatus {
	services := map[string]GatewayServiceHealth{
		"match":       fetchGatewayJSON(client, config.MatchServiceURL+"/api/system/status"),
		"platform":    fetchGatewayJSON(client, config.PlatformServiceURL+"/api/platform/status"),
		"matchmaking": fetchGatewayJSON(client, config.MatchmakingServiceURL+"/api/status"),
	}

	status := "ok"
	for name := range services {
		if !services[name].Healthy {
			status = "degraded"
		}
		service := services[name]
		service.URL = ""
		service.Payload = nil
		services[name] = service
	}

	return GatewaySystemStatus{
		Status:    status,
		Service:   "gateway",
		CheckedAt: time.Now().UTC(),
		Services:  services,
	}
}

func buildGatewayBootstrapPayload(config GatewayConfig, client *http.Client, request GatewayBootstrapRequest) GatewayBootstrapPayload {
	systemStatus := collectGatewayStatus(config, client)
	capabilities := fetchGatewayJSON(client, config.PlatformServiceURL+"/api/platform/capabilities")
	defaultQueue := fetchGatewayJSON(client, config.MatchmakingServiceURL+"/api/queues/default")
	guestSessions, sessionErrors := bootstrapGuestSessions(config, client, request)
	matchClaims, claimErrors := bootstrapMatchClaims(config, client, request.MatchID, guestSessions)
	accountSessions, accountErrors := bootstrapAccountSessions(config, client, request, guestSessions)
	queueTickets, queueErrors := bootstrapQueueTickets(config, client, guestSessions, accountSessions)
	recoveredMatch, recoveredMatchErrors := bootstrapRecoveredMatch(config, client, guestSessions, queueTickets)

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

func bootstrapGuestSessions(config GatewayConfig, client *http.Client, request GatewayBootstrapRequest) (*GatewayBootstrapGuestSessions, *GatewayBootstrapErrors) {
	sessions := &GatewayBootstrapGuestSessions{}
	errors := &GatewayBootstrapErrors{}

	if session, errMessage := bootstrapGuestSessionForSide(config, client, request.White); session != nil {
		sessions.White = session
	} else if errMessage != "" {
		errors.White = errMessage
	}

	if session, errMessage := bootstrapGuestSessionForSide(config, client, request.Black); session != nil {
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

func bootstrapGuestSessionForSide(config GatewayConfig, client *http.Client, identity *GatewayGuestIdentity) (*platform.GuestSession, string) {
	result := fetchGatewayJSONRequest(client, http.MethodPost, config.PlatformServiceURL+"/api/platform/guest-sessions", identity)
	if !result.Healthy && result.StatusCode == http.StatusUnauthorized && identity != nil && (identity.GuestID != "" || identity.SessionSecret != "") {
		result = fetchGatewayJSONRequest(client, http.MethodPost, config.PlatformServiceURL+"/api/platform/guest-sessions", GatewayGuestIdentity{})
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

func bootstrapMatchClaims(config GatewayConfig, client *http.Client, matchID string, sessions *GatewayBootstrapGuestSessions) (*GatewayBootstrapMatchClaims, *GatewayBootstrapErrors) {
	if matchID == "" || sessions == nil {
		return nil, nil
	}

	claims := &GatewayBootstrapMatchClaims{}
	errors := &GatewayBootstrapErrors{}

	if claim, errMessage := bootstrapMatchClaimForSide(config, client, matchID, sessions.White, nil); claim != nil {
		claims.White = sanitizeSeatClaim(claim)
	} else if errMessage != "" {
		errors.White = errMessage
	}

	if claim, errMessage := bootstrapMatchClaimForSide(config, client, matchID, sessions.Black, nil); claim != nil {
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

func bootstrapMatchClaimForSide(config GatewayConfig, client *http.Client, matchID string, session *platform.GuestSession, matchFields *matchClaimBootstrapFields) (*GatewaySeatClaim, string) {
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
	result := fetchGatewayJSONRequest(client, http.MethodPost, config.PlatformServiceURL+"/api/platform/match-claims", body)
	if !result.Healthy {
		return nil, gatewayErrorMessage(result, "failed to recover match claim")
	}

	claim, err := decodeGatewayPayload[GatewaySeatClaim](result.Payload)
	if err != nil {
		return nil, fmt.Sprintf("failed to decode match claim: %v", err)
	}
	return &claim, ""
}

func bootstrapAccountSessions(config GatewayConfig, client *http.Client, request GatewayBootstrapRequest, guestSessions *GatewayBootstrapGuestSessions) (*GatewayBootstrapAccountSessions, *GatewayBootstrapErrors) {
	sessions := &GatewayBootstrapAccountSessions{}
	errors := &GatewayBootstrapErrors{}

	if session, errMessage := bootstrapAccountSessionForSide(config, client, request.WhiteAccount, guestSessionsSide(guestSessions, "white")); session != nil {
		sessions.White = session
	} else if errMessage != "" {
		errors.White = errMessage
	}

	if session, errMessage := bootstrapAccountSessionForSide(config, client, request.BlackAccount, guestSessionsSide(guestSessions, "black")); session != nil {
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
) (*GatewayBootstrapQueueTickets, *GatewayBootstrapErrors) {
	tickets := &GatewayBootstrapQueueTickets{}
	errors := &GatewayBootstrapErrors{}

	if ticket, errMessage := bootstrapQueueTicketForSide(config, client, guestSessionsSide(guestSessions, "white"), accountSessionsSide(accountSessions, "white")); ticket != nil {
		tickets.White = ticket
	} else if errMessage != "" {
		errors.White = errMessage
	}

	if ticket, errMessage := bootstrapQueueTicketForSide(config, client, guestSessionsSide(guestSessions, "black"), accountSessionsSide(accountSessions, "black")); ticket != nil {
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

	result := fetchGatewayJSON(client, config.MatchmakingServiceURL+"/api/queues/tickets?"+params.Encode())
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
) (*GatewayBootstrapRecoveredMatch, *GatewayBootstrapErrors) {
	claims, errors := bootstrapActiveMatchClaims(config, client, guestSessions)
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
) (*GatewayBootstrapMatchClaims, *GatewayBootstrapErrors) {
	claims := &GatewayBootstrapMatchClaims{}
	errors := &GatewayBootstrapErrors{}

	if claim, errMessage := bootstrapActiveMatchClaimForSide(config, client, guestSessionsSide(guestSessions, "white")); claim != nil {
		claims.White = claim
	} else if errMessage != "" {
		errors.White = errMessage
	}

	if claim, errMessage := bootstrapActiveMatchClaimForSide(config, client, guestSessionsSide(guestSessions, "black")); claim != nil {
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

	result := fetchGatewayJSONRequest(client, http.MethodPost, config.PlatformServiceURL+"/api/platform/match-claims/active", payload)
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

func bootstrapAccountSessionForSide(config GatewayConfig, client *http.Client, identity *GatewayAccountIdentity, guestSession *platform.GuestSession) (*platform.AccountSession, string) {
	if identity == nil || strings.TrimSpace(identity.AccountID) == "" {
		return nil, ""
	}

	result := fetchGatewayJSONRequest(client, http.MethodPost, config.PlatformServiceURL+"/api/platform/account-sessions", map[string]string{
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
		reclaimed, errMessage := reclaimGatewayAccountSession(config, client, strings.TrimSpace(identity.AccountID), guestSession)
		if reclaimed != nil || errMessage != "" {
			return reclaimed, errMessage
		}
	}

	return nil, gatewayErrorMessage(result, "failed to bootstrap account session")
}

func reclaimGatewayAccountSession(config GatewayConfig, client *http.Client, accountID string, guestSession *platform.GuestSession) (*platform.AccountSession, string) {
	accountResult := fetchGatewayJSON(client, config.PlatformServiceURL+"/api/platform/accounts/"+accountID)
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

	claimResult := fetchGatewayJSONRequest(client, http.MethodPost, config.PlatformServiceURL+"/api/platform/accounts/claim", claimPayload)
	if !claimResult.Healthy {
		return nil, gatewayErrorMessage(claimResult, "failed to reclaim account session")
	}
	session, err := decodeGatewayPayload[platform.AccountSession](claimResult.Payload)
	if err != nil {
		return nil, fmt.Sprintf("failed to decode reclaimed account session: %v", err)
	}
	return &session, ""
}

func resolveGatewayClaimByToken(config GatewayConfig, client *http.Client, matchID, claimToken string) (*GatewaySeatClaim, string) {
	result := fetchGatewayJSONRequest(client, http.MethodPost, config.PlatformServiceURL+"/api/platform/match-claims/resolve", map[string]string{
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

func createGatewayPrivateMatch(config GatewayConfig, client *http.Client, request GatewayPrivateMatchRequest) (GatewayPrivateMatchResponse, int, error) {
	session, statusCode, err := ensureGatewayPrivateGuestSession(config, client, request.Guest)
	if err != nil {
		return GatewayPrivateMatchResponse{}, statusCode, err
	}
	accountSession, _, accountSessionErr := ensureGatewayPrivateAccountSession(config, client, request.Account, session)
	if accountSessionErr != nil {
		log.Printf("note: account session bootstrap skipped: %v", accountSessionErr)
	}
	return createGatewayPrivateMatchForSession(config, client, session, accountSession, request.Queue, request.ModeID, request.ClockSeconds, request.PreferredSeat)
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

	result := fetchGatewayJSONRequest(client, http.MethodPost, config.MatchServiceURL+"/api/matches", createReq)
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
	claim, claimErr := bootstrapMatchClaimForSide(config, client, snapshot.Match.MatchID, session, matchFields)
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

func joinGatewayPrivateMatch(config GatewayConfig, client *http.Client, matchID string, request GatewayPrivateMatchRequest) (GatewayPrivateMatchResponse, int, error) {
	session, statusCode, err := ensureGatewayPrivateGuestSession(config, client, request.Guest)
	if err != nil {
		return GatewayPrivateMatchResponse{}, statusCode, err
	}
	accountSession, _, accountSessionErr := ensureGatewayPrivateAccountSession(config, client, request.Account, session)
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

	result := fetchGatewayJSONRequest(client, http.MethodPost, config.MatchServiceURL+"/api/matches/"+matchID+"/join", joinReq)
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
	claim, claimErr := bootstrapMatchClaimForSide(config, client, matchID, session, joinMatchFields)
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

func rematchGatewayPrivateMatch(config GatewayConfig, client *http.Client, matchID string, request GatewayPrivateMatchRequest) (GatewayPrivateMatchResponse, int, error) {
	session, statusCode, err := ensureGatewayPrivateGuestSession(config, client, request.Guest)
	if err != nil {
		return GatewayPrivateMatchResponse{}, statusCode, err
	}
	accountSession, _, accountSessionErr := ensureGatewayPrivateAccountSession(config, client, request.Account, session)
	if accountSessionErr != nil {
		log.Printf("note: account session bootstrap skipped: %v", accountSessionErr)
	}

	result := fetchGatewayJSONRequest(client, http.MethodGet, config.MatchServiceURL+"/api/matches/"+matchID, nil)
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
	)
}

func createGatewayDirectChallenge(config GatewayConfig, client *http.Client, request GatewayDirectChallengeRequest) (GatewayDirectChallengeLaunchResponse, int, error) {
	session, statusCode, err := ensureGatewayPrivateGuestSession(config, client, request.Guest)
	if err != nil {
		return GatewayDirectChallengeLaunchResponse{}, statusCode, err
	}
	accountSession, statusCode, err := ensureGatewayPrivateAccountSession(config, client, request.Account, session)
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

	eligibility := fetchGatewayJSONRequest(client, http.MethodPost, config.PlatformServiceURL+"/api/platform/challenges/eligibility", map[string]string{
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
	})
	if err != nil {
		return GatewayDirectChallengeLaunchResponse{}, statusCode, err
	}

	createResult := fetchGatewayJSONRequest(client, http.MethodPost, config.PlatformServiceURL+"/api/platform/challenges", map[string]any{
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

func acceptGatewayDirectChallenge(config GatewayConfig, client *http.Client, challengeID string, request GatewayDirectChallengeAcceptRequest) (GatewayDirectChallengeLaunchResponse, int, error) {
	session, statusCode, err := ensureGatewayPrivateGuestSession(config, client, request.Guest)
	if err != nil {
		return GatewayDirectChallengeLaunchResponse{}, statusCode, err
	}
	accountSession, statusCode, err := ensureGatewayPrivateAccountSession(config, client, request.Account, session)
	if err != nil {
		return GatewayDirectChallengeLaunchResponse{}, statusCode, err
	}
	if accountSession == nil {
		return GatewayDirectChallengeLaunchResponse{}, http.StatusUnauthorized, errors.New("direct challenges require a signed-in account session")
	}

	viewResult := fetchGatewayJSONRequest(client, http.MethodPost, config.PlatformServiceURL+"/api/platform/challenges/"+challengeID+"/view", map[string]string{
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
	})
	if err != nil {
		return GatewayDirectChallengeLaunchResponse{}, statusCode, err
	}

	respondResult := fetchGatewayJSONRequest(client, http.MethodPost, config.PlatformServiceURL+"/api/platform/challenges/"+challengeID+"/respond", map[string]any{
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

func ensureGatewayPrivateGuestSession(config GatewayConfig, client *http.Client, identity GatewayGuestIdentity) (*platform.GuestSession, int, error) {
	session, errMessage := bootstrapGuestSessionForSide(config, client, &identity)
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

func ensureGatewayPrivateAccountSession(config GatewayConfig, client *http.Client, identity *GatewayAccountIdentity, guestSession *platform.GuestSession) (*platform.AccountSession, int, error) {
	if identity == nil || strings.TrimSpace(identity.AccountID) == "" {
		return nil, http.StatusOK, nil
	}
	session, errMessage := bootstrapAccountSessionForSide(config, client, identity, guestSession)
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
		claim, errMessage := resolveGatewayClaimByToken(config, client, matchID, strings.TrimSpace(req.Intent.PlayerClaimToken))
		if errMessage != "" {
			httputil.WriteError(w, http.StatusUnauthorized, errMessage)
			return
		}
		req.Intent.PlayerID = claim.PlayerID
		req.Intent.PlayerSecret = claim.PlayerSecret
		req.Intent.PlayerClaimToken = ""
	}

	result := fetchGatewayJSONRequest(client, http.MethodPost, config.MatchServiceURL+"/api/matches/"+matchID+"/intents", req)
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
		claim, errMessage := resolveGatewayClaimByToken(config, client, matchID, strings.TrimSpace(req.PlayerClaimToken))
		if errMessage != "" {
			httputil.WriteError(w, http.StatusUnauthorized, errMessage)
			return
		}
		req.PlayerID = claim.PlayerID
		req.PlayerSecret = claim.PlayerSecret
		req.PlayerClaimToken = ""
	}

	result := fetchGatewayJSONRequest(client, http.MethodPost, config.MatchServiceURL+"/api/matches/"+matchID+"/presence", req)
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

func fetchGatewayJSON(client *http.Client, url string) GatewayServiceHealth {
	return fetchGatewayJSONRequest(client, http.MethodGet, url, nil)
}

func fetchGatewayJSONRequest(client *http.Client, method, url string, payload any) GatewayServiceHealth {
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

	request, err := http.NewRequest(method, url, body)
	if err != nil {
		return GatewayServiceHealth{URL: url, Error: err.Error()}
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
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
	if status.Error != "" {
		return status.Error
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

func resolveInternalServiceURL(envKey string, defaultURL string) string {
	if u := os.Getenv(envKey); u != "" {
		return u
	}
	return defaultURL
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
