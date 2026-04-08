package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigFile(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	tests := []struct {
		name          string
		configContent string
		envVars       map[string]string
		wantEndpoint  string
		wantToken     string
		wantOrg       string
		wantEnv       string
	}{
		{
			name: "loads all values from config",
			configContent: `endpoint: https://api.trek.dev
token: test-token-123
org: test-org
env: production`,
			wantEndpoint: "https://api.trek.dev",
			wantToken:    "test-token-123",
			wantOrg:      "test-org",
			wantEnv:      "production",
		},
		{
			name: "env vars take precedence",
			configContent: `endpoint: https://config.trek.dev
token: config-token
org: config-org
env: staging`,
			envVars: map[string]string{
				"TREK_API_ENDPOINT": "https://env.trek.dev",
				"TREK_API_TOKEN":    "env-token",
			},
			wantEndpoint: "https://env.trek.dev",
			wantToken:    "env-token",
			wantOrg:      "config-org",
			wantEnv:      "staging",
		},
		{
			name:          "partial config",
			configContent: `endpoint: https://partial.trek.dev`,
			wantEndpoint:  "https://partial.trek.dev",
			wantToken:     "",
			wantOrg:       "",
			wantEnv:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global vars
			apiEndpoint = ""
			apiToken = ""
			orgID = ""
			env = ""

			// Set env vars
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			// Initialize from env vars first (simulates initConfig)
			if apiEndpoint == "" {
				apiEndpoint = os.Getenv("TREK_API_ENDPOINT")
			}
			if apiToken == "" {
				apiToken = os.Getenv("TREK_API_TOKEN")
			}

			// Write config file
			if tt.configContent != "" {
				if err := os.WriteFile(configPath, []byte(tt.configContent), 0644); err != nil {
					t.Fatalf("failed to write config file: %v", err)
				}
			}

			// Load config
			loadConfigFile(configPath)

			// Verify
			if apiEndpoint != tt.wantEndpoint {
				t.Errorf("apiEndpoint = %q, want %q", apiEndpoint, tt.wantEndpoint)
			}
			if apiToken != tt.wantToken {
				t.Errorf("apiToken = %q, want %q", apiToken, tt.wantToken)
			}
			if orgID != tt.wantOrg {
				t.Errorf("orgID = %q, want %q", orgID, tt.wantOrg)
			}
			if env != tt.wantEnv {
				t.Errorf("env = %q, want %q", env, tt.wantEnv)
			}
		})
	}
}

func TestLoadConfigFileNotFound(t *testing.T) {
	// Reset global vars
	apiEndpoint = ""
	apiToken = ""
	orgID = ""
	env = ""

	// Should not error on missing file
	loadConfigFile("/nonexistent/path/config.yaml")

	// Vars should remain empty
	if apiEndpoint != "" {
		t.Errorf("apiEndpoint should be empty, got %q", apiEndpoint)
	}
}

func TestLoadConfigFileInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Write invalid YAML
	if err := os.WriteFile(configPath, []byte("invalid: yaml: content: ["), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Reset global vars
	apiEndpoint = ""

	// Should not panic on invalid YAML
	loadConfigFile(configPath)

	// Vars should remain empty
	if apiEndpoint != "" {
		t.Errorf("apiEndpoint should be empty after invalid YAML, got %q", apiEndpoint)
	}
}

func TestGetClientValidation(t *testing.T) {
	tests := []struct {
		name        string
		endpoint    string
		token       string
		org         string
		environment string
		wantErr     string
	}{
		{
			name:    "missing endpoint",
			wantErr: "API endpoint required",
		},
		{
			name:     "missing token",
			endpoint: "https://api.trek.dev",
			wantErr:  "API token required",
		},
		{
			name:     "missing org",
			endpoint: "https://api.trek.dev",
			token:    "test-token",
			wantErr:  "org ID required",
		},
		{
			name:     "missing env",
			endpoint: "https://api.trek.dev",
			token:    "test-token",
			org:      "test-org",
			wantErr:  "env required",
		},
		{
			name:        "all valid",
			endpoint:    "https://api.trek.dev",
			token:       "test-token",
			org:         "test-org",
			environment: "prod",
			wantErr:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiEndpoint = tt.endpoint
			apiToken = tt.token
			orgID = tt.org
			env = tt.environment

			_, err := getClient()

			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("getClient() unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("getClient() expected error containing %q, got nil", tt.wantErr)
				} else if !contains(err.Error(), tt.wantErr) {
					t.Errorf("getClient() error = %q, want containing %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
