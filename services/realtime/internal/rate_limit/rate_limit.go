package rate_limit

import (
	"crypto/subtle"
	"math"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultAPIWindow    = time.Minute
	DefaultAPILimit     = 120
	DefaultAuthWindow   = 10 * time.Minute
	DefaultAuthLimit    = 20
	DefaultQueueWindow  = 30 * time.Second
	DefaultQueueLimit   = 30
)

type bucket struct {
	windowStart time.Time
	count       int
}

type Limiter struct {
	now     func() time.Time
	mu      sync.Mutex
	buckets map[string]bucket
	stopCh  chan struct{}
}

func New() *Limiter {
	l := &Limiter{
		now:     time.Now,
		buckets: make(map[string]bucket),
		stopCh:  make(chan struct{}),
	}
	go l.cleanupLoop()
	return l
}

func (l *Limiter) Close() {
	close(l.stopCh)
}

func (l *Limiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			l.mu.Lock()
			now := l.now().UTC()
			for key, b := range l.buckets {
				if now.Sub(b.windowStart) > 2*DefaultAPIWindow {
					delete(l.buckets, key)
				}
			}
			l.mu.Unlock()
		case <-l.stopCh:
			return
		}
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

func CSRFMiddleware(next http.Handler, allowedOrigins []string, internalToken string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Allow internal service-to-service requests with a valid token
		// to bypass the Origin/Referer CSRF check.
		if internalToken != "" {
			provided := strings.TrimSpace(r.Header.Get("X-Chess404-Service-Token"))
			if provided == "" {
				const prefix = "Bearer "
				auth := strings.TrimSpace(r.Header.Get("Authorization"))
				if strings.HasPrefix(auth, prefix) {
					provided = strings.TrimSpace(strings.TrimPrefix(auth, prefix))
				}
			}
			if provided != "" && subtle.ConstantTimeCompare([]byte(provided), []byte(internalToken)) == 1 {
				next.ServeHTTP(w, r)
				return
			}
		}

		origin := r.Header.Get("Origin")
		referer := r.Header.Get("Referer")
		if origin == "" && referer == "" {
			// Reject state-changing requests with no Origin and no Referer.
			// Without these headers, the request cannot be tied to a browser
			// origin, which is the standard CSRF defense. Non-browser clients
			// must set Origin explicitly.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"CSRF check failed: origin header required"}`))
			return
		}
		check := origin
		if check == "" {
			check = referer
		}
		// Same-origin: always allowed (the standard same-origin policy guard).
		selfOrigin := trustedSelfOrigin(r)
		if selfOrigin != "" && equalFoldOrigin(check, selfOrigin) {
			next.ServeHTTP(w, r)
			return
		}
		// Cross-origin: allowed only if (a) the origin is in the explicit allow
		// list. We no longer fall through to permissive if the allow list is
		// empty — the CSRF defense is now strictly tighter than CORS, so the
		// deployment must declare an allow list explicitly.
		for _, allowed := range allowedOrigins {
			if equalFoldOrigin(check, allowed) {
				next.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"CSRF check failed: origin not allowed"}`))
	})
}

func ContentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
			ct := r.Header.Get("Content-Type")
			if ct == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnsupportedMediaType)
				_, _ = w.Write([]byte(`{"error":"Content-Type header required"}`))
				return
			}
			if !strings.Contains(ct, "application/json") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnsupportedMediaType)
				_, _ = w.Write([]byte(`{"error":"Content-Type must be application/json"}`))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// trustedSelfOrigin reconstructs the expected Origin for the current request,
// taking forwarded scheme/host headers into account when the request was
// received in plain HTTP — the universal signal that a TLS-terminating
// reverse proxy (Railway, Fly, nginx, Caddy, ALB) is in front of the
// service. In that case r.Host reflects the proxy's internal name and
// r.TLS is nil, so the public-facing scheme/host come from the
// X-Forwarded-Proto and X-Forwarded-Host headers.
//
// When the request reaches the service over direct TLS (r.TLS != nil),
// forwarded headers are ignored — the TLS connection itself proves the
// request was not forwarded by a proxy in a way the client could
// influence, so the standard scheme+r.Host reconstruction is correct.
func trustedSelfOrigin(r *http.Request) string {
	host := r.Host
	scheme := "https://"

	if r.TLS == nil {
		scheme = "http://"
		if forwardedProto := firstForwardedValue(r.Header, "X-Forwarded-Proto"); forwardedProto != "" {
			scheme = forwardedProto + "://"
		}
		if forwardedHost := firstForwardedValue(r.Header, "X-Forwarded-Host"); forwardedHost != "" {
			host = forwardedHost
		}
	}

	if host == "" {
		return ""
	}
	return scheme + host
}

func firstForwardedValue(h http.Header, name string) string {
	raw := h.Get(name)
	if raw == "" {
		return ""
	}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			return part
		}
	}
	return ""
}

func ClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	// Trusted proxy headers must be enabled via env var. Without it, the
	// limiter uses r.RemoteAddr only, which cannot be spoofed by the client.
	trustForwarded := trustForwardedHeaders()
	if railwayIP := strings.TrimSpace(r.Header.Get("X-Railway-Client-Ip")); trustForwarded && railwayIP != "" {
		return railwayIP
	}
	if flyIP := strings.TrimSpace(r.Header.Get("Fly-Client-IP")); trustForwarded && flyIP != "" {
		return flyIP
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); trustForwarded && realIP != "" {
		return realIP
	}
	if trustForwarded {
		if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
			parts := strings.Split(forwarded, ",")
			if len(parts) > 0 {
				return strings.TrimSpace(parts[0])
			}
		}
	}
	if host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr)); err == nil {
		return strings.TrimSpace(host)
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self' ws: wss:; font-src 'self' data:; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

type headerStrippingResponseWriter struct {
	http.ResponseWriter
	stripped map[string]struct{}
}

func (w *headerStrippingResponseWriter) WriteHeader(code int) {
	for h := range w.stripped {
		w.ResponseWriter.Header().Del(h)
	}
	w.ResponseWriter.WriteHeader(code)
}

func NewHeaderStrippingMiddleware(headers ...string) func(http.Handler) http.Handler {
	stripped := make(map[string]struct{}, len(headers))
	for _, h := range headers {
		stripped[http.CanonicalHeaderKey(h)] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(&headerStrippingResponseWriter{
				ResponseWriter: w,
				stripped:       stripped,
			}, r)
		})
	}
}

func trustForwardedHeaders() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv("TRUST_FORWARDED_HEADERS")))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

// equalFoldOrigin reports whether two origin URLs are equivalent for CSRF
// checking. Origins are case-insensitive and must match exactly (scheme +
// host [+ port]). Prefix matching is unsafe: an attacker controlling
// "evil.com" can prefix-match against an allow-listed "evil.com.attacker.tld"
// when only HasPrefix is used. This helper performs strict, normalized
// equality.
func equalFoldOrigin(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	return strings.EqualFold(a, b)
}
