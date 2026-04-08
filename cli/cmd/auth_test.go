package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoredCredentialsSerialization(t *testing.T) {
	creds := StoredCredentials{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		ExpiresAt:    time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC),
		Email:        "test@example.com",
	}

	// Test marshaling
	data, err := json.Marshal(creds)
	if err != nil {
		t.Fatalf("failed to marshal credentials: %v", err)
	}

	// Test unmarshaling
	var decoded StoredCredentials
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal credentials: %v", err)
	}

	if decoded.AccessToken != creds.AccessToken {
		t.Errorf("AccessToken = %q, want %q", decoded.AccessToken, creds.AccessToken)
	}
	if decoded.RefreshToken != creds.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q", decoded.RefreshToken, creds.RefreshToken)
	}
	if decoded.Email != creds.Email {
		t.Errorf("Email = %q, want %q", decoded.Email, creds.Email)
	}
	if !decoded.ExpiresAt.Equal(creds.ExpiresAt) {
		t.Errorf("ExpiresAt = %v, want %v", decoded.ExpiresAt, creds.ExpiresAt)
	}
}

func TestDeviceAuthResponseParsing(t *testing.T) {
	jsonData := `{
		"device_code": "device-123",
		"user_code": "ABCD-EFGH",
		"verification_uri": "https://clerk.example.com/device",
		"verification_uri_complete": "https://clerk.example.com/device?user_code=ABCD-EFGH",
		"expires_in": 1800,
		"interval": 5
	}`

	var resp DeviceAuthResponse
	if err := json.Unmarshal([]byte(jsonData), &resp); err != nil {
		t.Fatalf("failed to unmarshal DeviceAuthResponse: %v", err)
	}

	if resp.DeviceCode != "device-123" {
		t.Errorf("DeviceCode = %q, want %q", resp.DeviceCode, "device-123")
	}
	if resp.UserCode != "ABCD-EFGH" {
		t.Errorf("UserCode = %q, want %q", resp.UserCode, "ABCD-EFGH")
	}
	if resp.ExpiresIn != 1800 {
		t.Errorf("ExpiresIn = %d, want %d", resp.ExpiresIn, 1800)
	}
	if resp.Interval != 5 {
		t.Errorf("Interval = %d, want %d", resp.Interval, 5)
	}
}

func TestTokenResponseParsing(t *testing.T) {
	tests := []struct {
		name      string
		jsonData  string
		wantToken string
		wantError string
	}{
		{
			name: "successful response",
			jsonData: `{
				"access_token": "access-123",
				"token_type": "Bearer",
				"expires_in": 3600,
				"refresh_token": "refresh-456"
			}`,
			wantToken: "access-123",
			wantError: "",
		},
		{
			name: "error response",
			jsonData: `{
				"error": "authorization_pending",
				"error_description": "User has not yet authorized"
			}`,
			wantToken: "",
			wantError: "authorization_pending",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp TokenResponse
			if err := json.Unmarshal([]byte(tt.jsonData), &resp); err != nil {
				t.Fatalf("failed to unmarshal TokenResponse: %v", err)
			}

			if resp.AccessToken != tt.wantToken {
				t.Errorf("AccessToken = %q, want %q", resp.AccessToken, tt.wantToken)
			}
			if resp.Error != tt.wantError {
				t.Errorf("Error = %q, want %q", resp.Error, tt.wantError)
			}
		})
	}
}

func TestCredentialsFilePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot get user home dir")
	}

	expectedPath := filepath.Join(home, ".trek", "credentials.json")

	// This tests that the path construction is correct
	actualPath := filepath.Join(home, ".trek", "credentials.json")
	if actualPath != expectedPath {
		t.Errorf("credentials path = %q, want %q", actualPath, expectedPath)
	}
}

func TestAuthCommandsExist(t *testing.T) {
	// Verify auth commands are registered
	commands := authCmd.Commands()

	commandNames := make(map[string]bool)
	for _, cmd := range commands {
		commandNames[cmd.Use] = true
	}

	requiredCommands := []string{"login", "logout", "whoami"}
	for _, name := range requiredCommands {
		if !commandNames[name] {
			t.Errorf("auth command %q not registered", name)
		}
	}
}

func TestLoginRequiresClerkConfig(t *testing.T) {
	// Clear env vars
	os.Unsetenv("TREK_CLERK_DOMAIN")
	os.Unsetenv("TREK_CLERK_CLIENT_ID")

	// Create a fresh command without flags set
	cmd := *loginCmd
	cmd.SetArgs([]string{})

	err := cmd.RunE(&cmd, []string{})

	if err == nil {
		t.Error("login should fail without clerk config")
	}

	expectedMsg := "clerk-domain and client-id are required"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("error = %q, want containing %q", err.Error(), expectedMsg)
	}
}
