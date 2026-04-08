package store

import (
	"context"
	"time"

	"github.com/bold-minds/trek"
)

// Store defines the interface for the Trek data store.
type Store interface {
	SessionStore
	PolicyStore
	AuditStore
	TokenStore
	OrgStore
	UserStore
	NotificationStore
	HealthStore
}

// HealthStore provides health check methods.
type HealthStore interface {
	// Ping verifies the database connection is alive.
	Ping(ctx context.Context) error
}

// SessionStore manages debug sessions.
type SessionStore interface {
	CreateSession(ctx context.Context, session *Session) error
	GetSession(ctx context.Context, orgID, envID, sessionID string) (*Session, error)
	ListSessions(ctx context.Context, orgID, envID string, filter SessionFilter) ([]Session, error)
	GetActiveSessions(ctx context.Context, orgID, envID, serviceName string) ([]trek.Session, error)
	RevokeSession(ctx context.Context, orgID, envID, sessionID string) error
	ExpireSessions(ctx context.Context) (int, error)
}

// PolicyStore manages org/env policies.
type PolicyStore interface {
	GetPolicy(ctx context.Context, orgID, envID string) (*Policy, error)
	SetPolicy(ctx context.Context, policy *Policy) error
}

// AuditStore manages audit events.
type AuditStore interface {
	CreateAuditEvent(ctx context.Context, event *AuditEvent) error
	ListAuditEvents(ctx context.Context, orgID, envID string, limit int, cursor string) ([]AuditEvent, string, error)
}

// TokenStore manages service tokens.
type TokenStore interface {
	ValidateToken(ctx context.Context, tokenHash string) (*ServiceToken, error)
	ValidateTokenPlain(ctx context.Context, plainToken string) (*ServiceToken, error)
	CreateToken(ctx context.Context, token *ServiceToken) error
	RevokeToken(ctx context.Context, orgID, envID, tokenID string) error
	ListTokens(ctx context.Context, orgID, envID string) ([]ServiceToken, error)
}

// OrgStore manages organizations and environments.
type OrgStore interface {
	GetOrg(ctx context.Context, orgID string) (*Org, error)
	GetEnv(ctx context.Context, orgID, envID string) (*Env, error)
	ListEnvs(ctx context.Context, orgID string) ([]Env, error)
	CreateEnv(ctx context.Context, env *Env) error
	DeleteEnv(ctx context.Context, orgID, envID string) error
}

// UserStore manages users and roles.
type UserStore interface {
	ListUsers(ctx context.Context, orgID string) ([]User, error)
	CreateUser(ctx context.Context, user *User) error
	DeleteUser(ctx context.Context, orgID, userID string) error
	ListRoles(ctx context.Context, orgID string) ([]Role, error)
	CreateRole(ctx context.Context, role *Role) error
	AssignRole(ctx context.Context, orgID, userID, roleID, envID string) error
	RevokeRole(ctx context.Context, orgID, userID, roleID, envID string) error
	GetUserRoles(ctx context.Context, orgID, userID string) ([]UserRole, error)
	GetUserPermissions(ctx context.Context, orgID, userID, envID string) ([]string, error)
}

// NotificationStore manages notification configurations.
type NotificationStore interface {
	ListNotificationConfigs(ctx context.Context, orgID, envID string) ([]NotificationConfig, error)
	CreateNotificationConfig(ctx context.Context, config *NotificationConfig) error
	UpdateNotificationConfig(ctx context.Context, orgID, configID string, config map[string]any, enabled *bool) error
	DeleteNotificationConfig(ctx context.Context, orgID, configID string) error
}

// Session is the database representation of a debug session.
type Session struct {
	ID           string            `json:"id"`
	OrgID        string            `json:"org_id"`
	EnvID        string            `json:"env_id"`
	Status       string            `json:"status"`
	Selector     trek.Selector     `json:"selector"`
	Level        trek.Level        `json:"level"`
	ExpiresAt    time.Time         `json:"expires_at"`
	Caps         trek.Caps         `json:"caps"`
	Labels       map[string]string `json:"labels,omitempty"`
	ServiceScope []string          `json:"service_scope,omitempty"`
	CreatedBy    string            `json:"created_by"`
	Reason       string            `json:"reason,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	RevokedAt    *time.Time        `json:"revoked_at,omitempty"`
}

// ToSDK converts a store Session to an SDK Session.
func (s *Session) ToSDK() trek.Session {
	return trek.Session{
		ID:           s.ID,
		Selector:     s.Selector,
		Level:        s.Level,
		ExpiresAt:    s.ExpiresAt,
		Caps:         s.Caps,
		Labels:       s.Labels,
		ServiceScope: s.ServiceScope,
	}
}

// SessionFilter filters session queries.
type SessionFilter struct {
	Status string `json:"status,omitempty"`
}

// Policy represents org/env policy settings.
type Policy struct {
	ID                  string    `json:"id"`
	OrgID               string    `json:"org_id"`
	EnvID               string    `json:"env_id"`
	MaxTTLSeconds       int       `json:"max_ttl_seconds"`
	AllowEmptySelector  bool      `json:"allow_empty_selector"`
	AllowedSelectorKeys []string  `json:"allowed_selector_keys,omitempty"`
	DefaultCaps         trek.Caps `json:"default_caps"`
	RequireReason       bool      `json:"require_reason"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// AuditEvent represents an audit log entry.
type AuditEvent struct {
	ID           string         `json:"id"`
	OrgID        string         `json:"org_id"`
	EnvID        string         `json:"env_id"`
	ActorUserID  string         `json:"actor_user_id,omitempty"`
	ActorTokenID string         `json:"actor_token_id,omitempty"`
	Action       string         `json:"action"`
	TargetType   string         `json:"target_type"`
	TargetID     string         `json:"target_id"`
	Payload      map[string]any `json:"payload,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

// ServiceToken represents a service authentication token.
type ServiceToken struct {
	ID               string     `json:"id"`
	OrgID            string     `json:"org_id"`
	EnvID            string     `json:"env_id"`
	TokenHash        string     `json:"-"`
	Name             string     `json:"name"`
	ServiceAllowlist []string   `json:"service_allowlist,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
}

// Org represents an organization.
type Org struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// Env represents an environment.
type Env struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// User represents a user in the system.
type User struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	OIDCSubject string    `json:"oidc_subject"`
	Email       string    `json:"email"`
	CreatedAt   time.Time `json:"created_at"`
}

// Role represents a role with permissions.
type Role struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	Name        string    `json:"name"`
	Permissions []string  `json:"permissions"`
	CreatedAt   time.Time `json:"created_at"`
}

// UserRole represents a user's role assignment.
type UserRole struct {
	UserID    string    `json:"user_id"`
	RoleID    string    `json:"role_id"`
	RoleName  string    `json:"role_name"`
	EnvID     string    `json:"env_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// NotificationConfig represents a notification configuration.
type NotificationConfig struct {
	ID        string         `json:"id"`
	OrgID     string         `json:"org_id"`
	EnvID     string         `json:"env_id,omitempty"`
	Type      string         `json:"type"`
	Config    map[string]any `json:"config"`
	Enabled   bool           `json:"enabled"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}
