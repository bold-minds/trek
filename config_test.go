package trek

import (
	"testing"
)

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: Config{
				ServiceName: "my-service",
				OrgID:       "org1",
				APIEndpoint: "http://localhost:8080",
				APIToken:    "token123",
			},
			wantErr: false,
		},
		{
			name: "missing service name",
			config: Config{
				OrgID:       "org1",
				APIEndpoint: "http://localhost:8080",
				APIToken:    "token123",
			},
			wantErr: true,
			errMsg:  "ServiceName",
		},
		{
			name: "missing org ID",
			config: Config{
				ServiceName: "my-service",
				APIEndpoint: "http://localhost:8080",
				APIToken:    "token123",
			},
			wantErr: true,
			errMsg:  "OrgID",
		},
		{
			name: "missing API endpoint",
			config: Config{
				ServiceName: "my-service",
				OrgID:       "org1",
				APIToken:    "token123",
			},
			wantErr: true,
			errMsg:  "APIEndpoint",
		},
		{
			name: "missing API token",
			config: Config{
				ServiceName: "my-service",
				OrgID:       "org1",
				APIEndpoint: "http://localhost:8080",
			},
			wantErr: true,
			errMsg:  "APIToken",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else {
					configErr, ok := err.(*ConfigError)
					if !ok {
						t.Errorf("expected ConfigError, got %T", err)
					} else if configErr.Field != tt.errMsg {
						t.Errorf("expected field %s, got %s", tt.errMsg, configErr.Field)
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestCommonRedactor(t *testing.T) {
	redactor := CommonRedactor()

	tests := []struct {
		key      string
		value    any
		expected any
		dropped  bool
	}{
		{"password", "secret123", "[REDACTED]", false},
		{"Password", "secret123", "[REDACTED]", false}, // case insensitive
		{"PASSWORD", "secret123", "[REDACTED]", false}, // case insensitive
		{"api_key", "key123", "[REDACTED]", false},
		{"token", "tok123", "[REDACTED]", false},
		{"username", "john", "john", false}, // not redacted
		{"email", "john@example.com", "john@example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result, dropped := redactor(tt.key, tt.value)
			if dropped != tt.dropped {
				t.Errorf("expected dropped=%v, got %v", tt.dropped, dropped)
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestConfigErrorMessage(t *testing.T) {
	err := &ConfigError{Field: "OrgID", Message: "required"}
	expected := "trek: config error: OrgID: required"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}
