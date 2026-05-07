package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	claims, err := openMatchClaimStore()
	if err != nil {
		log.Fatalf("failed to initialize match claim store: %v", err)
	}
	defer func() { _ = claims.Close() }()
	mux := buildPlatformMux(archive, guests, accounts, claims)

	addr := listenAddr("PLATFORM_ADDR", 8083)
	log.Printf("platform-service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func buildPlatformMux(archive *platform.MatchArchiveStore, guests platform.GuestDirectory, accounts platform.AccountDirectory, claims *platform.MatchClaimStore) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/api/platform/capabilities", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"guestPlay":        true,
			"rankedRequiresID": true,
			"profiles":         true,
			"ratings":          true,
			"matchHistory":     true,
			"moderation":       true,
		})
	})

	mux.HandleFunc("/api/platform/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":              "ok",
			"service":             "platform-service",
			"checkedAt":           time.Now().UTC(),
			"archiveBackend":      archive.Backend(),
			"archive":             archive.Stats(),
			"claimStoreBackend":   claims.Backend(),
			"claimLeaseSeconds":   claims.TTLSeconds(),
			"claims":              claims.Stats(),
			"accounts":            accounts.Stats(),
			"accountStoreBackend": accounts.Backend(),
			"guests":              guests.Stats(),
			"guestStoreBackend":   guests.Backend(),
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

		session, err := accounts.ResumeAccount(payload.AccountID, payload.SessionToken)
		if err != nil {
			switch err {
			case platform.ErrUnauthorizedAccountSession:
				http.Error(w, `{"error":"unauthorized account session"}`, http.StatusUnauthorized)
			case os.ErrNotExist:
				http.Error(w, `{"error":"unknown account"}`, http.StatusNotFound)
			case os.ErrInvalid:
				http.Error(w, `{"error":"accountId is required"}`, http.StatusBadRequest)
			default:
				http.Error(w, `{"error":"failed to resume account session"}`, http.StatusBadRequest)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(session)
	})

	mux.HandleFunc("/api/platform/accounts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		limit := platform.ParseListLimit(r.URL.Query().Get("limit"), 24)
		sortMode := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort")))
		seasonID := strings.TrimSpace(r.URL.Query().Get("seasonId"))
		accountItems := accounts.ListAccounts(0)
		seasonOptions := platform.BuildAvailableSeasonOptions(accountItems)
		accountsList := make([]platform.PublicAccountProfile, 0, len(accountItems))
		for _, account := range accountItems {
			profile := platform.BuildPublicAccountProfileForSeason(account, guests, seasonID)
			if seasonID != "" && profile.SelectedSeason == nil {
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
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"accounts":         accountsList,
			"seasons":          seasonOptions,
			"selectedSeasonId": seasonID,
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
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"account": platform.BuildDetailedPublicAccountProfileForSeason(account, guests, seasonID),
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
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"account": platform.BuildDetailedPublicAccountProfileForSeason(account, guests, seasonID),
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
			if _, _, changed, err := finalizeLinkedAccounts(accounts, payload.MatchID, entry.WhiteGuestID, entry.BlackGuestID, payload.Winner, whiteBefore, blackBefore); err != nil {
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

		whiteAccount, blackAccount, accountChanged, err := finalizeLinkedAccounts(accounts, payload.MatchID, entry.WhiteGuestID, entry.BlackGuestID, payload.Winner, whiteBefore, blackBefore)
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
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"matches":          matches,
			"selectedSeasonId": seasonID,
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
	matchID, whiteGuestID, blackGuestID, winner string,
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
	finalWhite, finalBlack, changed, err := accounts.FinalizeMatch(matchID, whiteAccount.AccountID, blackAccount.AccountID, winner)
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

func openMatchClaimStore() (*platform.MatchClaimStore, error) {
	switch strings.ToLower(envOrDefault("MATCH_CLAIM_STORE_BACKEND", "memory")) {
	case "redis":
		return platform.NewRedisMatchClaimStoreWithTTL(matchClaimStoreRedisURL(), matchClaimStoreRedisKey(), matchClaimStoreTTL())
	default:
		return platform.NewMatchClaimStoreWithTTL(matchClaimStoreTTL()), nil
	}
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
