package match

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
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

	c := newMatchContainer(state, []contracts.ResolvedEvent{startEvent}, newMatchPresenceState(state, now))

	if string(req.ModeID) == "computer" {
		diff := engine.ParseDifficulty(req.Difficulty)
		c.computer = engine.NewComputerOpponent(diff, "black")
		s.Log.Info("match:create: computer opponent initialized", "matchID", matchID, "difficulty", diff, "color", "black")
	}

	s.matches.Store(matchID, c)
	c.mu.Lock()

	broadcastSnap := buildSnapshotWithPresence(c.state, c.presence, len(c.events), []contracts.ResolvedEvent{startEvent}, now)
	persistSnap := buildSnapshotWithPresence(c.state, c.presence, len(c.events), c.events, now)
	c.mu.Unlock()

	s.persistSnapshot(persistSnap)
	s.saveToRedis(persistSnap, c.presence)
	s.Log.Info("match:create: ok", "matchID", matchID, "status", broadcastSnap.Match.Status, "turn", broadcastSnap.Match.Turn, "whiteFingerprint", redactPlayerSecret(broadcastSnap.Match.WhitePlayerSecret), "blackFingerprint", redactPlayerSecret(broadcastSnap.Match.BlackPlayerSecret), "computers", c.computer != nil)

	return broadcastSnap
}

func (s *Service) JoinMatchSeat(matchID string, req contracts.JoinMatchSeatRequest, now time.Time) (contracts.JoinMatchSeatResponse, error) {
	s.mu.Lock()
	c, ok := s.ensureMatchLoadedLocked(matchID)
	s.mu.Unlock()
	if !ok {
		return contracts.JoinMatchSeatResponse{}, ErrMatchNotFound
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	state := c.state
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

	presence := s.ensurePresenceStateLocked(c, now)

	if len(events) > 0 {
		c.events = append(c.events, events...)
		snapshot := buildSnapshotWithPresence(state, presence, len(c.events), events, now)
		persistSnap := buildSnapshotWithPresence(state, presence, len(c.events), c.events, now)
		s.persistSnapshot(persistSnap)
		s.saveToRedis(persistSnap, presence)
		s.broadcastLocked(c, snapshot)
		return contracts.JoinMatchSeatResponse{
			Match:              snapshot,
			SeatColor:          seatColor,
			Joined:             joined,
			WaitingForOpponent: state.Status == "waiting",
		}, nil
	}

	fullSnapshot := buildSnapshotWithPresence(state, presence, len(c.events), nil, now)
	if updated {
		s.persistSnapshot(fullSnapshot)
		s.saveToRedis(fullSnapshot, presence)
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
	c, ok := s.ensureMatchLoadedLocked(intent.MatchID)
	s.mu.Unlock()
	if !ok {
		return contracts.MatchSnapshotResponse{}, ErrMatchNotFound
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	state := c.state

	if intent.ClientMoveID != "" {
		for _, id := range state.SeenClientMoveIDs {
			if id == intent.ClientMoveID {
				presence := s.ensurePresenceStateLocked(c, now)
				return buildSnapshotWithPresence(state, presence, len(c.events), nil, now), nil
			}
		}
	}

	if intent.ExpectedSeqNum > 0 {
		currentSeq := c.seqNum
		if currentSeq > 0 && intent.ExpectedSeqNum < currentSeq {
			return contracts.MatchSnapshotResponse{}, ErrStaleClientState
		}
	}

	presence := s.ensurePresenceStateLocked(c, now)

	actorColor, _ := requireIntentColor(state, intent.PlayerID, intent.PlayerSecret)
	if err := rateLimitIntent(presence, actorColor, now); err != nil {
		return contracts.MatchSnapshotResponse{}, err
	}

	timeoutEvents := syncClockForMutation(state, now)
	if len(timeoutEvents) > 0 {
		if events, err := applyIntent(state, intent, now); err == nil {
			trackIntentTime(presence, actorColor, now)
			events = append(events, timeoutEvents...)
			c.events = append(c.events, events...)
			snapshot := buildSnapshotWithPresence(state, presence, len(c.events), events, now)
			persistSnap := buildSnapshot(state, len(c.events), c.events, now)
			s.persistSnapshot(persistSnap)
			s.saveToRedis(persistSnap, presence)
			s.broadcastLocked(c, snapshot)
			return snapshot, nil
		}
		c.events = append(c.events, timeoutEvents...)
		snapshot := buildSnapshotWithPresence(state, presence, len(c.events), timeoutEvents, now)
		persistSnap := buildSnapshot(state, len(c.events), c.events, now)
		s.persistSnapshot(persistSnap)
		s.saveToRedis(persistSnap, presence)
		s.broadcastLocked(c, snapshot)
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

	c.events = append(c.events, events...)
	snapshot := buildSnapshotWithPresence(state, presence, len(c.events), events, now)
	persistSnap := buildSnapshot(state, len(c.events), c.events, now)
	s.persistSnapshot(persistSnap)
	s.saveToRedis(persistSnap, presence)
	s.broadcastLocked(c, snapshot)

	s.autoPlayComputer(c, now)

	return snapshot, nil
}

func (s *Service) autoPlayComputer(c *matchContainer, now time.Time) {
	if c.computer == nil || c.state.Status != "active" || c.state.Turn != "black" {
		return
	}
	select {
	case s.computerCh <- computerMoveTask{c: c, now: now}:
	default:
		s.Log.Warn("computer move worker pool full, skipping computer move", "matchID", c.state.MatchID)
	}
}

func (s *Service) autoPlayComputerDepthLimited(c *matchContainer, now time.Time, depth int) {
	if depth > 5 {
		s.Log.Info("match:autoPlay: max recursion depth reached", "matchID", c.state.MatchID)
		return
	}
	if c.computer == nil || c.state.Status != "active" || c.state.Turn != "black" {
		return
	}

	computerIntent := c.computer.MakeMove(c.state)
	if computerIntent == nil {
		s.Log.Info("match:autoPlay: computer returned NIL intent", "matchID", c.state.MatchID, "turn", c.state.Turn, "status", c.state.Status)
		return
	}

	savedClock := c.state.Clock
	savedStatus := c.state.Status
	savedWinner := c.state.Winner
	savedFinishReason := c.state.FinishReason

	events, err := applyIntent(c.state, *computerIntent, now)
	if err != nil {
		c.state.Clock = savedClock
		c.state.Status = savedStatus
		c.state.Winner = savedWinner
		c.state.FinishReason = savedFinishReason
		return
	}

	// If the card requires target selection, have the computer pick a target
	if c.state.PendingCard != nil && c.computer != nil {
		targetIntent := c.computer.HandleSelectTarget(c.state)
		if targetIntent != nil {
			targetEvents, targetErr := applyIntent(c.state, *targetIntent, now)
			if targetErr == nil {
				events = append(events, targetEvents...)
			}
		}
	}

	if shouldEvaluateAutomaticMatchFinish(c.state, *computerIntent) {
		events = finalizeAutomaticMatchFinish(c.state, events, now, "computer")
	}

	timeoutEvents := syncClockForMutation(c.state, now)
	events = append(events, timeoutEvents...)

	c.events = append(c.events, events...)
	snapshot := buildSnapshotWithPresence(c.state, c.presence, len(c.events), events, now)
	persistSnap := buildSnapshot(c.state, len(c.events), c.events, now)
	s.persistSnapshot(persistSnap)
	s.saveToRedis(persistSnap, c.presence)
	s.broadcastLocked(c, snapshot)

	if c.state.Turn == "black" && c.state.Status == "active" {
		s.autoPlayComputerDepthLimited(c, now, depth+1)
	}
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

func chooseSeed(_ int64, _ int64) int64 {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err == nil {
		return int64(binary.LittleEndian.Uint64(buf[:]))
	}
	return time.Now().UnixNano()
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

func (s *Service) ensurePresenceStateLocked(c *matchContainer, now time.Time) *matchPresenceState {
	if c.presence != nil {
		return c.presence
	}
	c.presence = newMatchPresenceState(c.state, now)
	return c.presence
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
