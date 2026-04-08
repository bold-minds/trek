package trek

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Cache manages local caching of active sessions with background polling.
type Cache struct {
	client       *Client
	serviceName  string
	pollInterval time.Duration

	mu             sync.RWMutex
	sessions       []Session
	revision       string
	clockOffset    atomic.Int64
	lastServerTime atomic.Int64

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewCache creates a new session cache with background polling.
func NewCache(client *Client, serviceName string, pollInterval time.Duration) *Cache {
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}

	return &Cache{
		client:       client,
		serviceName:  serviceName,
		pollInterval: pollInterval,
		sessions:     []Session{},
		stopCh:       make(chan struct{}),
	}
}

// Start begins background polling for session updates.
func (c *Cache) Start(ctx context.Context) {
	c.wg.Add(1)
	go c.pollLoop(ctx)
}

// Stop halts background polling and waits for cleanup.
func (c *Cache) Stop() {
	close(c.stopCh)
	c.wg.Wait()
}

// GetSessions returns the current cached sessions.
func (c *Cache) GetSessions() []Session {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]Session, len(c.sessions))
	copy(result, c.sessions)
	return result
}

// ClockOffset returns the estimated clock offset between local and server time.
func (c *Cache) ClockOffset() time.Duration {
	return time.Duration(c.clockOffset.Load())
}

// Refresh forces an immediate refresh of the session cache.
// This is primarily for testing; production code should rely on polling.
func (c *Cache) Refresh(ctx context.Context) error {
	return c.fetchSessions(ctx)
}

func (c *Cache) pollLoop(ctx context.Context) {
	defer c.wg.Done()

	if err := c.fetchSessions(ctx); err != nil {
		slog.Warn("trek: initial session fetch failed", "error", err)
	}

	jitter := cryptoRandDuration(c.pollInterval / 4)
	ticker := time.NewTicker(c.pollInterval + jitter)
	defer ticker.Stop()

	backoff := c.pollInterval
	maxBackoff := 5 * time.Minute

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			if err := c.fetchSessions(ctx); err != nil {
				slog.Warn("trek: session fetch failed", "error", err, "backoff", backoff)

				backoff = min(backoff*2, maxBackoff)
				ticker.Reset(backoff)
			} else {
				backoff = c.pollInterval
				jitter = cryptoRandDuration(c.pollInterval / 4)
				ticker.Reset(c.pollInterval + jitter)
			}
		}
	}
}

func (c *Cache) fetchSessions(ctx context.Context) error {
	c.mu.RLock()
	currentRevision := c.revision
	c.mu.RUnlock()

	resp, err := c.client.GetActiveSessions(ctx, c.serviceName, currentRevision)
	if err != nil {
		return err
	}

	if resp == nil {
		return nil
	}

	if resp.Revision == currentRevision {
		c.updateClockOffset(resp.ServerTime)
		return nil
	}

	c.mu.Lock()
	c.sessions = resp.Sessions
	c.revision = resp.Revision
	c.mu.Unlock()

	c.updateClockOffset(resp.ServerTime)

	slog.Debug("trek: sessions updated",
		"count", len(resp.Sessions),
		"revision", resp.Revision,
	)

	return nil
}

func (c *Cache) updateClockOffset(serverTime time.Time) {
	if serverTime.IsZero() {
		return
	}

	localNow := time.Now()
	offset := serverTime.Sub(localNow)

	currentOffset := time.Duration(c.clockOffset.Load())
	smoothedOffset := (currentOffset*3 + offset) / 4

	c.clockOffset.Store(int64(smoothedOffset))
	c.lastServerTime.Store(localNow.UnixNano())
}

// ClockSkewGracePeriod is the grace period when server time is unavailable.
const ClockSkewGracePeriod = 30 * time.Second

// AdjustedNow returns the current time adjusted for clock skew.
// If server time hasn't been received recently (>30s), it applies a grace period
// by subtracting 30s from the local time for session expiration checks.
func (c *Cache) AdjustedNow() time.Time {
	localNow := time.Now()
	offset := time.Duration(c.clockOffset.Load())

	// Check if we have recent server time (within last 5 minutes)
	lastSync := c.lastServerTime.Load()
	if lastSync == 0 || time.Since(time.Unix(0, lastSync)) > 5*time.Minute {
		// No recent server time, apply grace period (subtract from local time
		// so we consider sessions expired 30s later than they actually are)
		return localNow.Add(-ClockSkewGracePeriod)
	}

	return localNow.Add(offset)
}

// SessionCount returns the number of cached sessions.
func (c *Cache) SessionCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.sessions)
}

// cryptoRandDuration returns a random duration between 0 and max using crypto/rand.
func cryptoRandDuration(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return max / 2 // fallback to middle value on error
	}
	n := binary.LittleEndian.Uint64(b[:])
	return time.Duration(n % uint64(max))
}

// Revision returns the current cache revision.
func (c *Cache) Revision() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.revision
}
