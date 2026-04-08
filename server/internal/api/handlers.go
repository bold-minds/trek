package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/bold-minds/trek"
	"github.com/bold-minds/trek/server/internal/store"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

// Handler provides HTTP handlers for the Trek API.
type Handler struct {
	store store.Store
}

// NewHandler creates a new API handler.
func NewHandler(s store.Store) *Handler {
	return &Handler{store: s}
}

// CreateSession handles POST /orgs/:org/envs/:env/sessions
func (h *Handler) CreateSession(c *fiber.Ctx) error {
	ctx := c.UserContext()
	orgID := c.Params("org")
	envID := c.Params("env")

	var req trek.CreateSessionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIError{
			Code:    "invalid_request",
			Message: "invalid request body",
		})
	}

	policy, err := h.store.GetPolicy(ctx, orgID, envID)
	if err != nil {
		log.Error().Err(err).Msg("get policy failed")
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	if err := validateSessionRequest(req, policy); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIError{
			Code:    "validation_error",
			Message: err.Error(),
		})
	}

	var expiresAt time.Time
	if req.ExpiresAt != nil {
		expiresAt = *req.ExpiresAt
	} else {
		expiresAt = time.Now().Add(time.Duration(req.TTLSeconds) * time.Second)
	}

	caps := policy.DefaultCaps
	if req.Caps != nil {
		caps = *req.Caps
	}

	sessionID := generateID("sess")
	session := &store.Session{
		ID:           sessionID,
		OrgID:        orgID,
		EnvID:        envID,
		Status:       "active",
		Selector:     req.Selector,
		Level:        req.Level,
		ExpiresAt:    expiresAt,
		Caps:         caps,
		Labels:       req.Labels,
		ServiceScope: req.ServiceScope,
		CreatedBy:    actorFromContext(ctx),
		Reason:       req.Reason,
		CreatedAt:    time.Now(),
	}

	if err := h.store.CreateSession(ctx, session); err != nil {
		log.Error().Err(err).Msg("create session failed")
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	if err := h.store.CreateAuditEvent(ctx, &store.AuditEvent{
		ID:          generateID("aud"),
		OrgID:       orgID,
		EnvID:       envID,
		ActorUserID: actorFromContext(ctx),
		Action:      "session.created",
		TargetType:  "session",
		TargetID:    sessionID,
		Payload: map[string]any{
			"selector": req.Selector,
			"level":    req.Level,
			"ttl":      req.TTLSeconds,
			"reason":   req.Reason,
		},
		CreatedAt: time.Now(),
	}); err != nil {
		log.Warn().Err(err).Msg("audit event creation failed")
	}

	return c.Status(fiber.StatusCreated).JSON(trek.CreateSessionResponse{
		ID:        sessionID,
		Status:    "active",
		ExpiresAt: expiresAt,
	})
}

// GetActiveSessions handles GET /orgs/:org/envs/:env/active-sessions
func (h *Handler) GetActiveSessions(c *fiber.Ctx) error {
	ctx := c.UserContext()
	orgID := c.Params("org")
	envID := c.Params("env")
	serviceName := c.Query("service_name")
	clientRevision := c.Query("revision")

	sessions, err := h.store.GetActiveSessions(ctx, orgID, envID, serviceName)
	if err != nil {
		log.Error().Err(err).Msg("get active sessions failed")
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	revision := computeRevision(sessions)

	if clientRevision == revision {
		return c.SendStatus(fiber.StatusNotModified)
	}

	return c.JSON(trek.ActiveSessionsResponse{
		Revision:   revision,
		ServerTime: time.Now(),
		Sessions:   sessions,
	})
}

// ListSessions handles GET /orgs/:org/envs/:env/sessions
func (h *Handler) ListSessions(c *fiber.Ctx) error {
	ctx := c.UserContext()
	orgID := c.Params("org")
	envID := c.Params("env")
	status := c.Query("status")

	sessions, err := h.store.ListSessions(ctx, orgID, envID, store.SessionFilter{Status: status})
	if err != nil {
		log.Error().Err(err).Msg("list sessions failed")
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	return c.JSON(fiber.Map{"sessions": sessions})
}

// RevokeSession handles POST /orgs/:org/envs/:env/sessions/:id/revoke
func (h *Handler) RevokeSession(c *fiber.Ctx) error {
	ctx := c.UserContext()
	orgID := c.Params("org")
	envID := c.Params("env")
	sessionID := c.Params("id")

	if err := h.store.RevokeSession(ctx, orgID, envID, sessionID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(APIError{
				Code:    "not_found",
				Message: "session not found",
			})
		}
		log.Error().Err(err).Msg("revoke session failed")
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	if err := h.store.CreateAuditEvent(ctx, &store.AuditEvent{
		ID:          generateID("aud"),
		OrgID:       orgID,
		EnvID:       envID,
		ActorUserID: actorFromContext(ctx),
		Action:      "session.revoked",
		TargetType:  "session",
		TargetID:    sessionID,
		Payload:     map[string]any{},
		CreatedAt:   time.Now(),
	}); err != nil {
		log.Warn().Err(err).Msg("audit event creation failed")
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// GetAudit handles GET /orgs/:org/envs/:env/audit
func (h *Handler) GetAudit(c *fiber.Ctx) error {
	ctx := c.UserContext()
	orgID := c.Params("org")
	envID := c.Params("env")
	cursor := c.Query("cursor")

	events, nextCursor, err := h.store.ListAuditEvents(ctx, orgID, envID, 50, cursor)
	if err != nil {
		log.Error().Err(err).Msg("list audit events failed")
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	return c.JSON(fiber.Map{
		"events":      events,
		"next_cursor": nextCursor,
	})
}

// GetPolicy handles GET /orgs/:org/policies
func (h *Handler) GetPolicy(c *fiber.Ctx) error {
	ctx := c.UserContext()
	orgID := c.Params("org")
	envID := c.Query("env")
	if envID == "" {
		envID = c.Params("env")
	}

	policy, err := h.store.GetPolicy(ctx, orgID, envID)
	if err != nil {
		log.Error().Err(err).Msg("get policy failed")
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	return c.JSON(policy)
}

// Liveness handles GET /healthz - checks if the process is running
func (h *Handler) Liveness(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}

// Readiness handles GET /readyz - checks if the service can handle traffic
func (h *Handler) Readiness(c *fiber.Ctx) error {
	ctx := c.UserContext()

	if err := h.store.Ping(ctx); err != nil {
		log.Error().Err(err).Msg("database health check failed")
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "unavailable",
			"checks": fiber.Map{
				"database": err.Error(),
			},
		})
	}

	return c.JSON(fiber.Map{
		"status": "ok",
		"checks": fiber.Map{
			"database": "ok",
		},
	})
}

// CreateToken handles POST /orgs/:org/envs/:env/tokens
func (h *Handler) CreateToken(c *fiber.Ctx) error {
	ctx := c.UserContext()
	orgID := c.Params("org")
	envID := c.Params("env")

	var req struct {
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

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		log.Error().Err(err).Msg("crypto/rand failed")
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}
	plainToken := hex.EncodeToString(tokenBytes)
	tokenHash, err := store.HashToken(plainToken)
	if err != nil {
		log.Error().Err(err).Msg("token hashing failed")
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	token := &store.ServiceToken{
		ID:        generateID("tok"),
		OrgID:     orgID,
		EnvID:     envID,
		TokenHash: tokenHash,
		Name:      req.Name,
		CreatedAt: time.Now(),
	}

	if err := h.store.CreateToken(ctx, token); err != nil {
		log.Error().Err(err).Msg("create token failed")
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	if err := h.store.CreateAuditEvent(ctx, &store.AuditEvent{
		ID:          generateID("aud"),
		OrgID:       orgID,
		EnvID:       envID,
		ActorUserID: actorFromContext(ctx),
		Action:      "token.created",
		TargetType:  "token",
		TargetID:    token.ID,
		Payload:     map[string]any{"name": req.Name},
		CreatedAt:   time.Now(),
	}); err != nil {
		log.Warn().Err(err).Msg("audit event creation failed")
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":    token.ID,
		"name":  token.Name,
		"token": plainToken,
	})
}

// ListTokens handles GET /orgs/:org/envs/:env/tokens
func (h *Handler) ListTokens(c *fiber.Ctx) error {
	ctx := c.UserContext()
	orgID := c.Params("org")
	envID := c.Params("env")

	tokens, err := h.store.ListTokens(ctx, orgID, envID)
	if err != nil {
		log.Error().Err(err).Msg("list tokens failed")
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	result := make([]fiber.Map, len(tokens))
	for i, t := range tokens {
		result[i] = fiber.Map{
			"id":         t.ID,
			"name":       t.Name,
			"created_at": t.CreatedAt,
		}
	}

	return c.JSON(fiber.Map{"tokens": result})
}

// RevokeToken handles DELETE /orgs/:org/envs/:env/tokens/:id
func (h *Handler) RevokeToken(c *fiber.Ctx) error {
	ctx := c.UserContext()
	orgID := c.Params("org")
	envID := c.Params("env")
	tokenID := c.Params("id")

	if err := h.store.RevokeToken(ctx, orgID, envID, tokenID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(APIError{
				Code:    "not_found",
				Message: "token not found",
			})
		}
		log.Error().Err(err).Msg("revoke token failed")
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	if err := h.store.CreateAuditEvent(ctx, &store.AuditEvent{
		ID:          generateID("aud"),
		OrgID:       orgID,
		EnvID:       envID,
		ActorUserID: actorFromContext(ctx),
		Action:      "token.revoked",
		TargetType:  "token",
		TargetID:    tokenID,
		Payload:     map[string]any{},
		CreatedAt:   time.Now(),
	}); err != nil {
		log.Warn().Err(err).Msg("audit event creation failed")
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// UpdatePolicy handles PUT /orgs/:org/envs/:env/policies
func (h *Handler) UpdatePolicy(c *fiber.Ctx) error {
	ctx := c.UserContext()
	orgID := c.Params("org")
	envID := c.Params("env")

	var req struct {
		MaxTTLSeconds      int       `json:"max_ttl_seconds"`
		AllowEmptySelector bool      `json:"allow_empty_selector"`
		RequireReason      bool      `json:"require_reason"`
		DefaultCaps        trek.Caps `json:"default_caps"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIError{
			Code:    "invalid_request",
			Message: "invalid request body",
		})
	}

	if req.MaxTTLSeconds <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(APIError{
			Code:    "validation_error",
			Message: "max_ttl_seconds must be positive",
		})
	}

	policy := &store.Policy{
		ID:                 generateID("pol"),
		OrgID:              orgID,
		EnvID:              envID,
		MaxTTLSeconds:      req.MaxTTLSeconds,
		AllowEmptySelector: req.AllowEmptySelector,
		RequireReason:      req.RequireReason,
		DefaultCaps:        req.DefaultCaps,
	}

	if err := h.store.SetPolicy(ctx, policy); err != nil {
		log.Error().Err(err).Msg("set policy failed")
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "internal error",
		})
	}

	if err := h.store.CreateAuditEvent(ctx, &store.AuditEvent{
		ID:          generateID("aud"),
		OrgID:       orgID,
		EnvID:       envID,
		ActorUserID: actorFromContext(ctx),
		Action:      "policy.updated",
		TargetType:  "policy",
		TargetID:    orgID + "/" + envID,
		Payload: map[string]any{
			"max_ttl_seconds":      req.MaxTTLSeconds,
			"allow_empty_selector": req.AllowEmptySelector,
			"require_reason":       req.RequireReason,
		},
		CreatedAt: time.Now(),
	}); err != nil {
		log.Warn().Err(err).Msg("audit event creation failed")
	}

	return c.JSON(policy)
}

// Helpers

func validateSessionRequest(req trek.CreateSessionRequest, policy *store.Policy) error {
	if req.Level != trek.LevelDebug && req.Level != trek.LevelTrace {
		return &ValidationError{Field: "level", Message: "must be 'debug' or 'trace'"}
	}

	if req.TTLSeconds <= 0 && req.ExpiresAt == nil {
		return &ValidationError{Field: "ttl_seconds", Message: "required"}
	}

	if req.TTLSeconds > policy.MaxTTLSeconds {
		return &ValidationError{Field: "ttl_seconds", Message: "exceeds max TTL"}
	}

	if policy.RequireReason && req.Reason == "" {
		return &ValidationError{Field: "reason", Message: "required by policy"}
	}

	if trek.IsEmptySelector(req.Selector) && !policy.AllowEmptySelector {
		return &ValidationError{Field: "selector", Message: "empty selector not allowed"}
	}

	// Enforce allowed selector keys if configured
	if len(policy.AllowedSelectorKeys) > 0 {
		allowedKeys := make(map[string]bool)
		for _, k := range policy.AllowedSelectorKeys {
			allowedKeys[k] = true
		}

		usedKeys := getSelectorKeys(req.Selector)
		for _, key := range usedKeys {
			if !allowedKeys[key] {
				return &ValidationError{Field: "selector", Message: "selector key '" + key + "' not allowed by policy"}
			}
		}
	}

	return nil
}

// getSelectorKeys returns the keys that are set in a selector.
func getSelectorKeys(sel trek.Selector) []string {
	var keys []string
	if sel.UserID != "" {
		keys = append(keys, "user_id")
	}
	if sel.TenantID != "" {
		keys = append(keys, "tenant_id")
	}
	if sel.RequestID != "" {
		keys = append(keys, "request_id")
	}
	if sel.Route != "" {
		keys = append(keys, "route")
	}
	if len(sel.Custom) > 0 {
		keys = append(keys, "custom")
	}
	return keys
}

func computeRevision(sessions []trek.Session) string {
	if len(sessions) == 0 {
		return "empty"
	}

	h := sha256.New()
	for _, s := range sessions {
		h.Write([]byte(s.ID))
		h.Write([]byte(s.ExpiresAt.Format(time.RFC3339)))
	}

	return hex.EncodeToString(h.Sum(nil)[:16])
}

func generateID(prefix string) string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return prefix + "_" + hex.EncodeToString(b)
}

func actorFromContext(ctx context.Context) string {
	// Get actor from service token if present
	if token := TokenFromContext(ctx); token != nil {
		return "token:" + token.ID
	}
	return ""
}

// ValidationError represents a request validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
