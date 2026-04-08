package api

import (
	"testing"
)

func TestAPIErrorStruct(t *testing.T) {
	err := APIError{
		Code:    ErrCodeBadRequest,
		Message: "invalid input",
		Details: map[string]any{"field": "selector"},
	}

	if err.Code != ErrCodeBadRequest {
		t.Errorf("Code = %q, want %q", err.Code, ErrCodeBadRequest)
	}
	if err.Message != "invalid input" {
		t.Errorf("Message = %q, want %q", err.Message, "invalid input")
	}
	if err.Details["field"] != "selector" {
		t.Errorf("Details[field] = %v, want %v", err.Details["field"], "selector")
	}
}

func TestErrorCodes(t *testing.T) {
	codes := []string{
		ErrCodeBadRequest,
		ErrCodeUnauthorized,
		ErrCodeForbidden,
		ErrCodeNotFound,
		ErrCodeConflict,
		ErrCodeValidationFailed,
		ErrCodeInternalError,
		ErrCodePolicyViolation,
	}

	for _, code := range codes {
		if code == "" {
			t.Errorf("error code should not be empty")
		}
	}

	// Verify uniqueness
	seen := make(map[string]bool)
	for _, code := range codes {
		if seen[code] {
			t.Errorf("duplicate error code: %s", code)
		}
		seen[code] = true
	}
}

func TestErrCodeBadRequest(t *testing.T) {
	if ErrCodeBadRequest != "bad_request" {
		t.Errorf("ErrCodeBadRequest = %q, want %q", ErrCodeBadRequest, "bad_request")
	}
}

func TestErrCodeUnauthorized(t *testing.T) {
	if ErrCodeUnauthorized != "unauthorized" {
		t.Errorf("ErrCodeUnauthorized = %q, want %q", ErrCodeUnauthorized, "unauthorized")
	}
}

func TestErrCodeForbidden(t *testing.T) {
	if ErrCodeForbidden != "forbidden" {
		t.Errorf("ErrCodeForbidden = %q, want %q", ErrCodeForbidden, "forbidden")
	}
}

func TestErrCodeNotFound(t *testing.T) {
	if ErrCodeNotFound != "not_found" {
		t.Errorf("ErrCodeNotFound = %q, want %q", ErrCodeNotFound, "not_found")
	}
}

func TestErrCodeConflict(t *testing.T) {
	if ErrCodeConflict != "conflict" {
		t.Errorf("ErrCodeConflict = %q, want %q", ErrCodeConflict, "conflict")
	}
}

func TestErrCodeValidationFailed(t *testing.T) {
	if ErrCodeValidationFailed != "validation_error" {
		t.Errorf("ErrCodeValidationFailed = %q, want %q", ErrCodeValidationFailed, "validation_error")
	}
}

func TestErrCodeInternalError(t *testing.T) {
	if ErrCodeInternalError != "internal_error" {
		t.Errorf("ErrCodeInternalError = %q, want %q", ErrCodeInternalError, "internal_error")
	}
}

func TestErrCodePolicyViolation(t *testing.T) {
	if ErrCodePolicyViolation != "policy_violation" {
		t.Errorf("ErrCodePolicyViolation = %q, want %q", ErrCodePolicyViolation, "policy_violation")
	}
}

func TestAPIErrorWithoutDetails(t *testing.T) {
	err := APIError{
		Code:    ErrCodeNotFound,
		Message: "resource not found",
	}

	if err.Details != nil {
		t.Error("Details should be nil when not set")
	}
}

func TestAPIErrorWithEmptyDetails(t *testing.T) {
	err := APIError{
		Code:    ErrCodeBadRequest,
		Message: "bad request",
		Details: map[string]any{},
	}

	if len(err.Details) != 0 {
		t.Error("Details should be empty")
	}
}
