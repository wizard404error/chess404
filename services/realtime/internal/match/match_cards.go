package match

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/chess404/realtime/internal/contracts"
)

func applyPlayCard(state *contracts.MatchState, intent contracts.PlayerIntent, now time.Time) ([]contracts.ResolvedEvent, error) {
	if err := ensureActive(state); err != nil {
		return nil, err
	}

	owner, err := requireIntentColor(state, intent.PlayerID, intent.PlayerSecret)
	if err != nil {
		return nil, err
	}
	if owner != state.Turn {
		return nil, errors.New("cards can only be played on your turn")
	}
	if state.DoubleMove != nil {
		return nil, errors.New("resolve the active double move before playing another card")
	}
	if state.PendingCard != nil {
		return nil, errors.New("resolve the pending card target first")
	}

	card, found := cardFromHand(state, owner, intent.CardID)
	if !found {
		return nil, errors.New("card not found in hand")
	}
	if state.UndoAgainst == owner {
		removeCardFromHand(state, owner, card.ID)
		state.UndoAgainst = ""
		state.DrawOfferedBy = ""
		state.UpdatedAt = now.UTC()
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "card_played", now, intent.PlayerID, map[string]any{
				"cardId":    card.ID,
				"mechanic":  card.Mechanic,
				"nullified": true,
			}),
			makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
				"effect": "undo_nullified_card",
			}),
		}, nil
	}

	if card.Mechanic == "doublemove_diff" || card.Mechanic == "doublemove_same" {
		moveType := "diff"
		if card.Mechanic == "doublemove_same" {
			moveType = "same"
		}
		removeCardFromHand(state, owner, card.ID)
		state.DoubleMove = &contracts.DoubleMoveState{
			Type:      moveType,
			MovesLeft: 2,
		}
		state.DrawOfferedBy = ""
		state.UpdatedAt = now.UTC()
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "card_played", now, intent.PlayerID, map[string]any{
				"cardId":     card.ID,
				"mechanic":   card.Mechanic,
				"doubleMove": state.DoubleMove,
			}),
		}, nil
	}
	if card.Mechanic == "undo" {
		removeCardFromHand(state, owner, card.ID)
		state.UndoAgainst = opposite(owner)
		state.DrawOfferedBy = ""
		state.UpdatedAt = now.UTC()
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "card_played", now, intent.PlayerID, map[string]any{
				"cardId":      card.ID,
				"mechanic":    card.Mechanic,
				"undoAgainst": state.UndoAgainst,
			}),
		}, nil
	}
	if card.Mechanic == "reverse" {
		if len(state.History) < 2 {
			return nil, errors.New("no move to reverse yet")
		}
		restored := state.History[len(state.History)-2]
		if king := findKing(restored.Board, owner); king != nil && isAttackedWithFusion(restored.Board, *king, opposite(owner), state.FortressZones) {
			return nil, errors.New("cannot reverse because your king would be in check")
		}
		if oppKing := findKing(restored.Board, opposite(owner)); oppKing != nil && isAttackedWithFusion(restored.Board, *oppKing, owner, state.FortressZones) {
			return nil, errors.New("cannot reverse because enemy king would be in check")
		}
		restorePositionState(state, restored)
		removeCardFromHand(state, owner, card.ID)
		state.UpdatedAt = now.UTC()
		events := []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "card_played", now, intent.PlayerID, map[string]any{
				"cardId":   card.ID,
				"mechanic": card.Mechanic,
			}),
			makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
				"effect": "reverse_applied",
			}),
		}
		if winner, reason := evaluateAutomaticMatchFinish(state); reason != "" {
			markMatchFinished(state, winner, reason, now)
			events = append(events, makeEvent(state.MatchID, "match_finished", now, intent.PlayerID, map[string]any{
				"result": reason,
				"winner": winner,
			}))
		}
		return events, nil
	}
	if card.Mechanic == "mirror" {
		removeCardFromHand(state, owner, card.ID)
		mirrored, from, to, err := applyMirrorCard(state, owner)
		if err != nil {
			return nil, err
		}
		// guard: if DoubleMove is active, decrement MovesLeft
		if state.DoubleMove != nil {
			state.DoubleMove.MovesLeft--
		}
		state.DrawOfferedBy = ""
		state.UpdatedAt = now.UTC()

		payload := map[string]any{
			"cardId":   card.ID,
			"mechanic": card.Mechanic,
			"mirrored": mirrored,
		}
		if from != nil {
			payload["from"] = from
		}
		if to != nil {
			payload["to"] = to
		}

		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "card_played", now, intent.PlayerID, payload),
			makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, payload),
		}, nil
	}
	if card.Mechanic == "gambler" {
		removeCardFromHand(state, owner, card.ID)
		payload := resolveGamblerCard(state, owner, now)
		state.DrawOfferedBy = ""
		state.UpdatedAt = now.UTC()
		payload["cardId"] = card.ID
		payload["mechanic"] = card.Mechanic
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "card_played", now, intent.PlayerID, payload),
		}, nil
	}
	if card.Mechanic == "radar" {
		removeCardFromHand(state, owner, card.ID)
		state.RadarRevealFor = owner
		state.DrawOfferedBy = ""
		state.UpdatedAt = now.UTC()
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "card_played", now, intent.PlayerID, map[string]any{
				"cardId":         card.ID,
				"mechanic":       card.Mechanic,
				"radarRevealFor": owner,
			}),
		}, nil
	}
	if card.Mechanic == "cheater" {
		removeCardFromHand(state, owner, card.ID)
		state.CheaterState = &contracts.CheaterState{
			OwnerColor: owner,
			TurnsLeft:  3,
		}
		state.DrawOfferedBy = ""
		state.UpdatedAt = now.UTC()
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "card_played", now, intent.PlayerID, map[string]any{
				"cardId":       card.ID,
				"mechanic":     card.Mechanic,
				"cheaterState": state.CheaterState,
			}),
		}, nil
	}
	if card.Mechanic == "joker" {
		state.PendingCard = &contracts.PendingCardState{
			CardID:     card.ID,
			Mechanic:   card.Mechanic,
			OwnerColor: owner,
			Options:    jokerTransformOptions(),
		}
		state.DrawOfferedBy = ""
		state.UpdatedAt = now.UTC()
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "card_played", now, intent.PlayerID, map[string]any{
				"cardId":   card.ID,
				"mechanic": card.Mechanic,
				"options":  state.PendingCard.Options,
			}),
		}, nil
	}

	state.PendingCard = &contracts.PendingCardState{
		CardID:     card.ID,
		Mechanic:   card.Mechanic,
		OwnerColor: owner,
	}
	state.UpdatedAt = now.UTC()

	return []contracts.ResolvedEvent{
		makeEvent(state.MatchID, "card_played", now, intent.PlayerID, map[string]any{
			"cardId":   card.ID,
			"mechanic": card.Mechanic,
		}),
	}, nil
}

func applySelectTarget(state *contracts.MatchState, intent contracts.PlayerIntent, now time.Time) ([]contracts.ResolvedEvent, error) {
	if err := ensureActive(state); err != nil {
		return nil, err
	}
	if state.PendingCard == nil {
		return nil, errors.New("no pending card target selection")
	}
	pending := state.PendingCard
	owner, err := requireIntentColor(state, intent.PlayerID, intent.PlayerSecret)
	if err != nil {
		return nil, err
	}
	if owner != pending.OwnerColor {
		return nil, errors.New("only the card owner can select the target")
	}

	switch pending.Mechanic {
	case "freeze":
		if intent.Target == nil {
			return nil, errors.New("target selection requires a target square")
		}
		targetPiece := pieceAt(state.Board, *intent.Target)
		if targetPiece == nil {
			return nil, errors.New("target square has no piece")
		}
		if targetPiece.Color == pending.OwnerColor || targetPiece.Type == "king" {
			return nil, errors.New("freeze requires an enemy non-king target")
		}
		targetPiece.Frozen = true
		replaceLastHistorySnapshot(state)
	case "shield":
		if intent.Target == nil {
			return nil, errors.New("target selection requires a target square")
		}
		targetPiece := pieceAt(state.Board, *intent.Target)
		if targetPiece == nil {
			return nil, errors.New("target square has no piece")
		}
		if targetPiece.Color != pending.OwnerColor || targetPiece.Type == "king" {
			return nil, errors.New("shield requires your own non-king target")
		}
		targetPiece.Shielded = true
		shieldTurn := state.FullMoveNum + 1
		targetPiece.ShieldTurn = &shieldTurn
		replaceLastHistorySnapshot(state)
	case "sniper":
		if intent.Target == nil {
			return nil, errors.New("target selection requires a target square")
		}
		targetPiece := pieceAt(state.Board, *intent.Target)
		if targetPiece == nil {
			return nil, errors.New("target square has no piece")
		}
		if targetPiece.Color == pending.OwnerColor || targetPiece.Type == "king" {
			return nil, errors.New("sniper requires an enemy non-king target")
		}
		if err := ensureRemovalDoesNotCreateCheck(state.Board, *intent.Target, pending.OwnerColor, state.FortressZones); err != nil {
			return nil, err
		}
		if targetPiece.Shielded {
			targetPiece.Shielded = false
			targetPiece.ShieldTurn = nil
			state.DrawOfferedBy = ""
			state.UpdatedAt = now.UTC()
			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
					"effect": "shield_blocked_capture",
					"target": intent.Target,
				}),
			}, nil
		}
		state.Board[intent.Target.Row][intent.Target.Col] = nil
		replaceLastHistorySnapshot(state)
	case "badsniper":
		if intent.Target == nil {
			return nil, errors.New("target selection requires a target square")
		}
		targetPiece := pieceAt(state.Board, *intent.Target)
		if targetPiece == nil {
			return nil, errors.New("target square has no piece")
		}
		if targetPiece.Color != pending.OwnerColor || targetPiece.Type == "king" {
			return nil, errors.New("badsniper requires your own non-king target")
		}
		if err := ensureRemovalDoesNotCreateCheck(state.Board, *intent.Target, pending.OwnerColor, state.FortressZones); err != nil {
			return nil, err
		}
		if targetPiece.Shielded {
			targetPiece.Shielded = false
			targetPiece.ShieldTurn = nil
			state.DrawOfferedBy = ""
			state.UpdatedAt = now.UTC()
			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
					"effect": "shield_blocked_capture",
					"target": intent.Target,
				}),
			}, nil
		}
		state.Board[intent.Target.Row][intent.Target.Col] = nil
		replaceLastHistorySnapshot(state)
	case "promote", "demote", "promotehim", "demotehim":
		if pending.Target == nil {
			if intent.Target == nil {
				return nil, errors.New("target selection requires a target square")
			}
			targetPiece := pieceAt(state.Board, *intent.Target)
			if targetPiece == nil {
				return nil, errors.New("target square has no piece")
			}
			if err := validateTransformTarget(targetPiece, pending.OwnerColor, pending.Mechanic, *intent.Target); err != nil {
				return nil, err
			}
			if targetPiece.Frozen && (pending.Mechanic == "demote" || pending.Mechanic == "demotehim" ||
				(pending.Mechanic == "promote" && targetPiece.Color == pending.OwnerColor) ||
				(pending.Mechanic == "promotehim" && targetPiece.Color != pending.OwnerColor)) {
				return nil, errors.New("transform cannot target a frozen piece")
			}
			options := safeTransformOptions(state.Board, *intent.Target, pending.Mechanic, state.FortressZones)
			if len(options) == 0 {
				return nil, fmt.Errorf("no safe %s options available", pending.Mechanic)
			}
			pending.Target = intent.Target
			pending.Options = options
			state.UpdatedAt = now.UTC()
			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
					"cardId":   pending.CardID,
					"mechanic": pending.Mechanic,
					"target":   intent.Target,
					"options":  options,
				}),
			}, nil
		}

		if intent.SelectionID == "" {
			return nil, errors.New("transform selection requires a selectionId")
		}
		if !containsString(pending.Options, intent.SelectionID) {
			return nil, errors.New("selected transform is not allowed")
		}
		targetPiece := pieceAt(state.Board, *pending.Target)
		if targetPiece == nil {
			return nil, errors.New("pending target piece no longer exists")
		}
		targetPiece.Type = intent.SelectionID
		replaceLastHistorySnapshot(state)
	case "teleport":
		if pending.Target == nil {
			if intent.Target == nil {
				return nil, errors.New("teleport requires selecting your piece first")
			}
			targetPiece := pieceAt(state.Board, *intent.Target)
			if targetPiece == nil || targetPiece.Color != pending.OwnerColor || targetPiece.Type == "king" {
				return nil, errors.New("teleport requires your own non-king target")
			}
			if targetPiece.Frozen {
				return nil, errors.New("teleport cannot target a frozen piece")
			}
			pending.Target = intent.Target
			state.UpdatedAt = now.UTC()
			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
					"cardId":   pending.CardID,
					"mechanic": pending.Mechanic,
					"target":   intent.Target,
				}),
			}, nil
		}
		if intent.Target == nil {
			return nil, errors.New("teleport requires choosing a destination square")
		}
		if !inBounds(intent.Target.Row, intent.Target.Col) {
			return nil, errors.New("teleport destination is out of bounds")
		}
		if pieceAt(state.Board, *intent.Target) != nil {
			return nil, errors.New("teleport destination must be empty")
		}
		if fortressEntryBlocked(state.FortressZones, pending.OwnerColor, *intent.Target) {
			return nil, errors.New("teleport destination is protected by an enemy fortress")
		}
		movingPiece := pieceAt(state.Board, *pending.Target)
		if movingPiece == nil {
			return nil, errors.New("teleport source piece no longer exists")
		}
		if movingPiece.Frozen {
			return nil, errors.New("teleport cannot move a frozen piece")
		}
		nextBoard := cloneBoard(state.Board)
		nextMovingPiece := nextBoard[pending.Target.Row][pending.Target.Col]
		nextBoard[intent.Target.Row][intent.Target.Col] = nextMovingPiece
		nextBoard[pending.Target.Row][pending.Target.Col] = nil
		if !kingsRemainSafe(nextBoard, state.FortressZones) {
			return nil, errors.New("teleport destination is not safe")
		}
		state.Board = nextBoard
		invalidateCastlingRightsForSquare(state, *pending.Target)
		replaceLastHistorySnapshot(state)
	case "jump":
		if pending.Target == nil {
			if intent.Target == nil {
				return nil, errors.New("jump requires selecting your piece first")
			}
			targetPiece := pieceAt(state.Board, *intent.Target)
			if targetPiece == nil || targetPiece.Color != pending.OwnerColor || targetPiece.Type == "king" || targetPiece.Type == "knight" {
				return nil, errors.New("jump requires your own non-king, non-knight target")
			}
			if targetPiece.Frozen {
				return nil, errors.New("jump cannot target a frozen piece")
			}
			pending.Target = intent.Target
			state.UpdatedAt = now.UTC()
			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
					"cardId":   pending.CardID,
					"mechanic": pending.Mechanic,
					"target":   intent.Target,
				}),
			}, nil
		}
		if intent.Target == nil {
			return nil, errors.New("jump requires choosing a destination square")
		}
		if !inBounds(intent.Target.Row, intent.Target.Col) {
			return nil, errors.New("jump destination is out of bounds")
		}
		if fortressEntryBlocked(state.FortressZones, pending.OwnerColor, *intent.Target) {
			return nil, errors.New("jump destination is protected by an enemy fortress")
		}
		fromPiece := pieceAt(state.Board, *pending.Target)
		if fromPiece == nil {
			return nil, errors.New("jump source piece no longer exists")
		}
		if fromPiece.Frozen {
			return nil, errors.New("jump cannot move a frozen piece")
		}
		destinationPiece := pieceAt(state.Board, *intent.Target)
		if destinationPiece != nil && destinationPiece.Color == pending.OwnerColor {
			return nil, errors.New("jump cannot land on your own piece")
		}
		if destinationPiece != nil && destinationPiece.Type == "king" {
			return nil, errors.New("jump cannot capture the king")
		}
		if !jumpDirectionValid(*pending.Target, *intent.Target, fromPiece.Type, fromPiece.Color) {
			return nil, errors.New("jump destination is invalid for that piece")
		}
		if !jumpHasExactlyOnePieceBetween(state.Board, *pending.Target, *intent.Target) {
			return nil, errors.New("jump must have exactly one piece in between")
		}
		if fromPiece.Type == "pawn" && pending.Target.Col == intent.Target.Col && destinationPiece != nil {
			return nil, errors.New("pawn can only jump straight to an empty square")
		}
		nextBoard := cloneBoard(state.Board)
		nextMovingPiece := nextBoard[pending.Target.Row][pending.Target.Col]
		nextBoard[intent.Target.Row][intent.Target.Col] = nextMovingPiece
		nextBoard[pending.Target.Row][pending.Target.Col] = nil
		if !kingsRemainSafe(nextBoard, state.FortressZones) {
			return nil, errors.New("jump destination is not safe")
		}
		state.Board = nextBoard
		invalidateCastlingRightsForSquare(state, *pending.Target)
		replaceLastHistorySnapshot(state)
	case "swapme":
		if pending.Target == nil {
			if intent.Target == nil {
				return nil, errors.New("swapme requires selecting your first piece")
			}
			targetPiece := pieceAt(state.Board, *intent.Target)
			if targetPiece == nil || targetPiece.Color != pending.OwnerColor || targetPiece.Type == "king" {
				return nil, errors.New("swapme requires your own non-king target")
			}
			if targetPiece.Frozen {
				return nil, errors.New("swapme cannot target a frozen piece")
			}
			pending.Target = intent.Target
			state.UpdatedAt = now.UTC()
			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
					"cardId":   pending.CardID,
					"mechanic": pending.Mechanic,
					"target":   intent.Target,
				}),
			}, nil
		}
		if intent.Target == nil {
			return nil, errors.New("swapme requires selecting your second piece")
		}
		firstPiece := pieceAt(state.Board, *pending.Target)
		secondPiece := pieceAt(state.Board, *intent.Target)
		if firstPiece == nil {
			return nil, errors.New("swapme first piece no longer exists")
		}
		if secondPiece == nil || secondPiece.Color != pending.OwnerColor || secondPiece.Type == "king" {
			return nil, errors.New("swapme requires your own non-king second piece")
		}
		if secondPiece.Frozen {
			return nil, errors.New("swapme cannot target a frozen piece")
		}
		if pending.Target.Row == intent.Target.Row && pending.Target.Col == intent.Target.Col {
			return nil, errors.New("swapme requires two different pieces")
		}
		nextBoard := cloneBoard(state.Board)
		nextBoard[pending.Target.Row][pending.Target.Col], nextBoard[intent.Target.Row][intent.Target.Col] = nextBoard[intent.Target.Row][intent.Target.Col], nextBoard[pending.Target.Row][pending.Target.Col]
		if !kingsRemainSafe(nextBoard, state.FortressZones) {
			return nil, errors.New("swapme would create check")
		}
		state.Board = nextBoard
		invalidateCastlingRightsForSquare(state, *pending.Target)
		invalidateCastlingRightsForSquare(state, *intent.Target)
		replaceLastHistorySnapshot(state)
	case "swapus":
		if pending.Target == nil {
			if intent.Target == nil {
				return nil, errors.New("swapus requires selecting your piece first")
			}
			targetPiece := pieceAt(state.Board, *intent.Target)
			if targetPiece == nil || targetPiece.Color != pending.OwnerColor || targetPiece.Type == "king" {
				return nil, errors.New("swapus requires your own non-king target")
			}
			if targetPiece.Frozen {
				return nil, errors.New("swapus cannot target a frozen piece")
			}
			pending.Target = intent.Target
			state.UpdatedAt = now.UTC()
			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
					"cardId":   pending.CardID,
					"mechanic": pending.Mechanic,
					"target":   intent.Target,
				}),
			}, nil
		}
		if intent.Target == nil {
			return nil, errors.New("swapus requires selecting an enemy piece")
		}
		firstPiece := pieceAt(state.Board, *pending.Target)
		secondPiece := pieceAt(state.Board, *intent.Target)
		if firstPiece == nil {
			return nil, errors.New("swapus first piece no longer exists")
		}
		if firstPiece.Frozen {
			return nil, errors.New("swapus cannot move a frozen piece")
		}
		if secondPiece == nil || secondPiece.Color == pending.OwnerColor || secondPiece.Type == "king" {
			return nil, errors.New("swapus requires an enemy non-king second piece")
		}
		if secondPiece.Frozen {
			return nil, errors.New("swapus cannot target a frozen piece")
		}
		if fortressEntryBlocked(state.FortressZones, pending.OwnerColor, *intent.Target) {
			return nil, errors.New("swapus cannot swap a piece into an enemy fortress")
		}
		nextBoard := cloneBoard(state.Board)
		nextBoard[pending.Target.Row][pending.Target.Col], nextBoard[intent.Target.Row][intent.Target.Col] = nextBoard[intent.Target.Row][intent.Target.Col], nextBoard[pending.Target.Row][pending.Target.Col]
		if !kingsRemainSafe(nextBoard, state.FortressZones) {
			return nil, errors.New("swapus would create check")
		}
		state.Board = nextBoard
		invalidateCastlingRightsForSquare(state, *pending.Target)
		invalidateCastlingRightsForSquare(state, *intent.Target)
		replaceLastHistorySnapshot(state)
	case "swaphim":
		if pending.Target == nil {
			if intent.Target == nil {
				return nil, errors.New("swaphim requires selecting the first enemy piece")
			}
			targetPiece := pieceAt(state.Board, *intent.Target)
			if targetPiece == nil || targetPiece.Color == pending.OwnerColor || targetPiece.Type == "king" {
				return nil, errors.New("swaphim requires an enemy non-king target")
			}
			if targetPiece.Frozen {
				return nil, errors.New("swaphim cannot target a frozen piece")
			}
			pending.Target = intent.Target
			state.UpdatedAt = now.UTC()
			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
					"cardId":   pending.CardID,
					"mechanic": pending.Mechanic,
					"target":   intent.Target,
				}),
			}, nil
		}
		if intent.Target == nil {
			return nil, errors.New("swaphim requires selecting the second enemy piece")
		}
		firstPiece := pieceAt(state.Board, *pending.Target)
		secondPiece := pieceAt(state.Board, *intent.Target)
		if firstPiece == nil {
			return nil, errors.New("swaphim first piece no longer exists")
		}
		if secondPiece == nil || secondPiece.Color == pending.OwnerColor || secondPiece.Type == "king" {
			return nil, errors.New("swaphim requires an enemy non-king second piece")
		}
		if secondPiece.Frozen {
			return nil, errors.New("swaphim cannot target a frozen piece")
		}
		if pending.Target.Row == intent.Target.Row && pending.Target.Col == intent.Target.Col {
			return nil, errors.New("swaphim requires two different enemy pieces")
		}
		nextBoard := cloneBoard(state.Board)
		nextBoard[pending.Target.Row][pending.Target.Col], nextBoard[intent.Target.Row][intent.Target.Col] = nextBoard[intent.Target.Row][intent.Target.Col], nextBoard[pending.Target.Row][pending.Target.Col]
		if !kingsRemainSafe(nextBoard, state.FortressZones) {
			return nil, errors.New("swaphim would create check")
		}
		state.Board = nextBoard
		invalidateCastlingRightsForSquare(state, *pending.Target)
		invalidateCastlingRightsForSquare(state, *intent.Target)
		replaceLastHistorySnapshot(state)
	case "borrow":
		if intent.Target == nil {
			return nil, errors.New("borrow requires a target square")
		}
		targetPiece := pieceAt(state.Board, *intent.Target)
		if targetPiece == nil || targetPiece.Color == pending.OwnerColor || targetPiece.Type == "king" {
			return nil, errors.New("borrow requires an enemy non-king target")
		}
		if targetPiece.Frozen {
			return nil, errors.New("borrow cannot target a frozen piece")
		}
		if targetPiece.BorrowCount >= 3 {
			return nil, errors.New("piece has been borrowed too many times")
		}
		if fortressEntryBlocked(state.FortressZones, pending.OwnerColor, *intent.Target) {
			return nil, errors.New("borrow cannot target a piece inside an enemy fortress")
		}
		nextBoard := cloneBoard(state.Board)
		nextTarget := nextBoard[intent.Target.Row][intent.Target.Col]
		nextTarget.Color = pending.OwnerColor
		nextTarget.Borrowed = true
		nextTarget.BorrowCount++
		if !kingsRemainSafe(nextBoard, state.FortressZones) {
			return nil, errors.New("borrow target is not safe")
		}
		state.Board = nextBoard
		replaceLastHistorySnapshot(state)
	case "mindcontrol":
		if intent.Target == nil {
			return nil, errors.New("mindcontrol requires a target square")
		}
		targetPiece := pieceAt(state.Board, *intent.Target)
		if targetPiece == nil || targetPiece.Color == pending.OwnerColor || targetPiece.Type == "king" {
			return nil, errors.New("mindcontrol requires an enemy non-king target")
		}
		if targetPiece.Frozen {
			return nil, errors.New("mindcontrol cannot target a frozen piece")
		}
		if targetPiece.Shielded {
			return nil, errors.New("mindcontrol cannot target a shielded piece")
		}
		if fortressEntryBlocked(state.FortressZones, pending.OwnerColor, *intent.Target) {
			return nil, errors.New("mindcontrol cannot target a piece inside an enemy fortress")
		}
		nextBoard := cloneBoard(state.Board)
		nextTarget := nextBoard[intent.Target.Row][intent.Target.Col]
		nextTarget.Color = pending.OwnerColor
		nextTarget.Borrowed = false
		if !kingsRemainSafe(nextBoard, state.FortressZones) {
			return nil, errors.New("mindcontrol target is not safe")
		}
		state.Board = nextBoard
		replaceLastHistorySnapshot(state)
	case "parasite":
		if pending.Target == nil {
			if intent.Target == nil {
				return nil, errors.New("parasite requires selecting your host piece first")
			}
			targetPiece := pieceAt(state.Board, *intent.Target)
			if targetPiece == nil || targetPiece.Color != pending.OwnerColor || targetPiece.Type == "king" {
				return nil, errors.New("parasite requires your own non-king host")
			}
			pending.Target = intent.Target
			pending.Options = []string{strconv.Itoa(pieceValue(targetPiece.Type))}
			state.UpdatedAt = now.UTC()
			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
					"cardId":   pending.CardID,
					"mechanic": pending.Mechanic,
					"target":   intent.Target,
					"options":  pending.Options,
				}),
			}, nil
		}
		if intent.Target == nil {
			return nil, errors.New("parasite requires selecting an enemy target")
		}
		targetPiece := pieceAt(state.Board, *intent.Target)
		if targetPiece == nil || targetPiece.Color == pending.OwnerColor || targetPiece.Type == "king" || targetPiece.Fake {
			return nil, errors.New("parasite requires an enemy non-king target")
		}
		hostPiece := pieceAt(state.Board, *pending.Target)
		if hostPiece == nil {
			return nil, errors.New("parasite host no longer exists")
		}
		if len(pending.Options) == 0 {
			return nil, errors.New("parasite host value is missing")
		}
		hostValue, err := strconv.Atoi(pending.Options[0])
		if err != nil {
			return nil, errors.New("parasite host value is invalid")
		}
		if pieceValue(targetPiece.Type) != hostValue {
			return nil, fmt.Errorf("parasite requires an enemy piece with the same value (%d)", hostValue)
		}
		nextBoard := cloneBoard(state.Board)
		nextBoard[pending.Target.Row][pending.Target.Col].ParasiteTarget = fmt.Sprintf("%d,%d", intent.Target.Row, intent.Target.Col)
		state.Board = nextBoard
		replaceLastHistorySnapshot(state)
	case "clone":
		if pending.Target == nil {
			if intent.Target == nil {
				return nil, errors.New("clone requires selecting your piece first")
			}
			targetPiece := pieceAt(state.Board, *intent.Target)
			if targetPiece == nil || targetPiece.Color != pending.OwnerColor || targetPiece.Type == "king" {
				return nil, errors.New("clone requires your own non-king target")
			}
			if targetPiece.Frozen {
				return nil, errors.New("clone cannot target a frozen piece")
			}
			pending.Target = intent.Target
			state.UpdatedAt = now.UTC()
			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
					"cardId":   pending.CardID,
					"mechanic": pending.Mechanic,
					"target":   intent.Target,
				}),
			}, nil
		}
		if intent.Target == nil {
			return nil, errors.New("clone requires choosing a destination square")
		}
		if !inBounds(intent.Target.Row, intent.Target.Col) {
			return nil, errors.New("clone destination is out of bounds")
		}
		sourcePiece := pieceAt(state.Board, *pending.Target)
		if sourcePiece == nil {
			return nil, errors.New("clone source piece no longer exists")
		}
		if sourcePiece.Frozen {
			return nil, errors.New("clone cannot copy a frozen piece")
		}
		if pieceAt(state.Board, *intent.Target) != nil {
			return nil, errors.New("clone destination must be empty")
		}
		if fortressEntryBlocked(state.FortressZones, pending.OwnerColor, *intent.Target) {
			return nil, errors.New("clone destination is protected by an enemy fortress")
		}
		if abs(intent.Target.Row-pending.Target.Row) > 1 || abs(intent.Target.Col-pending.Target.Col) > 1 || (intent.Target.Row == pending.Target.Row && intent.Target.Col == pending.Target.Col) {
			return nil, errors.New("clone destination must be adjacent")
		}
		nextBoard := cloneBoard(state.Board)
		nextBoard[intent.Target.Row][intent.Target.Col] = &contracts.Piece{
			Type:           sourcePiece.Type,
			Color:          sourcePiece.Color,
			Shielded:       sourcePiece.Shielded,
			ShieldTurn:     sourcePiece.ShieldTurn,
			Frozen:         sourcePiece.Frozen,
			Borrowed:       sourcePiece.Borrowed,
			Bomb:           sourcePiece.Bomb,
			Invisible:      sourcePiece.Invisible,
			InvisibleTurn:  sourcePiece.InvisibleTurn,
			InvisibleOver:  sourcePiece.InvisibleOver,
			FusedWith:      sourcePiece.FusedWith,
		}
		if !kingsRemainSafe(nextBoard, state.FortressZones) {
			return nil, errors.New("clone destination is not safe")
		}
		state.Board = nextBoard
		replaceLastHistorySnapshot(state)
	case "fakepiece":
		if intent.Target == nil {
			return nil, errors.New("fakepiece requires a target square")
		}
		if !inBounds(intent.Target.Row, intent.Target.Col) {
			return nil, errors.New("fakepiece target is out of bounds")
		}
		if pieceAt(state.Board, *intent.Target) != nil {
			return nil, errors.New("fakepiece must target an empty square")
		}
		if fortressEntryBlocked(state.FortressZones, pending.OwnerColor, *intent.Target) {
			return nil, errors.New("fake piece cannot be placed inside an enemy fortress")
		}
		nextBoard := cloneBoard(state.Board)
		nextBoard[intent.Target.Row][intent.Target.Col] = &contracts.Piece{
			Type:  "pawn",
			Color: pending.OwnerColor,
			Fake:  true,
		}
		king := findKing(nextBoard, pending.OwnerColor)
		if king != nil && isAttackedWithFusion(nextBoard, *king, opposite(pending.OwnerColor), state.FortressZones) {
			return nil, errors.New("placing fakepiece there would expose your king")
		}
		state.Board = nextBoard
		replaceLastHistorySnapshot(state)
	case "blackhole":
		if pending.Target == nil {
			if intent.Target == nil {
				return nil, errors.New("blackhole requires the first target square")
			}
			if intent.Target.Row < 0 || intent.Target.Row > 7 || intent.Target.Col < 0 || intent.Target.Col > 7 {
				return nil, errors.New("blackhole target out of bounds")
			}
			pending.Target = intent.Target
			state.UpdatedAt = now.UTC()
			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
					"cardId":   pending.CardID,
					"mechanic": pending.Mechanic,
					"target":   intent.Target,
				}),
			}, nil
		}
		if intent.Target == nil {
			return nil, errors.New("blackhole requires the second target square")
		}
		if intent.Target.Row < 0 || intent.Target.Row > 7 || intent.Target.Col < 0 || intent.Target.Col > 7 {
			return nil, errors.New("blackhole target out of bounds")
		}
		if pending.Target.Row == intent.Target.Row && pending.Target.Col == intent.Target.Col {
			return nil, errors.New("blackhole requires two different target squares")
		}
		state.BlackHoles = append(state.BlackHoles, contracts.BlackHoleZone{
			Sq1:        *pending.Target,
			Sq2:        *intent.Target,
			TurnsLeft:  2,
			OwnerColor: pending.OwnerColor,
		})
		replaceLastHistorySnapshot(state)
	case "smallsacrifice", "bigsacrifice":
		goal := 6
		rewardCount := 2
		if pending.Mechanic == "bigsacrifice" {
			goal = 14
			rewardCount = 3
		}
		selected := parseSquareOptions(pending.Options)
		if intent.Target == nil {
			return nil, errors.New("sacrifice requires a target square")
		}
		targetPiece := pieceAt(state.Board, *intent.Target)
		if targetPiece == nil {
			totalValue := selectedSquaresValue(state.Board, selected)
			if totalValue < goal {
				return nil, fmt.Errorf("sacrifice requires at least %d points", goal)
			}
			nextBoard := cloneBoard(state.Board)
			for _, sq := range selected {
				nextBoard[sq.Row][sq.Col] = nil
			}
			king := findKing(nextBoard, pending.OwnerColor)
			if king != nil && isAttackedWithFusion(nextBoard, *king, opposite(pending.OwnerColor), state.FortressZones) {
				return nil, errors.New("cannot sacrifice because it would leave your king in check")
			}
			state.Board = nextBoard
			drawn := addRewardCards(state, pending.OwnerColor, rewardCount, now)
			replaceLastHistorySnapshot(state)
			removeCardFromHand(state, pending.OwnerColor, pending.CardID)
			state.PendingCard = nil
			state.DrawOfferedBy = ""
			state.UpdatedAt = now.UTC()
			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
					"cardId":     pending.CardID,
					"mechanic":   pending.Mechanic,
					"target":     intent.Target,
					"selected":   selected,
					"totalValue": totalValue,
					"drawnCards": drawn,
				}),
			}, nil
		}
		if targetPiece.Color != pending.OwnerColor || targetPiece.Type == "king" {
			return nil, errors.New("sacrifice requires your own non-king pieces")
		}
		updated := toggleSquareInList(selected, *intent.Target)
		pending.Options = encodeSquareOptions(updated)
		state.UpdatedAt = now.UTC()
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
				"cardId":     pending.CardID,
				"mechanic":   pending.Mechanic,
				"target":     intent.Target,
				"selected":   updated,
				"totalValue": selectedSquaresValue(state.Board, updated),
			}),
		}, nil
	case "lavaground":
		if intent.Target == nil {
			return nil, errors.New("lavaground requires a target square")
		}
		if pieceAt(state.Board, *intent.Target) != nil {
			return nil, errors.New("lavaground must target an empty square")
		}
		for _, lava := range state.LavaSquares {
			if lava.Row == intent.Target.Row && lava.Col == intent.Target.Col {
				return nil, errors.New("lavaground already exists on that square")
			}
		}
		state.LavaSquares = append(state.LavaSquares, contracts.LavaSquare{
			Row:       intent.Target.Row,
			Col:       intent.Target.Col,
			MovesLeft: 2,
		})
		replaceLastHistorySnapshot(state)
	case "fog_village":
		if intent.Target == nil {
			return nil, errors.New("fog_village requires a target square")
		}
		centerRow := intent.Target.Row
		centerCol := intent.Target.Col
		if centerRow < 1 {
			centerRow = 1
		} else if centerRow > 6 {
			centerRow = 6
		}
		if centerCol < 1 {
			centerCol = 1
		} else if centerCol > 6 {
			centerCol = 6
		}
		nextFog := make([]contracts.FogZone, 0, len(state.FogZones)+1)
		for _, zone := range state.FogZones {
			if zone.OwnerColor != pending.OwnerColor {
				nextFog = append(nextFog, zone)
			}
		}
		nextFog = append(nextFog, contracts.FogZone{
			CenterRow:  centerRow,
			CenterCol:  centerCol,
			TurnsLeft:  2,
			OwnerColor: pending.OwnerColor,
		})
		state.FogZones = nextFog
		replaceLastHistorySnapshot(state)
	case "fortress":
		if intent.Target == nil {
			return nil, errors.New("fortress requires a target square")
		}
		topRow := clampInt(intent.Target.Row, 0, 6)
		leftCol := clampInt(intent.Target.Col, 0, 6)
		nextFortress := make([]contracts.FortressZone, 0, len(state.FortressZones)+1)
		for _, zone := range state.FortressZones {
			if zone.OwnerColor != pending.OwnerColor {
				nextFortress = append(nextFortress, zone)
			}
		}
		nextFortress = append(nextFortress, contracts.FortressZone{
			TopRow:     topRow,
			LeftCol:    leftCol,
			TurnsLeft:  2,
			OwnerColor: pending.OwnerColor,
		})
		state.FortressZones = nextFortress
		replaceLastHistorySnapshot(state)
	case "invisible":
		if intent.Target == nil {
			return nil, errors.New("invisible requires a target square")
		}
		targetPiece := pieceAt(state.Board, *intent.Target)
		if targetPiece == nil || targetPiece.Color != pending.OwnerColor || targetPiece.Type == "king" {
			return nil, errors.New("invisible requires your own non-king target")
		}
		nextBoard := cloneBoard(state.Board)
		nextPiece := nextBoard[intent.Target.Row][intent.Target.Col]
		nextBoard[intent.Target.Row][intent.Target.Col] = nil
		state.Board = nextBoard
		replaceLastHistorySnapshot(state)
		inFog := false
		for _, fz := range state.FogZones {
			if abs(intent.Target.Row-fz.CenterRow) <= 1 && abs(intent.Target.Col-fz.CenterCol) <= 1 {
				inFog = true
				break
			}
		}
		state.InvisiblePiece = &contracts.InvisiblePieceState{
			Row:        intent.Target.Row,
			Col:        intent.Target.Col,
			Piece:      *nextPiece,
			OwnerColor: pending.OwnerColor,
			RoundsLeft: 1,
			InFogZone:  inFog,
		}
	case "unabomber":
		if intent.Target == nil {
			return nil, errors.New("unabomber requires a target square")
		}
		targetPiece := pieceAt(state.Board, *intent.Target)
		if targetPiece == nil || targetPiece.Color != pending.OwnerColor || targetPiece.Type == "king" {
			return nil, errors.New("unabomber requires your own non-king target")
		}
		nextBoard := cloneBoard(state.Board)
		nextBoard[intent.Target.Row][intent.Target.Col].Bomb = true
		state.Board = nextBoard
		replaceLastHistorySnapshot(state)
		state.BombPieces = append(state.BombPieces, contracts.BombPiece{
			Row:        intent.Target.Row,
			Col:        intent.Target.Col,
			TurnsLeft:  2,
			OwnerColor: pending.OwnerColor,
		})
	case "halffuse":
		const halfFuseCap = 6
		if pending.Target == nil {
			if intent.Target == nil {
				return nil, errors.New("halffuse requires selecting your first piece")
			}
			targetPiece := pieceAt(state.Board, *intent.Target)
			if targetPiece == nil || targetPiece.Color != pending.OwnerColor || targetPiece.Type == "king" {
				return nil, errors.New("halffuse requires your own non-king piece")
			}
			if targetPiece.FusedWith != "" {
				return nil, errors.New("halffuse cannot target an already fused piece")
			}
			value := pieceValue(targetPiece.Type)
			if value >= halfFuseCap {
				return nil, errors.New("halffuse first piece is too expensive")
			}
			pending.Target = intent.Target
			pending.Options = []string{targetPiece.Type, strconv.Itoa(value)}
			state.UpdatedAt = now.UTC()
			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
					"cardId":   pending.CardID,
					"mechanic": pending.Mechanic,
					"target":   intent.Target,
				}),
			}, nil
		}
		if intent.Target == nil {
			return nil, errors.New("halffuse requires selecting the second piece")
		}
		if pending.Target.Row == intent.Target.Row && pending.Target.Col == intent.Target.Col {
			return nil, errors.New("halffuse requires a different second piece")
		}
		if abs(intent.Target.Row-pending.Target.Row) > 1 || abs(intent.Target.Col-pending.Target.Col) > 1 {
			return nil, errors.New("halffuse requires adjacent pieces")
		}
		if len(pending.Options) < 2 {
			return nil, errors.New("halffuse first piece metadata is missing")
		}
		firstType := pending.Options[0]
		firstValue, err := strconv.Atoi(pending.Options[1])
		if err != nil {
			return nil, errors.New("halffuse first piece value is invalid")
		}
		secondPiece := pieceAt(state.Board, *intent.Target)
		if secondPiece == nil || secondPiece.Color != pending.OwnerColor || secondPiece.Type == "king" {
			return nil, errors.New("halffuse requires your own non-king second piece")
		}
		if secondPiece.FusedWith != "" {
			return nil, errors.New("halffuse cannot target an already fused second piece")
		}
		isBishopRook := (firstType == "bishop" && secondPiece.Type == "rook") || (firstType == "rook" && secondPiece.Type == "bishop")
		if !isBishopRook && firstValue+pieceValue(secondPiece.Type) > halfFuseCap {
			return nil, errors.New("halffuse combined value exceeds the cap")
		}
		if redundancy := fusionRedundancy(firstType, secondPiece.Type, *pending.Target, *intent.Target); redundancy != "" {
			return nil, errors.New(redundancy)
		}
		nextBoard := cloneBoard(state.Board)
		nextBoard[pending.Target.Row][pending.Target.Col] = nil
		targetOnNext := nextBoard[intent.Target.Row][intent.Target.Col]
		if targetOnNext == nil {
			return nil, errors.New("halffuse second piece no longer exists")
		}
		if isBishopRook {
			targetOnNext.Type = "queen"
			targetOnNext.FusedWith = ""
		} else {
			targetOnNext.FusedWith = firstType
		}
		if !kingsRemainSafeWithFusion(nextBoard, state.FortressZones) {
			return nil, errors.New("halffuse would leave a king in check")
		}
		state.Board = nextBoard
		replaceLastHistorySnapshot(state)
	case "fullfusion":
		if pending.Target == nil {
			if intent.Target == nil {
				return nil, errors.New("fullfusion requires selecting your first piece")
			}
			targetPiece := pieceAt(state.Board, *intent.Target)
			if targetPiece == nil || targetPiece.Color != pending.OwnerColor || targetPiece.Type == "king" {
				return nil, errors.New("fullfusion requires your own non-king piece")
			}
			if targetPiece.FusedWith != "" {
				return nil, errors.New("fullfusion cannot target an already fused piece")
			}
			pending.Target = intent.Target
			pending.Options = []string{targetPiece.Type}
			state.UpdatedAt = now.UTC()
			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
					"cardId":   pending.CardID,
					"mechanic": pending.Mechanic,
					"target":   intent.Target,
				}),
			}, nil
		}
		if intent.Target == nil {
			return nil, errors.New("fullfusion requires selecting the second piece")
		}
		if pending.Target.Row == intent.Target.Row && pending.Target.Col == intent.Target.Col {
			return nil, errors.New("fullfusion requires a different second piece")
		}
		if abs(intent.Target.Row-pending.Target.Row) > 1 || abs(intent.Target.Col-pending.Target.Col) > 1 {
			return nil, errors.New("fullfusion requires adjacent pieces")
		}
		if len(pending.Options) < 1 {
			return nil, errors.New("fullfusion first piece metadata is missing")
		}
		firstType := pending.Options[0]
		secondPiece := pieceAt(state.Board, *intent.Target)
		if secondPiece == nil || secondPiece.Color != pending.OwnerColor || secondPiece.Type == "king" {
			return nil, errors.New("fullfusion requires your own non-king second piece")
		}
		if secondPiece.FusedWith != "" {
			return nil, errors.New("fullfusion cannot target an already fused second piece")
		}
		if redundancy := fusionRedundancy(firstType, secondPiece.Type, *pending.Target, *intent.Target); redundancy != "" {
			return nil, errors.New(redundancy)
		}
		nextBoard := cloneBoard(state.Board)
		nextBoard[pending.Target.Row][pending.Target.Col] = nil
		targetOnNext := nextBoard[intent.Target.Row][intent.Target.Col]
		if targetOnNext == nil {
			return nil, errors.New("fullfusion second piece no longer exists")
		}
		isBishopRook := (firstType == "bishop" && secondPiece.Type == "rook") || (firstType == "rook" && secondPiece.Type == "bishop")
		if isBishopRook {
			targetOnNext.Type = "queen"
			targetOnNext.FusedWith = ""
		} else {
			targetOnNext.FusedWith = firstType
		}
		if !kingsRemainSafeWithFusion(nextBoard, state.FortressZones) {
			return nil, errors.New("fullfusion would leave a king in check")
		}
		state.Board = nextBoard
		replaceLastHistorySnapshot(state)
	case "joker":
		if intent.SelectionID == "" {
			return nil, errors.New("joker transform requires a selectionId")
		}
		if !containsString(pending.Options, intent.SelectionID) {
			return nil, errors.New("selected joker transform is not allowed")
		}
		template, found := starterCardTemplate(intent.SelectionID)
		if !found {
			return nil, errors.New("selected joker transform template was not found")
		}
		removeCardFromHand(state, pending.OwnerColor, pending.CardID)
		template.ID = fmt.Sprintf("joker_%s_%s_%d", template.Mechanic, pending.OwnerColor, now.UnixMilli())
		addCardToHand(state, pending.OwnerColor, template)
		state.PendingCard = nil
		state.DrawOfferedBy = ""
		state.UpdatedAt = now.UTC()
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
				"cardId":      pending.CardID,
				"mechanic":    pending.Mechanic,
				"selectionId": intent.SelectionID,
				"newCard":     template,
			}),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported pending mechanic: %s", pending.Mechanic)
	}

	targetPayload := any(nil)
	if intent.Target != nil {
		targetPayload = intent.Target
	} else if pending.Target != nil {
		targetPayload = pending.Target
	}

	removeCardFromHand(state, pending.OwnerColor, pending.CardID)
	state.PendingCard = nil
	state.DrawOfferedBy = ""
	state.UpdatedAt = now.UTC()

	payload := map[string]any{
		"cardId":   pending.CardID,
		"mechanic": pending.Mechanic,
		"target":   targetPayload,
	}
	if intent.SelectionID != "" {
		payload["selectionId"] = intent.SelectionID
	}

	return []contracts.ResolvedEvent{
		makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, payload),
	}, nil
}

func applyMirrorCard(state *contracts.MatchState, owner string) (bool, *contracts.Square, *contracts.Square, error) {
	if state.LastMove == nil {
		return false, nil, nil, nil
	}

	movedPiece := pieceAt(state.Board, state.LastMove.To)
	if movedPiece == nil {
		return false, nil, nil, nil
	}

	dr := state.LastMove.To.Row - state.LastMove.From.Row
	dc := state.LastMove.To.Col - state.LastMove.From.Col
	movedType := movedPiece.Type

	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			piece := state.Board[row][col]
			if piece == nil || piece.Color != owner || piece.Type != movedType {
				continue
			}

			from := contracts.Square{Row: row, Col: col}
			to := contracts.Square{Row: row + dr, Col: col + dc}
			if !inBounds(to.Row, to.Col) {
				continue
			}
			if occupant := pieceAt(state.Board, to); occupant != nil && occupant.Color == owner {
				continue
			}

			capturedSquare := to
			captured := pieceAt(state.Board, to)
			if captured != nil && captured.Shielded {
				captured.Shielded = false
				captured.ShieldTurn = nil
				state.DrawOfferedBy = ""
				return true, &from, &to, nil
			}
			nextBoard := cloneBoard(state.Board)
			nextPiece := pieceAt(nextBoard, from)
			if nextPiece == nil {
				continue
			}

			movePiece(nextBoard, from, to, nextPiece, false)
			if err := resolveParasiteEffects(nextBoard, from, to, capturedSquare, captured, state.FortressZones); err != nil {
				continue
			}
			updateParasiteLinksForMove(nextBoard, from, to)

			king := findKing(nextBoard, owner)
			if king != nil && isAttackedWithFusion(nextBoard, *king, opposite(owner), state.FortressZones) {
				continue
			}

			state.Board = nextBoard
			updateBombTracker(state, from, to)
			resolveLavaEffects(state, to)
			replaceLastHistorySnapshot(state)
			return true, &from, &to, nil
		}
	}

	return false, nil, nil, nil
}

func invalidateCastlingRightsForSquare(state *contracts.MatchState, sq contracts.Square) {
	key := keyForSquare(sq)
	for _, existing := range state.Moved {
		if existing == key {
			return
		}
	}
	switch key {
	case "0-4", "0-0", "0-7", "7-4", "7-0", "7-7":
		state.Moved = append(state.Moved, key)
	}
}

func resolveGamblerCard(state *contracts.MatchState, owner string, now time.Time) map[string]any {
	opponent := opposite(owner)
	var myHand, oppHand []contracts.GameCard
	if owner == "black" {
		myHand = state.BlackHand
		oppHand = state.WhiteHand
	} else {
		myHand = state.WhiteHand
		oppHand = state.BlackHand
	}

	roll := deterministicCardIndex(state, len(myHand)+len(oppHand)+int(state.RNGSeed%100))
	win := len(oppHand) > 0 && (roll%2 == 0 || len(myHand) <= 1)
	if win && len(oppHand) > 0 {
		stolenIndex := deterministicCardIndex(state, len(oppHand)+1) % len(oppHand)
		stolen := oppHand[stolenIndex]
		removeCardFromHand(state, opponent, stolen.ID)
		forceAddCardToHand(state, owner, stolen)
		return map[string]any{
			"outcome": "win",
			"card":    stolen,
		}
	}

	candidates := filterCardsNotMechanic(myHand, "gambler")
	if len(candidates) > 0 {
		giveIndex := deterministicCardIndex(state, len(candidates)+3) % len(candidates)
		given := candidates[giveIndex]
		removeCardFromHand(state, owner, given.ID)
		forceAddCardToHand(state, opponent, given)
		return map[string]any{
			"outcome": "lose",
			"card":    given,
		}
	}

	return map[string]any{
		"outcome": "none",
	}
}

// forceAddCardToHand always adds card, bypassing the maxHandSize check in addCardToHand
// to ensure the swap completes. The caller should enforce maxHandSize separately.
func forceAddCardToHand(state *contracts.MatchState, owner string, card contracts.GameCard) {
	if owner == "black" {
		state.BlackHand = append(state.BlackHand, card)
		return
	}
	state.WhiteHand = append(state.WhiteHand, card)
}

func resolveLavaEffects(state *contracts.MatchState, landing contracts.Square) (bool, string) {
	if len(state.LavaSquares) == 0 {
		return false, ""
	}

	triggered := false
	capturedPieceType := ""
	nextLava := make([]contracts.LavaSquare, 0, len(state.LavaSquares))
	for _, lava := range state.LavaSquares {
		if lava.Row == landing.Row && lava.Col == landing.Col {
			triggered = true
			piece := pieceAt(state.Board, landing)
			if piece != nil && piece.Type != "king" {
				if piece.Shielded {
					piece.Shielded = false
					piece.ShieldTurn = nil
				} else {
					capturedPieceType = piece.Type
					state.Board[landing.Row][landing.Col] = nil
				}
			}
			continue
		}

		lava.MovesLeft--
		if lava.MovesLeft > 0 {
			nextLava = append(nextLava, lava)
		}
	}
	state.LavaSquares = nextLava
	return triggered, capturedPieceType
}

func updateBombTracker(state *contracts.MatchState, from, to contracts.Square) {
	if len(state.BombPieces) == 0 {
		return
	}
	for i := range state.BombPieces {
		bomb := &state.BombPieces[i]
		if bomb.Row == from.Row && bomb.Col == from.Col {
			bomb.Row = to.Row
			bomb.Col = to.Col
			return
		}
	}
}

func resolveBombEffects(state *contracts.MatchState) []contracts.Square {
	if len(state.BombPieces) == 0 {
		return nil
	}

	nextBombs := make([]contracts.BombPiece, 0, len(state.BombPieces))
	exploded := make([]contracts.Square, 0)
	for _, bomb := range state.BombPieces {
		piece := pieceAt(state.Board, contracts.Square{Row: bomb.Row, Col: bomb.Col})
		if piece == nil || !piece.Bomb {
			continue
		}
		if piece.Color != bomb.OwnerColor {
			log.Printf("bomb dropped for match %s: bomb.OwnerColor=%s piece.Color=%s", state.MatchID, bomb.OwnerColor, piece.Color)
			continue
		}

		bomb.TurnsLeft--
		if bomb.TurnsLeft <= 0 {
			piece.Bomb = false
			for dr := -1; dr <= 1; dr++ {
				for dc := -1; dc <= 1; dc++ {
					r := bomb.Row + dr
					c := bomb.Col + dc
					if !inBounds(r, c) {
						continue
					}
					target := state.Board[r][c]
					if target != nil && target.Type != "king" {
						if target.Shielded {
							target.Shielded = false
							target.ShieldTurn = nil
							continue
						}
						state.Board[r][c] = nil
						exploded = append(exploded, contracts.Square{Row: r, Col: c})
					}
				}
			}
			continue
		}

		nextBombs = append(nextBombs, bomb)
	}

	state.BombPieces = nextBombs
	return exploded
}

func resolveFogEffects(state *contracts.MatchState, justMovedColor string) {
	if len(state.FogZones) == 0 {
		return
	}

	nextFog := make([]contracts.FogZone, 0, len(state.FogZones))
	for _, zone := range state.FogZones {
		if zone.OwnerColor != justMovedColor {
			zone.TurnsLeft--
		}
		if zone.TurnsLeft > 0 {
			nextFog = append(nextFog, zone)
		}
	}
	state.FogZones = nextFog
}

func resolveFortressEffects(state *contracts.MatchState, justMovedColor string) {
	if len(state.FortressZones) == 0 {
		return
	}

	nextFortress := make([]contracts.FortressZone, 0, len(state.FortressZones))
	for _, zone := range state.FortressZones {
		if zone.OwnerColor != justMovedColor {
			zone.TurnsLeft--
		}
		if zone.TurnsLeft > 0 {
			nextFortress = append(nextFortress, zone)
		}
	}
	state.FortressZones = nextFortress
}

func fortressEntryBlocked(zones []contracts.FortressZone, moverColor string, target contracts.Square) bool {
	for _, zone := range zones {
		if zone.OwnerColor == moverColor {
			continue
		}
		if target.Row >= zone.TopRow && target.Row <= zone.TopRow+1 && target.Col >= zone.LeftCol && target.Col <= zone.LeftCol+1 {
			return true
		}
	}
	return false
}

func resolveBlackHoleEffects(state *contracts.MatchState, justMovedColor string) []contracts.Square {
	if len(state.BlackHoles) == 0 {
		return nil
	}

	nextBlackHoles := make([]contracts.BlackHoleZone, 0, len(state.BlackHoles))
	exploded := make([]contracts.Square, 0)
	seen := make(map[string]struct{})

	blow := func(center contracts.Square) {
		for dr := -1; dr <= 1; dr++ {
			for dc := -1; dc <= 1; dc++ {
				r := center.Row + dr
				c := center.Col + dc
				if !inBounds(r, c) {
					continue
				}
				target := state.Board[r][c]
				if target != nil && target.Type != "king" {
					if target.Shielded {
						target.Shielded = false
						target.ShieldTurn = nil
						continue
					}
					state.Board[r][c] = nil
					key := keyForCoords(r, c)
					if _, ok := seen[key]; !ok {
						seen[key] = struct{}{}
						exploded = append(exploded, contracts.Square{Row: r, Col: c})
					}
				}
			}
		}
	}

	for _, hole := range state.BlackHoles {
		if hole.OwnerColor != justMovedColor {
			hole.TurnsLeft--
		}
		if hole.TurnsLeft <= 0 {
			blow(hole.Sq1)
			blow(hole.Sq2)
			continue
		}
		nextBlackHoles = append(nextBlackHoles, hole)
	}

	state.BlackHoles = nextBlackHoles
	return exploded
}

func resolveParasiteEffects(board [][]*contracts.Piece, from, to, capturedSquare contracts.Square, capturedPiece *contracts.Piece, fortressZones []contracts.FortressZone) error {
	if capturedPiece == nil {
		return nil
	}

	// Fake pieces are not real; skip parasite-linked destruction
	if capturedPiece.Fake {
		return nil
	}

	if capturedPiece.ParasiteTarget != "" {
		if hostSq, ok := parseParasiteSquare(capturedPiece.ParasiteTarget); ok {
			hostPiece := pieceAt(board, hostSq)
			if hostPiece != nil && hostPiece.Type != "king" {
				if hostPiece.Shielded {
					hostPiece.Shielded = false
					hostPiece.ShieldTurn = nil
				} else {
					if err := ensurePieceRemovalKeepsOwnKingSafe(board, hostSq, fortressZones); err != nil {
						return errors.New("parasite capture would leave a king in check")
					}
					board[hostSq.Row][hostSq.Col] = nil
				}
			}
		}
	}

	for r := 0; r < len(board); r++ {
		for c := 0; c < len(board[r]); c++ {
			piece := board[r][c]
			if piece == nil || piece.ParasiteTarget == "" || piece.Fake {
				continue
			}
			targetSq, ok := parseParasiteSquare(piece.ParasiteTarget)
			if !ok || targetSq.Row != capturedSquare.Row || targetSq.Col != capturedSquare.Col {
				continue
			}
			if piece.Type != "king" {
				if piece.Shielded {
					piece.Shielded = false
					piece.ShieldTurn = nil
					continue
				}
				if err := ensurePieceRemovalKeepsOwnKingSafe(board, contracts.Square{Row: r, Col: c}, fortressZones); err != nil {
					return errors.New("parasite capture would leave a king in check")
				}
				board[r][c] = nil
			}
		}
	}

	return nil
}

func updateParasiteLinksForMove(board [][]*contracts.Piece, from, to contracts.Square) {
	for r := 0; r < len(board); r++ {
		for c := 0; c < len(board[r]); c++ {
			piece := board[r][c]
			if piece == nil || piece.ParasiteTarget == "" {
				continue
			}
			targetSq, ok := parseParasiteSquare(piece.ParasiteTarget)
			if !ok || targetSq.Row != from.Row || targetSq.Col != from.Col {
				continue
			}
			piece.ParasiteTarget = fmt.Sprintf("%d,%d", to.Row, to.Col)
		}
	}
}

func parseParasiteSquare(value string) (contracts.Square, bool) {
	parts := strings.Split(value, ",")
	if len(parts) != 2 {
		return contracts.Square{}, false
	}
	row, err := strconv.Atoi(parts[0])
	if err != nil {
		return contracts.Square{}, false
	}
	col, err := strconv.Atoi(parts[1])
	if err != nil {
		return contracts.Square{}, false
	}
	if !inBounds(row, col) {
		return contracts.Square{}, false
	}
	return contracts.Square{Row: row, Col: col}, true
}

func pieceValue(pieceType string) int {
	switch pieceType {
	case "pawn":
		return 1
	case "knight", "bishop":
		return 3
	case "rook":
		return 5
	case "queen":
		return 9
	default:
		return 0
	}
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func ensurePieceRemovalKeepsOwnKingSafe(board [][]*contracts.Piece, square contracts.Square, fortressZones []contracts.FortressZone) error {
	piece := pieceAt(board, square)
	if piece == nil {
		return nil
	}
	nextBoard := cloneBoard(board)
	nextBoard[square.Row][square.Col] = nil
	king := findKing(nextBoard, piece.Color)
	if king != nil && isAttacked(nextBoard, *king, opposite(piece.Color), fortressZones) {
		return errors.New("removal would leave king in check")
	}
	return nil
}

func ensureRemovalDoesNotCreateCheck(board [][]*contracts.Piece, target contracts.Square, ownerColor string, fortressZones []contracts.FortressZone) error {
	nextBoard := cloneBoard(board)
	nextBoard[target.Row][target.Col] = nil

	ownerKing := findKing(nextBoard, ownerColor)
	if ownerKing != nil && isAttacked(nextBoard, *ownerKing, opposite(ownerColor), fortressZones) {
		return errors.New("cannot remove that piece because it would leave your king in check")
	}

	enemyColor := opposite(ownerColor)
	enemyKing := findKing(nextBoard, enemyColor)
	if enemyKing != nil && isAttacked(nextBoard, *enemyKing, ownerColor, fortressZones) {
		return errors.New("cannot remove that piece because it would leave enemy king in check")
	}

	return nil
}

func safeTransformOptions(board [][]*contracts.Piece, target contracts.Square, mechanic string, fortressZones []contracts.FortressZone) []string {
	piece := pieceAt(board, target)
	if piece == nil {
		return nil
	}

	options := transformOptions(piece.Type, mechanic)
	if len(options) == 0 {
		return nil
	}

	safe := make([]string, 0, len(options))
	for _, option := range options {
		nextBoard := cloneBoard(board)
		nextPiece := nextBoard[target.Row][target.Col]
		if nextPiece == nil {
			continue
		}
		nextPiece.Type = option

		if !kingsRemainSafe(nextBoard, fortressZones) {
			continue
		}

		safe = append(safe, option)
	}

	return safe
}

func transformOptions(pieceType string, mechanic string) []string {
	switch mechanic {
	case "promote", "promotehim":
		switch pieceType {
		case "pawn":
			return []string{"knight", "bishop", "rook", "queen"}
		case "knight":
			return []string{"bishop", "rook", "queen"}
		case "bishop":
			return []string{"knight", "rook", "queen"}
		case "rook":
			return []string{"queen"}
		}
	case "demote", "demotehim":
		switch pieceType {
		case "queen":
			return []string{"rook", "bishop", "knight", "pawn"}
		case "rook":
			return []string{"bishop", "knight", "pawn"}
		case "bishop":
			return []string{"knight", "pawn"}
		case "knight":
			return []string{"pawn"}
		}
	}

	return nil
}

func validateTransformTarget(piece *contracts.Piece, ownerColor string, mechanic string, target contracts.Square) error {
	if piece == nil || piece.Type == "king" {
		switch mechanic {
		case "promotehim":
			return errors.New("promotehim requires an enemy non-king target")
		case "demotehim":
			return errors.New("demotehim requires a non-king target")
		case "promote":
			return errors.New("promote requires your own non-king target")
		default:
			return errors.New("demote requires your own non-king target")
		}
	}

	switch mechanic {
	case "promote":
		if piece.Color != ownerColor {
			return errors.New("promote requires your own non-king target")
		}
	case "demote":
		if piece.Color != ownerColor {
			return errors.New("demote requires your own non-king target")
		}
	case "promotehim":
		if piece.Color == ownerColor {
			return errors.New("promotehim requires an enemy non-king target")
		}
	case "demotehim":
		return nil
	}

	return nil
}

func isPawnOnPromotionRanks(row int, color string) bool {
	if color == "white" {
		return row >= 6
	}
	return row <= 1
}

func kingsRemainSafe(board [][]*contracts.Piece, fortressZones []contracts.FortressZone) bool {
	whiteKing := findKing(board, "white")
	if whiteKing != nil && isAttacked(board, *whiteKing, "black", fortressZones) {
		return false
	}
	blackKing := findKing(board, "black")
	if blackKing != nil && isAttacked(board, *blackKing, "white", fortressZones) {
		return false
	}
	return true
}

func kingsRemainSafeWithFusion(board [][]*contracts.Piece, fortressZones []contracts.FortressZone) bool {
	whiteKing := findKing(board, "white")
	if whiteKing != nil && isAttackedWithFusion(board, *whiteKing, "black", fortressZones) {
		return false
	}
	blackKing := findKing(board, "black")
	if blackKing != nil && isAttackedWithFusion(board, *blackKing, "white", fortressZones) {
		return false
	}
	return true
}

func isAttackedWithFusion(board [][]*contracts.Piece, target contracts.Square, by string, fortressZones []contracts.FortressZone) bool {
	if isAttacked(board, target, by, fortressZones) {
		return true
	}
	for r := 0; r < len(board); r++ {
		for c := 0; c < len(board[r]); c++ {
			piece := board[r][c]
			if piece == nil || piece.Color != by || piece.FusedWith == "" || piece.Fake {
				continue
			}
			tempBoard := cloneBoard(board)
			tempBoard[r][c] = &contracts.Piece{
				Type:           piece.FusedWith,
				Color:          piece.Color,
				Shielded:       piece.Shielded,
				ShieldTurn:     piece.ShieldTurn,
				Frozen:         piece.Frozen,
				Borrowed:       piece.Borrowed,
				ParasiteTarget: piece.ParasiteTarget,
				Bomb:           piece.Bomb,
				Invisible:      piece.Invisible,
				InvisibleTurn:  piece.InvisibleTurn,
				InvisibleOver:  piece.InvisibleOver,
			}
			if isAttacked(tempBoard, target, by, fortressZones) {
				return true
			}
		}
	}
	return false
}

func fusionRedundancy(typeA string, typeB string, sqA contracts.Square, sqB contracts.Square) string {
	if typeA == typeB {
		return "cannot fuse identical piece types"
	}
	if (typeA == "queen" && typeB == "rook") || (typeA == "rook" && typeB == "queen") {
		return "queen already moves like a rook"
	}
	if (typeA == "queen" && typeB == "bishop") || (typeA == "bishop" && typeB == "queen") {
		return "queen already moves like a bishop"
	}
	if (typeA == "queen" && typeB == "pawn") || (typeA == "pawn" && typeB == "queen") {
		return "queen already outclasses pawn movement"
	}
	if typeA == "bishop" && typeB == "bishop" && ((sqA.Row+sqA.Col)%2 == (sqB.Row+sqB.Col)%2) {
		return "bishops on the same color add no new movement"
	}
	return ""
}

func jumpHasExactlyOnePieceBetween(board [][]*contracts.Piece, from contracts.Square, to contracts.Square) bool {
	dr := to.Row - from.Row
	dc := to.Col - from.Col
	if dr == 0 && dc == 0 {
		return false
	}

	sr := sign(dr)
	sc := sign(dc)
	r := from.Row + sr
	c := from.Col + sc
	count := 0
	for r != to.Row || c != to.Col {
		if !inBounds(r, c) {
			return false
		}
		if board[r][c] != nil {
			count++
		}
		r += sr
		c += sc
	}

	return count == 1
}

func jumpDirectionValid(from contracts.Square, to contracts.Square, pieceType string, pieceColor string) bool {
	dr := to.Row - from.Row
	dc := to.Col - from.Col
	diag := abs(dr) == abs(dc)
	straight := dr == 0 || dc == 0

	switch pieceType {
	case "bishop":
		return diag
	case "rook":
		return straight
	case "queen":
		return diag || straight
	case "pawn":
		fwd := 1
		if pieceColor == "black" {
			fwd = -1
		}
		return (dc == 0 && (dr == fwd || dr == fwd*2)) || (abs(dc) == 2 && dr == fwd*2)
	default:
		return false
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
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
		historyKeys = append(historyKeys, positionKey(position.Board, position.Turn, sliceToSet(position.Moved), position.LastMove, position.WhiteHand, position.BlackHand))
	}
	currentKey := positionKey(state.Board, state.Turn, sliceToSet(state.Moved), state.LastMove, state.WhiteHand, state.BlackHand)
	if threefold(historyKeys, currentKey) {
		return "draw", "threefold_repetition"
	}

	_, isMate, isStale := gameStatusWithFusion(state.Board, state.Turn, state.LastMove, sliceToSet(state.Moved), state.FortressZones)
	if isMate || isStale {
		hand := state.WhiteHand
		if state.Turn == "black" {
			hand = state.BlackHand
		}
		if len(hand) > 0 {
			return "", ""
		}
	}
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
			// Borrow returns the piece to its owner after one turn, but only if the
			// piece still exists. If the borrowed piece was destroyed by Death (or
			// any other removal effect) while under the borrower's control, it is
			// permanently lost — the owner does not get it back. This is intentional
			// emergent gameplay: Borrow gives tempo at the risk of losing the piece.
			if piece.Borrowed && piece.Color == justMovedColor {
				if state.Board[r][c] != nil {
					piece.Color = opposite(justMovedColor)
					piece.Borrowed = false
				}
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
