package engine

import (
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/chess404/realtime/internal/contracts"
)

type Difficulty int

const (
	DifficultyBeginner Difficulty = iota
	DifficultyEasy
	DifficultyMedium
	DifficultyHard
	DifficultyExpert
)

func ParseDifficulty(s string) Difficulty {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "beginner":
		return DifficultyBeginner
	case "easy":
		return DifficultyEasy
	case "hard":
		return DifficultyHard
	case "expert":
		return DifficultyExpert
	default:
		return DifficultyMedium
	}
}

func (d Difficulty) SearchDepth() int {
	switch d {
	case DifficultyBeginner:
		return 2
	case DifficultyEasy:
		return 3
	case DifficultyMedium:
		return 4
	case DifficultyHard:
		return 6
	case DifficultyExpert:
		return 8
	default:
		return 4
	}
}

func (d Difficulty) ShouldPlayCard(card contracts.GameCard, score int) bool {
	switch d {
	case DifficultyBeginner:
		return score >= 60 && rand.Float64() < 0.3
	case DifficultyEasy:
		return score >= 50 && rand.Float64() < 0.5
	case DifficultyMedium:
		return score >= 40
	case DifficultyHard:
		return score >= 30
	case DifficultyExpert:
		return score >= 20
	default:
		return score >= 40
	}
}

type ComputerOpponent struct {
	Difficulty Difficulty
	Color      string
	rng        *rand.Rand
	tt         *TranspositionTable
	cardEval   *CardEvaluator
	mu         sync.Mutex
}

func NewComputerOpponent(difficulty Difficulty, color string) *ComputerOpponent {
	seed := time.Now().UnixNano()
	return &ComputerOpponent{
		Difficulty: difficulty,
		Color:      color,
		rng:        rand.New(rand.NewSource(seed)),
		tt:         NewTranspositionTable(1 << 16),
		cardEval:   NewCardEvaluator(rand.New(rand.NewSource(seed + 1))),
	}
}

func (co *ComputerOpponent) MakeMove(state *contracts.MatchState) *contracts.PlayerIntent {
	co.mu.Lock()
	defer co.mu.Unlock()

	if state.Status != "active" {
		return nil
	}

	if co.cardEval.ShouldPlayCard(state, co.Color == "white") {
		play := co.cardEval.BestCardToPlay(state, co.Color == "white")
		if play != nil && co.Difficulty.ShouldPlayCard(play.Card, play.Score) {
			return &contracts.PlayerIntent{
				Type:     "play_card",
				MatchID:  state.MatchID,
				CardID:   play.Card.ID,
			}
		}
	}

	// Probe opening book first (only in opening phase).
	if state.FullMoveNum <= 10 {
		bookMove := defaultBook.Probe(state, co.Color == "white")
		if bookMove != nil && bookMove.From != bookMove.To {
			return &contracts.PlayerIntent{
				Type:    "make_move",
				MatchID: state.MatchID,
				From:    &bookMove.From,
				To:      &bookMove.To,
			}
		}
	}

	searchDepth := co.Difficulty.SearchDepth()
	result := Search(state, searchDepth, co.tt)

	if result.BestMove.From == (contracts.Square{}) && result.BestMove.To == (contracts.Square{}) {
		return nil
	}

	intent := &contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  state.MatchID,
		From:     &result.BestMove.From,
		To:       &result.BestMove.To,
	}

	return intent
}

func (co *ComputerOpponent) HandleSelectTarget(state *contracts.MatchState) *contracts.PlayerIntent {
	if state.PendingCard == nil {
		return nil
	}

	mechanic := state.PendingCard.Mechanic
	ownerColor := state.PendingCard.OwnerColor

	if mechanic == "freeze" || mechanic == "sniper" || mechanic == "badsniper" ||
		mechanic == "demote" || mechanic == "demotehim" || mechanic == "promote" ||
		mechanic == "promotehim" || mechanic == "mindcontrol" || mechanic == "borrow" ||
		mechanic == "parasite" || mechanic == "lavaground" || mechanic == "clone" ||
		mechanic == "invisible" || mechanic == "fakepiece" {

		target := co.findBestTarget(state, mechanic, ownerColor)
		if target != nil {
			return &contracts.PlayerIntent{
				Type:        "select_target",
				MatchID:     state.MatchID,
				SelectionID: targetSelectionID(mechanic),
				Target:      target,
			}
		}
	}

	if mechanic == "swapme" || mechanic == "swapus" || mechanic == "swaphim" || mechanic == "halffuse" || mechanic == "fullfusion" {
		if state.PendingCard.Options != nil && len(state.PendingCard.Options) > 0 {
			return &contracts.PlayerIntent{
				Type:        "select_target",
				MatchID:     state.MatchID,
				SelectionID: state.PendingCard.Options[0],
			}
		}
	}

	return nil
}

func (co *ComputerOpponent) findBestTarget(state *contracts.MatchState, mechanic, ownerColor string) *contracts.Square {
	opponent := oppositeColor(ownerColor)

	switch mechanic {
	case "freeze", "sniper", "badsniper", "demotehim", "mindcontrol", "borrow", "parasite":
		bestValue := 0
		var bestSquare *contracts.Square
		for r := 0; r < 8; r++ {
			for c := 0; c < 8; c++ {
				piece := state.Board[r][c]
				if piece == nil || piece.Color != opponent || piece.Type == "king" {
					continue
				}
				value := pieceValue(piece.Type)
				if value > bestValue {
					bestValue = value
					sq := contracts.Square{Row: r, Col: c}
					bestSquare = &sq
				}
			}
		}
		return bestSquare

	case "demote", "promotehim":
		bestValue := 0
		var bestSquare *contracts.Square
		for r := 0; r < 8; r++ {
			for c := 0; c < 8; c++ {
				piece := state.Board[r][c]
				if piece == nil || piece.Type == "king" || piece.Type == "pawn" {
					continue
				}
				value := pieceValue(piece.Type)
				if piece.Color == opponent && value > bestValue {
					bestValue = value
					sq := contracts.Square{Row: r, Col: c}
					bestSquare = &sq
				}
			}
		}
		return bestSquare

	case "promote":
		bestValue := 0
		var bestSquare *contracts.Square
		for r := 0; r < 8; r++ {
			for c := 0; c < 8; c++ {
				piece := state.Board[r][c]
				if piece == nil || piece.Color != ownerColor || piece.Type == "king" || piece.Type == "queen" {
					continue
				}
				value := pieceValue(piece.Type)
				if value > bestValue {
					bestValue = value
					sq := contracts.Square{Row: r, Col: c}
					bestSquare = &sq
				}
			}
		}
		return bestSquare

	case "clone":
		bestValue := 0
		var bestSquare *contracts.Square
		for r := 0; r < 8; r++ {
			for c := 0; c < 8; c++ {
				piece := state.Board[r][c]
				if piece == nil || piece.Color != ownerColor || piece.Type == "king" {
					continue
				}
				value := pieceValue(piece.Type)
				if value > bestValue {
					bestValue = value
					sq := contracts.Square{Row: r, Col: c}
					bestSquare = &sq
				}
			}
		}
		return bestSquare

	case "invisible":
		bestValue := 0
		var bestSquare *contracts.Square
		for r := 0; r < 8; r++ {
			for c := 0; c < 8; c++ {
				piece := state.Board[r][c]
				if piece == nil || piece.Color != ownerColor || piece.Type == "king" || piece.Invisible {
					continue
				}
				value := pieceValue(piece.Type)
				if value > bestValue {
					bestValue = value
					sq := contracts.Square{Row: r, Col: c}
					bestSquare = &sq
				}
			}
		}
		return bestSquare
	}

	return nil
}

func targetSelectionID(mechanic string) string {
	switch mechanic {
	case "freeze":
		return "freeze_target"
	case "sniper", "badsniper":
		return "sniper_target"
	case "demote", "demotehim":
		return "demote_target"
	case "promote", "promotehim":
		return "promote_target"
	case "mindcontrol", "borrow":
		return "mindcontrol_target"
	case "clone":
		return "clone_source"
	case "invisible":
		return "invisible_source"
	case "lavaground":
		return "lavaground_target"
	case "parasite":
		return "parasite_target"
	case "fakepiece":
		return "fakepiece_target"
	default:
		return "target"
	}
}
