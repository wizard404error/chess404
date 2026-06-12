package anticheat

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// StockfishConfig controls how a Stockfish subprocess is launched and
// how it talks to us.
type StockfishConfig struct {
	// Binary is the path to the Stockfish executable. If empty, the
	// StockfishEngine uses the result of exec.LookPath("stockfish")
	// at start time.
	Binary string
	// HashMB is the transposition-table size in megabytes. Default: 16.
	// Higher values give stronger play at the cost of memory.
	HashMB int
	// Threads is the number of CPU threads. Default: 1. The
	// analysis-worker uses depth-limited search; 1 thread is
	// sufficient for our game volume.
	Threads int
	// StartTimeout is how long to wait for the engine to respond to
	// the initial "uci" command. Default: 5s.
	StartTimeout time.Duration
	// MoveTimeout is how long to wait for "bestmove" after each
	// "go" command. Default: 30s. Positions with long PVs at high
	// depth can take a while; 30s is comfortable for depth 20.
	MoveTimeout time.Duration
}

func (c StockfishConfig) withDefaults() StockfishConfig {
	if c.HashMB <= 0 {
		c.HashMB = 16
	}
	if c.Threads <= 0 {
		c.Threads = 1
	}
	if c.StartTimeout <= 0 {
		c.StartTimeout = 5 * time.Second
	}
	if c.MoveTimeout <= 0 {
		c.MoveTimeout = 30 * time.Second
	}
	return c
}

// StockfishEngine is an Engine backed by a Stockfish subprocess. The
// subprocess runs the universal chess interface (UCI) protocol over
// stdin/stdout. The engine keeps the subprocess warm across calls so
// the transposition table accumulates across positions — this gives a
// 10-50x speedup over cold-start per move.
type StockfishEngine struct {
	cfg    StockfishConfig
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	mu     sync.Mutex // serializes writes/reads to the subprocess
	closed atomic.Bool
}

// NewStockfishEngine launches a Stockfish subprocess and returns an
// Engine ready to evaluate positions. Returns an error if Stockfish
// cannot be started or fails to respond to the initial handshake.
func NewStockfishEngine(cfg StockfishConfig) (*StockfishEngine, error) {
	cfg = cfg.withDefaults()
	binary := strings.TrimSpace(cfg.Binary)
	if binary == "" {
		found, err := exec.LookPath("stockfish")
		if err != nil {
			return nil, fmt.Errorf("stockfish binary not found on PATH: %w", err)
		}
		binary = found
	}

	cmd := exec.Command(binary)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stockfish stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("stockfish stdout pipe: %w", err)
	}
	// Discard stderr unless we enable debug logging.
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("start stockfish: %w", err)
	}

	eng := &StockfishEngine{
		cfg:    cfg,
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
	}

	// Handshake: send "uci", wait for "uciok".
	if err := eng.handshake(cfg.StartTimeout); err != nil {
		_ = eng.Close()
		return nil, fmt.Errorf("stockfish handshake: %w", err)
	}

	// Configure options. Hash and threads affect search quality; the
	// rest is irrelevant for our use.
	setupCommands := []string{
		fmt.Sprintf("setoption name Hash value %d", cfg.HashMB),
		fmt.Sprintf("setoption name Threads value %d", cfg.Threads),
		"setoption name UCI_AnalyseMode value true",
		"ucinewgame",
		"isready",
	}
	if err := eng.runSimple(setupCommands, "readyok", cfg.StartTimeout); err != nil {
		_ = eng.Close()
		return nil, fmt.Errorf("stockfish setup: %w", err)
	}
	return eng, nil
}

// handshake sends "uci" and waits for the engine to acknowledge with
// "uciok". The engine may print other lines (id name, id author,
// option ...) before uciok; we read until we see uciok.
func (e *StockfishEngine) handshake(timeout time.Duration) error {
	if err := e.writeLine("uci"); err != nil {
		return err
	}
	return e.readUntil("uciok", timeout)
}

// readUntil reads lines from the engine until one of them matches
// target (prefix match). Returns the matching line or an error.
func (e *StockfishEngine) readUntil(target string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for %q", target)
		}
		line, err := e.stdout.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read engine line: %w", err)
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, target) {
			return nil
		}
	}
}

// runSimple sends commands and waits for the final target line. Used
// for "isready"/"readyok" pairs that need no parsing of intermediate
// output.
func (e *StockfishEngine) runSimple(commands []string, target string, timeout time.Duration) error {
	for _, cmd := range commands {
		if err := e.writeLine(cmd); err != nil {
			return err
		}
	}
	return e.readUntil(target, timeout)
}

// writeLine writes a single command followed by a newline.
func (e *StockfishEngine) writeLine(line string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed.Load() {
		return ErrEngineClosed
	}
	if _, err := io.WriteString(e.stdin, line+"\n"); err != nil {
		return fmt.Errorf("write to engine: %w", err)
	}
	return nil
}

// TopNMoves implements Engine. It sends "position fen ...", then
// "go depth N" and reads info lines until "bestmove" arrives. From the
// info lines it extracts one EngineMove per multipv rank (1..N).
//
// Notes on correctness:
//   - The position is set fresh per call so the engine never has
//     stale state.
//   - We do not call "ucinewgame" because that would clear the
//     transposition table; we want it to keep accumulating across
//     positions in the same game.
//   - mate scores are translated to ±10000 centipawns so the rank
//     ordering still works for forced wins/losses.
func (e *StockfishEngine) TopNMoves(ctx context.Context, fen string, depth, multiPV int) ([]EngineMove, error) {
	if e.closed.Load() {
		return nil, ErrEngineClosed
	}
	if depth <= 0 {
		depth = 20
	}
	if multiPV <= 0 {
		multiPV = 3
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// We do all I/O under the same lock to keep the protocol stream
	// simple. The alternative is a per-call lock plus a drain loop;
	// for our throughput (one game every few seconds) the simpler
	// approach is fine.
	if _, err := io.WriteString(e.stdin, fmt.Sprintf("setoption name MultiPV value %d\n", multiPV)); err != nil {
		return nil, fmt.Errorf("set MultiPV: %w", err)
	}
	if _, err := io.WriteString(e.stdin, "position fen "+fen+"\n"); err != nil {
		return nil, fmt.Errorf("set position: %w", err)
	}
	if _, err := io.WriteString(e.stdin, fmt.Sprintf("go depth %d\n", depth)); err != nil {
		return nil, fmt.Errorf("go: %w", err)
	}

	// Read until "bestmove" or context cancellation.
	byRank := make(map[int]EngineMove, multiPV)
	deadline := time.Now().Add(e.cfg.MoveTimeout)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for bestmove")
		}
		line, err := e.stdout.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("read engine line: %w", err)
		}
		line = strings.TrimSpace(line)
		if rank, score, move, ok := parseUCIInfoLine(line); ok {
			if _, dup := byRank[rank]; !dup {
				byRank[rank] = EngineMove{Move: move, ScoreCP: score, Rank: rank}
			}
		}
		if strings.HasPrefix(line, "bestmove ") {
			break
		}
	}

	out := make([]EngineMove, 0, len(byRank))
	for rank := 1; rank <= multiPV; rank++ {
		if m, ok := byRank[rank]; ok {
			out = append(out, m)
		}
	}
	return out, nil
}

// Close terminates the Stockfish subprocess. Safe to call multiple
// times.
func (e *StockfishEngine) Close() error {
	if !e.closed.CompareAndSwap(false, true) {
		return nil
	}
	if e.stdin != nil {
		// Send "quit" to let Stockfish exit cleanly.
		_, _ = io.WriteString(e.stdin, "quit\n")
		_ = e.stdin.Close()
	}
	if e.cmd != nil && e.cmd.Process != nil {
		// Give it a moment to exit; kill if it doesn't.
		done := make(chan struct{})
		go func() { _ = e.cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			_ = e.cmd.Process.Kill()
			<-done
		}
	}
	return nil
}

// Compile-time guard: StockfishEngine implements Engine.
var _ Engine = (*StockfishEngine)(nil)
var _ Engine = (*MockEngine)(nil)

// (intentionally blank) suppress unused imports if errors is not used
var _ = errors.New
