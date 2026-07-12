package engine

import (
	"encoding/binary"
	"math"
	"os"

	"github.com/chess404/realtime/internal/contracts"
)

type NNUE struct {
	// Input layer: piece-square encoding (12 piece types * 64 squares) + modifier flags
	InputSize  int
	HiddenSize int
	// Weights[0] = input→hidden (InputSize × HiddenSize)
	// Biases[0] = hidden bias (HiddenSize)
	// Weights[1] = hidden→output (HiddenSize × 1)
	// Biases[1] = output bias (1)
	Weights [][]float32
	Biases  [][]float32
	loaded  bool
}

const (
	nnuePieceTypes = 12  // 6 piece types * 2 colors
	nnueSquares    = 64  // 8×8 board
	nnueModifiers  = 5   // lava, bomb, fortress, fog, blackhole
	nnueInputSize  = nnuePieceTypes*nnueSquares + nnueModifiers
	nnueHiddenSize = 256
)

var defaultNNUE *NNUE

func init() {
	defaultNNUE = &NNUE{}
	if err := defaultNNUE.Load("nnue_weights.bin"); err != nil {
		// No weights file — NNUE stays unloaded, eval falls back to hand-crafted.
	}
}

func NewNNUE() *NNUE {
	return &NNUE{
		InputSize:  nnueInputSize,
		HiddenSize: nnueHiddenSize,
	}
}

func (n *NNUE) Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data) < 8 {
		return os.ErrClosed
	}
	inputSize := int(binary.LittleEndian.Uint32(data[0:4]))
	hiddenSize := int(binary.LittleEndian.Uint32(data[4:8]))
	if inputSize != nnueInputSize || hiddenSize != nnueHiddenSize {
		inputSize = nnueInputSize
		hiddenSize = nnueHiddenSize
	}
	n.InputSize = inputSize
	n.HiddenSize = hiddenSize

	offset := 8
	n.Weights = make([][]float32, 2)
	n.Biases = make([][]float32, 2)

	n.Weights[0] = make([]float32, inputSize*hiddenSize)
	n.Biases[0] = make([]float32, hiddenSize)
	n.Weights[1] = make([]float32, hiddenSize)
	n.Biases[1] = make([]float32, 1)

	expected := inputSize*hiddenSize*4 + hiddenSize*4 + hiddenSize*4 + 4
	if len(data)-offset < expected {
		return os.ErrClosed
	}
	for i := range n.Weights[0] {
		n.Weights[0][i] = math.Float32frombits(binary.LittleEndian.Uint32(data[offset:]))
		offset += 4
	}
	for i := range n.Biases[0] {
		n.Biases[0][i] = math.Float32frombits(binary.LittleEndian.Uint32(data[offset:]))
		offset += 4
	}
	for i := range n.Weights[1] {
		n.Weights[1][i] = math.Float32frombits(binary.LittleEndian.Uint32(data[offset:]))
		offset += 4
	}
	for i := range n.Biases[1] {
		n.Biases[1][i] = math.Float32frombits(binary.LittleEndian.Uint32(data[offset:]))
		offset += 4
	}
	n.loaded = true
	return nil
}

// Evaluate returns the NNUE evaluation from white's perspective.
func (n *NNUE) Evaluate(board [][]*contracts.Piece, lavas []contracts.LavaSquare, fortresses []contracts.FortressZone, bombs []contracts.BombPiece, fogs []contracts.FogZone, blackHoles []contracts.BlackHoleZone) int {
	if !n.loaded {
		return 0
	}
	input := make([]float32, nnueInputSize)
	n.encodeBoard(board, input)
	n.encodeModifiers(lavas, fortresses, bombs, fogs, blackHoles, input)

	hidden := make([]float32, n.HiddenSize)
	for j := range hidden {
		var sum float32
		base := j * n.InputSize
		for i := 0; i < n.InputSize; i++ {
			if input[i] != 0 {
				sum += n.Weights[0][base+i] * input[i]
			}
		}
		sum += n.Biases[0][j]
		if sum < 0 {
			sum *= 0.1 // ClippedReLU: leaky slope for negatives
		}
		hidden[j] = sum
	}

	var output float32
	for j := range hidden {
		if hidden[j] != 0 {
			output += n.Weights[1][j] * hidden[j]
		}
	}
	output += n.Biases[1][0]

	return int(output * 100) // Scale to centipawns
}

// Loaded returns true if NNUE weights have been loaded.
func (n *NNUE) Loaded() bool { return n.loaded }

// encodeBoard fills the sparse piece-square input features.
func (n *NNUE) encodeBoard(board [][]*contracts.Piece, input []float32) {
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			p := board[r][c]
			if p == nil {
				continue
			}
			colorIdx := 0
			if p.Color == "black" {
				colorIdx = 1
			}
			typeIdx := pieceNNUEIndex(p.Type)
			pieceSq := (colorIdx*6 + typeIdx) * 64
			sq := r*8 + c
			if pieceSq+sq < nnuePieceTypes*nnueSquares {
				input[pieceSq+sq] = 1.0
			}
		}
	}
}

// encodeModifiers fills the board-modifier input features.
func (n *NNUE) encodeModifiers(lavas []contracts.LavaSquare, fortresses []contracts.FortressZone, bombs []contracts.BombPiece, fogs []contracts.FogZone, blackHoles []contracts.BlackHoleZone, input []float32) {
	modOffset := nnuePieceTypes * nnueSquares
	if len(lavas) > 0 && modOffset < nnueInputSize {
		input[modOffset] = 1.0
	}
	modOffset++
	if len(bombs) > 0 && modOffset < nnueInputSize {
		input[modOffset] = 1.0
	}
	modOffset++
	if len(fortresses) > 0 && modOffset < nnueInputSize {
		input[modOffset] = 1.0
	}
	modOffset++
	if len(fogs) > 0 && modOffset < nnueInputSize {
		input[modOffset] = 1.0
	}
	modOffset++
	if len(blackHoles) > 0 && modOffset < nnueInputSize {
		input[modOffset] = 1.0
	}
}

func pieceNNUEIndex(ptype string) int {
	switch ptype {
	case "pawn":
		return 0
	case "knight":
		return 1
	case "bishop":
		return 2
	case "rook":
		return 3
	case "queen":
		return 4
	case "king":
		return 5
	}
	return 0
}


