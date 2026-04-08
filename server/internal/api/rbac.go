package api

import (
	"context"

	"github.com/bold-minds/trek/server/internal/store"
	"github.com/gofiber/fiber/v2"
)

// Permission constants for RBAC.
const (
	PermSessionCreate = "session:create"
	PermSessionRevoke = "session:revoke"
	PermSessionRead   = "session:read"
	PermTokenCreate   = "token:create"
	PermTokenRevoke   = "token:revoke"
	PermTokenRead     = "token:read"
	PermPolicyRead    = "policy:read"
	PermPolicyWrite   = "policy:write"
	PermAuditRead     = "audit:read"
	PermUserManage    = "user:manage"
	PermRoleManage    = "role:manage"
	PermEnvManage     = "env:manage"
	PermNotifyManage  = "notify:manage"
	PermAdmin         = "admin"
)

type permissionsContextKey struct{}

// RBACMiddleware provides role-based access control.
type RBACMiddleware struct {
	store store.Store
}

// NewRBACMiddleware creates a new RBAC middleware.
func NewRBACMiddleware(s store.Store) *RBACMiddleware {
	return &RBACMiddleware{store: s}
}

// LoadPermissions loads user permissions into the context.
func (m *RBACMiddleware) LoadPermissions(c *fiber.Ctx) error {
	ctx := c.UserContext()
	orgID := c.Params("org")
	envID := c.Params("env")

	if token := TokenFromContext(ctx); token != nil {
		permissions := []string{
			PermSessionCreate, PermSessionRevoke, PermSessionRead,
			PermTokenCreate, PermTokenRead, PermTokenRevoke,
			PermPolicyRead, PermPolicyWrite,
			PermAuditRead,
			PermUserManage, PermRoleManage, PermEnvManage, PermNotifyManage,
		}
		ctx = context.WithValue(ctx, permissionsContextKey{}, permissions)
		c.SetUserContext(ctx)
		return c.Next()
	}

	userID := userIDFromContext(ctx)
	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(APIError{
			Code:    "unauthorized",
			Message: "authentication required",
		})
	}

	permissions, err := m.store.GetUserPermissions(ctx, orgID, userID, envID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "failed to load permissions",
		})
	}

	ctx = context.WithValue(ctx, permissionsContextKey{}, permissions)
	c.SetUserContext(ctx)
	return c.Next()
}

// RequirePermission creates middleware that checks for a specific permission.
func (m *RBACMiddleware) RequirePermission(permission string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !HasPermission(c.UserContext(), permission) {
			return c.Status(fiber.StatusForbidden).JSON(APIError{
				Code:    "forbidden",
				Message: "permission denied: " + permission,
			})
		}
		return c.Next()
	}
}

// HasPermission checks if the current context has a specific permission.
func HasPermission(ctx context.Context, permission string) bool {
	permissions, ok := ctx.Value(permissionsContextKey{}).([]string)
	if !ok {
		return false
	}

	for _, p := range permissions {
		if p == permission || p == PermAdmin {
			return true
		}
	}
	return false
}

// PermissionsFromContext returns all permissions from context.
func PermissionsFromContext(ctx context.Context) []string {
	permissions, _ := ctx.Value(permissionsContextKey{}).([]string)
	return permissions
}

type userContextKey struct{}

// userIDFromContext extracts user ID from context (set by OIDC middleware).
func userIDFromContext(ctx context.Context) string {
	userID, _ := ctx.Value(userContextKey{}).(string)
	return userID
}

// ContextWithUserID adds user ID to context.
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userContextKey{}, userID)
}
