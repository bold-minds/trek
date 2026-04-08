package metrics

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Metrics collects Trek server metrics.
type Metrics struct {
	// Counters
	SessionsCreated atomic.Int64
	SessionsRevoked atomic.Int64
	SessionsExpired atomic.Int64
	RequestsTotal   atomic.Int64
	RequestsMatched atomic.Int64
	AuthFailures    atomic.Int64
	PollRequests    atomic.Int64

	// Gauges
	mu             sync.RWMutex
	activeSessions int64

	// Histograms (simplified)
	requestLatencies []float64
	latencyMu        sync.Mutex
}

// Global metrics instance
var global = &Metrics{}

// Get returns the global metrics instance.
func Get() *Metrics {
	return global
}

// IncrSessionsCreated increments the sessions created counter.
func (m *Metrics) IncrSessionsCreated() {
	m.SessionsCreated.Add(1)
}

// IncrSessionsRevoked increments the sessions revoked counter.
func (m *Metrics) IncrSessionsRevoked() {
	m.SessionsRevoked.Add(1)
}

// IncrSessionsExpired increments by the given count.
func (m *Metrics) IncrSessionsExpired(count int64) {
	m.SessionsExpired.Add(count)
}

// IncrRequestsTotal increments total requests.
func (m *Metrics) IncrRequestsTotal() {
	m.RequestsTotal.Add(1)
}

// IncrRequestsMatched increments matched requests.
func (m *Metrics) IncrRequestsMatched() {
	m.RequestsMatched.Add(1)
}

// IncrAuthFailures increments auth failures.
func (m *Metrics) IncrAuthFailures() {
	m.AuthFailures.Add(1)
}

// IncrPollRequests increments poll requests.
func (m *Metrics) IncrPollRequests() {
	m.PollRequests.Add(1)
}

// SetActiveSessions sets the active sessions gauge.
func (m *Metrics) SetActiveSessions(count int64) {
	m.mu.Lock()
	m.activeSessions = count
	m.mu.Unlock()
}

// RecordLatency records a request latency.
func (m *Metrics) RecordLatency(duration time.Duration) {
	m.latencyMu.Lock()
	defer m.latencyMu.Unlock()

	m.requestLatencies = append(m.requestLatencies, duration.Seconds()*1000)

	// Keep only last 1000 samples
	if len(m.requestLatencies) > 1000 {
		m.requestLatencies = m.requestLatencies[len(m.requestLatencies)-1000:]
	}
}

// Handler returns an HTTP handler for metrics endpoint.
func (m *Metrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		format := r.URL.Query().Get("format")

		if format == "prometheus" {
			m.writePrometheus(w)
		} else {
			m.writeJSON(w)
		}
	}
}

func (m *Metrics) writeJSON(w http.ResponseWriter) {
	m.mu.RLock()
	activeSessions := m.activeSessions
	m.mu.RUnlock()

	m.latencyMu.Lock()
	p50, p95, p99 := calculatePercentiles(m.requestLatencies)
	m.latencyMu.Unlock()

	data := map[string]any{
		"counters": map[string]int64{
			"sessions_created": m.SessionsCreated.Load(),
			"sessions_revoked": m.SessionsRevoked.Load(),
			"sessions_expired": m.SessionsExpired.Load(),
			"requests_total":   m.RequestsTotal.Load(),
			"requests_matched": m.RequestsMatched.Load(),
			"auth_failures":    m.AuthFailures.Load(),
			"poll_requests":    m.PollRequests.Load(),
		},
		"gauges": map[string]int64{
			"active_sessions": activeSessions,
		},
		"latency_ms": map[string]float64{
			"p50": p50,
			"p95": p95,
			"p99": p99,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (m *Metrics) writePrometheus(w http.ResponseWriter) {
	m.mu.RLock()
	activeSessions := m.activeSessions
	m.mu.RUnlock()

	m.latencyMu.Lock()
	p50, p95, p99 := calculatePercentiles(m.requestLatencies)
	m.latencyMu.Unlock()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	writeMetric(w, "trek_sessions_created_total", "counter", m.SessionsCreated.Load())
	writeMetric(w, "trek_sessions_revoked_total", "counter", m.SessionsRevoked.Load())
	writeMetric(w, "trek_sessions_expired_total", "counter", m.SessionsExpired.Load())
	writeMetric(w, "trek_requests_total", "counter", m.RequestsTotal.Load())
	writeMetric(w, "trek_requests_matched_total", "counter", m.RequestsMatched.Load())
	writeMetric(w, "trek_auth_failures_total", "counter", m.AuthFailures.Load())
	writeMetric(w, "trek_poll_requests_total", "counter", m.PollRequests.Load())
	writeMetric(w, "trek_active_sessions", "gauge", activeSessions)
	writeMetricFloat(w, "trek_request_latency_ms_p50", "gauge", p50)
	writeMetricFloat(w, "trek_request_latency_ms_p95", "gauge", p95)
	writeMetricFloat(w, "trek_request_latency_ms_p99", "gauge", p99)
}

func writeMetric(w http.ResponseWriter, name, typ string, value int64) {
	w.Write([]byte("# TYPE " + name + " " + typ + "\n"))
	w.Write([]byte(name + " " + itoa(value) + "\n"))
}

func writeMetricFloat(w http.ResponseWriter, name, typ string, value float64) {
	w.Write([]byte("# TYPE " + name + " " + typ + "\n"))
	w.Write([]byte(name + " " + ftoa(value) + "\n"))
}

func itoa(i int64) string {
	return strconv.FormatInt(i, 10)
}

func ftoa(f float64) string {
	b, _ := json.Marshal(f)
	return string(b)
}

func calculatePercentiles(samples []float64) (p50, p95, p99 float64) {
	if len(samples) == 0 {
		return 0, 0, 0
	}

	sorted := make([]float64, len(samples))
	copy(sorted, samples)
	sort.Float64s(sorted)

	p50 = sorted[len(sorted)*50/100]
	p95 = sorted[len(sorted)*95/100]
	p99 = sorted[len(sorted)*99/100]

	return
}

// MetricsMiddleware records metrics for each request.
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		Get().IncrRequestsTotal()

		next.ServeHTTP(w, r)

		Get().RecordLatency(time.Since(start))
	})
}

// FiberMetricsMiddleware records metrics for Fiber requests.
func FiberMetricsMiddleware(c *fiber.Ctx) error {
	start := time.Now()
	Get().IncrRequestsTotal()
	err := c.Next()
	Get().RecordLatency(time.Since(start))
	return err
}

// FiberHandler handles metrics endpoint for Fiber.
func FiberHandler(c *fiber.Ctx) error {
	format := c.Query("format")

	if format == "prometheus" {
		c.Set("Content-Type", "text/plain; charset=utf-8")
		return c.SendString(Get().prometheusString())
	}

	return c.JSON(Get().jsonData())
}

func (m *Metrics) jsonData() map[string]any {
	m.mu.RLock()
	activeSessions := m.activeSessions
	m.mu.RUnlock()

	m.latencyMu.Lock()
	p50, p95, p99 := calculatePercentiles(m.requestLatencies)
	m.latencyMu.Unlock()

	return map[string]any{
		"counters": map[string]int64{
			"sessions_created": m.SessionsCreated.Load(),
			"sessions_revoked": m.SessionsRevoked.Load(),
			"sessions_expired": m.SessionsExpired.Load(),
			"requests_total":   m.RequestsTotal.Load(),
			"requests_matched": m.RequestsMatched.Load(),
			"auth_failures":    m.AuthFailures.Load(),
			"poll_requests":    m.PollRequests.Load(),
		},
		"gauges": map[string]int64{
			"active_sessions": activeSessions,
		},
		"latency_ms": map[string]float64{
			"p50": p50,
			"p95": p95,
			"p99": p99,
		},
	}
}

func (m *Metrics) prometheusString() string {
	m.mu.RLock()
	activeSessions := m.activeSessions
	m.mu.RUnlock()

	m.latencyMu.Lock()
	p50, p95, p99 := calculatePercentiles(m.requestLatencies)
	m.latencyMu.Unlock()

	var b strings.Builder
	writeMetricStr(&b, "trek_sessions_created_total", "counter", m.SessionsCreated.Load())
	writeMetricStr(&b, "trek_sessions_revoked_total", "counter", m.SessionsRevoked.Load())
	writeMetricStr(&b, "trek_sessions_expired_total", "counter", m.SessionsExpired.Load())
	writeMetricStr(&b, "trek_requests_total", "counter", m.RequestsTotal.Load())
	writeMetricStr(&b, "trek_requests_matched_total", "counter", m.RequestsMatched.Load())
	writeMetricStr(&b, "trek_auth_failures_total", "counter", m.AuthFailures.Load())
	writeMetricStr(&b, "trek_poll_requests_total", "counter", m.PollRequests.Load())
	writeMetricStr(&b, "trek_active_sessions", "gauge", activeSessions)
	writeMetricFloatStr(&b, "trek_request_latency_ms_p50", "gauge", p50)
	writeMetricFloatStr(&b, "trek_request_latency_ms_p95", "gauge", p95)
	writeMetricFloatStr(&b, "trek_request_latency_ms_p99", "gauge", p99)
	return b.String()
}

func writeMetricStr(b *strings.Builder, name, typ string, value int64) {
	b.WriteString("# TYPE " + name + " " + typ + "\n")
	b.WriteString(name + " " + itoa(value) + "\n")
}

func writeMetricFloatStr(b *strings.Builder, name, typ string, value float64) {
	b.WriteString("# TYPE " + name + " " + typ + "\n")
	b.WriteString(name + " " + ftoa(value) + "\n")
}
