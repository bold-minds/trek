package store

import (
	"testing"
)

func TestHashToken(t *testing.T) {
	token := "test-token-12345"

	hash1, err := HashToken(token)
	if err != nil {
		t.Fatalf("HashToken() error = %v", err)
	}

	if hash1 == "" {
		t.Error("HashToken() returned empty hash")
	}

	if hash1 == token {
		t.Error("HashToken() should not return plaintext token")
	}

	// Hash should contain salt separator
	if !containsSeparator(hash1, "$") {
		t.Error("HashToken() should return salted hash with $ separator")
	}

	// Two hashes of same token should be different (due to random salt)
	hash2, err := HashToken(token)
	if err != nil {
		t.Fatalf("HashToken() second call error = %v", err)
	}

	if hash1 == hash2 {
		t.Error("HashToken() should produce different hashes due to random salt")
	}
}

func TestHashTokenWithSalt(t *testing.T) {
	token := "test-token"
	salt := []byte("fixed-salt-1234!")

	hash1 := HashTokenWithSalt(token, salt)
	hash2 := HashTokenWithSalt(token, salt)

	if hash1 != hash2 {
		t.Error("HashTokenWithSalt() with same salt should produce same hash")
	}

	// Different token should produce different hash
	hash3 := HashTokenWithSalt("different-token", salt)
	if hash1 == hash3 {
		t.Error("HashTokenWithSalt() different tokens should produce different hashes")
	}

	// Different salt should produce different hash
	differentSalt := []byte("different-salt!!")
	hash4 := HashTokenWithSalt(token, differentSalt)
	if hash1 == hash4 {
		t.Error("HashTokenWithSalt() different salts should produce different hashes")
	}
}

func TestVerifyToken(t *testing.T) {
	token := "my-secret-token"

	hash, err := HashToken(token)
	if err != nil {
		t.Fatalf("HashToken() error = %v", err)
	}

	tests := []struct {
		name       string
		token      string
		storedHash string
		want       bool
	}{
		{
			name:       "correct token",
			token:      token,
			storedHash: hash,
			want:       true,
		},
		{
			name:       "incorrect token",
			token:      "wrong-token",
			storedHash: hash,
			want:       false,
		},
		{
			name:       "empty token",
			token:      "",
			storedHash: hash,
			want:       false,
		},
		{
			name:       "empty hash",
			token:      token,
			storedHash: "",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VerifyToken(tt.token, tt.storedHash)
			if got != tt.want {
				t.Errorf("VerifyToken() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVerifyToken_LegacyUnsaltedHash(t *testing.T) {
	// Legacy hashes are plain SHA256 without salt
	// This tests backward compatibility
	token := "legacy-token"

	// A legacy hash wouldn't have the $ separator
	// The VerifyToken function should handle this case
	legacyHash := "abcdef1234567890" // Not a real SHA256, just for format testing

	// Should not match because it's not a valid hash of the token
	if VerifyToken(token, legacyHash) {
		t.Error("VerifyToken() should not match invalid legacy hash")
	}
}

func TestSplitHashParts(t *testing.T) {
	tests := []struct {
		name     string
		hash     string
		wantLen  int
		wantSalt string
	}{
		{
			name:     "salted hash",
			hash:     "salt$hash",
			wantLen:  2,
			wantSalt: "salt",
		},
		{
			name:    "legacy hash no separator",
			hash:    "legacyhashnoseparator",
			wantLen: 1,
		},
		{
			name:    "empty string",
			hash:    "",
			wantLen: 1,
		},
		{
			name:     "multiple separators",
			hash:     "salt$hash$extra",
			wantLen:  2,
			wantSalt: "salt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := splitHashParts(tt.hash)
			if len(parts) != tt.wantLen {
				t.Errorf("splitHashParts() returned %d parts, want %d", len(parts), tt.wantLen)
			}
			if tt.wantLen == 2 && parts[0] != tt.wantSalt {
				t.Errorf("splitHashParts() salt = %q, want %q", parts[0], tt.wantSalt)
			}
		})
	}
}

func TestNullString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"empty string", "", true},
		{"non-empty string", "value", false},
		{"whitespace", " ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nullString(tt.input)
			isNil := result == nil
			if isNil != tt.want {
				t.Errorf("nullString(%q) nil = %v, want %v", tt.input, isNil, tt.want)
			}
			if !isNil && *result != tt.input {
				t.Errorf("nullString(%q) = %q, want %q", tt.input, *result, tt.input)
			}
		})
	}
}

func TestDefaultPolicy(t *testing.T) {
	orgID := "test-org"
	envID := "test-env"

	policy := defaultPolicy(orgID, envID)

	if policy.OrgID != orgID {
		t.Errorf("OrgID = %q, want %q", policy.OrgID, orgID)
	}
	if policy.EnvID != envID {
		t.Errorf("EnvID = %q, want %q", policy.EnvID, envID)
	}
	if policy.MaxTTLSeconds != 1800 {
		t.Errorf("MaxTTLSeconds = %d, want %d", policy.MaxTTLSeconds, 1800)
	}
	if policy.AllowEmptySelector {
		t.Error("AllowEmptySelector should be false by default")
	}
	if !policy.RequireReason {
		t.Error("RequireReason should be true by default")
	}
	if policy.DefaultCaps.MaxDebugEventsPerRequest != 200 {
		t.Errorf("DefaultCaps.MaxDebugEventsPerRequest = %d, want %d",
			policy.DefaultCaps.MaxDebugEventsPerRequest, 200)
	}
	if policy.DefaultCaps.MaxDebugEventsPerSession != 5000 {
		t.Errorf("DefaultCaps.MaxDebugEventsPerSession = %d, want %d",
			policy.DefaultCaps.MaxDebugEventsPerSession, 5000)
	}
}

func TestSessionToSDK(t *testing.T) {
	session := &Session{
		ID:     "sess-123",
		OrgID:  "org-456",
		EnvID:  "env-789",
		Status: "active",
		Level:  "debug",
		Labels: map[string]string{
			"ticket": "TREK-123",
		},
		ServiceScope: []string{"api-gateway"},
	}

	sdk := session.ToSDK()

	if sdk.ID != session.ID {
		t.Errorf("ToSDK().ID = %q, want %q", sdk.ID, session.ID)
	}
	if sdk.Level != session.Level {
		t.Errorf("ToSDK().Level = %q, want %q", sdk.Level, session.Level)
	}
	if len(sdk.Labels) != len(session.Labels) {
		t.Errorf("ToSDK().Labels length = %d, want %d", len(sdk.Labels), len(session.Labels))
	}
	if len(sdk.ServiceScope) != len(session.ServiceScope) {
		t.Errorf("ToSDK().ServiceScope length = %d, want %d",
			len(sdk.ServiceScope), len(session.ServiceScope))
	}
}

func TestErrNotFound(t *testing.T) {
	if ErrNotFound == nil {
		t.Error("ErrNotFound should not be nil")
	}
	if ErrNotFound.Error() != "not found" {
		t.Errorf("ErrNotFound.Error() = %q, want %q", ErrNotFound.Error(), "not found")
	}
}

func TestNewPostgresStore(t *testing.T) {
	// Test that NewPostgresStore accepts nil pool (for interface compliance)
	store := NewPostgresStore(nil)
	if store == nil {
		t.Error("NewPostgresStore() should not return nil")
	}
}

func containsSeparator(s, sep string) bool {
	for i := range s {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			return true
		}
	}
	return false
}
