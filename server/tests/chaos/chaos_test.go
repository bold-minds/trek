package chaos

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bold-minds/trek"
)

func TestSDKGracefulDegradationOnServerDown(t *testing.T) {
	// Start a server that will be shut down
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"revision":"r1","server_time":"2024-01-01T00:00:00Z","sessions":[]}`))
	}))

	client := trek.NewClient(server.URL, "token", "org1", "prod")
	cache := trek.NewCache(client, "test-service", 100*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cache.Start(ctx)

	// Wait for initial fetch
	time.Sleep(50 * time.Millisecond)

	initialRequests := requestCount.Load()
	if initialRequests == 0 {
		t.Fatal("expected at least one request")
	}

	// Shut down server - simulating control plane outage
	server.Close()

	// Wait for a few poll cycles
	time.Sleep(300 * time.Millisecond)

	// SDK should still work - just return empty sessions
	sessions := cache.GetSessions()

	// The key test: SDK doesn't crash, returns cached (empty) sessions
	t.Logf("Sessions after server down: %d", len(sessions))

	// Test that evaluation still works with empty cache
	decision := trek.Decide(time.Now(), "test-service", trek.RequestContext{
		UserID: "user123",
		Route:  "/api/test",
	}, sessions)

	// Should not match (no sessions)
	if decision.Matched {
		t.Error("expected no match with empty sessions")
	}

	if decision.ReasonCode != trek.ReasonNoMatch {
		t.Errorf("expected NO_MATCH, got %s", decision.ReasonCode)
	}

	cache.Stop()
}

func TestSDKRecoveryAfterServerRestart(t *testing.T) {
	var serverUp atomic.Bool
	serverUp.Store(true)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !serverUp.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"revision":"r1","server_time":"2024-01-01T00:00:00Z","sessions":[
			{"id":"s1","selector":{"user_id":"user123"},"level":"debug","expires_at":"2099-01-01T00:00:00Z","token_mode":"none","caps":{},"labels":{}}
		]}`))
	}))
	defer server.Close()

	client := trek.NewClient(server.URL, "token", "org1", "prod")
	cache := trek.NewCache(client, "test-service", 100*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cache.Start(ctx)

	// Wait for initial fetch
	time.Sleep(50 * time.Millisecond)

	// Should have sessions
	sessions := cache.GetSessions()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	// Simulate server going down
	serverUp.Store(false)

	// Wait for a poll to fail
	time.Sleep(200 * time.Millisecond)

	// Should still have cached sessions
	sessions = cache.GetSessions()
	if len(sessions) != 1 {
		t.Errorf("expected cached session to remain, got %d", len(sessions))
	}

	// Bring server back up
	serverUp.Store(true)

	// Wait for recovery
	time.Sleep(200 * time.Millisecond)

	// Should still work
	sessions = cache.GetSessions()
	if len(sessions) != 1 {
		t.Errorf("expected session after recovery, got %d", len(sessions))
	}

	cache.Stop()
}

func TestEvaluatorDoesNotPanicOnMalformedData(t *testing.T) {
	testCases := []struct {
		name     string
		sessions []trek.Session
		ctx      trek.RequestContext
	}{
		{
			name:     "nil sessions",
			sessions: nil,
			ctx:      trek.RequestContext{UserID: "u1"},
		},
		{
			name:     "empty sessions",
			sessions: []trek.Session{},
			ctx:      trek.RequestContext{UserID: "u1"},
		},
		{
			name: "session with empty selector",
			sessions: []trek.Session{
				{ID: "s1", Selector: trek.Selector{}, Level: trek.LevelDebug, ExpiresAt: time.Now().Add(time.Hour)},
			},
			ctx: trek.RequestContext{UserID: "u1"},
		},
		{
			name: "session with nil labels",
			sessions: []trek.Session{
				{ID: "s1", Selector: trek.Selector{UserID: "u1"}, Level: trek.LevelDebug, ExpiresAt: time.Now().Add(time.Hour), Labels: nil},
			},
			ctx: trek.RequestContext{UserID: "u1"},
		},
		{
			name: "request context with nil custom",
			sessions: []trek.Session{
				{ID: "s1", Selector: trek.Selector{UserID: "u1"}, Level: trek.LevelDebug, ExpiresAt: time.Now().Add(time.Hour)},
			},
			ctx: trek.RequestContext{UserID: "u1", Custom: nil},
		},
		{
			name: "zero time in session",
			sessions: []trek.Session{
				{ID: "s1", Selector: trek.Selector{UserID: "u1"}, Level: trek.LevelDebug, ExpiresAt: time.Time{}},
			},
			ctx: trek.RequestContext{UserID: "u1"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic: %v", r)
				}
			}()

			// Should not panic
			decision := trek.Decide(time.Now(), "test", tc.ctx, tc.sessions)

			// Just verify we got some response
			_ = decision.Matched
			_ = decision.ReasonCode
		})
	}
}

func TestHighConcurrencyEvaluation(t *testing.T) {
	sessions := []trek.Session{
		{ID: "s1", Selector: trek.Selector{UserID: "u1"}, Level: trek.LevelDebug, ExpiresAt: time.Now().Add(time.Hour), Labels: map[string]string{}},
		{ID: "s2", Selector: trek.Selector{TenantID: "t1"}, Level: trek.LevelTrace, ExpiresAt: time.Now().Add(time.Hour), Labels: map[string]string{}},
		{ID: "s3", Selector: trek.Selector{Route: "/api/*"}, Level: trek.LevelDebug, ExpiresAt: time.Now().Add(time.Hour), Labels: map[string]string{}},
	}

	ctx := trek.RequestContext{
		UserID:   "u1",
		TenantID: "t1",
		Route:    "/api/test",
	}

	const goroutines = 100
	const iterations = 1000

	done := make(chan bool, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			for j := 0; j < iterations; j++ {
				decision := trek.Decide(time.Now(), "test", ctx, sessions)
				if !decision.Matched {
					t.Error("expected match")
					return
				}
			}
			done <- true
		}()
	}

	for i := 0; i < goroutines; i++ {
		<-done
	}

	t.Logf("Completed %d evaluations across %d goroutines", goroutines*iterations, goroutines)
}
