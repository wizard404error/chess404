package platform

import (
	"strings"

	"github.com/chess404/realtime/internal/contracts"
)

type PublicMatchArchiveEntry struct {
	MatchID            string                          `json:"matchId"`
	Status             string                          `json:"status"`
	Winner             string                          `json:"winner,omitempty"`
	FinishReason       string                          `json:"finishReason,omitempty"`
	RulesVersion       string                          `json:"rulesVersion"`
	Queue              string                          `json:"queue,omitempty"`
	ModeID             contracts.MatchModeID           `json:"modeId,omitempty"`
	WhiteGuestID       string                          `json:"whiteGuestId,omitempty"`
	BlackGuestID       string                          `json:"blackGuestId,omitempty"`
	WhiteAccountID     string                          `json:"whiteAccountId,omitempty"`
	BlackAccountID     string                          `json:"blackAccountId,omitempty"`
	WhiteAccountHandle string                          `json:"whiteAccountHandle,omitempty"`
	BlackAccountHandle string                          `json:"blackAccountHandle,omitempty"`
	WhiteName          string                          `json:"whiteName,omitempty"`
	BlackName          string                          `json:"blackName,omitempty"`
	CreatedAt          string                          `json:"createdAt"`
	UpdatedAt          string                          `json:"updatedAt"`
	MoveCount          int                             `json:"moveCount"`
	LastMove           string                          `json:"lastMove,omitempty"`
	WhiteHandCount     int                             `json:"whiteHandCount"`
	BlackHandCount     int                             `json:"blackHandCount"`
	ChatMessageCount   int                             `json:"chatMessageCount"`
	Snapshot           contracts.MatchSnapshotResponse `json:"snapshot"`
}

func IsPublicReplayableMatch(entry MatchArchiveEntry) bool {
	if !strings.EqualFold(strings.TrimSpace(entry.Status), "finished") {
		return false
	}
	return strings.TrimSpace(entry.MatchID) != ""
}

func IsPublicLiveSpectateMatch(entry MatchArchiveEntry) bool {
	if !strings.EqualFold(strings.TrimSpace(entry.Status), "active") {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(entry.Queue), "direct") {
		return false
	}
	if strings.TrimSpace(entry.Winner) != "" || strings.TrimSpace(entry.FinishReason) != "" {
		return false
	}
	return strings.TrimSpace(entry.MatchID) != ""
}

func BuildPublicMatchArchiveEntry(entry MatchArchiveEntry) PublicMatchArchiveEntry {
	snapshot := cloneSnapshot(entry.Snapshot)
	snapshot.Match = sanitizePublicMatchState(snapshot.Match)
	snapshot.Events = sanitizePublicEvents(snapshot.Events)

	return PublicMatchArchiveEntry{
		MatchID:            entry.MatchID,
		Status:             entry.Status,
		Winner:             entry.Winner,
		FinishReason:       entry.FinishReason,
		RulesVersion:       entry.RulesVersion,
		Queue:              entry.Queue,
		ModeID:             entry.ModeID,
		WhiteGuestID:       entry.WhiteGuestID,
		BlackGuestID:       entry.BlackGuestID,
		WhiteAccountID:     entry.WhiteAccountID,
		BlackAccountID:     entry.BlackAccountID,
		WhiteAccountHandle: entry.WhiteAccountHandle,
		BlackAccountHandle: entry.BlackAccountHandle,
		WhiteName:          entry.WhiteName,
		BlackName:          entry.BlackName,
		CreatedAt:          entry.CreatedAt.UTC().Format(timeLayoutRFC3339),
		UpdatedAt:          entry.UpdatedAt.UTC().Format(timeLayoutRFC3339),
		MoveCount:          entry.MoveCount,
		LastMove:           entry.LastMove,
		WhiteHandCount:     len(entry.Snapshot.Match.WhiteHand),
		BlackHandCount:     len(entry.Snapshot.Match.BlackHand),
		ChatMessageCount:   len(entry.Snapshot.Match.ChatMessages),
		Snapshot:           snapshot,
	}
}

const timeLayoutRFC3339 = "2006-01-02T15:04:05.999999999Z07:00"

func sanitizePublicMatchState(state contracts.MatchState) contracts.MatchState {
	safe := cloneMatchState(state)
	safe.WhiteHand = nil
	safe.BlackHand = nil
	safe.ChatMessages = nil
	safe.SeenClientMoveIDs = nil
	safe.History = nil
	return safe
}

func sanitizePublicEvents(events []contracts.ResolvedEvent) []contracts.ResolvedEvent {
	if len(events) == 0 {
		return nil
	}
	safe := make([]contracts.ResolvedEvent, 0, len(events))
	for _, event := range events {
		safe = append(safe, contracts.ResolvedEvent{
			ID:      event.ID,
			MatchID: event.MatchID,
			Type:    event.Type,
			At:      event.At,
		})
	}
	return safe
}
