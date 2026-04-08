package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bold-minds/trek/server/internal/store"
	"github.com/gofiber/fiber/v2"
)

func setupAdminTestRouter(s store.Store) *fiber.App {
	return NewRouter(s)
}

func TestListEnvironments(t *testing.T) {
	tokenHash, _ := store.HashToken("test-token")
	m := setupMockStoreWithOrgEnv("org1", "prod", tokenHash)
	app := setupAdminTestRouter(m)

	req := httptest.NewRequest("GET", "/orgs/org1/envs", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}
}

func TestCreateEnvironment(t *testing.T) {
	tokenHash, _ := store.HashToken("test-token")
	m := setupMockStoreWithOrgEnv("org1", "prod", tokenHash)
	app := setupAdminTestRouter(m)

	body := map[string]string{"name": "staging"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/orgs/org1/envs", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 201, got %d: %s", resp.StatusCode, string(respBody))
	}
}

func TestCreateEnvironmentMissingName(t *testing.T) {
	tokenHash, _ := store.HashToken("test-token")
	m := setupMockStoreWithOrgEnv("org1", "prod", tokenHash)
	app := setupAdminTestRouter(m)

	body := map[string]string{}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/orgs/org1/envs", bytes.NewReader(bodyBytes))
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

func TestListUsers(t *testing.T) {
	tokenHash, _ := store.HashToken("test-token")
	m := setupMockStoreWithOrgEnv("org1", "prod", tokenHash)
	app := setupAdminTestRouter(m)

	req := httptest.NewRequest("GET", "/orgs/org1/users", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}
}

func TestCreateUser(t *testing.T) {
	tokenHash, _ := store.HashToken("test-token")
	m := setupMockStoreWithOrgEnv("org1", "prod", tokenHash)
	app := setupAdminTestRouter(m)

	body := map[string]string{
		"oidc_subject": "google|12345",
		"email":        "user@example.com",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/orgs/org1/users", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 201, got %d: %s", resp.StatusCode, string(respBody))
	}
}

func TestCreateUserMissingFields(t *testing.T) {
	tokenHash, _ := store.HashToken("test-token")
	m := setupMockStoreWithOrgEnv("org1", "prod", tokenHash)
	app := setupAdminTestRouter(m)

	body := map[string]string{"email": "user@example.com"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/orgs/org1/users", bytes.NewReader(bodyBytes))
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

func TestListRoles(t *testing.T) {
	tokenHash, _ := store.HashToken("test-token")
	m := setupMockStoreWithOrgEnv("org1", "prod", tokenHash)
	app := setupAdminTestRouter(m)

	req := httptest.NewRequest("GET", "/orgs/org1/roles", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}
}

func TestCreateRole(t *testing.T) {
	tokenHash, _ := store.HashToken("test-token")
	m := setupMockStoreWithOrgEnv("org1", "prod", tokenHash)
	app := setupAdminTestRouter(m)

	body := map[string]any{
		"name":        "developer",
		"permissions": []string{"session:create", "session:read"},
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/orgs/org1/roles", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 201, got %d: %s", resp.StatusCode, string(respBody))
	}
}

func TestCreateRoleMissingName(t *testing.T) {
	tokenHash, _ := store.HashToken("test-token")
	m := setupMockStoreWithOrgEnv("org1", "prod", tokenHash)
	app := setupAdminTestRouter(m)

	body := map[string]any{"permissions": []string{}}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/orgs/org1/roles", bytes.NewReader(bodyBytes))
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

func TestListNotificationConfigs(t *testing.T) {
	tokenHash, _ := store.HashToken("test-token")
	m := setupMockStoreWithOrgEnv("org1", "prod", tokenHash)
	app := setupAdminTestRouter(m)

	req := httptest.NewRequest("GET", "/orgs/org1/notifications", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}
}

func TestCreateNotificationConfig(t *testing.T) {
	tokenHash, _ := store.HashToken("test-token")
	m := setupMockStoreWithOrgEnv("org1", "prod", tokenHash)
	app := setupAdminTestRouter(m)

	body := map[string]any{
		"type":    "slack",
		"config":  map[string]string{"webhook_url": "https://hooks.slack.com/..."},
		"enabled": true,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/orgs/org1/notifications", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 201, got %d: %s", resp.StatusCode, string(respBody))
	}
}

func TestCreateNotificationConfigInvalidType(t *testing.T) {
	tokenHash, _ := store.HashToken("test-token")
	m := setupMockStoreWithOrgEnv("org1", "prod", tokenHash)
	app := setupAdminTestRouter(m)

	body := map[string]any{
		"type":    "email", // invalid
		"config":  map[string]string{},
		"enabled": true,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/orgs/org1/notifications", bytes.NewReader(bodyBytes))
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
