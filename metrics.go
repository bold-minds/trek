package trek

import (
	"sync"
	"sync/atomic"
)

// Metrics tracks SDK-side metrics for observability.
type Metrics struct {
	// Counters
	RequestsTotal      atomic.Int64
	RequestsMatched    atomic.Int64
	CapsTriggered      atomic.Int64
	PollsTotal         atomic.Int64
	PollErrors         atomic.Int64
	DecisionLatencySum atomic.Int64
	DecisionCount      atomic.Int64

	// Gauges
	mu             sync.RWMutex
	activeSessions int
}

var globalMetrics = &Metrics{}

// GetMetrics returns the global metrics instance.
func GetMetrics() *Metrics {
	return globalMetrics
}

// IncrRequestsTotal increments total requests evaluated.
func (m *Metrics) IncrRequestsTotal() {
	m.RequestsTotal.Add(1)
}

// IncrRequestsMatched increments requests that matched a session.
func (m *Metrics) IncrRequestsMatched() {
	m.RequestsMatched.Add(1)
}

// IncrCapsTriggered increments cap triggers.
func (m *Metrics) IncrCapsTriggered() {
	m.CapsTriggered.Add(1)
}

// IncrPollsTotal increments total poll attempts.
func (m *Metrics) IncrPollsTotal() {
	m.PollsTotal.Add(1)
}

// IncrPollErrors increments poll errors.
func (m *Metrics) IncrPollErrors() {
	m.PollErrors.Add(1)
}

// RecordDecisionLatency records a decision latency in microseconds.
func (m *Metrics) RecordDecisionLatency(microseconds int64) {
	m.DecisionLatencySum.Add(microseconds)
	m.DecisionCount.Add(1)
}

// SetActiveSessions sets the current active session count.
func (m *Metrics) SetActiveSessions(count int) {
	m.mu.Lock()
	m.activeSessions = count
	m.mu.Unlock()
}

// GetActiveSessions returns the current active session count.
func (m *Metrics) GetActiveSessions() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeSessions
}

// Snapshot returns a copy of current metrics values.
func (m *Metrics) Snapshot() MetricsSnapshot {
	m.mu.RLock()
	activeSessions := m.activeSessions
	m.mu.RUnlock()

	decisionCount := m.DecisionCount.Load()
	var avgLatency float64
	if decisionCount > 0 {
		avgLatency = float64(m.DecisionLatencySum.Load()) / float64(decisionCount)
	}

	return MetricsSnapshot{
		RequestsTotal:          m.RequestsTotal.Load(),
		RequestsMatched:        m.RequestsMatched.Load(),
		CapsTriggered:          m.CapsTriggered.Load(),
		PollsTotal:             m.PollsTotal.Load(),
		PollErrors:             m.PollErrors.Load(),
		ActiveSessions:         activeSessions,
		AvgDecisionLatencyUsec: avgLatency,
	}
}

// MetricsSnapshot is a point-in-time snapshot of metrics.
type MetricsSnapshot struct {
	RequestsTotal          int64
	RequestsMatched        int64
	CapsTriggered          int64
	PollsTotal             int64
	PollErrors             int64
	ActiveSessions         int
	AvgDecisionLatencyUsec float64
}

// Reset resets all metrics to zero (useful for testing).
func (m *Metrics) Reset() {
	m.RequestsTotal.Store(0)
	m.RequestsMatched.Store(0)
	m.CapsTriggered.Store(0)
	m.PollsTotal.Store(0)
	m.PollErrors.Store(0)
	m.DecisionLatencySum.Store(0)
	m.DecisionCount.Store(0)
	m.mu.Lock()
	m.activeSessions = 0
	m.mu.Unlock()
}
