package trek

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// GlobalCaps provides global cap checking via the Trek server.
type GlobalCaps struct {
	client   *Client
	enabled  bool
	syncMode bool // If true, first event per session is checked synchronously

	// Cache to avoid hammering the server
	mu       sync.RWMutex
	blocked  map[string]time.Time // sessionID -> blocked until
	checked  map[string]bool      // sessionID -> already checked synchronously
	cooldown time.Duration
}

// NewGlobalCaps creates a new global caps checker.
func NewGlobalCaps(client *Client) *GlobalCaps {
	return &GlobalCaps{
		client:   client,
		enabled:  true,
		syncMode: true, // Default to sync mode for first check
		blocked:  make(map[string]time.Time),
		checked:  make(map[string]bool),
		cooldown: 5 * time.Second,
	}
}

// CheckAndIncrement checks global caps and increments counters.
// Returns true if the event should be allowed.
// On error, falls back to allowing (fail-open).
func (g *GlobalCaps) CheckAndIncrement(ctx context.Context, sessionID, requestID string, caps Caps) bool {
	if !g.enabled || g.client == nil {
		return true
	}

	// Check local blocked cache first
	if g.isBlocked(sessionID) {
		return false
	}

	// In sync mode, do synchronous check for first event per session
	if g.syncMode && !g.hasChecked(sessionID) {
		return g.checkSync(ctx, sessionID, requestID, caps)
	}

	// Make async check to avoid blocking log calls
	go g.checkAsync(sessionID, requestID, caps)

	return true
}

func (g *GlobalCaps) hasChecked(sessionID string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.checked[sessionID]
}

func (g *GlobalCaps) markChecked(sessionID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.checked[sessionID] = true
}

func (g *GlobalCaps) checkSync(ctx context.Context, sessionID, requestID string, caps Caps) bool {
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	result, err := g.client.IncrementCaps(checkCtx, sessionID, requestID,
		caps.MaxDebugEventsPerSession, caps.MaxDebugEventsPerRequest)

	g.markChecked(sessionID)

	if err != nil {
		slog.Debug("global caps sync check failed, allowing (fail-open)", "error", err)
		return true
	}

	if !result.Allowed {
		g.setBlocked(sessionID)
		slog.Warn("TREK_GLOBAL_CAP_REACHED",
			"session_id", sessionID,
			"session_count", result.SessionCount,
			"request_count", result.RequestCount,
		)
		return false
	}

	return true
}

func (g *GlobalCaps) checkAsync(sessionID, requestID string, caps Caps) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := g.client.IncrementCaps(ctx, sessionID, requestID,
		caps.MaxDebugEventsPerSession, caps.MaxDebugEventsPerRequest)

	if err != nil {
		slog.Debug("global caps check failed", "error", err)
		return
	}

	if !result.Allowed {
		g.setBlocked(sessionID)
		slog.Warn("TREK_GLOBAL_CAP_REACHED",
			"session_id", sessionID,
			"session_count", result.SessionCount,
			"request_count", result.RequestCount,
		)
	}
}

func (g *GlobalCaps) isBlocked(sessionID string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	blockedUntil, ok := g.blocked[sessionID]
	if !ok {
		return false
	}

	return time.Now().Before(blockedUntil)
}

func (g *GlobalCaps) setBlocked(sessionID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.blocked[sessionID] = time.Now().Add(g.cooldown)

	// Cleanup old entries
	now := time.Now()
	for k, v := range g.blocked {
		if now.After(v) {
			delete(g.blocked, k)
		}
	}
}

// Disable disables global caps checking.
func (g *GlobalCaps) Disable() {
	g.enabled = false
}

// Enable enables global caps checking.
func (g *GlobalCaps) Enable() {
	g.enabled = true
}

// SetSyncMode enables or disables synchronous checking for first event.
func (g *GlobalCaps) SetSyncMode(enabled bool) {
	g.syncMode = enabled
}
