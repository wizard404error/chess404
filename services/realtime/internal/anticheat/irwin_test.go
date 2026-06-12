package anticheat

import (
	"context"
	"testing"
	"time"
)

func TestParseUCIInfoLine_Valid(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		wantRank    int
		wantScore   int
		wantMove    string
		wantOK      bool
	}{
		{
			name:      "multipv 1 cp score",
			line:      "info depth 20 multipv 1 score cp 25 nodes 100000 nps 50000 tbhits 0 time 2000 pv e2e4 e7e5 g1f3",
			wantRank:  1,
			wantScore: 25,
			wantMove:  "e2e4",
			wantOK:    true,
		},
		{
			name:      "multipv 2 negative cp",
			line:      "info depth 20 multipv 2 score cp -42 nodes 100000 nps 50000 tbhits 0 time 2000 pv d2d4 d7d5",
			wantRank:  2,
			wantScore: -42,
			wantMove:  "d2d4",
			wantOK:    true,
		},
		{
			name:      "mate score becomes large cp",
			line:      "info depth 18 multipv 1 score mate 3 nodes 50000 nps 50000 tbhits 0 time 1500 pv e2e4 e7e5 d1h5",
			wantRank:  1,
			wantScore: 9997, // 10000 - 3
			wantMove:  "e2e4",
			wantOK:    true,
		},
		{
			name:      "mate negative becomes large negative",
			line:      "info depth 18 multipv 1 score mate -2 nodes 50000 nps 50000 tbhits 0 time 1500 pv e2e4",
			wantRank:  1,
			wantScore: -10002, // mate -2 = -10000 + (-2) = -10002
			wantMove:  "e2e4",
			wantOK:    true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rank, score, move, ok := parseUCIInfoLine(tc.line)
			if ok != tc.wantOK {
				t.Fatalf("ok mismatch: want %v got %v", tc.wantOK, ok)
			}
			if !ok {
				return
			}
			if rank != tc.wantRank {
				t.Fatalf("rank: want %d got %d", tc.wantRank, rank)
			}
			if score != tc.wantScore {
				t.Fatalf("score: want %d got %d", tc.wantScore, score)
			}
			if move != tc.wantMove {
				t.Fatalf("move: want %q got %q", tc.wantMove, move)
			}
		})
	}
}

func TestParseUCIInfoLine_Invalid(t *testing.T) {
	invalid := []string{
		"",
		"uciok",
		"readyok",
		"info depth 0",
		"info depth 20 nodes 100000",
		"info multipv 1 pv e2e4",          // no score
		"info score cp 25 pv e2e4",        // no multipv
		"info multipv 1 score cp 25",        // no pv
		"info multipv abc score cp 25 pv e2e4", // bad multipv
		"info multipv 1 score mate x pv e2e4",   // bad mate value
	}
	for _, line := range invalid {
		t.Run(line, func(t *testing.T) {
			_, _, _, ok := parseUCIInfoLine(line)
			if ok {
				t.Fatalf("expected ok=false for %q", line)
			}
		})
	}
}

func TestParseIntSafe(t *testing.T) {
	tests := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{"0", 0, false},
		{"1", 1, false},
		{"-1", -1, false},
		{"+1", 1, false},
		{"100", 100, false},
		{"-100", -100, false},
		{"", 0, true},
		{"abc", 0, true},
		{"-", 0, true},
		{"+", 0, true},
		{"-abc", 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := parseIntSafe(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err mismatch: want %v got %v", tc.wantErr, err)
			}
			if !tc.wantErr && got != tc.want {
				t.Fatalf("want %d got %d", tc.want, got)
			}
		})
	}
}

func TestAnalyzeIrwin_PerfectEnginePlay(t *testing.T) {
	// A player who always picks the engine's top-1 should hit
	// Top1Pct=100, Top3Pct=100, AvgRank=1.0, score=100.
	engine := NewMockEngine([]EngineMove{
		{Move: "e2e4", ScoreCP: 30, Rank: 1},
		{Move: "d2d4", ScoreCP: 25, Rank: 2},
		{Move: "g1f3", ScoreCP: 20, Rank: 3},
	})
	samples := []PositionSample{
		{FEN: "fen1", PlayedMove: "e2e4"},
		{FEN: "fen2", PlayedMove: "d2d4"},
		{FEN: "fen3", PlayedMove: "e2e4"},
		{FEN: "fen4", PlayedMove: "e2e4"},
	}
	res, err := AnalyzeIrwin(context.Background(), engine, samples, 20, 3)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if res.TotalPositions != 4 || res.EnginePositions != 4 {
		t.Fatalf("counts: %+v", res)
	}
	if res.Top1Count != 3 || res.Top3Count != 4 {
		t.Fatalf("expected top1=3 top3=4, got top1=%d top3=%d", res.Top1Count, res.Top3Count)
	}
	if res.Top1Pct != 75.0 {
		t.Fatalf("expected top1=75.0, got %.1f", res.Top1Pct)
	}
	if res.Top3Pct != 100.0 {
		t.Fatalf("expected top3=100.0, got %.1f", res.Top3Pct)
	}
	if res.AvgRank != 1.25 {
		t.Fatalf("expected avgRank=1.25, got %.2f", res.AvgRank)
	}
	if res.OutsideTopN != 0 {
		t.Fatalf("expected outside=0, got %d", res.OutsideTopN)
	}
}

func TestAnalyzeIrwin_HumanLikePlay(t *testing.T) {
	// A realistic human: matches top-1 ~30% of the time, top-2 ~20%,
	// outside top-2 ~50%. Using multiPV=2 so rank-3+ are "outside".
	engine := NewMockEngine([]EngineMove{
		{Move: "e2e4", ScoreCP: 30, Rank: 1},
		{Move: "d2d4", ScoreCP: 25, Rank: 2},
		{Move: "g1f3", ScoreCP: 20, Rank: 3},
		{Move: "b1c3", ScoreCP: 15, Rank: 4},
		{Move: "a2a3", ScoreCP: 5, Rank: 5},
	})
	// 30 plays: 9 top-1 (e2e4), 6 top-2 (d2d4), 15 outside top-2
	plays := []string{
		// 9 top-1: e2e4
		"e2e4", "e2e4", "e2e4", "e2e4", "e2e4", "e2e4", "e2e4", "e2e4", "e2e4",
		// 6 top-2: d2d4
		"d2d4", "d2d4", "d2d4", "d2d4", "d2d4", "d2d4",
		// 15 outside top-2: g1f3, b1c3, a2a3 (5 each)
		"g1f3", "b1c3", "a2a3", "g1f3", "b1c3", "a2a3",
		"g1f3", "b1c3", "a2a3", "g1f3", "b1c3", "a2a3",
		"g1f3", "b1c3", "a2a3",
	}
	samples := make([]PositionSample, len(plays))
	for i, m := range plays {
		samples[i] = PositionSample{FEN: "fen" + string(rune('a'+i)), PlayedMove: m}
	}
	res, _ := AnalyzeIrwin(context.Background(), engine, samples, 20, 2)
	// With multiPV=2: 9 top-1, 6 top-2, 15 outside top-2
	if res.Top1Count != 9 || res.Top3Count != 15 || res.OutsideTopN != 15 {
		t.Fatalf("counts: top1=%d top3=%d outside=%d", res.Top1Count, res.Top3Count, res.OutsideTopN)
	}
	if res.Top1Pct != 30.0 {
		t.Fatalf("top1: want 30.0 got %.1f", res.Top1Pct)
	}
	if res.Top3Pct != 50.0 {
		t.Fatalf("top3: want 50.0 got %.1f", res.Top3Pct)
	}
	// Score should be much lower than a cheater.
	if res.Score() > 50 {
		t.Fatalf("expected human-like score < 50, got %.1f", res.Score())
	}
}

func TestAnalyzeIrwin_CardMovesExcluded(t *testing.T) {
	engine := NewMockEngine([]EngineMove{
		{Move: "e2e4", Rank: 1},
		{Move: "d2d4", Rank: 2},
		{Move: "g1f3", Rank: 3},
	})
	samples := []PositionSample{
		{FEN: "f1", PlayedMove: "e2e4", IsCardMove: false},
		{FEN: "f2", PlayedMove: "recall-b1", IsCardMove: true}, // card move
		{FEN: "f3", PlayedMove: "e2e4", IsCardMove: false},
		{FEN: "f4", PlayedMove: "swap-a1h8", IsCardMove: true},
	}
	res, _ := AnalyzeIrwin(context.Background(), engine, samples, 20, 3)
	if res.TotalPositions != 4 {
		t.Fatalf("total: %d", res.TotalPositions)
	}
	if res.CardMoveCount != 2 {
		t.Fatalf("card: %d", res.CardMoveCount)
	}
	if res.EnginePositions != 2 {
		t.Fatalf("engine: %d", res.EnginePositions)
	}
	if res.Top1Count != 2 {
		t.Fatalf("top1: %d (card moves shouldn't count)", res.Top1Count)
	}
	// 2 evaluated, 2 top-1 = 100% top-1
	if res.Top1Pct != 100.0 {
		t.Fatalf("top1pct: %.1f", res.Top1Pct)
	}
}

func TestAnalyzeIrwin_EngineErrorsExcludedFromPercent(t *testing.T) {
	// Engine returns empty for some positions. Those should not
	// count in the percentages but should appear in EngineErrors.
	engine := NewMockEngine([]EngineMove{
		{Move: "e2e4", Rank: 1},
		{Move: "d2d4", Rank: 2},
		{Move: "g1f3", Rank: 3},
	})
	engine.SetForFEN("err1", []EngineMove{}) // engine returns nothing
	engine.SetForFEN("err2", []EngineMove{}) // engine returns nothing
	samples := []PositionSample{
		{FEN: "ok1", PlayedMove: "e2e4"},
		{FEN: "err1", PlayedMove: "e2e4"},
		{FEN: "ok2", PlayedMove: "e2e4"},
		{FEN: "err2", PlayedMove: "e2e4"},
	}
	res, _ := AnalyzeIrwin(context.Background(), engine, samples, 20, 3)
	if res.EngineErrors != 2 {
		t.Fatalf("errors: %d", res.EngineErrors)
	}
	if res.EnginePositions != 4 {
		t.Fatalf("engine positions: %d", res.EnginePositions)
	}
	// 2 evaluated, both top-1 = 100%
	if res.Top1Pct != 100.0 {
		t.Fatalf("top1pct: %.1f", res.Top1Pct)
	}
}

func TestAnalyzeIrwin_EmptySamples(t *testing.T) {
	engine := NewMockEngine(nil)
	res, err := AnalyzeIrwin(context.Background(), engine, nil, 20, 3)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.TotalPositions != 0 || res.Top1Pct != 0 {
		t.Fatalf("expected zeros, got %+v", res)
	}
}

func TestAnalyzeIrwin_NilEngineErrors(t *testing.T) {
	_, err := AnalyzeIrwin(context.Background(), nil, []PositionSample{{FEN: "x", PlayedMove: "e2e4"}}, 20, 3)
	if err == nil {
		t.Fatal("expected error for nil engine")
	}
}

func TestAnalyzeIrwin_ScoreIncreasesWithTop1(t *testing.T) {
	// Verify that Score() is monotonic in Top1Pct.
	low := Result{EnginePositions: 20, EngineErrors: 0, Top1Count: 5, Top3Count: 15, RankSum: 30}.Score()
	mid := Result{EnginePositions: 20, EngineErrors: 0, Top1Count: 12, Top3Count: 18, RankSum: 30}.Score()
	high := Result{EnginePositions: 20, EngineErrors: 0, Top1Count: 18, Top3Count: 20, RankSum: 30}.Score()
	if !(low < mid && mid < high) {
		t.Fatalf("expected low < mid < high, got %.1f < %.1f < %.1f", low, mid, high)
	}
}

func TestFindUCIPlay(t *testing.T) {
	top := []EngineMove{
		{Move: "e2e4", Rank: 1},
		{Move: "d2d4", Rank: 2},
		{Move: "g1f3", Rank: 3},
	}
	cases := []struct {
		played string
		want   int
	}{
		{"e2e4", 1},
		{"d2d4", 2},
		{"g1f3", 3},
		{"E2E4", 1}, // case-insensitive
		{"  e2e4  ", 1}, // whitespace-trimmed
		{"a2a4", 0}, // not in top-N
	}
	for _, tc := range cases {
		t.Run(tc.played, func(t *testing.T) {
			if got := findUCIPlay(top, tc.played); got != tc.want {
				t.Fatalf("played=%q want %d got %d", tc.played, tc.want, got)
			}
		})
	}
}

func TestMockEngine_DefaultAndPerFEN(t *testing.T) {
	e := NewMockEngine([]EngineMove{{Move: "a1a2", Rank: 1}})
	e.SetForFEN("specific", []EngineMove{{Move: "b1b2", Rank: 1}})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Default applies to unknown FENs.
	top, err := e.TopNMoves(ctx, "unknown", 20, 1)
	if err != nil || len(top) != 1 || top[0].Move != "a1a2" {
		t.Fatalf("default lookup: top=%+v err=%v", top, err)
	}
	// Specific overrides.
	top, err = e.TopNMoves(ctx, "specific", 20, 1)
	if err != nil || len(top) != 1 || top[0].Move != "b1b2" {
		t.Fatalf("specific lookup: top=%+v err=%v", top, err)
	}
}

func TestMockEngine_CloseIsIdempotent(t *testing.T) {
	e := NewMockEngine(nil)
	if err := e.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := e.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
	_, err := e.TopNMoves(context.Background(), "x", 20, 1)
	if err == nil {
		t.Fatal("expected error after close")
	}
}
