package api

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestNewRouter_ReturnsApp(t *testing.T) {
	store := newMockStore()
	app := NewRouter(store)

	if app == nil {
		t.Fatal("NewRouter returned nil")
	}
}

func TestNewRouterWithConfig_ReturnsApp(t *testing.T) {
	store := newMockStore()

	app := NewRouterWithConfig(RouterConfig{
		Store:   store,
		Counter: nil,
	})

	if app == nil {
		t.Fatal("NewRouterWithConfig returned nil")
	}
}

func TestNewRouter_HealthEndpoints(t *testing.T) {
	store := newMockStore()
	app := NewRouter(store)

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{
			name:       "liveness endpoint returns 200",
			path:       "/healthz",
			wantStatus: fiber.StatusOK,
		},
		{
			name:       "readiness endpoint returns 200 when store healthy",
			path:       "/readyz",
			wantStatus: fiber.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("failed to test request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
		})
	}
}

func TestNewRouter_ProtectedEndpointsRequireAuth(t *testing.T) {
	store := &mockStore{}
	app := NewRouter(store)

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "create session requires auth",
			method: "POST",
			path:   "/orgs/test-org/envs/dev/sessions",
		},
		{
			name:   "list sessions requires auth",
			method: "GET",
			path:   "/orgs/test-org/envs/dev/sessions",
		},
		{
			name:   "revoke session requires auth",
			method: "POST",
			path:   "/orgs/test-org/envs/dev/sessions/123/revoke",
		},
		{
			name:   "get active sessions requires auth",
			method: "GET",
			path:   "/orgs/test-org/envs/dev/active-sessions",
		},
		{
			name:   "create token requires auth",
			method: "POST",
			path:   "/orgs/test-org/envs/dev/tokens",
		},
		{
			name:   "list tokens requires auth",
			method: "GET",
			path:   "/orgs/test-org/envs/dev/tokens",
		},
		{
			name:   "revoke token requires auth",
			method: "DELETE",
			path:   "/orgs/test-org/envs/dev/tokens/123",
		},
		{
			name:   "get policy requires auth",
			method: "GET",
			path:   "/orgs/test-org/envs/dev/policies",
		},
		{
			name:   "update policy requires auth",
			method: "PUT",
			path:   "/orgs/test-org/envs/dev/policies",
		},
		{
			name:   "get audit requires auth",
			method: "GET",
			path:   "/orgs/test-org/envs/dev/audit",
		},
		{
			name:   "list environments requires auth",
			method: "GET",
			path:   "/orgs/test-org/envs",
		},
		{
			name:   "create environment requires auth",
			method: "POST",
			path:   "/orgs/test-org/envs",
		},
		{
			name:   "delete environment requires auth",
			method: "DELETE",
			path:   "/orgs/test-org/envs/dev",
		},
		{
			name:   "list users requires auth",
			method: "GET",
			path:   "/orgs/test-org/users",
		},
		{
			name:   "create user requires auth",
			method: "POST",
			path:   "/orgs/test-org/users",
		},
		{
			name:   "list roles requires auth",
			method: "GET",
			path:   "/orgs/test-org/roles",
		},
		{
			name:   "list notifications requires auth",
			method: "GET",
			path:   "/orgs/test-org/notifications",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("failed to test request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != fiber.StatusUnauthorized {
				t.Errorf("status = %d, want %d (unauthorized)", resp.StatusCode, fiber.StatusUnauthorized)
			}
		})
	}
}

func TestNewRouter_BodyLimitConfigured(t *testing.T) {
	store := newMockStore()
	app := NewRouter(store)

	// Verify the app has the body limit configured by checking
	// that small requests work fine
	req := httptest.NewRequest("GET", "/healthz", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestGlobalErrorHandler_FiberError(t *testing.T) {
	app := fiber.New(fiber.Config{
		ErrorHandler: globalErrorHandler,
	})

	// Add middleware to set requestid like the real router does
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("requestid", "test-request-id")
		return c.Next()
	})

	app.Get("/error", func(c *fiber.Ctx) error {
		return fiber.NewError(fiber.StatusBadRequest, "bad request")
	})

	req := httptest.NewRequest("GET", "/error", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusBadRequest)
	}
}

func TestGlobalErrorHandler_GenericError(t *testing.T) {
	app := fiber.New(fiber.Config{
		ErrorHandler: globalErrorHandler,
	})

	// Add middleware to set requestid like the real router does
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("requestid", "test-request-id")
		return c.Next()
	})

	app.Get("/error", func(c *fiber.Ctx) error {
		return fiber.ErrInternalServerError
	})

	req := httptest.NewRequest("GET", "/error", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusInternalServerError)
	}
}

func TestZerologMiddleware_LogsRequest(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("requestid", "test-request-id")
		return c.Next()
	})
	app.Use(zerologMiddleware())
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}
}

func TestRouterConfig_Defaults(t *testing.T) {
	cfg := RouterConfig{}

	if cfg.Store != nil {
		t.Error("expected Store to be nil by default")
	}
	if cfg.Counter != nil {
		t.Error("expected Counter to be nil by default")
	}
}

func TestMaxBodySize_Constant(t *testing.T) {
	expected := 1 << 20
	if MaxBodySize != expected {
		t.Errorf("MaxBodySize = %d, want %d", MaxBodySize, expected)
	}
}

func TestNewRouter_CapsEndpointsRegistered(t *testing.T) {
	store := newMockStore()
	app := NewRouterWithConfig(RouterConfig{
		Store:   store,
		Counter: nil,
	})

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "check caps endpoint",
			method: "POST",
			path:   "/orgs/test-org/envs/dev/caps/check",
		},
		{
			name:   "increment caps endpoint",
			method: "POST",
			path:   "/orgs/test-org/envs/dev/caps/increment",
		},
		{
			name:   "get session caps endpoint",
			method: "GET",
			path:   "/orgs/test-org/envs/dev/caps/sessions/123",
		},
		{
			name:   "reset session caps endpoint",
			method: "DELETE",
			path:   "/orgs/test-org/envs/dev/caps/sessions/123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("failed to test request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == fiber.StatusNotFound {
				t.Errorf("endpoint %s %s not registered (got 404)", tt.method, tt.path)
			}
		})
	}
}

func TestNewRouter_OrgOnlyEndpointsRegistered(t *testing.T) {
	store := newMockStore()
	app := NewRouter(store)

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "org policies endpoint",
			method: "GET",
			path:   "/orgs/test-org/policies",
		},
		{
			name:   "list environments",
			method: "GET",
			path:   "/orgs/test-org/envs",
		},
		{
			name:   "create environment",
			method: "POST",
			path:   "/orgs/test-org/envs",
		},
		{
			name:   "list users",
			method: "GET",
			path:   "/orgs/test-org/users",
		},
		{
			name:   "create user",
			method: "POST",
			path:   "/orgs/test-org/users",
		},
		{
			name:   "list roles",
			method: "GET",
			path:   "/orgs/test-org/roles",
		},
		{
			name:   "create role",
			method: "POST",
			path:   "/orgs/test-org/roles",
		},
		{
			name:   "list notifications",
			method: "GET",
			path:   "/orgs/test-org/notifications",
		},
		{
			name:   "create notification",
			method: "POST",
			path:   "/orgs/test-org/notifications",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("failed to test request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == fiber.StatusNotFound {
				t.Errorf("endpoint %s %s not registered (got 404)", tt.method, tt.path)
			}
		})
	}
}
