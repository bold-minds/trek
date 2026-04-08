package cmd

import (
	"testing"
	"time"

	"github.com/bold-minds/trek"
)

func TestCreateCommandRegistration(t *testing.T) {
	var found bool
	for _, cmd := range sessionCmd.Commands() {
		if cmd.Use == "create" {
			found = true
			break
		}
	}

	if !found {
		t.Error("create command not registered under session")
	}
}

func TestCreateCommandFlags(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
		wantType string
	}{
		{"user flag", "user", "string"},
		{"request flag", "request", "string"},
		{"tenant flag", "tenant", "string"},
		{"route flag", "route", "string"},
		{"ttl flag", "ttl", "duration"},
		{"level flag", "level", "string"},
		{"reason flag", "reason", "string"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := sessionCreateCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("flag %q not found", tt.flagName)
			}
		})
	}
}

func TestCreateCommandDefaults(t *testing.T) {
	// Test default TTL
	ttlFlag := sessionCreateCmd.Flags().Lookup("ttl")
	if ttlFlag == nil {
		t.Fatal("ttl flag not found")
	}
	if ttlFlag.DefValue != "15m0s" {
		t.Errorf("ttl default = %q, want %q", ttlFlag.DefValue, "15m0s")
	}

	// Test default level
	levelFlag := sessionCreateCmd.Flags().Lookup("level")
	if levelFlag == nil {
		t.Fatal("level flag not found")
	}
	if levelFlag.DefValue != "debug" {
		t.Errorf("level default = %q, want %q", levelFlag.DefValue, "debug")
	}
}

func TestRunCreateValidation_EmptySelector(t *testing.T) {
	// Reset global vars
	userID = ""
	requestID = ""
	tenantID = ""
	route = ""

	err := runCreate(sessionCreateCmd, []string{})

	if err == nil {
		t.Error("expected error for empty selector")
	}

	expectedMsg := "at least one selector field required"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("error = %q, want containing %q", err.Error(), expectedMsg)
	}
}

func TestRunCreateValidation_MissingClient(t *testing.T) {
	// Reset global vars
	apiEndpoint = ""
	apiToken = ""
	orgID = ""
	env = ""

	// Set a selector
	userID = "test-user"

	err := runCreate(sessionCreateCmd, []string{})

	if err == nil {
		t.Error("expected error for missing client config")
	}
}

func TestSelectorConstruction(t *testing.T) {
	tests := []struct {
		name      string
		userID    string
		requestID string
		tenantID  string
		route     string
		wantEmpty bool
	}{
		{
			name:      "all empty",
			wantEmpty: true,
		},
		{
			name:      "with user",
			userID:    "u123",
			wantEmpty: false,
		},
		{
			name:      "with request",
			requestID: "req-456",
			wantEmpty: false,
		},
		{
			name:      "with tenant",
			tenantID:  "tenant-789",
			wantEmpty: false,
		},
		{
			name:      "with route",
			route:     "/api/orders",
			wantEmpty: false,
		},
		{
			name:      "multiple selectors",
			userID:    "u123",
			tenantID:  "t456",
			wantEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector := trek.Selector{
				UserID:    tt.userID,
				RequestID: tt.requestID,
				TenantID:  tt.tenantID,
				Route:     tt.route,
			}

			isEmpty := trek.IsEmptySelector(selector)
			if isEmpty != tt.wantEmpty {
				t.Errorf("IsEmptySelector() = %v, want %v", isEmpty, tt.wantEmpty)
			}
		})
	}
}

func TestTTLParsing(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"minutes", "15m", 15 * time.Minute, false},
		{"hours", "1h", time.Hour, false},
		{"seconds", "30s", 30 * time.Second, false},
		{"mixed", "1h30m", 90 * time.Minute, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := time.ParseDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestLevelValidation(t *testing.T) {
	validLevels := []string{"debug", "trace", "info"}

	for _, level := range validLevels {
		t.Run(level, func(t *testing.T) {
			l := trek.Level(level)
			if l == "" {
				t.Errorf("level %q should be valid", level)
			}
		})
	}
}

func TestTTLToSeconds(t *testing.T) {
	tests := []struct {
		ttl  time.Duration
		want int
	}{
		{15 * time.Minute, 900},
		{time.Hour, 3600},
		{30 * time.Second, 30},
		{90 * time.Minute, 5400},
	}

	for _, tt := range tests {
		t.Run(tt.ttl.String(), func(t *testing.T) {
			got := int(tt.ttl.Seconds())
			if got != tt.want {
				t.Errorf("TTL.Seconds() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCreateSessionRequest(t *testing.T) {
	selector := trek.Selector{
		UserID: "u123",
	}

	req := trek.CreateSessionRequest{
		Selector:   selector,
		Level:      trek.Level("debug"),
		TTLSeconds: 900,
		Reason:     "testing",
	}

	if req.Selector.UserID != "u123" {
		t.Errorf("Selector.UserID = %q, want %q", req.Selector.UserID, "u123")
	}
	if req.Level != "debug" {
		t.Errorf("Level = %q, want %q", req.Level, "debug")
	}
	if req.TTLSeconds != 900 {
		t.Errorf("TTLSeconds = %d, want %d", req.TTLSeconds, 900)
	}
	if req.Reason != "testing" {
		t.Errorf("Reason = %q, want %q", req.Reason, "testing")
	}
}
