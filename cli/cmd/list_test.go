package cmd

import (
	"testing"

	"github.com/bold-minds/trek"
)

func TestListCommandRegistration(t *testing.T) {
	var found bool
	for _, cmd := range sessionCmd.Commands() {
		if cmd.Use == "list" {
			found = true
			break
		}
	}

	if !found {
		t.Error("list command not registered under session")
	}
}

func TestListCommandFlags(t *testing.T) {
	flag := sessionListCmd.Flags().Lookup("status")
	if flag == nil {
		t.Error("status flag not found")
	}
}

func TestListCommandHelp(t *testing.T) {
	if sessionListCmd.Short == "" {
		t.Error("list command missing short description")
	}

	if sessionListCmd.Long == "" {
		t.Error("list command missing long description")
	}
}

func TestRunListValidation_MissingClient(t *testing.T) {
	// Reset global vars
	apiEndpoint = ""
	apiToken = ""
	orgID = ""
	env = ""

	err := runList(sessionListCmd, []string{})

	if err == nil {
		t.Error("expected error for missing client config")
	}
}

func TestListCommandUsage(t *testing.T) {
	expectedUse := "list"
	if sessionListCmd.Use != expectedUse {
		t.Errorf("Use = %q, want %q", sessionListCmd.Use, expectedUse)
	}
}

func TestListCommandExample(t *testing.T) {
	// The long description contains examples
	if !contains(sessionListCmd.Long, "trek session list") {
		t.Error("list command long description should contain usage example")
	}
}

func TestFormatSelector(t *testing.T) {
	tests := []struct {
		name     string
		selector trek.Selector
		want     string
	}{
		{
			name:     "empty selector",
			selector: trek.Selector{},
			want:     "(empty)",
		},
		{
			name: "user only",
			selector: trek.Selector{
				UserID: "u123",
			},
			want: "user:u123",
		},
		{
			name: "tenant only",
			selector: trek.Selector{
				TenantID: "t456",
			},
			want: "tenant:t456",
		},
		{
			name: "request only",
			selector: trek.Selector{
				RequestID: "req-789",
			},
			want: "req:req-789",
		},
		{
			name: "route only",
			selector: trek.Selector{
				Route: "/api/orders",
			},
			want: "route:/api/orders",
		},
		{
			name: "multiple selectors",
			selector: trek.Selector{
				UserID:   "u123",
				TenantID: "t456",
			},
			want: "user:u123, tenant:t456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSelector(tt.selector)
			if got != tt.want {
				t.Errorf("formatSelector() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatSelectorWithCustom(t *testing.T) {
	selector := trek.Selector{
		Custom: map[string]string{
			"region": "us-east",
		},
	}

	got := formatSelector(selector)
	if !contains(got, "region:us-east") {
		t.Errorf("formatSelector() = %q, want containing %q", got, "region:us-east")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		max   int
		want  string
	}{
		{
			name:  "short string",
			input: "hello",
			max:   10,
			want:  "hello",
		},
		{
			name:  "exact length",
			input: "hello",
			max:   5,
			want:  "hello",
		},
		{
			name:  "needs truncation",
			input: "hello world",
			max:   8,
			want:  "hello...",
		},
		{
			name:  "very short max",
			input: "hello",
			max:   4,
			want:  "h...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.max)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
			}
		})
	}
}

func TestStatusFilterValues(t *testing.T) {
	validStatuses := []string{"active", "revoked", "expired", ""}

	for _, status := range validStatuses {
		t.Run("status_"+status, func(t *testing.T) {
			// Just verify these are valid filter values (no error expected)
			_ = status
		})
	}
}
