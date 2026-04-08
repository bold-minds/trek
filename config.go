package trek

import (
	"errors"
	"os"
	"strings"
	"time"
)

// Sentinel errors for the Trek SDK.
var (
	ErrNotConfigured = errors.New("trek: client not configured")
	ErrInvalidConfig = errors.New("trek: invalid configuration")
	ErrPollFailed    = errors.New("trek: failed to poll sessions")
)

// Config holds the configuration for the Trek SDK.
type Config struct {
	ServiceName  string
	OrgID        string
	Env          string
	APIEndpoint  string
	APIToken     string
	PollInterval time.Duration

	RedactAttr  RedactAttrFunc
	RedactEvent RedactEventFunc

	StrictInit       bool
	EnableGlobalCaps bool // Enable global caps via server (requires Redis on server)
}

// RedactAttrFunc is called for each log attribute to optionally redact it.
// Return the new value and whether to drop the attribute entirely.
type RedactAttrFunc func(key string, val any) (newVal any, drop bool)

// RedactEventFunc is called for each log event to optionally redact or drop it.
type RedactEventFunc func(msg string, attrs map[string]any) (newAttrs map[string]any, drop bool)

// DefaultConfig returns a Config with defaults and environment variable overrides.
func DefaultConfig() Config {
	pollInterval := 5 * time.Second
	if v := os.Getenv("TREK_POLL_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			pollInterval = d
		}
	}

	return Config{
		ServiceName:  getEnvOrDefault("TREK_SERVICE_NAME", "unknown"),
		OrgID:        os.Getenv("TREK_ORG_ID"),
		Env:          getEnvOrDefault("TREK_ENV", "dev"),
		APIEndpoint:  os.Getenv("TREK_API_ENDPOINT"),
		APIToken:     os.Getenv("TREK_API_TOKEN"),
		PollInterval: pollInterval,
		StrictInit:   os.Getenv("TREK_STRICT_INIT") == "true",
	}
}

// Validate checks that required configuration is present.
func (c Config) Validate() error {
	if c.ServiceName == "" {
		return &ConfigError{Field: "ServiceName", Message: "required"}
	}
	if c.OrgID == "" {
		return &ConfigError{Field: "OrgID", Message: "required"}
	}
	if c.APIEndpoint == "" {
		return &ConfigError{Field: "APIEndpoint", Message: "required"}
	}
	if c.APIToken == "" {
		return &ConfigError{Field: "APIToken", Message: "required"}
	}
	return nil
}

// ConfigError represents a configuration validation error.
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return "trek: config error: " + e.Field + ": " + e.Message
}

func getEnvOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// CommonRedactKeys is a list of keys commonly containing sensitive data.
var CommonRedactKeys = []string{
	"password",
	"passwd",
	"secret",
	"token",
	"api_key",
	"apikey",
	"authorization",
	"auth",
	"cookie",
	"set-cookie",
	"x-api-key",
	"x-auth-token",
	"ssn",
	"credit_card",
	"creditcard",
}

// CommonRedactor returns a RedactAttrFunc that redacts common sensitive keys.
// Key matching is case-insensitive.
func CommonRedactor() RedactAttrFunc {
	keySet := make(map[string]struct{}, len(CommonRedactKeys))
	for _, k := range CommonRedactKeys {
		keySet[strings.ToLower(k)] = struct{}{}
	}

	return func(key string, val any) (any, bool) {
		if _, found := keySet[strings.ToLower(key)]; found {
			return "[REDACTED]", false
		}
		return val, false
	}
}
