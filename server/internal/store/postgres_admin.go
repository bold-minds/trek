package store

import (
	"context"
	"encoding/json"
	"fmt"
)

// --- Environment Management ---

// CreateEnv creates a new environment.
func (s *PostgresStore) CreateEnv(ctx context.Context, env *Env) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO envs (id, org_id, name, created_at)
		VALUES ($1, $2, $3, $4)
	`, env.ID, env.OrgID, env.Name, env.CreatedAt)
	return err
}

// DeleteEnv deletes an environment.
func (s *PostgresStore) DeleteEnv(ctx context.Context, orgID, envID string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM envs WHERE id = $1 AND org_id = $2
	`, envID, orgID)
	return err
}

// --- User Management ---

// ListUsers lists all users in an organization.
func (s *PostgresStore) ListUsers(ctx context.Context, orgID string) ([]User, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, org_id, oidc_subject, email, created_at
		FROM users
		WHERE org_id = $1
		ORDER BY created_at DESC
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.OrgID, &u.OIDCSubject, &u.Email, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// CreateUser creates a new user.
func (s *PostgresStore) CreateUser(ctx context.Context, user *User) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO users (id, org_id, oidc_subject, email, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, user.ID, user.OrgID, user.OIDCSubject, user.Email, user.CreatedAt)
	return err
}

// DeleteUser deletes a user.
func (s *PostgresStore) DeleteUser(ctx context.Context, orgID, userID string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM users WHERE id = $1 AND org_id = $2
	`, userID, orgID)
	return err
}

// --- Role Management ---

// ListRoles lists all roles in an organization.
func (s *PostgresStore) ListRoles(ctx context.Context, orgID string) ([]Role, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, org_id, name, permissions, created_at
		FROM roles
		WHERE org_id = $1
		ORDER BY name
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []Role
	for rows.Next() {
		var r Role
		var permJSON []byte
		if err := rows.Scan(&r.ID, &r.OrgID, &r.Name, &permJSON, &r.CreatedAt); err != nil {
			return nil, err
		}
		if len(permJSON) > 0 {
			if err := json.Unmarshal(permJSON, &r.Permissions); err != nil {
				return nil, fmt.Errorf("unmarshal role permissions for role %s: %w", r.ID, err)
			}
		}
		roles = append(roles, r)
	}
	return roles, rows.Err()
}

// CreateRole creates a new role.
func (s *PostgresStore) CreateRole(ctx context.Context, role *Role) error {
	permJSON, _ := json.Marshal(role.Permissions)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO roles (id, org_id, name, permissions, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, role.ID, role.OrgID, role.Name, permJSON, role.CreatedAt)
	return err
}

// AssignRole assigns a role to a user.
func (s *PostgresStore) AssignRole(ctx context.Context, orgID, userID, roleID, envID string) error {
	var envIDVal interface{}
	if envID != "" {
		envIDVal = envID
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id, env_id, created_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (user_id, role_id, env_id) DO NOTHING
	`, userID, roleID, envIDVal)
	return err
}

// RevokeRole removes a role from a user.
func (s *PostgresStore) RevokeRole(ctx context.Context, orgID, userID, roleID, envID string) error {
	var envIDVal interface{}
	if envID != "" {
		envIDVal = envID
	}
	_, err := s.pool.Exec(ctx, `
		DELETE FROM user_roles
		WHERE user_id = $1 AND role_id = $2 AND (env_id = $3 OR ($3 IS NULL AND env_id IS NULL))
	`, userID, roleID, envIDVal)
	return err
}

// GetUserRoles gets all roles assigned to a user.
func (s *PostgresStore) GetUserRoles(ctx context.Context, orgID, userID string) ([]UserRole, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT ur.user_id, ur.role_id, r.name, ur.env_id, ur.created_at
		FROM user_roles ur
		JOIN roles r ON r.id = ur.role_id
		JOIN users u ON u.id = ur.user_id
		WHERE u.org_id = $1 AND ur.user_id = $2
	`, orgID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []UserRole
	for rows.Next() {
		var r UserRole
		var envID *string
		if err := rows.Scan(&r.UserID, &r.RoleID, &r.RoleName, &envID, &r.CreatedAt); err != nil {
			return nil, err
		}
		if envID != nil {
			r.EnvID = *envID
		}
		roles = append(roles, r)
	}
	return roles, rows.Err()
}

// GetUserPermissions gets all permissions for a user in a specific env.
func (s *PostgresStore) GetUserPermissions(ctx context.Context, orgID, userID, envID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT r.permissions
		FROM user_roles ur
		JOIN roles r ON r.id = ur.role_id
		JOIN users u ON u.id = ur.user_id
		WHERE u.org_id = $1 AND ur.user_id = $2 AND (ur.env_id = $3 OR ur.env_id IS NULL)
	`, orgID, userID, envID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	permSet := make(map[string]bool)
	for rows.Next() {
		var permJSON []byte
		if err := rows.Scan(&permJSON); err != nil {
			return nil, err
		}
		var perms []string
		if len(permJSON) > 0 {
			if err := json.Unmarshal(permJSON, &perms); err != nil {
				return nil, fmt.Errorf("unmarshal user permissions: %w", err)
			}
		}
		for _, p := range perms {
			permSet[p] = true
		}
	}

	var permissions []string
	for p := range permSet {
		permissions = append(permissions, p)
	}
	return permissions, rows.Err()
}

// --- Notification Config Management ---

// ListNotificationConfigs lists notification configs for an org/env.
func (s *PostgresStore) ListNotificationConfigs(ctx context.Context, orgID, envID string) ([]NotificationConfig, error) {
	query := `
		SELECT id, org_id, env_id, type, config, enabled, created_at, updated_at
		FROM notification_configs
		WHERE org_id = $1
	`
	args := []interface{}{orgID}

	if envID != "" {
		query += ` AND (env_id = $2 OR env_id IS NULL)`
		args = append(args, envID)
	}

	query += ` ORDER BY created_at DESC`

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []NotificationConfig
	for rows.Next() {
		var c NotificationConfig
		var envIDPtr *string
		var configJSON []byte
		if err := rows.Scan(&c.ID, &c.OrgID, &envIDPtr, &c.Type, &configJSON, &c.Enabled, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		if envIDPtr != nil {
			c.EnvID = *envIDPtr
		}
		if len(configJSON) > 0 {
			if err := json.Unmarshal(configJSON, &c.Config); err != nil {
				return nil, fmt.Errorf("unmarshal notification config %s: %w", c.ID, err)
			}
		}
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

// CreateNotificationConfig creates a notification config.
func (s *PostgresStore) CreateNotificationConfig(ctx context.Context, config *NotificationConfig) error {
	configJSON, _ := json.Marshal(config.Config)
	var envIDVal interface{}
	if config.EnvID != "" {
		envIDVal = config.EnvID
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO notification_configs (id, org_id, env_id, type, config, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, config.ID, config.OrgID, envIDVal, config.Type, configJSON, config.Enabled, config.CreatedAt, config.UpdatedAt)
	return err
}

// UpdateNotificationConfig updates a notification config.
func (s *PostgresStore) UpdateNotificationConfig(ctx context.Context, orgID, configID string, config map[string]any, enabled *bool) error {
	if config == nil && enabled == nil {
		return fmt.Errorf("nothing to update")
	}

	if config != nil {
		configJSON, _ := json.Marshal(config)
		_, err := s.pool.Exec(ctx, `
			UPDATE notification_configs SET config = $1, updated_at = NOW()
			WHERE id = $2 AND org_id = $3
		`, configJSON, configID, orgID)
		if err != nil {
			return err
		}
	}

	if enabled != nil {
		_, err := s.pool.Exec(ctx, `
			UPDATE notification_configs SET enabled = $1, updated_at = NOW()
			WHERE id = $2 AND org_id = $3
		`, *enabled, configID, orgID)
		if err != nil {
			return err
		}
	}

	return nil
}

// DeleteNotificationConfig deletes a notification config.
func (s *PostgresStore) DeleteNotificationConfig(ctx context.Context, orgID, configID string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM notification_configs WHERE id = $1 AND org_id = $2
	`, configID, orgID)
	return err
}
