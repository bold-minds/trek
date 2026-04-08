package api

import (
	"time"

	"github.com/bold-minds/trek/server/internal/store"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

// AdminHandler provides HTTP handlers for admin operations.
type AdminHandler struct {
	store store.Store
}

// NewAdminHandler creates a new admin handler.
func NewAdminHandler(s store.Store) *AdminHandler {
	return &AdminHandler{store: s}
}

// --- Notification Config Endpoints ---

// ListNotificationConfigs handles GET /orgs/:org/notifications
func (h *AdminHandler) ListNotificationConfigs(c *fiber.Ctx) error {
	ctx := c.UserContext()
	orgID := c.Params("org")
	envID := c.Query("env")

	configs, err := h.store.ListNotificationConfigs(ctx, orgID, envID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Str("request_id", c.Locals("requestid").(string)).Msg("list notification configs failed")
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	return c.JSON(fiber.Map{"configs": configs})
}

// CreateNotificationConfig handles POST /orgs/:org/notifications
func (h *AdminHandler) CreateNotificationConfig(c *fiber.Ctx) error {
	ctx := c.UserContext()
	orgID := c.Params("org")

	var req struct {
		EnvID   string         `json:"env_id"`
		Type    string         `json:"type"`
		Config  map[string]any `json:"config"`
		Enabled bool           `json:"enabled"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIError{
			Code:    "invalid_request",
			Message: "invalid request body",
		})
	}

	if req.Type != "slack" && req.Type != "webhook" {
		return c.Status(fiber.StatusBadRequest).JSON(APIError{
			Code:    "validation_error",
			Message: "type must be 'slack' or 'webhook'",
		})
	}

	config := &store.NotificationConfig{
		ID:        generateID("notif"),
		OrgID:     orgID,
		EnvID:     req.EnvID,
		Type:      req.Type,
		Config:    req.Config,
		Enabled:   req.Enabled,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := h.store.CreateNotificationConfig(ctx, config); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Str("request_id", c.Locals("requestid").(string)).Msg("create notification config failed")
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(config)
}

// UpdateNotificationConfig handles PUT /orgs/:org/notifications/:id
func (h *AdminHandler) UpdateNotificationConfig(c *fiber.Ctx) error {
	ctx := c.UserContext()
	orgID := c.Params("org")
	configID := c.Params("id")

	var req struct {
		Config  map[string]any `json:"config"`
		Enabled *bool          `json:"enabled"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIError{
			Code:    "invalid_request",
			Message: "invalid request body",
		})
	}

	if err := h.store.UpdateNotificationConfig(ctx, orgID, configID, req.Config, req.Enabled); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Str("config_id", configID).Str("request_id", c.Locals("requestid").(string)).Msg("update notification config failed")
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	return c.JSON(fiber.Map{"status": "updated"})
}

// DeleteNotificationConfig handles DELETE /orgs/:org/notifications/:id
func (h *AdminHandler) DeleteNotificationConfig(c *fiber.Ctx) error {
	ctx := c.UserContext()
	orgID := c.Params("org")
	configID := c.Params("id")

	if err := h.store.DeleteNotificationConfig(ctx, orgID, configID); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Str("config_id", configID).Str("request_id", c.Locals("requestid").(string)).Msg("delete notification config failed")
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// --- Environment Management Endpoints ---

// ListEnvironments handles GET /orgs/:org/envs
func (h *AdminHandler) ListEnvironments(c *fiber.Ctx) error {
	ctx := c.UserContext()
	orgID := c.Params("org")

	envs, err := h.store.ListEnvs(ctx, orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Str("request_id", c.Locals("requestid").(string)).Msg("list environments failed")
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	return c.JSON(fiber.Map{"environments": envs})
}

// CreateEnvironment handles POST /orgs/:org/envs
func (h *AdminHandler) CreateEnvironment(c *fiber.Ctx) error {
	ctx := c.UserContext()
	orgID := c.Params("org")

	var req struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIError{
			Code:    "invalid_request",
			Message: "invalid request body",
		})
	}

	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(APIError{
			Code:    "validation_error",
			Message: "name is required",
		})
	}

	envID := req.ID
	if envID == "" {
		envID = generateID("env")
	}

	env := &store.Env{
		ID:        envID,
		OrgID:     orgID,
		Name:      req.Name,
		CreatedAt: time.Now(),
	}

	if err := h.store.CreateEnv(ctx, env); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Str("env_name", req.Name).Str("request_id", c.Locals("requestid").(string)).Msg("create environment failed")
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(env)
}

// DeleteEnvironment handles DELETE /orgs/:org/envs/:env
func (h *AdminHandler) DeleteEnvironment(c *fiber.Ctx) error {
	ctx := c.UserContext()
	orgID := c.Params("org")
	envID := c.Params("env")

	if err := h.store.DeleteEnv(ctx, orgID, envID); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Str("env_id", envID).Str("request_id", c.Locals("requestid").(string)).Msg("delete environment failed")
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// --- User Management Endpoints ---

// ListUsers handles GET /orgs/:org/users
func (h *AdminHandler) ListUsers(c *fiber.Ctx) error {
	ctx := c.UserContext()
	orgID := c.Params("org")

	users, err := h.store.ListUsers(ctx, orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Str("request_id", c.Locals("requestid").(string)).Msg("list users failed")
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	return c.JSON(fiber.Map{"users": users})
}

// CreateUser handles POST /orgs/:org/users
func (h *AdminHandler) CreateUser(c *fiber.Ctx) error {
	ctx := c.UserContext()
	orgID := c.Params("org")

	var req struct {
		OIDCSubject string `json:"oidc_subject"`
		Email       string `json:"email"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIError{
			Code:    "invalid_request",
			Message: "invalid request body",
		})
	}

	if req.OIDCSubject == "" || req.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(APIError{
			Code:    "validation_error",
			Message: "oidc_subject and email are required",
		})
	}

	user := &store.User{
		ID:          generateID("usr"),
		OrgID:       orgID,
		OIDCSubject: req.OIDCSubject,
		Email:       req.Email,
		CreatedAt:   time.Now(),
	}

	if err := h.store.CreateUser(ctx, user); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Str("email", req.Email).Str("request_id", c.Locals("requestid").(string)).Msg("create user failed")
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(user)
}

// DeleteUser handles DELETE /orgs/:org/users/:id
func (h *AdminHandler) DeleteUser(c *fiber.Ctx) error {
	ctx := c.UserContext()
	orgID := c.Params("org")
	userID := c.Params("id")

	if err := h.store.DeleteUser(ctx, orgID, userID); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Str("user_id", userID).Str("request_id", c.Locals("requestid").(string)).Msg("delete user failed")
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// --- Role Management Endpoints ---

// ListRoles handles GET /orgs/:org/roles
func (h *AdminHandler) ListRoles(c *fiber.Ctx) error {
	ctx := c.UserContext()
	orgID := c.Params("org")

	roles, err := h.store.ListRoles(ctx, orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Str("request_id", c.Locals("requestid").(string)).Msg("list roles failed")
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	return c.JSON(fiber.Map{"roles": roles})
}

// CreateRole handles POST /orgs/:org/roles
func (h *AdminHandler) CreateRole(c *fiber.Ctx) error {
	ctx := c.UserContext()
	orgID := c.Params("org")

	var req struct {
		Name        string   `json:"name"`
		Permissions []string `json:"permissions"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIError{
			Code:    "invalid_request",
			Message: "invalid request body",
		})
	}

	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(APIError{
			Code:    "validation_error",
			Message: "name is required",
		})
	}

	role := &store.Role{
		ID:          generateID("role"),
		OrgID:       orgID,
		Name:        req.Name,
		Permissions: req.Permissions,
		CreatedAt:   time.Now(),
	}

	if err := h.store.CreateRole(ctx, role); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Str("role_name", req.Name).Str("request_id", c.Locals("requestid").(string)).Msg("create role failed")
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(role)
}

// AssignRole handles POST /orgs/:org/users/:id/roles
func (h *AdminHandler) AssignRole(c *fiber.Ctx) error {
	ctx := c.UserContext()
	orgID := c.Params("org")
	userID := c.Params("id")

	var req struct {
		RoleID string `json:"role_id"`
		EnvID  string `json:"env_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIError{
			Code:    "invalid_request",
			Message: "invalid request body",
		})
	}

	if req.RoleID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(APIError{
			Code:    "validation_error",
			Message: "role_id is required",
		})
	}

	if err := h.store.AssignRole(ctx, orgID, userID, req.RoleID, req.EnvID); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Str("user_id", userID).Str("role_id", req.RoleID).Str("request_id", c.Locals("requestid").(string)).Msg("assign role failed")
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	return c.JSON(fiber.Map{"status": "assigned"})
}

// RevokeRole handles DELETE /orgs/:org/users/:id/roles/:roleId
func (h *AdminHandler) RevokeRole(c *fiber.Ctx) error {
	ctx := c.UserContext()
	orgID := c.Params("org")
	userID := c.Params("id")
	roleID := c.Params("roleId")
	envID := c.Query("env_id")

	if err := h.store.RevokeRole(ctx, orgID, userID, roleID, envID); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Str("user_id", userID).Str("role_id", roleID).Str("request_id", c.Locals("requestid").(string)).Msg("revoke role failed")
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// GetUserRoles handles GET /orgs/:org/users/:id/roles
func (h *AdminHandler) GetUserRoles(c *fiber.Ctx) error {
	ctx := c.UserContext()
	orgID := c.Params("org")
	userID := c.Params("id")

	roles, err := h.store.GetUserRoles(ctx, orgID, userID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Str("user_id", userID).Str("request_id", c.Locals("requestid").(string)).Msg("get user roles failed")
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	return c.JSON(fiber.Map{"roles": roles})
}
