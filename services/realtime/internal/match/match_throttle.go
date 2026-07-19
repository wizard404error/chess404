package match

import (
	"fmt"
	"math"
	"time"
)

const (
	maxIntentBurst   = 5.0
	intentRefillRate = 10.0
)

func rateLimitIntent(presence *matchPresenceState, actorColor string, now time.Time) error {
	if presence == nil || actorColor == "" {
		return nil
	}
	var tokens *float64
	var lastRefill *time.Time
	if actorColor == "white" {
		tokens = &presence.WhiteTokens
		lastRefill = &presence.WhiteLastRefill
	} else if actorColor == "black" {
		tokens = &presence.BlackTokens
		lastRefill = &presence.BlackLastRefill
	} else {
		return nil
	}
	if !lastRefill.IsZero() {
		elapsed := now.Sub(*lastRefill).Seconds()
		*tokens = math.Min(maxIntentBurst, *tokens+elapsed*intentRefillRate)
	} else {
		*tokens = maxIntentBurst
	}
	*lastRefill = now

	if *tokens < 1.0 {
		return fmt.Errorf("rate limited: too many intents (max %.0f/sec burst %.0f)", intentRefillRate, maxIntentBurst)
	}
	*tokens--
	return nil
}

func trackIntentTime(presence *matchPresenceState, actorColor string, now time.Time) {
	if presence == nil || actorColor == "" {
		return
	}
	if actorColor == "white" {
		presence.WhiteLastIntentAt = now
	} else if actorColor == "black" {
		presence.BlackLastIntentAt = now
	}
}
