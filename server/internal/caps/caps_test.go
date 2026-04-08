package caps

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func setupTestRedis(t *testing.T) (*Counter, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	counter, err := NewCounter("redis://" + mr.Addr())
	if err != nil {
		mr.Close()
		t.Fatalf("failed to create counter: %v", err)
	}

	return counter, mr
}

func TestIncrementSession(t *testing.T) {
	counter, mr := setupTestRedis(t)
	defer mr.Close()
	defer counter.Close()

	ctx := context.Background()
	sessionID := "test-session-1"

	// First increment
	count, exceeded, err := counter.IncrementSession(ctx, sessionID, 10)
	if err != nil {
		t.Fatalf("increment failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}
	if exceeded {
		t.Error("should not exceed cap on first increment")
	}

	// Increment to exceed cap
	for i := 0; i < 10; i++ {
		count, exceeded, err = counter.IncrementSession(ctx, sessionID, 10)
		if err != nil {
			t.Fatalf("increment failed: %v", err)
		}
	}

	if !exceeded {
		t.Error("should exceed cap after 11 increments with max 10")
	}
	if count != 11 {
		t.Errorf("expected count 11, got %d", count)
	}
}

func TestIncrementRequest(t *testing.T) {
	counter, mr := setupTestRedis(t)
	defer mr.Close()
	defer counter.Close()

	ctx := context.Background()
	requestID := "test-request-1"

	count, exceeded, err := counter.IncrementRequest(ctx, requestID, 5)
	if err != nil {
		t.Fatalf("increment failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}
	if exceeded {
		t.Error("should not exceed cap on first increment")
	}
}

func TestGetSessionCount(t *testing.T) {
	counter, mr := setupTestRedis(t)
	defer mr.Close()
	defer counter.Close()

	ctx := context.Background()
	sessionID := "test-session-get"

	// Before any increments
	count, err := counter.GetSessionCount(ctx, sessionID)
	if err != nil {
		t.Fatalf("get count failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}

	// After increment
	counter.IncrementSession(ctx, sessionID, 100)
	count, err = counter.GetSessionCount(ctx, sessionID)
	if err != nil {
		t.Fatalf("get count failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}
}

func TestResetSession(t *testing.T) {
	counter, mr := setupTestRedis(t)
	defer mr.Close()
	defer counter.Close()

	ctx := context.Background()
	sessionID := "test-session-reset"

	counter.IncrementSession(ctx, sessionID, 100)

	err := counter.ResetSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("reset failed: %v", err)
	}

	count, err := counter.GetSessionCount(ctx, sessionID)
	if err != nil {
		t.Fatalf("get count failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 after reset, got %d", count)
	}
}

func TestCheckAndIncrement(t *testing.T) {
	counter, mr := setupTestRedis(t)
	defer mr.Close()
	defer counter.Close()

	ctx := context.Background()

	result, err := counter.CheckAndIncrement(ctx, "sess1", "req1", 10, 5)
	if err != nil {
		t.Fatalf("check and increment failed: %v", err)
	}

	if !result.Allowed {
		t.Error("should be allowed on first increment")
	}
	if result.SessionCount != 1 {
		t.Errorf("expected session count 1, got %d", result.SessionCount)
	}
	if result.RequestCount != 1 {
		t.Errorf("expected request count 1, got %d", result.RequestCount)
	}
}

func TestCheckAndIncrementCapReached(t *testing.T) {
	counter, mr := setupTestRedis(t)
	defer mr.Close()
	defer counter.Close()

	ctx := context.Background()

	// Exhaust session cap
	for i := 0; i < 5; i++ {
		counter.CheckAndIncrement(ctx, "sess2", "req2", 5, 100)
	}

	result, err := counter.CheckAndIncrement(ctx, "sess2", "req3", 5, 100)
	if err != nil {
		t.Fatalf("check and increment failed: %v", err)
	}

	if result.Allowed {
		t.Error("should not be allowed after cap reached")
	}
	if !result.SessionCapReached {
		t.Error("session cap should be reached")
	}
}

func TestZeroMaxAllowsUnlimited(t *testing.T) {
	counter, mr := setupTestRedis(t)
	defer mr.Close()
	defer counter.Close()

	ctx := context.Background()

	// With max=0, should never exceed
	for i := 0; i < 100; i++ {
		_, exceeded, err := counter.IncrementSession(ctx, "unlimited", 0)
		if err != nil {
			t.Fatalf("increment failed: %v", err)
		}
		if exceeded {
			t.Error("should not exceed with max=0")
		}
	}
}

func TestKeyExpiry(t *testing.T) {
	counter, mr := setupTestRedis(t)
	defer mr.Close()
	defer counter.Close()

	ctx := context.Background()

	counter.IncrementSession(ctx, "expiry-test", 100)

	// Check that TTL is set
	ttl := mr.TTL("trek:caps:session:expiry-test")
	if ttl <= 0 {
		t.Error("expected TTL to be set on session key")
	}
	if ttl > 24*time.Hour {
		t.Errorf("TTL should be <= 24h, got %v", ttl)
	}
}
