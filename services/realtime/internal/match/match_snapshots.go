package match

import (
	"time"

	"github.com/chess404/realtime/internal/contracts"
)

func cloneState(state *contracts.MatchState) contracts.MatchState {
	clone := *state
	clone.Board = cloneBoard(state.Board)
	clone.Moved = append([]string{}, state.Moved...)
	clone.MoveHistory = append([]string{}, state.MoveHistory...)
	clone.ChatMessages = append([]contracts.ChatMessage{}, state.ChatMessages...)
	clone.WhiteHand = append([]contracts.GameCard{}, state.WhiteHand...)
	clone.BlackHand = append([]contracts.GameCard{}, state.BlackHand...)
	clone.LavaSquares = append([]contracts.LavaSquare{}, state.LavaSquares...)
	clone.BombPieces = append([]contracts.BombPiece{}, state.BombPieces...)
	clone.BlackHoles = append([]contracts.BlackHoleZone{}, state.BlackHoles...)
	clone.FogZones = append([]contracts.FogZone{}, state.FogZones...)
	clone.FortressZones = append([]contracts.FortressZone{}, state.FortressZones...)
	clone.History = append([]contracts.PositionState{}, state.History...)
	clone.SeenClientMoveIDs = append([]string{}, state.SeenClientMoveIDs...)
	if state.DoubleMove != nil {
		doubleMove := *state.DoubleMove
		if state.DoubleMove.TrackedSq != nil {
			tracked := *state.DoubleMove.TrackedSq
			doubleMove.TrackedSq = &tracked
		}
		clone.DoubleMove = &doubleMove
	}
	if state.InvisiblePiece != nil {
		invisible := *state.InvisiblePiece
		clone.InvisiblePiece = &invisible
	}
	if state.CheaterState != nil {
		cheater := *state.CheaterState
		clone.CheaterState = &cheater
	}
	if state.LastMove != nil {
		last := *state.LastMove
		clone.LastMove = &last
	}
	if state.PendingCard != nil {
		pending := *state.PendingCard
		clone.PendingCard = &pending
	}
	return clone
}

func cloneStateAt(state *contracts.MatchState, presence *matchPresenceState, now time.Time) contracts.MatchState {
	clone := cloneState(state)
	if presence != nil {
		clone.WhiteConnected = presence.WhiteConnected
		clone.BlackConnected = presence.BlackConnected
		hideDisconnectGrace := presence.DisconnectGraceFor == disconnectGraceBoth
		if hideDisconnectGrace {
			clone.DisconnectGraceFor = ""
		} else {
			clone.DisconnectGraceFor = presence.DisconnectGraceFor
		}
		if !hideDisconnectGrace && presence.DisconnectGraceDeadline != nil {
			deadline := *presence.DisconnectGraceDeadline
			clone.DisconnectGraceDeadline = &deadline
		} else {
			clone.DisconnectGraceDeadline = nil
		}
	}
	applyClockView(&clone, now)
	return clone
}

func filterEventsForColor(events []contracts.ResolvedEvent, color string) []contracts.ResolvedEvent {
	if color == "" {
		return events
	}
	filtered := make([]contracts.ResolvedEvent, 0, len(events))
	for _, e := range events {
		if e.Type == "card_drawn" && e.Payload != nil {
			owner, _ := e.Payload["owner"].(string)
			if owner != "" && owner != color {
				continue
			}
		}
		filtered = append(filtered, e)
	}
	return filtered
}

func filterStateForColor(state contracts.MatchState, color string) contracts.MatchState {
	if color == "white" {
		state.BlackHand = nil
	} else if color == "black" {
		state.WhiteHand = nil
	} else {
		state.WhiteHand = nil
		state.BlackHand = nil
	}
	if state.InvisiblePiece != nil && state.InvisiblePiece.OwnerColor != color {
		state.InvisiblePiece = nil
	}
	if state.FogZones != nil {
		filteredFog := make([]contracts.FogZone, 0, len(state.FogZones))
		for _, fz := range state.FogZones {
			if fz.OwnerColor == color {
				filteredFog = append(filteredFog, fz)
			}
		}
		state.FogZones = filteredFog
	}
	return state
}

func applyClockView(state *contracts.MatchState, now time.Time) {
	if state.Status != "active" || state.Clock.RunningFor == "" || state.Clock.StartedAt == nil {
		return
	}

	elapsed := now.UnixMilli() - *state.Clock.StartedAt
	if elapsed <= 0 {
		return
	}

	switch state.Clock.RunningFor {
	case "white":
		state.Clock.WhiteMS = maxInt64(0, state.Clock.WhiteMS-elapsed)
	case "black":
		state.Clock.BlackMS = maxInt64(0, state.Clock.BlackMS-elapsed)
	}
	startedAt := now.UnixMilli()
	state.Clock.StartedAt = &startedAt
}

func buildSnapshot(state *contracts.MatchState, replayHead int, events []contracts.ResolvedEvent, now time.Time) contracts.MatchSnapshotResponse {
	return contracts.MatchSnapshotResponse{
		Match:        cloneStateAt(state, nil, now),
		ReplayHead:   replayHead,
		ReplayFrames: buildReplayFrames(state),
		Events:       cloneEvents(events),
	}
}

func buildSnapshotWithPresence(state *contracts.MatchState, presence *matchPresenceState, replayHead int, events []contracts.ResolvedEvent, now time.Time) contracts.MatchSnapshotResponse {
	return contracts.MatchSnapshotResponse{
		Match:        cloneStateAt(state, presence, now),
		ReplayHead:   replayHead,
		ReplayFrames: buildReplayFrames(state),
		Events:       cloneEvents(events),
	}
}

func buildReplayFrames(state *contracts.MatchState) []contracts.ReplayFrame {
	if len(state.History) == 0 {
		return nil
	}

	frames := make([]contracts.ReplayFrame, 0, len(state.History))
	for index, position := range state.History {
		frame := contracts.ReplayFrame{
			Index:         index,
			Turn:          position.Turn,
			Board:         cloneBoard(position.Board),
			HalfMoveClock: position.HalfMoveClock,
			FullMoveNum:   position.FullMoveNum,
			MoveHistory:   append([]string{}, position.MoveHistory...),
		}
		if position.LastMove != nil {
			last := *position.LastMove
			frame.LastMove = &last
		}
		frames = append(frames, frame)
	}
	return frames
}
