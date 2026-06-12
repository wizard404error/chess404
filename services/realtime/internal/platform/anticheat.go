package platform

import (
	"sort"
	"strings"
	"sync"
	"time"
)

type inMemoryAnticheatStore struct {
	mu        sync.Mutex
	analyses  map[string]AnticheatAnalysisRecord
	summaries map[string]AnticheatPlayerSummary
}

func newInMemoryAnticheatStore() *inMemoryAnticheatStore {
	return &inMemoryAnticheatStore{
		analyses:  make(map[string]AnticheatAnalysisRecord),
		summaries: make(map[string]AnticheatPlayerSummary),
	}
}

// NewInMemoryAnticheatStore returns an in-memory AnticheatStore, suitable
// for development and tests. Data is lost on process restart.
func NewInMemoryAnticheatStore() AnticheatStore {
	return newInMemoryAnticheatStore()
}

func (s *inMemoryAnticheatStore) Backend() string { return "memory" }

func (s *inMemoryAnticheatStore) Close() error { return nil }

func (s *inMemoryAnticheatStore) RecordAnalysis(record AnticheatAnalysisRecord) (AnticheatAnalysisRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if record.AnalysisID == "" {
		record.AnalysisID = "ach_" + randomToken(10)
	}
	if record.AnalyzedAt.IsZero() {
		record.AnalyzedAt = time.Now().UTC()
	}
	s.analyses[record.AnalysisID] = record
	return record, nil
}

func (s *inMemoryAnticheatStore) UpsertPlayerSummary(summary AnticheatPlayerSummary) (AnticheatPlayerSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if summary.LastAnalyzedAt.IsZero() {
		summary.LastAnalyzedAt = time.Now().UTC()
	}
	existing, ok := s.summaries[summary.PlayerID]
	if ok {
		if summary.TotalGames == 0 {
			summary.TotalGames = existing.TotalGames
		}
		if summary.RecentAnalyses == nil {
			summary.RecentAnalyses = existing.RecentAnalyses
		}
	}
	s.summaries[summary.PlayerID] = summary
	return summary, nil
}

func (s *inMemoryAnticheatStore) ListFlaggedPlayers(minScore float64, limit int) []AnticheatPlayerSummary {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]AnticheatPlayerSummary, 0)
	for _, summary := range s.summaries {
		if summary.SuspicionScore >= minScore {
			out = append(out, summary)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].SuspicionScore > out[j].SuspicionScore
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (s *inMemoryAnticheatStore) GetPlayerSummary(playerID string) (AnticheatPlayerSummary, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	summary, ok := s.summaries[strings.TrimSpace(playerID)]
	return summary, ok
}

func (s *inMemoryAnticheatStore) ListPlayerAnalyses(playerID string, limit int) []AnticheatAnalysisRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	resolvedPlayerID := strings.TrimSpace(playerID)
	out := make([]AnticheatAnalysisRecord, 0)
	for _, record := range s.analyses {
		if record.PlayerID == resolvedPlayerID {
			out = append(out, record)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].AnalyzedAt.After(out[j].AnalyzedAt)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (s *inMemoryAnticheatStore) Stats() AnticheatStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	stats := AnticheatStats{
		AnalysisCount:      len(s.analyses),
		PlayerCount:        len(s.summaries),
		FlaggedPlayerCount: 0,
	}
	for _, summary := range s.summaries {
		if summary.SuspicionScore >= AnticheatSuspicionThreshold {
			stats.FlaggedPlayerCount++
		}
	}
	return stats
}
