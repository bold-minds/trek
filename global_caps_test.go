package trek

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestGlobalCapsCheckAndIncrement(t *testing.T) {
	var callCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		json.NewEncoder(w).Encode(CapsCheckResponse{
			Allowed:      true,
			SessionCount: 1,
			RequestCount: 1,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", "org1", "prod")
	gc := NewGlobalCaps(client)
	gc.SetSyncMode(true) // Force sync mode for testing

	caps := Caps{
		MaxDebugEventsPerSession: 100,
		MaxDebugEventsPerRequest: 10,
	}

	// First call should be sync and allowed
	allowed := gc.CheckAndIncrement(context.Background(), "sess1", "req1", caps)
	if !allowed {
		t.Error("expected first call to be allowed")
	}

	// Verify server was called
	if callCount.Load() != 1 {
		t.Errorf("expected 1 server call, got %d", callCount.Load())
	}
}

func TestGlobalCapsDisabled(t *testing.T) {
	gc := NewGlobalCaps(nil)
	gc.Disable()

	allowed := gc.CheckAndIncrement(context.Background(), "sess1", "req1", Caps{})
	if !allowed {
		t.Error("expected disabled caps to allow all")
	}
}

func TestGlobalCapsNilClient(t *testing.T) {
	gc := NewGlobalCaps(nil)

	allowed := gc.CheckAndIncrement(context.Background(), "sess1", "req1", Caps{})
	if !allowed {
		t.Error("expected nil client to allow all (fail-open)")
	}
}

func TestGlobalCapsBlocking(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(CapsCheckResponse{
			Allowed:           false,
			SessionCapReached: true,
			SessionCount:      101,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", "org1", "prod")
	gc := NewGlobalCaps(client)
	gc.SetSyncMode(true)

	caps := Caps{MaxDebugEventsPerSession: 100}

	// First call triggers blocking
	allowed := gc.CheckAndIncrement(context.Background(), "sess1", "req1", caps)
	if allowed {
		t.Error("expected first call to be blocked after cap reached")
	}

	// Subsequent calls should be blocked from cache
	allowed = gc.CheckAndIncrement(context.Background(), "sess1", "req2", caps)
	if allowed {
		t.Error("expected cached blocked to remain blocked")
	}
}

func TestGlobalCapsEnableDisable(t *testing.T) {
	gc := NewGlobalCaps(nil)

	if !gc.enabled {
		t.Error("expected default enabled=true")
	}

	gc.Disable()
	if gc.enabled {
		t.Error("expected enabled=false after Disable()")
	}

	gc.Enable()
	if !gc.enabled {
		t.Error("expected enabled=true after Enable()")
	}
}

func TestGlobalCapsSyncModeToggle(t *testing.T) {
	gc := NewGlobalCaps(nil)

	if !gc.syncMode {
		t.Error("expected default syncMode=true")
	}

	gc.SetSyncMode(false)
	if gc.syncMode {
		t.Error("expected syncMode=false after SetSyncMode(false)")
	}

	gc.SetSyncMode(true)
	if !gc.syncMode {
		t.Error("expected syncMode=true after SetSyncMode(true)")
	}
}

func TestGlobalCapsAsyncAfterFirstCheck(t *testing.T) {
	var callCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		json.NewEncoder(w).Encode(CapsCheckResponse{Allowed: true})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", "org1", "prod")
	gc := NewGlobalCaps(client)
	gc.SetSyncMode(true)

	caps := Caps{MaxDebugEventsPerSession: 100}

	// First call is sync
	gc.CheckAndIncrement(context.Background(), "sess1", "req1", caps)
	firstCount := callCount.Load()

	// Second call should be async (session already checked)
	gc.CheckAndIncrement(context.Background(), "sess1", "req2", caps)

	// Give async call time to complete
	time.Sleep(50 * time.Millisecond)

	if callCount.Load() <= firstCount {
		t.Error("expected async call to eventually fire")
	}
}
