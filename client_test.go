package trek

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientGetActiveSessions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/orgs/org1/envs/prod/active-sessions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("missing or wrong authorization header")
		}

		resp := ActiveSessionsResponse{
			Revision:   "rev1",
			ServerTime: time.Now(),
			Sessions: []Session{
				{
					ID:        "sess1",
					Level:     LevelDebug,
					ExpiresAt: time.Now().Add(time.Hour),
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", "org1", "prod")
	resp, err := client.GetActiveSessions(context.Background(), "my-service", "")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if len(resp.Sessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(resp.Sessions))
	}
}

func TestClientGetActiveSessionsNotModified(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", "org1", "prod")
	resp, err := client.GetActiveSessions(context.Background(), "my-service", "rev1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != nil {
		t.Error("expected nil response for 304")
	}
}

func TestClientGetActiveSessionsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer server.Close()

	// Use NewClientWithHTTP to bypass retry logic for this test
	client := NewClientWithHTTP(server.URL, "test-token", "org1", "prod", &http.Client{})
	_, err := client.GetActiveSessions(context.Background(), "my-service", "")

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != 400 {
		t.Errorf("expected status 400, got %d", apiErr.StatusCode)
	}
}

func TestClientCreateSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var req CreateSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		if req.Selector.UserID != "u123" {
			t.Errorf("expected user u123, got %s", req.Selector.UserID)
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(CreateSessionResponse{
			ID:        "sess_123",
			Status:    "active",
			ExpiresAt: time.Now().Add(time.Hour),
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", "org1", "prod")
	resp, err := client.CreateSession(context.Background(), CreateSessionRequest{
		Selector:   Selector{UserID: "u123"},
		Level:      LevelDebug,
		TTLSeconds: 600,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "sess_123" {
		t.Errorf("expected sess_123, got %s", resp.ID)
	}
}

func TestClientRevokeSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/orgs/org1/envs/prod/sessions/sess_123/revoke" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", "org1", "prod")
	err := client.RevokeSession(context.Background(), "sess_123")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientRevokeSessionNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", "org1", "prod")
	err := client.RevokeSession(context.Background(), "nonexistent")

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", apiErr.StatusCode)
	}
}

func TestClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClientWithHTTP(server.URL, "test-token", "org1", "prod", &http.Client{
		Timeout: 10 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := client.GetActiveSessions(ctx, "my-service", "")

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestAPIErrorMessage(t *testing.T) {
	err := &APIError{StatusCode: 401, Message: "unauthorized"}
	expected := "trek api error: status 401: unauthorized"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}
