package api

import (
	"testing"
)

func TestNewCapsHandler(t *testing.T) {
	handler := NewCapsHandler(nil)

	if handler == nil {
		t.Fatal("NewCapsHandler() should not return nil")
	}
}

func TestNewCapsHandler_WithNilCounter(t *testing.T) {
	handler := NewCapsHandler(nil)

	if handler.counter != nil {
		t.Error("counter should be nil when initialized with nil")
	}
}

func TestCheckCapsRequest(t *testing.T) {
	req := CheckCapsRequest{
		SessionID:        "sess-123",
		RequestID:        "req-456",
		MaxPerSession:    100,
		MaxPerRequest:    50,
		IncrementSession: true,
		IncrementRequest: false,
	}

	if req.SessionID != "sess-123" {
		t.Errorf("SessionID = %q, want %q", req.SessionID, "sess-123")
	}
	if req.RequestID != "req-456" {
		t.Errorf("RequestID = %q, want %q", req.RequestID, "req-456")
	}
	if req.MaxPerSession != 100 {
		t.Errorf("MaxPerSession = %d, want %d", req.MaxPerSession, 100)
	}
	if req.MaxPerRequest != 50 {
		t.Errorf("MaxPerRequest = %d, want %d", req.MaxPerRequest, 50)
	}
	if !req.IncrementSession {
		t.Error("IncrementSession should be true")
	}
	if req.IncrementRequest {
		t.Error("IncrementRequest should be false")
	}
}

func TestCheckCapsRequest_Defaults(t *testing.T) {
	req := CheckCapsRequest{}

	if req.SessionID != "" {
		t.Error("SessionID default should be empty")
	}
	if req.RequestID != "" {
		t.Error("RequestID default should be empty")
	}
	if req.MaxPerSession != 0 {
		t.Errorf("MaxPerSession default = %d, want 0", req.MaxPerSession)
	}
	if req.MaxPerRequest != 0 {
		t.Errorf("MaxPerRequest default = %d, want 0", req.MaxPerRequest)
	}
	if req.IncrementSession {
		t.Error("IncrementSession default should be false")
	}
	if req.IncrementRequest {
		t.Error("IncrementRequest default should be false")
	}
}
