package platform

import (
	"testing"
	"time"
)

func newTestAnticheatStore() AnticheatStore {
	return newInMemoryAnticheatStore()
}

func TestAnticheatStoreRecordAnalysis(t *testing.T) {
	store := newTestAnticheatStore()
	record, err := store.RecordAnalysis(AnticheatAnalysisRecord{
		MatchID:        "match_1",
		PlayerID:       "player_1",
		ModeID:         "open_cards",
		Accuracy:       96.5,
		AvgCPL:         12.0,
		MaxCPL:         45,
		MoveCount:      42,
		CardMoves:      3,
		FlagsJSON:      `["high_accuracy","low_cpl"]`,
		TimeProfileJSON: `{"avg_think_time_ms":230,"flat_time_ratio":0.78}`,
		SuspicionScore: 85.0,
	})
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if record.AnalysisID == "" {
		t.Fatalf("expected non-empty analysis id")
	}
	if record.AnalyzedAt.IsZero() {
		t.Fatalf("expected analyzed_at to be populated")
	}
}

func TestAnticheatStoreUpsertPlayerSummary(t *testing.T) {
	store := newTestAnticheatStore()
	summary, err := store.UpsertPlayerSummary(AnticheatPlayerSummary{
		PlayerID:        "player_a",
		TotalGames:      20,
		AvgAccuracy:     90.0,
		AvgCPL:          20.0,
		LongestWinStreak: 8,
		CurrentWinStreak: 4,
		RatingGain20:    150,
		RatingGain50:    280,
		SuspicionScore:  72.5,
		RecentAnalyses:  []string{"a1", "a2"},
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if summary.LastAnalyzedAt.IsZero() {
		t.Fatalf("expected last_analyzed_at to be set")
	}

	// Upsert again with higher suspicion; expect fields to be overwritten.
	_, err = store.UpsertPlayerSummary(AnticheatPlayerSummary{
		PlayerID:       "player_a",
		TotalGames:     25,
		AvgAccuracy:    94.0,
		AvgCPL:         11.0,
		SuspicionScore: 92.0,
		RecentAnalyses: []string{"a3"},
	})
	if err != nil {
		t.Fatalf("upsert 2: %v", err)
	}
	got, ok := store.GetPlayerSummary("player_a")
	if !ok {
		t.Fatalf("expected to find player_a")
	}
	if got.TotalGames != 25 {
		t.Fatalf("expected total_games=25, got %d", got.TotalGames)
	}
	if got.SuspicionScore != 92.0 {
		t.Fatalf("expected suspicion=92, got %.1f", got.SuspicionScore)
	}
	if len(got.RecentAnalyses) != 1 {
		t.Fatalf("expected 1 recent analysis, got %d", len(got.RecentAnalyses))
	}
}

func TestAnticheatStoreListFlaggedPlayers(t *testing.T) {
	store := newTestAnticheatStore()
	_, _ = store.UpsertPlayerSummary(AnticheatPlayerSummary{PlayerID: "low", SuspicionScore: 30.0})
	_, _ = store.UpsertPlayerSummary(AnticheatPlayerSummary{PlayerID: "high", SuspicionScore: 90.0})
	_, _ = store.UpsertPlayerSummary(AnticheatPlayerSummary{PlayerID: "high2", SuspicionScore: 85.0})

	flagged := store.ListFlaggedPlayers(AnticheatSuspicionThreshold, 0)
	if len(flagged) != 2 {
		t.Fatalf("expected 2 flagged, got %d", len(flagged))
	}
	if flagged[0].PlayerID != "high" || flagged[1].PlayerID != "high2" {
		t.Fatalf("expected sort by score desc, got %s, %s", flagged[0].PlayerID, flagged[1].PlayerID)
	}

	// Limit should cap the result set.
	flagged = store.ListFlaggedPlayers(AnticheatSuspicionThreshold, 1)
	if len(flagged) != 1 {
		t.Fatalf("expected limit=1, got %d", len(flagged))
	}
}

func TestAnticheatStoreListPlayerAnalyses(t *testing.T) {
	store := newTestAnticheatStore()
	now := time.Now().UTC()
	_, _ = store.RecordAnalysis(AnticheatAnalysisRecord{MatchID: "m1", PlayerID: "p1", AnalyzedAt: now.Add(-2 * time.Hour), SuspicionScore: 70})
	_, _ = store.RecordAnalysis(AnticheatAnalysisRecord{MatchID: "m2", PlayerID: "p1", AnalyzedAt: now.Add(-1 * time.Hour), SuspicionScore: 80})
	_, _ = store.RecordAnalysis(AnticheatAnalysisRecord{MatchID: "m3", PlayerID: "p2", AnalyzedAt: now, SuspicionScore: 90})

	out := store.ListPlayerAnalyses("p1", 0)
	if len(out) != 2 {
		t.Fatalf("expected 2 analyses for p1, got %d", len(out))
	}
	if out[0].MatchID != "m2" {
		t.Fatalf("expected newest first, got %s", out[0].MatchID)
	}
}

func TestAnticheatStoreStats(t *testing.T) {
	store := newTestAnticheatStore()
	_, _ = store.RecordAnalysis(AnticheatAnalysisRecord{MatchID: "m1", PlayerID: "p1"})
	_, _ = store.RecordAnalysis(AnticheatAnalysisRecord{MatchID: "m2", PlayerID: "p2"})
	_, _ = store.UpsertPlayerSummary(AnticheatPlayerSummary{PlayerID: "p1", SuspicionScore: 50})
	_, _ = store.UpsertPlayerSummary(AnticheatPlayerSummary{PlayerID: "p2", SuspicionScore: 95})

	stats := store.Stats()
	if stats.AnalysisCount != 2 {
		t.Fatalf("expected 2 analyses, got %d", stats.AnalysisCount)
	}
	if stats.PlayerCount != 2 {
		t.Fatalf("expected 2 players, got %d", stats.PlayerCount)
	}
	if stats.FlaggedPlayerCount != 1 {
		t.Fatalf("expected 1 flagged (>=80), got %d", stats.FlaggedPlayerCount)
	}
}

func TestAnticheatStorePruneAnalysesOlderThan(t *testing.T) {
	store := newTestAnticheatStore()
	now := time.Now().UTC()
	_, _ = store.RecordAnalysis(AnticheatAnalysisRecord{AnalysisID: "old1", MatchID: "m1", PlayerID: "p1", AnalyzedAt: now.Add(-72 * time.Hour)})
	_, _ = store.RecordAnalysis(AnticheatAnalysisRecord{AnalysisID: "old2", MatchID: "m2", PlayerID: "p2", AnalyzedAt: now.Add(-48 * time.Hour)})
	_, _ = store.RecordAnalysis(AnticheatAnalysisRecord{AnalysisID: "new1", MatchID: "m3", PlayerID: "p1", AnalyzedAt: now.Add(-1 * time.Hour)})

	cutoff := now.Add(-24 * time.Hour)
	removed, err := store.PruneAnalysesOlderThan(cutoff)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if removed != 2 {
		t.Fatalf("expected 2 rows removed, got %d", removed)
	}
	stats := store.Stats()
	if stats.AnalysisCount != 1 {
		t.Fatalf("expected 1 analysis remaining, got %d", stats.AnalysisCount)
	}
}
