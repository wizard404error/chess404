package match

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/chess404/realtime/internal/contracts"
	"github.com/chess404/realtime/internal/engine"
)

// redactToken replaces common token and secret patterns with [REDACTED].
func redactToken(s string) string {
	re := regexp.MustCompile(`(?i)([?&](?:playerSecret|token|secret|s)=)[^&\s]+`)
	return re.ReplaceAllString(s, "${1}[REDACTED]")
}

func (s *Service) CreateMatch(req contracts.CreateMatchRequest, now time.Time) contracts.MatchSnapshotResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	matchID := req.MatchID
	if matchID == "" {
		b := make([]byte, 4)
		rand.Read(b)
		matchID = fmt.Sprintf("match_%d_%s", now.UnixMilli(), hex.EncodeToString(b))
	}
	s.Log.Info("match:create: starting", "matchID", matchID, "modeID", string(req.ModeID), "queue", req.Queue, "clockSeconds", req.ClockSeconds, "whiteGuestID", req.WhiteGuestID, "blackGuestID", req.BlackGuestID, "difficulty", req.Difficulty)

	clockMS := defaultClock
	if req.ClockSeconds > 0 {
		clockMS = req.ClockSeconds * 1000
	}
	increment := int64(0)
	if req.ClockIncrement > 0 {
		increment = req.ClockIncrement * 1000
	}

	startedAt := now.UnixMilli()
	hasWhiteSeat := strings.TrimSpace(req.WhiteGuestID) != ""
	hasBlackSeat := strings.TrimSpace(req.BlackGuestID) != ""
	hasPartialSeats := hasWhiteSeat != hasBlackSeat
	runningFor := ""
	var startedAtPtr *int64
	status := "active"
	if hasPartialSeats {
		status = "waiting"
	} else {
		runningFor = "white"
		startedAtPtr = &startedAt
	}
	state := &contracts.MatchState{
		MatchID:           matchID,
		RulesVersion:      rulesVersion,
		RNGSeed:           chooseSeed(req.Seed, startedAt),
		Queue:             req.Queue,
		ModeID:            contracts.NormalizeMatchModeID(string(req.ModeID)),
		WhiteGuestID:      strings.TrimSpace(req.WhiteGuestID),
		BlackGuestID:      strings.TrimSpace(req.BlackGuestID),
		WhiteAccountID:    req.WhiteAccountID,
		BlackAccountID:    req.BlackAccountID,
		WhiteName:         req.WhiteName,
		BlackName:         req.BlackName,
		WhitePlayerSecret: req.WhitePlayerSecret,
		BlackPlayerSecret: req.BlackPlayerSecret,
		Board:             makeBoard(),
		Turn:              "white",
		Moved:             []string{},
		HalfMoveClock:     0,
		FullMoveNum:       1,
		WhiteHand:         cloneCardsWithOwner(starterHandCardsForMode(req.StarterHandMode), "white"),
		BlackHand:         cloneCardsWithOwner(starterHandCardsForMode(req.StarterHandMode), "black"),
		MoveHistory:       []string{},
		ChatMessages:      []contracts.ChatMessage{},
		Clock: contracts.MatchClock{
			WhiteMS:    clockMS,
			BlackMS:    clockMS,
			RunningFor: runningFor,
			StartedAt:  startedAtPtr,
			Increment:  increment,
		},
		Status:    status,
		CreatedAt: now.UTC(),
		UpdatedAt: now.UTC(),
	}
	state.History = []contracts.PositionState{capturePositionState(state)}

	startEvent := makeEvent(matchID, "match_started", now, "", map[string]any{
		"turn": "white",
	})

	s.matches[matchID] = state
	s.events[matchID] = []contracts.ResolvedEvent{startEvent}
	s.subs[matchID] = make(map[chan contracts.MatchSnapshotResponse]string)
	s.presence[matchID] = newMatchPresenceState(state, now)

	if string(req.ModeID) == "computer" {
		diff := engine.ParseDifficulty(req.Difficulty)
		s.computers[matchID] = engine.NewComputerOpponent(diff, "black")
		s.Log.Info("match:create: computer opponent initialized", "matchID", matchID, "difficulty", diff, "color", "black")
	}

	broadcastSnap := buildSnapshotWithPresence(state, s.presence[matchID], len(s.events[matchID]), []contracts.ResolvedEvent{startEvent}, now)
	persistSnap := buildSnapshotWithPresence(state, s.presence[matchID], len(s.events[matchID]), s.events[matchID], now)
	s.persistSnapshot(persistSnap)
	s.saveToRedis(persistSnap, s.presence[matchID])
	s.Log.Info("match:create: ok", "matchID", matchID, "status", broadcastSnap.Match.Status, "turn", broadcastSnap.Match.Turn, "whiteFingerprint", redactPlayerSecret(broadcastSnap.Match.WhitePlayerSecret), "blackFingerprint", redactPlayerSecret(broadcastSnap.Match.BlackPlayerSecret), "computers", s.computers[matchID] != nil)

	return broadcastSnap
}

func (s *Service) JoinMatchSeat(matchID string, req contracts.JoinMatchSeatRequest, now time.Time) (contracts.JoinMatchSeatResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.ensureMatchLoadedLocked(matchID)
	if !ok {
		return contracts.JoinMatchSeatResponse{}, ErrMatchNotFound
	}
	if state.Status == "finished" {
		return contracts.JoinMatchSeatResponse{}, ErrMatchJoinFinished
	}

	guestID := strings.TrimSpace(req.GuestID)
	if guestID == "" {
		return contracts.JoinMatchSeatResponse{}, errors.New("guestId is required")
	}

	displayName := strings.TrimSpace(req.DisplayName)
	accountID := strings.TrimSpace(req.AccountID)
	playerSecret := strings.TrimSpace(req.PlayerSecret)
	preferredSeat := strings.ToLower(strings.TrimSpace(req.PreferredSeat))
	if preferredSeat != "" && preferredSeat != "white" && preferredSeat != "black" {
		return contracts.JoinMatchSeatResponse{}, errors.New("preferredSeat must be white or black")
	}

	seatColor := ""
	joined := false
	updated := false

	switch {
	case strings.EqualFold(state.WhiteGuestID, guestID):
		seatColor = "white"
		if displayName != "" && state.WhiteName != displayName {
			state.WhiteName = displayName
			updated = true
		}
		if accountID != "" && state.WhiteAccountID != accountID {
			state.WhiteAccountID = accountID
			updated = true
		}
		if state.WhitePlayerSecret == "" && playerSecret != "" {
			state.WhitePlayerSecret = playerSecret
			updated = true
		}
	case strings.EqualFold(state.BlackGuestID, guestID):
		seatColor = "black"
		if displayName != "" && state.BlackName != displayName {
			state.BlackName = displayName
			updated = true
		}
		if accountID != "" && state.BlackAccountID != accountID {
			state.BlackAccountID = accountID
			updated = true
		}
		if state.BlackPlayerSecret == "" && playerSecret != "" {
			state.BlackPlayerSecret = playerSecret
			updated = true
		}
	default:
		seatColor = chooseOpenSeat(state, preferredSeat)
		if seatColor == "" {
			return contracts.JoinMatchSeatResponse{}, ErrMatchSeatFull
		}
		if seatColor == "white" {
			state.WhiteGuestID = guestID
			state.WhiteName = displayName
			state.WhiteAccountID = accountID
			state.WhitePlayerSecret = playerSecret
		} else {
			state.BlackGuestID = guestID
			state.BlackName = displayName
			state.BlackAccountID = accountID
			state.BlackPlayerSecret = playerSecret
		}
		joined = true
		updated = true
	}

	events := make([]contracts.ResolvedEvent, 0, 2)
	if joined {
		events = append(events, makeEvent(matchID, "seat_joined", now, guestID, map[string]any{
			"event":     "seat_joined",
			"seatColor": seatColor,
			"guestId":   guestID,
		}))
	}

	if strings.TrimSpace(state.WhiteGuestID) != "" && strings.TrimSpace(state.BlackGuestID) != "" {
		if state.Status != "active" {
			startedAt := now.UnixMilli()
			state.Status = "active"
			state.Clock.RunningFor = "white"
			state.Clock.StartedAt = &startedAt
			updated = true
			events = append(events, makeEvent(matchID, "match_started", now, guestID, map[string]any{
				"turn": "white",
			}))
		}
	} else {
		if state.Status != "waiting" {
			state.Status = "waiting"
			updated = true
		}
		state.Clock.RunningFor = ""
		state.Clock.StartedAt = nil
	}

	if updated {
		state.UpdatedAt = now.UTC()
	}

	if len(events) > 0 {
		s.events[matchID] = append(s.events[matchID], events...)
		presence := s.ensurePresenceStateLocked(matchID, state, now)
		snapshot := buildSnapshotWithPresence(state, presence, len(s.events[matchID]), events, now)
		persistSnap := buildSnapshotWithPresence(state, presence, len(s.events[matchID]), s.events[matchID], now)
		s.persistSnapshot(persistSnap)
		s.saveToRedis(persistSnap, presence)
		s.broadcastLocked(matchID, snapshot)
		return contracts.JoinMatchSeatResponse{
			Match:              snapshot,
			SeatColor:          seatColor,
			Joined:             joined,
			WaitingForOpponent: state.Status == "waiting",
		}, nil
	}

	fullSnapshot := buildSnapshotWithPresence(state, s.ensurePresenceStateLocked(matchID, state, now), len(s.events[matchID]), nil, now)
	if updated {
		s.persistSnapshot(fullSnapshot)
		s.saveToRedis(fullSnapshot, s.presence[matchID])
	}
	return contracts.JoinMatchSeatResponse{
		Match:              fullSnapshot,
		SeatColor:          seatColor,
		Joined:             joined,
		WaitingForOpponent: state.Status == "waiting",
	}, nil
}

func (s *Service) ApplyIntent(intent contracts.PlayerIntent, now time.Time) (contracts.MatchSnapshotResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.ensureMatchLoadedLocked(intent.MatchID)
	if !ok {
		return contracts.MatchSnapshotResponse{}, ErrMatchNotFound
	}

	if intent.ClientMoveID != "" {
		for _, id := range state.SeenClientMoveIDs {
			if id == intent.ClientMoveID {
				presence := s.ensurePresenceStateLocked(intent.MatchID, state, now)
				return buildSnapshotWithPresence(state, presence, len(s.events[intent.MatchID]), nil, now), nil
			}
		}
	}

	if intent.ExpectedSeqNum > 0 {
		currentSeq := s.matchSeqNum[intent.MatchID]
		if currentSeq > 0 && intent.ExpectedSeqNum < currentSeq {
			return contracts.MatchSnapshotResponse{}, ErrStaleClientState
		}
	}

	presence := s.ensurePresenceStateLocked(intent.MatchID, state, now)

	actorColor, _ := requireIntentColor(state, intent.PlayerID, intent.PlayerSecret)
	if err := rateLimitIntent(presence, actorColor, now); err != nil {
		return contracts.MatchSnapshotResponse{}, err
	}

	timeoutEvents := syncClockForMutation(state, now)
	if len(timeoutEvents) > 0 {
		if events, err := applyIntent(state, intent, now); err == nil {
			trackIntentTime(presence, actorColor, now)
			events = append(events, timeoutEvents...)
			s.events[intent.MatchID] = append(s.events[intent.MatchID], events...)
			snapshot := buildSnapshotWithPresence(state, presence, len(s.events[intent.MatchID]), events, now)
			persistSnap := buildSnapshot(state, len(s.events[intent.MatchID]), s.events[intent.MatchID], now)
			s.persistSnapshot(persistSnap)
			s.saveToRedis(persistSnap, presence)
			s.broadcastLocked(intent.MatchID, snapshot)
			return snapshot, nil
		}
		s.events[intent.MatchID] = append(s.events[intent.MatchID], timeoutEvents...)
		snapshot := buildSnapshotWithPresence(state, presence, len(s.events[intent.MatchID]), timeoutEvents, now)
		persistSnap := buildSnapshot(state, len(s.events[intent.MatchID]), s.events[intent.MatchID], now)
		s.persistSnapshot(persistSnap)
		s.saveToRedis(persistSnap, presence)
		s.broadcastLocked(intent.MatchID, snapshot)
		return snapshot, nil
	}

	savedClock := state.Clock
	savedStatus := state.Status
	savedWinner := state.Winner
	savedFinishReason := state.FinishReason

	events, err := applyIntent(state, intent, now)
	if err != nil {
		state.Clock = savedClock
		state.Status = savedStatus
		state.Winner = savedWinner
		state.FinishReason = savedFinishReason
		return contracts.MatchSnapshotResponse{}, err
	}

	trackIntentTime(presence, actorColor, now)

	if shouldEvaluateAutomaticMatchFinish(state, intent) {
		events = finalizeAutomaticMatchFinish(state, events, now, intent.PlayerID)
	}

	timeoutEvents = syncClockForMutation(state, now)
	events = append(events, timeoutEvents...)

	if intent.ClientMoveID != "" {
		state.SeenClientMoveIDs = append(state.SeenClientMoveIDs, intent.ClientMoveID)
		if len(state.SeenClientMoveIDs) > 1000 {
			state.SeenClientMoveIDs = state.SeenClientMoveIDs[len(state.SeenClientMoveIDs)-1000:]
		}
	}

	s.events[intent.MatchID] = append(s.events[intent.MatchID], events...)
	snapshot := buildSnapshotWithPresence(state, presence, len(s.events[intent.MatchID]), events, now)
	persistSnap := buildSnapshot(state, len(s.events[intent.MatchID]), s.events[intent.MatchID], now)
	s.persistSnapshot(persistSnap)
	s.saveToRedis(persistSnap, presence)
	s.broadcastLocked(intent.MatchID, snapshot)

	s.autoPlayComputer(intent.MatchID, state, presence, now)

	return snapshot, nil
}

func (s *Service) autoPlayComputer(matchID string, state *contracts.MatchState, presence *matchPresenceState, now time.Time) {
	s.autoPlayComputerDepthLimited(matchID, state, presence, now, 0)
}

func (s *Service) autoPlayComputerDepthLimited(matchID string, state *contracts.MatchState, presence *matchPresenceState, now time.Time, depth int) {
	if depth > 5 {
		s.Log.Info("match:autoPlay: max recursion depth reached", "matchID", matchID)
		return
	}
	computer, ok := s.computers[matchID]
	if !ok || state.Status != "active" || state.Turn != "black" {
		return
	}

	computerIntent := computer.MakeMove(state)
	if computerIntent == nil {
		s.Log.Info("match:autoPlay: computer returned NIL intent", "matchID", matchID, "turn", state.Turn, "status", state.Status)
		return
	}

	savedClock := state.Clock
	savedStatus := state.Status
	savedWinner := state.Winner
	savedFinishReason := state.FinishReason

	events, err := applyIntent(state, *computerIntent, now)
	if err != nil {
		state.Clock = savedClock
		state.Status = savedStatus
		state.Winner = savedWinner
		state.FinishReason = savedFinishReason
		return
	}

	if shouldEvaluateAutomaticMatchFinish(state, *computerIntent) {
		events = finalizeAutomaticMatchFinish(state, events, now, "computer")
	}

	timeoutEvents := syncClockForMutation(state, now)
	events = append(events, timeoutEvents...)

	s.events[matchID] = append(s.events[matchID], events...)
	snapshot := buildSnapshotWithPresence(state, presence, len(s.events[matchID]), events, now)
	persistSnap := buildSnapshot(state, len(s.events[matchID]), s.events[matchID], now)
	s.persistSnapshot(persistSnap)
	s.saveToRedis(persistSnap, presence)
	s.broadcastLocked(matchID, snapshot)

	if state.Turn == "black" && state.Status == "active" {
		s.autoPlayComputerDepthLimited(matchID, state, presence, now, depth+1)
	}
}

func finalizeAutomaticMatchFinish(state *contracts.MatchState, events []contracts.ResolvedEvent, now time.Time, actorID string) []contracts.ResolvedEvent {
	if state == nil || state.Status != "active" {
		return events
	}

	winner, finishReason := evaluateAutomaticMatchFinish(state)
	if finishReason == "" {
		return events
	}

	markMatchFinished(state, winner, finishReason, now)

	clockUpdated := false
	for index := len(events) - 1; index >= 0; index-- {
		if events[index].Type != "clock_updated" {
			continue
		}
		if events[index].Payload == nil {
			events[index].Payload = map[string]any{}
		}
		events[index].Payload["runningFor"] = ""
		clockUpdated = true
		break
	}
	if !clockUpdated {
		events = append(events, makeEvent(state.MatchID, "clock_updated", now, actorID, map[string]any{
			"runningFor": "",
		}))
	}

	payload := map[string]any{
		"result": finishReason,
	}
	if winner != "" {
		payload["winner"] = winner
	}
	events = append(events, makeEvent(state.MatchID, "match_finished", now, actorID, payload))
	return events
}

func shouldEvaluateAutomaticMatchFinish(state *contracts.MatchState, intent contracts.PlayerIntent) bool {
	if state == nil || state.Status != "active" {
		return false
	}
	if state.InvisiblePiece != nil {
		return false
	}
	if state.DoubleMove != nil && state.DoubleMove.MovesLeft > 0 {
		return false
	}

	switch intent.Type {
	case "make_move":
		return true
	case "play_card", "select_target":
		return state.PendingCard == nil
	default:
		return false
	}
}

func evaluateAutomaticMatchFinish(state *contracts.MatchState) (string, string) {
	if state == nil {
		return "", ""
	}

	if state.HalfMoveClock >= 100 {
		return "draw", "fifty_move_rule"
	}

	historyKeys := make([]string, 0, len(state.History))
	for _, position := range state.History {
		historyKeys = append(historyKeys, positionKey(position.Board, position.Turn, sliceToSet(position.Moved), position.LastMove))
	}
	currentKey := positionKey(state.Board, state.Turn, sliceToSet(state.Moved), state.LastMove)
	if threefold(historyKeys, currentKey) {
		return "draw", "threefold_repetition"
	}

	_, isMate, isStale := gameStatusWithFusion(state.Board, state.Turn, state.LastMove, sliceToSet(state.Moved))
	if isMate {
		return opposite(state.Turn), "checkmate"
	}
	if isStale {
		return "draw", "stalemate"
	}
	if insufficientMaterial(state.Board) {
		return "draw", "insufficient_material"
	}

	return "", ""
}

func markMatchFinished(state *contracts.MatchState, winner string, finishReason string, now time.Time) {
	if state == nil {
		return
	}
	state.Status = "finished"
	state.Winner = winner
	state.FinishReason = finishReason
	state.DrawOfferedBy = ""
	state.Clock.RunningFor = ""
	state.Clock.StartedAt = nil
	state.PendingCard = nil
	state.DoubleMove = nil
	state.InvisiblePiece = nil
	state.FogZones = nil
	state.FortressZones = nil
	state.BombPieces = nil
	state.LavaSquares = nil
	state.BlackHoles = nil
	state.UndoAgainst = ""
	state.RadarRevealFor = ""
	state.CheaterState = nil
	state.UpdatedAt = now.UTC()
}

func applyIntent(state *contracts.MatchState, intent contracts.PlayerIntent, now time.Time) ([]contracts.ResolvedEvent, error) {
	switch intent.Type {
	case "make_move":
		return applyMove(state, intent, now)
	case "play_card":
		return applyPlayCard(state, intent, now)
	case "select_target":
		return applySelectTarget(state, intent, now)
	case "send_chat":
		return applyChat(state, intent, now)
	case "offer_draw":
		return applyOfferDraw(state, intent, now)
	case "respond_draw":
		return applyRespondDraw(state, intent, now)
	case "abort":
		return applyAbort(state, intent, now)
	case "resign":
		return applyResign(state, intent, now)
	default:
		return nil, fmt.Errorf("unsupported intent type: %s", intent.Type)
	}
}

func applyMove(state *contracts.MatchState, intent contracts.PlayerIntent, now time.Time) ([]contracts.ResolvedEvent, error) {
	if err := ensureActive(state); err != nil {
		return nil, err
	}
	if intent.From == nil || intent.To == nil {
		return nil, errors.New("move intent requires from and to")
	}
	actorColor, err := requireIntentColor(state, intent.PlayerID, intent.PlayerSecret)
	if err != nil {
		return nil, err
	}
	if actorColor != state.Turn {
		return nil, errors.New("cannot move out of turn")
	}

	if state.InvisiblePiece != nil &&
		state.InvisiblePiece.OwnerColor == state.Turn &&
		state.InvisiblePiece.Row == intent.From.Row &&
		state.InvisiblePiece.Col == intent.From.Col {
		return applyInvisibleMove(state, intent, now)
	}

	piece := pieceAt(state.Board, *intent.From)
	if piece == nil {
		return nil, errors.New("no piece on source square")
	}
	if piece.Color != actorColor {
		return nil, errors.New("cannot move opponent piece")
	}
	if piece.Frozen {
		return nil, errors.New("frozen pieces cannot move")
	}
	if state.DoubleMove != nil && state.DoubleMove.MovesLeft == 1 && state.DoubleMove.TrackedSq != nil {
		tracked := state.DoubleMove.TrackedSq
		if state.DoubleMove.Type == "same" && (intent.From.Row != tracked.Row || intent.From.Col != tracked.Col) {
			return nil, errors.New("solo double move requires moving the same piece again")
		}
		if state.DoubleMove.Type == "diff" && intent.From.Row == tracked.Row && intent.From.Col == tracked.Col {
			return nil, errors.New("twin double move requires moving a different piece")
		}
	}

	legal := legalMovesWithFusion(state.Board, *intent.From, state.LastMove, sliceToSet(state.Moved))
	if !containsSquare(legal, *intent.To) {
		return nil, errors.New("illegal move")
	}
	if fortressEntryBlocked(state.FortressZones, piece.Color, *intent.To) {
		return nil, errors.New("destination square is protected by an enemy fortress")
	}

	captured := pieceAt(state.Board, *intent.To)
	isEnPassantCapture := piece.Type == "pawn" && intent.From.Col != intent.To.Col && captured == nil
	capturedSquare := *intent.To
	if isEnPassantCapture {
		capturedSquare = contracts.Square{Row: intent.From.Row, Col: intent.To.Col}
		captured = pieceAt(state.Board, capturedSquare)
	}
	if captured != nil && captured.Shielded {
		captured.Shielded = false
		captured.ShieldTurn = nil
		state.DrawOfferedBy = ""
		state.UpdatedAt = now.UTC()
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
				"effect": "shield_blocked_capture",
				"target": intent.To,
			}),
		}, nil
	}

	nextBoard := cloneBoard(state.Board)
	nextPiece := pieceAt(nextBoard, *intent.From)
	movePiece(nextBoard, *intent.From, *intent.To, nextPiece, isEnPassantCapture)
	if err := resolveParasiteEffects(nextBoard, *intent.From, *intent.To, capturedSquare, captured); err != nil {
		return nil, err
	}
	updateParasiteLinksForMove(nextBoard, *intent.From, *intent.To)
	promotion := ""
	if nextPiece != nil && nextPiece.Type == "pawn" && !nextPiece.Fake && (intent.To.Row == 0 || intent.To.Row == 7) {
		promoteTo := "queen"
		if intent.Promotion != "" {
			switch intent.Promotion {
			case "queen", "rook", "bishop", "knight":
				promoteTo = intent.Promotion
			}
		}
		nextBoard[intent.To.Row][intent.To.Col].Type = promoteTo
		promotion = promoteTo
	}
	if state.DoubleMove != nil && state.DoubleMove.MovesLeft == 2 {
		oppKing := findKing(nextBoard, opposite(state.Turn))
		if oppKing != nil && isAttackedWithFusion(nextBoard, *oppKing, state.Turn) {
			return nil, errors.New("first double move cannot put enemy king in check")
		}
	}
	state.Board = nextBoard
	lavaTriggered, lavaCapturedPiece := resolveLavaEffects(state, *intent.To)
	updateBombTracker(state, *intent.From, *intent.To)

	captureOccurred := captured != nil
	if lavaTriggered && lavaCapturedPiece != "" {
		captureOccurred = true
	}
	notation := moveNotation(state.Board, *intent.From, *intent.To, piece, captureOccurred)
	state.Moved = append(state.Moved, keyForSquare(*intent.From))
	state.LastMove = &contracts.LastMove{From: *intent.From, To: *intent.To}
	state.DrawOfferedBy = ""
	state.HalfMoveClock = nextHalfMoveClock(state.HalfMoveClock, piece.Type, captureOccurred)
	if state.DoubleMove != nil {
		newMovesLeft := state.DoubleMove.MovesLeft - 1
		if newMovesLeft > 0 {
			tracked := &contracts.Square{Row: intent.To.Row, Col: intent.To.Col}
			state.DoubleMove = &contracts.DoubleMoveState{
				Type:      state.DoubleMove.Type,
				MovesLeft: newMovesLeft,
				TrackedSq: tracked,
				FirstNote: notation,
			}
			startedAt := now.UnixMilli()
			state.Clock.RunningFor = state.Turn
			state.Clock.StartedAt = &startedAt
			state.UpdatedAt = now.UTC()

			payload := map[string]any{
				"from":       intent.From,
				"to":         intent.To,
				"notation":   notation,
				"nextTurn":   state.Turn,
				"doubleMove": state.DoubleMove,
			}
			if promotion != "" {
				payload["promotion"] = promotion
			}
			if lavaTriggered {
				payload["lavaTriggered"] = true
				if lavaCapturedPiece != "" {
					payload["lavaCapturedPiece"] = lavaCapturedPiece
				}
			}

			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "move_applied", now, intent.PlayerID, payload),
				makeEvent(state.MatchID, "clock_updated", now, intent.PlayerID, map[string]any{
					"runningFor": state.Turn,
				}),
			}, nil
		}

		if state.DoubleMove.FirstNote != "" {
			state.MoveHistory = append(state.MoveHistory, state.DoubleMove.FirstNote+"+"+notation)
		} else {
			state.MoveHistory = append(state.MoveHistory, notation)
		}
		state.DoubleMove = nil
	} else {
		state.MoveHistory = append(state.MoveHistory, notation)
	}

	if state.Turn == "black" {
		state.FullMoveNum++
	}
	justMovedColor := state.Turn
	state.Turn = opposite(state.Turn)
	cleanupTemporaryEffects(state, justMovedColor)
	resolveFogEffects(state, justMovedColor)
	resolveFortressEffects(state, justMovedColor)
	bombExplodedSquares := resolveBombEffects(state)
	blackHoleExplodedSquares := resolveBlackHoleEffects(state, justMovedColor)
	roundDrawWhite, roundDrawBlack := drawRoundCards(state, now)
	startedAt := now.UnixMilli()
	if state.Clock.Increment > 0 {
		if justMovedColor == "white" {
			state.Clock.WhiteMS += state.Clock.Increment
		} else {
			state.Clock.BlackMS += state.Clock.Increment
		}
	}
	state.Clock.RunningFor = state.Turn
	state.Clock.StartedAt = &startedAt
	state.UpdatedAt = now.UTC()
	state.History = append(state.History, capturePositionState(state))

	posKey := positionKey(state.Board, state.Turn, sliceToSet(state.Moved), state.LastMove)
	repCount := 1
	for _, pos := range state.History {
		if positionKey(pos.Board, pos.Turn, sliceToSet(pos.Moved), pos.LastMove) == posKey {
			repCount++
		}
	}
	if repCount >= 3 {
		markMatchFinished(state, "draw", "threefold_repetition", now)
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "move_applied", now, intent.PlayerID, map[string]any{"notation": notation, "nextTurn": state.Turn}),
			makeEvent(state.MatchID, "match_finished", now, intent.PlayerID, map[string]any{"result": "threefold_repetition", "winner": "draw"}),
		}, nil
	}
	if state.HalfMoveClock >= 100 {
		markMatchFinished(state, "draw", "fifty_move_rule", now)
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "move_applied", now, intent.PlayerID, map[string]any{"notation": notation, "nextTurn": state.Turn}),
			makeEvent(state.MatchID, "match_finished", now, intent.PlayerID, map[string]any{"result": "fifty_move_rule", "winner": "draw"}),
		}, nil
	}
	if insufficientMaterial(state.Board) {
		markMatchFinished(state, "draw", "insufficient_material", now)
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "move_applied", now, intent.PlayerID, map[string]any{"notation": notation, "nextTurn": state.Turn}),
			makeEvent(state.MatchID, "match_finished", now, intent.PlayerID, map[string]any{"result": "insufficient_material", "winner": "draw"}),
		}, nil
	}

	payload := map[string]any{
		"from":     intent.From,
		"to":       intent.To,
		"notation": notation,
		"nextTurn": state.Turn,
	}
	if promotion != "" {
		payload["promotion"] = promotion
	}
	if lavaTriggered {
		payload["lavaTriggered"] = true
		if lavaCapturedPiece != "" {
			payload["lavaCapturedPiece"] = lavaCapturedPiece
		}
	}
	if len(bombExplodedSquares) > 0 {
		payload["bombExplodedSquares"] = bombExplodedSquares
	}
	if len(blackHoleExplodedSquares) > 0 {
		payload["blackHoleExplodedSquares"] = blackHoleExplodedSquares
	}
	if len(roundDrawWhite) > 0 {
		payload["roundDrawWhite"] = roundDrawWhite
	}
	if len(roundDrawBlack) > 0 {
		payload["roundDrawBlack"] = roundDrawBlack
	}

	return []contracts.ResolvedEvent{
		makeEvent(state.MatchID, "move_applied", now, intent.PlayerID, payload),
		makeEvent(state.MatchID, "clock_updated", now, intent.PlayerID, map[string]any{
			"runningFor": state.Turn,
		}),
	}, nil
}

func applyInvisibleMove(state *contracts.MatchState, intent contracts.PlayerIntent, now time.Time) ([]contracts.ResolvedEvent, error) {
	invisible := state.InvisiblePiece
	if invisible == nil {
		return nil, errors.New("no invisible piece to move")
	}

	ghostBoard := cloneBoard(state.Board)
	ghostBoard[intent.From.Row][intent.From.Col] = &contracts.Piece{
		Type:           invisible.Piece.Type,
		Color:          invisible.Piece.Color,
		Shielded:       invisible.Piece.Shielded,
		ShieldTurn:     invisible.Piece.ShieldTurn,
		Frozen:         invisible.Piece.Frozen,
		Borrowed:       invisible.Piece.Borrowed,
		ParasiteTarget: invisible.Piece.ParasiteTarget,
		Bomb:           invisible.Piece.Bomb,
		Invisible:      invisible.Piece.Invisible,
		InvisibleTurn:  invisible.Piece.InvisibleTurn,
		InvisibleOver:  invisible.Piece.InvisibleOver,
		FusedWith:      invisible.Piece.FusedWith,
	}

	legal := legalMoves(ghostBoard, *intent.From, state.LastMove, sliceToSet(state.Moved))
	if !containsSquare(legal, *intent.To) {
		return nil, errors.New("illegal move")
	}
	if fortressEntryBlocked(state.FortressZones, invisible.Piece.Color, *intent.To) {
		return nil, errors.New("destination square is protected by an enemy fortress")
	}

	targetPiece := pieceAt(state.Board, *intent.To)
	givesCheckBoard := cloneBoard(state.Board)
	givesCheckBoard[intent.To.Row][intent.To.Col] = &contracts.Piece{
		Type:           invisible.Piece.Type,
		Color:          invisible.Piece.Color,
		Shielded:       invisible.Piece.Shielded,
		ShieldTurn:     invisible.Piece.ShieldTurn,
		Frozen:         invisible.Piece.Frozen,
		Borrowed:       invisible.Piece.Borrowed,
		ParasiteTarget: invisible.Piece.ParasiteTarget,
		Bomb:           invisible.Piece.Bomb,
		Invisible:      invisible.Piece.Invisible,
		InvisibleTurn:  invisible.Piece.InvisibleTurn,
		InvisibleOver:  invisible.Piece.InvisibleOver,
		FusedWith:      invisible.Piece.FusedWith,
	}
	oppKing := findKing(givesCheckBoard, opposite(state.Turn))
	givesCheck := oppKing != nil && isAttacked(givesCheckBoard, *oppKing, state.Turn)
	isCapture := targetPiece != nil && targetPiece.Color != state.Turn
	isMove2 := invisible.RoundsLeft <= 0

	if targetPiece != nil && targetPiece.Shielded {
		targetPiece.Shielded = false
		targetPiece.ShieldTurn = nil
		state.DrawOfferedBy = ""
		state.UpdatedAt = now.UTC()
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
				"effect": "shield_blocked_capture",
				"target": intent.To,
			}),
		}, nil
	}

	if givesCheck || isMove2 {
		nextBoard := cloneBoard(state.Board)
		nextBoard[intent.To.Row][intent.To.Col] = &contracts.Piece{
			Type:           invisible.Piece.Type,
			Color:          invisible.Piece.Color,
			Shielded:       invisible.Piece.Shielded,
			ShieldTurn:     invisible.Piece.ShieldTurn,
			Frozen:         invisible.Piece.Frozen,
			Borrowed:       invisible.Piece.Borrowed,
			ParasiteTarget: invisible.Piece.ParasiteTarget,
			Bomb:           invisible.Piece.Bomb,
			Invisible:      invisible.Piece.Invisible,
			InvisibleTurn:  invisible.Piece.InvisibleTurn,
			InvisibleOver:  invisible.Piece.InvisibleOver,
			FusedWith:      invisible.Piece.FusedWith,
		}
		state.Board = nextBoard
		replaceLastHistorySnapshot(state)
		state.InvisiblePiece = nil
	} else {
		state.InvisiblePiece = &contracts.InvisiblePieceState{
			Row:        intent.To.Row,
			Col:        intent.To.Col,
			Piece:      invisible.Piece,
			OwnerColor: invisible.OwnerColor,
			RoundsLeft: invisible.RoundsLeft,
		}
	}
	updateBombTracker(state, *intent.From, *intent.To)

	notation := fmt.Sprintf("%s→%s", squareName(*intent.From), squareName(*intent.To))
	state.Moved = append(state.Moved, keyForSquare(*intent.From))
	state.LastMove = &contracts.LastMove{From: *intent.From, To: *intent.To}
	state.MoveHistory = append(state.MoveHistory, notation)
	state.DrawOfferedBy = ""
	captureOccurred := isCapture && (givesCheck || isMove2)
	state.HalfMoveClock = nextHalfMoveClock(state.HalfMoveClock, invisible.Piece.Type, captureOccurred)
	if state.Turn == "black" {
		state.FullMoveNum++
	}
	justMovedColor := state.Turn
	state.Turn = opposite(state.Turn)
	cleanupTemporaryEffects(state, justMovedColor)
	resolveFogEffects(state, justMovedColor)
	resolveFortressEffects(state, justMovedColor)
	bombExplodedSquares := resolveBombEffects(state)
	blackHoleExplodedSquares := resolveBlackHoleEffects(state, justMovedColor)
	roundDrawWhite, roundDrawBlack := drawRoundCards(state, now)
	startedAt := now.UnixMilli()
	if state.Clock.Increment > 0 {
		if justMovedColor == "white" {
			state.Clock.WhiteMS += state.Clock.Increment
		} else {
			state.Clock.BlackMS += state.Clock.Increment
		}
	}
	state.Clock.RunningFor = state.Turn
	state.Clock.StartedAt = &startedAt
	state.UpdatedAt = now.UTC()
	state.History = append(state.History, capturePositionState(state))

	posKey := positionKey(state.Board, state.Turn, sliceToSet(state.Moved), state.LastMove)
	repCount := 1
	for _, pos := range state.History {
		if positionKey(pos.Board, pos.Turn, sliceToSet(pos.Moved), pos.LastMove) == posKey {
			repCount++
		}
	}
	if repCount >= 3 {
		markMatchFinished(state, "draw", "threefold_repetition", now)
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "move_applied", now, intent.PlayerID, map[string]any{"notation": notation, "nextTurn": state.Turn}),
			makeEvent(state.MatchID, "match_finished", now, intent.PlayerID, map[string]any{"result": "threefold_repetition", "winner": "draw"}),
		}, nil
	}
	if state.HalfMoveClock >= 100 {
		markMatchFinished(state, "draw", "fifty_move_rule", now)
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "move_applied", now, intent.PlayerID, map[string]any{"notation": notation, "nextTurn": state.Turn}),
			makeEvent(state.MatchID, "match_finished", now, intent.PlayerID, map[string]any{"result": "fifty_move_rule", "winner": "draw"}),
		}, nil
	}
	if insufficientMaterial(state.Board) {
		markMatchFinished(state, "draw", "insufficient_material", now)
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "move_applied", now, intent.PlayerID, map[string]any{"notation": notation, "nextTurn": state.Turn}),
			makeEvent(state.MatchID, "match_finished", now, intent.PlayerID, map[string]any{"result": "insufficient_material", "winner": "draw"}),
		}, nil
	}

	payload := map[string]any{
		"from":     intent.From,
		"to":       intent.To,
		"notation": notation,
		"nextTurn": state.Turn,
	}
	if givesCheck {
		payload["materialized"] = "check"
	} else if isMove2 && isCapture {
		payload["materialized"] = "capture"
	}
	if len(bombExplodedSquares) > 0 {
		payload["bombExplodedSquares"] = bombExplodedSquares
	}
	if len(blackHoleExplodedSquares) > 0 {
		payload["blackHoleExplodedSquares"] = blackHoleExplodedSquares
	}
	if len(roundDrawWhite) > 0 {
		payload["roundDrawWhite"] = roundDrawWhite
	}
	if len(roundDrawBlack) > 0 {
		payload["roundDrawBlack"] = roundDrawBlack
	}

	return []contracts.ResolvedEvent{
		makeEvent(state.MatchID, "move_applied", now, intent.PlayerID, payload),
		makeEvent(state.MatchID, "clock_updated", now, intent.PlayerID, map[string]any{
			"runningFor": state.Turn,
		}),
	}, nil
}

func applyChat(state *contracts.MatchState, intent contracts.PlayerIntent, now time.Time) ([]contracts.ResolvedEvent, error) {
	text := strings.TrimSpace(intent.Text)
	if text == "" {
		return nil, errors.New("chat text is empty")
	}
	if len(text) > 500 {
		return nil, errors.New("chat message too long (max 500 characters)")
	}

	sender, err := requireIntentColor(state, intent.PlayerID, intent.PlayerSecret)
	if err != nil {
		return nil, err
	}

	for i := len(state.ChatMessages) - 1; i >= 0; i-- {
		msg := state.ChatMessages[i]
		if msg.Sender == sender && now.UTC().Sub(msg.SentAt) < time.Second {
			return nil, errors.New("chat rate limited (one message per second)")
		}
		if msg.Sender != sender && now.UTC().Sub(msg.SentAt) < time.Second {
			break
		}
	}

	state.ChatMessages = append(state.ChatMessages, contracts.ChatMessage{
		Sender: sender,
		Text:   text,
		SentAt: now.UTC(),
	})
	state.UpdatedAt = now.UTC()

	return []contracts.ResolvedEvent{
		makeEvent(state.MatchID, "chat_sent", now, intent.PlayerID, map[string]any{
			"sender": sender,
			"text":   text,
		}),
	}, nil
}

func applyOfferDraw(state *contracts.MatchState, intent contracts.PlayerIntent, now time.Time) ([]contracts.ResolvedEvent, error) {
	if err := ensureActive(state); err != nil {
		return nil, err
	}

	offeredBy, err := requireIntentColor(state, intent.PlayerID, intent.PlayerSecret)
	if err != nil {
		return nil, err
	}

	minInterval := 15 * time.Second
	drawOfferTime := state.DrawOfferTime
	if drawOfferTime != nil && now.UTC().Sub(*drawOfferTime) < minInterval {
		return nil, errors.New("draw offer rate limited")
	}

	state.DrawOfferedBy = offeredBy
	nowUTC := now.UTC()
	state.DrawOfferTime = &nowUTC
	state.UpdatedAt = nowUTC

	return []contracts.ResolvedEvent{
		makeEvent(state.MatchID, "draw_offered", now, intent.PlayerID, map[string]any{
			"offeredBy": offeredBy,
		}),
	}, nil
}

func applyRespondDraw(state *contracts.MatchState, intent contracts.PlayerIntent, now time.Time) ([]contracts.ResolvedEvent, error) {
	if err := ensureActive(state); err != nil {
		return nil, err
	}
	if state.DrawOfferedBy == "" {
		return nil, errors.New("no draw offer to respond to")
	}
	if intent.Accept == nil {
		return nil, errors.New("respond_draw requires accept")
	}
	responder, err := requireIntentColor(state, intent.PlayerID, intent.PlayerSecret)
	if err != nil {
		return nil, err
	}
	if responder == state.DrawOfferedBy {
		return nil, errors.New("draw offer cannot be resolved by the offering side")
	}

	if *intent.Accept {
		markMatchFinished(state, "draw", "draw_agreement", now)
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "draw_resolved", now, intent.PlayerID, map[string]any{"accept": true}),
			makeEvent(state.MatchID, "match_finished", now, intent.PlayerID, map[string]any{"result": "draw_agreement", "winner": "draw"}),
		}, nil
	}

	state.DrawOfferedBy = ""
	state.UpdatedAt = now.UTC()
	return []contracts.ResolvedEvent{
		makeEvent(state.MatchID, "draw_resolved", now, intent.PlayerID, map[string]any{"accept": false}),
	}, nil
}

func applyAbort(state *contracts.MatchState, intent contracts.PlayerIntent, now time.Time) ([]contracts.ResolvedEvent, error) {
	if err := ensureActive(state); err != nil {
		return nil, err
	}
	if _, err := requireIntentColor(state, intent.PlayerID, intent.PlayerSecret); err != nil {
		return nil, err
	}
	if len(state.MoveHistory) > 1 {
		return nil, errors.New("abort is only allowed before black completes the first reply")
	}

	markMatchFinished(state, "aborted", "abort", now)

	return []contracts.ResolvedEvent{
		makeEvent(state.MatchID, "match_finished", now, intent.PlayerID, map[string]any{
			"result": "abort",
			"winner": "aborted",
		}),
	}, nil
}

func applyResign(state *contracts.MatchState, intent contracts.PlayerIntent, now time.Time) ([]contracts.ResolvedEvent, error) {
	if err := ensureActive(state); err != nil {
		return nil, err
	}

	resigningColor, err := requireIntentColor(state, intent.PlayerID, intent.PlayerSecret)
	if err != nil {
		return nil, err
	}
	winner := opposite(resigningColor)
	markMatchFinished(state, winner, "resign", now)

	return []contracts.ResolvedEvent{
		makeEvent(state.MatchID, "match_finished", now, intent.PlayerID, map[string]any{
			"result": "resign",
			"winner": winner,
		}),
	}, nil
}

func ensureActive(state *contracts.MatchState) error {
	if state.Status != "active" {
		return errors.New("match is not active")
	}
	return nil
}

func chooseOpenSeat(state *contracts.MatchState, preferred string) string {
	if state == nil {
		return ""
	}
	isWhiteOpen := strings.TrimSpace(state.WhiteGuestID) == ""
	isBlackOpen := strings.TrimSpace(state.BlackGuestID) == ""
	switch preferred {
	case "white":
		if isWhiteOpen {
			return "white"
		}
		return ""
	case "black":
		if isBlackOpen {
			return "black"
		}
		return ""
	}
	if isWhiteOpen {
		return "white"
	}
	if isBlackOpen {
		return "black"
	}
	return ""
}

func requireIntentColor(state *contracts.MatchState, playerID, playerSecret string) (string, error) {
	value := strings.TrimSpace(playerID)
	resolvedColor := ""
	hasGuestIDs := false
	if state != nil {
		switch {
		case state.WhiteGuestID != "" && strings.EqualFold(value, strings.TrimSpace(state.WhiteGuestID)):
			resolvedColor = "white"
		case state.BlackGuestID != "" && strings.EqualFold(value, strings.TrimSpace(state.BlackGuestID)):
			resolvedColor = "black"
		}
		hasGuestIDs = state.WhiteGuestID != "" || state.BlackGuestID != ""
	}
	if resolvedColor != "" && state != nil {
		requiredSecret := ""
		if resolvedColor == "white" {
			requiredSecret = strings.TrimSpace(state.WhitePlayerSecret)
		} else if resolvedColor == "black" {
			requiredSecret = strings.TrimSpace(state.BlackPlayerSecret)
		}
		if requiredSecret == "" {
			return "", errors.New("match has no player secret configured")
		}
		if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(playerSecret)), []byte(requiredSecret)) != 1 {
			return "", errors.New("unauthorized player secret")
		}
		return resolvedColor, nil
	}
	if state != nil && !hasGuestIDs {
		lowerValue := strings.ToLower(value)
		switch {
		case strings.Contains(lowerValue, "white"):
			resolvedColor = "white"
		case strings.Contains(lowerValue, "black"):
			resolvedColor = "black"
		}
		if resolvedColor != "" {
			requiredSecret := ""
			if resolvedColor == "white" {
				requiredSecret = strings.TrimSpace(state.WhitePlayerSecret)
			} else if resolvedColor == "black" {
				requiredSecret = strings.TrimSpace(state.BlackPlayerSecret)
			}
			if requiredSecret == "" {
				return "", errors.New("match has no player secret configured")
			}
			if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(playerSecret)), []byte(requiredSecret)) != 1 {
				return "", errors.New("unauthorized player secret")
			}
			return resolvedColor, nil
		}
	}
	if state != nil {
		if state.WhitePlayerSecret != "" && subtle.ConstantTimeCompare([]byte(strings.TrimSpace(playerSecret)), []byte(strings.TrimSpace(state.WhitePlayerSecret))) == 1 {
			return "white", nil
		}
		if state.BlackPlayerSecret != "" && subtle.ConstantTimeCompare([]byte(strings.TrimSpace(playerSecret)), []byte(strings.TrimSpace(state.BlackPlayerSecret))) == 1 {
			return "black", nil
		}
	}
	return "", errors.New("unrecognized player id")
}

func opposite(color string) string {
	if color == "white" {
		return "black"
	}
	return "white"
}

func chooseSeed(_ int64, fallback int64) int64 {
	return fallback
}

func makeEvent(matchID, eventType string, now time.Time, actorID string, payload map[string]any) contracts.ResolvedEvent {
	return contracts.ResolvedEvent{
		ID:      fmt.Sprintf("%s_%s_%d", matchID, eventType, now.UnixMilli()),
		MatchID: matchID,
		Type:    eventType,
		ActorID: actorID,
		At:      now.UTC(),
		Payload: payload,
	}
}

func cloneState(state *contracts.MatchState) contracts.MatchState {
	clone := *state
	clone.Board = cloneBoard(state.Board)
	clone.Moved = append([]string{}, state.Moved...)
	clone.MoveHistory = append([]string{}, state.MoveHistory...)
	clone.ChatMessages = append([]contracts.ChatMessage{}, state.ChatMessages...)
	clone.WhiteHand = append([]contracts.GameCard{}, state.WhiteHand...)
	clone.BlackHand = append([]contracts.GameCard{}, state.BlackHand...)
	clone.UndoAgainst = state.UndoAgainst
	clone.LavaSquares = append([]contracts.LavaSquare{}, state.LavaSquares...)
	clone.BombPieces = append([]contracts.BombPiece{}, state.BombPieces...)
	clone.BlackHoles = append([]contracts.BlackHoleZone{}, state.BlackHoles...)
	clone.FogZones = append([]contracts.FogZone{}, state.FogZones...)
	clone.FortressZones = append([]contracts.FortressZone{}, state.FortressZones...)
	clone.History = append([]contracts.PositionState{}, state.History...)
	clone.SeenClientMoveIDs = append([]string{}, state.SeenClientMoveIDs...)
	clone.RadarRevealFor = state.RadarRevealFor
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

func capturePositionState(state *contracts.MatchState) contracts.PositionState {
	position := contracts.PositionState{
		Board:          cloneBoard(state.Board),
		LavaSquares:    append([]contracts.LavaSquare{}, state.LavaSquares...),
		BombPieces:     append([]contracts.BombPiece{}, state.BombPieces...),
		BlackHoles:     append([]contracts.BlackHoleZone{}, state.BlackHoles...),
		FogZones:       append([]contracts.FogZone{}, state.FogZones...),
		FortressZones:  append([]contracts.FortressZone{}, state.FortressZones...),
		RadarRevealFor: state.RadarRevealFor,
		Turn:           state.Turn,
		Moved:          append([]string{}, state.Moved...),
		HalfMoveClock:  state.HalfMoveClock,
		FullMoveNum:    state.FullMoveNum,
		MoveHistory:    append([]string{}, state.MoveHistory...),
		WhiteHand:      append([]contracts.GameCard{}, state.WhiteHand...),
		BlackHand:      append([]contracts.GameCard{}, state.BlackHand...),
		UndoAgainst:    state.UndoAgainst,
		DrawOfferedBy:  state.DrawOfferedBy,
	}
	if state.InvisiblePiece != nil {
		invisible := *state.InvisiblePiece
		position.InvisiblePiece = &invisible
	}
	if state.CheaterState != nil {
		cheater := *state.CheaterState
		position.CheaterState = &cheater
	}
	if state.LastMove != nil {
		last := *state.LastMove
		position.LastMove = &last
	}
	if state.PendingCard != nil {
		pending := *state.PendingCard
		if pending.Target != nil {
			t := *pending.Target
			pending.Target = &t
		}
		position.PendingCard = &pending
	}
	if state.DoubleMove != nil {
		dm := *state.DoubleMove
		if state.DoubleMove.TrackedSq != nil {
			tracked := *state.DoubleMove.TrackedSq
			dm.TrackedSq = &tracked
		}
		position.DoubleMove = &dm
	}
	return position
}

func restorePositionState(state *contracts.MatchState, position contracts.PositionState) {
	state.Board = cloneBoard(position.Board)
	replaceLastHistorySnapshot(state)
	state.LavaSquares = append([]contracts.LavaSquare{}, position.LavaSquares...)
	state.BombPieces = append([]contracts.BombPiece{}, position.BombPieces...)
	state.BlackHoles = append([]contracts.BlackHoleZone{}, position.BlackHoles...)
	state.FogZones = append([]contracts.FogZone{}, position.FogZones...)
	state.FortressZones = append([]contracts.FortressZone{}, position.FortressZones...)
	state.RadarRevealFor = position.RadarRevealFor
	state.Moved = append([]string{}, position.Moved...)
	state.HalfMoveClock = position.HalfMoveClock
	state.FullMoveNum = position.FullMoveNum
	state.MoveHistory = append([]string{}, position.MoveHistory...)
	state.WhiteHand = append([]contracts.GameCard{}, position.WhiteHand...)
	state.BlackHand = append([]contracts.GameCard{}, position.BlackHand...)
	state.UndoAgainst = position.UndoAgainst
	state.DrawOfferedBy = position.DrawOfferedBy
	if position.InvisiblePiece != nil {
		invisible := *position.InvisiblePiece
		state.InvisiblePiece = &invisible
	} else {
		state.InvisiblePiece = nil
	}
	if position.CheaterState != nil {
		cheater := *position.CheaterState
		state.CheaterState = &cheater
	} else {
		state.CheaterState = nil
	}
	if position.LastMove != nil {
		last := *position.LastMove
		state.LastMove = &last
	} else {
		state.LastMove = nil
	}
	if position.PendingCard != nil {
		pending := *position.PendingCard
		if pending.Target != nil {
			t := *pending.Target
			pending.Target = &t
		}
		state.PendingCard = &pending
	} else {
		state.PendingCard = nil
	}
	if position.DoubleMove != nil {
		dm := *position.DoubleMove
		if position.DoubleMove.TrackedSq != nil {
			tracked := *position.DoubleMove.TrackedSq
			dm.TrackedSq = &tracked
		}
		state.DoubleMove = &dm
	} else {
		state.DoubleMove = nil
	}
}

func replaceLastHistorySnapshot(state *contracts.MatchState) {
	snapshot := capturePositionState(state)
	if len(state.History) == 0 {
		state.History = []contracts.PositionState{snapshot}
		return
	}
	state.History[len(state.History)-1] = snapshot
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

func filterStateForColor(state contracts.MatchState, color string) contracts.MatchState {
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

func syncClockForMutation(state *contracts.MatchState, now time.Time) []contracts.ResolvedEvent {
	if state.Status != "active" || state.Clock.RunningFor == "" || state.Clock.StartedAt == nil {
		return nil
	}

	elapsed := now.UnixMilli() - *state.Clock.StartedAt
	if elapsed <= 0 {
		return nil
	}

	state.UpdatedAt = now.UTC()
	startedAt := now.UnixMilli()
	switch state.Clock.RunningFor {
	case "white":
		state.Clock.WhiteMS -= elapsed
		if state.Clock.WhiteMS <= 0 {
			state.Clock.WhiteMS = 0
			winner := "black"
			reason := "timeout"
			if insufficientMaterial(state.Board) {
				winner = "draw"
				reason = "timeout_vs_insufficient_material"
			}
			markMatchFinished(state, winner, reason, now)
			return timeoutEvents(state.MatchID, now, "white", winner)
		}
	case "black":
		state.Clock.BlackMS -= elapsed
		if state.Clock.BlackMS <= 0 {
			state.Clock.BlackMS = 0
			winner := "white"
			reason := "timeout"
			if insufficientMaterial(state.Board) {
				winner = "draw"
				reason = "timeout_vs_insufficient_material"
			}
			markMatchFinished(state, winner, reason, now)
			return timeoutEvents(state.MatchID, now, "black", winner)
		}
	}

	state.Clock.StartedAt = &startedAt
	return nil
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

func timeoutEvents(matchID string, now time.Time, flaggedColor string, winner string) []contracts.ResolvedEvent {
	return []contracts.ResolvedEvent{
		makeEvent(matchID, "clock_updated", now, "", map[string]any{
			"runningFor": "",
			"flagged":    flaggedColor,
		}),
		makeEvent(matchID, "match_finished", now, "", map[string]any{
			"result": "timeout",
			"winner": winner,
		}),
	}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func cleanupTemporaryEffects(state *contracts.MatchState, justMovedColor string) {
	for r := 0; r < len(state.Board); r++ {
		for c := 0; c < len(state.Board[r]); c++ {
			piece := state.Board[r][c]
			if piece == nil {
				continue
			}
			if piece.Frozen && piece.Color == justMovedColor {
				piece.Frozen = false
			}
			if piece.Shielded && piece.ShieldTurn != nil && state.FullMoveNum >= *piece.ShieldTurn {
				piece.Shielded = false
				piece.ShieldTurn = nil
			}
			if piece.Borrowed && piece.Color == justMovedColor {
				piece.Color = opposite(justMovedColor)
				piece.Borrowed = false
			}
		}
	}
	if state.InvisiblePiece != nil {
		if state.InvisiblePiece.OwnerColor == justMovedColor {
			state.InvisiblePiece.RoundsLeft--
		} else if state.InvisiblePiece.RoundsLeft <= 0 {
			state.InvisiblePiece = nil
		}
	}
	if state.RadarRevealFor != "" && state.Turn != state.RadarRevealFor {
		state.RadarRevealFor = ""
	}
	if state.CheaterState != nil && state.Turn != state.CheaterState.OwnerColor {
		state.CheaterState.TurnsLeft--
		if state.CheaterState.TurnsLeft <= 0 {
			state.CheaterState = nil
		}
	}
	if state.UndoAgainst == justMovedColor {
		state.UndoAgainst = ""
	}
}

func newMatchPresenceState(state *contracts.MatchState, now time.Time) *matchPresenceState {
	presence := &matchPresenceState{}
	if strings.TrimSpace(state.WhiteGuestID) != "" {
		presence.WhiteLastSeenAt = now
		presence.WhiteConnected = true
	}
	if strings.TrimSpace(state.BlackGuestID) != "" {
		presence.BlackLastSeenAt = now
		presence.BlackConnected = true
	}
	if state.ModeID == contracts.MatchModeComputer {
		presence.BlackLastSeenAt = now
		presence.BlackConnected = true
	}
	return presence
}

func newRecoveredMatchPresenceState(state *contracts.MatchState) *matchPresenceState {
	presence := &matchPresenceState{}
	lastSeen := recoveredPresenceSeedTime(state)
	if strings.TrimSpace(state.WhiteGuestID) != "" {
		presence.WhiteLastSeenAt = lastSeen
		presence.WhiteConnected = false
	}
	if strings.TrimSpace(state.BlackGuestID) != "" {
		presence.BlackLastSeenAt = lastSeen
		presence.BlackConnected = false
	}
	if state.ModeID == contracts.MatchModeComputer {
		presence.BlackLastSeenAt = lastSeen
		presence.BlackConnected = true
	}
	return presence
}

func recoveredPresenceSeedTime(state *contracts.MatchState) time.Time {
	if state == nil {
		return time.Time{}
	}
	if !state.UpdatedAt.IsZero() {
		return state.UpdatedAt.UTC()
	}
	if !state.CreatedAt.IsZero() {
		return state.CreatedAt.UTC()
	}
	return time.Time{}
}

func (s *Service) ensurePresenceStateLocked(matchID string, state *contracts.MatchState, now time.Time) *matchPresenceState {
	if presence, ok := s.presence[matchID]; ok && presence != nil {
		return presence
	}
	presence := newMatchPresenceState(state, now)
	s.presence[matchID] = presence
	return presence
}

func presenceHeartbeat(presence *matchPresenceState, color string, now time.Time) bool {
	if presence == nil {
		return false
	}

	changed := false
	switch color {
	case "white":
		if !presence.WhiteConnected || !presence.WhiteLastSeenAt.Equal(now) {
			changed = true
		}
		presence.WhiteConnected = true
		presence.WhiteLastSeenAt = now
	case "black":
		if !presence.BlackConnected || !presence.BlackLastSeenAt.Equal(now) {
			changed = true
		}
		presence.BlackConnected = true
		presence.BlackLastSeenAt = now
	}

	if presence.DisconnectGraceFor == color || presence.DisconnectGraceFor == disconnectGraceBoth {
		presence.DisconnectGraceFor = ""
		presence.DisconnectGraceDeadline = nil
		changed = true
	}

	return changed
}

func evaluatePresenceRuntime(state *contracts.MatchState, presence *matchPresenceState, now time.Time) []contracts.ResolvedEvent {
	if state == nil || presence == nil {
		return nil
	}
	if state.Status == "finished" {
		presence.DisconnectGraceFor = ""
		presence.DisconnectGraceDeadline = nil
		return nil
	}

	whiteOccupied := strings.TrimSpace(state.WhiteGuestID) != ""
	blackOccupied := strings.TrimSpace(state.BlackGuestID) != "" || state.ModeID == contracts.MatchModeComputer

	if whiteOccupied {
		presence.WhiteConnected = now.Sub(presence.WhiteLastSeenAt) <= presenceHeartbeatTimeout
	} else {
		presence.WhiteConnected = false
	}
	if blackOccupied {
		if state.ModeID == contracts.MatchModeComputer {
			presence.BlackConnected = true
			presence.BlackLastSeenAt = now
		} else {
			presence.BlackConnected = now.Sub(presence.BlackLastSeenAt) <= presenceHeartbeatTimeout
		}
	} else {
		presence.BlackConnected = false
	}

	if state.Status != "active" || !whiteOccupied || !blackOccupied {
		presence.DisconnectGraceFor = ""
		presence.DisconnectGraceDeadline = nil
		return nil
	}

	disconnectedColor := ""
	switch {
	case !presence.WhiteConnected && presence.BlackConnected:
		disconnectedColor = "white"
	case presence.WhiteConnected && !presence.BlackConnected:
		disconnectedColor = "black"
	case !presence.WhiteConnected && !presence.BlackConnected:
		disconnectedColor = disconnectGraceBoth
	default:
		disconnectedColor = ""
	}

	if disconnectedColor == "" {
		presence.DisconnectGraceFor = ""
		presence.DisconnectGraceDeadline = nil
		return nil
	}

	deadline := presenceDisconnectDeadline(presence, disconnectedColor)
	if deadline.IsZero() {
		deadline = now.Add(disconnectGracePeriod)
	}
	if presence.DisconnectGraceFor != disconnectedColor || presence.DisconnectGraceDeadline == nil || !presence.DisconnectGraceDeadline.Equal(deadline) {
		presence.DisconnectGraceFor = disconnectedColor
		presence.DisconnectGraceDeadline = &deadline
		if now.Before(deadline) {
			return nil
		}
	}

	if now.Before(*presence.DisconnectGraceDeadline) {
		return nil
	}

	presence.DisconnectGraceFor = ""
	presence.DisconnectGraceDeadline = nil
	if disconnectedColor == disconnectGraceBoth {
		winner := "draw"
		finishReason := "abandon"
		result := "abandon"
		if len(state.MoveHistory) == 0 {
			winner = "aborted"
			finishReason = "abort"
			result = "abort"
		}
		markMatchFinished(state, winner, finishReason, now)
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "match_finished", now, "", map[string]any{
				"result":       result,
				"winner":       state.Winner,
				"disconnected": disconnectedColor,
			}),
		}
	}
	markMatchFinished(state, opposite(disconnectedColor), "abandon", now)
	return []contracts.ResolvedEvent{
		makeEvent(state.MatchID, "match_finished", now, "", map[string]any{
			"result":       "abandon",
			"winner":       state.Winner,
			"disconnected": disconnectedColor,
		}),
	}
}

func presenceDisconnectDeadline(presence *matchPresenceState, disconnectedColor string) time.Time {
	if presence == nil {
		return time.Time{}
	}
	switch disconnectedColor {
	case "white":
		if presence.WhiteLastSeenAt.IsZero() {
			return time.Time{}
		}
		return presence.WhiteLastSeenAt.Add(presenceHeartbeatTimeout + disconnectGracePeriod)
	case "black":
		if presence.BlackLastSeenAt.IsZero() {
			return time.Time{}
		}
		return presence.BlackLastSeenAt.Add(presenceHeartbeatTimeout + disconnectGracePeriod)
	case disconnectGraceBoth:
		lastSeen := presence.WhiteLastSeenAt
		if presence.BlackLastSeenAt.After(lastSeen) {
			lastSeen = presence.BlackLastSeenAt
		}
		if lastSeen.IsZero() {
			return time.Time{}
		}
		return lastSeen.Add(presenceHeartbeatTimeout + disconnectGraceBothPeriod)
	default:
		return time.Time{}
	}
}
