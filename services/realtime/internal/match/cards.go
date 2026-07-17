//go:generate node ../../../../packages/game-core/scripts/sync-cards-json.mjs
package match

import (
	"crypto/rand"
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	mrand "math/rand"
	"strings"
	"sync"
	"time"

	"github.com/chess404/realtime/internal/contracts"
)

var (
	cardRand   *mrand.Rand
	cardRandMu sync.Mutex
)

func init() {
	var seedBuf [8]byte
	if _, err := rand.Read(seedBuf[:]); err != nil {
		seedBuf[0] = byte(time.Now().UnixNano())
	}
	cardRand = mrand.New(mrand.NewSource(int64(binary.LittleEndian.Uint64(seedBuf[:]))))
}

//go:embed cards.json
var cardsJSON []byte

var loadCardsOnce sync.Once
var loadedCards []contracts.GameCard

func getStarterCards() []contracts.GameCard {
	loadCardsOnce.Do(func() {
		if err := json.Unmarshal(cardsJSON, &loadedCards); err != nil {
			loadedCards = starterCardsLegacy
		}
	})
	return loadedCards
}

var starterCardsLegacy = []contracts.GameCard{
	{
		ID:       "freeze",
		Name:     "Freeze",
		Mechanic: "freeze",
		Type:     "trap",
		Rarity:   "common",
		Color:    "#1a2e1a",
		Accent:   "#4ade80",
		Icon:     "\U0001F9CA",
		Desc:     "Freeze one enemy piece for 1 turn. Not king.",
	},
	{
		ID:       "shield",
		Name:     "Shield",
		Mechanic: "shield",
		Type:     "trap",
		Rarity:   "common",
		Color:    "#1a2e1a",
		Accent:   "#4ade80",
		Icon:     "\U0001F6E1\uFE0F",
		Desc:     "Protect one of your pieces from capture for 1 turn.",
	},
	{
		ID:       "sniper",
		Name:     "Sniper",
		Mechanic: "sniper",
		Type:     "spell",
		Rarity:   "epic",
		Color:    "#2d1a4a",
		Accent:   "#c084fc",
		Icon:     "\U0001F3AF",
		Desc:     "Remove any enemy piece from the board. Not king.",
	},
	{
		ID:       "badsniper",
		Name:     "Bad Sniper",
		Mechanic: "badsniper",
		Type:     "spell",
		Rarity:   "trash",
		Color:    "#1c1c1c",
		Accent:   "#6b7280",
		Icon:     "\U0001F52B",
		Desc:     "Remove one of your own pieces from the board. Not king.",
	},
	{
		ID:       "promote",
		Name:     "Promote",
		Mechanic: "promote",
		Type:     "spell",
		Rarity:   "common",
		Color:    "#1a2e1a",
		Accent:   "#4ade80",
		Icon:     "\u2B06\uFE0F",
		Desc:     "Upgrade one of your pieces to a stronger type. Not king.",
	},
	{
		ID:       "demote",
		Name:     "Demote",
		Mechanic: "demote",
		Type:     "spell",
		Rarity:   "trash",
		Color:    "#1c1c1c",
		Accent:   "#6b7280",
		Icon:     "\u2B07\uFE0F",
		Desc:     "Lower one of your own pieces to a weaker type. Not king.",
	},
	{
		ID:       "promotehim",
		Name:     "Promote Him",
		Mechanic: "promotehim",
		Type:     "spell",
		Rarity:   "trash",
		Color:    "#1c1c1c",
		Accent:   "#6b7280",
		Icon:     "\U0001F4C8",
		Desc:     "Promote an enemy piece to a stronger type. Not king.",
	},
	{
		ID:       "demotehim",
		Name:     "Demote Him",
		Mechanic: "demotehim",
		Type:     "spell",
		Rarity:   "rare",
		Color:    "#1a2a4a",
		Accent:   "#60a5fa",
		Icon:     "\U0001F4C9",
		Desc:     "Lower any piece to a weaker type. Not king.",
	},
	{
		ID:       "teleport",
		Name:     "Teleport",
		Mechanic: "teleport",
		Type:     "spell",
		Rarity:   "rare",
		Color:    "#1a2a4a",
		Accent:   "#60a5fa",
		Icon:     "\U0001F300",
		Desc:     "Move one of your pieces to any empty square. Not king.",
	},
	{
		ID:       "jump",
		Name:     "Jump",
		Mechanic: "jump",
		Type:     "spell",
		Rarity:   "common",
		Color:    "#1a2e1a",
		Accent:   "#4ade80",
		Icon:     "\U0001F998",
		Desc:     "Jump over exactly one piece using your piece's movement pattern. Not king or knight.",
	},
	{
		ID:       "doublemove_diff",
		Name:     "Double Move (Twin)",
		Mechanic: "doublemove_diff",
		Type:     "spell",
		Rarity:   "rare",
		Color:    "#1a2a4a",
		Accent:   "#60a5fa",
		Icon:     "\U0001F465",
		Desc:     "Move two different pieces this turn.",
	},
	{
		ID:       "doublemove_same",
		Name:     "Double Move (Solo)",
		Mechanic: "doublemove_same",
		Type:     "spell",
		Rarity:   "rare",
		Color:    "#1a2a4a",
		Accent:   "#60a5fa",
		Icon:     "\U0001F3C3",
		Desc:     "Move the same piece twice this turn.",
	},
	{
		ID:       "swapme",
		Name:     "Swap Me",
		Mechanic: "swapme",
		Type:     "spell",
		Rarity:   "common",
		Color:    "#1a2e1a",
		Accent:   "#4ade80",
		Icon:     "\U0001F504",
		Desc:     "Exchange positions of two of your pieces. No check. No king.",
	},
	{
		ID:       "swapus",
		Name:     "Swap Us",
		Mechanic: "swapus",
		Type:     "spell",
		Rarity:   "rare",
		Color:    "#1a2a4a",
		Accent:   "#60a5fa",
		Icon:     "\u2194\uFE0F",
		Desc:     "Swap one of your pieces with one enemy piece. No kings.",
	},
	{
		ID:       "swaphim",
		Name:     "Swap Him",
		Mechanic: "swaphim",
		Type:     "spell",
		Rarity:   "rare",
		Color:    "#1a2a4a",
		Accent:   "#60a5fa",
		Icon:     "\U0001F501",
		Desc:     "Swap two enemy pieces. No kings.",
	},
	{
		ID:       "borrow",
		Name:     "Borrow",
		Mechanic: "borrow",
		Type:     "spell",
		Rarity:   "epic",
		Color:    "#2d1a4a",
		Accent:   "#c084fc",
		Icon:     "\U0001F90F",
		Desc:     "Control one enemy piece for 1 turn. Not king.",
	},
	{
		ID:       "mindcontrol",
		Name:     "Mind Control",
		Mechanic: "mindcontrol",
		Type:     "spell",
		Rarity:   "legendary",
		Color:    "#4a2a00",
		Accent:   "#f59e0b",
		Icon:     "\U0001F9E0",
		Desc:     "Permanently steal one enemy piece. Not king.",
	},
	{
		ID:       "parasite",
		Name:     "Parasite",
		Mechanic: "parasite",
		Type:     "spell",
		Rarity:   "epic",
		Color:    "#2d1a4a",
		Accent:   "#c084fc",
		Icon:     "\U0001F9A0",
		Desc:     "Link your piece to enemy piece. If yours dies, theirs dies too.",
	},
	{
		ID:       "clone",
		Name:     "Clone",
		Mechanic: "clone",
		Type:     "spell",
		Rarity:   "epic",
		Color:    "#2d1a4a",
		Accent:   "#c084fc",
		Icon:     "\U0001F9EC",
		Desc:     "Copy one of your pieces onto an adjacent empty square. Not king.",
	},
	{
		ID:       "lavaground",
		Name:     "Lava Ground",
		Mechanic: "lavaground",
		Type:     "trap",
		Rarity:   "rare",
		Color:    "#1a2a4a",
		Accent:   "#60a5fa",
		Icon:     "\U0001F30B",
		Desc:     "Mark one square. Any piece there next turn is destroyed. Not king.",
	},
	{
		ID:       "fog_village",
		Name:     "Fog Village",
		Mechanic: "fog_village",
		Type:     "spell",
		Rarity:   "common",
		Color:    "#1a2e1a",
		Accent:   "#4ade80",
		Icon:     "\U0001F32B\uFE0F",
		Desc:     "Create a 3x3 fog zone that hides your pieces for 2 turns.",
	},
	{
		ID:       "invisible",
		Name:     "Invisible",
		Mechanic: "invisible",
		Type:     "trap",
		Rarity:   "rare",
		Color:    "#1a2a4a",
		Accent:   "#60a5fa",
		Icon:     "\U0001F441\uFE0F",
		Desc:     "Hide one of your pieces for 1 round. Not king.",
	},
	{
		ID:       "unabomber",
		Name:     "Unabomber",
		Mechanic: "unabomber",
		Type:     "trap",
		Rarity:   "epic",
		Color:    "#2d1a4a",
		Accent:   "#c084fc",
		Icon:     "\U0001F4A3",
		Desc:     "Attach a bomb to one of your pieces. It explodes in 2 turns.",
	},
	{
		ID:       "halffuse",
		Name:     "Half Fuse",
		Mechanic: "halffuse",
		Type:     "spell",
		Rarity:   "rare",
		Color:    "#1a2a4a",
		Accent:   "#60a5fa",
		Icon:     "\u2697\uFE0F",
		Desc:     "Fuse two adjacent own pieces if their combined value is 6 or less.",
	},
	{
		ID:       "fullfusion",
		Name:     "Full Fusion",
		Mechanic: "fullfusion",
		Type:     "spell",
		Rarity:   "legendary",
		Color:    "#4a2a00",
		Accent:   "#f59e0b",
		Icon:     "\U0001F52E",
		Desc:     "Fuse two adjacent own pieces without a value cap.",
	},
	{
		ID:       "fortress",
		Name:     "Fortress",
		Mechanic: "fortress",
		Type:     "spell",
		Rarity:   "epic",
		Color:    "#2d1a4a",
		Accent:   "#c084fc",
		Icon:     "\U0001F3F0",
		Desc:     "Create a 2x2 zone enemies cannot enter for 2 turns.",
	},
	{
		ID:       "reverse",
		Name:     "Reverse",
		Mechanic: "reverse",
		Type:     "trap",
		Rarity:   "epic",
		Color:    "#2d1a4a",
		Accent:   "#c084fc",
		Icon:     "\u23EA",
		Desc:     "Undo opponent's last move.",
	},
	{
		ID:       "undo",
		Name:     "Undo",
		Mechanic: "undo",
		Type:     "trap",
		Rarity:   "epic",
		Color:    "#2d1a4a",
		Accent:   "#c084fc",
		Icon:     "\u21A9\uFE0F",
		Desc:     "Nullify the next card your opponent plays.",
	},
	{
		ID:       "mirror",
		Name:     "Mirror",
		Mechanic: "mirror",
		Type:     "spell",
		Rarity:   "rare",
		Color:    "#142033",
		Accent:   "#93c5fd",
		Icon:     "\U0001FA9E",
		Desc:     "Repeat the last move pattern with one of your matching pieces.",
	},
	{
		ID:       "fakepiece",
		Name:     "Fake Piece",
		Mechanic: "fakepiece",
		Type:     "trap",
		Rarity:   "common",
		Color:    "#1a2e1a",
		Accent:   "#4ade80",
		Icon:     "\U0001F47B",
		Desc:     "Place a fake pawn on an empty square.",
	},
	{
		ID:       "blackhole",
		Name:     "Black Hole",
		Mechanic: "blackhole",
		Type:     "spell",
		Rarity:   "epic",
		Color:    "#2d1a4a",
		Accent:   "#c084fc",
		Icon:     "\U0001F573\uFE0F",
		Desc:     "Choose 2 squares. After 2 turns all adjacent pieces explode. Kings immune.",
	},
	{
		ID:       "smallsacrifice",
		Name:     "Small Sacrifice",
		Mechanic: "smallsacrifice",
		Type:     "spell",
		Rarity:   "rare",
		Color:    "#4a1a1a",
		Accent:   "#f87171",
		Icon:     "\U0001FAF8",
		Desc:     "Sacrifice your own pieces totaling 6+ points to draw 2 cards.",
	},
	{
		ID:       "bigsacrifice",
		Name:     "Big Sacrifice",
		Mechanic: "bigsacrifice",
		Type:     "spell",
		Rarity:   "epic",
		Color:    "#4a1a1a",
		Accent:   "#fb7185",
		Icon:     "\U0001F48E",
		Desc:     "Sacrifice your own pieces totaling 14+ points to draw 3 cards.",
	},
	{
		ID:       "gambler",
		Name:     "Gambler",
		Mechanic: "gambler",
		Type:     "spell",
		Rarity:   "trash",
		Color:    "#1c1c1c",
		Accent:   "#6b7280",
		Icon:     "\U0001F3B2",
		Desc:     "50% steal a card from opponent. 50% give one of yours away.",
	},
	{
		ID:       "radar",
		Name:     "Radar",
		Mechanic: "radar",
		Type:     "spell",
		Rarity:   "common",
		Color:    "#1a2a4a",
		Accent:   "#60a5fa",
		Icon:     "\U0001F4E1",
		Desc:     "Reveal the enemy hand for the rest of your turn.",
	},
	{
		ID:       "oracle",
		Name:     "Oracle",
		Mechanic: "cheater",
		Type:     "spell",
		Rarity:   "trash",
		Color:    "#1c1c1c",
		Accent:   "#f59e0b",
		Icon:     "\U0001F4A1",
		Desc:     "Show engine help for this turn and your next two turns.",
	},
	{
		ID:       "joker",
		Name:     "Joker",
		Mechanic: "joker",
		Type:     "spell",
		Rarity:   "rare",
		Color:    "#4a2a00",
		Accent:   "#f59e0b",
		Icon:     "\U0001F0CF",
		Desc:     "Choose any backend-supported card from the full pool instantly.",
	},
}

const starterHandModeStarterThree = "starter_three"

func cardTemplateByMechanic(mechanic string) contracts.GameCard {
	for _, card := range getStarterCards() {
		if card.Mechanic == mechanic {
			return card
		}
	}
	return contracts.GameCard{}
}

func starterHandCardsForMode(mode string) []contracts.GameCard {
	if strings.EqualFold(strings.TrimSpace(mode), starterHandModeStarterThree) {
		return []contracts.GameCard{
			cardTemplateByMechanic("freeze"),
			cardTemplateByMechanic("shield"),
			cardTemplateByMechanic("smallsacrifice"),
		}
	}
	return getStarterCards()
}

func cloneCardsWithOwner(cards []contracts.GameCard, owner string) []contracts.GameCard {
	out := make([]contracts.GameCard, 0, len(cards))
	for index, card := range cards {
		next := card
		next.ID = fmt.Sprintf("%s_%d_%s", card.ID, index+1, owner)
		out = append(out, next)
	}
	return out
}

func cardFromHand(state *contracts.MatchState, owner string, cardID string) (contracts.GameCard, bool) {
	hand := state.WhiteHand
	if owner == "black" {
		hand = state.BlackHand
	}
	for _, card := range hand {
		if card.ID == cardID {
			return card, true
		}
	}
	return contracts.GameCard{}, false
}

func removeCardFromHand(state *contracts.MatchState, owner string, cardID string) {
	hand := state.WhiteHand
	if owner == "black" {
		hand = state.BlackHand
	}
	filtered := make([]contracts.GameCard, 0, len(hand))
	for _, card := range hand {
		if card.ID != cardID {
			filtered = append(filtered, card)
		}
	}
	if owner == "black" {
		state.BlackHand = filtered
	} else {
		state.WhiteHand = filtered
	}
}

func addRewardCards(state *contracts.MatchState, owner string, count int, now time.Time) []contracts.GameCard {
	var hand *[]contracts.GameCard
	if owner == "black" {
		hand = &state.BlackHand
	} else {
		hand = &state.WhiteHand
	}

	drawn := make([]contracts.GameCard, 0, count)
	for i := 0; i < count; i++ {
		template := rewardTemplateForState(state, len(drawn))
		next := template
		next.ID = fmt.Sprintf("%s_reward_%d_%d_%s", template.ID, now.UnixMilli(), i+1, owner)
		*hand = append(*hand, next)
		drawn = append(drawn, next)
	}
	return drawn
}

func drawRoundCards(state *contracts.MatchState, now time.Time) (white []contracts.GameCard, black []contracts.GameCard, whiteSkipped bool, blackSkipped bool) {
	if state.Turn != "white" || state.FullMoveNum < drawFromRound {
		return nil, nil, false, false
	}
	if (state.FullMoveNum-drawFromRound)%drawEveryRounds != 0 {
		return nil, nil, false, false
	}
	if len(state.WhiteHand) >= maxHandSize {
		whiteSkipped = true
	} else {
		white = addRewardCards(state, "white", 1, now)
	}
	if len(state.BlackHand) >= maxHandSize {
		blackSkipped = true
	} else {
		black = addRewardCards(state, "black", 1, now)
	}
	return white, black, whiteSkipped, blackSkipped
}

func rewardTemplateForState(state *contracts.MatchState, offset int) contracts.GameCard {
	index := deterministicCardIndex(state, offset)
	if index < 0 {
		index = 0
	}
	pool := getStarterCards()
	return pool[index]
}

func deterministicCardIndex(state *contracts.MatchState, offset int) int {
	pool := getStarterCards()
	cardRandMu.Lock()
	idx := cardRand.Intn(len(pool))
	cardRandMu.Unlock()
	return idx
}

func parseSquareOptions(options []string) []contracts.Square {
	selected := make([]contracts.Square, 0, len(options))
	for _, option := range options {
		if sq, ok := parseParasiteSquare(option); ok {
			selected = append(selected, sq)
		}
	}
	return selected
}

func encodeSquareOptions(squares []contracts.Square) []string {
	options := make([]string, 0, len(squares))
	for _, sq := range squares {
		options = append(options, fmt.Sprintf("%d,%d", sq.Row, sq.Col))
	}
	return options
}

func toggleSquareInList(values []contracts.Square, target contracts.Square) []contracts.Square {
	next := make([]contracts.Square, 0, len(values))
	removed := false
	for _, value := range values {
		if value.Row == target.Row && value.Col == target.Col {
			removed = true
			continue
		}
		next = append(next, value)
	}
	if !removed {
		next = append(next, target)
	}
	return next
}

func selectedSquaresValue(board [][]*contracts.Piece, selected []contracts.Square) int {
	total := 0
	for _, sq := range selected {
		piece := pieceAt(board, sq)
		if piece == nil {
			continue
		}
		total += pieceValue(piece.Type)
	}
	return total
}

func addCardToHand(state *contracts.MatchState, owner string, card contracts.GameCard) bool {
	if owner == "black" {
		if len(state.BlackHand) >= maxHandSize {
			return false
		}
		state.BlackHand = append(state.BlackHand, card)
		return true
	}
	if len(state.WhiteHand) >= maxHandSize {
		return false
	}
	state.WhiteHand = append(state.WhiteHand, card)
	return true
}

func filterCardsNotMechanic(hand []contracts.GameCard, mechanic string) []contracts.GameCard {
	filtered := make([]contracts.GameCard, 0, len(hand))
	for _, card := range hand {
		if card.Mechanic != mechanic {
			filtered = append(filtered, card)
		}
	}
	return filtered
}

func cloneEvents(events []contracts.ResolvedEvent) []contracts.ResolvedEvent {
	cloned := make([]contracts.ResolvedEvent, 0, len(events))
	for _, event := range events {
		next := event
		next.Payload = copyPayload(event.Payload)
		cloned = append(cloned, next)
	}
	return cloned
}

func copyPayload(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func cloneBoard(board [][]*contracts.Piece) [][]*contracts.Piece {
	next := make([][]*contracts.Piece, len(board))
	for r := range board {
		next[r] = make([]*contracts.Piece, len(board[r]))
		for c := range board[r] {
			if board[r][c] == nil {
				continue
			}
			piece := *board[r][c]
			next[r][c] = &piece
		}
	}
	return next
}

func jokerTransformOptions() []string {
	pool := getStarterCards()
	options := make([]string, 0, len(pool))
	for _, card := range pool {
		if card.Mechanic == "joker" {
			continue
		}
		options = append(options, card.Mechanic)
	}
	return options
}

func starterCardTemplate(mechanic string) (contracts.GameCard, bool) {
	for _, card := range getStarterCards() {
		if card.Mechanic == mechanic {
			return card, true
		}
	}
	return contracts.GameCard{}, false
}
