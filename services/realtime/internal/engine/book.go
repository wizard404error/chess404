package engine

import (
	"encoding/json"
	"math/rand"
	"os"

	"github.com/chess404/realtime/internal/contracts"
)

type BookEntry struct {
	Move   string  `json:"move"`   // UCI notation
	Weight float64 `json:"weight"` // selection weight (higher = more likely)
}

type OpeningBook struct {
	entries map[uint64][]BookEntry // Zobrist hash → possible moves
	rng     *rand.Rand
}

var defaultBook *OpeningBook

func init() {
	defaultBook = NewOpeningBook()
	if err := defaultBook.Load("opening_book.json"); err != nil {
		// No book file — engine plays without opening book.
	}
}

func NewOpeningBook() *OpeningBook {
	return &OpeningBook{
		entries: make(map[uint64][]BookEntry),
		rng:     rand.New(rand.NewSource(42)),
	}
}

func (b *OpeningBook) Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	// The JSON stores keys as decimal strings of the uint64 hash.
	raw := make(map[string][]BookEntry)
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	b.entries = make(map[uint64][]BookEntry, len(raw))
	for keyStr, entries := range raw {
		hash := hashStringToUint64(keyStr)
		b.entries[hash] = entries
	}
	return nil
}

// Probe returns the best move for a given position from the opening book.
// Returns nil if the position is not in the book.
func (b *OpeningBook) Probe(state *contracts.MatchState, isWhite bool) *Move {
	if len(b.entries) == 0 {
		return nil
	}

	hash := defaultHasher.Hash(state)
	entries, ok := b.entries[hash]
	if !ok || len(entries) == 0 {
		return nil
	}

	totalWeight := 0.0
	for _, e := range entries {
		totalWeight += e.Weight
	}
	if totalWeight <= 0 {
		return nil
	}

	r := b.rng.Float64() * totalWeight
	cumulative := 0.0
	for _, e := range entries {
		cumulative += e.Weight
		if r <= cumulative {
			return uciToMove(e.Move)
		}
	}

	return uciToMove(entries[len(entries)-1].Move)
}

func uciToMove(uci string) *Move {
	if len(uci) < 4 {
		return nil
	}
	fromCol := int(uci[0] - 'a')
	fromRow := int(uci[1] - '1')
	toCol := int(uci[2] - 'a')
	toRow := int(uci[3] - '1')

	m := &Move{
		From: contracts.Square{Row: fromRow, Col: fromCol},
		To:   contracts.Square{Row: toRow, Col: toCol},
	}
	if len(uci) == 5 {
		promo := uci[4]
		switch promo {
		case 'q':
			m.Promotion = "queen"
		case 'r':
			m.Promotion = "rook"
		case 'b':
			m.Promotion = "bishop"
		case 'n':
			m.Promotion = "knight"
		}
	}
	return m
}

func hashStringToUint64(s string) uint64 {
	var h uint64
	for i := 0; i < len(s); i++ {
		h = h*131 + uint64(s[i])
	}
	return h
}
