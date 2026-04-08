package trek

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsEnabled_NotInitialized(t *testing.T) {
	// Ensure global instance is nil
	globalMu.Lock()
	oldInstance := globalInstance
	globalInstance = nil
	globalMu.Unlock()

	defer func() {
		globalMu.Lock()
		globalInstance = oldInstance
		globalMu.Unlock()
	}()

	if IsEnabled() {
		t.Error("IsEnabled() should return false when not initialized")
	}
}

func TestAPIClient_NotInitialized(t *testing.T) {
	globalMu.Lock()
	oldInstance := globalInstance
	globalInstance = nil
	globalMu.Unlock()

	defer func() {
		globalMu.Lock()
		globalInstance = oldInstance
		globalMu.Unlock()
	}()

	client := APIClient()
	if client != nil {
		t.Error("APIClient() should return nil when not initialized")
	}
}

func TestGlobalCache_NotInitialized(t *testing.T) {
	globalMu.Lock()
	oldInstance := globalInstance
	globalInstance = nil
	globalMu.Unlock()

	defer func() {
		globalMu.Lock()
		globalInstance = oldInstance
		globalMu.Unlock()
	}()

	cache := GlobalCache()
	if cache != nil {
		t.Error("GlobalCache() should return nil when not initialized")
	}
}

func TestHTTPMiddleware_NotInitialized(t *testing.T) {
	globalMu.Lock()
	oldInstance := globalInstance
	globalInstance = nil
	globalMu.Unlock()

	defer func() {
		globalMu.Lock()
		globalInstance = oldInstance
		globalMu.Unlock()
	}()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := HTTPMiddleware(handler)

	// Should return the original handler when not initialized
	if wrapped == nil {
		t.Fatal("HTTPMiddleware() should not return nil")
	}

	// Test that it still works
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestShutdown_NotInitialized(t *testing.T) {
	globalMu.Lock()
	oldInstance := globalInstance
	globalInstance = nil
	globalMu.Unlock()

	defer func() {
		globalMu.Lock()
		globalInstance = oldInstance
		globalMu.Unlock()
	}()

	// Should not panic
	Shutdown()
}

func TestSetExtractor_NotInitialized(t *testing.T) {
	globalMu.Lock()
	oldInstance := globalInstance
	globalInstance = nil
	globalMu.Unlock()

	defer func() {
		globalMu.Lock()
		globalInstance = oldInstance
		globalMu.Unlock()
	}()

	extractor := ContextExtractor{
		UserIDHeader: "X-Test-User",
	}

	// Should not panic
	SetExtractor(extractor)
}

func TestInit_InvalidConfig(t *testing.T) {
	// Empty config should fail validation
	cfg := Config{}

	err := Init(cfg)

	// With StrictInit false (default), should not return error
	if err != nil {
		t.Errorf("Init() with invalid config and StrictInit=false should not error, got: %v", err)
	}
}

func TestInit_InvalidConfigStrict(t *testing.T) {
	cfg := Config{
		StrictInit: true,
	}

	err := Init(cfg)

	// With StrictInit true, should return error
	if err == nil {
		t.Error("Init() with invalid config and StrictInit=true should return error")
	}
}

func TestWrapDefaultHandler_NotInitialized(t *testing.T) {
	globalMu.Lock()
	oldInstance := globalInstance
	globalInstance = nil
	globalMu.Unlock()

	defer func() {
		globalMu.Lock()
		globalInstance = oldInstance
		globalMu.Unlock()
	}()

	handler := WrapDefaultHandler(nil)

	if handler == nil {
		t.Fatal("WrapDefaultHandler() should not return nil")
	}
}

func TestInstance_Struct(t *testing.T) {
	instance := &Instance{
		config:     Config{OrgID: "test-org"},
		client:     nil,
		cache:      nil,
		extractor:  ContextExtractor{},
		globalCaps: nil,
	}

	if instance.config.OrgID != "test-org" {
		t.Errorf("config.OrgID = %q, want %q", instance.config.OrgID, "test-org")
	}
}
