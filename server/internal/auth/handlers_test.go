package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bold-minds/trek/server/internal/store"
)

type mockStoreForHandlers struct {
	store.Store
}

func TestNewAuthHandlers(t *testing.T) {
	provider := &OIDCProvider{}
	mockStore := &mockStoreForHandlers{}

	handlers := NewAuthHandlers(provider, mockStore)

	if handlers == nil {
		t.Fatal("NewAuthHandlers returned nil")
	}
	if handlers.provider != provider {
		t.Error("provider not set correctly")
	}
	if handlers.store != mockStore {
		t.Error("store not set correctly")
	}
	if handlers.sessions == nil {
		t.Error("sessions not initialized")
	}
}

func TestAuthHandlers_HandleLogin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/openid-configuration" {
			json.NewEncoder(w).Encode(DiscoveryDocument{
				Issuer:                "https://auth.example.com",
				AuthorizationEndpoint: "https://auth.example.com/authorize",
				TokenEndpoint:         "https://auth.example.com/token",
				UserinfoEndpoint:      "https://auth.example.com/userinfo",
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
		t.Fatalf("failed to create provider: %v", err)
	}

	handlers := NewAuthHandlers(provider, &mockStoreForHandlers{})

	t.Run("redirects to authorization URL", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/login", nil)
		rec := httptest.NewRecorder()

		handlers.HandleLogin(rec, req)

		if rec.Code != http.StatusFound {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusFound)
		}

		location := rec.Header().Get("Location")
		if location == "" {
			t.Error("missing Location header")
		}

		cookies := rec.Result().Cookies()
		var hasStateCookie, hasNonceCookie bool
		for _, c := range cookies {
			if c.Name == "oauth_state" {
				hasStateCookie = true
				if c.Value == "" {
					t.Error("state cookie is empty")
				}
				if !c.HttpOnly {
					t.Error("state cookie should be HttpOnly")
				}
			}
			if c.Name == "oauth_nonce" {
				hasNonceCookie = true
				if c.Value == "" {
					t.Error("nonce cookie is empty")
				}
			}
		}

		if !hasStateCookie {
			t.Error("missing state cookie")
		}
		if !hasNonceCookie {
			t.Error("missing nonce cookie")
		}
	})
}

func TestAuthHandlers_HandleCallback(t *testing.T) {
	t.Run("returns error when state cookie missing", func(t *testing.T) {
		handlers := NewAuthHandlers(&OIDCProvider{}, &mockStoreForHandlers{})

		req := httptest.NewRequest("GET", "/callback?code=abc&state=xyz", nil)
		rec := httptest.NewRecorder()

		handlers.HandleCallback(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("returns error when nonce cookie missing", func(t *testing.T) {
		handlers := NewAuthHandlers(&OIDCProvider{}, &mockStoreForHandlers{})

		req := httptest.NewRequest("GET", "/callback?code=abc&state=xyz", nil)
		req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "xyz"})
		rec := httptest.NewRecorder()

		handlers.HandleCallback(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("returns error when state mismatch", func(t *testing.T) {
		handlers := NewAuthHandlers(&OIDCProvider{}, &mockStoreForHandlers{})

		req := httptest.NewRequest("GET", "/callback?code=abc&state=wrong", nil)
		req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "expected"})
		req.AddCookie(&http.Cookie{Name: "oauth_nonce", Value: "nonce"})
		rec := httptest.NewRecorder()

		handlers.HandleCallback(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("returns error when code missing", func(t *testing.T) {
		handlers := NewAuthHandlers(&OIDCProvider{}, &mockStoreForHandlers{})

		req := httptest.NewRequest("GET", "/callback?state=xyz", nil)
		req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "xyz"})
		req.AddCookie(&http.Cookie{Name: "oauth_nonce", Value: "nonce"})
		rec := httptest.NewRecorder()

		handlers.HandleCallback(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})
}

func TestAuthHandlers_HandleLogout(t *testing.T) {
	handlers := NewAuthHandlers(&OIDCProvider{}, &mockStoreForHandlers{})

	handlers.sessions.mu.Lock()
	handlers.sessions.sessions["test-session"] = &UserSession{
		ID:     "test-session",
		UserID: "user-123",
		Email:  "test@example.com",
	}
	handlers.sessions.mu.Unlock()

	t.Run("clears session and redirects", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/logout", nil)
		req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
		rec := httptest.NewRecorder()

		handlers.HandleLogout(rec, req)

		if rec.Code != http.StatusFound {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusFound)
		}

		handlers.sessions.mu.RLock()
		_, exists := handlers.sessions.sessions["test-session"]
		handlers.sessions.mu.RUnlock()

		if exists {
			t.Error("session should be deleted")
		}

		var sessionCleared bool
		for _, c := range rec.Result().Cookies() {
			if c.Name == "session_id" && c.MaxAge < 0 {
				sessionCleared = true
			}
		}
		if !sessionCleared {
			t.Error("session cookie should be cleared")
		}
	})

	t.Run("handles missing session cookie gracefully", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/logout", nil)
		rec := httptest.NewRecorder()

		handlers.HandleLogout(rec, req)

		if rec.Code != http.StatusFound {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusFound)
		}
	})
}

func TestAuthHandlers_HandleMe(t *testing.T) {
	handlers := NewAuthHandlers(&OIDCProvider{}, &mockStoreForHandlers{})

	handlers.sessions.mu.Lock()
	handlers.sessions.sessions["valid-session"] = &UserSession{
		ID:        "valid-session",
		UserID:    "user-123",
		Email:     "test@example.com",
		Name:      "Test User",
		OrgID:     "org-1",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	handlers.sessions.mu.Unlock()

	t.Run("returns user info for authenticated user", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/me", nil)
		req.AddCookie(&http.Cookie{Name: "session_id", Value: "valid-session"})
		rec := httptest.NewRecorder()

		handlers.HandleMe(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		var response map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["user_id"] != "user-123" {
			t.Errorf("user_id = %v, want %v", response["user_id"], "user-123")
		}
		if response["email"] != "test@example.com" {
			t.Errorf("email = %v, want %v", response["email"], "test@example.com")
		}
	})

	t.Run("returns unauthorized for unauthenticated user", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/me", nil)
		rec := httptest.NewRecorder()

		handlers.HandleMe(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})
}

func TestAuthHandlers_GetSession(t *testing.T) {
	handlers := NewAuthHandlers(&OIDCProvider{}, &mockStoreForHandlers{})

	handlers.sessions.mu.Lock()
	handlers.sessions.sessions["valid-session"] = &UserSession{
		ID:        "valid-session",
		UserID:    "user-123",
		Email:     "test@example.com",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	handlers.sessions.sessions["expired-session"] = &UserSession{
		ID:        "expired-session",
		UserID:    "user-456",
		Email:     "expired@example.com",
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	handlers.sessions.mu.Unlock()

	t.Run("returns session for valid cookie", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "session_id", Value: "valid-session"})

		session := handlers.GetSession(req)
		if session == nil {
			t.Fatal("expected session, got nil")
		}
		if session.ID != "valid-session" {
			t.Errorf("session.ID = %q, want %q", session.ID, "valid-session")
		}
	})

	t.Run("returns nil for missing cookie", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)

		session := handlers.GetSession(req)
		if session != nil {
			t.Errorf("expected nil, got %v", session)
		}
	})

	t.Run("returns nil for unknown session", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "session_id", Value: "unknown-session"})

		session := handlers.GetSession(req)
		if session != nil {
			t.Errorf("expected nil, got %v", session)
		}
	})

	t.Run("returns nil and cleans up expired session", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "session_id", Value: "expired-session"})

		session := handlers.GetSession(req)
		if session != nil {
			t.Errorf("expected nil for expired session, got %v", session)
		}

		handlers.sessions.mu.RLock()
		_, exists := handlers.sessions.sessions["expired-session"]
		handlers.sessions.mu.RUnlock()

		if exists {
			t.Error("expired session should be deleted")
		}
	})
}

func TestAuthHandlers_RequireAuth(t *testing.T) {
	handlers := NewAuthHandlers(&OIDCProvider{}, &mockStoreForHandlers{})

	handlers.sessions.mu.Lock()
	handlers.sessions.sessions["valid-session"] = &UserSession{
		ID:        "valid-session",
		UserID:    "user-123",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	handlers.sessions.mu.Unlock()

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	t.Run("allows authenticated request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		req.AddCookie(&http.Cookie{Name: "session_id", Value: "valid-session"})
		rec := httptest.NewRecorder()

		handlers.RequireAuth(nextHandler).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("blocks unauthenticated request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		rec := httptest.NewRecorder()

		handlers.RequireAuth(nextHandler).ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})
}

func TestGenerateSecureToken(t *testing.T) {
	t.Run("generates non-empty token", func(t *testing.T) {
		token := generateSecureToken(32)
		if token == "" {
			t.Error("token is empty")
		}
	})

	t.Run("generates different tokens on each call", func(t *testing.T) {
		token1 := generateSecureToken(32)
		token2 := generateSecureToken(32)
		if token1 == token2 {
			t.Error("tokens should be different")
		}
	})

	t.Run("generates tokens of consistent length", func(t *testing.T) {
		token1 := generateSecureToken(16)
		token2 := generateSecureToken(16)
		if len(token1) != len(token2) {
			t.Errorf("token lengths differ: %d vs %d", len(token1), len(token2))
		}
	})
}

func TestSessionStore(t *testing.T) {
	t.Run("concurrent access is safe", func(t *testing.T) {
		store := &SessionStore{sessions: make(map[string]*UserSession)}

		done := make(chan bool)

		go func() {
			for i := 0; i < 100; i++ {
				store.mu.Lock()
				store.sessions["key"] = &UserSession{ID: "test"}
				store.mu.Unlock()
			}
			done <- true
		}()

		go func() {
			for i := 0; i < 100; i++ {
				store.mu.RLock()
				_ = store.sessions["key"]
				store.mu.RUnlock()
			}
			done <- true
		}()

		<-done
		<-done
	})
}

func TestUserSession(t *testing.T) {
	session := &UserSession{
		ID:          "sess-1",
		UserID:      "user-1",
		Email:       "test@example.com",
		Name:        "Test User",
		OrgID:       "org-1",
		AccessToken: "token-123",
		ExpiresAt:   time.Now().Add(time.Hour),
		CreatedAt:   time.Now(),
	}

	if session.ID == "" {
		t.Error("ID should not be empty")
	}
	if session.UserID == "" {
		t.Error("UserID should not be empty")
	}
	if session.ExpiresAt.IsZero() {
		t.Error("ExpiresAt should be set")
	}
}
