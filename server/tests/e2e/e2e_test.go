package e2e

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/bold-minds/trek"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	serverURL     string
	dbPool        *pgxpool.Pool
	testOrgID     = "org_e2e_test"
	testEnvID     = "env_e2e_test"
	testToken     = "e2e-test-token"
	containerID   string
	containerPort = "5433"
)

func TestMain(m *testing.M) {
	serverURL = os.Getenv("TREK_SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}

	dbURL := os.Getenv("DATABASE_URL")
	useDockerDB := dbURL == ""

	if useDockerDB {
		var err error
		containerID, err = startPostgresContainer()
		if err != nil {
			fmt.Printf("Failed to start postgres container: %v\n", err)
			os.Exit(1)
		}
		defer stopPostgresContainer(containerID)

		dbURL = fmt.Sprintf("postgres://trek:trek@localhost:%s/trek?sslmode=disable", containerPort)

		if err := waitForPostgres(dbURL, 30*time.Second); err != nil {
			fmt.Printf("Postgres failed to become ready: %v\n", err)
			os.Exit(1)
		}
	}

	var err error
	dbPool, err = pgxpool.New(context.Background(), dbURL)
	if err != nil {
		fmt.Printf("Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer dbPool.Close()

	if useDockerDB {
		if err := runMigrations(dbURL); err != nil {
			fmt.Printf("Failed to run migrations: %v\n", err)
			os.Exit(1)
		}
	}

	if err := setupTestData(); err != nil {
		fmt.Printf("Failed to setup test data: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	cleanupTestData()

	os.Exit(code)
}

func startPostgresContainer() (string, error) {
	cmd := exec.Command("docker", "run", "-d",
		"--name", "trek-e2e-postgres",
		"-e", "POSTGRES_DB=trek",
		"-e", "POSTGRES_USER=trek",
		"-e", "POSTGRES_PASSWORD=trek",
		"-p", containerPort+":5432",
		"postgres:16-alpine",
	)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("docker run failed: %s", string(exitErr.Stderr))
		}
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func stopPostgresContainer(id string) {
	exec.Command("docker", "stop", id).Run()
	exec.Command("docker", "rm", id).Run()
}

func waitForPostgres(dbURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pool, err := pgxpool.New(context.Background(), dbURL)
		if err == nil {
			pool.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("postgres not ready after %v", timeout)
}

func runMigrations(dbURL string) error {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	migrationSQL, err := os.ReadFile("../../db/migrations/001_initial.up.sql")
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	_, err = pool.Exec(ctx, string(migrationSQL))
	if err != nil {
		return fmt.Errorf("failed to run migration: %w", err)
	}

	return nil
}

func setupTestData() error {
	ctx := context.Background()

	_, err := dbPool.Exec(ctx, `
		INSERT INTO orgs (id, name, created_at) VALUES ($1, 'E2E Test Org', NOW())
		ON CONFLICT (id) DO NOTHING
	`, testOrgID)
	if err != nil {
		return err
	}

	_, err = dbPool.Exec(ctx, `
		INSERT INTO envs (id, org_id, name, created_at) VALUES ($1, $2, 'e2e', NOW())
		ON CONFLICT (id) DO NOTHING
	`, testEnvID, testOrgID)
	if err != nil {
		return err
	}

	tokenHash := hashToken(testToken)
	_, err = dbPool.Exec(ctx, `
		INSERT INTO service_tokens (id, org_id, env_id, token_hash, name, created_at) 
		VALUES ('tok_e2e', $1, $2, $3, 'E2E Test Token', NOW())
		ON CONFLICT (id) DO NOTHING
	`, testOrgID, testEnvID, tokenHash)
	if err != nil {
		return err
	}

	_, err = dbPool.Exec(ctx, `
		INSERT INTO policies (id, org_id, env_id, max_ttl_seconds, allow_empty_selector, require_reason, created_at, updated_at)
		VALUES ('pol_e2e', $1, $2, 3600, false, false, NOW(), NOW())
		ON CONFLICT (org_id, env_id) DO NOTHING
	`, testOrgID, testEnvID)

	return err
}

func cleanupTestData() {
	ctx := context.Background()
	dbPool.Exec(ctx, `DELETE FROM sessions WHERE org_id = $1`, testOrgID)
	dbPool.Exec(ctx, `DELETE FROM audit_events WHERE org_id = $1`, testOrgID)
}

func hashToken(token string) string {
	// Use the actual store.HashToken implementation for consistency
	// This is a legacy unsalted hash for backward compatibility in tests
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func TestE2ECreateAndListSession(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Create session
	reqBody := trek.CreateSessionRequest{
		Selector:   trek.Selector{UserID: "e2e_user_123"},
		Level:      trek.LevelDebug,
		TTLSeconds: 600,
		Reason:     "E2E test",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST",
		fmt.Sprintf("%s/orgs/%s/envs/%s/sessions", serverURL, testOrgID, testEnvID),
		bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Skipf("Server not available: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var createResp trek.CreateSessionResponse
	json.NewDecoder(resp.Body).Decode(&createResp)

	if createResp.ID == "" {
		t.Fatal("expected session ID")
	}

	// List active sessions
	req, _ = http.NewRequest("GET",
		fmt.Sprintf("%s/orgs/%s/envs/%s/active-sessions", serverURL, testOrgID, testEnvID),
		nil)
	req.Header.Set("Authorization", "Bearer "+testToken)

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("list request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var listResp trek.ActiveSessionsResponse
	json.NewDecoder(resp.Body).Decode(&listResp)

	found := false
	for _, s := range listResp.Sessions {
		if s.ID == createResp.ID {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("created session not found in active sessions")
	}

	// Revoke session
	req, _ = http.NewRequest("POST",
		fmt.Sprintf("%s/orgs/%s/envs/%s/sessions/%s/revoke", serverURL, testOrgID, testEnvID, createResp.ID),
		nil)
	req.Header.Set("Authorization", "Bearer "+testToken)

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("revoke request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestE2EAuditTrail(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Create a session to generate audit event
	reqBody := trek.CreateSessionRequest{
		Selector:   trek.Selector{UserID: "audit_test_user"},
		Level:      trek.LevelDebug,
		TTLSeconds: 300,
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST",
		fmt.Sprintf("%s/orgs/%s/envs/%s/sessions", serverURL, testOrgID, testEnvID),
		bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Skipf("Server not available: %v", err)
	}
	resp.Body.Close()

	// Check audit log
	req, _ = http.NewRequest("GET",
		fmt.Sprintf("%s/orgs/%s/envs/%s/audit", serverURL, testOrgID, testEnvID),
		nil)
	req.Header.Set("Authorization", "Bearer "+testToken)

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("audit request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var auditResp struct {
		Events []struct {
			Action string `json:"Action"`
		} `json:"events"`
	}
	json.NewDecoder(resp.Body).Decode(&auditResp)

	if len(auditResp.Events) == 0 {
		t.Error("expected audit events")
	}
}

func TestE2ESDKIntegration(t *testing.T) {
	// Test SDK client against real server
	client := trek.NewClient(serverURL, testToken, testOrgID, testEnvID)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create session via SDK
	resp, err := client.CreateSession(ctx, trek.CreateSessionRequest{
		Selector:   trek.Selector{TenantID: "sdk_test_tenant"},
		Level:      trek.LevelTrace,
		TTLSeconds: 300,
	})
	if err != nil {
		t.Skipf("Server not available: %v", err)
	}

	if resp.ID == "" {
		t.Fatal("expected session ID")
	}

	// Get active sessions
	sessions, err := client.GetActiveSessions(ctx, "test-service", "")
	if err != nil {
		t.Fatalf("get active sessions failed: %v", err)
	}

	if sessions == nil {
		t.Fatal("expected sessions response")
	}

	// Revoke via SDK
	err = client.RevokeSession(ctx, resp.ID)
	if err != nil {
		t.Fatalf("revoke failed: %v", err)
	}
}
