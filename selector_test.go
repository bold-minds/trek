package trek

import "testing"

func TestMatchSelector(t *testing.T) {
	tests := []struct {
		name     string
		selector Selector
		ctx      RequestContext
		want     bool
	}{
		{
			name:     "empty selector matches everything",
			selector: Selector{},
			ctx:      RequestContext{UserID: "user-123", TenantID: "tenant-abc"},
			want:     true,
		},
		{
			name:     "exact user ID match",
			selector: Selector{UserID: "user-123"},
			ctx:      RequestContext{UserID: "user-123"},
			want:     true,
		},
		{
			name:     "user ID mismatch",
			selector: Selector{UserID: "user-123"},
			ctx:      RequestContext{UserID: "user-456"},
			want:     false,
		},
		{
			name:     "exact tenant ID match",
			selector: Selector{TenantID: "tenant-abc"},
			ctx:      RequestContext{TenantID: "tenant-abc"},
			want:     true,
		},
		{
			name:     "tenant ID mismatch",
			selector: Selector{TenantID: "tenant-abc"},
			ctx:      RequestContext{TenantID: "tenant-xyz"},
			want:     false,
		},
		{
			name:     "exact request ID match",
			selector: Selector{RequestID: "req-001"},
			ctx:      RequestContext{RequestID: "req-001"},
			want:     true,
		},
		{
			name:     "request ID mismatch",
			selector: Selector{RequestID: "req-001"},
			ctx:      RequestContext{RequestID: "req-002"},
			want:     false,
		},
		{
			name:     "exact route match",
			selector: Selector{Route: "/api/users"},
			ctx:      RequestContext{Route: "/api/users"},
			want:     true,
		},
		{
			name:     "route mismatch",
			selector: Selector{Route: "/api/users"},
			ctx:      RequestContext{Route: "/api/orders"},
			want:     false,
		},
		{
			name:     "route prefix match with wildcard",
			selector: Selector{Route: "/api/*"},
			ctx:      RequestContext{Route: "/api/users/123"},
			want:     true,
		},
		{
			name:     "route prefix no match",
			selector: Selector{Route: "/api/*"},
			ctx:      RequestContext{Route: "/admin/users"},
			want:     false,
		},
		{
			name:     "multiple fields all match (AND semantics)",
			selector: Selector{UserID: "user-123", TenantID: "tenant-abc"},
			ctx:      RequestContext{UserID: "user-123", TenantID: "tenant-abc"},
			want:     true,
		},
		{
			name:     "multiple fields partial match fails",
			selector: Selector{UserID: "user-123", TenantID: "tenant-abc"},
			ctx:      RequestContext{UserID: "user-123", TenantID: "tenant-xyz"},
			want:     false,
		},
		{
			name:     "custom fields match",
			selector: Selector{Custom: map[string]string{"env": "prod", "region": "us-east"}},
			ctx:      RequestContext{Custom: map[string]string{"env": "prod", "region": "us-east", "extra": "value"}},
			want:     true,
		},
		{
			name:     "custom fields partial match fails",
			selector: Selector{Custom: map[string]string{"env": "prod", "region": "us-east"}},
			ctx:      RequestContext{Custom: map[string]string{"env": "prod", "region": "eu-west"}},
			want:     false,
		},
		{
			name:     "custom field missing in context fails",
			selector: Selector{Custom: map[string]string{"env": "prod"}},
			ctx:      RequestContext{Custom: map[string]string{}},
			want:     false,
		},
		{
			name:     "empty selector custom with nil context custom matches",
			selector: Selector{Custom: map[string]string{}},
			ctx:      RequestContext{Custom: nil},
			want:     true,
		},
		{
			name:     "selector with custom on nil context custom fails",
			selector: Selector{Custom: map[string]string{"key": "value"}},
			ctx:      RequestContext{Custom: nil},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchSelector(tt.selector, tt.ctx)
			if got != tt.want {
				t.Errorf("MatchSelector() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchRoute(t *testing.T) {
	tests := []struct {
		name          string
		selectorRoute string
		requestRoute  string
		want          bool
	}{
		{
			name:          "exact match",
			selectorRoute: "/api/users",
			requestRoute:  "/api/users",
			want:          true,
		},
		{
			name:          "exact mismatch",
			selectorRoute: "/api/users",
			requestRoute:  "/api/orders",
			want:          false,
		},
		{
			name:          "prefix match with wildcard",
			selectorRoute: "/api/*",
			requestRoute:  "/api/users",
			want:          true,
		},
		{
			name:          "prefix match nested path",
			selectorRoute: "/api/*",
			requestRoute:  "/api/users/123/orders",
			want:          true,
		},
		{
			name:          "prefix match exact at boundary",
			selectorRoute: "/api/*",
			requestRoute:  "/api/",
			want:          true,
		},
		{
			name:          "prefix no match different base",
			selectorRoute: "/api/*",
			requestRoute:  "/admin/users",
			want:          false,
		},
		{
			name:          "wildcard at root",
			selectorRoute: "/*",
			requestRoute:  "/anything/here",
			want:          true,
		},
		{
			name:          "empty selector route matches empty request",
			selectorRoute: "",
			requestRoute:  "",
			want:          true,
		},
		{
			name:          "empty selector route does not match non-empty",
			selectorRoute: "",
			requestRoute:  "/api",
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchRoute(tt.selectorRoute, tt.requestRoute)
			if got != tt.want {
				t.Errorf("matchRoute(%q, %q) = %v, want %v", tt.selectorRoute, tt.requestRoute, got, tt.want)
			}
		})
	}
}

func TestMatchCustom(t *testing.T) {
	tests := []struct {
		name           string
		selectorCustom map[string]string
		requestCustom  map[string]string
		want           bool
	}{
		{
			name:           "both empty",
			selectorCustom: map[string]string{},
			requestCustom:  map[string]string{},
			want:           true,
		},
		{
			name:           "selector empty request has values",
			selectorCustom: map[string]string{},
			requestCustom:  map[string]string{"key": "value"},
			want:           true,
		},
		{
			name:           "exact match single key",
			selectorCustom: map[string]string{"env": "prod"},
			requestCustom:  map[string]string{"env": "prod"},
			want:           true,
		},
		{
			name:           "exact match multiple keys",
			selectorCustom: map[string]string{"env": "prod", "region": "us-east"},
			requestCustom:  map[string]string{"env": "prod", "region": "us-east"},
			want:           true,
		},
		{
			name:           "request has extra keys",
			selectorCustom: map[string]string{"env": "prod"},
			requestCustom:  map[string]string{"env": "prod", "region": "us-east"},
			want:           true,
		},
		{
			name:           "value mismatch",
			selectorCustom: map[string]string{"env": "prod"},
			requestCustom:  map[string]string{"env": "staging"},
			want:           false,
		},
		{
			name:           "key missing in request",
			selectorCustom: map[string]string{"env": "prod"},
			requestCustom:  map[string]string{"region": "us-east"},
			want:           false,
		},
		{
			name:           "request nil with non-empty selector",
			selectorCustom: map[string]string{"env": "prod"},
			requestCustom:  nil,
			want:           false,
		},
		{
			name:           "request nil with empty selector",
			selectorCustom: map[string]string{},
			requestCustom:  nil,
			want:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchCustom(tt.selectorCustom, tt.requestCustom)
			if got != tt.want {
				t.Errorf("matchCustom() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelectorSpecificity(t *testing.T) {
	tests := []struct {
		name     string
		selector Selector
		want     int
	}{
		{
			name:     "empty selector",
			selector: Selector{},
			want:     0,
		},
		{
			name:     "user ID only",
			selector: Selector{UserID: "user-123"},
			want:     1,
		},
		{
			name:     "tenant ID only",
			selector: Selector{TenantID: "tenant-abc"},
			want:     1,
		},
		{
			name:     "request ID only",
			selector: Selector{RequestID: "req-001"},
			want:     1,
		},
		{
			name:     "route only",
			selector: Selector{Route: "/api/users"},
			want:     1,
		},
		{
			name:     "custom only with one key",
			selector: Selector{Custom: map[string]string{"env": "prod"}},
			want:     1,
		},
		{
			name:     "custom only with multiple keys counts as 1",
			selector: Selector{Custom: map[string]string{"env": "prod", "region": "us-east"}},
			want:     1,
		},
		{
			name:     "two fields",
			selector: Selector{UserID: "user-123", TenantID: "tenant-abc"},
			want:     2,
		},
		{
			name:     "all fields",
			selector: Selector{UserID: "u", TenantID: "t", RequestID: "r", Route: "/", Custom: map[string]string{"k": "v"}},
			want:     5,
		},
		{
			name:     "empty custom map does not count",
			selector: Selector{UserID: "user-123", Custom: map[string]string{}},
			want:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SelectorSpecificity(tt.selector)
			if got != tt.want {
				t.Errorf("SelectorSpecificity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsEmptySelector(t *testing.T) {
	tests := []struct {
		name     string
		selector Selector
		want     bool
	}{
		{
			name:     "empty selector",
			selector: Selector{},
			want:     true,
		},
		{
			name:     "selector with user ID",
			selector: Selector{UserID: "user-123"},
			want:     false,
		},
		{
			name:     "selector with empty custom map",
			selector: Selector{Custom: map[string]string{}},
			want:     true,
		},
		{
			name:     "selector with non-empty custom map",
			selector: Selector{Custom: map[string]string{"k": "v"}},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsEmptySelector(tt.selector)
			if got != tt.want {
				t.Errorf("IsEmptySelector() = %v, want %v", got, tt.want)
			}
		})
	}
}
