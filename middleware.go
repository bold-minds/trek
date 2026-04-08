package trek

import (
	"net/http"
	"strings"
	"time"
)

// ContextExtractor extracts request context from an HTTP request.
type ContextExtractor struct {
	UserIDHeader    string
	RequestIDHeader string
	TenantIDHeader  string
	CustomHeaders   map[string]string
}

// DefaultExtractor returns a ContextExtractor with common header mappings.
func DefaultExtractor() ContextExtractor {
	return ContextExtractor{
		UserIDHeader:    "X-User-ID",
		RequestIDHeader: "X-Request-ID",
		TenantIDHeader:  "X-Tenant-ID",
		CustomHeaders:   map[string]string{},
	}
}

// Extract extracts a RequestContext from an HTTP request.
func (e ContextExtractor) Extract(r *http.Request) RequestContext {
	ctx := RequestContext{
		Route:  r.URL.Path,
		Custom: make(map[string]string),
	}

	if e.UserIDHeader != "" {
		ctx.UserID = r.Header.Get(e.UserIDHeader)
	}

	if e.RequestIDHeader != "" {
		ctx.RequestID = r.Header.Get(e.RequestIDHeader)
	}

	if e.TenantIDHeader != "" {
		ctx.TenantID = r.Header.Get(e.TenantIDHeader)
	}

	for customKey, headerName := range e.CustomHeaders {
		if val := r.Header.Get(headerName); val != "" {
			ctx.Custom[customKey] = val
		}
	}

	return ctx
}

// MiddlewareConfig configures the Trek HTTP middleware.
type MiddlewareConfig struct {
	Extractor    ContextExtractor
	SessionCache SessionCache
	ServiceName  string
	ClockOffset  func() time.Duration
}

// SessionCache is the interface for accessing cached sessions.
type SessionCache interface {
	GetSessions() []Session
	ClockOffset() time.Duration
}

// Middleware returns an HTTP middleware that evaluates Trek sessions for each request.
func Middleware(cache SessionCache, serviceName string, extractor ContextExtractor) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			reqCtx := extractor.Extract(r)
			sessions := cache.GetSessions()

			now := time.Now().Add(cache.ClockOffset())
			decision := Decide(now, serviceName, reqCtx, sessions)

			// Record metrics
			metrics := GetMetrics()
			metrics.IncrRequestsTotal()
			if decision.Matched {
				metrics.IncrRequestsMatched()
			}
			metrics.RecordDecisionLatency(time.Since(start).Microseconds())

			ctx := ContextWithDecision(r.Context(), &decision)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// MiddlewareFunc is a convenience wrapper that returns http.HandlerFunc.
func MiddlewareFunc(cache SessionCache, serviceName string, extractor ContextExtractor, handler http.HandlerFunc) http.HandlerFunc {
	m := Middleware(cache, serviceName, extractor)
	return m(handler).ServeHTTP
}

// RouteTemplate attempts to extract a route template from common router patterns.
// This is a best-effort helper; for accurate templates, configure your router to provide them.
func RouteTemplate(r *http.Request) string {
	if pattern := r.Header.Get("X-Route-Pattern"); pattern != "" {
		return pattern
	}

	path := r.URL.Path
	path = normalizePathParams(path)

	return path
}

// normalizePathParams replaces common ID patterns with placeholders.
func normalizePathParams(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if looksLikeID(part) {
			parts[i] = ":id"
		}
	}
	return strings.Join(parts, "/")
}

// looksLikeID returns true if a path segment looks like an ID.
func looksLikeID(s string) bool {
	if len(s) == 0 {
		return false
	}

	if isNumeric(s) && len(s) > 2 {
		return true
	}

	if isUUID(s) {
		return true
	}

	return false
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return false
	}
	return true
}
