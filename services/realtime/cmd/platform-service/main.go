package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chess404/realtime/internal/contracts"
	"github.com/chess404/realtime/internal/platform"
)

func main() {
	archive, err := openArchiveStore()
	if err != nil {
		log.Fatalf("failed to initialize archive store: %v", err)
	}
	defer func() { _ = archive.Close() }()
	guests, err := openGuestDirectory()
	if err != nil {
		log.Fatalf("failed to initialize guest store: %v", err)
	}
	defer func() { _ = guests.Close() }()
	accounts, err := openAccountStore()
	if err != nil {
		log.Fatalf("failed to initialize account store: %v", err)
	}
	defer func() { _ = accounts.Close() }()
	friends, err := openFriendshipStore()
	if err != nil {
		log.Fatalf("failed to initialize friendship store: %v", err)
	}
	defer func() { _ = friends.Close() }()
	moderation, err := openModerationStore()
	if err != nil {
		log.Fatalf("failed to initialize moderation store: %v", err)
	}
	defer func() { _ = moderation.Close() }()
	challenges, err := openDirectChallengeStore()
	if err != nil {
		log.Fatalf("failed to initialize direct challenge store: %v", err)
	}
	defer func() { _ = challenges.Close() }()
	notifications, err := openNotificationStore()
	if err != nil {
		log.Fatalf("failed to initialize notification store: %v", err)
	}
	defer func() { _ = notifications.Close() }()
	emailOutbox, err := openAccountEmailOutboxStore()
	if err != nil {
		log.Fatalf("failed to initialize account email outbox: %v", err)
	}
	defer func() { _ = emailOutbox.Close() }()
	emailSender, err := openAccountEmailSender()
	if err != nil {
		log.Fatalf("failed to initialize account email delivery: %v", err)
	}
	dispatcherContext, cancelEmailDispatch := context.WithCancel(context.Background())
	defer cancelEmailDispatch()
	newAccountEmailDispatcher(emailOutbox, emailSender, time.Now).Start(dispatcherContext)
	securityAudit, err := openAccountSecurityAuditStore()
	if err != nil {
		log.Fatalf("failed to initialize account security audit store: %v", err)
	}
	defer func() { _ = securityAudit.Close() }()
	claims, err := openMatchClaimStore()
	if err != nil {
		log.Fatalf("failed to initialize match claim store: %v", err)
	}
	defer func() { _ = claims.Close() }()
	mux := buildPlatformMux(archive, guests, accounts, friends, moderation, challenges, notifications, emailOutbox, securityAudit, claims)

	addr := listenAddr("PLATFORM_ADDR", 8083)
	log.Printf("platform-service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func buildPlatformMux(archive *platform.MatchArchiveStore, guests platform.GuestDirectory, accounts platform.AccountDirectory, friends platform.FriendshipDirectory, moderation platform.ModerationDirectory, challenges platform.DirectChallengeDirectory, notifications platform.AccountNotificationDirectory, emailOutbox platform.AccountEmailOutboxDirectory, securityAudit platform.AccountSecurityAuditDirectory, claims *platform.MatchClaimStore) http.Handler {
	mux := http.NewServeMux()
	authThrottle := newPlatformAuthThrottle(time.Now)
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

	mux.HandleFunc("/api/platform/capabilities", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"guestPlay":               true,
			"rankedRequiresID":        true,
			"accountRegistration":     true,
			"profiles":                true,
			"ratings":                 true,
			"matchHistory":            true,
			"friends":                 true,
			"friendChallenges":        true,
			"inbox":                   true,
			"presence":                true,
			"moderation":              true,
			"moderationAdmin":         moderationAdminConfigured(),
			"emailVerification":       true,
			"passwordReset":           true,
			"authEmailDelivery":       true,
			"authEmailDispatch":       configuredAccountEmailDeliveryProvider() != "disabled",
			"accountSecurityActivity": true,
		})
	})

	mux.HandleFunc("/api/platform/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":                    "ok",
			"service":                   "platform-service",
			"checkedAt":                 time.Now().UTC(),
			"archiveBackend":            archive.Backend(),
			"archive":                   archive.Stats(),
			"claimStoreBackend":         claims.Backend(),
			"claimLeaseSeconds":         claims.TTLSeconds(),
			"claims":                    claims.Stats(),
			"accounts":                  accounts.Stats(),
			"accountStoreBackend":       accounts.Backend(),
			"friends":                   friends.Stats(),
			"friendStoreBackend":        friends.Backend(),
			"moderation":                moderation.Stats(),
			"moderationStoreBackend":    moderation.Backend(),
			"moderationAdminConfigured": moderationAdminConfigured(),
			"challenges":                challenges.Stats(),
			"challengeStoreBackend":     challenges.Backend(),
			"notifications":             notifications.Stats(),
			"notificationStoreBackend":  notifications.Backend(),
			"emailOutbox":               emailOutbox.Stats(),
			"emailOutboxBackend":        emailOutbox.Backend(),
			"emailDeliveryProvider":     configuredAccountEmailDeliveryProvider(),
			"emailDeliveryEnabled":      configuredAccountEmailDeliveryProvider() != "disabled",
			"securityAudit":             securityAudit.Stats(),
			"securityAuditBackend":      securityAudit.Backend(),
			"guests":                    guests.Stats(),
			"guestStoreBackend":         guests.Backend(),
		})
	})

	mux.HandleFunc("/api/platform/guest-sessions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			GuestID       string `json:"guestId"`
			SessionSecret string `json:"sessionSecret"`
			SessionToken  string `json:"sessionToken"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			_ = json.NewDecoder(r.Body).Decode(&payload)
		}
		var session platform.GuestSession
		var err error
		switch {
		case strings.TrimSpace(payload.GuestID) != "" && strings.TrimSpace(payload.SessionToken) != "":
			session, err = guests.ResumeGuestByToken(payload.GuestID, payload.SessionToken)
			if err == platform.ErrUnauthorizedGuestSession && strings.TrimSpace(payload.SessionSecret) != "" {
				session, err = guests.EnsureGuest(payload.GuestID, payload.SessionSecret)
			}
		default:
			session, err = guests.EnsureGuest(payload.GuestID, payload.SessionSecret)
		}
		if err != nil {
			if err == platform.ErrUnauthorizedGuestSession {
				http.Error(w, `{"error":"unauthorized guest session"}`, http.StatusUnauthorized)
				return
			}
			http.Error(w, `{"error":"failed to create guest session"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(session)
	})

	mux.HandleFunc("/api/platform/match-claims", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			MatchID       string `json:"matchId"`
			GuestID       string `json:"guestId"`
			SessionSecret string `json:"sessionSecret"`
			SessionToken  string `json:"sessionToken"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid match claim payload"}`, http.StatusBadRequest)
				return
			}
		}

		session, err := resumeGuestFromPayload(guests, payload.GuestID, payload.SessionSecret, payload.SessionToken)
		if err != nil {
			switch err {
			case platform.ErrUnauthorizedGuestSession:
				http.Error(w, `{"error":"unauthorized guest session"}`, http.StatusUnauthorized)
			case os.ErrNotExist:
				http.Error(w, `{"error":"unknown guest"}`, http.StatusNotFound)
			default:
				http.Error(w, `{"error":"failed to resume guest session"}`, http.StatusBadRequest)
			}
			return
		}

		if claim, ok := claims.Get(payload.MatchID, session.Guest.GuestID); ok {
			if strings.TrimSpace(claim.PlayerSecret) == "" {
				claim.PlayerSecret = session.SessionSecret
			}
			if err := claims.Put(claim); err == nil {
				if renewedClaim, renewed := claims.Get(payload.MatchID, session.Guest.GuestID); renewed {
					claim = renewedClaim
				}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(claim)
			return
		}

		matchState, _, ok := archive.LoadMatch(payload.MatchID)
		if !ok {
			http.Error(w, `{"error":"unknown match archive"}`, http.StatusNotFound)
			return
		}

		seatColor := ""
		playerSecret := ""
		switch session.Guest.GuestID {
		case matchState.WhiteGuestID:
			seatColor = "white"
			playerSecret = matchState.WhitePlayerSecret
		case matchState.BlackGuestID:
			seatColor = "black"
			playerSecret = matchState.BlackPlayerSecret
		default:
			http.Error(w, `{"error":"guest does not own a seat in this match"}`, http.StatusForbidden)
			return
		}
		if strings.TrimSpace(playerSecret) == "" {
			playerSecret = session.SessionSecret
		}

		claim := platform.MatchSeatClaim{
			MatchID:      matchState.MatchID,
			GuestID:      session.Guest.GuestID,
			SeatColor:    seatColor,
			PlayerID:     session.Guest.GuestID,
			PlayerSecret: playerSecret,
			Queue:        matchState.Queue,
			ModeID:       matchState.ModeID,
			WhiteGuestID: matchState.WhiteGuestID,
			BlackGuestID: matchState.BlackGuestID,
			WhiteName:    matchState.WhiteName,
			BlackName:    matchState.BlackName,
			Status:       matchState.Status,
		}
		if err := claims.Put(claim); err != nil {
			http.Error(w, `{"error":"failed to cache match claim"}`, http.StatusInternalServerError)
			return
		}
		if storedClaim, ok := claims.Get(claim.MatchID, claim.GuestID); ok {
			claim = storedClaim
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(claim)
	})

	mux.HandleFunc("/api/platform/match-claims/resolve", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			MatchID    string `json:"matchId"`
			ClaimToken string `json:"claimToken"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid match claim resolve payload"}`, http.StatusBadRequest)
				return
			}
		}
		if strings.TrimSpace(payload.MatchID) == "" || strings.TrimSpace(payload.ClaimToken) == "" {
			http.Error(w, `{"error":"matchId and claimToken are required"}`, http.StatusBadRequest)
			return
		}

		claim, ok := claims.GetByToken(payload.MatchID, payload.ClaimToken)
		if !ok {
			http.Error(w, `{"error":"unknown room claim token"}`, http.StatusNotFound)
			return
		}
		if err := claims.Put(claim); err == nil {
			if renewedClaim, renewed := claims.Get(payload.MatchID, claim.GuestID); renewed {
				claim = renewedClaim
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(claim)
	})

	mux.HandleFunc("/api/platform/accounts/claim", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			GuestID       string `json:"guestId"`
			SessionSecret string `json:"sessionSecret"`
			SessionToken  string `json:"sessionToken"`
			Handle        string `json:"handle"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid account claim payload"}`, http.StatusBadRequest)
				return
			}
		}

		session, err := resumeGuestFromPayload(guests, payload.GuestID, payload.SessionSecret, payload.SessionToken)
		if err != nil {
			switch err {
			case platform.ErrUnauthorizedGuestSession:
				http.Error(w, `{"error":"unauthorized guest session"}`, http.StatusUnauthorized)
			case os.ErrNotExist:
				http.Error(w, `{"error":"unknown guest"}`, http.StatusNotFound)
			case os.ErrInvalid:
				http.Error(w, `{"error":"guestId is required"}`, http.StatusBadRequest)
			default:
				http.Error(w, `{"error":"failed to resume guest session"}`, http.StatusBadRequest)
			}
			return
		}

		accountSession, err := accounts.ClaimGuest(session.Guest, payload.Handle)
		if err != nil {
			switch err {
			case platform.ErrInvalidAccountHandle:
				http.Error(w, `{"error":"invalid account handle"}`, http.StatusBadRequest)
			case platform.ErrAccountHandleTaken:
				http.Error(w, `{"error":"account handle already taken"}`, http.StatusConflict)
			default:
				http.Error(w, `{"error":"failed to claim account"}`, http.StatusInternalServerError)
			}
			return
		}
		recordAccountSecurityEvent(securityAudit, accountSession.Account.AccountID, platform.AccountSecurityEventKindAccountClaimed, accountSession.Account.Handle)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(accountSession)
	})

	mux.HandleFunc("/api/platform/account-sessions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID    string `json:"accountId"`
			SessionToken string `json:"sessionToken"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid account session payload"}`, http.StatusBadRequest)
				return
			}
		}

		session, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken)
		if !ok {
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(session)
	})

	mux.HandleFunc("/api/platform/account-sessions/overview", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID    string `json:"accountId"`
			SessionToken string `json:"sessionToken"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid account session overview payload"}`, http.StatusBadRequest)
				return
			}
		}

		if _, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken); !ok {
			return
		}
		overview, err := accounts.ListAccountSessions(payload.AccountID, payload.SessionToken)
		if err != nil {
			writeAccountSessionError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(overview)
	})

	mux.HandleFunc("/api/platform/account-security/overview", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID    string `json:"accountId"`
			SessionToken string `json:"sessionToken"`
			Limit        int    `json:"limit"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid account security overview payload"}`, http.StatusBadRequest)
				return
			}
		}
		if _, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken); !ok {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(securityAudit.ListOverview(payload.AccountID, payload.Limit))
	})

	mux.HandleFunc("/api/platform/account-sessions/revoke", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID    string `json:"accountId"`
			SessionToken string `json:"sessionToken"`
			RevokeToken  string `json:"revokeToken"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid account session revoke payload"}`, http.StatusBadRequest)
				return
			}
		}
		if strings.TrimSpace(payload.RevokeToken) == "" {
			http.Error(w, `{"error":"revokeToken is required"}`, http.StatusBadRequest)
			return
		}
		if _, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken); !ok {
			return
		}
		if err := accounts.RevokeAccountSession(payload.AccountID, payload.SessionToken, payload.RevokeToken); err != nil {
			writeAccountSessionError(w, err)
			return
		}
		recordAccountSecurityEvent(securityAudit, payload.AccountID, platform.AccountSecurityEventKindSessionRevoked, sessionTokenFingerprint(payload.RevokeToken))
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/api/platform/account-sessions/revoke-others", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID    string `json:"accountId"`
			SessionToken string `json:"sessionToken"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid account session revoke-others payload"}`, http.StatusBadRequest)
				return
			}
		}
		if _, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken); !ok {
			return
		}
		overview, err := accounts.ListAccountSessions(payload.AccountID, payload.SessionToken)
		if err != nil {
			writeAccountSessionError(w, err)
			return
		}
		if err := accounts.RevokeOtherAccountSessions(payload.AccountID, payload.SessionToken); err != nil {
			writeAccountSessionError(w, err)
			return
		}
		revokedCount := len(overview.Sessions) - 1
		if revokedCount < 0 {
			revokedCount = 0
		}
		recordAccountSecurityEvent(securityAudit, payload.AccountID, platform.AccountSecurityEventKindOtherSessionsRevoked, fmt.Sprintf("%d", revokedCount))
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/api/platform/account-presence", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID    string `json:"accountId"`
			SessionToken string `json:"sessionToken"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid account presence payload"}`, http.StatusBadRequest)
				return
			}
		}

		if _, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken); !ok {
			return
		}
		session, err := accounts.TouchPresence(payload.AccountID, payload.SessionToken)
		if err != nil {
			writeAccountSessionError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(session)
	})

	mux.HandleFunc("/api/platform/inbox/overview", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID    string `json:"accountId"`
			SessionToken string `json:"sessionToken"`
			Limit        int    `json:"limit"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid inbox overview payload"}`, http.StatusBadRequest)
				return
			}
		}
		session, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken)
		if !ok {
			return
		}
		respondNotificationOverview(w, guests, accounts, notifications, session.Account.AccountID, payload.Limit)
	})

	mux.HandleFunc("/api/platform/inbox/read", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID      string `json:"accountId"`
			SessionToken   string `json:"sessionToken"`
			NotificationID string `json:"notificationId"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid inbox read payload"}`, http.StatusBadRequest)
				return
			}
		}
		session, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken)
		if !ok {
			return
		}
		if _, err := notifications.MarkRead(session.Account.AccountID, payload.NotificationID); err != nil {
			writeNotificationError(w, err)
			return
		}
		respondNotificationOverview(w, guests, accounts, notifications, session.Account.AccountID, 48)
	})

	mux.HandleFunc("/api/platform/inbox/read-all", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID    string `json:"accountId"`
			SessionToken string `json:"sessionToken"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid inbox read-all payload"}`, http.StatusBadRequest)
				return
			}
		}
		session, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken)
		if !ok {
			return
		}
		if _, err := notifications.MarkAllRead(session.Account.AccountID); err != nil {
			writeNotificationError(w, err)
			return
		}
		respondNotificationOverview(w, guests, accounts, notifications, session.Account.AccountID, 48)
	})

	mux.HandleFunc("/api/platform/inbox/stream", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID    string `json:"accountId"`
			SessionToken string `json:"sessionToken"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid inbox stream payload"}`, http.StatusBadRequest)
				return
			}
		}
		session, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken)
		if !ok {
			return
		}
		flusher, streamOK := w.(http.Flusher)
		if !streamOK {
			http.Error(w, `{"error":"streaming unsupported"}`, http.StatusInternalServerError)
			return
		}
		events, cancel := notifications.Subscribe(session.Account.AccountID, 32)
		defer cancel()

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		_, _ = fmt.Fprint(w, ": connected\n\n")
		flusher.Flush()

		heartbeat := time.NewTicker(25 * time.Second)
		defer heartbeat.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case event, ok := <-events:
				if !ok {
					return
				}
				if err := writeAccountNotificationStreamEvent(w, flusher, event); err != nil {
					return
				}
			case <-heartbeat.C:
				_, _ = fmt.Fprint(w, ": keep-alive\n\n")
				flusher.Flush()
			}
		}
	})

	mux.HandleFunc("/api/platform/account-auth/credentials", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID    string `json:"accountId"`
			SessionToken string `json:"sessionToken"`
			Email        string `json:"email"`
			Password     string `json:"password"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid account auth payload"}`, http.StatusBadRequest)
				return
			}
		}
		if allowed, retryAfter := authThrottle.allowCredentialSetup(r, payload.AccountID); !allowed {
			writeAuthRateLimitError(w, retryAfter)
			return
		}
		if _, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken); !ok {
			return
		}

		session, err := accounts.EnablePasswordLogin(payload.AccountID, payload.SessionToken, payload.Email, payload.Password)
		if err != nil {
			switch err {
			case platform.ErrUnauthorizedAccountSession:
				http.Error(w, `{"error":"unauthorized account session"}`, http.StatusUnauthorized)
			case platform.ErrInvalidAccountEmail:
				http.Error(w, `{"error":"invalid account email"}`, http.StatusBadRequest)
			case platform.ErrAccountEmailTaken:
				http.Error(w, `{"error":"account email already taken"}`, http.StatusConflict)
			case platform.ErrInvalidAccountPassword:
				http.Error(w, `{"error":"invalid account password"}`, http.StatusBadRequest)
			case os.ErrNotExist:
				http.Error(w, `{"error":"unknown account"}`, http.StatusNotFound)
			case os.ErrInvalid:
				http.Error(w, `{"error":"accountId is required"}`, http.StatusBadRequest)
			default:
				http.Error(w, `{"error":"failed to enable account login"}`, http.StatusInternalServerError)
			}
			return
		}
		recordAccountSecurityEvent(securityAudit, session.Account.AccountID, platform.AccountSecurityEventKindPasswordLoginEnabled, payload.Email)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(session)
	})

	mux.HandleFunc("/api/platform/account-auth/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			Handle        string `json:"handle"`
			Email         string `json:"email"`
			Password      string `json:"password"`
			GuestID       string `json:"guestId"`
			SessionSecret string `json:"sessionSecret"`
			SessionToken  string `json:"sessionToken"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid account registration payload"}`, http.StatusBadRequest)
				return
			}
		}
		if allowed, retryAfter := authThrottle.allowRegistration(r, payload.Handle, payload.Email); !allowed {
			writeAuthRateLimitError(w, retryAfter)
			return
		}

		var guestSession platform.GuestSession
		var err error
		if strings.TrimSpace(payload.GuestID) != "" || strings.TrimSpace(payload.SessionSecret) != "" || strings.TrimSpace(payload.SessionToken) != "" {
			guestSession, err = resumeGuestFromPayload(guests, payload.GuestID, payload.SessionSecret, payload.SessionToken)
		} else {
			guestSession, err = guests.EnsureGuest("", "")
		}
		if err != nil {
			switch err {
			case platform.ErrUnauthorizedGuestSession:
				http.Error(w, `{"error":"unauthorized guest session"}`, http.StatusUnauthorized)
			case os.ErrNotExist:
				http.Error(w, `{"error":"unknown guest session"}`, http.StatusNotFound)
			default:
				http.Error(w, `{"error":"failed to restore guest session"}`, http.StatusInternalServerError)
			}
			return
		}

		accountSession, err := accounts.RegisterGuestAccount(guestSession.Guest, payload.Handle, payload.Email, payload.Password)
		if err != nil {
			writeAccountAuthError(w, err)
			return
		}
		if restriction, restricted := moderation.GetAccountRestriction(accountSession.Account.AccountID); restricted {
			_ = accounts.LogoutAccount(accountSession.Account.AccountID, accountSession.SessionToken)
			writeAccountRestrictionError(w, restriction)
			return
		}
		recordAccountSecurityEvent(securityAudit, accountSession.Account.AccountID, platform.AccountSecurityEventKindAccountClaimed, accountSession.Account.Handle)
		recordAccountSecurityEvent(securityAudit, accountSession.Account.AccountID, platform.AccountSecurityEventKindPasswordLoginEnabled, strings.TrimSpace(payload.Email))

		var (
			overview        platform.AccountAuthOverview
			delivery        *platform.AccountEmailDelivery
			previewToken    string
			verificationExp time.Time
		)
		challenge, verificationErr := accounts.StartEmailVerification(accountSession.Account.AccountID, accountSession.SessionToken)
		if verificationErr == nil {
			accountProfile, _ := accounts.GetAccount(accountSession.Account.AccountID)
			if queued, queueErr := emailOutbox.QueueDelivery(platform.BuildAccountEmailVerificationDelivery(accountProfile, challenge, accountAuthPublicBaseURL())); queueErr == nil {
				delivery = &queued
			}
			recordAccountSecurityEvent(securityAudit, accountSession.Account.AccountID, platform.AccountSecurityEventKindEmailVerificationRequested, challenge.Email)
			previewToken = challenge.Token
			verificationExp = challenge.ExpiresAt
		} else if verificationErr != platform.ErrAccountEmailAlreadyVerified {
			writeAccountAuthError(w, verificationErr)
			return
		}

		overview, err = accounts.GetAccountAuthOverview(accountSession.Account.AccountID, accountSession.SessionToken)
		if err != nil {
			writeAccountAuthError(w, err)
			return
		}

		response := map[string]any{
			"account":  accountSession,
			"guest":    guestSession,
			"overview": overview,
		}
		if delivery != nil {
			response["delivery"] = delivery
		}
		if !verificationExp.IsZero() {
			response["requestedVerification"] = true
			response["expiresAt"] = verificationExp
		}
		if accountAuthPreviewEnabled() && strings.TrimSpace(previewToken) != "" {
			response["previewToken"] = previewToken
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	})

	mux.HandleFunc("/api/platform/account-auth/overview", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID    string `json:"accountId"`
			SessionToken string `json:"sessionToken"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid account auth overview payload"}`, http.StatusBadRequest)
				return
			}
		}

		if _, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken); !ok {
			return
		}
		overview, err := accounts.GetAccountAuthOverview(payload.AccountID, payload.SessionToken)
		if err != nil {
			writeAccountAuthError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(overview)
	})

	mux.HandleFunc("/api/platform/email-outbox/overview", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID    string `json:"accountId"`
			SessionToken string `json:"sessionToken"`
			Limit        int    `json:"limit"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid email outbox overview payload"}`, http.StatusBadRequest)
				return
			}
		}
		if _, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken); !ok {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(emailOutbox.ListOverview(payload.AccountID, payload.Limit))
	})

	mux.HandleFunc("/api/platform/account-auth/email-verification/request", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID    string `json:"accountId"`
			SessionToken string `json:"sessionToken"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid account verification payload"}`, http.StatusBadRequest)
				return
			}
		}
		if allowed, retryAfter := authThrottle.allowEmailVerification(r, payload.AccountID); !allowed {
			writeAuthRateLimitError(w, retryAfter)
			return
		}
		if _, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken); !ok {
			return
		}
		challenge, err := accounts.StartEmailVerification(payload.AccountID, payload.SessionToken)
		if err != nil {
			writeAccountAuthError(w, err)
			return
		}
		accountProfile, _ := accounts.GetAccount(payload.AccountID)
		delivery, err := emailOutbox.QueueDelivery(platform.BuildAccountEmailVerificationDelivery(accountProfile, challenge, accountAuthPublicBaseURL()))
		if err != nil {
			http.Error(w, `{"error":"failed to queue account verification email"}`, http.StatusInternalServerError)
			return
		}
		overview, err := accounts.GetAccountAuthOverview(payload.AccountID, payload.SessionToken)
		if err != nil {
			writeAccountAuthError(w, err)
			return
		}
		response := map[string]any{
			"overview":  overview,
			"requested": true,
			"email":     challenge.Email,
			"expiresAt": challenge.ExpiresAt,
			"delivery":  delivery,
		}
		recordAccountSecurityEvent(securityAudit, payload.AccountID, platform.AccountSecurityEventKindEmailVerificationRequested, challenge.Email)
		if accountAuthPreviewEnabled() {
			response["previewToken"] = challenge.Token
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	})

	mux.HandleFunc("/api/platform/account-auth/email-verification/confirm", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID string `json:"accountId"`
			Token     string `json:"token"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid email verification confirmation payload"}`, http.StatusBadRequest)
				return
			}
		}
		if allowed, retryAfter := authThrottle.allowEmailVerification(r, payload.AccountID); !allowed {
			writeAuthRateLimitError(w, retryAfter)
			return
		}
		overview, err := accounts.VerifyEmail(payload.AccountID, payload.Token)
		if err != nil {
			writeAccountAuthError(w, err)
			return
		}
		recordAccountSecurityEvent(securityAudit, overview.AccountID, platform.AccountSecurityEventKindEmailVerified, overview.Email)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(overview)
	})

	mux.HandleFunc("/api/platform/account-auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			Identifier string `json:"identifier"`
			Password   string `json:"password"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid account login payload"}`, http.StatusBadRequest)
				return
			}
		}
		if allowed, retryAfter := authThrottle.allowLogin(r, payload.Identifier); !allowed {
			writeAuthRateLimitError(w, retryAfter)
			return
		}

		accountSession, err := accounts.LoginWithPassword(payload.Identifier, payload.Password)
		if err != nil {
			switch err {
			case platform.ErrUnauthorizedAccountCredentials:
				http.Error(w, `{"error":"unauthorized account credentials"}`, http.StatusUnauthorized)
			default:
				http.Error(w, `{"error":"failed to sign in"}`, http.StatusInternalServerError)
			}
			return
		}
		if restriction, restricted := moderation.GetAccountRestriction(accountSession.Account.AccountID); restricted {
			_ = accounts.LogoutAccount(accountSession.Account.AccountID, accountSession.SessionToken)
			writeAccountRestrictionError(w, restriction)
			return
		}
		recordAccountSecurityEvent(securityAudit, accountSession.Account.AccountID, platform.AccountSecurityEventKindPasswordLoginSucceeded, accountSession.Account.Handle)
		guestSession, err := guests.IssueGuestSession(resolvePrimaryGuestID(accountSession.Account))
		if err != nil {
			http.Error(w, `{"error":"failed to restore account guest session"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"account": accountSession,
			"guest":   guestSession,
		})
	})

	mux.HandleFunc("/api/platform/account-auth/password-reset/request", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			Identifier string `json:"identifier"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid password reset request payload"}`, http.StatusBadRequest)
				return
			}
		}
		if allowed, retryAfter := authThrottle.allowPasswordReset(r, payload.Identifier); !allowed {
			writeAuthRateLimitError(w, retryAfter)
			return
		}
		challenge, err := accounts.StartPasswordReset(payload.Identifier)
		if err != nil {
			writeAccountAuthError(w, err)
			return
		}
		var delivery *platform.AccountEmailDelivery
		if strings.TrimSpace(challenge.Token) != "" {
			accountProfile, _ := accounts.GetAccount(challenge.AccountID)
			queued, queueErr := emailOutbox.QueueDelivery(platform.BuildAccountPasswordResetDelivery(accountProfile, challenge, accountAuthPublicBaseURL()))
			if queueErr == nil {
				delivery = &queued
			}
		}
		response := map[string]any{
			"requested": challenge.Requested,
		}
		if delivery != nil {
			response["delivery"] = delivery
		}
		if challenge.Requested && strings.TrimSpace(challenge.AccountID) != "" {
			recordAccountSecurityEvent(securityAudit, challenge.AccountID, platform.AccountSecurityEventKindPasswordResetRequested, challenge.Email)
		}
		if accountAuthPreviewEnabled() && strings.TrimSpace(challenge.Token) != "" {
			response["previewToken"] = challenge.Token
			response["previewAccountId"] = challenge.AccountID
			response["email"] = challenge.Email
			response["expiresAt"] = challenge.ExpiresAt
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	})

	mux.HandleFunc("/api/platform/account-auth/password-reset/confirm", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID string `json:"accountId"`
			Token     string `json:"token"`
			Password  string `json:"password"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid password reset confirmation payload"}`, http.StatusBadRequest)
				return
			}
		}
		if allowed, retryAfter := authThrottle.allowPasswordReset(r, payload.AccountID); !allowed {
			writeAuthRateLimitError(w, retryAfter)
			return
		}

		accountSession, err := accounts.ResetPassword(payload.AccountID, payload.Token, payload.Password)
		if err != nil {
			writeAccountAuthError(w, err)
			return
		}
		recordAccountSecurityEvent(securityAudit, accountSession.Account.AccountID, platform.AccountSecurityEventKindPasswordResetCompleted, accountSession.Account.Handle)
		guestSession, err := guests.IssueGuestSession(resolvePrimaryGuestID(accountSession.Account))
		if err != nil {
			http.Error(w, `{"error":"failed to restore account guest session"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"account": accountSession,
			"guest":   guestSession,
		})
	})

	mux.HandleFunc("/api/platform/account-auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID    string `json:"accountId"`
			SessionToken string `json:"sessionToken"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid account logout payload"}`, http.StatusBadRequest)
				return
			}
		}

		if err := accounts.LogoutAccount(payload.AccountID, payload.SessionToken); err != nil {
			switch err {
			case platform.ErrUnauthorizedAccountSession:
				http.Error(w, `{"error":"unauthorized account session"}`, http.StatusUnauthorized)
			case os.ErrNotExist:
				http.Error(w, `{"error":"unknown account"}`, http.StatusNotFound)
			case os.ErrInvalid:
				http.Error(w, `{"error":"accountId is required"}`, http.StatusBadRequest)
			default:
				http.Error(w, `{"error":"failed to sign out"}`, http.StatusInternalServerError)
			}
			return
		}
		recordAccountSecurityEvent(securityAudit, payload.AccountID, platform.AccountSecurityEventKindSessionSignedOut, sessionTokenFingerprint(payload.SessionToken))

		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/api/platform/friends/overview", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID    string `json:"accountId"`
			SessionToken string `json:"sessionToken"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid friend overview payload"}`, http.StatusBadRequest)
				return
			}
		}

		session, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken)
		if !ok {
			return
		}

		respondFriendOverview(w, guests, accounts, friends, session.Account.AccountID)
	})

	mux.HandleFunc("/api/platform/friends/requests", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID    string `json:"accountId"`
			SessionToken string `json:"sessionToken"`
			TargetHandle string `json:"targetHandle"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid friend request payload"}`, http.StatusBadRequest)
				return
			}
		}

		session, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken)
		if !ok {
			return
		}
		target, ok := findAccountByHandle(accounts, payload.TargetHandle)
		if !ok {
			http.Error(w, `{"error":"unknown target handle"}`, http.StatusNotFound)
			return
		}
		if err := requireAccountInteractionAllowed(moderation, session.Account.AccountID, target.AccountID); err != nil {
			writeModerationError(w, err)
			return
		}
		request, err := friends.SendRequest(session.Account.AccountID, target.AccountID)
		if err != nil {
			switch err {
			case platform.ErrInvalidFriendRequest:
				http.Error(w, `{"error":"invalid friend request"}`, http.StatusBadRequest)
			case platform.ErrAlreadyFriends:
				http.Error(w, `{"error":"accounts are already friends"}`, http.StatusConflict)
			case platform.ErrFriendRequestAlreadyExists:
				http.Error(w, `{"error":"friend request already exists"}`, http.StatusConflict)
			default:
				http.Error(w, `{"error":"failed to send friend request"}`, http.StatusInternalServerError)
			}
			return
		}
		notificationKind := platform.AccountNotificationKindFriendRequestReceived
		if request.Status == platform.FriendRequestStatusAccepted {
			notificationKind = platform.AccountNotificationKindFriendRequestAccepted
		}
		if _, err := notifications.CreateNotification(target.AccountID, session.Account.AccountID, notificationKind, platform.AccountNotificationOptions{
			FriendRequestID: request.RequestID,
		}); err != nil {
			log.Printf("failed to create friend request notification for %s -> %s: %v", session.Account.AccountID, target.AccountID, err)
		}

		respondFriendOverview(w, guests, accounts, friends, session.Account.AccountID)
	})

	mux.HandleFunc("/api/platform/friends/requests/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !strings.HasSuffix(r.URL.Path, "/respond") {
			http.NotFound(w, r)
			return
		}
		requestID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/platform/friends/requests/"), "/respond")
		if strings.TrimSpace(requestID) == "" {
			http.NotFound(w, r)
			return
		}
		var payload struct {
			AccountID    string `json:"accountId"`
			SessionToken string `json:"sessionToken"`
			Accept       bool   `json:"accept"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid friend response payload"}`, http.StatusBadRequest)
				return
			}
		}

		session, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken)
		if !ok {
			return
		}
		requestOverview := friends.ListOverview(session.Account.AccountID)
		requestTargetAccountID := findFriendRequestCounterpartyAccountID(requestOverview, requestID, session.Account.AccountID)
		if requestTargetAccountID == "" {
			http.Error(w, `{"error":"friend request not found"}`, http.StatusNotFound)
			return
		}
		if err := requireAccountInteractionAllowed(moderation, session.Account.AccountID, requestTargetAccountID); err != nil {
			writeModerationError(w, err)
			return
		}
		request, err := friends.RespondToRequest(session.Account.AccountID, requestID, payload.Accept)
		if err != nil {
			switch err {
			case platform.ErrFriendRequestNotFound:
				http.Error(w, `{"error":"friend request not found"}`, http.StatusNotFound)
			case platform.ErrUnauthorizedFriendRequest:
				http.Error(w, `{"error":"unauthorized friend request"}`, http.StatusForbidden)
			case platform.ErrInvalidFriendRequest:
				http.Error(w, `{"error":"invalid friend request"}`, http.StatusBadRequest)
			default:
				http.Error(w, `{"error":"failed to update friend request"}`, http.StatusInternalServerError)
			}
			return
		}
		if payload.Accept {
			if _, err := notifications.CreateNotification(requestTargetAccountID, session.Account.AccountID, platform.AccountNotificationKindFriendRequestAccepted, platform.AccountNotificationOptions{
				FriendRequestID: request.RequestID,
			}); err != nil {
				log.Printf("failed to create friend acceptance notification for %s -> %s: %v", session.Account.AccountID, requestTargetAccountID, err)
			}
		}

		respondFriendOverview(w, guests, accounts, friends, session.Account.AccountID)
	})

	mux.HandleFunc("/api/platform/friends/remove", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID       string `json:"accountId"`
			SessionToken    string `json:"sessionToken"`
			FriendAccountID string `json:"friendAccountId"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid remove friend payload"}`, http.StatusBadRequest)
				return
			}
		}

		session, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken)
		if !ok {
			return
		}
		if err := friends.RemoveFriend(session.Account.AccountID, payload.FriendAccountID); err != nil {
			switch err {
			case platform.ErrInvalidFriendRequest:
				http.Error(w, `{"error":"invalid friend removal"}`, http.StatusBadRequest)
			default:
				http.Error(w, `{"error":"failed to remove friend"}`, http.StatusInternalServerError)
			}
			return
		}

		respondFriendOverview(w, guests, accounts, friends, session.Account.AccountID)
	})

	mux.HandleFunc("/api/platform/moderation/overview", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID    string `json:"accountId"`
			SessionToken string `json:"sessionToken"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid moderation overview payload"}`, http.StatusBadRequest)
				return
			}
		}

		session, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken)
		if !ok {
			return
		}
		respondModerationOverview(w, guests, accounts, moderation, session.Account.AccountID)
	})

	mux.HandleFunc("/api/platform/moderation/blocks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID       string `json:"accountId"`
			SessionToken    string `json:"sessionToken"`
			TargetAccountID string `json:"targetAccountId"`
			Reason          string `json:"reason"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid account block payload"}`, http.StatusBadRequest)
				return
			}
		}

		session, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken)
		if !ok {
			return
		}
		target, ok := accounts.GetAccount(strings.TrimSpace(payload.TargetAccountID))
		if !ok {
			http.Error(w, `{"error":"unknown target account"}`, http.StatusNotFound)
			return
		}
		if _, err := moderation.BlockAccount(session.Account.AccountID, target.AccountID, payload.Reason); err != nil {
			writeModerationError(w, err)
			return
		}
		if err := friends.RemoveFriend(session.Account.AccountID, target.AccountID); err != nil && err != platform.ErrInvalidFriendRequest {
			http.Error(w, `{"error":"failed to remove social connection"}`, http.StatusInternalServerError)
			return
		}
		if err := challenges.PurgePair(session.Account.AccountID, target.AccountID); err != nil && err != platform.ErrInvalidDirectChallenge {
			http.Error(w, `{"error":"failed to purge pending direct challenges"}`, http.StatusInternalServerError)
			return
		}
		if err := notifications.PurgePair(session.Account.AccountID, target.AccountID); err != nil && err != platform.ErrInvalidAccountNotification {
			log.Printf("failed to purge notifications for blocked pair %s/%s: %v", session.Account.AccountID, target.AccountID, err)
		}
		respondModerationOverview(w, guests, accounts, moderation, session.Account.AccountID)
	})

	mux.HandleFunc("/api/platform/moderation/blocks/remove", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID       string `json:"accountId"`
			SessionToken    string `json:"sessionToken"`
			TargetAccountID string `json:"targetAccountId"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid account unblock payload"}`, http.StatusBadRequest)
				return
			}
		}

		session, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken)
		if !ok {
			return
		}
		if err := moderation.UnblockAccount(session.Account.AccountID, payload.TargetAccountID); err != nil {
			writeModerationError(w, err)
			return
		}
		respondModerationOverview(w, guests, accounts, moderation, session.Account.AccountID)
	})

	mux.HandleFunc("/api/platform/moderation/reports", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID       string `json:"accountId"`
			SessionToken    string `json:"sessionToken"`
			TargetAccountID string `json:"targetAccountId"`
			Category        string `json:"category"`
			Details         string `json:"details"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid player report payload"}`, http.StatusBadRequest)
				return
			}
		}

		session, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken)
		if !ok {
			return
		}
		target, ok := accounts.GetAccount(strings.TrimSpace(payload.TargetAccountID))
		if !ok {
			http.Error(w, `{"error":"unknown target account"}`, http.StatusNotFound)
			return
		}
		if _, err := moderation.CreateReport(session.Account.AccountID, target.AccountID, payload.Category, payload.Details); err != nil {
			writeModerationError(w, err)
			return
		}
		respondModerationOverview(w, guests, accounts, moderation, session.Account.AccountID)
	})

	mux.HandleFunc("/api/platform/moderation/admin/overview", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID    string `json:"accountId"`
			SessionToken string `json:"sessionToken"`
			Limit        int    `json:"limit"`
			Status       string `json:"status"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid moderation admin overview payload"}`, http.StatusBadRequest)
				return
			}
		}
		session, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken)
		if !ok {
			return
		}
		if !isModerationAdminAccount(session.Account) {
			writeModerationAdminAuthError(w)
			return
		}
		respondModerationAdminOverview(w, guests, accounts, moderation, session.Account.AccountID, payload.Limit, payload.Status)
	})

	mux.HandleFunc("/api/platform/moderation/admin/reports/resolve", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID    string `json:"accountId"`
			SessionToken string `json:"sessionToken"`
			ReportID     string `json:"reportId"`
			Action       string `json:"action"`
			Restriction  string `json:"restriction"`
			Note         string `json:"note"`
			Limit        int    `json:"limit"`
			Status       string `json:"status"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid moderation admin resolution payload"}`, http.StatusBadRequest)
				return
			}
		}
		session, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken)
		if !ok {
			return
		}
		if !isModerationAdminAccount(session.Account) {
			writeModerationAdminAuthError(w)
			return
		}
		resolvedReport, _, err := moderation.ResolveReport(session.Account.AccountID, payload.ReportID, payload.Action, payload.Note)
		if err != nil {
			writeModerationError(w, err)
			return
		}
		switch strings.TrimSpace(strings.ToLower(payload.Restriction)) {
		case "", "none":
		case "clear":
			if err := moderation.ClearAccountRestriction(resolvedReport.TargetAccountID); err != nil && err != platform.ErrAccountRestrictionNotFound {
				writeModerationError(w, err)
				return
			}
		default:
			if _, err := moderation.SetAccountRestriction(session.Account.AccountID, resolvedReport.TargetAccountID, payload.Restriction, payload.Note, resolvedReport.ReportID); err != nil {
				writeModerationError(w, err)
				return
			}
		}
		recordAccountSecurityEvent(securityAudit, session.Account.AccountID, platform.AccountSecurityEventKindModeratorReviewRecorded, resolvedReport.ReportID)
		respondModerationAdminOverview(w, guests, accounts, moderation, session.Account.AccountID, payload.Limit, payload.Status)
	})

	mux.HandleFunc("/api/platform/challenges/overview", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID    string `json:"accountId"`
			SessionToken string `json:"sessionToken"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid challenge overview payload"}`, http.StatusBadRequest)
				return
			}
		}

		session, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken)
		if !ok {
			return
		}

		respondChallengeOverview(w, guests, accounts, challenges, session.Account.AccountID)
	})

	mux.HandleFunc("/api/platform/challenges/eligibility", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID       string `json:"accountId"`
			SessionToken    string `json:"sessionToken"`
			TargetAccountID string `json:"targetAccountId"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid challenge eligibility payload"}`, http.StatusBadRequest)
				return
			}
		}

		session, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken)
		if !ok {
			return
		}
		target, ok := accounts.GetAccount(strings.TrimSpace(payload.TargetAccountID))
		if !ok {
			http.Error(w, `{"error":"unknown target account"}`, http.StatusNotFound)
			return
		}
		if err := requireAccountInteractionAllowed(moderation, session.Account.AccountID, target.AccountID); err != nil {
			writeModerationError(w, err)
			return
		}
		if !friends.AreFriends(session.Account.AccountID, target.AccountID) {
			http.Error(w, `{"error":"direct challenges require an accepted friendship"}`, http.StatusForbidden)
			return
		}
		if err := challenges.CanCreateChallenge(session.Account.AccountID, target.AccountID); err != nil {
			switch err {
			case platform.ErrInvalidDirectChallenge:
				http.Error(w, `{"error":"invalid direct challenge"}`, http.StatusBadRequest)
			case platform.ErrDirectChallengeAlreadyExists:
				http.Error(w, `{"error":"a pending direct challenge already exists for this friend"}`, http.StatusConflict)
			default:
				http.Error(w, `{"error":"failed to validate direct challenge"}`, http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"viewer": platform.BuildPublicAccountProfile(session.Account, guests),
			"target": platform.BuildPublicAccountProfile(target, guests),
		})
	})

	mux.HandleFunc("/api/platform/challenges", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AccountID       string                `json:"accountId"`
			SessionToken    string                `json:"sessionToken"`
			TargetAccountID string                `json:"targetAccountId"`
			MatchID         string                `json:"matchId"`
			ModeID          contracts.MatchModeID `json:"modeId"`
			ClockSeconds    int64                 `json:"clockSeconds"`
			ChallengerSeat  string                `json:"challengerSeat"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid challenge create payload"}`, http.StatusBadRequest)
				return
			}
		}

		session, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken)
		if !ok {
			return
		}
		target, ok := accounts.GetAccount(strings.TrimSpace(payload.TargetAccountID))
		if !ok {
			http.Error(w, `{"error":"unknown target account"}`, http.StatusNotFound)
			return
		}
		if err := requireAccountInteractionAllowed(moderation, session.Account.AccountID, target.AccountID); err != nil {
			writeModerationError(w, err)
			return
		}
		if !friends.AreFriends(session.Account.AccountID, target.AccountID) {
			http.Error(w, `{"error":"direct challenges require an accepted friendship"}`, http.StatusForbidden)
			return
		}
		challenge, err := challenges.CreateChallenge(session.Account.AccountID, target.AccountID, payload.MatchID, payload.ModeID, payload.ClockSeconds, payload.ChallengerSeat)
		if err != nil {
			switch err {
			case platform.ErrInvalidDirectChallenge:
				http.Error(w, `{"error":"invalid direct challenge"}`, http.StatusBadRequest)
			case platform.ErrDirectChallengeAlreadyExists:
				http.Error(w, `{"error":"a pending direct challenge already exists for this friend"}`, http.StatusConflict)
			default:
				http.Error(w, `{"error":"failed to create direct challenge"}`, http.StatusInternalServerError)
			}
			return
		}
		if _, err := notifications.CreateNotification(target.AccountID, session.Account.AccountID, platform.AccountNotificationKindDirectChallengeReceived, platform.AccountNotificationOptions{
			ChallengeID:    challenge.ChallengeID,
			MatchID:        challenge.MatchID,
			ModeID:         challenge.ModeID,
			ChallengerSeat: challenge.ChallengerSeat,
		}); err != nil {
			log.Printf("failed to create direct challenge notification for %s -> %s: %v", session.Account.AccountID, target.AccountID, err)
		}

		writeDirectChallengeView(w, guests, accounts, challenge, session.Account.AccountID)
	})

	mux.HandleFunc("/api/platform/challenges/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/platform/challenges/")
		if strings.TrimSpace(path) == "" {
			http.NotFound(w, r)
			return
		}
		parts := strings.Split(path, "/")
		if len(parts) != 2 {
			http.NotFound(w, r)
			return
		}
		challengeID := strings.TrimSpace(parts[0])
		action := strings.TrimSpace(parts[1])
		if challengeID == "" {
			http.NotFound(w, r)
			return
		}

		switch action {
		case "view":
			var payload struct {
				AccountID    string `json:"accountId"`
				SessionToken string `json:"sessionToken"`
			}
			if r.Body != nil {
				defer r.Body.Close()
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					http.Error(w, `{"error":"invalid challenge view payload"}`, http.StatusBadRequest)
					return
				}
			}
			session, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken)
			if !ok {
				return
			}
			challenge, ok := challenges.GetChallenge(challengeID)
			if !ok {
				http.Error(w, `{"error":"direct challenge not found"}`, http.StatusNotFound)
				return
			}
			if challenge.ChallengerAccountID != session.Account.AccountID && challenge.TargetAccountID != session.Account.AccountID {
				http.Error(w, `{"error":"unauthorized direct challenge"}`, http.StatusForbidden)
				return
			}
			if err := requireAccountInteractionAllowed(moderation, challenge.ChallengerAccountID, challenge.TargetAccountID); err != nil {
				writeModerationError(w, err)
				return
			}
			writeDirectChallengeView(w, guests, accounts, challenge, session.Account.AccountID)
		case "respond":
			var payload struct {
				AccountID    string `json:"accountId"`
				SessionToken string `json:"sessionToken"`
				Accept       bool   `json:"accept"`
			}
			if r.Body != nil {
				defer r.Body.Close()
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					http.Error(w, `{"error":"invalid challenge response payload"}`, http.StatusBadRequest)
					return
				}
			}
			session, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken)
			if !ok {
				return
			}
			if existingChallenge, ok := challenges.GetChallenge(challengeID); ok {
				if err := requireAccountInteractionAllowed(moderation, existingChallenge.ChallengerAccountID, existingChallenge.TargetAccountID); err != nil {
					writeModerationError(w, err)
					return
				}
			}
			challenge, err := challenges.RespondToChallenge(session.Account.AccountID, challengeID, payload.Accept)
			if err != nil {
				switch err {
				case platform.ErrDirectChallengeNotFound:
					http.Error(w, `{"error":"direct challenge not found"}`, http.StatusNotFound)
				case platform.ErrUnauthorizedDirectChallenge:
					http.Error(w, `{"error":"unauthorized direct challenge"}`, http.StatusForbidden)
				case platform.ErrInvalidDirectChallenge:
					http.Error(w, `{"error":"invalid direct challenge"}`, http.StatusBadRequest)
				case platform.ErrDirectChallengeNotPending:
					http.Error(w, `{"error":"direct challenge is no longer pending"}`, http.StatusConflict)
				default:
					http.Error(w, `{"error":"failed to update direct challenge"}`, http.StatusInternalServerError)
				}
				return
			}
			notificationKind := platform.AccountNotificationKindDirectChallengeDeclined
			if payload.Accept {
				notificationKind = platform.AccountNotificationKindDirectChallengeAccepted
			}
			if _, err := notifications.CreateNotification(challenge.ChallengerAccountID, session.Account.AccountID, notificationKind, platform.AccountNotificationOptions{
				ChallengeID:    challenge.ChallengeID,
				MatchID:        challenge.MatchID,
				ModeID:         challenge.ModeID,
				ChallengerSeat: challenge.ChallengerSeat,
			}); err != nil {
				log.Printf("failed to create challenge response notification for %s -> %s: %v", session.Account.AccountID, challenge.ChallengerAccountID, err)
			}
			writeDirectChallengeView(w, guests, accounts, challenge, session.Account.AccountID)
		case "cancel":
			var payload struct {
				AccountID    string `json:"accountId"`
				SessionToken string `json:"sessionToken"`
			}
			if r.Body != nil {
				defer r.Body.Close()
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					http.Error(w, `{"error":"invalid challenge cancel payload"}`, http.StatusBadRequest)
					return
				}
			}
			session, ok := resumeAllowedAccountSessionOrWrite(w, accounts, moderation, payload.AccountID, payload.SessionToken)
			if !ok {
				return
			}
			if existingChallenge, ok := challenges.GetChallenge(challengeID); ok {
				if err := requireAccountInteractionAllowed(moderation, existingChallenge.ChallengerAccountID, existingChallenge.TargetAccountID); err != nil {
					writeModerationError(w, err)
					return
				}
			}
			challenge, err := challenges.CancelChallenge(session.Account.AccountID, challengeID)
			if err != nil {
				switch err {
				case platform.ErrDirectChallengeNotFound:
					http.Error(w, `{"error":"direct challenge not found"}`, http.StatusNotFound)
				case platform.ErrUnauthorizedDirectChallenge:
					http.Error(w, `{"error":"unauthorized direct challenge"}`, http.StatusForbidden)
				case platform.ErrInvalidDirectChallenge:
					http.Error(w, `{"error":"invalid direct challenge"}`, http.StatusBadRequest)
				case platform.ErrDirectChallengeNotPending:
					http.Error(w, `{"error":"direct challenge is no longer pending"}`, http.StatusConflict)
				default:
					http.Error(w, `{"error":"failed to cancel direct challenge"}`, http.StatusInternalServerError)
				}
				return
			}
			if _, err := notifications.CreateNotification(challenge.TargetAccountID, session.Account.AccountID, platform.AccountNotificationKindDirectChallengeCanceled, platform.AccountNotificationOptions{
				ChallengeID:    challenge.ChallengeID,
				MatchID:        challenge.MatchID,
				ModeID:         challenge.ModeID,
				ChallengerSeat: challenge.ChallengerSeat,
			}); err != nil {
				log.Printf("failed to create challenge cancellation notification for %s -> %s: %v", session.Account.AccountID, challenge.TargetAccountID, err)
			}
			writeDirectChallengeView(w, guests, accounts, challenge, session.Account.AccountID)
		default:
			http.NotFound(w, r)
		}
	})

	mux.HandleFunc("/api/platform/accounts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		limit := platform.ParseListLimit(r.URL.Query().Get("limit"), 24)
		sortMode := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort")))
		seasonID := strings.TrimSpace(r.URL.Query().Get("seasonId"))
		modeID := parseOptionalModeID(r.URL.Query().Get("modeId"))
		query := normalizeAccountQuery(r.URL.Query().Get("query"))
		accountItems := filterAccountsByQuery(accounts.ListAccounts(0), query)
		seasonOptions := platform.BuildAvailableSeasonOptionsForMode(accountItems, modeID)
		accountsList := make([]platform.PublicAccountProfile, 0, len(accountItems))
		for _, account := range accountItems {
			profile := platform.BuildPublicAccountProfileForSeasonAndMode(account, guests, seasonID, modeID)
			if seasonID != "" && profile.SelectedSeason == nil {
				continue
			}
			if modeID != "" && profile.MatchesPlayed == 0 {
				continue
			}
			accountsList = append(accountsList, profile)
		}
		if sortMode == "rating" {
			if seasonID != "" {
				platform.SortPublicAccountsBySelectedSeason(accountsList)
			} else {
				platform.SortPublicAccountsByRating(accountsList)
			}
		}
		if limit > 0 && len(accountsList) > limit {
			accountsList = accountsList[:limit]
		}
		summary := platform.BuildAccountLeaderboardSummary(accountsList, seasonID, modeID)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"accounts":         accountsList,
			"seasons":          seasonOptions,
			"summary":          summary,
			"selectedSeasonId": seasonID,
			"selectedModeId":   modeID,
			"selectedQuery":    query,
		})
	})

	mux.HandleFunc("/api/platform/accounts/by-guest/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		guestID := strings.TrimPrefix(r.URL.Path, "/api/platform/accounts/by-guest/")
		if guestID == "" {
			http.NotFound(w, r)
			return
		}
		account, ok := accounts.GetAccountByGuest(guestID)
		if !ok {
			http.NotFound(w, r)
			return
		}
		seasonID := strings.TrimSpace(r.URL.Query().Get("seasonId"))
		modeID := parseOptionalModeID(r.URL.Query().Get("modeId"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"account": platform.BuildDetailedPublicAccountProfileForSeasonAndMode(account, guests, seasonID, modeID),
		})
	})

	mux.HandleFunc("/api/platform/accounts/by-handle/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handlePath := strings.TrimPrefix(r.URL.Path, "/api/platform/accounts/by-handle/")
		if handlePath == "" {
			http.NotFound(w, r)
			return
		}
		resolvedHandle, err := url.PathUnescape(handlePath)
		if err != nil {
			http.Error(w, `{"error":"invalid account handle"}`, http.StatusBadRequest)
			return
		}
		account, ok := findAccountByHandle(accounts, resolvedHandle)
		if !ok {
			http.NotFound(w, r)
			return
		}
		seasonID := strings.TrimSpace(r.URL.Query().Get("seasonId"))
		modeID := parseOptionalModeID(r.URL.Query().Get("modeId"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"account": platform.BuildDetailedPublicAccountProfileForSeasonAndMode(account, guests, seasonID, modeID),
		})
	})

	mux.HandleFunc("/api/platform/accounts/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		accountID := strings.TrimPrefix(r.URL.Path, "/api/platform/accounts/")
		if accountID == "" {
			http.NotFound(w, r)
			return
		}
		account, ok := accounts.GetAccount(accountID)
		if !ok {
			http.NotFound(w, r)
			return
		}
		seasonID := strings.TrimSpace(r.URL.Query().Get("seasonId"))
		modeID := parseOptionalModeID(r.URL.Query().Get("modeId"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"account": platform.BuildDetailedPublicAccountProfileForSeasonAndMode(account, guests, seasonID, modeID),
		})
	})

	mux.HandleFunc("/api/platform/guest-results", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			MatchID      string `json:"matchId"`
			WhiteGuestID string `json:"whiteGuestId"`
			BlackGuestID string `json:"blackGuestId"`
			Winner       string `json:"winner"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid guest result payload"}`, http.StatusBadRequest)
				return
			}
		}
		entry, ok := archive.Get(payload.MatchID)
		if !ok {
			http.Error(w, `{"error":"unknown match archive"}`, http.StatusBadRequest)
			return
		}
		if err := validateGuestResult(entry, payload.MatchID, payload.WhiteGuestID, payload.BlackGuestID, payload.Winner); err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
			return
		}
		whiteBefore, ok := guests.GetGuest(entry.WhiteGuestID)
		if !ok {
			http.Error(w, `{"error":"unknown white guest"}`, http.StatusBadRequest)
			return
		}
		blackBefore, ok := guests.GetGuest(entry.BlackGuestID)
		if !ok {
			http.Error(w, `{"error":"unknown black guest"}`, http.StatusBadRequest)
			return
		}
		var (
			whiteAccountProfile platform.PublicAccountProfile
			blackAccountProfile platform.PublicAccountProfile
			accountChanged      bool
		)
		_, whiteLinked := accounts.GetAccountByGuest(entry.WhiteGuestID)
		_, blackLinked := accounts.GetAccountByGuest(entry.BlackGuestID)
		if whiteLinked && blackLinked {
			if _, _, changed, err := finalizeLinkedAccounts(accounts, payload.MatchID, entry.WhiteGuestID, entry.BlackGuestID, payload.Winner, entry.Queue, entry.ModeID, whiteBefore, blackBefore); err != nil {
				http.Error(w, `{"error":"failed to finalize linked account result"}`, http.StatusBadRequest)
				return
			} else {
				accountChanged = changed
			}
			if account, ok := accounts.GetAccountByGuest(entry.WhiteGuestID); ok {
				whiteAccountProfile = platform.BuildPublicAccountProfile(account, guests)
			}
			if account, ok := accounts.GetAccountByGuest(entry.BlackGuestID); ok {
				blackAccountProfile = platform.BuildPublicAccountProfile(account, guests)
			}
		}
		white, black, changed, err := guests.FinalizeMatch(payload.MatchID, payload.WhiteGuestID, payload.BlackGuestID, payload.Winner)
		if err != nil {
			http.Error(w, `{"error":"failed to finalize guest result"}`, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"changed":      changed || accountChanged,
			"white":        white,
			"black":        black,
			"whiteAccount": whiteAccountProfile,
			"blackAccount": blackAccountProfile,
		})
	})

	mux.HandleFunc("/api/platform/account-results", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			MatchID        string `json:"matchId"`
			WhiteAccountID string `json:"whiteAccountId"`
			BlackAccountID string `json:"blackAccountId"`
			Winner         string `json:"winner"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"error":"invalid account result payload"}`, http.StatusBadRequest)
				return
			}
		}

		entry, ok := archive.Get(payload.MatchID)
		if !ok {
			http.Error(w, `{"error":"unknown match archive"}`, http.StatusBadRequest)
			return
		}
		entry = enrichArchiveEntry(accounts, entry)

		whiteBefore, ok := guests.GetGuest(entry.WhiteGuestID)
		if !ok {
			http.Error(w, `{"error":"unknown white guest"}`, http.StatusBadRequest)
			return
		}
		blackBefore, ok := guests.GetGuest(entry.BlackGuestID)
		if !ok {
			http.Error(w, `{"error":"unknown black guest"}`, http.StatusBadRequest)
			return
		}

		whiteAccount, ok := accounts.GetAccount(payload.WhiteAccountID)
		if !ok {
			http.Error(w, `{"error":"unknown white account"}`, http.StatusBadRequest)
			return
		}
		blackAccount, ok := accounts.GetAccount(payload.BlackAccountID)
		if !ok {
			http.Error(w, `{"error":"unknown black account"}`, http.StatusBadRequest)
			return
		}

		if err := validateAccountResult(entry, payload.MatchID, whiteAccount, blackAccount, payload.Winner); err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
			return
		}

		whiteAccount, blackAccount, accountChanged, err := finalizeLinkedAccounts(accounts, payload.MatchID, entry.WhiteGuestID, entry.BlackGuestID, payload.Winner, entry.Queue, entry.ModeID, whiteBefore, blackBefore)
		if err != nil {
			http.Error(w, `{"error":"failed to finalize account result"}`, http.StatusBadRequest)
			return
		}
		white, black, guestChanged, err := guests.FinalizeMatch(payload.MatchID, entry.WhiteGuestID, entry.BlackGuestID, payload.Winner)
		if err != nil {
			http.Error(w, `{"error":"failed to finalize account result"}`, http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"changed":      guestChanged || accountChanged,
			"white":        white,
			"black":        black,
			"whiteAccount": platform.BuildPublicAccountProfile(whiteAccount, guests),
			"blackAccount": platform.BuildPublicAccountProfile(blackAccount, guests),
		})
	})

	mux.HandleFunc("/api/platform/rankings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"players": guests.ListGuests(platform.ParseListLimit(r.URL.Query().Get("limit"), 20)),
		})
	})

	mux.HandleFunc("/api/platform/guests", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"guests": guests.ListRecentGuests(platform.ParseListLimit(r.URL.Query().Get("limit"), 24)),
		})
	})

	mux.HandleFunc("/api/platform/guests/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		guestID := strings.TrimPrefix(r.URL.Path, "/api/platform/guests/")
		if guestID == "" {
			http.NotFound(w, r)
			return
		}
		guest, ok := guests.GetGuest(guestID)
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"guest": guest,
		})
	})

	mux.HandleFunc("/api/platform/matches", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		guestID := r.URL.Query().Get("guestId")
		accountID := r.URL.Query().Get("accountId")
		seasonID := strings.TrimSpace(r.URL.Query().Get("seasonId"))
		modeID := parseOptionalModeID(r.URL.Query().Get("modeId"))
		statusFilter := parseOptionalMatchStatus(r.URL.Query().Get("status"))
		var matches []platform.MatchArchiveEntry
		if accountID != "" {
			account, ok := accounts.GetAccount(accountID)
			if ok {
				matches = archive.ListByAccount(account.AccountID, account.LinkedGuestIDs, platform.ParseListLimit(r.URL.Query().Get("limit"), 20))
			} else {
				matches = []platform.MatchArchiveEntry{}
			}
		} else if guestID != "" {
			matches = archive.ListByGuest(guestID, platform.ParseListLimit(r.URL.Query().Get("limit"), 20))
		} else {
			matches = archive.List(platform.ParseListLimit(r.URL.Query().Get("limit"), 20))
		}
		for i := range matches {
			matches[i] = enrichArchiveEntry(accounts, matches[i])
		}
		if seasonID != "" {
			matches = filterArchivedMatchesBySeason(matches, seasonID)
		}
		if modeID != "" {
			matches = filterArchivedMatchesByMode(matches, modeID)
		}
		if statusFilter != "" {
			matches = filterArchivedMatchesByStatus(matches, statusFilter)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"matches":          matches,
			"selectedSeasonId": seasonID,
			"selectedModeId":   modeID,
			"selectedStatus":   statusFilter,
		})
	})

	mux.HandleFunc("/api/platform/matches/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		matchID := strings.TrimPrefix(r.URL.Path, "/api/platform/matches/")
		if matchID == "" {
			http.NotFound(w, r)
			return
		}
		entry, ok := archive.Get(matchID)
		if !ok {
			http.NotFound(w, r)
			return
		}
		entry = enrichArchiveEntry(accounts, entry)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(entry)
	})
	return mux
}

func enrichArchiveEntry(accounts platform.AccountDirectory, entry platform.MatchArchiveEntry) platform.MatchArchiveEntry {
	if accounts == nil {
		return entry
	}
	if entry.WhiteAccountID == "" {
		if account, ok := accounts.GetAccountByGuest(entry.WhiteGuestID); ok {
			entry.WhiteAccountID = account.AccountID
			if entry.WhiteAccountHandle == "" {
				entry.WhiteAccountHandle = account.Handle
			}
		}
	} else if entry.WhiteAccountHandle == "" {
		if account, ok := accounts.GetAccount(entry.WhiteAccountID); ok {
			entry.WhiteAccountHandle = account.Handle
		}
	}
	if entry.BlackAccountID == "" {
		if account, ok := accounts.GetAccountByGuest(entry.BlackGuestID); ok {
			entry.BlackAccountID = account.AccountID
			if entry.BlackAccountHandle == "" {
				entry.BlackAccountHandle = account.Handle
			}
		}
	} else if entry.BlackAccountHandle == "" {
		if account, ok := accounts.GetAccount(entry.BlackAccountID); ok {
			entry.BlackAccountHandle = account.Handle
		}
	}
	if entry.Snapshot.Match.WhiteAccountID == "" {
		entry.Snapshot.Match.WhiteAccountID = entry.WhiteAccountID
	}
	if entry.Snapshot.Match.BlackAccountID == "" {
		entry.Snapshot.Match.BlackAccountID = entry.BlackAccountID
	}
	return entry
}

func filterArchivedMatchesBySeason(matches []platform.MatchArchiveEntry, seasonID string) []platform.MatchArchiveEntry {
	if seasonID == "" {
		return matches
	}
	filtered := make([]platform.MatchArchiveEntry, 0, len(matches))
	for _, entry := range matches {
		playedAt := entry.UpdatedAt
		if playedAt.IsZero() {
			playedAt = entry.CreatedAt
		}
		if playedAt.UTC().Format("2006-01") == seasonID {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func filterArchivedMatchesByMode(matches []platform.MatchArchiveEntry, modeID contracts.MatchModeID) []platform.MatchArchiveEntry {
	filtered := make([]platform.MatchArchiveEntry, 0, len(matches))
	for _, entry := range matches {
		if contracts.NormalizeMatchModeID(string(entry.ModeID)) != modeID {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func filterArchivedMatchesByStatus(matches []platform.MatchArchiveEntry, status string) []platform.MatchArchiveEntry {
	filtered := make([]platform.MatchArchiveEntry, 0, len(matches))
	for _, entry := range matches {
		if strings.TrimSpace(entry.Status) != status {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func validateGuestResult(entry platform.MatchArchiveEntry, matchID, whiteGuestID, blackGuestID, winner string) error {
	if err := validateArchivedRatedResult(entry, matchID, winner); err != nil {
		return err
	}
	if entry.WhiteGuestID != "" && entry.WhiteGuestID != whiteGuestID {
		return errGuestResult("white guest does not match archived seat")
	}
	if entry.BlackGuestID != "" && entry.BlackGuestID != blackGuestID {
		return errGuestResult("black guest does not match archived seat")
	}
	return nil
}

func validateAccountResult(entry platform.MatchArchiveEntry, matchID string, whiteAccount, blackAccount platform.AccountProfile, winner string) error {
	if err := validateArchivedRatedResult(entry, matchID, winner); err != nil {
		return err
	}
	if entry.WhiteGuestID == "" || entry.BlackGuestID == "" {
		return errGuestResult("archived match seats are incomplete")
	}
	if entry.WhiteAccountID != "" && entry.WhiteAccountID != whiteAccount.AccountID {
		return errGuestResult("white account does not match archived seat")
	}
	if entry.BlackAccountID != "" && entry.BlackAccountID != blackAccount.AccountID {
		return errGuestResult("black account does not match archived seat")
	}
	if !accountOwnsGuest(whiteAccount, entry.WhiteGuestID) {
		return errGuestResult("white account does not own archived white seat")
	}
	if !accountOwnsGuest(blackAccount, entry.BlackGuestID) {
		return errGuestResult("black account does not own archived black seat")
	}
	return nil
}

func validateArchivedRatedResult(entry platform.MatchArchiveEntry, matchID, winner string) error {
	if entry.Queue != "rated" {
		return errGuestResult("only rated matches can finalize guest results")
	}
	if entry.Status != "finished" {
		return errGuestResult("only finished matches can finalize guest results")
	}
	if entry.MatchID != matchID {
		return errGuestResult("match archive mismatch")
	}
	if entry.Winner != winner {
		return errGuestResult("winner does not match archived result")
	}
	return nil
}

func accountOwnsGuest(account platform.AccountProfile, guestID string) bool {
	if account.PrimaryGuestID == guestID {
		return true
	}
	for _, linkedGuestID := range account.LinkedGuestIDs {
		if linkedGuestID == guestID {
			return true
		}
	}
	return false
}

func finalizeLinkedAccounts(
	accounts platform.AccountDirectory,
	matchID, whiteGuestID, blackGuestID, winner, queue string,
	modeID contracts.MatchModeID,
	whiteBefore, blackBefore platform.GuestProfile,
) (platform.AccountProfile, platform.AccountProfile, bool, error) {
	if _, _, err := accounts.SyncGuestStats(whiteBefore); err != nil {
		return platform.AccountProfile{}, platform.AccountProfile{}, false, err
	}
	if _, _, err := accounts.SyncGuestStats(blackBefore); err != nil {
		return platform.AccountProfile{}, platform.AccountProfile{}, false, err
	}
	whiteAccount, ok := accounts.GetAccountByGuest(whiteGuestID)
	if !ok {
		return platform.AccountProfile{}, platform.AccountProfile{}, false, os.ErrNotExist
	}
	blackAccount, ok := accounts.GetAccountByGuest(blackGuestID)
	if !ok {
		return platform.AccountProfile{}, platform.AccountProfile{}, false, os.ErrNotExist
	}
	finalWhite, finalBlack, changed, err := accounts.FinalizeMatch(matchID, whiteAccount.AccountID, blackAccount.AccountID, winner, queue, modeID)
	if err != nil {
		return platform.AccountProfile{}, platform.AccountProfile{}, false, err
	}
	return finalWhite, finalBlack, changed, nil
}

type errGuestResult string

func (e errGuestResult) Error() string {
	return string(e)
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

func parseOptionalModeID(raw string) contracts.MatchModeID {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	return contracts.NormalizeMatchModeID(raw)
}

func parseOptionalMatchStatus(raw string) string {
	switch strings.TrimSpace(raw) {
	case "active", "finished":
		return strings.TrimSpace(raw)
	default:
		return ""
	}
}

func normalizeAccountQuery(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func filterAccountsByQuery(accounts []platform.AccountProfile, query string) []platform.AccountProfile {
	if query == "" {
		return accounts
	}
	filtered := make([]platform.AccountProfile, 0, len(accounts))
	for _, account := range accounts {
		if strings.Contains(strings.ToLower(strings.TrimSpace(account.Handle)), query) {
			filtered = append(filtered, account)
		}
	}
	return filtered
}

func findAccountByHandle(accounts platform.AccountDirectory, handle string) (platform.AccountProfile, bool) {
	query := normalizeAccountQuery(handle)
	if query == "" {
		return platform.AccountProfile{}, false
	}
	for _, account := range accounts.ListAccounts(0) {
		if strings.ToLower(strings.TrimSpace(account.Handle)) == query {
			return account, true
		}
	}
	return platform.AccountProfile{}, false
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

func guestStorePath() string {
	if value := os.Getenv("GUEST_STORE_PATH"); value != "" {
		return value
	}
	return filepath.Join("data", "guest-profiles.json")
}

func guestStoreSQLitePath() string {
	if value := os.Getenv("GUEST_STORE_SQLITE_PATH"); value != "" {
		return value
	}
	return filepath.Join("data", "guest-profiles.sqlite")
}

func guestStorePostgresURL() string {
	return envOrDefault("GUEST_STORE_POSTGRES_URL", "postgres://postgres:postgres@127.0.0.1:5432/chess404?sslmode=disable")
}

func accountStorePath() string {
	if value := os.Getenv("ACCOUNT_STORE_PATH"); value != "" {
		return value
	}
	return filepath.Join("data", "accounts.json")
}

func accountStoreSQLitePath() string {
	if value := os.Getenv("ACCOUNT_STORE_SQLITE_PATH"); value != "" {
		return value
	}
	return filepath.Join("data", "accounts.sqlite")
}

func accountStorePostgresURL() string {
	return envOrDefault("ACCOUNT_STORE_POSTGRES_URL", "postgres://postgres:postgres@127.0.0.1:5432/chess404?sslmode=disable")
}

func friendshipStorePath() string {
	if value := os.Getenv("FRIEND_STORE_PATH"); value != "" {
		return value
	}
	return filepath.Join("data", "friendships.json")
}

func friendshipStoreSQLitePath() string {
	if value := os.Getenv("FRIEND_STORE_SQLITE_PATH"); value != "" {
		return value
	}
	return filepath.Join("data", "friendships.sqlite")
}

func friendshipStorePostgresURL() string {
	return envOrDefault("FRIEND_STORE_POSTGRES_URL", "postgres://postgres:postgres@127.0.0.1:5432/chess404?sslmode=disable")
}

func directChallengeStorePath() string {
	if value := os.Getenv("DIRECT_CHALLENGE_STORE_PATH"); value != "" {
		return value
	}
	return filepath.Join("data", "direct_challenges.json")
}

func directChallengeStoreSQLitePath() string {
	if value := os.Getenv("DIRECT_CHALLENGE_STORE_SQLITE_PATH"); value != "" {
		return value
	}
	return filepath.Join("data", "direct_challenges.sqlite")
}

func moderationStorePath() string {
	if value := os.Getenv("MODERATION_STORE_PATH"); value != "" {
		return value
	}
	return filepath.Join("data", "moderation.json")
}

func moderationStoreSQLitePath() string {
	if value := os.Getenv("MODERATION_STORE_SQLITE_PATH"); value != "" {
		return value
	}
	return filepath.Join("data", "moderation.sqlite")
}

func moderationStorePostgresURL() string {
	return envOrDefault("MODERATION_STORE_POSTGRES_URL", "postgres://postgres:postgres@127.0.0.1:5432/chess404?sslmode=disable")
}

func directChallengeStorePostgresURL() string {
	return envOrDefault("DIRECT_CHALLENGE_STORE_POSTGRES_URL", "postgres://postgres:postgres@127.0.0.1:5432/chess404?sslmode=disable")
}

func notificationStorePath() string {
	if value := os.Getenv("NOTIFICATION_STORE_PATH"); value != "" {
		return value
	}
	return filepath.Join("data", "notifications.json")
}

func notificationStoreSQLitePath() string {
	if value := os.Getenv("NOTIFICATION_STORE_SQLITE_PATH"); value != "" {
		return value
	}
	return filepath.Join("data", "notifications.sqlite")
}

func notificationStorePostgresURL() string {
	return envOrDefault("NOTIFICATION_STORE_POSTGRES_URL", "postgres://postgres:postgres@127.0.0.1:5432/chess404?sslmode=disable")
}

func accountEmailOutboxStorePath() string {
	if value := os.Getenv("ACCOUNT_EMAIL_OUTBOX_PATH"); value != "" {
		return value
	}
	return filepath.Join("data", "account-email-outbox.json")
}

func accountEmailOutboxSQLitePath() string {
	if value := os.Getenv("ACCOUNT_EMAIL_OUTBOX_SQLITE_PATH"); value != "" {
		return value
	}
	return filepath.Join("data", "account-email-outbox.sqlite")
}

func accountEmailOutboxPostgresURL() string {
	return envOrDefault("ACCOUNT_EMAIL_OUTBOX_POSTGRES_URL", "postgres://postgres:postgres@127.0.0.1:5432/chess404?sslmode=disable")
}

func accountSecurityAuditStorePath() string {
	if value := os.Getenv("ACCOUNT_SECURITY_AUDIT_PATH"); value != "" {
		return value
	}
	return filepath.Join("data", "account-security-audit.json")
}

func accountSecurityAuditSQLitePath() string {
	if value := os.Getenv("ACCOUNT_SECURITY_AUDIT_SQLITE_PATH"); value != "" {
		return value
	}
	return filepath.Join("data", "account-security-audit.sqlite")
}

func accountSecurityAuditPostgresURL() string {
	return envOrDefault("ACCOUNT_SECURITY_AUDIT_POSTGRES_URL", "postgres://postgres:postgres@127.0.0.1:5432/chess404?sslmode=disable")
}

func matchClaimStoreRedisURL() string {
	return envOrDefault("MATCH_CLAIM_STORE_REDIS_URL", "redis://127.0.0.1:6379/0")
}

func matchClaimStoreRedisKey() string {
	return envOrDefault("MATCH_CLAIM_STORE_REDIS_KEY", "chess404:platform:match-claims")
}

func matchClaimStoreTTL() time.Duration {
	seconds := platform.ParseListLimit(os.Getenv("MATCH_CLAIM_STORE_TTL_SECONDS"), int((12*time.Hour)/time.Second))
	if seconds <= 0 {
		seconds = int((12 * time.Hour) / time.Second)
	}
	return time.Duration(seconds) * time.Second
}

func moderationAdminConfigured() bool {
	return len(configuredModerationAdminHandles()) > 0 || len(configuredModerationAdminAccountIDs()) > 0
}

func configuredModerationAdminHandles() map[string]struct{} {
	return parseModerationAdminSet(os.Getenv("PLATFORM_ADMIN_HANDLES"), true)
}

func configuredModerationAdminAccountIDs() map[string]struct{} {
	return parseModerationAdminSet(os.Getenv("PLATFORM_ADMIN_ACCOUNT_IDS"), false)
}

func parseModerationAdminSet(value string, lowercase bool) map[string]struct{} {
	items := make(map[string]struct{})
	for _, part := range strings.FieldsFunc(value, func(r rune) bool {
		switch r {
		case ',', ';', '\n', '\r', '\t', ' ':
			return true
		default:
			return false
		}
	}) {
		resolved := strings.TrimSpace(part)
		if lowercase {
			resolved = strings.ToLower(resolved)
		}
		if resolved == "" {
			continue
		}
		items[resolved] = struct{}{}
	}
	return items
}

func openGuestDirectory() (platform.GuestDirectory, error) {
	switch strings.ToLower(envOrDefault("GUEST_STORE_BACKEND", "file")) {
	case "sqlite":
		return platform.NewSQLiteGuestStore(guestStoreSQLitePath())
	case "postgres":
		return platform.NewPostgresGuestStore(guestStorePostgresURL())
	default:
		return platform.NewGuestStore(guestStorePath())
	}
}

func openAccountStore() (platform.AccountDirectory, error) {
	switch strings.ToLower(envOrDefault("ACCOUNT_STORE_BACKEND", "file")) {
	case "sqlite":
		return platform.NewSQLiteAccountStore(accountStoreSQLitePath())
	case "postgres":
		return platform.NewPostgresAccountStore(accountStorePostgresURL())
	default:
		return platform.NewAccountStore(accountStorePath())
	}
}

func openFriendshipStore() (platform.FriendshipDirectory, error) {
	switch strings.ToLower(envOrDefault("FRIEND_STORE_BACKEND", "file")) {
	case "sqlite":
		return platform.NewSQLiteFriendshipStore(friendshipStoreSQLitePath())
	case "postgres":
		return platform.NewPostgresFriendshipStore(friendshipStorePostgresURL())
	default:
		return platform.NewFriendshipStore(friendshipStorePath())
	}
}

func openModerationStore() (platform.ModerationDirectory, error) {
	switch strings.ToLower(envOrDefault("MODERATION_STORE_BACKEND", "file")) {
	case "sqlite":
		return platform.NewSQLiteModerationStore(moderationStoreSQLitePath())
	case "postgres":
		return platform.NewPostgresModerationStore(moderationStorePostgresURL())
	default:
		return platform.NewModerationStore(moderationStorePath())
	}
}

func openDirectChallengeStore() (platform.DirectChallengeDirectory, error) {
	switch strings.ToLower(envOrDefault("DIRECT_CHALLENGE_STORE_BACKEND", "file")) {
	case "sqlite":
		return platform.NewSQLiteDirectChallengeStore(directChallengeStoreSQLitePath())
	case "postgres":
		return platform.NewPostgresDirectChallengeStore(directChallengeStorePostgresURL())
	default:
		return platform.NewDirectChallengeStore(directChallengeStorePath())
	}
}

func openNotificationStore() (platform.AccountNotificationDirectory, error) {
	switch strings.ToLower(envOrDefault("NOTIFICATION_STORE_BACKEND", "file")) {
	case "sqlite":
		return platform.NewSQLiteAccountNotificationStore(notificationStoreSQLitePath())
	case "postgres":
		return platform.NewPostgresAccountNotificationStore(notificationStorePostgresURL())
	default:
		return platform.NewAccountNotificationStore(notificationStorePath())
	}
}

func openAccountEmailOutboxStore() (platform.AccountEmailOutboxDirectory, error) {
	switch strings.ToLower(envOrDefault("ACCOUNT_EMAIL_OUTBOX_BACKEND", "file")) {
	case "sqlite":
		return platform.NewSQLiteAccountEmailOutboxStore(accountEmailOutboxSQLitePath())
	case "postgres":
		return platform.NewPostgresAccountEmailOutboxStore(accountEmailOutboxPostgresURL())
	default:
		return platform.NewAccountEmailOutboxStore(accountEmailOutboxStorePath())
	}
}

func openAccountSecurityAuditStore() (platform.AccountSecurityAuditDirectory, error) {
	switch strings.ToLower(envOrDefault("ACCOUNT_SECURITY_AUDIT_BACKEND", "file")) {
	case "sqlite":
		return platform.NewSQLiteAccountSecurityAuditStore(accountSecurityAuditSQLitePath())
	case "postgres":
		return platform.NewPostgresAccountSecurityAuditStore(accountSecurityAuditPostgresURL())
	default:
		return platform.NewAccountSecurityAuditStore(accountSecurityAuditStorePath())
	}
}

func openMatchClaimStore() (*platform.MatchClaimStore, error) {
	switch strings.ToLower(envOrDefault("MATCH_CLAIM_STORE_BACKEND", "memory")) {
	case "redis":
		return platform.NewRedisMatchClaimStoreWithTTL(matchClaimStoreRedisURL(), matchClaimStoreRedisKey(), matchClaimStoreTTL())
	default:
		return platform.NewMatchClaimStoreWithTTL(matchClaimStoreTTL()), nil
	}
}

func accountAuthPreviewEnabled() bool {
	value := strings.TrimSpace(strings.ToLower(envOrDefault("ACCOUNT_AUTH_EXPOSE_PREVIEW_TOKENS", "true")))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func accountAuthPublicBaseURL() string {
	return envOrDefault("ACCOUNT_AUTH_PUBLIC_BASE_URL", "http://127.0.0.1:3000")
}

func recordAccountSecurityEvent(store platform.AccountSecurityAuditDirectory, accountID, kind, detail string) {
	if store == nil || strings.TrimSpace(accountID) == "" {
		return
	}
	if _, err := store.RecordEvent(platform.AccountSecurityEventRequest{
		AccountID: accountID,
		Kind:      kind,
		Detail:    detail,
	}); err != nil {
		log.Printf("failed to record account security event %s for %s: %v", kind, accountID, err)
	}
}

func sessionTokenFingerprint(token string) string {
	resolved := strings.TrimSpace(token)
	if resolved == "" {
		return ""
	}
	if len(resolved) <= 12 {
		return resolved
	}
	return fmt.Sprintf("%s...%s", resolved[:8], resolved[len(resolved)-4:])
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

func resolvePrimaryGuestID(account platform.AccountProfile) string {
	if resolved := strings.TrimSpace(account.PrimaryGuestID); resolved != "" {
		return resolved
	}
	for _, guestID := range account.LinkedGuestIDs {
		if resolved := strings.TrimSpace(guestID); resolved != "" {
			return resolved
		}
	}
	return ""
}

func resumeGuestFromPayload(guests platform.GuestDirectory, guestID, sessionSecret, sessionToken string) (platform.GuestSession, error) {
	resolvedGuestID := strings.TrimSpace(guestID)
	if resolvedGuestID == "" {
		return platform.GuestSession{}, os.ErrInvalid
	}
	resolvedSecret := strings.TrimSpace(sessionSecret)
	resolvedToken := strings.TrimSpace(sessionToken)
	if resolvedToken != "" {
		session, err := guests.ResumeGuestByToken(resolvedGuestID, resolvedToken)
		if err == nil {
			return session, nil
		}
		if err != platform.ErrUnauthorizedGuestSession || resolvedSecret == "" {
			return platform.GuestSession{}, err
		}
	}
	return guests.ResumeGuest(resolvedGuestID, resolvedSecret)
}

type friendOverviewResponse struct {
	Viewer   platform.PublicAccountProfile `json:"viewer"`
	Friends  []friendshipView              `json:"friends"`
	Incoming []friendRequestView           `json:"incoming"`
	Outgoing []friendRequestView           `json:"outgoing"`
}

type friendshipView struct {
	FriendshipID string                        `json:"friendshipId"`
	Account      platform.PublicAccountProfile `json:"account"`
	CreatedAt    time.Time                     `json:"createdAt"`
}

type friendRequestView struct {
	RequestID string                        `json:"requestId"`
	Status    string                        `json:"status"`
	Account   platform.PublicAccountProfile `json:"account"`
	CreatedAt time.Time                     `json:"createdAt"`
	UpdatedAt time.Time                     `json:"updatedAt"`
}

type challengeOverviewResponse struct {
	Viewer   platform.PublicAccountProfile `json:"viewer"`
	Incoming []directChallengeView         `json:"incoming"`
	Outgoing []directChallengeView         `json:"outgoing"`
}

type directChallengeView struct {
	ChallengeID    string                        `json:"challengeId"`
	Status         string                        `json:"status"`
	Account        platform.PublicAccountProfile `json:"account"`
	MatchID        string                        `json:"matchId"`
	ModeID         contracts.MatchModeID         `json:"modeId,omitempty"`
	ClockSeconds   int64                         `json:"clockSeconds,omitempty"`
	ChallengerSeat string                        `json:"challengerSeat,omitempty"`
	ViewerSeat     string                        `json:"viewerSeat,omitempty"`
	CreatedAt      time.Time                     `json:"createdAt"`
	UpdatedAt      time.Time                     `json:"updatedAt"`
}

type notificationOverviewResponse struct {
	Viewer        platform.PublicAccountProfile `json:"viewer"`
	Notifications []accountNotificationView     `json:"notifications"`
	UnreadCount   int                           `json:"unreadCount"`
}

type accountNotificationView struct {
	NotificationID  string                        `json:"notificationId"`
	Kind            string                        `json:"kind"`
	Actor           platform.PublicAccountProfile `json:"actor"`
	FriendRequestID string                        `json:"friendRequestId,omitempty"`
	ChallengeID     string                        `json:"challengeId,omitempty"`
	MatchID         string                        `json:"matchId,omitempty"`
	ModeID          contracts.MatchModeID         `json:"modeId,omitempty"`
	ChallengerSeat  string                        `json:"challengerSeat,omitempty"`
	CreatedAt       time.Time                     `json:"createdAt"`
	UpdatedAt       time.Time                     `json:"updatedAt"`
	ReadAt          *time.Time                    `json:"readAt,omitempty"`
}

type moderationOverviewResponse struct {
	Viewer           platform.PublicAccountProfile `json:"viewer"`
	OutgoingBlocks   []accountBlockView            `json:"outgoingBlocks"`
	IncomingBlocks   []accountBlockView            `json:"incomingBlocks"`
	SubmittedReports []playerReportView            `json:"submittedReports"`
}

type moderationAdminOverviewResponse struct {
	Viewer             platform.PublicAccountProfile `json:"viewer"`
	SelectedStatus     string                        `json:"selectedStatus,omitempty"`
	Reports            []moderationAdminReportView   `json:"reports"`
	RecentActions      []moderationActionAuditView   `json:"recentActions"`
	ActiveRestrictions []accountRestrictionView      `json:"activeRestrictions"`
}

type accountBlockView struct {
	BlockID   string                        `json:"blockId"`
	Direction string                        `json:"direction"`
	Reason    string                        `json:"reason,omitempty"`
	Account   platform.PublicAccountProfile `json:"account"`
	CreatedAt time.Time                     `json:"createdAt"`
	UpdatedAt time.Time                     `json:"updatedAt"`
}

type playerReportView struct {
	ReportID       string                         `json:"reportId"`
	Category       string                         `json:"category"`
	Details        string                         `json:"details,omitempty"`
	Status         string                         `json:"status"`
	Target         platform.PublicAccountProfile  `json:"target"`
	ReviewedBy     *platform.PublicAccountProfile `json:"reviewedBy,omitempty"`
	ReviewedAt     *time.Time                     `json:"reviewedAt,omitempty"`
	ResolutionNote string                         `json:"resolutionNote,omitempty"`
	CreatedAt      time.Time                      `json:"createdAt"`
	UpdatedAt      time.Time                      `json:"updatedAt"`
}

type moderationAdminReportView struct {
	ReportID          string                         `json:"reportId"`
	Category          string                         `json:"category"`
	Details           string                         `json:"details,omitempty"`
	Status            string                         `json:"status"`
	Reporter          platform.PublicAccountProfile  `json:"reporter"`
	Target            platform.PublicAccountProfile  `json:"target"`
	TargetRestriction *accountRestrictionView        `json:"targetRestriction,omitempty"`
	ReviewedBy        *platform.PublicAccountProfile `json:"reviewedBy,omitempty"`
	ReviewedAt        *time.Time                     `json:"reviewedAt,omitempty"`
	ResolutionNote    string                         `json:"resolutionNote,omitempty"`
	CreatedAt         time.Time                      `json:"createdAt"`
	UpdatedAt         time.Time                      `json:"updatedAt"`
}

type moderationActionAuditView struct {
	ActionID       string                        `json:"actionId"`
	ReportID       string                        `json:"reportId"`
	PreviousStatus string                        `json:"previousStatus"`
	NextStatus     string                        `json:"nextStatus"`
	Action         string                        `json:"action"`
	Note           string                        `json:"note,omitempty"`
	Moderator      platform.PublicAccountProfile `json:"moderator"`
	Reporter       platform.PublicAccountProfile `json:"reporter"`
	Target         platform.PublicAccountProfile `json:"target"`
	CreatedAt      time.Time                     `json:"createdAt"`
}

type accountRestrictionView struct {
	RestrictionID string                         `json:"restrictionId"`
	Account       platform.PublicAccountProfile  `json:"account"`
	Kind          string                         `json:"kind"`
	Reason        string                         `json:"reason,omitempty"`
	ReportID      string                         `json:"reportId,omitempty"`
	AppliedBy     *platform.PublicAccountProfile `json:"appliedBy,omitempty"`
	CreatedAt     time.Time                      `json:"createdAt"`
	UpdatedAt     time.Time                      `json:"updatedAt"`
}

func writeAccountSessionError(w http.ResponseWriter, err error) {
	switch err {
	case platform.ErrUnauthorizedAccountSession:
		http.Error(w, `{"error":"unauthorized account session"}`, http.StatusUnauthorized)
	case platform.ErrAccountRestricted:
		http.Error(w, `{"error":"account access restricted"}`, http.StatusForbidden)
	case os.ErrNotExist:
		http.Error(w, `{"error":"unknown account"}`, http.StatusNotFound)
	case os.ErrInvalid:
		http.Error(w, `{"error":"accountId is required"}`, http.StatusBadRequest)
	default:
		http.Error(w, `{"error":"failed to resume account session"}`, http.StatusBadRequest)
	}
}

func writeAccountAuthError(w http.ResponseWriter, err error) {
	switch err {
	case platform.ErrUnauthorizedAccountSession:
		http.Error(w, `{"error":"unauthorized account session"}`, http.StatusUnauthorized)
	case platform.ErrAccountRestricted:
		http.Error(w, `{"error":"account access restricted"}`, http.StatusForbidden)
	case platform.ErrInvalidAccountEmail:
		http.Error(w, `{"error":"invalid account email"}`, http.StatusBadRequest)
	case platform.ErrAccountEmailTaken:
		http.Error(w, `{"error":"account email already taken"}`, http.StatusConflict)
	case platform.ErrInvalidAccountPassword:
		http.Error(w, `{"error":"invalid account password"}`, http.StatusBadRequest)
	case platform.ErrAccountLoginUnavailable:
		http.Error(w, `{"error":"account login is not enabled"}`, http.StatusBadRequest)
	case platform.ErrAccountEmailAlreadyVerified:
		http.Error(w, `{"error":"account email already verified"}`, http.StatusConflict)
	case platform.ErrUnauthorizedAccountEmailVerification:
		http.Error(w, `{"error":"unauthorized account email verification"}`, http.StatusUnauthorized)
	case platform.ErrAccountEmailNotVerified:
		http.Error(w, `{"error":"account email is not verified"}`, http.StatusForbidden)
	case platform.ErrUnauthorizedAccountPasswordReset:
		http.Error(w, `{"error":"unauthorized account password reset"}`, http.StatusUnauthorized)
	case platform.ErrUnauthorizedAccountCredentials:
		http.Error(w, `{"error":"unauthorized account credentials"}`, http.StatusUnauthorized)
	case os.ErrNotExist:
		http.Error(w, `{"error":"unknown account"}`, http.StatusNotFound)
	case os.ErrInvalid:
		http.Error(w, `{"error":"accountId is required"}`, http.StatusBadRequest)
	default:
		http.Error(w, `{"error":"failed to update account authentication"}`, http.StatusInternalServerError)
	}
}

func writeModerationError(w http.ResponseWriter, err error) {
	switch err {
	case platform.ErrAccountInteractionBlocked:
		http.Error(w, `{"error":"account interaction blocked"}`, http.StatusForbidden)
	case platform.ErrAccountRestricted:
		http.Error(w, `{"error":"account access restricted"}`, http.StatusForbidden)
	case platform.ErrAccountBlockNotFound:
		http.Error(w, `{"error":"account block not found"}`, http.StatusNotFound)
	case platform.ErrInvalidAccountBlock:
		http.Error(w, `{"error":"invalid account block"}`, http.StatusBadRequest)
	case platform.ErrInvalidAccountRestriction:
		http.Error(w, `{"error":"invalid account restriction"}`, http.StatusBadRequest)
	case platform.ErrAccountRestrictionNotFound:
		http.Error(w, `{"error":"account restriction not found"}`, http.StatusNotFound)
	case platform.ErrInvalidPlayerReport:
		http.Error(w, `{"error":"invalid player report"}`, http.StatusBadRequest)
	case platform.ErrPlayerReportNotFound:
		http.Error(w, `{"error":"player report not found"}`, http.StatusNotFound)
	case platform.ErrInvalidModerationReview:
		http.Error(w, `{"error":"invalid moderation review"}`, http.StatusBadRequest)
	default:
		http.Error(w, `{"error":"failed to update moderation state"}`, http.StatusInternalServerError)
	}
}

func writeAccountRestrictionError(w http.ResponseWriter, restriction platform.AccountRestriction) {
	message := "account access restricted"
	switch restriction.Kind {
	case platform.AccountRestrictionKindSuspended:
		message = "account suspended"
	case platform.AccountRestrictionKindBanned:
		message = "account banned"
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":             message,
		"restrictionKind":   restriction.Kind,
		"restrictionReason": strings.TrimSpace(restriction.Reason),
	})
}

func ensureAllowedAccountSession(accounts platform.AccountDirectory, moderation platform.ModerationDirectory, accountID, sessionToken string) (platform.AccountSession, *platform.AccountRestriction, error) {
	session, err := accounts.ResumeAccount(accountID, sessionToken)
	if err != nil {
		return platform.AccountSession{}, nil, err
	}
	if restriction, ok := moderation.GetAccountRestriction(session.Account.AccountID); ok {
		_ = accounts.LogoutAccount(session.Account.AccountID, session.SessionToken)
		return platform.AccountSession{}, &restriction, platform.ErrAccountRestricted
	}
	return session, nil, nil
}

func resumeAllowedAccountSessionOrWrite(w http.ResponseWriter, accounts platform.AccountDirectory, moderation platform.ModerationDirectory, accountID, sessionToken string) (platform.AccountSession, bool) {
	session, restriction, err := ensureAllowedAccountSession(accounts, moderation, accountID, sessionToken)
	if err != nil {
		if err == platform.ErrAccountRestricted && restriction != nil {
			writeAccountRestrictionError(w, *restriction)
		} else {
			writeAccountSessionError(w, err)
		}
		return platform.AccountSession{}, false
	}
	return session, true
}

func writeModerationAdminAuthError(w http.ResponseWriter) {
	http.Error(w, `{"error":"moderation admin access required"}`, http.StatusForbidden)
}

func writeNotificationError(w http.ResponseWriter, err error) {
	switch err {
	case platform.ErrAccountNotificationNotFound:
		http.Error(w, `{"error":"account notification not found"}`, http.StatusNotFound)
	case platform.ErrUnauthorizedAccountNotification:
		http.Error(w, `{"error":"unauthorized account notification"}`, http.StatusForbidden)
	case platform.ErrInvalidAccountNotification:
		http.Error(w, `{"error":"invalid account notification"}`, http.StatusBadRequest)
	default:
		http.Error(w, `{"error":"failed to update inbox"}`, http.StatusInternalServerError)
	}
}

func requireAccountInteractionAllowed(moderation platform.ModerationDirectory, accountID, otherAccountID string) error {
	if moderation.IsBlockedEitherDirection(accountID, otherAccountID) {
		return platform.ErrAccountInteractionBlocked
	}
	return nil
}

func isModerationAdminAccount(account platform.AccountProfile) bool {
	resolvedAccountID := strings.TrimSpace(account.AccountID)
	if resolvedAccountID != "" {
		if _, ok := configuredModerationAdminAccountIDs()[resolvedAccountID]; ok {
			return true
		}
	}
	resolvedHandle := strings.ToLower(strings.TrimSpace(account.Handle))
	if resolvedHandle != "" {
		if _, ok := configuredModerationAdminHandles()[resolvedHandle]; ok {
			return true
		}
	}
	return false
}

func findFriendRequestCounterpartyAccountID(overview platform.FriendshipOverview, requestID, viewerAccountID string) string {
	resolvedRequestID := strings.TrimSpace(requestID)
	if resolvedRequestID == "" {
		return ""
	}
	for _, request := range overview.Incoming {
		if request.RequestID == resolvedRequestID {
			return request.RequesterAccountID
		}
	}
	for _, request := range overview.Outgoing {
		if request.RequestID == resolvedRequestID {
			return request.TargetAccountID
		}
	}
	return ""
}

func respondFriendOverview(w http.ResponseWriter, guests platform.GuestDirectory, accounts platform.AccountDirectory, friends platform.FriendshipDirectory, accountID string) {
	viewerAccount, ok := accounts.GetAccount(accountID)
	if !ok {
		http.Error(w, `{"error":"unknown account"}`, http.StatusNotFound)
		return
	}

	overview := friends.ListOverview(accountID)
	response := friendOverviewResponse{
		Viewer:   platform.BuildPublicAccountProfile(viewerAccount, guests),
		Friends:  make([]friendshipView, 0, len(overview.Friends)),
		Incoming: make([]friendRequestView, 0, len(overview.Incoming)),
		Outgoing: make([]friendRequestView, 0, len(overview.Outgoing)),
	}

	for _, friendship := range overview.Friends {
		friendAccountID := platform.FriendAccountForViewer(friendship, accountID)
		friendAccount, ok := accounts.GetAccount(friendAccountID)
		if !ok {
			continue
		}
		response.Friends = append(response.Friends, friendshipView{
			FriendshipID: friendship.FriendshipID,
			Account:      platform.BuildPublicAccountProfile(friendAccount, guests),
			CreatedAt:    friendship.CreatedAt,
		})
	}
	for _, request := range overview.Incoming {
		requester, ok := accounts.GetAccount(request.RequesterAccountID)
		if !ok {
			continue
		}
		response.Incoming = append(response.Incoming, friendRequestView{
			RequestID: request.RequestID,
			Status:    request.Status,
			Account:   platform.BuildPublicAccountProfile(requester, guests),
			CreatedAt: request.CreatedAt,
			UpdatedAt: request.UpdatedAt,
		})
	}
	for _, request := range overview.Outgoing {
		target, ok := accounts.GetAccount(request.TargetAccountID)
		if !ok {
			continue
		}
		response.Outgoing = append(response.Outgoing, friendRequestView{
			RequestID: request.RequestID,
			Status:    request.Status,
			Account:   platform.BuildPublicAccountProfile(target, guests),
			CreatedAt: request.CreatedAt,
			UpdatedAt: request.UpdatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func respondNotificationOverview(w http.ResponseWriter, guests platform.GuestDirectory, accounts platform.AccountDirectory, notifications platform.AccountNotificationDirectory, accountID string, limit int) {
	viewerAccount, ok := accounts.GetAccount(accountID)
	if !ok {
		http.Error(w, `{"error":"unknown account"}`, http.StatusNotFound)
		return
	}

	overview := notifications.ListOverview(accountID, limit)
	response := notificationOverviewResponse{
		Viewer:        platform.BuildPublicAccountProfile(viewerAccount, guests),
		Notifications: make([]accountNotificationView, 0, len(overview.Notifications)),
		UnreadCount:   overview.UnreadCount,
	}
	for _, notification := range overview.Notifications {
		view, ok := buildAccountNotificationView(guests, accounts, notification)
		if !ok {
			continue
		}
		response.Notifications = append(response.Notifications, view)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func writeAccountNotificationStreamEvent(w http.ResponseWriter, flusher http.Flusher, event platform.AccountNotificationEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: notification\ndata: %s\n\n", payload); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func buildAccountNotificationView(guests platform.GuestDirectory, accounts platform.AccountDirectory, notification platform.AccountNotification) (accountNotificationView, bool) {
	actor, ok := accounts.GetAccount(notification.ActorAccountID)
	if !ok {
		return accountNotificationView{}, false
	}
	return accountNotificationView{
		NotificationID:  notification.NotificationID,
		Kind:            notification.Kind,
		Actor:           platform.BuildPublicAccountProfile(actor, guests),
		FriendRequestID: notification.FriendRequestID,
		ChallengeID:     notification.ChallengeID,
		MatchID:         notification.MatchID,
		ModeID:          notification.ModeID,
		ChallengerSeat:  notification.ChallengerSeat,
		CreatedAt:       notification.CreatedAt,
		UpdatedAt:       notification.UpdatedAt,
		ReadAt:          notification.ReadAt,
	}, true
}

func respondModerationOverview(w http.ResponseWriter, guests platform.GuestDirectory, accounts platform.AccountDirectory, moderation platform.ModerationDirectory, accountID string) {
	viewerAccount, ok := accounts.GetAccount(accountID)
	if !ok {
		http.Error(w, `{"error":"unknown account"}`, http.StatusNotFound)
		return
	}

	overview := moderation.ListOverview(accountID)
	response := moderationOverviewResponse{
		Viewer:           platform.BuildPublicAccountProfile(viewerAccount, guests),
		OutgoingBlocks:   make([]accountBlockView, 0, len(overview.OutgoingBlocks)),
		IncomingBlocks:   make([]accountBlockView, 0, len(overview.IncomingBlocks)),
		SubmittedReports: make([]playerReportView, 0, len(overview.SubmittedReports)),
	}
	for _, block := range overview.OutgoingBlocks {
		target, ok := accounts.GetAccount(block.TargetAccountID)
		if !ok {
			continue
		}
		response.OutgoingBlocks = append(response.OutgoingBlocks, accountBlockView{
			BlockID:   block.BlockID,
			Direction: "outgoing",
			Reason:    block.Reason,
			Account:   platform.BuildPublicAccountProfile(target, guests),
			CreatedAt: block.CreatedAt,
			UpdatedAt: block.UpdatedAt,
		})
	}
	for _, block := range overview.IncomingBlocks {
		blocker, ok := accounts.GetAccount(block.BlockerAccountID)
		if !ok {
			continue
		}
		response.IncomingBlocks = append(response.IncomingBlocks, accountBlockView{
			BlockID:   block.BlockID,
			Direction: "incoming",
			Reason:    block.Reason,
			Account:   platform.BuildPublicAccountProfile(blocker, guests),
			CreatedAt: block.CreatedAt,
			UpdatedAt: block.UpdatedAt,
		})
	}
	for _, report := range overview.SubmittedReports {
		view, ok := buildPlayerReportView(guests, accounts, report)
		if ok {
			response.SubmittedReports = append(response.SubmittedReports, view)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func respondModerationAdminOverview(w http.ResponseWriter, guests platform.GuestDirectory, accounts platform.AccountDirectory, moderation platform.ModerationDirectory, accountID string, limit int, status string) {
	viewerAccount, ok := accounts.GetAccount(accountID)
	if !ok {
		http.Error(w, `{"error":"unknown account"}`, http.StatusNotFound)
		return
	}

	overview := moderation.ListAdminOverview(limit, status)
	response := moderationAdminOverviewResponse{
		Viewer:             platform.BuildPublicAccountProfile(viewerAccount, guests),
		SelectedStatus:     normalizeModerationStatusFilter(status),
		Reports:            make([]moderationAdminReportView, 0, len(overview.Reports)),
		RecentActions:      make([]moderationActionAuditView, 0, len(overview.RecentActions)),
		ActiveRestrictions: make([]accountRestrictionView, 0, len(overview.ActiveRestrictions)),
	}
	restrictionViews := make(map[string]accountRestrictionView, len(overview.ActiveRestrictions))
	for _, restriction := range overview.ActiveRestrictions {
		view, ok := buildAccountRestrictionView(guests, accounts, restriction)
		if !ok {
			continue
		}
		restrictionViews[restriction.AccountID] = view
		response.ActiveRestrictions = append(response.ActiveRestrictions, view)
	}
	for _, report := range overview.Reports {
		view, ok := buildModerationAdminReportView(guests, accounts, report)
		if ok {
			if restriction, restricted := restrictionViews[report.TargetAccountID]; restricted {
				restrictionCopy := restriction
				view.TargetRestriction = &restrictionCopy
			}
			response.Reports = append(response.Reports, view)
		}
	}
	for _, action := range overview.RecentActions {
		view, ok := buildModerationActionAuditView(guests, accounts, action)
		if ok {
			response.RecentActions = append(response.RecentActions, view)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func buildPlayerReportView(guests platform.GuestDirectory, accounts platform.AccountDirectory, report platform.PlayerReport) (playerReportView, bool) {
	target, ok := accounts.GetAccount(report.TargetAccountID)
	if !ok {
		return playerReportView{}, false
	}
	var reviewedBy *platform.PublicAccountProfile
	if resolvedReviewerID := strings.TrimSpace(report.ReviewedByAccountID); resolvedReviewerID != "" {
		if reviewer, ok := accounts.GetAccount(resolvedReviewerID); ok {
			profile := platform.BuildPublicAccountProfile(reviewer, guests)
			reviewedBy = &profile
		}
	}
	return playerReportView{
		ReportID:       report.ReportID,
		Category:       string(report.Category),
		Details:        report.Details,
		Status:         report.Status,
		Target:         platform.BuildPublicAccountProfile(target, guests),
		ReviewedBy:     reviewedBy,
		ReviewedAt:     report.ReviewedAt,
		ResolutionNote: report.ResolutionNote,
		CreatedAt:      report.CreatedAt,
		UpdatedAt:      report.UpdatedAt,
	}, true
}

func buildModerationAdminReportView(guests platform.GuestDirectory, accounts platform.AccountDirectory, report platform.PlayerReport) (moderationAdminReportView, bool) {
	reporter, ok := accounts.GetAccount(report.ReporterAccountID)
	if !ok {
		return moderationAdminReportView{}, false
	}
	target, ok := accounts.GetAccount(report.TargetAccountID)
	if !ok {
		return moderationAdminReportView{}, false
	}
	var reviewedBy *platform.PublicAccountProfile
	if resolvedReviewerID := strings.TrimSpace(report.ReviewedByAccountID); resolvedReviewerID != "" {
		if reviewer, ok := accounts.GetAccount(resolvedReviewerID); ok {
			profile := platform.BuildPublicAccountProfile(reviewer, guests)
			reviewedBy = &profile
		}
	}
	return moderationAdminReportView{
		ReportID:       report.ReportID,
		Category:       string(report.Category),
		Details:        report.Details,
		Status:         report.Status,
		Reporter:       platform.BuildPublicAccountProfile(reporter, guests),
		Target:         platform.BuildPublicAccountProfile(target, guests),
		ReviewedBy:     reviewedBy,
		ReviewedAt:     report.ReviewedAt,
		ResolutionNote: report.ResolutionNote,
		CreatedAt:      report.CreatedAt,
		UpdatedAt:      report.UpdatedAt,
	}, true
}

func buildAccountRestrictionView(guests platform.GuestDirectory, accounts platform.AccountDirectory, restriction platform.AccountRestriction) (accountRestrictionView, bool) {
	account, ok := accounts.GetAccount(restriction.AccountID)
	if !ok {
		return accountRestrictionView{}, false
	}
	var appliedBy *platform.PublicAccountProfile
	if strings.TrimSpace(restriction.AppliedByAccountID) != "" {
		if moderator, ok := accounts.GetAccount(restriction.AppliedByAccountID); ok {
			profile := platform.BuildPublicAccountProfile(moderator, guests)
			appliedBy = &profile
		}
	}
	return accountRestrictionView{
		RestrictionID: restriction.RestrictionID,
		Account:       platform.BuildPublicAccountProfile(account, guests),
		Kind:          restriction.Kind,
		Reason:        restriction.Reason,
		ReportID:      restriction.ReportID,
		AppliedBy:     appliedBy,
		CreatedAt:     restriction.CreatedAt,
		UpdatedAt:     restriction.UpdatedAt,
	}, true
}

func buildModerationActionAuditView(guests platform.GuestDirectory, accounts platform.AccountDirectory, action platform.ModerationActionAudit) (moderationActionAuditView, bool) {
	moderator, ok := accounts.GetAccount(action.ModeratorAccountID)
	if !ok {
		return moderationActionAuditView{}, false
	}
	reporter, ok := accounts.GetAccount(action.ReporterAccountID)
	if !ok {
		return moderationActionAuditView{}, false
	}
	target, ok := accounts.GetAccount(action.TargetAccountID)
	if !ok {
		return moderationActionAuditView{}, false
	}
	return moderationActionAuditView{
		ActionID:       action.ActionID,
		ReportID:       action.ReportID,
		PreviousStatus: action.PreviousStatus,
		NextStatus:     action.NextStatus,
		Action:         action.Action,
		Note:           action.Note,
		Moderator:      platform.BuildPublicAccountProfile(moderator, guests),
		Reporter:       platform.BuildPublicAccountProfile(reporter, guests),
		Target:         platform.BuildPublicAccountProfile(target, guests),
		CreatedAt:      action.CreatedAt,
	}, true
}

func normalizeModerationStatusFilter(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case platform.PlayerReportStatusOpen:
		return platform.PlayerReportStatusOpen
	case platform.PlayerReportStatusUnderReview:
		return platform.PlayerReportStatusUnderReview
	case platform.PlayerReportStatusResolvedActioned:
		return platform.PlayerReportStatusResolvedActioned
	case platform.PlayerReportStatusResolvedDismissed:
		return platform.PlayerReportStatusResolvedDismissed
	default:
		return ""
	}
}

func respondChallengeOverview(w http.ResponseWriter, guests platform.GuestDirectory, accounts platform.AccountDirectory, challenges platform.DirectChallengeDirectory, accountID string) {
	viewerAccount, ok := accounts.GetAccount(accountID)
	if !ok {
		http.Error(w, `{"error":"unknown account"}`, http.StatusNotFound)
		return
	}

	overview := challenges.ListOverview(accountID)
	response := challengeOverviewResponse{
		Viewer:   platform.BuildPublicAccountProfile(viewerAccount, guests),
		Incoming: make([]directChallengeView, 0, len(overview.Incoming)),
		Outgoing: make([]directChallengeView, 0, len(overview.Outgoing)),
	}

	for _, challenge := range overview.Incoming {
		if view, ok := buildDirectChallengeView(guests, accounts, challenge, accountID); ok {
			response.Incoming = append(response.Incoming, view)
		}
	}
	for _, challenge := range overview.Outgoing {
		if view, ok := buildDirectChallengeView(guests, accounts, challenge, accountID); ok {
			response.Outgoing = append(response.Outgoing, view)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func writeDirectChallengeView(w http.ResponseWriter, guests platform.GuestDirectory, accounts platform.AccountDirectory, challenge platform.DirectChallenge, viewerAccountID string) {
	view, ok := buildDirectChallengeView(guests, accounts, challenge, viewerAccountID)
	if !ok {
		http.Error(w, `{"error":"unknown direct challenge account"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(view)
}

func buildDirectChallengeView(guests platform.GuestDirectory, accounts platform.AccountDirectory, challenge platform.DirectChallenge, viewerAccountID string) (directChallengeView, bool) {
	opponentAccountID := platform.ChallengeOpponentAccountID(challenge, viewerAccountID)
	account, ok := accounts.GetAccount(opponentAccountID)
	if !ok {
		return directChallengeView{}, false
	}
	return directChallengeView{
		ChallengeID:    challenge.ChallengeID,
		Status:         challenge.Status,
		Account:        platform.BuildPublicAccountProfile(account, guests),
		MatchID:        challenge.MatchID,
		ModeID:         challenge.ModeID,
		ClockSeconds:   challenge.ClockSeconds,
		ChallengerSeat: challenge.ChallengerSeat,
		ViewerSeat:     platform.ChallengeViewerSeat(challenge, viewerAccountID),
		CreatedAt:      challenge.CreatedAt,
		UpdatedAt:      challenge.UpdatedAt,
	}, true
}
