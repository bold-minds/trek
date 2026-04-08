package api

import (
	"context"
	"strings"

	"github.com/bold-minds/trek/server/internal/store"
	"github.com/gofiber/fiber/v2"
)

// AuthMiddleware validates service tokens and user authentication.
type AuthMiddleware struct {
	store store.Store
}

// NewAuthMiddleware creates a new auth middleware.
func NewAuthMiddleware(s store.Store) *AuthMiddleware {
	return &AuthMiddleware{store: s}
}

// ServiceToken validates Bearer tokens for SDK/service access.
func (m *AuthMiddleware) ServiceToken(c *fiber.Ctx) error {
	auth := c.Get("Authorization")
	if auth == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(APIError{
			Code:    "unauthorized",
			Message: "missing authorization header",
		})
	}

	if !strings.HasPrefix(auth, "Bearer ") {
		return c.Status(fiber.StatusUnauthorized).JSON(APIError{
			Code:    "unauthorized",
			Message: "invalid authorization format",
		})
	}

	token := strings.TrimPrefix(auth, "Bearer ")

	serviceToken, err := m.store.ValidateTokenPlain(c.UserContext(), token)
	if err != nil {
		if err == store.ErrNotFound {
			return c.Status(fiber.StatusUnauthorized).JSON(APIError{
				Code:    "unauthorized",
				Message: "invalid token",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	orgID := c.Params("org")
	envID := c.Params("env")

	if serviceToken.OrgID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(APIError{
			Code:    "forbidden",
			Message: "token not valid for this org",
		})
	}
	if envID != "" && serviceToken.EnvID != envID {
		return c.Status(fiber.StatusForbidden).JSON(APIError{
			Code:    "forbidden",
			Message: "token not valid for this env",
		})
	}

	ctx := context.WithValue(c.UserContext(), tokenContextKey{}, serviceToken)
	c.SetUserContext(ctx)
	return c.Next()
}

// RequireOrgEnv ensures org and env exist.
func (m *AuthMiddleware) RequireOrgEnv(c *fiber.Ctx) error {
	ctx := c.UserContext()
	orgID := c.Params("org")
	envID := c.Params("env")

	if _, err := m.store.GetOrg(ctx, orgID); err != nil {
		if err == store.ErrNotFound {
			return c.Status(fiber.StatusNotFound).JSON(APIError{
				Code:    "not_found",
				Message: "org not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	if _, err := m.store.GetEnv(ctx, orgID, envID); err != nil {
		if err == store.ErrNotFound {
			return c.Status(fiber.StatusNotFound).JSON(APIError{
				Code:    "not_found",
				Message: "env not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	return c.Next()
}

type tokenContextKey struct{}

// TokenFromContext retrieves the service token from context.
func TokenFromContext(ctx context.Context) *store.ServiceToken {
	if v := ctx.Value(tokenContextKey{}); v != nil {
		if t, ok := v.(*store.ServiceToken); ok {
			return t
		}
	}
	return nil
}
