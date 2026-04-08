package api

import (
	"context"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/bold-minds/trek/server/internal/store"
	"github.com/gofiber/fiber/v2"
)

type mockStoreForRBAC struct {
	store.Store
	getUserPermissionsFn func(ctx context.Context, orgID, userID, envID string) ([]string, error)
}

func (m *mockStoreForRBAC) GetUserPermissions(ctx context.Context, orgID, userID, envID string) ([]string, error) {
	if m.getUserPermissionsFn != nil {
		return m.getUserPermissionsFn(ctx, orgID, userID, envID)
	}
	return nil, nil
}

func TestNewRBACMiddleware(t *testing.T) {
	mockStore := &mockStoreForRBAC{}
	middleware := NewRBACMiddleware(mockStore)

	if middleware == nil {
		t.Fatal("NewRBACMiddleware returned nil")
	}
	if middleware.store != mockStore {
		t.Error("store not set correctly")
	}
}

func TestHasPermission(t *testing.T) {
	tests := []struct {
		name        string
		permissions []string
		permission  string
		want        bool
	}{
		{
			name:        "no permissions returns false",
			permissions: nil,
			permission:  PermSessionCreate,
			want:        false,
		},
		{
			name:        "empty permissions returns false",
			permissions: []string{},
			permission:  PermSessionCreate,
			want:        false,
		},
		{
			name:        "has exact permission",
			permissions: []string{PermSessionCreate, PermSessionRead},
			permission:  PermSessionCreate,
			want:        true,
		},
		{
			name:        "does not have permission",
			permissions: []string{PermSessionRead},
			permission:  PermSessionCreate,
			want:        false,
		},
		{
			name:        "admin permission grants all",
			permissions: []string{PermAdmin},
			permission:  PermSessionCreate,
			want:        true,
		},
		{
			name:        "admin permission grants policy write",
			permissions: []string{PermAdmin},
			permission:  PermPolicyWrite,
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.permissions != nil {
				ctx = context.WithValue(ctx, permissionsContextKey{}, tt.permissions)
			}

			got := HasPermission(ctx, tt.permission)
			if got != tt.want {
				t.Errorf("HasPermission() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPermissionsFromContext(t *testing.T) {
	t.Run("returns nil when no permissions in context", func(t *testing.T) {
		ctx := context.Background()
		perms := PermissionsFromContext(ctx)
		if perms != nil {
			t.Errorf("expected nil, got %v", perms)
		}
	})

	t.Run("returns permissions when present", func(t *testing.T) {
		expected := []string{PermSessionCreate, PermSessionRead}
		ctx := context.WithValue(context.Background(), permissionsContextKey{}, expected)
		perms := PermissionsFromContext(ctx)
		if len(perms) != len(expected) {
			t.Errorf("len(perms) = %d, want %d", len(perms), len(expected))
		}
	})

	t.Run("returns nil for wrong type", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), permissionsContextKey{}, "not-a-slice")
		perms := PermissionsFromContext(ctx)
		if perms != nil {
			t.Errorf("expected nil for wrong type, got %v", perms)
		}
	})
}

func TestContextWithUserID(t *testing.T) {
	ctx := context.Background()
	userID := "user-123"

	newCtx := ContextWithUserID(ctx, userID)
	got := userIDFromContext(newCtx)

	if got != userID {
		t.Errorf("userIDFromContext() = %q, want %q", got, userID)
	}
}

func TestUserIDFromContext(t *testing.T) {
	t.Run("returns empty string when no user ID", func(t *testing.T) {
		ctx := context.Background()
		got := userIDFromContext(ctx)
		if got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	t.Run("returns user ID when present", func(t *testing.T) {
		expected := "user-456"
		ctx := context.WithValue(context.Background(), userContextKey{}, expected)
		got := userIDFromContext(ctx)
		if got != expected {
			t.Errorf("userIDFromContext() = %q, want %q", got, expected)
		}
	})
}

func TestRBACMiddleware_LoadPermissions(t *testing.T) {
	tests := []struct {
		name            string
		hasToken        bool
		hasUserID       bool
		mockPermissions []string
		mockError       error
		expectedStatus  int
		expectFullPerms bool
	}{
		{
			name:            "grants full permissions for service token",
			hasToken:        true,
			expectedStatus:  fiber.StatusOK,
			expectFullPerms: true,
		},
		{
			name:           "returns unauthorized for no user ID and no token",
			hasToken:       false,
			hasUserID:      false,
			expectedStatus: fiber.StatusUnauthorized,
		},
		{
			name:            "loads permissions for user",
			hasToken:        false,
			hasUserID:       true,
			mockPermissions: []string{PermSessionCreate, PermSessionRead},
			expectedStatus:  fiber.StatusOK,
		},
		{
			name:           "returns internal error on permission load failure",
			hasToken:       false,
			hasUserID:      true,
			mockError:      context.DeadlineExceeded,
			expectedStatus: fiber.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &mockStoreForRBAC{
				getUserPermissionsFn: func(ctx context.Context, orgID, userID, envID string) ([]string, error) {
					if tt.mockError != nil {
						return nil, tt.mockError
					}
					return tt.mockPermissions, nil
				},
			}

			app := fiber.New()
			middleware := NewRBACMiddleware(mockStore)

			app.Use(func(c *fiber.Ctx) error {
				ctx := c.UserContext()
				if tt.hasToken {
					ctx = context.WithValue(ctx, tokenContextKey{}, &store.ServiceToken{ID: "token-1"})
				}
				if tt.hasUserID {
					ctx = context.WithValue(ctx, userContextKey{}, "user-123")
				}
				c.SetUserContext(ctx)
				return c.Next()
			})

			app.Get("/orgs/:org/envs/:env/test", middleware.LoadPermissions, func(c *fiber.Ctx) error {
				return c.SendStatus(fiber.StatusOK)
			})

			req := httptest.NewRequest("GET", "/orgs/org-1/envs/dev/test", nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test() error: %v", err)
			}

			if resp.StatusCode != tt.expectedStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("status = %d, want %d, body: %s", resp.StatusCode, tt.expectedStatus, body)
			}
		})
	}
}

func TestRBACMiddleware_RequirePermission(t *testing.T) {
	tests := []struct {
		name           string
		permissions    []string
		requiredPerm   string
		expectedStatus int
	}{
		{
			name:           "denies when permission missing",
			permissions:    []string{PermSessionRead},
			requiredPerm:   PermSessionCreate,
			expectedStatus: fiber.StatusForbidden,
		},
		{
			name:           "allows when permission present",
			permissions:    []string{PermSessionCreate, PermSessionRead},
			requiredPerm:   PermSessionCreate,
			expectedStatus: fiber.StatusOK,
		},
		{
			name:           "allows when admin permission present",
			permissions:    []string{PermAdmin},
			requiredPerm:   PermSessionCreate,
			expectedStatus: fiber.StatusOK,
		},
		{
			name:           "denies when no permissions",
			permissions:    nil,
			requiredPerm:   PermSessionCreate,
			expectedStatus: fiber.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &mockStoreForRBAC{}
			app := fiber.New()
			middleware := NewRBACMiddleware(mockStore)

			app.Use(func(c *fiber.Ctx) error {
				if tt.permissions != nil {
					ctx := context.WithValue(c.UserContext(), permissionsContextKey{}, tt.permissions)
					c.SetUserContext(ctx)
				}
				return c.Next()
			})

			app.Get("/test", middleware.RequirePermission(tt.requiredPerm), func(c *fiber.Ctx) error {
				return c.SendStatus(fiber.StatusOK)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test() error: %v", err)
			}

			if resp.StatusCode != tt.expectedStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("status = %d, want %d, body: %s", resp.StatusCode, tt.expectedStatus, body)
			}
		})
	}
}

func TestPermissionConstants(t *testing.T) {
	permissions := []string{
		PermSessionCreate,
		PermSessionRevoke,
		PermSessionRead,
		PermTokenCreate,
		PermTokenRevoke,
		PermTokenRead,
		PermPolicyRead,
		PermPolicyWrite,
		PermAuditRead,
		PermUserManage,
		PermRoleManage,
		PermEnvManage,
		PermNotifyManage,
		PermAdmin,
	}

	seen := make(map[string]bool)
	for _, p := range permissions {
		if p == "" {
			t.Error("empty permission constant found")
		}
		if seen[p] {
			t.Errorf("duplicate permission constant: %s", p)
		}
		seen[p] = true
	}
}
