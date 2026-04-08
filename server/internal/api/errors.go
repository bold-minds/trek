package api

import (
	"github.com/gofiber/fiber/v2"
)

// APIError represents a structured API error response.
type APIError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// Error codes for consistent API responses.
const (
	ErrCodeBadRequest       = "bad_request"
	ErrCodeUnauthorized     = "unauthorized"
	ErrCodeForbidden        = "forbidden"
	ErrCodeNotFound         = "not_found"
	ErrCodeConflict         = "conflict"
	ErrCodeValidationFailed = "validation_error"
	ErrCodeInternalError    = "internal_error"
	ErrCodePolicyViolation  = "policy_violation"
)

// NewBadRequest creates a 400 Bad Request error.
func NewBadRequest(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusBadRequest).JSON(APIError{
		Code:    ErrCodeBadRequest,
		Message: message,
	})
}

// NewUnauthorized creates a 401 Unauthorized error.
func NewUnauthorized(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusUnauthorized).JSON(APIError{
		Code:    ErrCodeUnauthorized,
		Message: message,
	})
}

// NewForbidden creates a 403 Forbidden error.
func NewForbidden(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusForbidden).JSON(APIError{
		Code:    ErrCodeForbidden,
		Message: message,
	})
}

// NewNotFound creates a 404 Not Found error.
func NewNotFound(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusNotFound).JSON(APIError{
		Code:    ErrCodeNotFound,
		Message: message,
	})
}

// NewConflict creates a 409 Conflict error.
func NewConflict(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusConflict).JSON(APIError{
		Code:    ErrCodeConflict,
		Message: message,
	})
}

// NewValidationError creates a 400 error with validation details.
func NewValidationError(c *fiber.Ctx, field, message string) error {
	return c.Status(fiber.StatusBadRequest).JSON(APIError{
		Code:    ErrCodeValidationFailed,
		Message: message,
		Details: map[string]any{"field": field},
	})
}

// NewPolicyViolation creates a 400 error for policy violations.
func NewPolicyViolation(c *fiber.Ctx, message string, details map[string]any) error {
	return c.Status(fiber.StatusBadRequest).JSON(APIError{
		Code:    ErrCodePolicyViolation,
		Message: message,
		Details: details,
	})
}

// NewInternalError creates a 500 Internal Server Error.
func NewInternalError(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusInternalServerError).JSON(APIError{
		Code:    ErrCodeInternalError,
		Message: message,
	})
}
