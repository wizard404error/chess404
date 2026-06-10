package anticheat

import (
	"time"

	"github.com/chess404/realtime/internal/engine"
	"github.com/chess404/realtime/internal/contracts"
)

func AnalyzeGame(record *GameRecord) *AnalysisResult {
	result := &AnalysisResult{
		MatchID:   record.MatchID,
		MoveCount: len(record.Moves),
		AnalyzedAt: time.Now().UTC(),
	}

	whiteCPL := []int{}
	blackCPL := []int{}

	board := makeBoard()
	turn := "white"

	for _, move := range record.Moves {
		engineState := &contracts.MatchState{
			Board: board,
			Turn:  turn,
			Moved: []string{},
		}

		bestResult := engine.Search(engineState, 4, nil)
		actualScore := evaluateAfterMove(board, move, turn)

		cpl := bestResult.Score - actualScore
		if cpl < 0 {
			cpl = 0
		}

		if move.Color == "white" {
			whiteCPL = append(whiteCPL, cpl)
		} else {
			blackCPL = append(blackCPL, cpl)
		}

		applyMoveToBoard(board, move)
		if turn == "white" {
			turn = "black"
		} else {
			turn = "white"
		}
	}

	result.CPL = whiteCPL
	if record.BlackID == result.PlayerID {
		result.CPL = blackCPL
	}

	result.AvgCPL = avgInt(result.CPL)
	result.MaxCPL = maxInt(result.CPL)
	result.Accuracy = CalculateAccuracy(result.CPL)
	result.TimeProfile = AnalyzeTimeProfile(record.Moves)
	result.SuspicionScore = CalculateSuspicion(result)
	result.Flags = DetectFlags(result)

	return result
}

func evaluateAfterMove(board [][]*contracts.Piece, move MoveRecord, color string) int {
	from := parseSquare(move.From)
	to := parseSquare(move.To)

	if from == nil || to == nil {
		return 0
	}

	newBoard := cloneBoard(board)
	piece := newBoard[from.Row][from.Col]
	if piece == nil {
		return 0
	}
	newBoard[to.Row][to.Col] = piece
	newBoard[from.Row][from.Col] = nil

	return engine.Evaluate(newBoard, color)
}

func parseSquare(s string) *contracts.Square {
	if len(s) < 2 {
		return nil
	}
	col := int(s[0] - 'a')
	row := int(s[1] - '1')
	if col < 0 || col > 7 || row < 0 || row > 7 {
		return nil
	}
	return &contracts.Square{Row: row, Col: col}
}

func makeBoard() [][]*contracts.Piece {
	b := make([][]*contracts.Piece, 8)
	for r := 0; r < 8; r++ {
		b[r] = make([]*contracts.Piece, 8)
	}
	setupPosition(b)
	return b
}

func setupPosition(b [][]*contracts.Piece) {
	b[0] = []*contracts.Piece{
		{Type: "rook", Color: "white"}, {Type: "knight", Color: "white"},
		{Type: "bishop", Color: "white"}, {Type: "queen", Color: "white"},
		{Type: "king", Color: "white"}, {Type: "bishop", Color: "white"},
		{Type: "knight", Color: "white"}, {Type: "rook", Color: "white"},
	}
	for c := 0; c < 8; c++ {
		b[1][c] = &contracts.Piece{Type: "pawn", Color: "white"}
	}
	b[7] = []*contracts.Piece{
		{Type: "rook", Color: "black"}, {Type: "knight", Color: "black"},
		{Type: "bishop", Color: "black"}, {Type: "queen", Color: "black"},
		{Type: "king", Color: "black"}, {Type: "bishop", Color: "black"},
		{Type: "knight", Color: "black"}, {Type: "rook", Color: "black"},
	}
	for c := 0; c < 8; c++ {
		b[6][c] = &contracts.Piece{Type: "pawn", Color: "black"}
	}
}

func cloneBoard(b [][]*contracts.Piece) [][]*contracts.Piece {
	clone := make([][]*contracts.Piece, 8)
	for r := 0; r < 8; r++ {
		clone[r] = make([]*contracts.Piece, 8)
		for c := 0; c < 8; c++ {
			if b[r][c] != nil {
				p := *b[r][c]
				clone[r][c] = &p
			}
		}
	}
	return clone
}

func applyMoveToBoard(board [][]*contracts.Piece, move MoveRecord) {
	from := parseSquare(move.From)
	to := parseSquare(move.To)
	if from == nil || to == nil {
		return
	}
	piece := board[from.Row][from.Col]
	board[to.Row][to.Col] = piece
	board[from.Row][from.Col] = nil
}

func avgInt(vals []int) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0
	for _, v := range vals {
		sum += v
	}
	return float64(sum) / float64(len(vals))
}

func maxInt(vals []int) int {
	max := 0
	for _, v := range vals {
		if v > max {
			max = v
		}
	}
	return max
}
