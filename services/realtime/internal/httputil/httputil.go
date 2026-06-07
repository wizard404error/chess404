package httputil

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func EnvOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func ListenAddr(envKey string, defaultPort int) string {
	addr := strings.TrimSpace(os.Getenv(envKey))
	if addr != "" {
		return addr
	}
	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
		return net.JoinHostPort("", port)
	}
	return net.JoinHostPort("", itoa(defaultPort))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("httputil: failed to encode JSON response: %v", err)
	}
}

func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, map[string]any{"error": message})
}

func LimitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		next.ServeHTTP(w, r)
	})
}

func NowUTC() time.Time {
	return time.Now().UTC()
}

func ParseAllowedOrigins() []string {
	raw := EnvOrDefault("ALLOWED_ORIGINS", "")
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func IsOriginAllowed(origin string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, a := range allowed {
		if strings.EqualFold(origin, a) {
			return true
		}
	}
	return false
}

// RedactURLCredentials returns urlString with any embedded user-info password
// replaced by "REDACTED". URLs without user-info are returned unchanged.
// Malformed URLs return "<unparseable-url>" so we never log a raw secret.
func RedactURLCredentials(urlString string) string {
	if urlString == "" {
		return ""
	}
	u, err := url.Parse(urlString)
	if err != nil {
		return "<unparseable-url>"
	}
	if u.User == nil {
		return urlString
	}
	if _, hasPass := u.User.Password(); hasPass {
		u.User = url.UserPassword(u.User.Username(), "REDACTED")
	}
	return u.String()
}
