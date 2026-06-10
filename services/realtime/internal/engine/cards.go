package engine

import (
	"math/rand"

	"github.com/chess404/realtime/internal/contracts"
)

type CardEvaluator struct {
	rng *rand.Rand
}

func NewCardEvaluator(rng *rand.Rand) *CardEvaluator {
	return &CardEvaluator{rng: rng}
}

type CardPlay struct {
	Card     contracts.GameCard
	Score    int
	Target   *contracts.Square
	Decision string
}

func (ce *CardEvaluator) EvaluateHand(hand []contracts.GameCard, state *contracts.MatchState, isWhite bool) []CardPlay {
	plays := make([]CardPlay, 0, len(hand))
	for _, card := range hand {
		play := ce.evaluateCard(card, state, isWhite)
		plays = append(plays, play)
	}
	return plays
}

func (ce *CardEvaluator) evaluateCard(card contracts.GameCard, state *contracts.MatchState, isWhite bool) CardPlay {
	score := 0
	color := "black"
	if isWhite {
		color = "white"
	}

	switch card.Mechanic {
	case "freeze":
		score = ce.freezeValue(state, color)
	case "shield":
		score = ce.shieldValue(state, color)
	case "sniper":
		score = ce.sniperValue(state, color)
	case "badsniper":
		score = -ce.sniperValue(state, color) / 2
	case "teleport":
		score = 40
	case "jump":
		score = 30
	case "swapme":
		score = ce.swapMeValue(state, color)
	case "swapus":
		score = ce.swapUsValue(state, color)
	case "swaphim":
		score = ce.swapHimValue(state, color)
	case "clone":
		score = 50
	case "halffuse":
		score = 60
	case "fullfusion":
		score = 80
	case "doublemove_same", "doublemove_diff":
		score = 70
	case "promote":
		score = ce.promoteValue(state, color)
	case "demote":
		score = ce.demoteValue(state, color)
	case "demotehim":
		score = ce.demoteValue(state, oppositeColor(color))
	case "promotehim":
		score = -30
	case "mindcontrol":
		score = 100
	case "borrow":
		score = 70
	case "reverse":
		score = ce.reverseValue(state, color)
	case "undo":
		score = 50
	case "mirror":
		score = 40
	case "invisible":
		score = 45
	case "lavaground":
		score = 35
	case "fog_village":
		score = 30
	case "fortress":
		score = 55
	case "radar":
		score = 25
	case "unabomber":
		score = 60
	case "blackhole":
		score = 65
	case "parasite":
		score = 55
	case "fakepiece":
		score = 20
	case "cheater":
		score = 30
	case "gambler":
		score = 10
	case "smallsacrifice":
		score = ce.sacrificeValue(state, color, 6)
	case "bigsacrifice":
		score = ce.sacrificeValue(state, color, 14)
	case "joker":
		score = 75
	}

	return CardPlay{
		Card:  card,
		Score: score,
	}
}

func (ce *CardEvaluator) ShouldPlayCard(state *contracts.MatchState, isWhite bool) bool {
	if state.Status != "active" {
		return false
	}

	hand := state.BlackHand
	if isWhite {
		hand = state.WhiteHand
	}
	if len(hand) == 0 {
		return false
	}

	plays := ce.EvaluateHand(hand, state, isWhite)
	if len(plays) == 0 {
		return false
	}

	bestScore := plays[0].Score
	for _, p := range plays[1:] {
		if p.Score > bestScore {
			bestScore = p.Score
		}
	}

	if bestScore >= 50 {
		return true
	}

	if len(hand) >= 5 && bestScore >= 30 {
		return true
	}

	return false
}

func (ce *CardEvaluator) BestCardToPlay(state *contracts.MatchState, isWhite bool) *CardPlay {
	hand := state.BlackHand
	if isWhite {
		hand = state.WhiteHand
	}
	if len(hand) == 0 {
		return nil
	}

	plays := ce.EvaluateHand(hand, state, isWhite)
	if len(plays) == 0 {
		return nil
	}

	best := &plays[0]
	for i := range plays[1:] {
		if plays[i+1].Score > best.Score {
			best = &plays[i+1]
		}
	}

	if best.Score < 20 {
		return nil
	}

	return best
}

func (ce *CardEvaluator) freezeValue(state *contracts.MatchState, color string) int {
	opponent := oppositeColor(color)
	bestValue := 0
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			piece := state.Board[r][c]
			if piece != nil && piece.Color == opponent && piece.Type != "king" && !piece.Frozen {
				value := pieceValue(piece.Type)
				if value > bestValue {
					bestValue = value
				}
			}
		}
	}
	return bestValue / 5
}

func (ce *CardEvaluator) shieldValue(state *contracts.MatchState, color string) int {
	bestValue := 0
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			piece := state.Board[r][c]
			if piece != nil && piece.Color == color && piece.Type != "king" && !piece.Shielded {
				value := pieceValue(piece.Type)
				if value > bestValue {
					bestValue = value
				}
			}
		}
	}
	return bestValue / 4
}

func (ce *CardEvaluator) sniperValue(state *contracts.MatchState, color string) int {
	opponent := oppositeColor(color)
	bestValue := 0
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			piece := state.Board[r][c]
			if piece != nil && piece.Color == opponent && piece.Type != "king" {
				value := pieceValue(piece.Type)
				if value > bestValue {
					bestValue = value
				}
			}
		}
	}
	return bestValue / 3
}

func (ce *CardEvaluator) swapMeValue(state *contracts.MatchState, color string) int {
	bestDiff := 0
	for r1 := 0; r1 < 8; r1++ {
		for c1 := 0; c1 < 8; c1++ {
			p1 := state.Board[r1][c1]
			if p1 == nil || p1.Color != color || p1.Type == "king" {
				continue
			}
			for r2 := 0; r2 < 8; r2++ {
				for c2 := 0; c2 < 8; c2++ {
					p2 := state.Board[r2][c2]
					if p2 == nil || p2.Color != color || p2.Type == "king" {
						continue
					}
					if r1 == r2 && c1 == c2 {
						continue
					}
					diff := positionalBonus(p2, r1, c1, false) - positionalBonus(p2, r2, c2, false)
					if diff > bestDiff {
						bestDiff = diff
					}
				}
			}
		}
	}
	return bestDiff / 2
}

func (ce *CardEvaluator) swapUsValue(state *contracts.MatchState, color string) int {
	opponent := oppositeColor(color)
	bestDiff := 0
	for r1 := 0; r1 < 8; r1++ {
		for c1 := 0; c1 < 8; c1++ {
			p1 := state.Board[r1][c1]
			if p1 == nil || p1.Color != color || p1.Type == "king" {
				continue
			}
			for r2 := 0; r2 < 8; r2++ {
				for c2 := 0; c2 < 8; c2++ {
					p2 := state.Board[r2][c2]
					if p2 == nil || p2.Color != opponent || p2.Type == "king" {
						continue
					}
					diff := pieceValue(p2.Type) - pieceValue(p1.Type)
					if diff > bestDiff {
						bestDiff = diff
					}
				}
			}
		}
	}
	return bestDiff / 2
}

func (ce *CardEvaluator) swapHimValue(state *contracts.MatchState, color string) int {
	opponent := oppositeColor(color)
	bestDiff := 0
	for r1 := 0; r1 < 8; r1++ {
		for c1 := 0; c1 < 8; c1++ {
			p1 := state.Board[r1][c1]
			if p1 == nil || p1.Color != opponent || p1.Type == "king" {
				continue
			}
			for r2 := 0; r2 < 8; r2++ {
				for c2 := 0; c2 < 8; c2++ {
					p2 := state.Board[r2][c2]
					if p2 == nil || p2.Color != opponent || p2.Type == "king" {
						continue
					}
					if r1 == r2 && c1 == c2 {
						continue
					}
					diff := positionalBonus(p2, r1, c1, false) - positionalBonus(p2, r2, c2, false)
					if diff > bestDiff {
						bestDiff = diff
					}
				}
			}
		}
	}
	return bestDiff / 3
}

func (ce *CardEvaluator) promoteValue(state *contracts.MatchState, color string) int {
	bestValue := 0
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			piece := state.Board[r][c]
			if piece != nil && piece.Color == color && piece.Type != "king" && piece.Type != "queen" {
				value := pieceValue(piece.Type)
				if value > bestValue {
					bestValue = value
				}
			}
		}
	}
	return bestValue / 3
}

func (ce *CardEvaluator) demoteValue(state *contracts.MatchState, color string) int {
	bestValue := 0
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			piece := state.Board[r][c]
			if piece != nil && piece.Color == color && piece.Type != "king" && piece.Type != "pawn" {
				value := pieceValue(piece.Type)
				if value > bestValue {
					bestValue = value
				}
			}
		}
	}
	return bestValue / 3
}

func (ce *CardEvaluator) reverseValue(state *contracts.MatchState, color string) int {
	if state.LastMove == nil {
		return 0
	}
	piece := state.Board[state.LastMove.To.Row][state.LastMove.To.Col]
	if piece == nil || piece.Color == color {
		return 0
	}
	return pieceValue(piece.Type) / 2
}

func (ce *CardEvaluator) sacrificeValue(state *contracts.MatchState, color string, threshold int) int {
	totalValue := 0
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			piece := state.Board[r][c]
			if piece != nil && piece.Color == color && piece.Type != "king" {
				totalValue += pieceValue(piece.Type)
			}
		}
	}
	if totalValue >= threshold*100 {
		return 50
	}
	return 0
}
