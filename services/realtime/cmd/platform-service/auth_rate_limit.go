package main

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
	registrationRateWindow           = time.Hour
	registrationRateLimitPerIP       = 8
	registrationRateLimitPerIdentity = 4
	loginRateWindow                  = 10 * time.Minute
	loginRateLimitPerIP              = 20
	loginRateLimitPerIdentifier      = 8
	passwordResetRateWindow          = time.Hour
	passwordResetRateLimitPerIP      = 12
	passwordResetRateLimitPerID      = 4
	emailVerificationRateWindow      = time.Hour
	emailVerificationRateLimitPerIP  = 12
	emailVerificationRateLimitPerID  = 4
	credentialRateWindow             = time.Hour
	credentialRateLimitPerIP         = 10
	credentialRateLimitPerAccount    = 4
)

type authRateBucket struct {
	windowStart time.Time
	count       int
}

type platformAuthThrottle struct {
	now     func() time.Time
	mu      sync.Mutex
	buckets map[string]authRateBucket
}

func newPlatformAuthThrottle(now func() time.Time) *platformAuthThrottle {
	if now == nil {
		now = time.Now
	}
	return &platformAuthThrottle{
		now:     now,
		buckets: map[string]authRateBucket{},
	}
}

func (t *platformAuthThrottle) allowLogin(r *http.Request, identifier string) (bool, time.Duration) {
	return t.allowScoped(
		loginRateWindow,
		throttleClientKey("auth:login:ip", requestClientIP(r)),
		loginRateLimitPerIP,
		throttleIdentityKey("auth:login:id", identifier),
		loginRateLimitPerIdentifier,
	)
}

func (t *platformAuthThrottle) allowRegistration(r *http.Request, handle, email string) (bool, time.Duration) {
	identifier := strings.TrimSpace(email)
	if identifier == "" {
		identifier = handle
	}
	return t.allowScoped(
		registrationRateWindow,
		throttleClientKey("auth:register:ip", requestClientIP(r)),
		registrationRateLimitPerIP,
		throttleIdentityKey("auth:register:id", identifier),
		registrationRateLimitPerIdentity,
	)
}

func (t *platformAuthThrottle) allowPasswordReset(r *http.Request, identifier string) (bool, time.Duration) {
	return t.allowScoped(
		passwordResetRateWindow,
		throttleClientKey("auth:password-reset:ip", requestClientIP(r)),
		passwordResetRateLimitPerIP,
		throttleIdentityKey("auth:password-reset:id", identifier),
		passwordResetRateLimitPerID,
	)
}

func (t *platformAuthThrottle) allowEmailVerification(r *http.Request, accountID string) (bool, time.Duration) {
	return t.allowScoped(
		emailVerificationRateWindow,
		throttleClientKey("auth:email-verification:ip", requestClientIP(r)),
		emailVerificationRateLimitPerIP,
		throttleIdentityKey("auth:email-verification:account", accountID),
		emailVerificationRateLimitPerID,
	)
}

func (t *platformAuthThrottle) allowCredentialSetup(r *http.Request, accountID string) (bool, time.Duration) {
	return t.allowScoped(
		credentialRateWindow,
		throttleClientKey("auth:credentials:ip", requestClientIP(r)),
		credentialRateLimitPerIP,
		throttleIdentityKey("auth:credentials:account", accountID),
		credentialRateLimitPerAccount,
	)
}

func (t *platformAuthThrottle) allowScoped(window time.Duration, primaryKey string, primaryLimit int, secondaryKey string, secondaryLimit int) (bool, time.Duration) {
	allowed, retryAfter := t.allow(primaryKey, window, primaryLimit)
	if !allowed {
		return false, retryAfter
	}
	allowed, secondRetryAfter := t.allow(secondaryKey, window, secondaryLimit)
	if !allowed {
		return false, secondRetryAfter
	}
	return true, 0
}

func (t *platformAuthThrottle) allow(key string, window time.Duration, limit int) (bool, time.Duration) {
	resolvedKey := strings.TrimSpace(key)
	if resolvedKey == "" || limit <= 0 {
		return true, 0
	}
	now := t.now().UTC()
	t.mu.Lock()
	defer t.mu.Unlock()

	bucket := t.buckets[resolvedKey]
	if bucket.windowStart.IsZero() || now.Sub(bucket.windowStart) >= window {
		bucket = authRateBucket{
			windowStart: now,
			count:       0,
		}
	}
	if bucket.count >= limit {
		retryAfter := bucket.windowStart.Add(window).Sub(now)
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		return false, retryAfter
	}
	bucket.count++
	t.buckets[resolvedKey] = bucket
	t.pruneExpiredBucketsLocked(now, window)
	return true, 0
}

func (t *platformAuthThrottle) pruneExpiredBucketsLocked(now time.Time, window time.Duration) {
	if len(t.buckets) < 2048 {
		return
	}
	cutoff := now.Add(-window)
	for key, bucket := range t.buckets {
		if bucket.windowStart.Before(cutoff) {
			delete(t.buckets, key)
		}
	}
}

func throttleClientKey(prefix, value string) string {
	resolved := strings.TrimSpace(strings.ToLower(value))
	if resolved == "" {
		resolved = "unknown"
	}
	return prefix + ":" + resolved
}

func throttleIdentityKey(prefix, value string) string {
	resolved := strings.TrimSpace(strings.ToLower(value))
	if resolved == "" {
		resolved = "unknown"
	}
	return prefix + ":" + resolved
}

func requestClientIP(r *http.Request) string {
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

func writeAuthRateLimitError(w http.ResponseWriter, retryAfter time.Duration) {
	seconds := int(math.Ceil(retryAfter.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	http.Error(w, `{"error":"too many authentication requests"}`, http.StatusTooManyRequests)
}
