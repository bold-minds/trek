package caps

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// Counter tracks debug event counts globally using Redis/ValKey.
type Counter struct {
	client *redis.Client
	prefix string
}

// NewCounter creates a new global caps counter.
func NewCounter(redisURL string) (*Counter, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	return &Counter{
		client: client,
		prefix: "trek:caps:",
	}, nil
}

// Close closes the Redis connection.
func (c *Counter) Close() error {
	return c.client.Close()
}

// IncrementSession increments the session-level debug event counter.
// Returns the new count and whether the cap has been exceeded.
func (c *Counter) IncrementSession(ctx context.Context, sessionID string, maxEvents int) (int64, bool, error) {
	key := c.prefix + "session:" + sessionID

	count, err := c.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, false, fmt.Errorf("incr session counter: %w", err)
	}

	// Set expiry on first increment (24h - sessions shouldn't live longer)
	if count == 1 {
		if err := c.client.Expire(ctx, key, 24*time.Hour).Err(); err != nil {
			slog.Warn("failed to set session key expiry", "key", key, "error", err)
		}
	}

	exceeded := maxEvents > 0 && count > int64(maxEvents)
	return count, exceeded, nil
}

// IncrementRequest increments the request-level debug event counter.
// Returns the new count and whether the cap has been exceeded.
func (c *Counter) IncrementRequest(ctx context.Context, requestID string, maxEvents int) (int64, bool, error) {
	key := c.prefix + "request:" + requestID

	count, err := c.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, false, fmt.Errorf("incr request counter: %w", err)
	}

	// Request counters expire after 5 minutes
	if count == 1 {
		if err := c.client.Expire(ctx, key, 5*time.Minute).Err(); err != nil {
			slog.Warn("failed to set request key expiry", "key", key, "error", err)
		}
	}

	exceeded := maxEvents > 0 && count > int64(maxEvents)
	return count, exceeded, nil
}

// GetSessionCount returns the current session event count.
func (c *Counter) GetSessionCount(ctx context.Context, sessionID string) (int64, error) {
	key := c.prefix + "session:" + sessionID
	count, err := c.client.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return count, err
}

// GetRequestCount returns the current request event count.
func (c *Counter) GetRequestCount(ctx context.Context, requestID string) (int64, error) {
	key := c.prefix + "request:" + requestID
	count, err := c.client.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return count, err
}

// CheckAndIncrement atomically checks caps and increments counters.
// Returns whether the event should be allowed.
type CheckResult struct {
	Allowed           bool  `json:"allowed"`
	SessionCount      int64 `json:"session_count"`
	RequestCount      int64 `json:"request_count"`
	SessionCapReached bool  `json:"session_cap_reached"`
	RequestCapReached bool  `json:"request_cap_reached"`
}

func (c *Counter) CheckAndIncrement(ctx context.Context, sessionID, requestID string, maxPerSession, maxPerRequest int) (*CheckResult, error) {
	result := &CheckResult{Allowed: true}

	// Check session cap
	if sessionID != "" {
		count, exceeded, err := c.IncrementSession(ctx, sessionID, maxPerSession)
		if err != nil {
			return nil, err
		}
		result.SessionCount = count
		result.SessionCapReached = exceeded
		if exceeded {
			result.Allowed = false
		}
	}

	// Check request cap
	if requestID != "" {
		count, exceeded, err := c.IncrementRequest(ctx, requestID, maxPerRequest)
		if err != nil {
			return nil, err
		}
		result.RequestCount = count
		result.RequestCapReached = exceeded
		if exceeded {
			result.Allowed = false
		}
	}

	return result, nil
}

// ResetSession resets the session counter (for testing or manual override).
func (c *Counter) ResetSession(ctx context.Context, sessionID string) error {
	key := c.prefix + "session:" + sessionID
	return c.client.Del(ctx, key).Err()
}

// ResetRequest resets the request counter.
func (c *Counter) ResetRequest(ctx context.Context, requestID string) error {
	key := c.prefix + "request:" + requestID
	return c.client.Del(ctx, key).Err()
}
