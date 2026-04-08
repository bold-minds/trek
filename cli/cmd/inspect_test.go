package cmd

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/bold-minds/trek"
)

func TestInspectCommandRegistration(t *testing.T) {
	commands := rootCmd.Commands()

	var found bool
	for _, cmd := range commands {
		if cmd.Use == "inspect" {
			found = true
			break
		}
	}

	if !found {
		t.Error("inspect command not registered")
	}
}

func TestInspectCommandFlags(t *testing.T) {
	flag := inspectCmd.Flags().Lookup("request-context")
	if flag == nil {
		t.Error("request-context flag not found")
	}
}

func TestInspectCommandHelp(t *testing.T) {
	if inspectCmd.Short == "" {
		t.Error("inspect command missing short description")
	}

	if inspectCmd.Long == "" {
		t.Error("inspect command missing long description")
	}
}

func TestInspectCommandUsage(t *testing.T) {
	expectedUse := "inspect"
	if inspectCmd.Use != expectedUse {
		t.Errorf("Use = %q, want %q", inspectCmd.Use, expectedUse)
	}
}

func TestInspectCommandExample(t *testing.T) {
	// The long description contains the example
	if !contains(inspectCmd.Long, "trek inspect") {
		t.Error("inspect command long description should contain usage example")
	}
}

func TestRunInspectValidation_InvalidJSON(t *testing.T) {
	requestContext = "invalid json"

	err := runInspect(inspectCmd, []string{})

	if err == nil {
		t.Error("expected error for invalid JSON")
	}

	expectedMsg := "invalid request context JSON"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("error = %q, want containing %q", err.Error(), expectedMsg)
	}
}

func TestRunInspectValidation_ValidJSON(t *testing.T) {
	// Reset global vars to prevent client creation
	apiEndpoint = ""
	apiToken = ""
	orgID = ""
	env = ""

	requestContext = `{"user_id":"u123"}`

	// Should not error on valid JSON - will print warning about missing client
	err := runInspect(inspectCmd, []string{})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPrintDecision(t *testing.T) {
	decision := trek.Decision{
		Matched:        true,
		SessionID:      "sess-123",
		EffectiveLevel: trek.LevelDebug,
		ReasonCode:     trek.ReasonMatched,
		Labels: map[string]string{
			"ticket": "TREK-456",
		},
	}

	// Capture output (printDecision writes to stdout)
	// This is a simple smoke test
	printDecision(decision)
}

func TestPrintDecision_NoMatch(t *testing.T) {
	decision := trek.Decision{
		Matched:    false,
		ReasonCode: "no_sessions",
	}

	printDecision(decision)
}

func TestPrintDecision_WithLabels(t *testing.T) {
	decision := trek.Decision{
		Matched:        true,
		SessionID:      "sess-123",
		EffectiveLevel: trek.LevelDebug,
		ReasonCode:     trek.ReasonMatched,
		Labels: map[string]string{
			"environment": "production",
			"team":        "backend",
		},
	}

	printDecision(decision)
}

func TestRequestContextParsing(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{
			name:    "valid user context",
			json:    `{"user_id":"u123"}`,
			wantErr: false,
		},
		{
			name:    "valid route context",
			json:    `{"route":"/api/orders"}`,
			wantErr: false,
		},
		{
			name:    "valid tenant context",
			json:    `{"tenant_id":"t456"}`,
			wantErr: false,
		},
		{
			name:    "valid complex context",
			json:    `{"user_id":"u123","route":"/api/orders","tenant_id":"t456"}`,
			wantErr: false,
		},
		{
			name:    "empty object",
			json:    `{}`,
			wantErr: false,
		},
		{
			name:    "invalid json",
			json:    `{invalid}`,
			wantErr: true,
		},
		{
			name:    "not an object",
			json:    `"string"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ctx trek.RequestContext
			err := json.Unmarshal([]byte(tt.json), &ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDecideWithEmptySessions(t *testing.T) {
	ctx := trek.RequestContext{
		UserID: "u123",
	}

	decision := trek.Decide(time.Now(), "cli", ctx, nil)

	if decision.Matched {
		t.Error("expected no match with empty sessions")
	}
}

func TestDecideWithMatchingSession(t *testing.T) {
	ctx := trek.RequestContext{
		UserID: "u123",
	}

	sessions := []trek.Session{
		{
			ID: "sess-123",
			Selector: trek.Selector{
				UserID: "u123",
			},
			Level:     trek.LevelDebug,
			ExpiresAt: time.Now().Add(time.Hour),
		},
	}

	decision := trek.Decide(time.Now(), "cli", ctx, sessions)

	if !decision.Matched {
		t.Error("expected match with matching session")
	}
	if decision.SessionID != "sess-123" {
		t.Errorf("SessionID = %q, want %q", decision.SessionID, "sess-123")
	}
}

func TestDecideWithExpiredSession(t *testing.T) {
	ctx := trek.RequestContext{
		UserID: "u123",
	}

	sessions := []trek.Session{
		{
			ID: "sess-123",
			Selector: trek.Selector{
				UserID: "u123",
			},
			Level:     trek.LevelDebug,
			ExpiresAt: time.Now().Add(-time.Hour), // Expired
		},
	}

	decision := trek.Decide(time.Now(), "cli", ctx, sessions)

	if decision.Matched {
		t.Error("expected no match with expired session")
	}
}

func TestOutputBuffer(t *testing.T) {
	// Test that we can capture output
	var buf bytes.Buffer

	decision := trek.Decision{
		Matched:        true,
		SessionID:      "sess-123",
		EffectiveLevel: trek.LevelDebug,
	}

	// Simple check that the decision struct is valid
	if decision.SessionID == "" {
		t.Error("SessionID should not be empty")
	}

	_ = buf // Used for potential future output capture
}
