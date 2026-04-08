package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/bold-minds/trek/server/internal/store"
)

// AuthHandlers provides HTTP handlers for authentication.
type AuthHandlers struct {
	provider *OIDCProvider
	store    store.Store
	sessions *SessionStore
}

// SessionStore manages user sessions.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*UserSession
}

// UserSession represents an authenticated user session.
type UserSession struct {
	ID          string
	UserID      string
	Email       string
	Name        string
	OrgID       string
	AccessToken string
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

// NewAuthHandlers creates new auth handlers.
func NewAuthHandlers(provider *OIDCProvider, s store.Store) *AuthHandlers {
	return &AuthHandlers{
		provider: provider,
		store:    s,
		sessions: &SessionStore{sessions: make(map[string]*UserSession)},
	}
}

// HandleLogin initiates the OIDC login flow.
func (h *AuthHandlers) HandleLogin(w http.ResponseWriter, r *http.Request) {
	state := generateSecureToken(32)
	nonce := generateSecureToken(32)

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_nonce",
		Value:    nonce,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})

	authURL, err := h.provider.GetAuthorizationURL(state, nonce)
	if err != nil {
		http.Error(w, "Failed to generate auth URL", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, authURL, http.StatusFound)
}

// HandleCallback handles the OIDC callback.
func (h *AuthHandlers) HandleCallback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil {
		http.Error(w, "Missing state cookie", http.StatusBadRequest)
		return
	}

	nonceCookie, err := r.Cookie("oauth_nonce")
	if err != nil {
		http.Error(w, "Missing nonce cookie", http.StatusBadRequest)
		return
	}

	if r.URL.Query().Get("state") != stateCookie.Value {
		http.Error(w, "State mismatch", http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Missing code", http.StatusBadRequest)
		return
	}

	tokenResp, err := h.provider.ExchangeCode(r.Context(), code)
	if err != nil {
		http.Error(w, "Token exchange failed", http.StatusInternalServerError)
		return
	}

	// Validate nonce from ID token if present
	if tokenResp.IDToken != "" {
		if err := h.provider.ValidateIDTokenNonce(tokenResp.IDToken, nonceCookie.Value); err != nil {
			http.Error(w, "Nonce validation failed", http.StatusBadRequest)
			return
		}
	}

	userInfo, err := h.provider.GetUserInfo(r.Context(), tokenResp.AccessToken)
	if err != nil {
		http.Error(w, "Failed to get user info", http.StatusInternalServerError)
		return
	}

	if !h.provider.ValidateEmail(userInfo.Email) {
		http.Error(w, "Email not allowed", http.StatusForbidden)
		return
	}

	sessionID := generateSecureToken(32)
	session := &UserSession{
		ID:          sessionID,
		UserID:      userInfo.Subject,
		Email:       userInfo.Email,
		Name:        userInfo.Name,
		AccessToken: tokenResp.AccessToken,
		ExpiresAt:   time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		CreatedAt:   time.Now(),
	}

	h.sessions.mu.Lock()
	h.sessions.sessions[sessionID] = session
	h.sessions.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   tokenResp.ExpiresIn,
	})

	http.SetCookie(w, &http.Cookie{
		Name:   "oauth_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name:   "oauth_nonce",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

// HandleLogout logs the user out.
func (h *AuthHandlers) HandleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_id")
	if err == nil {
		h.sessions.mu.Lock()
		delete(h.sessions.sessions, cookie.Value)
		h.sessions.mu.Unlock()
	}

	http.SetCookie(w, &http.Cookie{
		Name:   "session_id",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

// HandleMe returns the current user info.
func (h *AuthHandlers) HandleMe(w http.ResponseWriter, r *http.Request) {
	session := h.GetSession(r)
	if session == nil {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"user_id": session.UserID,
		"email":   session.Email,
		"name":    session.Name,
		"org_id":  session.OrgID,
	})
}

// GetSession returns the current session from the request.
func (h *AuthHandlers) GetSession(r *http.Request) *UserSession {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return nil
	}

	h.sessions.mu.RLock()
	session, ok := h.sessions.sessions[cookie.Value]
	h.sessions.mu.RUnlock()

	if !ok {
		return nil
	}

	if time.Now().After(session.ExpiresAt) {
		h.sessions.mu.Lock()
		delete(h.sessions.sessions, cookie.Value)
		h.sessions.mu.Unlock()
		return nil
	}

	return session
}

// RequireAuth is middleware that requires authentication.
func (h *AuthHandlers) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session := h.GetSession(r)
		if session == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func generateSecureToken(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}
