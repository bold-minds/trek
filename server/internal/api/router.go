package api

import (
	"github.com/bold-minds/trek/server/internal/caps"
	"github.com/bold-minds/trek/server/internal/store"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/rs/zerolog/log"
)

const (
	MaxBodySize = 1 << 20 // 1MB max request body
)

// RouterConfig holds router configuration.
type RouterConfig struct {
	Store   store.Store
	Counter *caps.Counter
}

// NewRouter creates the HTTP router with all routes configured.
// This is a convenience wrapper around NewRouterWithConfig that uses
// default configuration options. For production use, prefer
// NewRouterWithConfig with explicit configuration.
func NewRouter(s store.Store) *fiber.App {
	return NewRouterWithConfig(RouterConfig{Store: s})
}

// NewRouterWithConfig creates the HTTP router with full configuration.
func NewRouterWithConfig(cfg RouterConfig) *fiber.App {
	app := fiber.New(fiber.Config{
		BodyLimit:             MaxBodySize,
		DisableStartupMessage: true,
		ErrorHandler:          globalErrorHandler,
	})

	app.Use(requestid.New())
	app.Use(zerologMiddleware())
	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(c *fiber.Ctx, e interface{}) {
			log.Error().
				Str("request_id", c.Locals("requestid").(string)).
				Str("path", c.Path()).
				Str("method", c.Method()).
				Interface("panic", e).
				Msg("panic recovered")
		},
	}))

	h := NewHandler(cfg.Store)
	admin := NewAdminHandler(cfg.Store)
	auth := NewAuthMiddleware(cfg.Store)
	rbac := NewRBACMiddleware(cfg.Store)
	capsHandler := NewCapsHandler(cfg.Counter)

	app.Get("/healthz", h.Liveness)
	app.Get("/readyz", h.Readiness)

	orgEnv := app.Group("/orgs/:org/envs/:env",
		auth.ServiceToken,
		auth.RequireOrgEnv,
		rbac.LoadPermissions,
	)

	orgEnv.Post("/sessions", rbac.RequirePermission(PermSessionCreate), h.CreateSession)
	orgEnv.Get("/sessions", rbac.RequirePermission(PermSessionRead), h.ListSessions)
	orgEnv.Post("/sessions/:id/revoke", rbac.RequirePermission(PermSessionRevoke), h.RevokeSession)
	orgEnv.Get("/active-sessions", rbac.RequirePermission(PermSessionRead), h.GetActiveSessions)

	orgEnv.Post("/tokens", rbac.RequirePermission(PermTokenCreate), h.CreateToken)
	orgEnv.Get("/tokens", rbac.RequirePermission(PermTokenRead), h.ListTokens)
	orgEnv.Delete("/tokens/:id", rbac.RequirePermission(PermTokenRevoke), h.RevokeToken)

	orgEnv.Get("/policies", rbac.RequirePermission(PermPolicyRead), h.GetPolicy)
	orgEnv.Put("/policies", rbac.RequirePermission(PermPolicyWrite), h.UpdatePolicy)

	orgEnv.Get("/audit", rbac.RequirePermission(PermAuditRead), h.GetAudit)

	orgEnv.Post("/caps/check", rbac.RequirePermission(PermSessionRead), capsHandler.CheckCaps)
	orgEnv.Post("/caps/increment", rbac.RequirePermission(PermSessionCreate), capsHandler.IncrementCaps)
	orgEnv.Get("/caps/sessions/:id", rbac.RequirePermission(PermSessionRead), capsHandler.GetSessionCaps)
	orgEnv.Delete("/caps/sessions/:id", rbac.RequirePermission(PermSessionRevoke), capsHandler.ResetSessionCaps)

	orgOnly := app.Group("/orgs/:org",
		auth.ServiceToken,
		rbac.LoadPermissions,
	)

	orgOnly.Get("/policies", rbac.RequirePermission(PermPolicyRead), h.GetPolicy)

	orgOnly.Get("/envs", rbac.RequirePermission(PermEnvManage), admin.ListEnvironments)
	orgOnly.Post("/envs", rbac.RequirePermission(PermEnvManage), admin.CreateEnvironment)
	orgOnly.Delete("/envs/:env", rbac.RequirePermission(PermEnvManage), admin.DeleteEnvironment)

	orgOnly.Get("/users", rbac.RequirePermission(PermUserManage), admin.ListUsers)
	orgOnly.Post("/users", rbac.RequirePermission(PermUserManage), admin.CreateUser)
	orgOnly.Delete("/users/:id", rbac.RequirePermission(PermUserManage), admin.DeleteUser)
	orgOnly.Get("/users/:id/roles", rbac.RequirePermission(PermUserManage), admin.GetUserRoles)
	orgOnly.Post("/users/:id/roles", rbac.RequirePermission(PermRoleManage), admin.AssignRole)
	orgOnly.Delete("/users/:id/roles/:roleId", rbac.RequirePermission(PermRoleManage), admin.RevokeRole)

	orgOnly.Get("/roles", rbac.RequirePermission(PermRoleManage), admin.ListRoles)
	orgOnly.Post("/roles", rbac.RequirePermission(PermRoleManage), admin.CreateRole)

	orgOnly.Get("/notifications", rbac.RequirePermission(PermNotifyManage), admin.ListNotificationConfigs)
	orgOnly.Post("/notifications", rbac.RequirePermission(PermNotifyManage), admin.CreateNotificationConfig)
	orgOnly.Put("/notifications/:id", rbac.RequirePermission(PermNotifyManage), admin.UpdateNotificationConfig)
	orgOnly.Delete("/notifications/:id", rbac.RequirePermission(PermNotifyManage), admin.DeleteNotificationConfig)

	return app
}

func zerologMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		log.Debug().
			Str("request_id", c.Locals("requestid").(string)).
			Str("method", c.Method()).
			Str("path", c.Path()).
			Str("ip", c.IP()).
			Msg("request started")

		err := c.Next()

		log.Info().
			Str("request_id", c.Locals("requestid").(string)).
			Str("method", c.Method()).
			Str("path", c.Path()).
			Int("status", c.Response().StatusCode()).
			Msg("request completed")

		return err
	}
}

func globalErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}

	log.Error().
		Str("request_id", c.Locals("requestid").(string)).
		Str("path", c.Path()).
		Err(err).
		Msg("unhandled error")

	return c.Status(code).JSON(APIError{
		Code:    "internal_error",
		Message: "An internal error occurred",
	})
}
