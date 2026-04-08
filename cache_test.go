package trek

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestNewCache(t *testing.T) {
	client := &Client{}

	t.Run("uses default poll interval if zero", func(t *testing.T) {
		cache := NewCache(client, "test-service", 0)
		if cache.pollInterval != 5*time.Second {
			t.Errorf("pollInterval = %v, want %v", cache.pollInterval, 5*time.Second)
		}
	})

	t.Run("uses default poll interval if negative", func(t *testing.T) {
		cache := NewCache(client, "test-service", -1*time.Second)
		if cache.pollInterval != 5*time.Second {
			t.Errorf("pollInterval = %v, want %v", cache.pollInterval, 5*time.Second)
		}
	})

	t.Run("uses provided poll interval", func(t *testing.T) {
		cache := NewCache(client, "test-service", 30*time.Second)
		if cache.pollInterval != 30*time.Second {
			t.Errorf("pollInterval = %v, want %v", cache.pollInterval, 30*time.Second)
		}
	})

	t.Run("initializes with empty sessions", func(t *testing.T) {
		cache := NewCache(client, "test-service", 5*time.Second)
		sessions := cache.GetSessions()
		if len(sessions) != 0 {
			t.Errorf("sessions length = %d, want 0", len(sessions))
		}
	})

	t.Run("sets service name", func(t *testing.T) {
		cache := NewCache(client, "my-service", 5*time.Second)
		if cache.serviceName != "my-service" {
			t.Errorf("serviceName = %q, want %q", cache.serviceName, "my-service")
		}
	})
}

func TestCache_GetSessions(t *testing.T) {
	client := &Client{}
	cache := NewCache(client, "test-service", 5*time.Second)

	t.Run("returns empty slice initially", func(t *testing.T) {
		sessions := cache.GetSessions()
		if sessions == nil {
			t.Error("GetSessions() returned nil, want empty slice")
		}
		if len(sessions) != 0 {
			t.Errorf("len(GetSessions()) = %d, want 0", len(sessions))
		}
	})

	t.Run("returns copy of sessions", func(t *testing.T) {
		testSessions := []Session{
			{ID: "sess-1", Level: LevelDebug},
			{ID: "sess-2", Level: LevelInfo},
		}
		cache.mu.Lock()
		cache.sessions = testSessions
		cache.mu.Unlock()

		sessions := cache.GetSessions()
		if len(sessions) != 2 {
			t.Errorf("len(GetSessions()) = %d, want 2", len(sessions))
		}

		sessions[0].ID = "modified"
		originalSessions := cache.GetSessions()
		if originalSessions[0].ID == "modified" {
			t.Error("GetSessions() returned reference to internal slice, not a copy")
		}
	})
}

func TestCache_ClockOffset(t *testing.T) {
	client := &Client{}
	cache := NewCache(client, "test-service", 5*time.Second)

	t.Run("returns zero initially", func(t *testing.T) {
		offset := cache.ClockOffset()
		if offset != 0 {
			t.Errorf("ClockOffset() = %v, want 0", offset)
		}
	})

	t.Run("returns stored offset", func(t *testing.T) {
		cache.clockOffset.Store(int64(5 * time.Second))
		offset := cache.ClockOffset()
		if offset != 5*time.Second {
			t.Errorf("ClockOffset() = %v, want %v", offset, 5*time.Second)
		}
	})
}

func TestCache_InternalUpdate(t *testing.T) {
	client := &Client{}
	cache := NewCache(client, "test-service", 5*time.Second)

	t.Run("direct session update via internal fields", func(t *testing.T) {
		newSessions := []Session{
			{ID: "sess-1", Level: LevelDebug},
		}

		cache.mu.Lock()
		cache.sessions = newSessions
		cache.revision = "rev-123"
		cache.mu.Unlock()

		sessions := cache.GetSessions()
		if len(sessions) != 1 {
			t.Errorf("len(GetSessions()) = %d, want 1", len(sessions))
		}

		if cache.Revision() != "rev-123" {
			t.Errorf("Revision() = %q, want %q", cache.Revision(), "rev-123")
		}
	})

	t.Run("updateClockOffset calculates offset from server time", func(t *testing.T) {
		// Reset offset to zero for clean test
		cache.clockOffset.Store(0)

		// Apply multiple updates to stabilize the smoothing
		for i := 0; i < 4; i++ {
			serverTime := time.Now().Add(2 * time.Second)
			cache.updateClockOffset(serverTime)
		}

		offset := cache.ClockOffset()
		// After smoothing, should be close to 2s
		if offset < 1*time.Second || offset > 3*time.Second {
			t.Errorf("ClockOffset() = %v, expected ~2s (with smoothing)", offset)
		}
	})

	t.Run("updateClockOffset ignores zero time", func(t *testing.T) {
		cache.clockOffset.Store(int64(1 * time.Second))
		cache.updateClockOffset(time.Time{})

		offset := cache.ClockOffset()
		if offset != 1*time.Second {
			t.Errorf("ClockOffset() = %v, want %v (should not change)", offset, 1*time.Second)
		}
	})
}

func TestCache_ConcurrentAccess(t *testing.T) {
	client := &Client{}
	cache := NewCache(client, "test-service", 5*time.Second)

	var wg sync.WaitGroup
	iterations := 100

	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			cache.mu.Lock()
			cache.sessions = []Session{{ID: "sess-1"}}
			cache.revision = "rev"
			cache.mu.Unlock()
			cache.updateClockOffset(time.Now())
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = cache.GetSessions()
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = cache.ClockOffset()
		}
	}()

	wg.Wait()
}

func TestCache_StartStop(t *testing.T) {
	// Create a mock server that returns empty sessions
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"revision":"rev-1","sessions":[],"server_time":"` + time.Now().Format(time.RFC3339Nano) + `"}`))
	}))
	defer server.Close()

	client := NewClientWithHTTP(server.URL, "test-token", "test-org", "test-env", server.Client())
	cache := NewCache(client, "test-service", 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())

	cache.Start(ctx)

	// Give it time to make at least one poll
	time.Sleep(20 * time.Millisecond)

	cancel()

	cache.Stop()
}

func TestCache_StopWithoutStart(t *testing.T) {
	client := &Client{}
	cache := NewCache(client, "test-service", 5*time.Second)

	// Calling Stop on a cache that was never started should not panic
	// This verifies the stopCh is properly initialized
	cache.Stop()
}
