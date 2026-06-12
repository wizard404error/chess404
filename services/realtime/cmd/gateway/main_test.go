package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chess404/realtime/internal/contracts"
)

func TestResolveInternalServiceURLAddsRailwayPortFallback(t *testing.T) {
	resolved := resolveInternalServiceURL("http://platform-service.railway.internal", "http://platform-service.railway.internal:8080")
	if resolved != "http://platform-service.railway.internal:8080" {
		t.Fatalf("expected railway internal host to gain :8080, got %q", resolved)
	}
}

func TestResolveInternalServiceURLFallsBackForInvalidTemplate(t *testing.T) {
	resolved := resolveInternalServiceURL("${{platform-service.RAILWAY_PRIVATE_DOMAIN}}", "http://platform-service.railway.internal:8080")
	if resolved != "http://platform-service.railway.internal:8080" {
		t.Fatalf("expected invalid template value to use fallback, got %q", resolved)
	}
}

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
	if payload.Services["platform"].Healthy {
		t.Fatalf("expected platform service to be unhealthy, got %#v", payload.Services["platform"])
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
	if blackClaim["playerSecret"] != "" {
		t.Fatalf("expected black seat claim playerSecret to be stripped, got %#v", blackClaim["playerSecret"])
	}
	if _, ok := blackClaim["claimToken"]; ok {
		t.Fatalf("expected black seat claim claimToken to be stripped, got %#v", blackClaim["claimToken"])
	}
}

func TestGatewayBootstrapRecoversQueueTicketsAndActiveMatch(t *testing.T) {
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
		case "/api/platform/match-claims/active":
			var payload map[string]string
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if payload["guestId"] != "guest_white" {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"matchId":      "direct-room-44",
				"guestId":      "guest_white",
				"seatColor":    "white",
				"playerId":     "player_white",
				"playerSecret": "claim-secret",
				"claimToken":   "claim-token",
				"queue":        "direct",
				"modeId":       "hidden_cards",
				"whiteGuestId": "guest_white",
				"blackGuestId": "guest_black",
				"whiteName":    "Guest guest_white",
				"blackName":    "Guest guest_black",
				"status":       "waiting",
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
		case "/api/queues/tickets":
			guestID := r.URL.Query().Get("guestId")
			switch guestID {
			case "guest_white":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"ticket": map[string]any{
						"ticketId":    "ticket_white",
						"guestId":     "guest_white",
						"displayName": "Guest guest_white",
						"queue":       "rated",
						"modeId":      "hidden_cards",
						"status":      "queued",
						"rating":      1200,
						"createdAt":   "2026-05-22T00:00:00Z",
						"updatedAt":   "2026-05-22T00:00:05Z",
					},
				})
			case "guest_black":
				http.NotFound(w, r)
			default:
				http.NotFound(w, r)
			}
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
		"white":{"guestId":"guest_white","sessionSecret":"secret_white"},
		"black":{"guestId":"guest_black","sessionSecret":"secret_black"}
	}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected bootstrap recovery to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}

	var envelope contracts.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode bootstrap envelope: %v", err)
	}
	payload, ok := envelope.Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected bootstrap payload map, got %#v", envelope.Payload)
	}

	queueTickets, ok := payload["queueTickets"].(map[string]any)
	if !ok {
		t.Fatalf("expected queue tickets recovery, got %#v", payload["queueTickets"])
	}
	whiteTicket := queueTickets["white"].(map[string]any)
	if whiteTicket["ticketId"] != "ticket_white" || whiteTicket["status"] != "queued" {
		t.Fatalf("expected queued ticket recovery, got %#v", whiteTicket)
	}

	recoveredMatch, ok := payload["recoveredMatch"].(map[string]any)
	if !ok {
		t.Fatalf("expected active match recovery, got %#v", payload["recoveredMatch"])
	}
	if recoveredMatch["matchId"] != "direct-room-44" || recoveredMatch["viewerSeat"] != "white" || recoveredMatch["queue"] != "direct" {
		t.Fatalf("unexpected recovered match payload %#v", recoveredMatch)
	}
}

func TestGatewayCreatesPrivateMatch(t *testing.T) {
	matchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/matches":
			if r.Method != http.MethodPost {
				t.Fatalf("expected create private match post, got %s", r.Method)
			}
			var payload map[string]any
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if payload["queue"] != "direct" {
				t.Fatalf("expected direct queue, got %#v", payload["queue"])
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"match": map[string]any{
					"matchId":      "private-room-1",
					"status":       "waiting",
					"queue":        "direct",
					"modeId":       "open_cards",
					"whiteGuestId": "white-guest",
					"whiteName":    "Guest white-guest",
				},
				"replayHead": 1,
				"events":     []map[string]any{},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "service": "match-service"})
		}
	}))
	defer matchServer.Close()

	platformServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/platform/guest-sessions":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"guest": map[string]any{
					"guestId":       "white-guest",
					"displayName":   "Guest white-guest",
					"rating":        1200,
					"matchesPlayed": 0,
					"wins":          0,
					"losses":        0,
					"draws":         0,
					"createdAt":     "2026-01-01T00:00:00Z",
					"lastSeenAt":    "2026-01-01T00:00:00Z",
				},
				"sessionSecret": "white-secret",
			})
		case "/api/platform/match-claims":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"matchId":      "private-room-1",
				"guestId":      "white-guest",
				"seatColor":    "white",
				"playerId":     "white-guest",
				"playerSecret": "white-secret",
				"claimToken":   "claim-white",
				"queue":        "direct",
				"status":       "waiting",
			})
		case "/api/platform/status", "/api/platform/capabilities":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer platformServer.Close()

	matchmakingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	}))
	defer matchmakingServer.Close()

	mux := buildGatewayMux(GatewayConfig{
		MatchServiceURL:       matchServer.URL,
		PlatformServiceURL:    platformServer.URL,
		MatchmakingServiceURL: matchmakingServer.URL,
	}, matchServer.Client())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/private-matches", strings.NewReader(`{
		"guest":{"guestId":"white-guest","sessionSecret":"white-secret"},
		"modeId":"open_cards",
		"preferredSeat":"white"
	}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected private room create to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload GatewayPrivateMatchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected private room create response to decode, got %v", err)
	}
	if payload.MatchID != "private-room-1" || payload.SeatColor != "white" || !payload.WaitingForOpponent {
		t.Fatalf("unexpected private room create payload: %#v", payload)
	}
}

func TestGatewayJoinsPrivateMatch(t *testing.T) {
	matchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/matches/private-room-2/join":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"match": map[string]any{
					"match": map[string]any{
						"matchId":      "private-room-2",
						"status":       "active",
						"queue":        "direct",
						"modeId":       "hidden_cards",
						"whiteGuestId": "white-guest",
						"blackGuestId": "black-guest",
					},
					"replayHead": 2,
					"events":     []map[string]any{},
				},
				"seatColor":          "black",
				"joined":             true,
				"waitingForOpponent": false,
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "service": "match-service"})
		}
	}))
	defer matchServer.Close()

	platformServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/platform/guest-sessions":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"guest": map[string]any{
					"guestId":       "black-guest",
					"displayName":   "Guest black-guest",
					"rating":        1200,
					"matchesPlayed": 0,
					"wins":          0,
					"losses":        0,
					"draws":         0,
					"createdAt":     "2026-01-01T00:00:00Z",
					"lastSeenAt":    "2026-01-01T00:00:00Z",
				},
				"sessionSecret": "black-secret",
			})
		case "/api/platform/match-claims":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"matchId":      "private-room-2",
				"guestId":      "black-guest",
				"seatColor":    "black",
				"playerId":     "black-guest",
				"playerSecret": "black-secret",
				"claimToken":   "claim-black",
				"queue":        "direct",
				"status":       "active",
			})
		case "/api/platform/status", "/api/platform/capabilities":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer platformServer.Close()

	matchmakingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	}))
	defer matchmakingServer.Close()

	mux := buildGatewayMux(GatewayConfig{
		MatchServiceURL:       matchServer.URL,
		PlatformServiceURL:    platformServer.URL,
		MatchmakingServiceURL: matchmakingServer.URL,
	}, matchServer.Client())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/private-matches/private-room-2/join", strings.NewReader(`{
		"guest":{"guestId":"black-guest","sessionSecret":"black-secret"}
	}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected private room join to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload GatewayPrivateMatchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected private room join response to decode, got %v", err)
	}
	if payload.MatchID != "private-room-2" || payload.SeatColor != "black" || payload.WaitingForOpponent {
		t.Fatalf("unexpected private room join payload: %#v", payload)
	}
}

func TestGatewayCreatesPrivateRematchRoom(t *testing.T) {
	matchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/matches/private-room-finished":
			if r.Method != http.MethodGet {
				t.Fatalf("expected rematch source load to use get, got %s", r.Method)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"match": map[string]any{
					"matchId":      "private-room-finished",
					"status":       "finished",
					"queue":        "direct",
					"modeId":       "hidden_cards",
					"whiteGuestId": "white-guest",
					"blackGuestId": "black-guest",
				},
				"replayHead": 44,
				"events":     []map[string]any{},
			})
		case "/api/matches":
			if r.Method != http.MethodPost {
				t.Fatalf("expected rematch room create to use post, got %s", r.Method)
			}
			var payload map[string]any
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if payload["queue"] != "direct" {
				t.Fatalf("expected rematch queue to stay direct, got %#v", payload["queue"])
			}
			if payload["modeId"] != "hidden_cards" {
				t.Fatalf("expected rematch mode to stay hidden_cards, got %#v", payload["modeId"])
			}
			if payload["clockSeconds"] != float64(600) {
				t.Fatalf("expected rematch clock to stay 600 seconds, got %#v", payload["clockSeconds"])
			}
			if payload["blackGuestId"] != "black-guest" {
				t.Fatalf("expected rematch creator to keep black seat, got %#v", payload)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"match": map[string]any{
					"matchId":      "private-room-rematch-1",
					"status":       "waiting",
					"queue":        "direct",
					"modeId":       "hidden_cards",
					"blackGuestId": "black-guest",
					"blackName":    "Guest black-guest",
				},
				"replayHead": 1,
				"events":     []map[string]any{},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "service": "match-service"})
		}
	}))
	defer matchServer.Close()

	platformServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/platform/guest-sessions":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"guest": map[string]any{
					"guestId":       "black-guest",
					"displayName":   "Guest black-guest",
					"rating":        1200,
					"matchesPlayed": 0,
					"wins":          0,
					"losses":        0,
					"draws":         0,
					"createdAt":     "2026-01-01T00:00:00Z",
					"lastSeenAt":    "2026-01-01T00:00:00Z",
				},
				"sessionSecret": "black-secret",
			})
		case "/api/platform/match-claims":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"matchId":      "private-room-rematch-1",
				"guestId":      "black-guest",
				"seatColor":    "black",
				"playerId":     "black-guest",
				"playerSecret": "black-secret",
				"claimToken":   "claim-black-rematch",
				"queue":        "direct",
				"status":       "waiting",
			})
		case "/api/platform/status", "/api/platform/capabilities":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer platformServer.Close()

	matchmakingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	}))
	defer matchmakingServer.Close()

	mux := buildGatewayMux(GatewayConfig{
		MatchServiceURL:       matchServer.URL,
		PlatformServiceURL:    platformServer.URL,
		MatchmakingServiceURL: matchmakingServer.URL,
	}, matchServer.Client())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/private-matches/private-room-finished/rematch", strings.NewReader(`{
		"guest":{"guestId":"black-guest","sessionSecret":"black-secret"},
		"clockSeconds":600
	}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected private rematch create to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload GatewayPrivateMatchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected private rematch response to decode, got %v", err)
	}
	if payload.MatchID != "private-room-rematch-1" || payload.SeatColor != "black" || !payload.WaitingForOpponent {
		t.Fatalf("unexpected private rematch payload: %#v", payload)
	}
}

func TestGatewayCreatesDirectChallenge(t *testing.T) {
	matchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/matches":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"match": map[string]any{
					"matchId":      "challenge-room-1",
					"status":       "waiting",
					"queue":        "direct",
					"modeId":       "hidden_cards",
					"whiteGuestId": "white-guest",
					"whiteName":    "Guest white-guest",
				},
				"replayHead": 1,
				"events":     []map[string]any{},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "service": "match-service"})
		}
	}))
	defer matchServer.Close()

	platformServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
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
		case "/api/platform/challenges/eligibility":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case "/api/platform/challenges":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"challengeId":    "challenge-1",
				"status":         "pending",
				"matchId":        "challenge-room-1",
				"modeId":         "hidden_cards",
				"clockSeconds":   900,
				"challengerSeat": "white",
				"viewerSeat":     "white",
			})
		case "/api/platform/match-claims":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"matchId":      "challenge-room-1",
				"guestId":      "white-guest",
				"seatColor":    "white",
				"playerId":     "white-guest",
				"playerSecret": "white-secret",
				"claimToken":   "claim-white",
				"queue":        "direct",
				"status":       "waiting",
			})
		case "/api/platform/status", "/api/platform/capabilities":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer platformServer.Close()

	matchmakingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	}))
	defer matchmakingServer.Close()

	mux := buildGatewayMux(GatewayConfig{
		MatchServiceURL:       matchServer.URL,
		PlatformServiceURL:    platformServer.URL,
		MatchmakingServiceURL: matchmakingServer.URL,
	}, matchServer.Client())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/challenges", strings.NewReader(`{
		"guest":{"guestId":"white-guest","sessionSecret":"white-secret"},
		"account":{"accountId":"acct-white","sessionToken":"accttok-white"},
		"targetAccountId":"acct-black",
		"modeId":"hidden_cards",
		"preferredSeat":"white",
		"clockSeconds":900
	}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected direct challenge create to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload GatewayDirectChallengeLaunchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected direct challenge create response to decode, got %v", err)
	}
	if payload.ChallengeID != "challenge-1" || payload.ModeID != contracts.MatchModeHiddenCards || payload.Match.MatchID != "challenge-room-1" {
		t.Fatalf("unexpected direct challenge create payload: %#v", payload)
	}
}

func TestGatewayAcceptsDirectChallenge(t *testing.T) {
	matchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/matches/challenge-room-2/join":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"match": map[string]any{
					"match": map[string]any{
						"matchId":      "challenge-room-2",
						"status":       "active",
						"queue":        "direct",
						"modeId":       "open_cards",
						"whiteGuestId": "white-guest",
						"blackGuestId": "black-guest",
					},
					"replayHead": 2,
					"events":     []map[string]any{},
				},
				"seatColor":          "black",
				"joined":             true,
				"waitingForOpponent": false,
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "service": "match-service"})
		}
	}))
	defer matchServer.Close()

	platformServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
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
		case "/api/platform/account-sessions":
			var payload map[string]string
			_ = json.NewDecoder(r.Body).Decode(&payload)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"account": map[string]any{
					"accountId":      payload["accountId"],
					"handle":         "nova_black",
					"primaryGuestId": "black-guest",
					"linkedGuestIds": []string{"black-guest"},
					"createdAt":      "2026-01-01T00:00:00Z",
					"lastSeenAt":     "2026-01-02T00:00:00Z",
				},
				"sessionToken": payload["sessionToken"],
				"expiresAt":    "2026-12-31T00:00:00Z",
			})
		case "/api/platform/challenges/challenge-2/view":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"challengeId":    "challenge-2",
				"status":         "pending",
				"matchId":        "challenge-room-2",
				"modeId":         "open_cards",
				"clockSeconds":   600,
				"challengerSeat": "white",
				"viewerSeat":     "black",
			})
		case "/api/platform/challenges/challenge-2/respond":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"challengeId":    "challenge-2",
				"status":         "accepted",
				"matchId":        "challenge-room-2",
				"modeId":         "open_cards",
				"clockSeconds":   600,
				"challengerSeat": "white",
				"viewerSeat":     "black",
			})
		case "/api/platform/match-claims":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"matchId":      "challenge-room-2",
				"guestId":      "black-guest",
				"seatColor":    "black",
				"playerId":     "black-guest",
				"playerSecret": "black-secret",
				"claimToken":   "claim-black",
				"queue":        "direct",
				"status":       "active",
			})
		case "/api/platform/status", "/api/platform/capabilities":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer platformServer.Close()

	matchmakingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	}))
	defer matchmakingServer.Close()

	mux := buildGatewayMux(GatewayConfig{
		MatchServiceURL:       matchServer.URL,
		PlatformServiceURL:    platformServer.URL,
		MatchmakingServiceURL: matchmakingServer.URL,
	}, matchServer.Client())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/challenges/challenge-2/accept", strings.NewReader(`{
		"guest":{"guestId":"black-guest","sessionSecret":"black-secret"},
		"account":{"accountId":"acct-black","sessionToken":"accttok-black"}
	}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected direct challenge accept to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload GatewayDirectChallengeLaunchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected direct challenge accept response to decode, got %v", err)
	}
	if payload.ChallengeID != "challenge-2" || payload.Match.MatchID != "challenge-room-2" || payload.Match.SeatColor != "black" {
		t.Fatalf("unexpected direct challenge accept payload: %#v", payload)
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

func TestGatewayPresenceProxyResolvesClaimToken(t *testing.T) {
	matchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/matches/room-presence/presence" {
			http.NotFound(w, r)
			return
		}
		var req contracts.MatchPresenceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("expected forwarded presence body to decode, got %v", err)
		}
		if req.PlayerSecret != "resolved-room-secret" || req.PlayerID != "guest_claimed" || req.PlayerClaimToken != "" {
			t.Fatalf("expected gateway to inject resolved presence secret, got %#v", req)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer matchServer.Close()

	platformServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/platform/match-claims/resolve":
			var payload map[string]string
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if payload["claimToken"] != "claimtok_presence" || payload["matchId"] != "room-presence" {
				t.Fatalf("expected gateway presence resolve request to include claim token and match id, got %#v", payload)
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
	req := httptest.NewRequest(http.MethodPost, "/api/matches/room-presence/presence", strings.NewReader(`{
		"playerId":"ignored-client-id",
		"playerClaimToken":"claimtok_presence"
	}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected gateway presence proxy to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestGatewayForwardsOriginHeaderToBackendServices verifies that when the
// browser sends a request to the gateway, the gateway forwards a clean
// Origin header to the backend services (platform/match). This is required
// so the destination services' CSRF middleware can validate the request
// against its allow-list, instead of rejecting it with 403 "origin header
// required".
//
// The browser does NOT send an Origin header for same-origin POSTs — it
// only sends a Referer (with a path) — so the gateway must reconstruct the
// Origin from the source request's host information. A path-bearing
// Referer like https://example.com/play does not match a bare origin
// https://example.com/ in the allow-list.
func TestGatewayForwardsOriginHeaderToBackendServices(t *testing.T) {
	const wantOrigin = "https://web-production-9a697.up.railway.app"
	const refererWithPath = "https://web-production-9a697.up.railway.app/play"

	var (
		platformCalls   int
		platformOrigins []string
		matchCalls      int
		matchOrigins    []string
	)

	platformServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/platform/status":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case "/api/platform/capabilities":
			_ = json.NewEncoder(w).Encode(map[string]any{"profiles": true})
		case "/api/platform/guest-sessions":
			platformCalls++
			platformOrigins = append(platformOrigins, r.Header.Get("Origin"))
			var payload map[string]string
			_ = json.NewDecoder(r.Body).Decode(&payload)
			guestID := strings.TrimSpace(payload["guestId"])
			if guestID == "" {
				guestID = "gw_guest_white"
			}
			secret := strings.TrimSpace(payload["sessionSecret"])
			if secret == "" {
				secret = "gw_secret_white"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"guest": map[string]any{
					"guestId":       guestID,
					"displayName":   "Guest",
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
		case "/api/platform/match-claims":
			platformOrigins = append(platformOrigins, r.Header.Get("Origin"))
			var payload map[string]string
			_ = json.NewDecoder(r.Body).Decode(&payload)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"matchId":      payload["matchId"],
				"guestId":      payload["guestId"],
				"seatColor":    "white",
				"playerId":     "white_player",
				"playerSecret": "claim-secret",
				"claimToken":   "claim-token",
				"queue":        "direct",
				"status":       "active",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer platformServer.Close()

	matchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/system/status":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case "/api/matches":
			matchCalls++
			matchOrigins = append(matchOrigins, r.Header.Get("Origin"))
			var payload map[string]any
			_ = json.NewDecoder(r.Body).Decode(&payload)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"match": map[string]any{
					"matchId":         "match_forward_origin",
					"queue":           "direct",
					"modeId":          "open_cards",
					"status":          "waiting",
					"whiteGuestId":    "gw_guest_white",
					"blackGuestId":    "gw_guest_black",
					"whiteName":       "White",
					"blackName":       "Black",
					"clockSeconds":    600,
					"clockIncrement":  5,
					"createdAt":       "2026-01-01T00:00:00Z",
					"updatedAt":       "2026-01-01T00:00:00Z",
					"moveCount":       0,
					"version":         0,
					"firstMoveAuthor": "",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer matchServer.Close()

	mux := buildGatewayMux(GatewayConfig{
		MatchServiceURL:       matchServer.URL,
		PlatformServiceURL:    platformServer.URL,
		MatchmakingServiceURL: matchServer.URL,
	}, matchServer.Client())

	rec := httptest.NewRecorder()
	// Simulate a same-origin POST arriving at the gateway from behind a
	// reverse proxy: browser sends no Origin, only Referer with a path.
	// The proxy (e.g., Next.js or Railway) sets X-Forwarded-Proto and
	// X-Forwarded-Host so the gateway can reconstruct the public origin.
	req := httptest.NewRequest(http.MethodPost, "/api/private-matches", strings.NewReader(`{
		"guest": {"guestId":"gw_guest_white","sessionSecret":"gw_secret_white","displayName":"White"},
		"queue":"direct","preferredSeat":"white"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Referer", refererWithPath)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "web-production-9a697.up.railway.app")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected gateway to create match, got %d body=%s", rec.Code, rec.Body.String())
	}
	if platformCalls == 0 {
		t.Fatalf("expected platform-service to be called at least once")
	}
	if matchCalls == 0 {
		t.Fatalf("expected match-service to be called at least once")
	}
	for i, origin := range platformOrigins {
		if origin != wantOrigin {
			t.Fatalf("platform call #%d: expected Origin %q, got %q", i, wantOrigin, origin)
		}
	}
	for i, origin := range matchOrigins {
		if origin != wantOrigin {
			t.Fatalf("match call #%d: expected Origin %q, got %q", i, wantOrigin, origin)
		}
	}
}

// TestGatewayForwardsExplicitOriginHeader covers the cross-origin case:
// the browser sends an explicit Origin header. The gateway should pass it
// through (extracting just scheme+host) to backend services.
func TestGatewayForwardsExplicitOriginHeader(t *testing.T) {
	const browserOrigin = "https://web-production-9a697.up.railway.app"
	// Some browsers may add a path; the gateway should strip it.
	const browserOriginWithPath = "https://web-production-9a697.up.railway.app/some/page"
	const wantOrigin = "https://web-production-9a697.up.railway.app"

	var (
		platformOrigin string
		matchOrigin    string
	)

	platformServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/platform/status":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case "/api/platform/capabilities":
			_ = json.NewEncoder(w).Encode(map[string]any{"profiles": true})
		case "/api/platform/guest-sessions":
			platformOrigin = r.Header.Get("Origin")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"guest": map[string]any{
					"guestId":       "gw_explicit",
					"displayName":   "Guest",
					"rating":        1200,
					"matchesPlayed": 0,
					"wins":          0,
					"losses":        0,
					"draws":         0,
					"createdAt":     "2026-01-01T00:00:00Z",
					"lastSeenAt":    "2026-01-01T00:00:00Z",
				},
				"sessionSecret": "gw_secret",
			})
		case "/api/platform/match-claims":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"matchId":      "match_explicit_origin",
				"guestId":      "gw_explicit",
				"seatColor":    "white",
				"playerId":     "white_player",
				"playerSecret": "claim-secret",
				"claimToken":   "claim-token",
				"queue":        "direct",
				"status":       "active",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer platformServer.Close()

	matchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/system/status" {
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
			return
		}
		if r.URL.Path == "/api/matches" {
			matchOrigin = r.Header.Get("Origin")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"match": map[string]any{
					"matchId":         "match_explicit_origin",
					"queue":           "direct",
					"modeId":          "open_cards",
					"status":          "waiting",
					"whiteGuestId":    "gw_explicit",
					"blackGuestId":    "gw_explicit",
					"whiteName":       "White",
					"blackName":       "Black",
					"clockSeconds":    600,
					"clockIncrement":  5,
					"createdAt":       "2026-01-01T00:00:00Z",
					"updatedAt":       "2026-01-01T00:00:00Z",
					"moveCount":       0,
					"version":         0,
					"firstMoveAuthor": "",
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer matchServer.Close()

	mux := buildGatewayMux(GatewayConfig{
		MatchServiceURL:       matchServer.URL,
		PlatformServiceURL:    platformServer.URL,
		MatchmakingServiceURL: matchServer.URL,
	}, matchServer.Client())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/private-matches", strings.NewReader(`{
		"guest": {"guestId":"gw_explicit","sessionSecret":"gw_secret","displayName":"White"},
		"queue":"direct","preferredSeat":"white"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", browserOriginWithPath)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected gateway to create match, got %d body=%s", rec.Code, rec.Body.String())
	}
	if platformOrigin != wantOrigin {
		t.Fatalf("platform: expected Origin %q, got %q", wantOrigin, platformOrigin)
	}
	if matchOrigin != wantOrigin {
		t.Fatalf("match: expected Origin %q, got %q", wantOrigin, matchOrigin)
	}
}
