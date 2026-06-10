package rate_limit

import (
	"sync"
	"time"
)

type FloodDetector struct {
	mu          sync.RWMutex
	trackers    map[string]*ipTracker
	threshold   int
	window      time.Duration
	banDuration time.Duration
	bans        map[string]time.Time
}

type ipTracker struct {
	timestamps []time.Time
	requests   int
}

func NewFloodDetector(threshold int, window, banDuration time.Duration) *FloodDetector {
	fd := &FloodDetector{
		trackers:    make(map[string]*ipTracker),
		threshold:   threshold,
		window:      window,
		banDuration: banDuration,
		bans:        make(map[string]time.Time),
	}
	go fd.cleanupLoop()
	return fd
}

func (fd *FloodDetector) IsFlood(ip string) bool {
	fd.mu.RLock()
	if banEnd, ok := fd.bans[ip]; ok {
		fd.mu.RUnlock()
		return time.Now().Before(banEnd)
	}
	fd.mu.RUnlock()

	fd.mu.Lock()
	defer fd.mu.Unlock()

	tracker, ok := fd.trackers[ip]
	if !ok {
		tracker = &ipTracker{}
		fd.trackers[ip] = tracker
	}

	now := time.Now()
	cutoff := now.Add(-fd.window)

	validTimestamps := tracker.timestamps[:0]
	for _, ts := range tracker.timestamps {
		if ts.After(cutoff) {
			validTimestamps = append(validTimestamps, ts)
		}
	}
	tracker.timestamps = validTimestamps

	tracker.timestamps = append(tracker.timestamps, now)
	tracker.requests = len(tracker.timestamps)

	if tracker.requests >= fd.threshold {
		fd.bans[ip] = now.Add(fd.banDuration)
		return true
	}

	return false
}

func (fd *FloodDetector) IsBanned(ip string) bool {
	fd.mu.RLock()
	defer fd.mu.RUnlock()

	banEnd, ok := fd.bans[ip]
	if !ok {
		return false
	}
	return time.Now().Before(banEnd)
}

func (fd *FloodDetector) Ban(ip string, duration time.Duration) {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	fd.bans[ip] = time.Now().Add(duration)
}

func (fd *FloodDetector) Unban(ip string) {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	delete(fd.bans, ip)
}

func (fd *FloodDetector) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		fd.mu.Lock()
		now := time.Now()
		for ip, banEnd := range fd.bans {
			if now.After(banEnd) {
				delete(fd.bans, ip)
			}
		}
		cutoff := now.Add(-fd.window)
		for ip, tracker := range fd.trackers {
			valid := tracker.timestamps[:0]
			for _, ts := range tracker.timestamps {
				if ts.After(cutoff) {
					valid = append(valid, ts)
				}
			}
			if len(valid) == 0 {
				delete(fd.trackers, ip)
			} else {
				tracker.timestamps = valid
			}
		}
		fd.mu.Unlock()
	}
}

func (fd *FloodDetector) Stats() FloodStats {
	fd.mu.RLock()
	defer fd.mu.RUnlock()

	return FloodStats{
		TrackedIPs: len(fd.trackers),
		BannedIPs:  len(fd.bans),
	}
}

type FloodStats struct {
	TrackedIPs int `json:"tracked_ips"`
	BannedIPs  int `json:"banned_ips"`
}
