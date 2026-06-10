package httputil

import (
	"log/slog"
	"math"
	"math/rand"
	"time"
)

func RetryWithBackoff(maxAttempts int, baseDelay time.Duration, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt < maxAttempts-1 {
			delay := baseDelay * time.Duration(math.Pow(2, float64(attempt)))
			jitter := time.Duration(rand.Int63n(int64(delay / 2)))
			slog.Debug("retrying after error",
				"attempt", attempt+1,
				"max_attempts", maxAttempts,
				"delay_ms", (delay + jitter).Milliseconds(),
				"error", err,
			)
			time.Sleep(delay + jitter)
		}
	}
	return lastErr
}
