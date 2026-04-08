package trek

import (
	"testing"
)

func TestMetricsSnapshot(t *testing.T) {
	m := &Metrics{}
	m.Reset()

	// Verify initial state
	snap := m.Snapshot()
	if snap.RequestsTotal != 0 {
		t.Errorf("expected RequestsTotal=0, got %d", snap.RequestsTotal)
	}
	if snap.RequestsMatched != 0 {
		t.Errorf("expected RequestsMatched=0, got %d", snap.RequestsMatched)
	}

	// Increment counters
	m.IncrRequestsTotal()
	m.IncrRequestsTotal()
	m.IncrRequestsMatched()
	m.IncrCapsTriggered()
	m.IncrPollsTotal()
	m.IncrPollErrors()

	snap = m.Snapshot()
	if snap.RequestsTotal != 2 {
		t.Errorf("expected RequestsTotal=2, got %d", snap.RequestsTotal)
	}
	if snap.RequestsMatched != 1 {
		t.Errorf("expected RequestsMatched=1, got %d", snap.RequestsMatched)
	}
	if snap.CapsTriggered != 1 {
		t.Errorf("expected CapsTriggered=1, got %d", snap.CapsTriggered)
	}
	if snap.PollsTotal != 1 {
		t.Errorf("expected PollsTotal=1, got %d", snap.PollsTotal)
	}
	if snap.PollErrors != 1 {
		t.Errorf("expected PollErrors=1, got %d", snap.PollErrors)
	}
}

func TestMetricsActiveSessions(t *testing.T) {
	m := &Metrics{}
	m.Reset()

	if m.GetActiveSessions() != 0 {
		t.Errorf("expected ActiveSessions=0, got %d", m.GetActiveSessions())
	}

	m.SetActiveSessions(5)
	if m.GetActiveSessions() != 5 {
		t.Errorf("expected ActiveSessions=5, got %d", m.GetActiveSessions())
	}

	snap := m.Snapshot()
	if snap.ActiveSessions != 5 {
		t.Errorf("expected snapshot ActiveSessions=5, got %d", snap.ActiveSessions)
	}
}

func TestMetricsDecisionLatency(t *testing.T) {
	m := &Metrics{}
	m.Reset()

	m.RecordDecisionLatency(100)
	m.RecordDecisionLatency(200)
	m.RecordDecisionLatency(300)

	snap := m.Snapshot()
	// Average should be (100+200+300)/3 = 200
	if snap.AvgDecisionLatencyUsec != 200.0 {
		t.Errorf("expected avg latency=200, got %f", snap.AvgDecisionLatencyUsec)
	}
}

func TestMetricsReset(t *testing.T) {
	m := &Metrics{}

	m.IncrRequestsTotal()
	m.IncrRequestsMatched()
	m.IncrCapsTriggered()
	m.SetActiveSessions(10)
	m.RecordDecisionLatency(500)

	m.Reset()

	snap := m.Snapshot()
	if snap.RequestsTotal != 0 {
		t.Errorf("expected RequestsTotal=0 after reset, got %d", snap.RequestsTotal)
	}
	if snap.RequestsMatched != 0 {
		t.Errorf("expected RequestsMatched=0 after reset, got %d", snap.RequestsMatched)
	}
	if snap.CapsTriggered != 0 {
		t.Errorf("expected CapsTriggered=0 after reset, got %d", snap.CapsTriggered)
	}
	if snap.ActiveSessions != 0 {
		t.Errorf("expected ActiveSessions=0 after reset, got %d", snap.ActiveSessions)
	}
	if snap.AvgDecisionLatencyUsec != 0 {
		t.Errorf("expected AvgDecisionLatencyUsec=0 after reset, got %f", snap.AvgDecisionLatencyUsec)
	}
}

func TestGlobalMetrics(t *testing.T) {
	// Verify GetMetrics returns the global instance
	m1 := GetMetrics()
	m2 := GetMetrics()

	if m1 != m2 {
		t.Error("GetMetrics should return same instance")
	}

	// Reset for clean state
	m1.Reset()
}
