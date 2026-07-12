package engine

import (
	"math/rand"

	"github.com/chess404/realtime/internal/contracts"
)

// Zobrist hashing for the transposition table. We need random 64-bit values
// for every (piece-type × color × square) combination, plus side-to-move,
// castling rights, and en-passant file. The hash is XOR-ed incrementally
// so that apply/unapply is O(1).

type ZobristHasher struct {
	pieceKeys [12][64]uint64  // [pieceIndex][squareIndex]
	sideKey   uint64
	epKeys    [8]uint64       // [file]
	castleKeys [4]uint64      // KQkq order
}

// pieceIndex maps type+color to 0..11.
var pieceIndex = map[[2]string]int{
	{"pawn", "white"}:   0,
	{"pawn", "black"}:   1,
	{"knight", "white"}: 2,
	{"knight", "black"}: 3,
	{"bishop", "white"}: 4,
	{"bishop", "black"}: 5,
	{"rook", "white"}:   6,
	{"rook", "black"}:   7,
	{"queen", "white"}:  8,
	{"queen", "black"}:  9,
	{"king", "white"}:   10,
	{"king", "black"}:   11,
}

func NewZobristHasher(rng *rand.Rand) *ZobristHasher {
	z := &ZobristHasher{}
	for i := 0; i < 12; i++ {
		for j := 0; j < 64; j++ {
			z.pieceKeys[i][j] = rng.Uint64()
		}
	}
	z.sideKey = rng.Uint64()
	for i := 0; i < 8; i++ {
		z.epKeys[i] = rng.Uint64()
	}
	for i := 0; i < 4; i++ {
		z.castleKeys[i] = rng.Uint64()
	}
	return z
}

// Hash computes the full Zobrist hash for a given MatchState.
func (z *ZobristHasher) Hash(state *contracts.MatchState) uint64 {
	var h uint64
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			p := state.Board[r][c]
			if p == nil {
				continue
			}
			if idx, ok := pieceIndex[[2]string{p.Type, p.Color}]; ok {
				h ^= z.pieceKeys[idx][r*8+c]
			}
		}
	}
	if state.Turn == "black" {
		h ^= z.sideKey
	}
	if state.LastMove != nil {
		lastPiece := state.Board[state.LastMove.To.Row][state.LastMove.To.Col]
		if lastPiece != nil && lastPiece.Type == "pawn" && abs(state.LastMove.From.Row-state.LastMove.To.Row) == 2 {
			epFile := state.LastMove.To.Col
			h ^= z.epKeys[epFile]
		}
	}
	movedSet := sliceToSet(state.Moved)
	if _, k := movedSet["0-4"]; !k {
		if _, r := movedSet["0-7"]; !r {
			h ^= z.castleKeys[0] // K
		}
		if _, r := movedSet["0-0"]; !r {
			h ^= z.castleKeys[1] // Q
		}
	}
	if _, k := movedSet["7-4"]; !k {
		if _, r := movedSet["7-7"]; !r {
			h ^= z.castleKeys[2] // k
		}
		if _, r := movedSet["7-0"]; !r {
			h ^= z.castleKeys[3] // q
		}
	}
	return h
}
