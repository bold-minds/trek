package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bold-minds/trek"
	"github.com/bold-minds/trek/server/internal/store"
	"github.com/gofiber/fiber/v2"
)

type mockStore struct {
	sessions    map[string]*store.Session
	policies    map[string]*store.Policy
	auditEvents []store.AuditEvent
	tokens      map[string]*store.ServiceToken
	orgs        map[string]*store.Org
	envs        map[string]*store.Env
}

func newMockStore() *mockStore {
	return &mockStore{
		sessions: make(map[string]*store.Session),
		policies: make(map[string]*store.Policy),
		tokens:   make(map[string]*store.ServiceToken),
		orgs:     make(map[string]*store.Org),
		envs:     make(map[string]*store.Env),
	}
}

func (m *mockStore) CreateSession(ctx context.Context, session *store.Session) error {
	m.sessions[session.ID] = session
	return nil
}

func (m *mockStore) GetSession(ctx context.Context, orgID, envID, sessionID string) (*store.Session, error) {
	s, ok := m.sessions[sessionID]
	if !ok {
		return nil, store.ErrNotFound
	}
	return s, nil
}

func (m *mockStore) ListSessions(ctx context.Context, orgID, envID string, filter store.SessionFilter) ([]store.Session, error) {
	var result []store.Session
	for _, s := range m.sessions {
		if s.OrgID == orgID && s.EnvID == envID {
			if filter.Status == "" || s.Status == filter.Status {
				result = append(result, *s)
			}
		}
	}
	return result, nil
}

func (m *mockStore) GetActiveSessions(ctx context.Context, orgID, envID, serviceName string) ([]trek.Session, error) {
	var result []trek.Session
	now := time.Now()
	for _, s := range m.sessions {
		if s.OrgID == orgID && s.EnvID == envID && s.Status == "active" && s.ExpiresAt.After(now) {
			result = append(result, s.ToSDK())
		}
	}
	return result, nil
}

func (m *mockStore) RevokeSession(ctx context.Context, orgID, envID, sessionID string) error {
	s, ok := m.sessions[sessionID]
	if !ok {
		return store.ErrNotFound
	}
	s.Status = "revoked"
	now := time.Now()
	s.RevokedAt = &now
	return nil
}

func (m *mockStore) ExpireSessions(ctx context.Context) (int, error) {
	return 0, nil
}

func (m *mockStore) Ping(ctx context.Context) error {
	return nil
}

func (m *mockStore) GetPolicy(ctx context.Context, orgID, envID string) (*store.Policy, error) {
	key := orgID + "/" + envID
	if p, ok := m.policies[key]; ok {
		return p, nil
	}
	return &store.Policy{
		OrgID:              orgID,
		EnvID:              envID,
		MaxTTLSeconds:      1800,
		AllowEmptySelector: false,
		RequireReason:      false,
		DefaultCaps: trek.Caps{
			MaxDebugEventsPerRequest: 200,
			MaxDebugEventsPerSession: 5000,
		},
	}, nil
}

func (m *mockStore) SetPolicy(ctx context.Context, policy *store.Policy) error {
	key := policy.OrgID + "/" + policy.EnvID
	m.policies[key] = policy
	return nil
}

func (m *mockStore) CreateAuditEvent(ctx context.Context, event *store.AuditEvent) error {
	m.auditEvents = append(m.auditEvents, *event)
	return nil
}

func (m *mockStore) ListAuditEvents(ctx context.Context, orgID, envID string, limit int, cursor string) ([]store.AuditEvent, string, error) {
	var result []store.AuditEvent
	for _, e := range m.auditEvents {
		if e.OrgID == orgID {
			result = append(result, e)
		}
	}
	return result, "", nil
}

func (m *mockStore) ValidateToken(ctx context.Context, tokenHash string) (*store.ServiceToken, error) {
	t, ok := m.tokens[tokenHash]
	if !ok {
		return nil, store.ErrNotFound
	}
	return t, nil
}

func (m *mockStore) ValidateTokenPlain(ctx context.Context, plainToken string) (*store.ServiceToken, error) {
	// For testing, verify against stored hashes
	for _, t := range m.tokens {
		if store.VerifyToken(plainToken, t.TokenHash) {
			return t, nil
		}
	}
	return nil, store.ErrNotFound
}

func (m *mockStore) CreateToken(ctx context.Context, token *store.ServiceToken) error {
	m.tokens[token.TokenHash] = token
	return nil
}

func (m *mockStore) RevokeToken(ctx context.Context, orgID, envID, tokenID string) error {
	return nil
}

func (m *mockStore) ListTokens(ctx context.Context, orgID, envID string) ([]store.ServiceToken, error) {
	return nil, nil
}

func (m *mockStore) GetOrg(ctx context.Context, orgID string) (*store.Org, error) {
	o, ok := m.orgs[orgID]
	if !ok {
		return nil, store.ErrNotFound
	}
	return o, nil
}

func (m *mockStore) GetEnv(ctx context.Context, orgID, envID string) (*store.Env, error) {
	key := orgID + "/" + envID
	e, ok := m.envs[key]
	if !ok {
		return nil, store.ErrNotFound
	}
	return e, nil
}

func (m *mockStore) ListEnvs(ctx context.Context, orgID string) ([]store.Env, error) {
	return nil, nil
}

func (m *mockStore) CreateEnv(ctx context.Context, env *store.Env) error {
	return nil
}

func (m *mockStore) DeleteEnv(ctx context.Context, orgID, envID string) error {
	return nil
}

func (m *mockStore) ListUsers(ctx context.Context, orgID string) ([]store.User, error) {
	return nil, nil
}

func (m *mockStore) CreateUser(ctx context.Context, user *store.User) error {
	return nil
}

func (m *mockStore) DeleteUser(ctx context.Context, orgID, userID string) error {
	return nil
}

func (m *mockStore) ListRoles(ctx context.Context, orgID string) ([]store.Role, error) {
	return nil, nil
}

func (m *mockStore) CreateRole(ctx context.Context, role *store.Role) error {
	return nil
}

func (m *mockStore) AssignRole(ctx context.Context, orgID, userID, roleID, envID string) error {
	return nil
}

func (m *mockStore) RevokeRole(ctx context.Context, orgID, userID, roleID, envID string) error {
	return nil
}

func (m *mockStore) GetUserRoles(ctx context.Context, orgID, userID string) ([]store.UserRole, error) {
	return nil, nil
}

func (m *mockStore) GetUserPermissions(ctx context.Context, orgID, userID, envID string) ([]string, error) {
	return nil, nil
}

func (m *mockStore) ListNotificationConfigs(ctx context.Context, orgID, envID string) ([]store.NotificationConfig, error) {
	return nil, nil
}

func (m *mockStore) CreateNotificationConfig(ctx context.Context, config *store.NotificationConfig) error {
	return nil
}

func (m *mockStore) UpdateNotificationConfig(ctx context.Context, orgID, configID string, config map[string]any, enabled *bool) error {
	return nil
}

func (m *mockStore) DeleteNotificationConfig(ctx context.Context, orgID, configID string) error {
	return nil
}

func setupTestRouter(s store.Store) *fiber.App {
	return NewRouter(s)
}

func setupMockStoreWithOrgEnv(orgID, envID, tokenHash string) *mockStore {
	m := newMockStore()
	m.orgs[orgID] = &store.Org{ID: orgID, Name: "Test Org"}
	m.envs[orgID+"/"+envID] = &store.Env{ID: envID, OrgID: orgID, Name: "test"}
	m.tokens[tokenHash] = &store.ServiceToken{
		ID:        "tok_1",
		OrgID:     orgID,
		EnvID:     envID,
		TokenHash: tokenHash,
		Name:      "test-token",
	}
	return m
}

func TestLiveness(t *testing.T) {
	m := newMockStore()
	app := setupTestRouter(m)

	req := httptest.NewRequest("GET", "/healthz", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]string
	json.Unmarshal(body, &result)

	if result["status"] != "ok" {
		t.Errorf("expected status ok, got %s", result["status"])
	}
}

func TestCreateSession(t *testing.T) {
	tokenHash, _ := store.HashToken("test-token")
	m := setupMockStoreWithOrgEnv("org1", "prod", tokenHash)
	app := setupTestRouter(m)

	body := trek.CreateSessionRequest{
		Selector:   trek.Selector{UserID: "u123"},
		Level:      trek.LevelDebug,
		TTLSeconds: 600,
		Reason:     "testing",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/orgs/org1/envs/prod/sessions", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 201, got %d: %s", resp.StatusCode, string(body))
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result trek.CreateSessionResponse
	json.Unmarshal(respBody, &result)

	if result.ID == "" {
		t.Error("expected session ID")
	}
	if result.Status != "active" {
		t.Errorf("expected active status, got %s", result.Status)
	}

	if len(m.sessions) != 1 {
		t.Errorf("expected 1 session in store, got %d", len(m.sessions))
	}

	if len(m.auditEvents) != 1 {
		t.Errorf("expected 1 audit event, got %d", len(m.auditEvents))
	}
}

func TestCreateSessionEmptySelectorRejected(t *testing.T) {
	tokenHash, _ := store.HashToken("test-token")
	m := setupMockStoreWithOrgEnv("org1", "prod", tokenHash)
	app := setupTestRouter(m)

	body := trek.CreateSessionRequest{
		Selector:   trek.Selector{},
		Level:      trek.LevelDebug,
		TTLSeconds: 600,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/orgs/org1/envs/prod/sessions", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		respBody, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 400, got %d: %s", resp.StatusCode, string(respBody))
	}
}

func TestGetActiveSessions(t *testing.T) {
	tokenHash, _ := store.HashToken("test-token")
	m := setupMockStoreWithOrgEnv("org1", "prod", tokenHash)
	m.sessions["sess_1"] = &store.Session{
		ID:        "sess_1",
		OrgID:     "org1",
		EnvID:     "prod",
		Status:    "active",
		Selector:  trek.Selector{UserID: "u123"},
		Level:     trek.LevelDebug,
		ExpiresAt: time.Now().Add(10 * time.Minute),
		Labels:    map[string]string{},
	}

	app := setupTestRouter(m)

	req := httptest.NewRequest("GET", "/orgs/org1/envs/prod/active-sessions", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result trek.ActiveSessionsResponse
	json.Unmarshal(respBody, &result)

	if len(result.Sessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(result.Sessions))
	}
	if result.Revision == "" {
		t.Error("expected revision")
	}
}

func TestRevokeSession(t *testing.T) {
	tokenHash, _ := store.HashToken("test-token")
	m := setupMockStoreWithOrgEnv("org1", "prod", tokenHash)
	m.sessions["sess_1"] = &store.Session{
		ID:        "sess_1",
		OrgID:     "org1",
		EnvID:     "prod",
		Status:    "active",
		Selector:  trek.Selector{UserID: "u123"},
		Level:     trek.LevelDebug,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}

	app := setupTestRouter(m)

	req := httptest.NewRequest("POST", "/orgs/org1/envs/prod/sessions/sess_1/revoke", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}

	if resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 204, got %d: %s", resp.StatusCode, string(respBody))
	}

	if m.sessions["sess_1"].Status != "revoked" {
		t.Errorf("expected session to be revoked")
	}
}

func TestUnauthorizedWithoutToken(t *testing.T) {
	m := newMockStore()
	app := setupTestRouter(m)

	req := httptest.NewRequest("GET", "/orgs/org1/envs/prod/active-sessions", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestUnauthorizedWithInvalidToken(t *testing.T) {
	m := newMockStore()
	m.orgs["org1"] = &store.Org{ID: "org1", Name: "Test Org"}
	m.envs["org1/prod"] = &store.Env{ID: "prod", OrgID: "org1", Name: "prod"}
	app := setupTestRouter(m)

	req := httptest.NewRequest("GET", "/orgs/org1/envs/prod/active-sessions", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestCreateSessionTTLExceedsMax(t *testing.T) {
	tokenHash, _ := store.HashToken("test-token")
	m := setupMockStoreWithOrgEnv("org1", "prod", tokenHash)
	// Set a policy with max TTL of 600 seconds
	m.policies["org1/prod"] = &store.Policy{
		OrgID:              "org1",
		EnvID:              "prod",
		MaxTTLSeconds:      600,
		AllowEmptySelector: false,
		RequireReason:      false,
	}
	app := setupTestRouter(m)

	body := trek.CreateSessionRequest{
		Selector:   trek.Selector{UserID: "u123"},
		Level:      trek.LevelDebug,
		TTLSeconds: 3600, // Exceeds max
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/orgs/org1/envs/prod/sessions", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		respBody, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 400 for TTL exceeding max, got %d: %s", resp.StatusCode, string(respBody))
	}
}

func TestRevokeNonExistentSession(t *testing.T) {
	tokenHash, _ := store.HashToken("test-token")
	m := setupMockStoreWithOrgEnv("org1", "prod", tokenHash)
	app := setupTestRouter(m)

	req := httptest.NewRequest("POST", "/orgs/org1/envs/prod/sessions/nonexistent/revoke", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}

	if resp.StatusCode != http.StatusNotFound {
		respBody, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 404 for non-existent session, got %d: %s", resp.StatusCode, string(respBody))
	}
}

func TestCreateSessionRequireReason(t *testing.T) {
	tokenHash, _ := store.HashToken("test-token")
	m := setupMockStoreWithOrgEnv("org1", "prod", tokenHash)
	m.policies["org1/prod"] = &store.Policy{
		OrgID:              "org1",
		EnvID:              "prod",
		MaxTTLSeconds:      1800,
		AllowEmptySelector: false,
		RequireReason:      true,
	}
	app := setupTestRouter(m)

	body := trek.CreateSessionRequest{
		Selector:   trek.Selector{UserID: "u123"},
		Level:      trek.LevelDebug,
		TTLSeconds: 600,
		// No reason provided
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/orgs/org1/envs/prod/sessions", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		respBody, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 400 for missing reason, got %d: %s", resp.StatusCode, string(respBody))
	}
}

func TestUpdatePolicyInvalidTTL(t *testing.T) {
	tokenHash, _ := store.HashToken("test-token")
	m := setupMockStoreWithOrgEnv("org1", "prod", tokenHash)
	app := setupTestRouter(m)

	body := map[string]any{
		"max_ttl_seconds":      0, // Invalid
		"allow_empty_selector": false,
		"require_reason":       true,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/orgs/org1/envs/prod/policies", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		respBody, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 400 for invalid TTL, got %d: %s", resp.StatusCode, string(respBody))
	}
}

func TestOrgNotFound(t *testing.T) {
	tokenHash, _ := store.HashToken("test-token")
	m := newMockStore()
	// Don't add org, only add env and token
	m.tokens[tokenHash] = &store.ServiceToken{
		ID:        "tok_1",
		OrgID:     "org1",
		EnvID:     "prod",
		TokenHash: tokenHash,
		Name:      "test-token",
	}
	app := setupTestRouter(m)

	req := httptest.NewRequest("GET", "/orgs/org1/envs/prod/active-sessions", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent org, got %d", resp.StatusCode)
	}
}

func TestEnvNotFound(t *testing.T) {
	tokenHash, _ := store.HashToken("test-token")
	m := newMockStore()
	m.orgs["org1"] = &store.Org{ID: "org1", Name: "Test Org"}
	// Don't add env
	m.tokens[tokenHash] = &store.ServiceToken{
		ID:        "tok_1",
		OrgID:     "org1",
		EnvID:     "prod",
		TokenHash: tokenHash,
		Name:      "test-token",
	}
	app := setupTestRouter(m)

	req := httptest.NewRequest("GET", "/orgs/org1/envs/prod/active-sessions", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent env, got %d", resp.StatusCode)
	}
}

func TestTokenHashVerification(t *testing.T) {
	// Test legacy unsalted hash
	token := "test-token-123"
	legacyHash, err := store.HashToken(token)
	if err != nil {
		t.Fatalf("failed to hash token: %v", err)
	}

	if !store.VerifyToken(token, legacyHash) {
		t.Error("expected legacy hash to verify")
	}

	if store.VerifyToken("wrong-token", legacyHash) {
		t.Error("expected wrong token to not verify")
	}
}
