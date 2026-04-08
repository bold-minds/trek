package store

import (
	"testing"
	"time"
)

func TestEnvStruct(t *testing.T) {
	env := Env{
		ID:        "env-123",
		OrgID:     "org-456",
		Name:      "production",
		CreatedAt: time.Now(),
	}

	if env.ID == "" {
		t.Error("Env.ID should not be empty")
	}
	if env.OrgID == "" {
		t.Error("Env.OrgID should not be empty")
	}
	if env.Name == "" {
		t.Error("Env.Name should not be empty")
	}
	if env.CreatedAt.IsZero() {
		t.Error("Env.CreatedAt should not be zero")
	}
}

func TestUserStruct(t *testing.T) {
	user := User{
		ID:          "user-123",
		OrgID:       "org-456",
		OIDCSubject: "oidc|123456",
		Email:       "test@example.com",
		CreatedAt:   time.Now(),
	}

	if user.ID == "" {
		t.Error("User.ID should not be empty")
	}
	if user.OrgID == "" {
		t.Error("User.OrgID should not be empty")
	}
	if user.OIDCSubject == "" {
		t.Error("User.OIDCSubject should not be empty")
	}
	if user.Email == "" {
		t.Error("User.Email should not be empty")
	}
}

func TestRoleStruct(t *testing.T) {
	role := Role{
		ID:          "role-123",
		OrgID:       "org-456",
		Name:        "admin",
		Permissions: []string{"sessions:create", "sessions:revoke"},
		CreatedAt:   time.Now(),
	}

	if role.ID == "" {
		t.Error("Role.ID should not be empty")
	}
	if role.Name == "" {
		t.Error("Role.Name should not be empty")
	}
	if len(role.Permissions) != 2 {
		t.Errorf("Role.Permissions length = %d, want %d", len(role.Permissions), 2)
	}
}

func TestUserRoleStruct(t *testing.T) {
	userRole := UserRole{
		UserID:    "user-123",
		RoleID:    "role-456",
		RoleName:  "admin",
		EnvID:     "env-789",
		CreatedAt: time.Now(),
	}

	if userRole.UserID == "" {
		t.Error("UserRole.UserID should not be empty")
	}
	if userRole.RoleID == "" {
		t.Error("UserRole.RoleID should not be empty")
	}
	if userRole.RoleName == "" {
		t.Error("UserRole.RoleName should not be empty")
	}
}

func TestUserRoleStruct_OrgWide(t *testing.T) {
	// Org-wide role (no specific env)
	userRole := UserRole{
		UserID:    "user-123",
		RoleID:    "role-456",
		RoleName:  "viewer",
		EnvID:     "", // Empty means org-wide
		CreatedAt: time.Now(),
	}

	if userRole.EnvID != "" {
		t.Error("Org-wide UserRole.EnvID should be empty")
	}
}

func TestNotificationConfigStruct(t *testing.T) {
	config := NotificationConfig{
		ID:    "config-123",
		OrgID: "org-456",
		EnvID: "env-789",
		Type:  "slack",
		Config: map[string]any{
			"webhook_url": "https://hooks.slack.com/xxx",
			"channel":     "#alerts",
		},
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if config.ID == "" {
		t.Error("NotificationConfig.ID should not be empty")
	}
	if config.Type == "" {
		t.Error("NotificationConfig.Type should not be empty")
	}
	if config.Config == nil {
		t.Error("NotificationConfig.Config should not be nil")
	}
	if !config.Enabled {
		t.Error("NotificationConfig.Enabled should be true")
	}
}

func TestNotificationConfigTypes(t *testing.T) {
	validTypes := []string{"slack", "webhook", "email"}

	for _, typ := range validTypes {
		config := NotificationConfig{
			Type: typ,
		}
		if config.Type == "" {
			t.Errorf("NotificationConfig should accept type %q", typ)
		}
	}
}

func TestOrgStruct(t *testing.T) {
	org := Org{
		ID:        "org-123",
		Name:      "Acme Corp",
		CreatedAt: time.Now(),
	}

	if org.ID == "" {
		t.Error("Org.ID should not be empty")
	}
	if org.Name == "" {
		t.Error("Org.Name should not be empty")
	}
	if org.CreatedAt.IsZero() {
		t.Error("Org.CreatedAt should not be zero")
	}
}

func TestServiceTokenStruct(t *testing.T) {
	token := ServiceToken{
		ID:               "tok-123",
		OrgID:            "org-456",
		EnvID:            "env-789",
		TokenHash:        "hashed-value",
		Name:             "api-gateway-token",
		ServiceAllowlist: []string{"api-gateway", "web-frontend"},
		CreatedAt:        time.Now(),
		RevokedAt:        nil,
	}

	if token.ID == "" {
		t.Error("ServiceToken.ID should not be empty")
	}
	if token.Name == "" {
		t.Error("ServiceToken.Name should not be empty")
	}
	if len(token.ServiceAllowlist) != 2 {
		t.Errorf("ServiceToken.ServiceAllowlist length = %d, want %d",
			len(token.ServiceAllowlist), 2)
	}
	if token.RevokedAt != nil {
		t.Error("ServiceToken.RevokedAt should be nil for active token")
	}
}

func TestServiceTokenStruct_Revoked(t *testing.T) {
	revokedAt := time.Now()
	token := ServiceToken{
		ID:        "tok-123",
		OrgID:     "org-456",
		EnvID:     "env-789",
		TokenHash: "hashed-value",
		Name:      "revoked-token",
		CreatedAt: time.Now().Add(-24 * time.Hour),
		RevokedAt: &revokedAt,
	}

	if token.RevokedAt == nil {
		t.Error("Revoked ServiceToken.RevokedAt should not be nil")
	}
}

func TestAuditEventStruct(t *testing.T) {
	event := AuditEvent{
		ID:           "evt-123",
		OrgID:        "org-456",
		EnvID:        "env-789",
		ActorUserID:  "user-abc",
		ActorTokenID: "",
		Action:       "session.create",
		TargetType:   "session",
		TargetID:     "sess-xyz",
		Payload: map[string]any{
			"selector": "user:123",
			"ttl":      3600,
		},
		CreatedAt: time.Now(),
	}

	if event.ID == "" {
		t.Error("AuditEvent.ID should not be empty")
	}
	if event.Action == "" {
		t.Error("AuditEvent.Action should not be empty")
	}
	if event.TargetType == "" {
		t.Error("AuditEvent.TargetType should not be empty")
	}
	if event.Payload == nil {
		t.Error("AuditEvent.Payload should not be nil")
	}
}

func TestAuditEventStruct_TokenActor(t *testing.T) {
	event := AuditEvent{
		ID:           "evt-123",
		OrgID:        "org-456",
		ActorUserID:  "",
		ActorTokenID: "tok-abc",
		Action:       "session.create",
		TargetType:   "session",
		TargetID:     "sess-xyz",
		CreatedAt:    time.Now(),
	}

	if event.ActorTokenID == "" {
		t.Error("AuditEvent.ActorTokenID should not be empty for token actor")
	}
	if event.ActorUserID != "" {
		t.Error("AuditEvent.ActorUserID should be empty for token actor")
	}
}

func TestPolicyStruct(t *testing.T) {
	policy := Policy{
		ID:                  "pol-123",
		OrgID:               "org-456",
		EnvID:               "env-789",
		MaxTTLSeconds:       3600,
		AllowEmptySelector:  false,
		AllowedSelectorKeys: []string{"user", "tenant"},
		RequireReason:       true,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	if policy.ID == "" {
		t.Error("Policy.ID should not be empty")
	}
	if policy.MaxTTLSeconds <= 0 {
		t.Error("Policy.MaxTTLSeconds should be positive")
	}
	if len(policy.AllowedSelectorKeys) != 2 {
		t.Errorf("Policy.AllowedSelectorKeys length = %d, want %d",
			len(policy.AllowedSelectorKeys), 2)
	}
}

func TestSessionStruct(t *testing.T) {
	session := Session{
		ID:        "sess-123",
		OrgID:     "org-456",
		EnvID:     "env-789",
		Status:    "active",
		Level:     "debug",
		CreatedBy: "user-abc",
		Reason:    "debugging issue #123",
		CreatedAt: time.Now(),
	}

	if session.ID == "" {
		t.Error("Session.ID should not be empty")
	}
	if session.Status == "" {
		t.Error("Session.Status should not be empty")
	}
	if session.Level == "" {
		t.Error("Session.Level should not be empty")
	}
}

func TestSessionFilter(t *testing.T) {
	filter := SessionFilter{
		Status: "active",
	}

	if filter.Status != "active" {
		t.Errorf("SessionFilter.Status = %q, want %q", filter.Status, "active")
	}

	// Empty filter
	emptyFilter := SessionFilter{}
	if emptyFilter.Status != "" {
		t.Error("Empty SessionFilter.Status should be empty")
	}
}
