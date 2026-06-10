package httputil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"github.com/chess404/realtime/internal/metrics"
)

type contextKey string

const requestIDKey contextKey = "request_id"

func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func generateRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

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

func WithLogging(serviceName string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := r.Header.Get("X-Request-Id")
		if reqID == "" {
			reqID = generateRequestID()
		}

		w.Header().Set("X-Request-Id", reqID)
		ctx := WithRequestID(r.Context(), reqID)
		r = r.WithContext(ctx)

		recorder := &responseRecorder{ResponseWriter: w, status: http.StatusOK}

		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.RemoteAddr
		}

		next.ServeHTTP(recorder, r)

		duration := time.Since(start).Milliseconds()

		attrs := []any{
			"requestId", reqID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration_ms", duration,
			"ip", ip,
		}

		metrics.RecordRequest(r.Method, r.URL.Path, recorder.status, duration)

		if recorder.status >= 500 {
			slog.Error("http request", attrs...)
		} else if recorder.status >= 400 {
			slog.Warn("http request", attrs...)
		} else {
			slog.Info("http request", attrs...)
		}
	})
}
