package match

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/chess404/realtime/internal/contracts"
)

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

	legal := legalMovesWithFusion(state.Board, *intent.From, state.LastMove, sliceToSet(state.Moved), state.FortressZones)
	if !containsSquare(legal, *intent.To) {
		return nil, errors.New("illegal move")
	}
	if fortressEntryBlocked(state.FortressZones, piece.Color, *intent.To) {
		return nil, errors.New("destination square is protected by an enemy fortress")
	}
	if isSlider(piece.Type) && pathCrossesFortress(state.Board, *intent.From, *intent.To, state.FortressZones, piece.Color) {
		return nil, errors.New("move path crosses an enemy fortress")
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
	if err := resolveParasiteEffects(nextBoard, *intent.From, *intent.To, capturedSquare, captured, state.FortressZones); err != nil {
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
		if oppKing != nil && isAttackedWithFusion(nextBoard, *oppKing, state.Turn, state.FortressZones) {
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
	roundDrawWhite, roundDrawBlack, whiteSkipped, blackSkipped := drawRoundCards(state, now)
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

	posKey := positionKey(state.Board, state.Turn, sliceToSet(state.Moved), state.LastMove, state.WhiteHand, state.BlackHand)
	repCount := 1
	for _, pos := range state.History {
		if positionKey(pos.Board, pos.Turn, sliceToSet(pos.Moved), pos.LastMove, pos.WhiteHand, pos.BlackHand) == posKey {
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

	extraEvents := make([]contracts.ResolvedEvent, 0)
	if len(roundDrawWhite) > 0 {
		extraEvents = append(extraEvents, makeEvent(state.MatchID, "card_drawn", now, "", map[string]any{"owner": "white", "cards": roundDrawWhite}))
	}
	if len(roundDrawBlack) > 0 {
		extraEvents = append(extraEvents, makeEvent(state.MatchID, "card_drawn", now, "", map[string]any{"owner": "black", "cards": roundDrawBlack}))
	}
	if whiteSkipped {
		extraEvents = append(extraEvents, makeEvent(state.MatchID, "card_draw_lost", now, "", map[string]any{"owner": "white", "reason": "hand_full"}))
	}
	if blackSkipped {
		extraEvents = append(extraEvents, makeEvent(state.MatchID, "card_draw_lost", now, "", map[string]any{"owner": "black", "reason": "hand_full"}))
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

	result := []contracts.ResolvedEvent{
		makeEvent(state.MatchID, "move_applied", now, intent.PlayerID, payload),
		makeEvent(state.MatchID, "clock_updated", now, intent.PlayerID, map[string]any{
			"runningFor": state.Turn,
		}),
	}
	result = append(result, extraEvents...)
	return result, nil
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

	legal := legalMoves(ghostBoard, *intent.From, state.LastMove, sliceToSet(state.Moved), state.FortressZones)
	if !containsSquare(legal, *intent.To) {
		return nil, errors.New("illegal move")
	}
	if fortressEntryBlocked(state.FortressZones, invisible.Piece.Color, *intent.To) {
		return nil, errors.New("destination square is protected by an enemy fortress")
	}
	if isSlider(invisible.Piece.Type) && pathCrossesFortress(ghostBoard, *intent.From, *intent.To, state.FortressZones, invisible.Piece.Color) {
		return nil, errors.New("move path crosses an enemy fortress")
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
	givesCheck := oppKing != nil && isAttacked(givesCheckBoard, *oppKing, state.Turn, state.FortressZones)
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
	roundDrawWhite, roundDrawBlack, whiteSkipped, blackSkipped := drawRoundCards(state, now)
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

	posKey := positionKey(state.Board, state.Turn, sliceToSet(state.Moved), state.LastMove, state.WhiteHand, state.BlackHand)
	repCount := 1
	for _, pos := range state.History {
		if positionKey(pos.Board, pos.Turn, sliceToSet(pos.Moved), pos.LastMove, pos.WhiteHand, pos.BlackHand) == posKey {
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

	extraEvents := make([]contracts.ResolvedEvent, 0)
	if len(roundDrawWhite) > 0 {
		extraEvents = append(extraEvents, makeEvent(state.MatchID, "card_drawn", now, "", map[string]any{"owner": "white", "cards": roundDrawWhite}))
	}
	if len(roundDrawBlack) > 0 {
		extraEvents = append(extraEvents, makeEvent(state.MatchID, "card_drawn", now, "", map[string]any{"owner": "black", "cards": roundDrawBlack}))
	}
	if whiteSkipped {
		extraEvents = append(extraEvents, makeEvent(state.MatchID, "card_draw_lost", now, "", map[string]any{"owner": "white", "reason": "hand_full"}))
	}
	if blackSkipped {
		extraEvents = append(extraEvents, makeEvent(state.MatchID, "card_draw_lost", now, "", map[string]any{"owner": "black", "reason": "hand_full"}))
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

	result := []contracts.ResolvedEvent{
		makeEvent(state.MatchID, "move_applied", now, intent.PlayerID, payload),
		makeEvent(state.MatchID, "clock_updated", now, intent.PlayerID, map[string]any{
			"runningFor": state.Turn,
		}),
	}
	result = append(result, extraEvents...)
	return result, nil
}

var htmlTagRegex = regexp.MustCompile(`<[^>]*>`)
var unsafeCharsRegex = regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F]`)

func sanitizeChatText(raw string) string {
	s := htmlTagRegex.ReplaceAllString(raw, "")
	s = unsafeCharsRegex.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func applyChat(state *contracts.MatchState, intent contracts.PlayerIntent, now time.Time) ([]contracts.ResolvedEvent, error) {
	text := sanitizeChatText(intent.Text)
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
	const maxChatMessages = 200
	if len(state.ChatMessages) > maxChatMessages {
		state.ChatMessages = state.ChatMessages[len(state.ChatMessages)-maxChatMessages:]
	}
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

func isSlider(pieceType string) bool {
	return pieceType == "rook" || pieceType == "bishop" || pieceType == "queen"
}

func pathCrossesFortress(board [][]*contracts.Piece, from, to contracts.Square, fortressZones []contracts.FortressZone, moverColor string) bool {
	dr := sign(to.Row - from.Row)
	dc := sign(to.Col - from.Col)
	r, c := from.Row+dr, from.Col+dc
	for r != to.Row || c != to.Col {
		if isInsideEnemyFortress(contracts.Square{Row: r, Col: c}, fortressZones, moverColor) {
			return true
		}
		r += dr
		c += dc
	}
	return false
}
