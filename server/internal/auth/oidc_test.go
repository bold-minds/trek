package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewOIDCProvider(t *testing.T) {
	t.Run("returns error for empty issuer URL", func(t *testing.T) {
		_, err := NewOIDCProvider(OIDCConfig{
			IssuerURL: "",
		})
		if err == nil {
			t.Error("expected error for empty issuer URL")
		}
	})

	t.Run("returns error when discovery fetch fails", func(t *testing.T) {
		_, err := NewOIDCProvider(OIDCConfig{
			IssuerURL: "http://invalid-url-that-does-not-exist.example.com",
		})
		if err == nil {
			t.Error("expected error for invalid issuer URL")
		}
	})

	t.Run("creates provider with valid discovery document", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/.well-known/openid-configuration" {
				json.NewEncoder(w).Encode(DiscoveryDocument{
					Issuer:                "https://auth.example.com",
					AuthorizationEndpoint: "https://auth.example.com/authorize",
					TokenEndpoint:         "https://auth.example.com/token",
					UserinfoEndpoint:      "https://auth.example.com/userinfo",
					JwksURI:               "https://auth.example.com/.well-known/jwks.json",
				})
				return
			}
			http.NotFound(w, r)
		}))
		defer server.Close()

		provider, err := NewOIDCProvider(OIDCConfig{
			IssuerURL:    server.URL,
			ClientID:     "test-client",
			ClientSecret: "test-secret",
			RedirectURL:  "http://localhost/callback",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if provider == nil {
			t.Fatal("provider is nil")
		}
	})
}

func TestOIDCProvider_GetAuthorizationURL(t *testing.T) {
	provider := &OIDCProvider{
		config: OIDCConfig{
			ClientID:    "test-client",
			RedirectURL: "http://localhost/callback",
			Scopes:      []string{"openid", "email"},
		},
		discoveryDoc: &DiscoveryDocument{
			AuthorizationEndpoint: "https://auth.example.com/authorize",
		},
	}

	t.Run("returns error when discovery doc is nil", func(t *testing.T) {
		nilDocProvider := &OIDCProvider{config: OIDCConfig{}}
		_, err := nilDocProvider.GetAuthorizationURL("state", "nonce")
		if err == nil {
			t.Error("expected error when discovery doc is nil")
		}
	})

	t.Run("constructs authorization URL with provided state and nonce", func(t *testing.T) {
		url, err := provider.GetAuthorizationURL("test-state", "test-nonce")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if url == "" {
			t.Error("URL is empty")
		}

		expectedPrefix := "https://auth.example.com/authorize?"
		if len(url) < len(expectedPrefix) || url[:len(expectedPrefix)] != expectedPrefix {
			t.Errorf("URL = %q, expected prefix %q", url, expectedPrefix)
		}

		if !contains(url, "client_id=test-client") {
			t.Errorf("URL missing client_id")
		}
		if !contains(url, "state=test-state") {
			t.Errorf("URL missing state")
		}
		if !contains(url, "nonce=test-nonce") {
			t.Errorf("URL missing nonce")
		}
	})

	t.Run("uses default scopes when none configured", func(t *testing.T) {
		noScopesProvider := &OIDCProvider{
			config: OIDCConfig{
				ClientID:    "test-client",
				RedirectURL: "http://localhost/callback",
			},
			discoveryDoc: &DiscoveryDocument{
				AuthorizationEndpoint: "https://auth.example.com/authorize",
			},
		}

		url, err := noScopesProvider.GetAuthorizationURL("state", "nonce")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !contains(url, "scope=openid") {
			t.Errorf("URL missing default openid scope")
		}
	})
}

func TestOIDCProvider_ExchangeCode(t *testing.T) {
	t.Run("returns error when discovery doc is nil", func(t *testing.T) {
		provider := &OIDCProvider{config: OIDCConfig{}}
		_, err := provider.ExchangeCode(nil, "code")
		if err == nil {
			t.Error("expected error when discovery doc is nil")
		}
	})

	t.Run("exchanges code successfully", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/token" && r.Method == http.MethodPost {
				json.NewEncoder(w).Encode(TokenResponse{
					AccessToken:  "access-token-123",
					TokenType:    "Bearer",
					ExpiresIn:    3600,
					RefreshToken: "refresh-token-456",
					IDToken:      "id-token-789",
				})
				return
			}
			http.NotFound(w, r)
		}))
		defer server.Close()

		provider := &OIDCProvider{
			config: OIDCConfig{
				ClientID:     "test-client",
				ClientSecret: "test-secret",
				RedirectURL:  "http://localhost/callback",
			},
			httpClient: http.DefaultClient,
			discoveryDoc: &DiscoveryDocument{
				TokenEndpoint: server.URL + "/token",
			},
		}

		resp, err := provider.ExchangeCode(context.Background(), "auth-code")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.AccessToken != "access-token-123" {
			t.Errorf("AccessToken = %q, want %q", resp.AccessToken, "access-token-123")
		}
		if resp.RefreshToken != "refresh-token-456" {
			t.Errorf("RefreshToken = %q, want %q", resp.RefreshToken, "refresh-token-456")
		}
	})

	t.Run("returns error on non-200 response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		provider := &OIDCProvider{
			config:     OIDCConfig{},
			httpClient: http.DefaultClient,
			discoveryDoc: &DiscoveryDocument{
				TokenEndpoint: server.URL + "/token",
			},
		}

		_, err := provider.ExchangeCode(context.Background(), "invalid-code")
		if err == nil {
			t.Error("expected error for non-200 response")
		}
	})
}

func TestOIDCProvider_GetUserInfo(t *testing.T) {
	t.Run("returns error when discovery doc is nil", func(t *testing.T) {
		provider := &OIDCProvider{config: OIDCConfig{}}
		_, err := provider.GetUserInfo(context.Background(), "token")
		if err == nil {
			t.Error("expected error when discovery doc is nil")
		}
	})

	t.Run("fetches user info successfully", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/userinfo" {
				authHeader := r.Header.Get("Authorization")
				if authHeader != "Bearer valid-token" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				json.NewEncoder(w).Encode(UserInfo{
					Subject:       "user-123",
					Email:         "user@example.com",
					EmailVerified: true,
					Name:          "Test User",
				})
				return
			}
			http.NotFound(w, r)
		}))
		defer server.Close()

		provider := &OIDCProvider{
			config:     OIDCConfig{},
			httpClient: http.DefaultClient,
			discoveryDoc: &DiscoveryDocument{
				UserinfoEndpoint: server.URL + "/userinfo",
			},
		}

		userInfo, err := provider.GetUserInfo(context.Background(), "valid-token")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if userInfo.Subject != "user-123" {
			t.Errorf("Subject = %q, want %q", userInfo.Subject, "user-123")
		}
		if userInfo.Email != "user@example.com" {
			t.Errorf("Email = %q, want %q", userInfo.Email, "user@example.com")
		}
	})
}

func TestOIDCProvider_ValidateIDTokenNonce(t *testing.T) {
	makeToken := func(claims IDTokenClaims) string {
		header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
		payload, _ := json.Marshal(claims)
		payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
		signature := base64.RawURLEncoding.EncodeToString([]byte("fake-signature"))
		return header + "." + payloadB64 + "." + signature
	}

	provider := &OIDCProvider{}

	t.Run("returns error for invalid token format", func(t *testing.T) {
		err := provider.ValidateIDTokenNonce("invalid-token", "nonce")
		if err == nil {
			t.Error("expected error for invalid token format")
		}
	})

	t.Run("returns error for token with wrong number of parts", func(t *testing.T) {
		err := provider.ValidateIDTokenNonce("only.two", "nonce")
		if err == nil {
			t.Error("expected error for token with 2 parts")
		}
	})

	t.Run("returns error for nonce mismatch", func(t *testing.T) {
		token := makeToken(IDTokenClaims{Nonce: "actual-nonce"})
		err := provider.ValidateIDTokenNonce(token, "expected-nonce")
		if err == nil {
			t.Error("expected error for nonce mismatch")
		}
	})

	t.Run("succeeds when nonce matches", func(t *testing.T) {
		token := makeToken(IDTokenClaims{Nonce: "correct-nonce"})
		err := provider.ValidateIDTokenNonce(token, "correct-nonce")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("returns error for invalid base64 payload", func(t *testing.T) {
		invalidToken := "header.!!!invalid-base64!!!.signature"
		err := provider.ValidateIDTokenNonce(invalidToken, "nonce")
		if err == nil {
			t.Error("expected error for invalid base64")
		}
	})
}

func TestOIDCProvider_ValidateEmail(t *testing.T) {
	tests := []struct {
		name          string
		allowedEmails []string
		allowedDomain string
		email         string
		want          bool
	}{
		{
			name:  "allows any email when no restrictions",
			email: "anyone@anywhere.com",
			want:  true,
		},
		{
			name:          "allows email in allowed list",
			allowedEmails: []string{"user@example.com", "admin@example.com"},
			email:         "user@example.com",
			want:          true,
		},
		{
			name:          "allows email case-insensitively",
			allowedEmails: []string{"User@Example.com"},
			email:         "user@example.com",
			want:          true,
		},
		{
			name:          "rejects email not in allowed list",
			allowedEmails: []string{"user@example.com"},
			email:         "other@example.com",
			want:          false,
		},
		{
			name:          "allows email with matching domain",
			allowedDomain: "example.com",
			email:         "anyone@example.com",
			want:          true,
		},
		{
			name:          "allows domain case-insensitively",
			allowedDomain: "Example.COM",
			email:         "user@example.com",
			want:          true,
		},
		{
			name:          "rejects email with non-matching domain",
			allowedDomain: "example.com",
			email:         "user@other.com",
			want:          false,
		},
		{
			name:          "rejects invalid email format for domain check",
			allowedDomain: "example.com",
			email:         "invalid-email",
			want:          false,
		},
		{
			name:          "allowed emails takes precedence over domain",
			allowedEmails: []string{"special@other.com"},
			allowedDomain: "example.com",
			email:         "user@example.com",
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &OIDCProvider{
				config: OIDCConfig{
					AllowedEmails: tt.allowedEmails,
					AllowedDomain: tt.allowedDomain,
				},
			}

			got := provider.ValidateEmail(tt.email)
			if got != tt.want {
				t.Errorf("ValidateEmail(%q) = %v, want %v", tt.email, got, tt.want)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
