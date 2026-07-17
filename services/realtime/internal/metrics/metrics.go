package metrics

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	requestCount   = newCounterVec("http_requests_total", "Total HTTP requests", []string{"method", "path", "status"})
	requestLatency = newHistogramVec("http_request_duration_ms", "HTTP request duration in milliseconds", []string{"method", "path"}, []float64{5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000})
	requestInFlight = newGauge("http_requests_in_flight", "HTTP requests currently in flight")
	wsActive = newGauge("ws_connections_active", "Active WebSocket connections")
	matchesActive = newGauge("matches_active", "Currently active matches")
	matchesCreated = newCounter("matches_created_total", "Total matches created")
	queueDepth = newGauge("queue_depth", "Current matchmaking queue depth")
	PushSnapshotDrops = newCounter("push_snapshot_drops_total", "Snapshots dropped due to full subscriber buffer")
)

func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Require a valid internal service token for the metrics endpoint
		// to prevent leaking operational data to unauthenticated clients.
		token := strings.TrimSpace(r.Header.Get("Authorization"))
		expected := strings.TrimSpace(os.Getenv("INTERNAL_SERVICE_TOKEN"))
		if expected != "" {
			const prefix = "Bearer "
			if strings.HasPrefix(token, prefix) {
				token = strings.TrimSpace(strings.TrimPrefix(token, prefix))
			}
			if token == "" || token != expected {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		data := Collect()
		w.Write(data)
	})
}

func RecordRequest(method, path string, status int, durationMs int64) {
	statusStr := http.StatusText(status)
	requestCount.WithLabelValues(method, path, statusStr).Inc()
	requestLatency.WithLabelValues(method, path).Observe(float64(durationMs))
}

func IncInFlight()  { requestInFlight.Inc() }
func DecInFlight()  { requestInFlight.Dec() }
func IncWS()        { wsActive.Inc() }
func DecWS()        { wsActive.Dec() }
func IncMatch()     { matchesActive.Inc() }
func DecMatch()     { matchesActive.Dec() }
func IncCreated()   { matchesCreated.Inc() }
func SetQueueDepth(d float64) { queueDepth.Set(d) }

type counterVec struct {
	name   string
	help   string
	labels []string
	mu     sync.Mutex
	counts map[string]*counter
}

func newCounterVec(name, help string, labels []string) *counterVec {
	return &counterVec{name: name, help: help, labels: labels, counts: make(map[string]*counter)}
}

func (cv *counterVec) WithLabelValues(vals ...string) *counter {
	key := ""
	for i, v := range vals {
		if i > 0 {
			key += ","
		}
		key += v
	}
	cv.mu.Lock()
	defer cv.mu.Unlock()
	if c, ok := cv.counts[key]; ok {
		return c
	}
	c := &counter{name: cv.name, labels: make(map[string]string)}
	for i, label := range cv.labels {
		if i < len(vals) {
			c.labels[label] = vals[i]
		}
	}
	cv.counts[key] = c
	return c
}

type counter struct {
	name   string
	labels map[string]string
	value  float64
	mu     sync.Mutex
}

func (c *counter) Inc()           { c.Add(1) }
func (c *counter) Add(v float64)  { c.mu.Lock(); c.value += v; c.mu.Unlock() }
func (c *counter) Value() float64 { c.mu.Lock(); defer c.mu.Unlock(); return c.value }

type gauge struct {
	name  string
	value float64
	mu    sync.Mutex
}

func newGauge(name, help string) *gauge {
	return &gauge{name: name}
}

func (g *gauge) Inc()           { g.Add(1) }
func (g *gauge) Dec()           { g.Add(-1) }
func (g *gauge) Add(v float64)  { g.mu.Lock(); g.value += v; g.mu.Unlock() }
func (g *gauge) Set(v float64)  { g.mu.Lock(); g.value = v; g.mu.Unlock() }
func (g *gauge) Value() float64 { g.mu.Lock(); defer g.mu.Unlock(); return g.value }

type standaloneCounter struct {
	name  string
	value float64
	mu    sync.Mutex
}

func newCounter(name, help string) *standaloneCounter {
	return &standaloneCounter{name: name}
}

func NewCounter(name, help string) *standaloneCounter {
	return &standaloneCounter{name: name}
}

func (c *standaloneCounter) Inc()           { c.Add(1) }
func (c *standaloneCounter) Add(v float64)  { c.mu.Lock(); c.value += v; c.mu.Unlock() }
func (c *standaloneCounter) Value() float64 { c.mu.Lock(); defer c.mu.Unlock(); return c.value }

type histogramVec struct {
	name   string
	help   string
	labels []string
	bounds []float64
	mu     sync.Mutex
	hists  map[string]*histogram
}

func newHistogramVec(name, help string, labels []string, bounds []float64) *histogramVec {
	return &histogramVec{name: name, help: help, labels: labels, bounds: bounds, hists: make(map[string]*histogram)}
}

func (hv *histogramVec) WithLabelValues(vals ...string) *histogram {
	key := ""
	for i, v := range vals {
		if i > 0 {
			key += ","
		}
		key += v
	}
	hv.mu.Lock()
	defer hv.mu.Unlock()
	if h, ok := hv.hists[key]; ok {
		return h
	}
	h := &histogram{bounds: hv.bounds, buckets: make([]float64, len(hv.bounds)+1)}
	hv.hists[key] = h
	return h
}

type histogram struct {
	bounds  []float64
	buckets []float64
	count   float64
	sum     float64
	mu      sync.Mutex
}

func (h *histogram) Observe(v float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.count++
	h.sum += v
	for i, bound := range h.bounds {
		if v <= bound {
			h.buckets[i]++
			return
		}
	}
	h.buckets[len(h.bounds)]++
}

func Collect() []byte {
	var out []byte
	metrics := []struct {
		name string
		help string
		typ  string
		fn   func()
	}{
		{"http_requests_total", "Total HTTP requests", "counter", nil},
		{"http_request_duration_ms", "HTTP request duration in milliseconds", "histogram", nil},
		{"http_requests_in_flight", "HTTP requests currently in flight", "gauge", nil},
		{"ws_connections_active", "Active WebSocket connections", "gauge", nil},
		{"matches_active", "Currently active matches", "gauge", nil},
		{"matches_created_total", "Total matches created", "counter", nil},
		{"queue_depth", "Current matchmaking queue depth", "gauge", nil},
	}
	_ = metrics

	out = append(out, "# HELP http_requests_total Total HTTP requests\n"...)
	out = append(out, "# TYPE http_requests_total counter\n"...)
	requestCount.mu.Lock()
	for key, c := range requestCount.counts {
		c.mu.Lock()
		out = append(out, "http_requests_total{"+key+"} "+formatFloat(c.value)+"\n"...)
		c.mu.Unlock()
	}
	requestCount.mu.Unlock()

	out = append(out, "# HELP http_requests_in_flight HTTP requests currently in flight\n"...)
	out = append(out, "# TYPE http_requests_in_flight gauge\n"...)
	out = append(out, "http_requests_in_flight "+formatFloat(requestInFlight.Value())+"\n"...)

	out = append(out, "# HELP ws_connections_active Active WebSocket connections\n"...)
	out = append(out, "# TYPE ws_connections_active gauge\n"...)
	out = append(out, "ws_connections_active "+formatFloat(wsActive.Value())+"\n"...)

	out = append(out, "# HELP matches_active Currently active matches\n"...)
	out = append(out, "# TYPE matches_active gauge\n"...)
	out = append(out, "matches_active "+formatFloat(matchesActive.Value())+"\n"...)

	out = append(out, "# HELP matches_created_total Total matches created\n"...)
	out = append(out, "# TYPE matches_created_total counter\n"...)
	out = append(out, "matches_created_total "+formatFloat(matchesCreated.Value())+"\n"...)

	out = append(out, "# HELP queue_depth Current matchmaking queue depth\n"...)
	out = append(out, "# TYPE queue_depth gauge\n"...)
	out = append(out, "queue_depth "+formatFloat(queueDepth.Value())+"\n"...)

	out = append(out, "# HELP http_request_duration_ms HTTP request duration in milliseconds\n"...)
	out = append(out, "# TYPE http_request_duration_ms histogram\n"...)
	requestLatency.mu.Lock()
	for key, h := range requestLatency.hists {
		h.mu.Lock()
		for i, bound := range h.bounds {
			out = append(out, "http_request_duration_ms_bucket{"+key+",le="+formatFloat(bound)+"} "+formatFloat(h.buckets[i])+"\n"...)
		}
		out = append(out, "http_request_duration_ms_bucket{"+key+",le=+Inf} "+formatFloat(h.count)+"\n"...)
		out = append(out, "http_request_duration_ms_sum{"+key+"} "+formatFloat(h.sum)+"\n"...)
		out = append(out, "http_request_duration_ms_count{"+key+"} "+formatFloat(h.count)+"\n"...)
		h.mu.Unlock()
	}
	requestLatency.mu.Unlock()

	return out
}

func formatFloat(f float64) string {
	if f == 0 {
		return "0"
	}
	buf := make([]byte, 0, 20)
	if f < 0 {
		buf = append(buf, '-')
		f = -f
	}

	intPart := int64(f)
	frac := f - float64(intPart)

	if intPart == 0 {
		buf = append(buf, '0')
	} else {
		tmp := make([]byte, 0, 20)
		for intPart > 0 {
			tmp = append(tmp, byte('0'+intPart%10))
			intPart /= 10
		}
		for i := len(tmp) - 1; i >= 0; i-- {
			buf = append(buf, tmp[i])
		}
	}

	if frac > 0 {
		buf = append(buf, '.')
		for i := 0; i < 3 && frac > 0; i++ {
			frac *= 10
			digit := int(frac)
			buf = append(buf, byte('0'+digit))
			frac -= float64(digit)
		}
	}

	return string(buf)
}

func init() {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			slog.Debug("metrics snapshot",
				"active_matches", int64(matchesActive.Value()),
				"ws_connections", int64(wsActive.Value()),
				"in_flight", int64(requestInFlight.Value()),
				"queue_depth", int64(queueDepth.Value()),
			)
		}
	}()
}
