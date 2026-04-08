package trek

import (
	"testing"
	"time"
)

func TestRequestContext(t *testing.T) {
	ctx := RequestContext{
		UserID:    "u123",
		RequestID: "req-456",
		TenantID:  "t789",
		Route:     "/api/orders",
		Custom: map[string]string{
			"region": "us-east",
		},
	}

	if ctx.UserID != "u123" {
		t.Errorf("UserID = %q, want %q", ctx.UserID, "u123")
	}
	if ctx.RequestID != "req-456" {
		t.Errorf("RequestID = %q, want %q", ctx.RequestID, "req-456")
	}
	if ctx.TenantID != "t789" {
		t.Errorf("TenantID = %q, want %q", ctx.TenantID, "t789")
	}
	if ctx.Route != "/api/orders" {
		t.Errorf("Route = %q, want %q", ctx.Route, "/api/orders")
	}
	if ctx.Custom["region"] != "us-east" {
		t.Errorf("Custom[region] = %q, want %q", ctx.Custom["region"], "us-east")
	}
}

func TestSelector(t *testing.T) {
	selector := Selector{
		UserID:    "u123",
		RequestID: "req-456",
		TenantID:  "t789",
		Route:     "/api/*",
		Custom: map[string]string{
			"env": "prod",
		},
	}

	if selector.UserID != "u123" {
		t.Errorf("UserID = %q, want %q", selector.UserID, "u123")
	}
	if selector.TenantID != "t789" {
		t.Errorf("TenantID = %q, want %q", selector.TenantID, "t789")
	}
}

func TestCaps(t *testing.T) {
	caps := Caps{
		MaxDebugEventsPerRequest: 100,
		MaxDebugEventsPerSession: 5000,
	}

	if caps.MaxDebugEventsPerRequest != 100 {
		t.Errorf("MaxDebugEventsPerRequest = %d, want %d", caps.MaxDebugEventsPerRequest, 100)
	}
	if caps.MaxDebugEventsPerSession != 5000 {
		t.Errorf("MaxDebugEventsPerSession = %d, want %d", caps.MaxDebugEventsPerSession, 5000)
	}
}

func TestSession(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour)
	session := Session{
		ID: "sess-123",
		Selector: Selector{
			UserID: "u123",
		},
		Level:     LevelDebug,
		ExpiresAt: expiresAt,
		Caps: Caps{
			MaxDebugEventsPerRequest: 200,
		},
		Labels: map[string]string{
			"ticket": "TREK-456",
		},
		ServiceScope: []string{"api-gateway"},
	}

	if session.ID != "sess-123" {
		t.Errorf("ID = %q, want %q", session.ID, "sess-123")
	}
	if session.Level != LevelDebug {
		t.Errorf("Level = %q, want %q", session.Level, LevelDebug)
	}
	if len(session.ServiceScope) != 1 {
		t.Errorf("ServiceScope length = %d, want %d", len(session.ServiceScope), 1)
	}
}

func TestDecision(t *testing.T) {
	decision := Decision{
		Matched:        true,
		SessionID:      "sess-123",
		EffectiveLevel: LevelDebug,
		ReasonCode:     ReasonMatched,
		Labels: map[string]string{
			"ticket": "TREK-456",
		},
		Caps: Caps{
			MaxDebugEventsPerRequest: 100,
		},
	}

	if !decision.Matched {
		t.Error("Matched should be true")
	}
	if decision.SessionID != "sess-123" {
		t.Errorf("SessionID = %q, want %q", decision.SessionID, "sess-123")
	}
	if decision.ReasonCode != ReasonMatched {
		t.Errorf("ReasonCode = %q, want %q", decision.ReasonCode, ReasonMatched)
	}
}

func TestLevel_Constants(t *testing.T) {
	if LevelInfo != "info" {
		t.Errorf("LevelInfo = %q, want %q", LevelInfo, "info")
	}
	if LevelDebug != "debug" {
		t.Errorf("LevelDebug = %q, want %q", LevelDebug, "debug")
	}
	if LevelTrace != "trace" {
		t.Errorf("LevelTrace = %q, want %q", LevelTrace, "trace")
	}
}

func TestLevel_Priority(t *testing.T) {
	tests := []struct {
		level    Level
		expected int
	}{
		{LevelTrace, 3},
		{LevelDebug, 2},
		{LevelInfo, 1},
		{Level("unknown"), 0},
		{Level(""), 0},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			got := tt.level.Priority()
			if got != tt.expected {
				t.Errorf("Priority() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestLevel_PriorityOrdering(t *testing.T) {
	if LevelTrace.Priority() <= LevelDebug.Priority() {
		t.Error("Trace priority should be higher than Debug")
	}
	if LevelDebug.Priority() <= LevelInfo.Priority() {
		t.Error("Debug priority should be higher than Info")
	}
}

func TestReasonCode_Constants(t *testing.T) {
	if ReasonMatched != "MATCHED" {
		t.Errorf("ReasonMatched = %q, want %q", ReasonMatched, "MATCHED")
	}
	if ReasonNoMatch != "NO_MATCH" {
		t.Errorf("ReasonNoMatch = %q, want %q", ReasonNoMatch, "NO_MATCH")
	}
	if ReasonExpired != "EXPIRED" {
		t.Errorf("ReasonExpired = %q, want %q", ReasonExpired, "EXPIRED")
	}
}

func TestNoMatchDecision(t *testing.T) {
	decision := NoMatchDecision(ReasonNoMatch)

	if decision.Matched {
		t.Error("Matched should be false")
	}
	if decision.SessionID != "" {
		t.Errorf("SessionID = %q, want empty", decision.SessionID)
	}
	if decision.EffectiveLevel != LevelInfo {
		t.Errorf("EffectiveLevel = %q, want %q", decision.EffectiveLevel, LevelInfo)
	}
	if decision.ReasonCode != ReasonNoMatch {
		t.Errorf("ReasonCode = %q, want %q", decision.ReasonCode, ReasonNoMatch)
	}
	if decision.Labels == nil {
		t.Error("Labels should not be nil")
	}
}

func TestNoMatchDecision_EmptyReason(t *testing.T) {
	decision := NoMatchDecision("")

	if decision.ReasonCode != ReasonNoMatch {
		t.Errorf("ReasonCode = %q, want %q for empty input", decision.ReasonCode, ReasonNoMatch)
	}
}

func TestNoMatchDecision_CustomReason(t *testing.T) {
	decision := NoMatchDecision(ReasonExpired)

	if decision.ReasonCode != ReasonExpired {
		t.Errorf("ReasonCode = %q, want %q", decision.ReasonCode, ReasonExpired)
	}
}

func TestActiveSessionsResponse(t *testing.T) {
	resp := ActiveSessionsResponse{
		Revision:   "rev-123",
		ServerTime: time.Now(),
		Sessions: []Session{
			{ID: "sess-1"},
			{ID: "sess-2"},
		},
	}

	if resp.Revision != "rev-123" {
		t.Errorf("Revision = %q, want %q", resp.Revision, "rev-123")
	}
	if len(resp.Sessions) != 2 {
		t.Errorf("Sessions length = %d, want %d", len(resp.Sessions), 2)
	}
}

func TestCreateSessionRequest(t *testing.T) {
	ttl := 3600
	req := CreateSessionRequest{
		Selector: Selector{
			UserID: "u123",
		},
		Level:      LevelDebug,
		TTLSeconds: ttl,
		Reason:     "debugging issue",
		Labels: map[string]string{
			"ticket": "TREK-123",
		},
	}

	if req.Selector.UserID != "u123" {
		t.Errorf("Selector.UserID = %q, want %q", req.Selector.UserID, "u123")
	}
	if req.Level != LevelDebug {
		t.Errorf("Level = %q, want %q", req.Level, LevelDebug)
	}
	if req.TTLSeconds != 3600 {
		t.Errorf("TTLSeconds = %d, want %d", req.TTLSeconds, 3600)
	}
	if req.Reason != "debugging issue" {
		t.Errorf("Reason = %q, want %q", req.Reason, "debugging issue")
	}
}

func TestCreateSessionResponse(t *testing.T) {
	resp := CreateSessionResponse{
		ID:        "sess-123",
		Status:    "active",
		ExpiresAt: time.Now().Add(time.Hour),
	}

	if resp.ID != "sess-123" {
		t.Errorf("ID = %q, want %q", resp.ID, "sess-123")
	}
	if resp.Status != "active" {
		t.Errorf("Status = %q, want %q", resp.Status, "active")
	}
}

func TestSessionStatus_Constants(t *testing.T) {
	if SessionStatusActive != "active" {
		t.Errorf("SessionStatusActive = %q, want %q", SessionStatusActive, "active")
	}
	if SessionStatusRevoked != "revoked" {
		t.Errorf("SessionStatusRevoked = %q, want %q", SessionStatusRevoked, "revoked")
	}
	if SessionStatusExpired != "expired" {
		t.Errorf("SessionStatusExpired = %q, want %q", SessionStatusExpired, "expired")
	}
}
