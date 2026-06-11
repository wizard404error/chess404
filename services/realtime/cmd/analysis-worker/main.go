package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chess404/realtime/internal/anticheat"
)

type Job struct {
	MatchID   string `json:"matchId"`
	WhiteID   string `json:"whiteId"`
	BlackID   string `json:"blackId"`
	GameJSON  string `json:"gameJson"`
}

type Result struct {
	MatchID  string                    `json:"matchId"`
	PlayerID string                    `json:"playerId"`
	Result   *anticheat.AnalysisResult `json:"result"`
}

func main() {
	log.SetPrefix("[analysis-worker] ")
	log.Println("Starting analysis worker...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Shutting down...")
		cancel()
	}()

	worker := &Worker{}
	log.Println("Worker ready, polling for jobs...")
	worker.Run(ctx)
}

type Worker struct{}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.pollAndProcess(ctx)
		}
	}
}

func (w *Worker) pollAndProcess(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	default:
	}
	log.Println("Polling for analysis jobs...")
}

func (w *Worker) ProcessJob(ctx context.Context, job *Job) (*Result, error) {
	record := &anticheat.GameRecord{
		MatchID: job.MatchID,
		WhiteID: job.WhiteID,
		BlackID: job.BlackID,
		Moves:   []anticheat.MoveRecord{},
	}

	if err := json.Unmarshal([]byte(job.GameJSON), &record.Moves); err != nil {
		return nil, fmt.Errorf("unmarshal moves: %w", err)
	}

	result := anticheat.AnalyzeGame(record, job.WhiteID)

	return &Result{
		MatchID:  job.MatchID,
		PlayerID: job.WhiteID,
		Result:   result,
	}, nil
}
