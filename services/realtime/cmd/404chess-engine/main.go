package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/chess404/realtime/internal/contracts"
	"github.com/chess404/realtime/internal/engine"
)

func main() {
	fmt.Println("404chess-engine v1.0 by chess404")
	scanner := bufio.NewScanner(os.Stdin)

	var tt *engine.TranspositionTable
	var currentState *contracts.MatchState

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		cmd := parts[0]

		switch cmd {
		case "uci":
			fmt.Println("id name 404chess-engine")
			fmt.Println("id author chess404")
			fmt.Println("uciok")

		case "isready":
			if tt == nil {
				tt = engine.NewTranspositionTable(1 << 20)
			}
			fmt.Println("readyok")

		case "position":
			if len(parts) < 2 {
				continue
			}
			var fen string
			moveStart := len(parts)

			if parts[1] == "startpos" {
				fen = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
				moveStart = 2
			} else if parts[1] == "fen" && len(parts) >= 7 {
				fen = strings.Join(parts[2:8], " ")
				moveStart = 8
			}

			currentState = engine.MatchStateFromFEN(fen)
			if currentState == nil {
				fmt.Printf("info string error: failed to parse FEN: %s\n", fen)
				continue
			}

			// Apply moves
			for i := moveStart; i < len(parts); i++ {
				if parts[i] == "moves" {
					continue
				}
				move := parseUCIMove(parts[i])
				if move == nil {
					break
				}
				currentState = engine.ApplyMoveCopy(currentState, move)
			}

		case "go":
			if currentState == nil {
				continue
			}
			depth := 4
			for i := 1; i < len(parts); i++ {
				if parts[i] == "depth" && i+1 < len(parts) {
					d, err := strconv.Atoi(parts[i+1])
					if err == nil {
						depth = d
					}
				}
			}
			if tt == nil {
				tt = engine.NewTranspositionTable(1 << 20)
			}

			isWhite := currentState.Turn == "white"
			moves := engine.GenerateAllMoves(currentState, isWhite)
			if len(moves) == 0 {
				if engine.IsKingInCheck(currentState) {
					fmt.Println("info string checkmate")
				} else {
					fmt.Println("info string stalemate")
				}
				continue
			}

			result := engine.Search(currentState, depth, tt)
			if result.BestMove.From.Row == 0 && result.BestMove.From.Col == 0 &&
				result.BestMove.To.Row == 0 && result.BestMove.To.Col == 0 {
				result.BestMove = moves[0]
			}

			fmt.Printf("info depth %d score cp %d nodes %d\n", depth, result.Score, result.Nodes)
			fmt.Printf("bestmove %s\n", engine.MoveToUCI(&result.BestMove))

		case "perft":
			if currentState == nil || len(parts) < 2 {
				continue
			}
			depth, err := strconv.Atoi(parts[1])
			if err != nil {
				continue
			}
			divide := engine.PerftDivide(currentState, depth)
			total := 0
			for move, count := range divide {
				fmt.Printf("%s: %d\n", move, count)
				total += count
			}
			fmt.Printf("\nTotal: %d\n", total)

		case "print":
			if currentState != nil {
				fmt.Println(engine.BoardToSimpleFEN(currentState))
			}

		case "quit", "exit":
			return
		}
	}
}

func parseUCIMove(uci string) *engine.Move {
	if len(uci) < 4 {
		return nil
	}
	fromCol := int(uci[0] - 'a')
	fromRow := int(uci[1] - '1')
	toCol := int(uci[2] - 'a')
	toRow := int(uci[3] - '1')
	m := &engine.Move{
		From: contracts.Square{Row: fromRow, Col: fromCol},
		To:   contracts.Square{Row: toRow, Col: toCol},
	}
	if len(uci) == 5 {
		switch uci[4] {
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
