package rate_limit

import (
	"sync"
	"time"
)

type CompositeRateLimiter struct {
	mu       sync.RWMutex
	fast     map[string]*slidingWindow
	slow     map[string]*slidingWindow
	fastRate int
	fastWindow time.Duration
	slowRate int
	slowWindow time.Duration
}

type slidingWindow struct {
	timestamps []time.Time
}

func NewCompositeRateLimiter(fastRate int, fastWindow time.Duration, slowRate int, slowWindow time.Duration) *CompositeRateLimiter {
	crl := &CompositeRateLimiter{
		fast:       make(map[string]*slidingWindow),
		slow:       make(map[string]*slidingWindow),
		fastRate:   fastRate,
		fastWindow: fastWindow,
		slowRate:   slowRate,
		slowWindow: slowWindow,
	}
	go crl.cleanup()
	return crl
}

func (crl *CompositeRateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		crl.mu.Lock()
		now := time.Now()
		for key, sw := range crl.fast {
			cutoff := now.Add(-crl.fastWindow)
			valid := sw.timestamps[:0]
			for _, ts := range sw.timestamps {
				if ts.After(cutoff) {
					valid = append(valid, ts)
				}
			}
			if len(valid) == 0 {
				delete(crl.fast, key)
			} else {
				sw.timestamps = valid
			}
		}
		for key, sw := range crl.slow {
			cutoff := now.Add(-crl.slowWindow)
			valid := sw.timestamps[:0]
			for _, ts := range sw.timestamps {
				if ts.After(cutoff) {
					valid = append(valid, ts)
				}
			}
			if len(valid) == 0 {
				delete(crl.slow, key)
			} else {
				sw.timestamps = valid
			}
		}
		crl.mu.Unlock()
	}
}

func (crl *CompositeRateLimiter) Allow(key string) bool {
	return crl.AllowFast(key) && crl.AllowSlow(key)
}

func (crl *CompositeRateLimiter) AllowFast(key string) bool {
	crl.mu.Lock()
	defer crl.mu.Unlock()

	sw, ok := crl.fast[key]
	if !ok {
		sw = &slidingWindow{}
		crl.fast[key] = sw
	}

	now := time.Now()
	cutoff := now.Add(-crl.fastWindow)

	valid := sw.timestamps[:0]
	for _, ts := range sw.timestamps {
		if ts.After(cutoff) {
			valid = append(valid, ts)
		}
	}
	sw.timestamps = valid

	if len(sw.timestamps) >= crl.fastRate {
		return false
	}

	sw.timestamps = append(sw.timestamps, now)
	return true
}

func (crl *CompositeRateLimiter) AllowSlow(key string) bool {
	crl.mu.Lock()
	defer crl.mu.Unlock()

	sw, ok := crl.slow[key]
	if !ok {
		sw = &slidingWindow{}
		crl.slow[key] = sw
	}

	now := time.Now()
	cutoff := now.Add(-crl.slowWindow)

	valid := sw.timestamps[:0]
	for _, ts := range sw.timestamps {
		if ts.After(cutoff) {
			valid = append(valid, ts)
		}
	}
	sw.timestamps = valid

	if len(sw.timestamps) >= crl.slowRate {
		return false
	}

	sw.timestamps = append(sw.timestamps, now)
	return true
}

func (crl *CompositeRateLimiter) Stats() CompositeRateStats {
	crl.mu.RLock()
	defer crl.mu.RUnlock()

	return CompositeRateStats{
		FastTrackedIPs: len(crl.fast),
		SlowTrackedIPs: len(crl.slow),
	}
}

type CompositeRateStats struct {
	FastTrackedIPs int `json:"fast_tracked_ips"`
	SlowTrackedIPs int `json:"slow_tracked_ips"`
}
