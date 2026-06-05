package httputil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type responseRecorder struct {
	http.ResponseWriter
	status int
	size   int
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.size += n
	return n, err
}

func generateRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

type RequestLog struct {
	Time       string `json:"time"`
	Level      string `json:"level"`
	Service    string `json:"service,omitempty"`
	RequestID  string `json:"request_id"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	Status     int    `json:"status"`
	DurationMs int64  `json:"duration_ms"`
	UserAgent  string `json:"user_agent"`
	IP         string `json:"ip"`
}

func WithLogging(serviceName string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := r.Header.Get("X-Request-Id")
		if reqID == "" {
			reqID = generateRequestID()
		}
		
		w.Header().Set("X-Request-Id", reqID)
		
		ctx := context.WithValue(r.Context(), "request_id", reqID)
		r = r.WithContext(ctx)

		recorder := &responseRecorder{ResponseWriter: w, status: http.StatusOK}

		// ClientIP would normally come from rate_limit package, but to avoid circular imports, 
		// we just grab the straightforward ones here or leave it to standard remoteAddr.
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.RemoteAddr
		}

		next.ServeHTTP(recorder, r)

		duration := time.Since(start).Milliseconds()

		logEntry := RequestLog{
			Time:       time.Now().UTC().Format(time.RFC3339Nano),
			Level:      "INFO",
			Service:    serviceName,
			RequestID:  reqID,
			Method:     r.Method,
			Path:       r.URL.Path,
			Status:     recorder.status,
			DurationMs: duration,
			UserAgent:  r.UserAgent(),
			IP:         ip,
		}

		if recorder.status >= 500 {
			logEntry.Level = "ERROR"
		} else if recorder.status >= 400 {
			logEntry.Level = "WARN"
		}

		logBytes, _ := json.Marshal(logEntry)
		fmt.Fprintln(os.Stdout, string(logBytes))
	})
}
