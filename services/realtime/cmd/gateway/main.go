package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/chess404/realtime/internal/contracts"
	"github.com/chess404/realtime/internal/platform"
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
	MatchID      string    `json:"matchId"`
	GuestID      string    `json:"guestId"`
	SeatColor    string    `json:"seatColor"`
	PlayerID     string    `json:"playerId"`
	PlayerSecret string    `json:"playerSecret"`
	ClaimToken   string    `json:"claimToken,omitempty"`
	ExpiresAt    time.Time `json:"expiresAt,omitempty"`
	Queue        string    `json:"queue,omitempty"`
	WhiteGuestID string    `json:"whiteGuestId,omitempty"`
	BlackGuestID string    `json:"blackGuestId,omitempty"`
	WhiteName    string    `json:"whiteName,omitempty"`
	BlackName    string    `json:"blackName,omitempty"`
	Status       string    `json:"status,omitempty"`
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

type GatewayBootstrapErrors struct {
	White string `json:"white,omitempty"`
	Black string `json:"black,omitempty"`
}

type GatewayBootstrapPayload struct {
	Status             string                           `json:"status"`
	RealtimeReady      bool                             `json:"realtimeReady"`
	PlatformReady      bool                             `json:"platformReady"`
	MatchmakingReady   bool                             `json:"matchmakingReady"`
	Authoritative      bool                             `json:"authoritative"`
	Services           map[string]GatewayServiceHealth  `json:"services"`
	ServiceEndpoints   GatewayConfig                    `json:"serviceEndpoints"`
	PlatformCaps       any                              `json:"platformCaps,omitempty"`
	DefaultQueue       any                              `json:"defaultQueue,omitempty"`
	GuestSessions      *GatewayBootstrapGuestSessions   `json:"guestSessions,omitempty"`
	MatchClaims        *GatewayBootstrapMatchClaims     `json:"matchClaims,omitempty"`
	AccountSessions    *GatewayBootstrapAccountSessions `json:"accountSessions,omitempty"`
	SessionErrors      *GatewayBootstrapErrors          `json:"sessionErrors,omitempty"`
	ClaimErrors        *GatewayBootstrapErrors          `json:"claimErrors,omitempty"`
	AccountErrors      *GatewayBootstrapErrors          `json:"accountErrors,omitempty"`
	RequestedMatchID   string                           `json:"requestedMatchId,omitempty"`
	BootstrapCheckedAt time.Time                        `json:"bootstrapCheckedAt"`
	Message            string                           `json:"message"`
}

func main() {
	config := gatewayConfigFromEnv()
	client := &http.Client{Timeout: 3 * time.Second}
	mux := buildGatewayMux(config, client)

	addr := listenAddr("GATEWAY_ADDR", 8080)
	log.Printf("gateway listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
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
		writeJSON(w, http.StatusOK, map[string]any{
			"status":    "ok",
			"service":   "gateway",
			"checkedAt": time.Now().UTC(),
		})
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":    "ok",
			"service":   "gateway",
			"checkedAt": time.Now().UTC(),
		})
	})

	mux.HandleFunc("/api/system/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		writeJSON(w, http.StatusOK, collectGatewayStatus(config, client))
	})

	mux.HandleFunc("/api/session/bootstrap", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var payload GatewayBootstrapPayload
		if r.Method == http.MethodPost {
			var request GatewayBootstrapRequest
			if r.Body != nil {
				defer r.Body.Close()
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					writeError(w, http.StatusBadRequest, "invalid bootstrap payload")
					return
				}
			}
			payload = buildGatewayBootstrapPayload(config, client, request)
		} else {
			payload = buildGatewayBootstrapPayload(config, client, GatewayBootstrapRequest{})
		}

		writeJSON(w, http.StatusOK, contracts.Envelope{
			Type:    "gateway.bootstrap",
			Payload: payload,
		})
	})

	mux.HandleFunc("/api/matches/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/matches/")
		if path == "" {
			writeError(w, http.StatusNotFound, "match id required")
			return
		}
		parts := strings.Split(path, "/")
		if len(parts) != 2 || parts[1] != "intents" {
			writeError(w, http.StatusNotFound, "route not found")
			return
		}
		proxyGatewayIntent(w, r, config, client, parts[0])
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
	for _, service := range services {
		if !service.Healthy {
			status = "degraded"
			break
		}
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

	return GatewayBootstrapPayload{
		Status:             systemStatus.Status,
		RealtimeReady:      systemStatus.Services["match"].Healthy,
		PlatformReady:      systemStatus.Services["platform"].Healthy,
		MatchmakingReady:   systemStatus.Services["matchmaking"].Healthy,
		Authoritative:      systemStatus.Services["match"].Healthy,
		Services:           systemStatus.Services,
		ServiceEndpoints:   config,
		PlatformCaps:       capabilities.Payload,
		DefaultQueue:       defaultQueue.Payload,
		GuestSessions:      guestSessions,
		MatchClaims:        matchClaims,
		AccountSessions:    accountSessions,
		SessionErrors:      sessionErrors,
		ClaimErrors:        claimErrors,
		AccountErrors:      accountErrors,
		RequestedMatchID:   request.MatchID,
		BootstrapCheckedAt: time.Now().UTC(),
		Message:            bootstrapMessage(systemStatus),
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

	if claim, errMessage := bootstrapMatchClaimForSide(config, client, matchID, sessions.White); claim != nil {
		claims.White = claim
	} else if errMessage != "" {
		errors.White = errMessage
	}

	if claim, errMessage := bootstrapMatchClaimForSide(config, client, matchID, sessions.Black); claim != nil {
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

func bootstrapMatchClaimForSide(config GatewayConfig, client *http.Client, matchID string, session *platform.GuestSession) (*GatewaySeatClaim, string) {
	if session == nil || session.Guest.GuestID == "" || session.SessionSecret == "" {
		return nil, ""
	}

	result := fetchGatewayJSONRequest(client, http.MethodPost, config.PlatformServiceURL+"/api/platform/match-claims", map[string]string{
		"matchId":       matchID,
		"guestId":       session.Guest.GuestID,
		"sessionSecret": session.SessionSecret,
	})
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

func guestSessionsSide(sessions *GatewayBootstrapGuestSessions, side string) *platform.GuestSession {
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

func proxyGatewayIntent(w http.ResponseWriter, r *http.Request, config GatewayConfig, client *http.Client, matchID string) {
	var req contracts.ApplyIntentRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	req.Intent.MatchID = matchID
	if strings.TrimSpace(req.Intent.PlayerClaimToken) != "" {
		claim, errMessage := resolveGatewayClaimByToken(config, client, matchID, strings.TrimSpace(req.Intent.PlayerClaimToken))
		if errMessage != "" {
			writeError(w, http.StatusUnauthorized, errMessage)
			return
		}
		req.Intent.PlayerID = claim.PlayerID
		req.Intent.PlayerSecret = claim.PlayerSecret
		req.Intent.PlayerClaimToken = ""
	}

	result := fetchGatewayJSONRequest(client, http.MethodPost, config.MatchServiceURL+"/api/matches/"+matchID+"/intents", req)
	if result.Error != "" && result.StatusCode == 0 {
		writeError(w, http.StatusBadGateway, result.Error)
		return
	}
	writeJSON(w, statusOrDefault(result.StatusCode, http.StatusBadGateway), result.Payload)
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

	var responsePayload any
	if err := json.NewDecoder(response.Body).Decode(&responsePayload); err != nil {
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
		MatchServiceURL:       strings.TrimRight(envOrDefault("MATCH_SERVICE_INTERNAL_URL", "http://127.0.0.1:8082"), "/"),
		PlatformServiceURL:    strings.TrimRight(envOrDefault("PLATFORM_SERVICE_INTERNAL_URL", "http://127.0.0.1:8083"), "/"),
		MatchmakingServiceURL: strings.TrimRight(envOrDefault("MATCHMAKING_SERVICE_INTERNAL_URL", "http://127.0.0.1:8084"), "/"),
	}
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
