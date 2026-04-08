package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/bold-minds/trek/server/internal/notify"
	"github.com/bold-minds/trek/server/internal/store"
)

// ExpiryScheduler runs periodic session expiry sweeps.
type ExpiryScheduler struct {
	store    store.Store
	notifier *notify.Notifier
	interval time.Duration
	stopCh   chan struct{}
}

// NewExpiryScheduler creates a new expiry scheduler.
func NewExpiryScheduler(s store.Store, n *notify.Notifier, interval time.Duration) *ExpiryScheduler {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &ExpiryScheduler{
		store:    s,
		notifier: n,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the expiry sweep loop.
func (s *ExpiryScheduler) Start(ctx context.Context) {
	slog.Info("expiry scheduler started", "interval", s.interval)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("expiry scheduler stopped (context)")
			return
		case <-s.stopCh:
			slog.Info("expiry scheduler stopped")
			return
		case <-ticker.C:
			s.runSweep(ctx)
		}
	}
}

// Stop stops the scheduler.
func (s *ExpiryScheduler) Stop() {
	close(s.stopCh)
}

func (s *ExpiryScheduler) runSweep(ctx context.Context) {
	start := time.Now()

	count, err := s.store.ExpireSessions(ctx)
	if err != nil {
		slog.Error("expiry sweep failed", "error", err)
		return
	}

	duration := time.Since(start)

	if count > 0 {
		slog.Info("expiry sweep completed",
			"expired_count", count,
			"duration_ms", duration.Milliseconds(),
		)

		if s.notifier != nil {
			s.notifier.NotifySessionsExpired(ctx, count)
		}
	} else {
		slog.Debug("expiry sweep completed", "expired_count", 0, "duration_ms", duration.Milliseconds())
	}
}

// RunOnce runs a single expiry sweep (for testing).
func (s *ExpiryScheduler) RunOnce(ctx context.Context) (int, error) {
	return s.store.ExpireSessions(ctx)
}
