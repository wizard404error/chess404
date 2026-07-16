package main

import (
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/chess404/realtime/internal/platform"
)

// AnticheatAnalysisRequest is what the analysis-worker POSTs after each
// match. We use the existing AnalysisResult from internal/anticheat and
// let the worker compute it; here we just receive and persist.
type AnticheatAnalysisRequest struct {
	MatchID        string  `json:"matchId"`
	PlayerID       string  `json:"playerId"`
	ModeID         string  `json:"modeId"`
	Accuracy       float64 `json:"accuracy"`
	AvgCPL         float64 `json:"avgCpl"`
	MaxCPL         int     `json:"maxCpl"`
	MoveCount      int     `json:"moveCount"`
	CardMoves      int     `json:"cardMoves"`
	Flags          []string `json:"flags"`
	TimeProfile    map[string]any `json:"timeProfile"`
	SuspicionScore float64 `json:"suspicionScore"`
}

type anticheatAnalysisResponse struct {
	AnalysisID      string `json:"analysisId"`
	SuspicionScore  float64 `json:"suspicionScore"`
	AutoAction      string `json:"autoAction"`
	PlayerSummaryID string `json:"playerSummaryId"`
}

type anticheatFlaggedPlayerListResponse struct {
	MinScore float64                              `json:"minScore"`
	Limit    int                                  `json:"limit"`
	Players  []platform.AnticheatPlayerSummary    `json:"players"`
}

type anticheatPlayerDetailResponse struct {
	PlayerID string                              `json:"playerId"`
	Summary  platform.AnticheatPlayerSummary     `json:"summary"`
	Analyses []platform.AnticheatAnalysisRecord  `json:"analyses"`
	Stats    platform.AnticheatStats             `json:"stats"`
}

// registerAnticheatRoutes wires the four anticheat endpoints into the
// platform-service mux. They live in a separate file to keep main.go
// from growing unbounded; the rest of the wiring (store init, mux
// registration) happens in main.go.
//
//   POST /api/platform/anticheat/analyses              (internal) receive a new analysis
//   GET  /api/platform/anticheat/players?minScore=&limit=  (admin) list flagged players
//   GET  /api/platform/anticheat/players/{playerId}    (admin) per-player detail
//   POST /api/platform/anticheat/players/{playerId}/action (admin) take action
func registerAnticheatRoutes(mux *http.ServeMux, anticheatStore platform.AnticheatStore) {
	mux.HandleFunc("/api/platform/anticheat/analyses", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		if !requireAnalysisWorkerRequest(w, r) {
			return
		}
		var req AnticheatAnalysisRequest
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, `{"error":"invalid analysis payload"}`, http.StatusBadRequest)
				return
			}
		}
		playerID := strings.TrimSpace(req.PlayerID)
		matchID := strings.TrimSpace(req.MatchID)
		if playerID == "" || matchID == "" {
			http.Error(w, `{"error":"playerId and matchId are required"}`, http.StatusBadRequest)
			return
		}

		flagsJSON, _ := json.Marshal(req.Flags)
		timeProfileJSON, _ := json.Marshal(req.TimeProfile)
		record, err := anticheatStore.RecordAnalysis(platform.AnticheatAnalysisRecord{
			MatchID:         matchID,
			PlayerID:        playerID,
			ModeID:          req.ModeID,
			Accuracy:        req.Accuracy,
			AvgCPL:          req.AvgCPL,
			MaxCPL:          req.MaxCPL,
			MoveCount:       req.MoveCount,
			CardMoves:       req.CardMoves,
			FlagsJSON:       string(flagsJSON),
			TimeProfileJSON: string(timeProfileJSON),
			SuspicionScore:  req.SuspicionScore,
		})
		if err != nil {
			log.Printf("anticheat: failed to record analysis for player=%s match=%s: %v", playerID, matchID, err)
			http.Error(w, `{"error":"failed to record analysis"}`, http.StatusInternalServerError)
			return
		}

		// Roll up into the player summary. We don't have historical game
		// results here (that lives on the match-service); the worker
		// computes the streak / rating gain locally and posts both the
		// analysis and the summary it computed. For Phase 1 we keep
		// this endpoint single-purpose: store analysis, and if the
		// worker also posts a summary, that goes through a separate
		// endpoint (upsert-summary). The current worker just POSTs
		// the analysis; the summary is built up from past analyses.
		summary, ok := anticheatStore.GetPlayerSummary(playerID)
		if !ok {
			summary = platform.AnticheatPlayerSummary{PlayerID: playerID}
		}
		summary.TotalGames++
		summary.AvgAccuracy = updateRunningAverage(summary.AvgAccuracy, summary.TotalGames-1, req.Accuracy)
		summary.AvgCPL = updateRunningAverage(summary.AvgCPL, summary.TotalGames-1, req.AvgCPL)
		summary.SuspicionScore = updateRunningAverage(summary.SuspicionScore, summary.TotalGames-1, req.SuspicionScore)
		summary.RecentAnalyses = appendRecent(summary.RecentAnalyses, record.AnalysisID, 20)
		savedSummary, err := anticheatStore.UpsertPlayerSummary(summary)
		if err != nil {
			log.Printf("anticheat: failed to update player summary for %s: %v", playerID, err)
		}

		// Auto-action: when a player crosses the suspicion threshold,
		// record a warning event and (if the rating is recoverable)
		// tag the summary so admin tooling can see what happened. We
		// do NOT auto-ban; the threshold is conservative and the
		// decision to ban stays with a human review.
		autoAction := ""
		if savedSummary.SuspicionScore >= platform.AnticheatSuspicionThreshold && savedSummary.ActionTaken == "" {
			savedSummary.ActionTaken = platform.AnticheatActionWarning
			autoAction = platform.AnticheatActionWarning
			if _, err := anticheatStore.UpsertPlayerSummary(savedSummary); err != nil {
				log.Printf("anticheat: failed to record auto-action for %s: %v", playerID, err)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(anticheatAnalysisResponse{
			AnalysisID:      record.AnalysisID,
			SuspicionScore:  record.SuspicionScore,
			AutoAction:      autoAction,
			PlayerSummaryID: savedSummary.PlayerID,
		})
	})

	mux.HandleFunc("/api/platform/anticheat/players", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		if !requireInternalServiceRequest(w, r) {
			return
		}
		minScore := 0.0
		if v := r.URL.Query().Get("minScore"); v != "" {
			if parsed, err := parseFloat(v); err == nil {
				minScore = parsed
			}
		}
		limit := 50
		if v := r.URL.Query().Get("limit"); v != "" {
			if parsed, err := parseInt(v); err == nil && parsed > 0 {
				limit = parsed
			}
		}
		players := anticheatStore.ListFlaggedPlayers(minScore, limit)
		if players == nil {
			players = []platform.AnticheatPlayerSummary{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(anticheatFlaggedPlayerListResponse{
			MinScore: minScore,
			Limit:    limit,
			Players:  players,
		})
	})

	mux.HandleFunc("/api/platform/anticheat/players/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/platform/anticheat/players/")
		if path == "" {
			http.Error(w, `{"error":"playerId is required"}`, http.StatusBadRequest)
			return
		}
		// Path can be either "/{playerId}" or "/{playerId}/action".
		parts := strings.Split(path, "/")
		playerID := parts[0]
		isAction := len(parts) == 2 && parts[1] == "action"

		if isAction {
			if r.Method != http.MethodPost {
				http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
				return
			}
			if !requireInternalServiceRequest(w, r) {
				return
			}
			var payload struct {
				Action string `json:"action"`
				Detail string `json:"detail"`
			}
			if r.Body != nil {
				defer r.Body.Close()
				_ = json.NewDecoder(r.Body).Decode(&payload)
			}
			action := strings.TrimSpace(payload.Action)
			if action == "" {
				http.Error(w, `{"error":"action is required"}`, http.StatusBadRequest)
				return
			}
			summary, ok := anticheatStore.GetPlayerSummary(playerID)
			if !ok {
				http.Error(w, `{"error":"player not found"}`, http.StatusNotFound)
				return
			}
			summary.ActionTaken = action
			_, err := anticheatStore.UpsertPlayerSummary(summary)
			if err != nil {
				http.Error(w, `{"error":"failed to record action"}`, http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"playerId": playerID,
				"action":   action,
				"detail":   payload.Detail,
			})
			return
		}

		// GET /api/platform/anticheat/players/{playerId}
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		if !requireInternalServiceRequest(w, r) {
			return
		}
		summary, ok := anticheatStore.GetPlayerSummary(playerID)
		if !ok {
			summary = platform.AnticheatPlayerSummary{PlayerID: playerID}
		}
		analyses := anticheatStore.ListPlayerAnalyses(playerID, 0)
		if analyses == nil {
			analyses = []platform.AnticheatAnalysisRecord{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(anticheatPlayerDetailResponse{
			PlayerID: playerID,
			Summary:  summary,
			Analyses: analyses,
			Stats:    anticheatStore.Stats(),
		})
	})
}

// updateRunningAverage returns the running average after `count` prior
// samples with mean `prev` and adding a new sample `next`. If count is
// 0, returns `next`. The math is straightforward:
// newMean = (prev*count + next) / (count+1).
func updateRunningAverage(prev float64, count int, next float64) float64 {
	if count <= 0 {
		return next
	}
	return (prev*float64(count) + next) / float64(count+1)
}

// appendRecent appends an id to the recent-list, trimming to `keep` items.
func appendRecent(existing []string, id string, keep int) []string {
	if existing == nil {
		existing = []string{}
	}
	out := append(existing, id)
	if keep > 0 && len(out) > keep {
		out = out[len(out)-keep:]
	}
	return out
}

// requireAnalysisWorkerRequest checks the Authorization header against
// ANALYSIS_WORKER_SECRET. If the env var is empty, auth is skipped (dev mode).
// If the env var is set and the Bearer token is missing or invalid, a 401 is
// returned.
func requireAnalysisWorkerRequest(w http.ResponseWriter, r *http.Request) bool {
	workerSecret := strings.TrimSpace(os.Getenv("ANALYSIS_WORKER_SECRET"))
	if workerSecret == "" {
		return true
	}
	const prefix = "Bearer "
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(auth, prefix) {
		provided := strings.TrimSpace(strings.TrimPrefix(auth, prefix))
		if subtle.ConstantTimeCompare([]byte(provided), []byte(workerSecret)) == 1 {
			return true
		}
	}
	respondError(w, http.StatusUnauthorized, "unauthorized")
	return false
}

func parseFloat(s string) (float64, error) {
	var f float64
	_, err := jsonNumberScan(s, &f)
	return f, err
}

func parseInt(s string) (int, error) {
	var i int
	_, err := jsonNumberScan(s, &i)
	return i, err
}

func jsonNumberScan(s string, target any) (int, error) {
	dec := json.NewDecoder(strings.NewReader(s))
	err := dec.Decode(target)
	return 1, err
}
