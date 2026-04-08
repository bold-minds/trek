package perf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bold-minds/trek"
)

var (
	serverURL = "http://localhost:8080"
	testOrgID = "org_perf_test"
	testEnvID = "env_perf_test"
	testToken = "perf-test-token"
)

func init() {
	if url := os.Getenv("TREK_SERVER_URL"); url != "" {
		serverURL = url
	}
}

func BenchmarkCreateSession(b *testing.B) {
	client := &http.Client{Timeout: 10 * time.Second}

	reqBody := trek.CreateSessionRequest{
		Selector:   trek.Selector{UserID: "bench_user"},
		Level:      trek.LevelDebug,
		TTLSeconds: 60,
	}
	body, _ := json.Marshal(reqBody)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("POST",
			fmt.Sprintf("%s/orgs/%s/envs/%s/sessions", serverURL, testOrgID, testEnvID),
			bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+testToken)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			b.Skipf("Server not available: %v", err)
		}
		resp.Body.Close()
	}
}

func BenchmarkGetActiveSessions(b *testing.B) {
	client := &http.Client{Timeout: 10 * time.Second}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET",
			fmt.Sprintf("%s/orgs/%s/envs/%s/active-sessions", serverURL, testOrgID, testEnvID),
			nil)
		req.Header.Set("Authorization", "Bearer "+testToken)

		resp, err := client.Do(req)
		if err != nil {
			b.Skipf("Server not available: %v", err)
		}
		resp.Body.Close()
	}
}

func BenchmarkEvaluatorDecision(b *testing.B) {
	now := time.Now()
	ctx := trek.RequestContext{
		UserID:    "user123",
		TenantID:  "tenant456",
		Route:     "/api/orders/123",
		RequestID: "req-abc",
	}

	sessions := make([]trek.Session, 100)
	for i := 0; i < 100; i++ {
		sessions[i] = trek.Session{
			ID:        fmt.Sprintf("sess_%d", i),
			Selector:  trek.Selector{UserID: fmt.Sprintf("user%d", i)},
			Level:     trek.LevelDebug,
			ExpiresAt: now.Add(10 * time.Minute),
			Labels:    map[string]string{},
		}
	}
	// Add one matching session
	sessions[50].Selector.UserID = "user123"

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		trek.Decide(now, "api", ctx, sessions)
	}
}

func TestLoadTest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load test in short mode")
	}

	concurrency := 50
	duration := 10 * time.Second

	var (
		totalRequests atomic.Int64
		errors        atomic.Int64
		latencies     sync.Map
	)

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
		},
	}

	done := make(chan struct{})
	cancel := func() { close(done) }

	var wg sync.WaitGroup

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for {
				select {
				case <-done:
					return
				default:
				}

				reqStart := time.Now()

				req, _ := http.NewRequest("GET",
					fmt.Sprintf("%s/orgs/%s/envs/%s/active-sessions", serverURL, testOrgID, testEnvID),
					nil)
				req.Header.Set("Authorization", "Bearer "+testToken)

				resp, err := client.Do(req)
				if err != nil {
					errors.Add(1)
					continue
				}
				resp.Body.Close()

				if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
					errors.Add(1)
				}

				totalRequests.Add(1)
				latency := time.Since(reqStart)
				latencies.Store(totalRequests.Load(), latency)
			}
		}(i)
	}

	time.Sleep(duration)
	cancel()
	wg.Wait()

	elapsed := time.Since(start)
	total := totalRequests.Load()
	errs := errors.Load()
	rps := float64(total) / elapsed.Seconds()

	// Calculate latency percentiles
	var allLatencies []time.Duration
	latencies.Range(func(key, value any) bool {
		allLatencies = append(allLatencies, value.(time.Duration))
		return true
	})

	var p50, p95, p99 time.Duration
	if len(allLatencies) > 0 {
		// Sort latencies
		for i := 0; i < len(allLatencies); i++ {
			for j := i + 1; j < len(allLatencies); j++ {
				if allLatencies[i] > allLatencies[j] {
					allLatencies[i], allLatencies[j] = allLatencies[j], allLatencies[i]
				}
			}
		}
		p50 = allLatencies[len(allLatencies)*50/100]
		p95 = allLatencies[len(allLatencies)*95/100]
		p99 = allLatencies[len(allLatencies)*99/100]
	}

	t.Logf("Load Test Results:")
	t.Logf("  Duration:     %v", elapsed)
	t.Logf("  Concurrency:  %d", concurrency)
	t.Logf("  Total Reqs:   %d", total)
	t.Logf("  Errors:       %d (%.2f%%)", errs, float64(errs)/float64(total)*100)
	t.Logf("  RPS:          %.2f", rps)
	t.Logf("  Latency p50:  %v", p50)
	t.Logf("  Latency p95:  %v", p95)
	t.Logf("  Latency p99:  %v", p99)

	// Assertions
	if rps < 100 {
		t.Logf("Warning: RPS below 100, got %.2f", rps)
	}
	if p99 > 100*time.Millisecond {
		t.Logf("Warning: p99 latency above 100ms, got %v", p99)
	}
}
