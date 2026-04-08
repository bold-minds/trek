package trek

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

const fixturesURL = "https://raw.githubusercontent.com/bold-minds/trek-spec/main/fixtures/v1.json"

type Fixture struct {
	Name           string           `json:"name"`
	Now            time.Time        `json:"now"`
	ServiceName    string           `json:"service_name"`
	RequestContext RequestContext   `json:"request_context"`
	Sessions       []FixtureSession `json:"sessions"`
	Expected       Expected         `json:"expected"`
}

type FixtureSession struct {
	ID           string            `json:"id"`
	Selector     Selector          `json:"selector"`
	Level        Level             `json:"level"`
	ExpiresAt    time.Time         `json:"expires_at"`
	Caps         Caps              `json:"caps"`
	Labels       map[string]string `json:"labels"`
	ServiceScope []string          `json:"service_scope"`
}

type Expected struct {
	Matched        bool              `json:"matched"`
	SessionID      *string           `json:"session_id"`
	EffectiveLevel Level             `json:"effective_level"`
	ReasonCode     ReasonCode        `json:"reason_code"`
	Labels         map[string]string `json:"labels"`
}

func TestConformance(t *testing.T) {
	fixtures, err := loadFixtures()
	if err != nil {
		t.Fatalf("failed to load fixtures: %v", err)
	}

	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			sessions := convertSessions(f.Sessions)
			serviceName := f.ServiceName
			if serviceName == "" {
				serviceName = "test-service"
			}

			decision := Decide(f.Now, serviceName, f.RequestContext, sessions)

			if decision.Matched != f.Expected.Matched {
				t.Errorf("matched: got %v, want %v", decision.Matched, f.Expected.Matched)
			}

			expectedSessionID := ""
			if f.Expected.SessionID != nil {
				expectedSessionID = *f.Expected.SessionID
			}
			if decision.SessionID != expectedSessionID {
				t.Errorf("session_id: got %q, want %q", decision.SessionID, expectedSessionID)
			}

			if decision.EffectiveLevel != f.Expected.EffectiveLevel {
				t.Errorf("effective_level: got %q, want %q", decision.EffectiveLevel, f.Expected.EffectiveLevel)
			}

			if decision.ReasonCode != f.Expected.ReasonCode {
				t.Errorf("reason_code: got %q, want %q", decision.ReasonCode, f.Expected.ReasonCode)
			}

			if !labelsEqual(decision.Labels, f.Expected.Labels) {
				t.Errorf("labels: got %v, want %v", decision.Labels, f.Expected.Labels)
			}
		})
	}
}

func loadFixtures() ([]Fixture, error) {
	// Try local fixtures first (relative to spec/ in monorepo)
	localPath := "./spec/fixtures/v1.json"
	body, err := os.ReadFile(localPath)
	if err == nil {
		var fixtures []Fixture
		if err := json.Unmarshal(body, &fixtures); err != nil {
			return nil, fmt.Errorf("JSON unmarshal failed: %w", err)
		}
		return fixtures, nil
	}

	// Fallback to fetching from GitHub
	resp, err := http.Get(fixturesURL)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET failed (and local fixtures not found): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body failed: %w", err)
	}

	var fixtures []Fixture
	if err := json.Unmarshal(body, &fixtures); err != nil {
		return nil, fmt.Errorf("JSON unmarshal failed: %w", err)
	}

	return fixtures, nil
}

func convertSessions(fixtureSessions []FixtureSession) []Session {
	sessions := make([]Session, len(fixtureSessions))
	for i, fs := range fixtureSessions {
		sessions[i] = Session{
			ID:           fs.ID,
			Selector:     fs.Selector,
			Level:        fs.Level,
			ExpiresAt:    fs.ExpiresAt,
			Caps:         fs.Caps,
			Labels:       fs.Labels,
			ServiceScope: fs.ServiceScope,
		}
	}
	return sessions
}

func labelsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestDecideNoSessions(t *testing.T) {
	now := time.Now()
	ctx := RequestContext{UserID: "u123", Route: "/api/test"}

	decision := Decide(now, "api", ctx, nil)

	if decision.Matched {
		t.Error("expected no match with empty sessions")
	}
	if decision.ReasonCode != ReasonNoMatch {
		t.Errorf("expected NO_MATCH, got %s", decision.ReasonCode)
	}
}

func TestDecideExpiredSession(t *testing.T) {
	now := time.Now()
	ctx := RequestContext{UserID: "u123", Route: "/api/test"}
	sessions := []Session{
		{
			ID:        "s1",
			Selector:  Selector{UserID: "u123"},
			Level:     LevelDebug,
			ExpiresAt: now.Add(-1 * time.Minute),
			Labels:    map[string]string{},
		},
	}

	decision := Decide(now, "api", ctx, sessions)

	if decision.Matched {
		t.Error("expected no match with expired session")
	}
	if decision.ReasonCode != ReasonExpired {
		t.Errorf("expected EXPIRED, got %s", decision.ReasonCode)
	}
}

func TestDecideBasicMatch(t *testing.T) {
	now := time.Now()
	ctx := RequestContext{UserID: "u123", Route: "/api/test"}
	sessions := []Session{
		{
			ID:        "s1",
			Selector:  Selector{UserID: "u123"},
			Level:     LevelDebug,
			ExpiresAt: now.Add(10 * time.Minute),
			Labels:    map[string]string{"env": "test"},
		},
	}

	decision := Decide(now, "api", ctx, sessions)

	if !decision.Matched {
		t.Error("expected match")
	}
	if decision.SessionID != "s1" {
		t.Errorf("expected session s1, got %s", decision.SessionID)
	}
	if decision.EffectiveLevel != LevelDebug {
		t.Errorf("expected debug level, got %s", decision.EffectiveLevel)
	}
	if decision.Labels["env"] != "test" {
		t.Errorf("expected label env=test, got %v", decision.Labels)
	}
}

func TestTieBreakByLevel(t *testing.T) {
	now := time.Now()
	ctx := RequestContext{UserID: "u123", Route: "/api/test"}
	sessions := []Session{
		{
			ID:        "s1",
			Selector:  Selector{UserID: "u123"},
			Level:     LevelDebug,
			ExpiresAt: now.Add(10 * time.Minute),
			Labels:    map[string]string{},
		},
		{
			ID:        "s2",
			Selector:  Selector{UserID: "u123"},
			Level:     LevelTrace,
			ExpiresAt: now.Add(10 * time.Minute),
			Labels:    map[string]string{},
		},
	}

	decision := Decide(now, "api", ctx, sessions)

	if decision.SessionID != "s2" {
		t.Errorf("expected s2 (trace) to win, got %s", decision.SessionID)
	}
}

func TestTieBreakBySpecificity(t *testing.T) {
	now := time.Now()
	ctx := RequestContext{UserID: "u123", Route: "/api/test"}
	sessions := []Session{
		{
			ID:        "s1",
			Selector:  Selector{UserID: "u123"},
			Level:     LevelDebug,
			ExpiresAt: now.Add(10 * time.Minute),
			Labels:    map[string]string{},
		},
		{
			ID:        "s2",
			Selector:  Selector{UserID: "u123", Route: "/api/test"},
			Level:     LevelDebug,
			ExpiresAt: now.Add(10 * time.Minute),
			Labels:    map[string]string{},
		},
	}

	decision := Decide(now, "api", ctx, sessions)

	if decision.SessionID != "s2" {
		t.Errorf("expected s2 (more specific) to win, got %s", decision.SessionID)
	}
}

func TestServiceScopeFiltering(t *testing.T) {
	now := time.Now()
	ctx := RequestContext{UserID: "u123", Route: "/api/test"}
	sessions := []Session{
		{
			ID:           "s1",
			Selector:     Selector{UserID: "u123"},
			Level:        LevelDebug,
			ExpiresAt:    now.Add(10 * time.Minute),
			ServiceScope: []string{"worker", "billing"},
			Labels:       map[string]string{},
		},
	}

	decision := Decide(now, "api", ctx, sessions)

	if decision.Matched {
		t.Error("expected no match when service not in scope")
	}

	decision = Decide(now, "worker", ctx, sessions)

	if !decision.Matched {
		t.Error("expected match when service in scope")
	}
}
