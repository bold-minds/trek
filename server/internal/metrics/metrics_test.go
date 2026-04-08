package metrics

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestGet_ReturnsSameInstance(t *testing.T) {
	m1 := Get()
	m2 := Get()

	if m1 != m2 {
		t.Error("Get() should return the same instance")
	}
}

func TestMetrics_IncrSessionsCreated(t *testing.T) {
	m := &Metrics{}

	m.IncrSessionsCreated()
	m.IncrSessionsCreated()
	m.IncrSessionsCreated()

	if got := m.SessionsCreated.Load(); got != 3 {
		t.Errorf("SessionsCreated = %d, want 3", got)
	}
}

func TestMetrics_IncrSessionsRevoked(t *testing.T) {
	m := &Metrics{}

	m.IncrSessionsRevoked()
	m.IncrSessionsRevoked()

	if got := m.SessionsRevoked.Load(); got != 2 {
		t.Errorf("SessionsRevoked = %d, want 2", got)
	}
}

func TestMetrics_IncrSessionsExpired(t *testing.T) {
	m := &Metrics{}

	m.IncrSessionsExpired(5)
	m.IncrSessionsExpired(10)

	if got := m.SessionsExpired.Load(); got != 15 {
		t.Errorf("SessionsExpired = %d, want 15", got)
	}
}

func TestMetrics_IncrRequestsTotal(t *testing.T) {
	m := &Metrics{}

	m.IncrRequestsTotal()

	if got := m.RequestsTotal.Load(); got != 1 {
		t.Errorf("RequestsTotal = %d, want 1", got)
	}
}

func TestMetrics_IncrRequestsMatched(t *testing.T) {
	m := &Metrics{}

	m.IncrRequestsMatched()
	m.IncrRequestsMatched()

	if got := m.RequestsMatched.Load(); got != 2 {
		t.Errorf("RequestsMatched = %d, want 2", got)
	}
}

func TestMetrics_IncrAuthFailures(t *testing.T) {
	m := &Metrics{}

	m.IncrAuthFailures()

	if got := m.AuthFailures.Load(); got != 1 {
		t.Errorf("AuthFailures = %d, want 1", got)
	}
}

func TestMetrics_IncrPollRequests(t *testing.T) {
	m := &Metrics{}

	m.IncrPollRequests()
	m.IncrPollRequests()
	m.IncrPollRequests()

	if got := m.PollRequests.Load(); got != 3 {
		t.Errorf("PollRequests = %d, want 3", got)
	}
}

func TestMetrics_SetActiveSessions(t *testing.T) {
	m := &Metrics{}

	m.SetActiveSessions(42)

	m.mu.RLock()
	got := m.activeSessions
	m.mu.RUnlock()

	if got != 42 {
		t.Errorf("activeSessions = %d, want 42", got)
	}

	m.SetActiveSessions(10)

	m.mu.RLock()
	got = m.activeSessions
	m.mu.RUnlock()

	if got != 10 {
		t.Errorf("activeSessions = %d, want 10", got)
	}
}

func TestMetrics_RecordLatency(t *testing.T) {
	m := &Metrics{}

	m.RecordLatency(10 * time.Millisecond)
	m.RecordLatency(20 * time.Millisecond)
	m.RecordLatency(30 * time.Millisecond)

	m.latencyMu.Lock()
	got := len(m.requestLatencies)
	m.latencyMu.Unlock()

	if got != 3 {
		t.Errorf("requestLatencies count = %d, want 3", got)
	}
}

func TestMetrics_RecordLatency_CapAt1000(t *testing.T) {
	m := &Metrics{}

	for i := 0; i < 1100; i++ {
		m.RecordLatency(time.Millisecond)
	}

	m.latencyMu.Lock()
	got := len(m.requestLatencies)
	m.latencyMu.Unlock()

	if got != 1000 {
		t.Errorf("requestLatencies count = %d, want 1000 (capped)", got)
	}
}

func TestCalculatePercentiles_EmptySamples(t *testing.T) {
	p50, p95, p99 := calculatePercentiles(nil)

	if p50 != 0 || p95 != 0 || p99 != 0 {
		t.Errorf("empty samples: p50=%f, p95=%f, p99=%f, want all zeros", p50, p95, p99)
	}
}

func TestCalculatePercentiles_SingleSample(t *testing.T) {
	samples := []float64{100.0}
	p50, p95, p99 := calculatePercentiles(samples)

	if p50 != 100.0 {
		t.Errorf("p50 = %f, want 100.0", p50)
	}
	if p95 != 100.0 {
		t.Errorf("p95 = %f, want 100.0", p95)
	}
	if p99 != 100.0 {
		t.Errorf("p99 = %f, want 100.0", p99)
	}
}

func TestCalculatePercentiles_MultipleSamples(t *testing.T) {
	samples := make([]float64, 100)
	for i := range samples {
		samples[i] = float64(i + 1)
	}

	p50, p95, p99 := calculatePercentiles(samples)

	// The implementation uses index = len*percentile/100
	// For 100 samples: p50 index=50, p95 index=95, p99 index=99
	// Values at those indices are 51, 96, 100 (1-indexed data)
	if p50 != 51 {
		t.Errorf("p50 = %f, want 51", p50)
	}
	if p95 != 96 {
		t.Errorf("p95 = %f, want 96", p95)
	}
	if p99 != 100 {
		t.Errorf("p99 = %f, want 100", p99)
	}
}

func TestMetrics_Handler_JSON(t *testing.T) {
	m := &Metrics{}
	m.IncrSessionsCreated()
	m.IncrSessionsCreated()
	m.SetActiveSessions(5)
	m.RecordLatency(10 * time.Millisecond)

	handler := m.Handler()

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %s, want application/json", contentType)
	}

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	counters, ok := data["counters"].(map[string]any)
	if !ok {
		t.Fatal("missing counters in response")
	}

	if counters["sessions_created"].(float64) != 2 {
		t.Errorf("sessions_created = %v, want 2", counters["sessions_created"])
	}

	gauges, ok := data["gauges"].(map[string]any)
	if !ok {
		t.Fatal("missing gauges in response")
	}

	if gauges["active_sessions"].(float64) != 5 {
		t.Errorf("active_sessions = %v, want 5", gauges["active_sessions"])
	}
}

func TestMetrics_Handler_Prometheus(t *testing.T) {
	m := &Metrics{}
	m.IncrSessionsCreated()
	m.IncrRequestsTotal()
	m.SetActiveSessions(10)

	handler := m.Handler()

	req := httptest.NewRequest("GET", "/metrics?format=prometheus", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/plain") {
		t.Errorf("Content-Type = %s, want text/plain", contentType)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	bodyStr := string(body)

	expectedMetrics := []string{
		"trek_sessions_created_total",
		"trek_requests_total",
		"trek_active_sessions",
		"trek_request_latency_ms_p50",
		"trek_request_latency_ms_p95",
		"trek_request_latency_ms_p99",
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(bodyStr, metric) {
			t.Errorf("missing metric %s in prometheus output", metric)
		}
	}
}

func TestMetricsMiddleware(t *testing.T) {
	originalGlobal := global
	global = &Metrics{}
	defer func() { global = originalGlobal }()

	handler := MetricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if Get().RequestsTotal.Load() != 1 {
		t.Errorf("RequestsTotal = %d, want 1", Get().RequestsTotal.Load())
	}

	Get().latencyMu.Lock()
	latencyCount := len(Get().requestLatencies)
	Get().latencyMu.Unlock()

	if latencyCount != 1 {
		t.Errorf("latency samples = %d, want 1", latencyCount)
	}
}

func TestFiberMetricsMiddleware(t *testing.T) {
	originalGlobal := global
	global = &Metrics{}
	defer func() { global = originalGlobal }()

	app := fiber.New()
	app.Use(FiberMetricsMiddleware)
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if Get().RequestsTotal.Load() != 1 {
		t.Errorf("RequestsTotal = %d, want 1", Get().RequestsTotal.Load())
	}
}

func TestFiberHandler_JSON(t *testing.T) {
	originalGlobal := global
	global = &Metrics{}
	global.IncrSessionsCreated()
	global.SetActiveSessions(3)
	defer func() { global = originalGlobal }()

	app := fiber.New()
	app.Get("/metrics", FiberHandler)

	req := httptest.NewRequest("GET", "/metrics", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	counters := data["counters"].(map[string]any)
	if counters["sessions_created"].(float64) != 1 {
		t.Errorf("sessions_created = %v, want 1", counters["sessions_created"])
	}
}

func TestFiberHandler_Prometheus(t *testing.T) {
	originalGlobal := global
	global = &Metrics{}
	global.IncrSessionsRevoked()
	defer func() { global = originalGlobal }()

	app := fiber.New()
	app.Get("/metrics", FiberHandler)

	req := httptest.NewRequest("GET", "/metrics?format=prometheus", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	if !strings.Contains(string(body), "trek_sessions_revoked_total") {
		t.Error("missing trek_sessions_revoked_total in prometheus output")
	}
}

func TestMetrics_jsonData(t *testing.T) {
	m := &Metrics{}
	m.IncrSessionsCreated()
	m.IncrSessionsRevoked()
	m.IncrSessionsExpired(3)
	m.IncrRequestsTotal()
	m.IncrRequestsMatched()
	m.IncrAuthFailures()
	m.IncrPollRequests()
	m.SetActiveSessions(7)
	m.RecordLatency(50 * time.Millisecond)

	data := m.jsonData()

	counters := data["counters"].(map[string]int64)
	if counters["sessions_created"] != 1 {
		t.Errorf("sessions_created = %d, want 1", counters["sessions_created"])
	}
	if counters["sessions_revoked"] != 1 {
		t.Errorf("sessions_revoked = %d, want 1", counters["sessions_revoked"])
	}
	if counters["sessions_expired"] != 3 {
		t.Errorf("sessions_expired = %d, want 3", counters["sessions_expired"])
	}
	if counters["requests_total"] != 1 {
		t.Errorf("requests_total = %d, want 1", counters["requests_total"])
	}
	if counters["requests_matched"] != 1 {
		t.Errorf("requests_matched = %d, want 1", counters["requests_matched"])
	}
	if counters["auth_failures"] != 1 {
		t.Errorf("auth_failures = %d, want 1", counters["auth_failures"])
	}
	if counters["poll_requests"] != 1 {
		t.Errorf("poll_requests = %d, want 1", counters["poll_requests"])
	}

	gauges := data["gauges"].(map[string]int64)
	if gauges["active_sessions"] != 7 {
		t.Errorf("active_sessions = %d, want 7", gauges["active_sessions"])
	}
}

func TestMetrics_prometheusString(t *testing.T) {
	m := &Metrics{}
	m.IncrSessionsCreated()
	m.SetActiveSessions(5)

	output := m.prometheusString()

	expectedLines := []string{
		"# TYPE trek_sessions_created_total counter",
		"trek_sessions_created_total 1",
		"# TYPE trek_active_sessions gauge",
		"trek_active_sessions 5",
		"# TYPE trek_request_latency_ms_p50 gauge",
	}

	for _, line := range expectedLines {
		if !strings.Contains(output, line) {
			t.Errorf("missing line in prometheus output: %s", line)
		}
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{-1, "-1"},
		{12345, "12345"},
		{-9999, "-9999"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := itoa(tt.input)
			if got != tt.want {
				t.Errorf("itoa(%d) = %s, want %s", tt.input, got, tt.want)
			}
		})
	}
}

func TestFtoa(t *testing.T) {
	tests := []struct {
		input float64
		want  string
	}{
		{0, "0"},
		{1.5, "1.5"},
		{-1.5, "-1.5"},
		{123.456, "123.456"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := ftoa(tt.input)
			if got != tt.want {
				t.Errorf("ftoa(%f) = %s, want %s", tt.input, got, tt.want)
			}
		})
	}
}

func TestWriteMetric(t *testing.T) {
	w := httptest.NewRecorder()
	writeMetric(w, "test_metric", "counter", 42)

	body := w.Body.String()
	if !strings.Contains(body, "# TYPE test_metric counter") {
		t.Error("missing TYPE line")
	}
	if !strings.Contains(body, "test_metric 42") {
		t.Error("missing metric value line")
	}
}

func TestWriteMetricFloat(t *testing.T) {
	w := httptest.NewRecorder()
	writeMetricFloat(w, "test_gauge", "gauge", 3.14)

	body := w.Body.String()
	if !strings.Contains(body, "# TYPE test_gauge gauge") {
		t.Error("missing TYPE line")
	}
	if !strings.Contains(body, "test_gauge 3.14") {
		t.Error("missing metric value line")
	}
}

func TestWriteMetricStr(t *testing.T) {
	var b strings.Builder
	writeMetricStr(&b, "str_metric", "counter", 100)

	output := b.String()
	if !strings.Contains(output, "# TYPE str_metric counter") {
		t.Error("missing TYPE line")
	}
	if !strings.Contains(output, "str_metric 100") {
		t.Error("missing metric value line")
	}
}

func TestWriteMetricFloatStr(t *testing.T) {
	var b strings.Builder
	writeMetricFloatStr(&b, "float_metric", "gauge", 2.718)

	output := b.String()
	if !strings.Contains(output, "# TYPE float_metric gauge") {
		t.Error("missing TYPE line")
	}
	if !strings.Contains(output, "float_metric 2.718") {
		t.Error("missing metric value line")
	}
}

func TestMetrics_ConcurrentAccess(t *testing.T) {
	m := &Metrics{}

	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				m.IncrSessionsCreated()
				m.IncrRequestsTotal()
				m.SetActiveSessions(int64(j))
				m.RecordLatency(time.Millisecond)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if m.SessionsCreated.Load() != 1000 {
		t.Errorf("SessionsCreated = %d, want 1000", m.SessionsCreated.Load())
	}

	if m.RequestsTotal.Load() != 1000 {
		t.Errorf("RequestsTotal = %d, want 1000", m.RequestsTotal.Load())
	}
}
