package api

import (
	"github.com/bold-minds/trek/server/internal/caps"
	"github.com/gofiber/fiber/v2"
)

// CapsHandler handles cap checking API requests.
type CapsHandler struct {
	counter *caps.Counter
}

// NewCapsHandler creates a new caps handler.
func NewCapsHandler(counter *caps.Counter) *CapsHandler {
	return &CapsHandler{counter: counter}
}

// CheckCapsRequest is the request body for cap checking.
type CheckCapsRequest struct {
	SessionID        string `json:"session_id"`
	RequestID        string `json:"request_id"`
	MaxPerSession    int    `json:"max_per_session"`
	MaxPerRequest    int    `json:"max_per_request"`
	IncrementSession bool   `json:"increment_session"`
	IncrementRequest bool   `json:"increment_request"`
}

// CheckCaps handles POST /orgs/:org/envs/:env/caps/check
func (h *CapsHandler) CheckCaps(c *fiber.Ctx) error {
	if h.counter == nil {
		return c.JSON(caps.CheckResult{Allowed: true})
	}

	var req CheckCapsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIError{
			Code:    "invalid_request",
			Message: "invalid request body",
		})
	}

	ctx := c.UserContext()

	if req.IncrementSession || req.IncrementRequest {
		sessionID := ""
		requestID := ""
		if req.IncrementSession {
			sessionID = req.SessionID
		}
		if req.IncrementRequest {
			requestID = req.RequestID
		}

		result, err := h.counter.CheckAndIncrement(ctx, sessionID, requestID, req.MaxPerSession, req.MaxPerRequest)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(APIError{
				Code:    "internal_error",
				Message: "counter error",
			})
		}
		return c.JSON(result)
	}

	result := &caps.CheckResult{Allowed: true}

	if req.SessionID != "" {
		count, err := h.counter.GetSessionCount(ctx, req.SessionID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(APIError{
				Code:    "internal_error",
				Message: "counter error",
			})
		}
		result.SessionCount = count
		if req.MaxPerSession > 0 && count >= int64(req.MaxPerSession) {
			result.SessionCapReached = true
			result.Allowed = false
		}
	}

	if req.RequestID != "" {
		count, err := h.counter.GetRequestCount(ctx, req.RequestID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(APIError{
				Code:    "internal_error",
				Message: "counter error",
			})
		}
		result.RequestCount = count
		if req.MaxPerRequest > 0 && count >= int64(req.MaxPerRequest) {
			result.RequestCapReached = true
			result.Allowed = false
		}
	}

	return c.JSON(result)
}

// IncrementCaps handles POST /orgs/:org/envs/:env/caps/increment
func (h *CapsHandler) IncrementCaps(c *fiber.Ctx) error {
	if h.counter == nil {
		return c.SendStatus(fiber.StatusNoContent)
	}

	var req struct {
		SessionID     string `json:"session_id"`
		RequestID     string `json:"request_id"`
		MaxPerSession int    `json:"max_per_session"`
		MaxPerRequest int    `json:"max_per_request"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIError{
			Code:    "invalid_request",
			Message: "invalid request body",
		})
	}

	ctx := c.UserContext()

	result, err := h.counter.CheckAndIncrement(ctx, req.SessionID, req.RequestID, req.MaxPerSession, req.MaxPerRequest)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "counter error",
		})
	}

	return c.JSON(result)
}

// GetSessionCaps handles GET /orgs/:org/envs/:env/caps/sessions/:id
func (h *CapsHandler) GetSessionCaps(c *fiber.Ctx) error {
	if h.counter == nil {
		return c.JSON(fiber.Map{"count": 0})
	}

	sessionID := c.Params("id")

	count, err := h.counter.GetSessionCount(c.UserContext(), sessionID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "counter error",
		})
	}

	return c.JSON(fiber.Map{"count": count})
}

// ResetSessionCaps handles DELETE /orgs/:org/envs/:env/caps/sessions/:id
func (h *CapsHandler) ResetSessionCaps(c *fiber.Ctx) error {
	if h.counter == nil {
		return c.SendStatus(fiber.StatusNoContent)
	}

	sessionID := c.Params("id")

	if err := h.counter.ResetSession(c.UserContext(), sessionID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(APIError{
			Code:    "internal_error",
			Message: "counter error",
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}
