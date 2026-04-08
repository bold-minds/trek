// Package trek provides targeted, time-bounded debug logging for Go applications.
package trek

import "time"

// RequestContext contains the extracted context from an incoming request.
// The SDK middleware populates this from headers, auth claims, and route info.
type RequestContext struct {
	UserID    string            `json:"user_id"`
	RequestID string            `json:"request_id"`
	TenantID  string            `json:"tenant_id"`
	Route     string            `json:"route"`
	Custom    map[string]string `json:"custom"`
}

// Selector defines the match conditions for a debug session.
// All specified fields must match (AND semantics).
type Selector struct {
	UserID    string            `json:"user_id,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
	TenantID  string            `json:"tenant_id,omitempty"`
	Route     string            `json:"route,omitempty"`
	Custom    map[string]string `json:"custom,omitempty"`
}

// Caps defines rate limiting for debug log events.
type Caps struct {
	MaxDebugEventsPerRequest int `json:"max_debug_events_per_request,omitempty"`
	MaxDebugEventsPerSession int `json:"max_debug_events_per_session,omitempty"`
}

// Session represents a debug session that targets specific traffic.
type Session struct {
	ID           string            `json:"id"`
	Selector     Selector          `json:"selector"`
	Level        Level             `json:"level"`
	ExpiresAt    time.Time         `json:"expires_at"`
	Caps         Caps              `json:"caps"`
	Labels       map[string]string `json:"labels"`
	ServiceScope []string          `json:"service_scope"`
}

// Decision is the result of evaluating a request against active sessions.
type Decision struct {
	Matched        bool              `json:"matched"`
	SessionID      string            `json:"session_id"`
	EffectiveLevel Level             `json:"effective_level"`
	ReasonCode     ReasonCode        `json:"reason_code"`
	Labels         map[string]string `json:"labels"`
	Caps           Caps              `json:"caps"`
}

// Level represents logging verbosity levels.
type Level string

const (
	LevelInfo  Level = "info"
	LevelDebug Level = "debug"
	LevelTrace Level = "trace"
)

// LevelPriority returns the priority of a level for tie-breaking.
// Higher priority = more verbose.
func (l Level) Priority() int {
	switch l {
	case LevelTrace:
		return 3
	case LevelDebug:
		return 2
	case LevelInfo:
		return 1
	default:
		return 0
	}
}

// ReasonCode indicates why a particular decision was made.
type ReasonCode string

const (
	ReasonMatched ReasonCode = "MATCHED"
	ReasonNoMatch ReasonCode = "NO_MATCH"
	ReasonExpired ReasonCode = "EXPIRED"
)

// NoMatchDecision returns a decision indicating no session matched.
func NoMatchDecision(reason ReasonCode) Decision {
	if reason == "" {
		reason = ReasonNoMatch
	}
	return Decision{
		Matched:        false,
		SessionID:      "",
		EffectiveLevel: LevelInfo,
		ReasonCode:     reason,
		Labels:         map[string]string{},
		Caps:           Caps{},
	}
}

// ActiveSessionsResponse is the response from the control plane's active-sessions endpoint.
type ActiveSessionsResponse struct {
	Revision   string    `json:"revision"`
	ServerTime time.Time `json:"server_time"`
	Sessions   []Session `json:"sessions"`
}

// CreateSessionRequest is the request body for creating a new debug session.
type CreateSessionRequest struct {
	Selector     Selector          `json:"selector"`
	Level        Level             `json:"level"`
	TTLSeconds   int               `json:"ttl_seconds,omitempty"`
	ExpiresAt    *time.Time        `json:"expires_at,omitempty"`
	Caps         *Caps             `json:"caps,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	ServiceScope []string          `json:"service_scope,omitempty"`
	Reason       string            `json:"reason"`
}

// CreateSessionResponse is the response after creating a debug session.
type CreateSessionResponse struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expires_at"`
}

// SessionStatus represents the lifecycle status of a session.
type SessionStatus string

const (
	SessionStatusActive  SessionStatus = "active"
	SessionStatusRevoked SessionStatus = "revoked"
	SessionStatusExpired SessionStatus = "expired"
)
