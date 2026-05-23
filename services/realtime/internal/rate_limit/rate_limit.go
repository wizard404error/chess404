package rate_limit

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultAPIWindow    = time.Minute
	DefaultAPILimit     = 60
	DefaultAuthWindow   = 10 * time.Minute
	DefaultAuthLimit    = 20
	DefaultQueueWindow  = 30 * time.Second
	DefaultQueueLimit   = 10
)

type bucket struct {
	windowStart time.Time
	count       int
}

type Limiter struct {
	now     func() time.Time
	mu      sync.Mutex
	buckets map[string]bucket
}

func New() *Limiter {
	return &Limiter{
		now:     time.Now,
		buckets: make(map[string]bucket),
	}
}

func (l *Limiter) Allow(key string, window time.Duration, limit int) (bool, time.Duration) {
	if key == "" || limit <= 0 {
		return true, 0
	}
	now := l.now().UTC()
	l.mu.Lock()
	defer l.mu.Unlock()

	b := l.buckets[key]
	if b.windowStart.IsZero() || now.Sub(b.windowStart) >= window {
		b = bucket{windowStart: now, count: 0}
	}
	if b.count >= limit {
		retryAfter := b.windowStart.Add(window).Sub(now)
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		return false, retryAfter
	}
	b.count++
	l.buckets[key] = b
	return true, 0
}

func (l *Limiter) Middleware(window time.Duration, limit int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := ClientIP(r)
			key := "rl:" + ip
			allowed, retryAfter := l.Allow(key, window, limit)
			if !allowed {
				seconds := int(math.Ceil(retryAfter.Seconds()))
				if seconds < 1 {
					seconds = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(seconds))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func ClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	if host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr)); err == nil {
		return strings.TrimSpace(host)
	}
	return strings.TrimSpace(r.RemoteAddr)
}
