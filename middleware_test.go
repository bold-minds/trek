package trek

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDefaultExtractor(t *testing.T) {
	extractor := DefaultExtractor()

	if extractor.UserIDHeader != "X-User-ID" {
		t.Errorf("UserIDHeader = %q, want %q", extractor.UserIDHeader, "X-User-ID")
	}
	if extractor.RequestIDHeader != "X-Request-ID" {
		t.Errorf("RequestIDHeader = %q, want %q", extractor.RequestIDHeader, "X-Request-ID")
	}
	if extractor.TenantIDHeader != "X-Tenant-ID" {
		t.Errorf("TenantIDHeader = %q, want %q", extractor.TenantIDHeader, "X-Tenant-ID")
	}
}

func TestContextExtractor_Extract(t *testing.T) {
	tests := []struct {
		name      string
		extractor ContextExtractor
		headers   map[string]string
		path      string
		want      RequestContext
	}{
		{
			name:      "extracts all standard headers",
			extractor: DefaultExtractor(),
			headers: map[string]string{
				"X-User-ID":    "user-123",
				"X-Request-ID": "req-456",
				"X-Tenant-ID":  "tenant-789",
			},
			path: "/api/users",
			want: RequestContext{
				UserID:    "user-123",
				RequestID: "req-456",
				TenantID:  "tenant-789",
				Route:     "/api/users",
				Custom:    map[string]string{},
			},
		},
		{
			name:      "handles missing headers",
			extractor: DefaultExtractor(),
			headers:   map[string]string{},
			path:      "/api/orders",
			want: RequestContext{
				UserID:    "",
				RequestID: "",
				TenantID:  "",
				Route:     "/api/orders",
				Custom:    map[string]string{},
			},
		},
		{
			name: "extracts custom headers",
			extractor: ContextExtractor{
				UserIDHeader:  "X-User-ID",
				CustomHeaders: map[string]string{"env": "X-Environment", "region": "X-Region"},
			},
			headers: map[string]string{
				"X-User-ID":     "user-123",
				"X-Environment": "production",
				"X-Region":      "us-east-1",
			},
			path: "/api/items",
			want: RequestContext{
				UserID:    "user-123",
				RequestID: "",
				TenantID:  "",
				Route:     "/api/items",
				Custom:    map[string]string{"env": "production", "region": "us-east-1"},
			},
		},
		{
			name: "ignores missing custom headers",
			extractor: ContextExtractor{
				CustomHeaders: map[string]string{"env": "X-Environment"},
			},
			headers: map[string]string{},
			path:    "/api/test",
			want: RequestContext{
				Route:  "/api/test",
				Custom: map[string]string{},
			},
		},
		{
			name: "empty extractor config",
			extractor: ContextExtractor{
				UserIDHeader:    "",
				RequestIDHeader: "",
				TenantIDHeader:  "",
			},
			headers: map[string]string{
				"X-User-ID": "user-123",
			},
			path: "/api/empty",
			want: RequestContext{
				UserID:    "",
				RequestID: "",
				TenantID:  "",
				Route:     "/api/empty",
				Custom:    map[string]string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			got := tt.extractor.Extract(req)

			if got.UserID != tt.want.UserID {
				t.Errorf("UserID = %q, want %q", got.UserID, tt.want.UserID)
			}
			if got.RequestID != tt.want.RequestID {
				t.Errorf("RequestID = %q, want %q", got.RequestID, tt.want.RequestID)
			}
			if got.TenantID != tt.want.TenantID {
				t.Errorf("TenantID = %q, want %q", got.TenantID, tt.want.TenantID)
			}
			if got.Route != tt.want.Route {
				t.Errorf("Route = %q, want %q", got.Route, tt.want.Route)
			}
			if len(got.Custom) != len(tt.want.Custom) {
				t.Errorf("len(Custom) = %d, want %d", len(got.Custom), len(tt.want.Custom))
			}
			for k, v := range tt.want.Custom {
				if got.Custom[k] != v {
					t.Errorf("Custom[%q] = %q, want %q", k, got.Custom[k], v)
				}
			}
		})
	}
}

func TestRouteTemplate(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		headers map[string]string
		want    string
	}{
		{
			name: "uses X-Route-Pattern header if present",
			path: "/api/users/123",
			headers: map[string]string{
				"X-Route-Pattern": "/api/users/:id",
			},
			want: "/api/users/:id",
		},
		{
			name: "normalizes numeric IDs",
			path: "/api/users/12345/orders",
			want: "/api/users/:id/orders",
		},
		{
			name: "normalizes UUIDs",
			path: "/api/users/550e8400-e29b-41d4-a716-446655440000",
			want: "/api/users/:id",
		},
		{
			name: "keeps short numeric segments",
			path: "/api/v1/users",
			want: "/api/v1/users",
		},
		{
			name: "handles root path",
			path: "/",
			want: "/",
		},
		{
			name: "handles empty path",
			path: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com"+tt.path, nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			got := RouteTemplate(req)
			if got != tt.want {
				t.Errorf("RouteTemplate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizePathParams(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/api/users/12345", "/api/users/:id"},
		{"/api/users/550e8400-e29b-41d4-a716-446655440000", "/api/users/:id"},
		{"/api/v1/items", "/api/v1/items"},
		{"/api/users/ab", "/api/users/ab"},
		{"/", "/"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := normalizePathParams(tt.path)
			if got != tt.want {
				t.Errorf("normalizePathParams(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestLooksLikeID(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"12345", true},
		{"550e8400-e29b-41d4-a716-446655440000", true},
		{"users", false},
		{"v1", false},
		{"ab", false},
		{"12", false},
		{"", false},
		{"123", true},
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			got := looksLikeID(tt.s)
			if got != tt.want {
				t.Errorf("looksLikeID(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"12345", true},
		{"0", true},
		{"123abc", false},
		{"abc", false},
		{"", true},
		{"-123", false},
		{"12.34", false},
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			got := isNumeric(tt.s)
			if got != tt.want {
				t.Errorf("isNumeric(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestIsUUID(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"550e8400-e29b-41d4-a716-446655440000", true},
		{"00000000-0000-0000-0000-000000000000", true},
		{"not-a-uuid", false},
		{"550e8400e29b41d4a716446655440000", false},
		{"550e8400-e29b-41d4-a716-44665544000", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			got := isUUID(tt.s)
			if got != tt.want {
				t.Errorf("isUUID(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

type mockSessionCache struct {
	sessions    []Session
	clockOffset time.Duration
}

func (m *mockSessionCache) GetSessions() []Session {
	return m.sessions
}

func (m *mockSessionCache) ClockOffset() time.Duration {
	return m.clockOffset
}

func TestMiddleware(t *testing.T) {
	t.Run("passes request through with no sessions", func(t *testing.T) {
		cache := &mockSessionCache{sessions: []Session{}}
		extractor := DefaultExtractor()

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			decision := DecisionFromContext(r.Context())
			if decision == nil {
				t.Error("DecisionFromContext returned nil")
				return
			}
			if decision.Matched {
				t.Error("expected no match with empty sessions")
			}
			w.WriteHeader(http.StatusOK)
		})

		middleware := Middleware(cache, "test-service", extractor)
		req := httptest.NewRequest("GET", "/api/users", nil)
		rec := httptest.NewRecorder()

		middleware(handler).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("sets decision in context on match", func(t *testing.T) {
		cache := &mockSessionCache{
			sessions: []Session{
				{
					ID:        "sess-1",
					Selector:  Selector{UserID: "user-123"},
					Level:     LevelDebug,
					ExpiresAt: time.Now().Add(time.Hour),
				},
			},
		}
		extractor := DefaultExtractor()

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			decision := DecisionFromContext(r.Context())
			if decision == nil {
				t.Error("DecisionFromContext returned nil")
				return
			}
			if !decision.Matched {
				t.Error("expected match")
			}
			if decision.SessionID != "sess-1" {
				t.Errorf("SessionID = %q, want %q", decision.SessionID, "sess-1")
			}
			w.WriteHeader(http.StatusOK)
		})

		middleware := Middleware(cache, "test-service", extractor)
		req := httptest.NewRequest("GET", "/api/users", nil)
		req.Header.Set("X-User-ID", "user-123")
		rec := httptest.NewRecorder()

		middleware(handler).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})
}
