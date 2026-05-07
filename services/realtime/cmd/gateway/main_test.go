package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chess404/realtime/internal/contracts"
)

func TestGatewayStatusAggregatesHealthyServices(t *testing.T) {
	matchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "service": "match-service"})
	}))
	defer matchServer.Close()

	platformServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/platform/status":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "service": "platform-service"})
		case "/api/platform/capabilities":
			_ = json.NewEncoder(w).Encode(map[string]any{"profiles": true})
		default:
			http.NotFound(w, r)
		}
	}))
	defer platformServer.Close()

	matchmakingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/status":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "service": "matchmaking-service"})
		case "/api/queues/default":
			_ = json.NewEncoder(w).Encode(map[string]any{"queue": "rated"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer matchmakingServer.Close()

	mux := buildGatewayMux(GatewayConfig{
		MatchServiceURL:       matchServer.URL,
		PlatformServiceURL:    platformServer.URL,
		MatchmakingServiceURL: matchmakingServer.URL,
	}, matchServer.Client())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/system/status", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected gateway status to succeed, got %d", rec.Code)
	}

	var payload GatewaySystemStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected gateway status to decode, got %v", err)
	}
	if payload.Status != "ok" {
		t.Fatalf("expected overall ok status, got %#v", payload)
	}
	if !payload.Services["match"].Healthy || !payload.Services["platform"].Healthy || !payload.Services["matchmaking"].Healthy {
		t.Fatalf("expected all services healthy, got %#v", payload.Services)
	}
}

func TestGatewayStatusReportsDegradedService(t *testing.T) {
	matchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	}))
	defer matchServer.Close()

	mux := buildGatewayMux(GatewayConfig{
		MatchServiceURL:       matchServer.URL,
		PlatformServiceURL:    "http://127.0.0.1:1",
		MatchmakingServiceURL: matchServer.URL,
	}, &http.Client{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/system/status", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected gateway status to return ok envelope, got %d", rec.Code)
	}

	var payload GatewaySystemStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected degraded gateway status to decode, got %v", err)
	}
	if payload.Status != "degraded" {
		t.Fatalf("expected degraded overall status, got %#v", payload)
	}
	if payload.Services["platform"].Healthy || payload.Services["platform"].Error == "" {
		t.Fatalf("expected platform service failure to be surfaced, got %#v", payload.Services["platform"])
	}
}

func TestGatewayBootstrapIncludesCapabilitiesAndQueue(t *testing.T) {
	matchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "service": "match-service"})
	}))
	defer matchServer.Close()

	platformServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/platform/status":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "service": "platform-service"})
		case "/api/platform/capabilities":
			_ = json.NewEncoder(w).Encode(map[string]any{"ratings": true, "profiles": true})
		default:
			http.NotFound(w, r)
		}
	}))
	defer platformServer.Close()

	matchmakingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/status":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "service": "matchmaking-service"})
		case "/api/queues/default":
			_ = json.NewEncoder(w).Encode(map[string]any{"queue": "rated", "status": "open"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer matchmakingServer.Close()

	mux := buildGatewayMux(GatewayConfig{
		MatchServiceURL:       matchServer.URL,
		PlatformServiceURL:    platformServer.URL,
		MatchmakingServiceURL: matchmakingServer.URL,
	}, matchServer.Client())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/session/bootstrap", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected bootstrap to succeed, got %d", rec.Code)
	}

	var envelope contracts.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("expected bootstrap envelope to decode, got %v", err)
	}
	if envelope.Type != "gateway.bootstrap" {
		t.Fatalf("expected gateway bootstrap envelope, got %#v", envelope)
	}

	payload, ok := envelope.Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected bootstrap payload map, got %#v", envelope.Payload)
	}
	if payload["status"] != "ok" {
		t.Fatalf("expected bootstrap payload status ok, got %#v", payload)
	}
	message, _ := payload["message"].(string)
	if !strings.Contains(message, "ready") {
		t.Fatalf("expected bootstrap message to mention readiness, got %q", message)
	}
}

func TestGatewayPostBootstrapReturnsGuestSessionsAndSeatClaims(t *testing.T) {
	matchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "service": "match-service"})
	}))
	defer matchServer.Close()

	platformServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/platform/status":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "service": "platform-service"})
		case "/api/platform/capabilities":
			_ = json.NewEncoder(w).Encode(map[string]any{"profiles": true, "ratings": true})
		case "/api/platform/guest-sessions":
			var payload map[string]string
			_ = json.NewDecoder(r.Body).Decode(&payload)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"guest": map[string]any{
					"guestId":       payload["guestId"],
					"displayName":   "Guest " + payload["guestId"],
					"rating":        1200,
					"matchesPlayed": 0,
					"wins":          0,
					"losses":        0,
					"draws":         0,
					"createdAt":     "2026-01-01T00:00:00Z",
					"lastSeenAt":    "2026-01-01T00:00:00Z",
				},
				"sessionSecret": payload["sessionSecret"],
			})
		case "/api/platform/match-claims":
			var payload map[string]string
			_ = json.NewDecoder(r.Body).Decode(&payload)
			seatColor := "white"
			playerID := "white_player"
			if payload["guestId"] == "black-guest" {
				seatColor = "black"
				playerID = "black_player"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"matchId":      payload["matchId"],
				"guestId":      payload["guestId"],
				"seatColor":    seatColor,
				"playerId":     playerID,
				"playerSecret": "claim-" + payload["guestId"],
				"claimToken":   "token-" + payload["guestId"],
				"queue":        "rated",
				"status":       "active",
			})
		case "/api/platform/account-sessions":
			var payload map[string]string
			_ = json.NewDecoder(r.Body).Decode(&payload)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"account": map[string]any{
					"accountId":      payload["accountId"],
					"handle":         "aurora_white",
					"primaryGuestId": "white-guest",
					"linkedGuestIds": []string{"white-guest"},
					"createdAt":      "2026-01-01T00:00:00Z",
					"lastSeenAt":     "2026-01-02T00:00:00Z",
				},
				"sessionToken": payload["sessionToken"],
				"expiresAt":    "2026-12-31T00:00:00Z",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer platformServer.Close()

	matchmakingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/status":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "service": "matchmaking-service"})
		case "/api/queues/default":
			_ = json.NewEncoder(w).Encode(map[string]any{"queue": "rated", "status": "open"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer matchmakingServer.Close()

	mux := buildGatewayMux(GatewayConfig{
		MatchServiceURL:       matchServer.URL,
		PlatformServiceURL:    platformServer.URL,
		MatchmakingServiceURL: matchmakingServer.URL,
	}, matchServer.Client())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/session/bootstrap", strings.NewReader(`{
		"matchId":"room-77",
		"white":{"guestId":"white-guest","sessionSecret":"white-secret"},
		"black":{"guestId":"black-guest","sessionSecret":"black-secret"},
		"whiteAccount":{"accountId":"acct-white","sessionToken":"accttok-white"}
	}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected post bootstrap to succeed, got %d", rec.Code)
	}

	var envelope contracts.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("expected post bootstrap envelope to decode, got %v", err)
	}
	payload, ok := envelope.Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected bootstrap payload map, got %#v", envelope.Payload)
	}
	guestSessions, ok := payload["guestSessions"].(map[string]any)
	if !ok {
		t.Fatalf("expected guest sessions in bootstrap payload, got %#v", payload["guestSessions"])
	}
	matchClaims, ok := payload["matchClaims"].(map[string]any)
	if !ok {
		t.Fatalf("expected match claims in bootstrap payload, got %#v", payload["matchClaims"])
	}
	accountSessions, ok := payload["accountSessions"].(map[string]any)
	if !ok {
		t.Fatalf("expected account sessions in bootstrap payload, got %#v", payload["accountSessions"])
	}
	whiteSession := guestSessions["white"].(map[string]any)
	whiteGuest := whiteSession["guest"].(map[string]any)
	if whiteGuest["guestId"] != "white-guest" {
		t.Fatalf("expected white guest session to round-trip, got %#v", whiteGuest)
	}
	whiteAccountSession := accountSessions["white"].(map[string]any)
	whiteAccount := whiteAccountSession["account"].(map[string]any)
	if whiteAccount["accountId"] != "acct-white" || whiteAccountSession["sessionToken"] != "accttok-white" {
		t.Fatalf("expected white account session to round-trip, got %#v", whiteAccountSession)
	}
	blackClaim := matchClaims["black"].(map[string]any)
	if blackClaim["playerSecret"] != "claim-black-guest" || blackClaim["claimToken"] != "token-black-guest" {
		t.Fatalf("expected black seat claim to be returned, got %#v", blackClaim)
	}
}

func TestGatewayPostBootstrapRecoversStaleAccountSessionFromGuestSession(t *testing.T) {
	matchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "service": "match-service"})
	}))
	defer matchServer.Close()

	platformServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/platform/status":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "service": "platform-service"})
		case "/api/platform/capabilities":
			_ = json.NewEncoder(w).Encode(map[string]any{"profiles": true, "ratings": true})
		case "/api/platform/guest-sessions":
			var payload map[string]string
			_ = json.NewDecoder(r.Body).Decode(&payload)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"guest": map[string]any{
					"guestId":       payload["guestId"],
					"displayName":   "Guest " + payload["guestId"],
					"rating":        1200,
					"matchesPlayed": 0,
					"wins":          0,
					"losses":        0,
					"draws":         0,
					"createdAt":     "2026-01-01T00:00:00Z",
					"lastSeenAt":    "2026-01-01T00:00:00Z",
				},
				"sessionSecret": payload["sessionSecret"],
				"sessionToken":  "guesttok-" + payload["guestId"],
				"expiresAt":     "2026-12-31T00:00:00Z",
			})
		case "/api/platform/account-sessions":
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "unauthorized account session"})
		case "/api/platform/accounts/acct-white":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"account": map[string]any{
					"accountId":      "acct-white",
					"handle":         "aurora_white",
					"primaryGuestId": "white-guest",
					"linkedGuestIds": []string{"white-guest"},
					"createdAt":      "2026-01-01T00:00:00Z",
					"lastSeenAt":     "2026-01-02T00:00:00Z",
				},
			})
		case "/api/platform/accounts/claim":
			var payload map[string]string
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if payload["guestId"] != "white-guest" || payload["handle"] != "aurora_white" || payload["sessionToken"] != "guesttok-white-guest" {
				t.Fatalf("expected guest-backed reclaim payload, got %#v", payload)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"account": map[string]any{
					"accountId":      "acct-white",
					"handle":         "aurora_white",
					"primaryGuestId": "white-guest",
					"linkedGuestIds": []string{"white-guest"},
					"createdAt":      "2026-01-01T00:00:00Z",
					"lastSeenAt":     "2026-01-03T00:00:00Z",
				},
				"sessionToken": "accttok-refreshed",
				"expiresAt":    "2026-12-31T00:00:00Z",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer platformServer.Close()

	matchmakingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/status":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "service": "matchmaking-service"})
		case "/api/queues/default":
			_ = json.NewEncoder(w).Encode(map[string]any{"queue": "rated", "status": "open"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer matchmakingServer.Close()

	mux := buildGatewayMux(GatewayConfig{
		MatchServiceURL:       matchServer.URL,
		PlatformServiceURL:    platformServer.URL,
		MatchmakingServiceURL: matchmakingServer.URL,
	}, matchServer.Client())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/session/bootstrap", strings.NewReader(`{
		"white":{"guestId":"white-guest","sessionSecret":"white-secret"},
		"whiteAccount":{"accountId":"acct-white","sessionToken":"accttok-stale"}
	}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected stale account recovery bootstrap to succeed, got %d", rec.Code)
	}

	var envelope contracts.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("expected recovery bootstrap envelope to decode, got %v", err)
	}
	payload, ok := envelope.Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected bootstrap payload map, got %#v", envelope.Payload)
	}
	accountSessions, ok := payload["accountSessions"].(map[string]any)
	if !ok {
		t.Fatalf("expected recovered account sessions in bootstrap payload, got %#v", payload["accountSessions"])
	}
	whiteAccountSession := accountSessions["white"].(map[string]any)
	if whiteAccountSession["sessionToken"] != "accttok-refreshed" {
		t.Fatalf("expected stale account session to be refreshed through guest reclaim, got %#v", whiteAccountSession)
	}
}

func TestGatewayPostBootstrapRecoversFromUnauthorizedGuestResume(t *testing.T) {
	matchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "service": "match-service"})
	}))
	defer matchServer.Close()

	var emptyGuestCalls int
	platformServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/platform/status":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "service": "platform-service"})
		case "/api/platform/capabilities":
			_ = json.NewEncoder(w).Encode(map[string]any{"profiles": true})
		case "/api/platform/guest-sessions":
			var payload map[string]string
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if payload["guestId"] == "stale-white" {
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": "unauthorized guest session"})
				return
			}
			if payload["guestId"] == "" {
				emptyGuestCalls++
				guestID := "white-fresh"
				secret := "white-fresh-secret"
				if emptyGuestCalls > 1 {
					guestID = "black-fresh"
					secret = "black-fresh-secret"
				}
				_ = json.NewEncoder(w).Encode(map[string]any{
					"guest": map[string]any{
						"guestId":       guestID,
						"displayName":   guestID,
						"rating":        1200,
						"matchesPlayed": 0,
						"wins":          0,
						"losses":        0,
						"draws":         0,
						"createdAt":     "2026-01-01T00:00:00Z",
						"lastSeenAt":    "2026-01-01T00:00:00Z",
					},
					"sessionSecret": secret,
				})
				return
			}
			http.Error(w, `{"error":"unexpected guest payload"}`, http.StatusBadRequest)
		default:
			http.NotFound(w, r)
		}
	}))
	defer platformServer.Close()

	matchmakingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/status":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "service": "matchmaking-service"})
		case "/api/queues/default":
			_ = json.NewEncoder(w).Encode(map[string]any{"queue": "rated", "status": "open"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer matchmakingServer.Close()

	mux := buildGatewayMux(GatewayConfig{
		MatchServiceURL:       matchServer.URL,
		PlatformServiceURL:    platformServer.URL,
		MatchmakingServiceURL: matchmakingServer.URL,
	}, matchServer.Client())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/session/bootstrap", strings.NewReader(`{
		"white":{"guestId":"stale-white","sessionSecret":"stale-secret"}
	}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected post bootstrap recovery to succeed, got %d", rec.Code)
	}

	var envelope contracts.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("expected recovery bootstrap envelope to decode, got %v", err)
	}
	payload, ok := envelope.Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected bootstrap payload map, got %#v", envelope.Payload)
	}
	guestSessions := payload["guestSessions"].(map[string]any)
	whiteSession := guestSessions["white"].(map[string]any)
	whiteGuest := whiteSession["guest"].(map[string]any)
	if whiteGuest["guestId"] != "white-fresh" {
		t.Fatalf("expected stale white session to be replaced, got %#v", whiteGuest)
	}
	blackSession := guestSessions["black"].(map[string]any)
	blackGuest := blackSession["guest"].(map[string]any)
	if blackGuest["guestId"] != "black-fresh" {
		t.Fatalf("expected bootstrap to create black guest session too, got %#v", blackGuest)
	}
}

func TestGatewayIntentProxyResolvesClaimToken(t *testing.T) {
	matchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/matches/room-intents/intents" {
			http.NotFound(w, r)
			return
		}
		var req contracts.ApplyIntentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("expected forwarded intent body to decode, got %v", err)
		}
		if req.Intent.PlayerSecret != "resolved-room-secret" || req.Intent.PlayerID != "guest_claimed" || req.Intent.PlayerClaimToken != "" {
			t.Fatalf("expected gateway to inject resolved claim secret, got %#v", req.Intent)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"match": map[string]any{
				"matchId": "room-intents",
				"turn":    "black",
			},
		})
	}))
	defer matchServer.Close()

	platformServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/platform/match-claims/resolve":
			var payload map[string]string
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if payload["claimToken"] != "claimtok_gateway" || payload["matchId"] != "room-intents" {
				t.Fatalf("expected gateway resolve request to include claim token and match id, got %#v", payload)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"matchId":      payload["matchId"],
				"guestId":      "guest_claimed",
				"seatColor":    "white",
				"playerId":     "guest_claimed",
				"playerSecret": "resolved-room-secret",
				"claimToken":   payload["claimToken"],
			})
		case "/api/platform/status":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "service": "platform-service"})
		case "/api/platform/capabilities":
			_ = json.NewEncoder(w).Encode(map[string]any{"profiles": true})
		default:
			http.NotFound(w, r)
		}
	}))
	defer platformServer.Close()

	matchmakingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/status":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "service": "matchmaking-service"})
		case "/api/queues/default":
			_ = json.NewEncoder(w).Encode(map[string]any{"queue": "rated", "status": "open"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer matchmakingServer.Close()

	mux := buildGatewayMux(GatewayConfig{
		MatchServiceURL:       matchServer.URL,
		PlatformServiceURL:    platformServer.URL,
		MatchmakingServiceURL: matchmakingServer.URL,
	}, matchServer.Client())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/matches/room-intents/intents", strings.NewReader(`{
		"intent":{
			"type":"offer_draw",
			"matchId":"room-intents",
			"playerId":"ignored-client-id",
			"playerClaimToken":"claimtok_gateway"
		}
	}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected gateway intent proxy to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}
}
