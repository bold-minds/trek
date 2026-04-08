package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/bold-minds/trek"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore implements Store using PostgreSQL.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a new PostgreSQL-backed store implementation.
// The provided pool must be configured with appropriate connection limits
// and timeouts before passing to this constructor. The pool is not managed
// by this store and should be closed by the caller when done.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// Ping verifies the database connection is alive.
func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// CreateSession creates a new debug session.
func (s *PostgresStore) CreateSession(ctx context.Context, session *Session) error {
	selectorJSON, err := json.Marshal(session.Selector)
	if err != nil {
		return fmt.Errorf("marshal selector: %w", err)
	}

	capsJSON, err := json.Marshal(session.Caps)
	if err != nil {
		return fmt.Errorf("marshal caps: %w", err)
	}

	labelsJSON, err := json.Marshal(session.Labels)
	if err != nil {
		return fmt.Errorf("marshal labels: %w", err)
	}

	var serviceScopeJSON []byte
	if session.ServiceScope != nil {
		serviceScopeJSON, err = json.Marshal(session.ServiceScope)
		if err != nil {
			return fmt.Errorf("marshal service_scope: %w", err)
		}
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO sessions (id, org_id, env_id, status, selector, level, expires_at, caps, labels, service_scope, created_by, reason, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, session.ID, session.OrgID, session.EnvID, session.Status, selectorJSON, session.Level,
		session.ExpiresAt, capsJSON, labelsJSON, serviceScopeJSON, session.CreatedBy, session.Reason, session.CreatedAt)

	return err
}

// GetSession retrieves a session by ID.
func (s *PostgresStore) GetSession(ctx context.Context, orgID, envID, sessionID string) (*Session, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, org_id, env_id, status, selector, level, expires_at, caps, labels, service_scope, created_by, reason, created_at, revoked_at
		FROM sessions
		WHERE id = $1 AND org_id = $2 AND env_id = $3
	`, sessionID, orgID, envID)

	return scanSession(row)
}

// ListSessions lists sessions with optional filtering.
func (s *PostgresStore) ListSessions(ctx context.Context, orgID, envID string, filter SessionFilter) ([]Session, error) {
	query := `
		SELECT id, org_id, env_id, status, selector, level, expires_at, caps, labels, service_scope, created_by, reason, created_at, revoked_at
		FROM sessions
		WHERE org_id = $1 AND env_id = $2
	`
	args := []any{orgID, envID}

	if filter.Status != "" {
		query += " AND status = $3"
		args = append(args, filter.Status)
	}

	query += " ORDER BY created_at DESC LIMIT 100"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		sess, err := scanSessionFromRows(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, *sess)
	}

	return sessions, rows.Err()
}

// GetActiveSessions returns active sessions for SDK polling.
func (s *PostgresStore) GetActiveSessions(ctx context.Context, orgID, envID, serviceName string) ([]trek.Session, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, selector, level, expires_at, caps, labels, service_scope
		FROM sessions
		WHERE org_id = $1 AND env_id = $2 AND status = 'active' AND expires_at > NOW()
	`, orgID, envID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []trek.Session
	for rows.Next() {
		var (
			id                           string
			selectorJSON, capsJSON       []byte
			labelsJSON, serviceScopeJSON []byte
			level                        string
			expiresAt                    time.Time
		)

		if err := rows.Scan(&id, &selectorJSON, &level, &expiresAt, &capsJSON, &labelsJSON, &serviceScopeJSON); err != nil {
			return nil, err
		}

		var selector trek.Selector
		if err := json.Unmarshal(selectorJSON, &selector); err != nil {
			return nil, fmt.Errorf("unmarshal selector: %w", err)
		}

		var caps trek.Caps
		if len(capsJSON) > 0 {
			if err := json.Unmarshal(capsJSON, &caps); err != nil {
				return nil, fmt.Errorf("unmarshal caps: %w", err)
			}
		}

		var labels map[string]string
		if len(labelsJSON) > 0 {
			if err := json.Unmarshal(labelsJSON, &labels); err != nil {
				return nil, fmt.Errorf("unmarshal labels: %w", err)
			}
		}

		var serviceScope []string
		if len(serviceScopeJSON) > 0 {
			if err := json.Unmarshal(serviceScopeJSON, &serviceScope); err != nil {
				return nil, fmt.Errorf("unmarshal service_scope: %w", err)
			}
		}

		sessions = append(sessions, trek.Session{
			ID:           id,
			Selector:     selector,
			Level:        trek.Level(level),
			ExpiresAt:    expiresAt,
			Caps:         caps,
			Labels:       labels,
			ServiceScope: serviceScope,
		})
	}

	return sessions, rows.Err()
}

// RevokeSession marks a session as revoked.
func (s *PostgresStore) RevokeSession(ctx context.Context, orgID, envID, sessionID string) error {
	result, err := s.pool.Exec(ctx, `
		UPDATE sessions SET status = 'revoked', revoked_at = NOW()
		WHERE id = $1 AND org_id = $2 AND env_id = $3 AND status = 'active'
	`, sessionID, orgID, envID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// ExpireSessions marks expired sessions.
func (s *PostgresStore) ExpireSessions(ctx context.Context) (int, error) {
	result, err := s.pool.Exec(ctx, `
		UPDATE sessions SET status = 'expired'
		WHERE status = 'active' AND expires_at <= NOW()
	`)
	if err != nil {
		return 0, err
	}

	return int(result.RowsAffected()), nil
}

// GetPolicy retrieves the effective policy for an org/env.
func (s *PostgresStore) GetPolicy(ctx context.Context, orgID, envID string) (*Policy, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, org_id, env_id, max_ttl_seconds, allow_empty_selector, allowed_selector_keys, default_caps, require_reason, created_at, updated_at
		FROM policies
		WHERE org_id = $1 AND (env_id = $2 OR env_id IS NULL)
		ORDER BY env_id NULLS LAST
		LIMIT 1
	`, orgID, envID)

	var p Policy
	var allowedKeysJSON, defaultCapsJSON []byte
	var envIDPtr *string

	err := row.Scan(&p.ID, &p.OrgID, &envIDPtr, &p.MaxTTLSeconds, &p.AllowEmptySelector,
		&allowedKeysJSON, &defaultCapsJSON, &p.RequireReason, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return defaultPolicy(orgID, envID), nil
	}
	if err != nil {
		return nil, err
	}

	if envIDPtr != nil {
		p.EnvID = *envIDPtr
	}

	if len(allowedKeysJSON) > 0 {
		if err := json.Unmarshal(allowedKeysJSON, &p.AllowedSelectorKeys); err != nil {
			return nil, fmt.Errorf("unmarshal allowed_selector_keys: %w", err)
		}
	}

	if len(defaultCapsJSON) > 0 {
		if err := json.Unmarshal(defaultCapsJSON, &p.DefaultCaps); err != nil {
			return nil, fmt.Errorf("unmarshal default_caps: %w", err)
		}
	}

	return &p, nil
}

// SetPolicy creates or updates a policy.
func (s *PostgresStore) SetPolicy(ctx context.Context, policy *Policy) error {
	allowedKeysJSON, _ := json.Marshal(policy.AllowedSelectorKeys)
	defaultCapsJSON, _ := json.Marshal(policy.DefaultCaps)

	_, err := s.pool.Exec(ctx, `
		INSERT INTO policies (id, org_id, env_id, max_ttl_seconds, allow_empty_selector, allowed_selector_keys, default_caps, require_reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
		ON CONFLICT (org_id, env_id) DO UPDATE SET
			max_ttl_seconds = EXCLUDED.max_ttl_seconds,
			allow_empty_selector = EXCLUDED.allow_empty_selector,
			allowed_selector_keys = EXCLUDED.allowed_selector_keys,
			default_caps = EXCLUDED.default_caps,
			require_reason = EXCLUDED.require_reason,
			updated_at = NOW()
	`, policy.ID, policy.OrgID, nullString(policy.EnvID), policy.MaxTTLSeconds,
		policy.AllowEmptySelector, allowedKeysJSON, defaultCapsJSON, policy.RequireReason)

	return err
}

// CreateAuditEvent records an audit event.
func (s *PostgresStore) CreateAuditEvent(ctx context.Context, event *AuditEvent) error {
	payloadJSON, _ := json.Marshal(event.Payload)

	_, err := s.pool.Exec(ctx, `
		INSERT INTO audit_events (id, org_id, env_id, actor_user_id, actor_token_id, action, target_type, target_id, payload, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, event.ID, event.OrgID, nullString(event.EnvID), nullString(event.ActorUserID),
		nullString(event.ActorTokenID), event.Action, event.TargetType, event.TargetID, payloadJSON, event.CreatedAt)

	return err
}

// ListAuditEvents returns paginated audit events.
func (s *PostgresStore) ListAuditEvents(ctx context.Context, orgID, envID string, limit int, cursor string) ([]AuditEvent, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := `
		SELECT id, org_id, env_id, actor_user_id, actor_token_id, action, target_type, target_id, payload, created_at
		FROM audit_events
		WHERE org_id = $1 AND (env_id = $2 OR $2 = '')
	`
	args := []any{orgID, envID}

	if cursor != "" {
		query += " AND id < $3"
		args = append(args, cursor)
	}

	query += " ORDER BY created_at DESC LIMIT $" + fmt.Sprint(len(args)+1)
	args = append(args, limit+1)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var events []AuditEvent
	for rows.Next() {
		var e AuditEvent
		var envIDPtr, actorUserIDPtr, actorTokenIDPtr *string
		var payloadJSON []byte

		if err := rows.Scan(&e.ID, &e.OrgID, &envIDPtr, &actorUserIDPtr, &actorTokenIDPtr,
			&e.Action, &e.TargetType, &e.TargetID, &payloadJSON, &e.CreatedAt); err != nil {
			return nil, "", err
		}

		if envIDPtr != nil {
			e.EnvID = *envIDPtr
		}
		if actorUserIDPtr != nil {
			e.ActorUserID = *actorUserIDPtr
		}
		if actorTokenIDPtr != nil {
			e.ActorTokenID = *actorTokenIDPtr
		}

		if len(payloadJSON) > 0 {
			if err := json.Unmarshal(payloadJSON, &e.Payload); err != nil {
				return nil, "", fmt.Errorf("unmarshal audit event payload %s: %w", e.ID, err)
			}
		}

		events = append(events, e)
	}

	var nextCursor string
	if len(events) > limit {
		nextCursor = events[limit].ID
		events = events[:limit]
	}

	return events, nextCursor, rows.Err()
}

// ValidateTokenPlain validates a plaintext token against stored hashes.
// Supports both legacy (unsalted SHA256) and new (salted) hash formats.
func (s *PostgresStore) ValidateTokenPlain(ctx context.Context, plainToken string) (*ServiceToken, error) {
	// Fetch all non-revoked tokens and verify against each
	// This is necessary because salted hashes can't be looked up directly
	rows, err := s.pool.Query(ctx, `
		SELECT id, org_id, env_id, token_hash, name, service_allowlist, created_at, revoked_at
		FROM service_tokens
		WHERE revoked_at IS NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var t ServiceToken
		var serviceScopeJSON []byte

		if err := rows.Scan(&t.ID, &t.OrgID, &t.EnvID, &t.TokenHash, &t.Name, &serviceScopeJSON, &t.CreatedAt, &t.RevokedAt); err != nil {
			return nil, err
		}

		if VerifyToken(plainToken, t.TokenHash) {
			if len(serviceScopeJSON) > 0 {
				if err := json.Unmarshal(serviceScopeJSON, &t.ServiceAllowlist); err != nil {
					return nil, fmt.Errorf("unmarshal service allowlist for token %s: %w", t.ID, err)
				}
			}
			return &t, nil
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return nil, ErrNotFound
}

// ValidateToken validates a service token by hash (legacy method).
func (s *PostgresStore) ValidateToken(ctx context.Context, tokenHash string) (*ServiceToken, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, org_id, env_id, token_hash, name, service_allowlist, created_at, revoked_at
		FROM service_tokens
		WHERE token_hash = $1 AND revoked_at IS NULL
	`, tokenHash)

	var t ServiceToken
	var serviceScopeJSON []byte

	err := row.Scan(&t.ID, &t.OrgID, &t.EnvID, &t.TokenHash, &t.Name, &serviceScopeJSON, &t.CreatedAt, &t.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if len(serviceScopeJSON) > 0 {
		if err := json.Unmarshal(serviceScopeJSON, &t.ServiceAllowlist); err != nil {
			return nil, fmt.Errorf("unmarshal service allowlist for token %s: %w", t.ID, err)
		}
	}

	return &t, nil
}

// CreateToken creates a new service token.
func (s *PostgresStore) CreateToken(ctx context.Context, token *ServiceToken) error {
	var serviceScopeJSON []byte
	if token.ServiceAllowlist != nil {
		serviceScopeJSON, _ = json.Marshal(token.ServiceAllowlist)
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO service_tokens (id, org_id, env_id, token_hash, name, service_allowlist, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, token.ID, token.OrgID, token.EnvID, token.TokenHash, token.Name, serviceScopeJSON, token.CreatedAt)

	return err
}

// RevokeToken revokes a service token, verifying org/env ownership.
func (s *PostgresStore) RevokeToken(ctx context.Context, orgID, envID, tokenID string) error {
	result, err := s.pool.Exec(ctx, `
		UPDATE service_tokens SET revoked_at = NOW() 
		WHERE id = $1 AND org_id = $2 AND env_id = $3 AND revoked_at IS NULL
	`, tokenID, orgID, envID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListTokens lists service tokens.
func (s *PostgresStore) ListTokens(ctx context.Context, orgID, envID string) ([]ServiceToken, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, org_id, env_id, token_hash, name, service_allowlist, created_at, revoked_at
		FROM service_tokens
		WHERE org_id = $1 AND env_id = $2
		ORDER BY created_at DESC
	`, orgID, envID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []ServiceToken
	for rows.Next() {
		var t ServiceToken
		var serviceScopeJSON []byte

		if err := rows.Scan(&t.ID, &t.OrgID, &t.EnvID, &t.TokenHash, &t.Name, &serviceScopeJSON, &t.CreatedAt, &t.RevokedAt); err != nil {
			return nil, err
		}

		if len(serviceScopeJSON) > 0 {
			if err := json.Unmarshal(serviceScopeJSON, &t.ServiceAllowlist); err != nil {
				return nil, fmt.Errorf("unmarshal service allowlist for token %s: %w", t.ID, err)
			}
		}

		tokens = append(tokens, t)
	}

	return tokens, rows.Err()
}

// GetOrg retrieves an organization.
func (s *PostgresStore) GetOrg(ctx context.Context, orgID string) (*Org, error) {
	row := s.pool.QueryRow(ctx, `SELECT id, name, created_at FROM orgs WHERE id = $1`, orgID)

	var o Org
	err := row.Scan(&o.ID, &o.Name, &o.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &o, err
}

// GetEnv retrieves an environment.
func (s *PostgresStore) GetEnv(ctx context.Context, orgID, envID string) (*Env, error) {
	row := s.pool.QueryRow(ctx, `SELECT id, org_id, name, created_at FROM envs WHERE id = $1 AND org_id = $2`, envID, orgID)

	var e Env
	err := row.Scan(&e.ID, &e.OrgID, &e.Name, &e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &e, err
}

// ListEnvs lists environments for an org.
func (s *PostgresStore) ListEnvs(ctx context.Context, orgID string) ([]Env, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, org_id, name, created_at FROM envs WHERE org_id = $1`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var envs []Env
	for rows.Next() {
		var e Env
		if err := rows.Scan(&e.ID, &e.OrgID, &e.Name, &e.CreatedAt); err != nil {
			return nil, err
		}
		envs = append(envs, e)
	}

	return envs, rows.Err()
}

// Helper functions

func scanSession(row pgx.Row) (*Session, error) {
	var s Session
	var selectorJSON, capsJSON, labelsJSON, serviceScopeJSON []byte
	var createdByPtr *string

	err := row.Scan(&s.ID, &s.OrgID, &s.EnvID, &s.Status, &selectorJSON, &s.Level,
		&s.ExpiresAt, &capsJSON, &labelsJSON, &serviceScopeJSON, &createdByPtr, &s.Reason, &s.CreatedAt, &s.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if createdByPtr != nil {
		s.CreatedBy = *createdByPtr
	}

	if err := json.Unmarshal(selectorJSON, &s.Selector); err != nil {
		slog.Warn("failed to unmarshal selector", "session_id", s.ID, "error", err)
	}
	if err := json.Unmarshal(capsJSON, &s.Caps); err != nil {
		slog.Warn("failed to unmarshal caps", "session_id", s.ID, "error", err)
	}
	if err := json.Unmarshal(labelsJSON, &s.Labels); err != nil {
		slog.Warn("failed to unmarshal labels", "session_id", s.ID, "error", err)
	}
	if len(serviceScopeJSON) > 0 {
		if err := json.Unmarshal(serviceScopeJSON, &s.ServiceScope); err != nil {
			slog.Warn("failed to unmarshal service_scope", "session_id", s.ID, "error", err)
		}
	}

	return &s, nil
}

func scanSessionFromRows(rows pgx.Rows) (*Session, error) {
	var s Session
	var selectorJSON, capsJSON, labelsJSON, serviceScopeJSON []byte
	var createdByPtr *string

	err := rows.Scan(&s.ID, &s.OrgID, &s.EnvID, &s.Status, &selectorJSON, &s.Level,
		&s.ExpiresAt, &capsJSON, &labelsJSON, &serviceScopeJSON, &createdByPtr, &s.Reason, &s.CreatedAt, &s.RevokedAt)
	if err != nil {
		return nil, err
	}

	if createdByPtr != nil {
		s.CreatedBy = *createdByPtr
	}

	if err := json.Unmarshal(selectorJSON, &s.Selector); err != nil {
		slog.Warn("failed to unmarshal selector", "session_id", s.ID, "error", err)
	}
	if err := json.Unmarshal(capsJSON, &s.Caps); err != nil {
		slog.Warn("failed to unmarshal caps", "session_id", s.ID, "error", err)
	}
	if err := json.Unmarshal(labelsJSON, &s.Labels); err != nil {
		slog.Warn("failed to unmarshal labels", "session_id", s.ID, "error", err)
	}
	if len(serviceScopeJSON) > 0 {
		if err := json.Unmarshal(serviceScopeJSON, &s.ServiceScope); err != nil {
			slog.Warn("failed to unmarshal service_scope", "session_id", s.ID, "error", err)
		}
	}

	return &s, nil
}

func defaultPolicy(orgID, envID string) *Policy {
	return &Policy{
		OrgID:              orgID,
		EnvID:              envID,
		MaxTTLSeconds:      1800,
		AllowEmptySelector: false,
		DefaultCaps: trek.Caps{
			MaxDebugEventsPerRequest: 200,
			MaxDebugEventsPerSession: 5000,
		},
		RequireReason: true,
	}
}

func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// HashToken hashes a token with a salt for secure storage.
// Format: salt$hash where salt is 16 bytes base64 encoded.
// Returns an error if cryptographic random generation fails.
func HashToken(token string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}
	return HashTokenWithSalt(token, salt), nil
}

// HashTokenWithSalt hashes a token with a provided salt.
func HashTokenWithSalt(token string, salt []byte) string {
	h := sha256.Sum256(append(salt, []byte(token)...))
	return base64.StdEncoding.EncodeToString(salt) + "$" + hex.EncodeToString(h[:])
}

// VerifyToken checks if a plaintext token matches a stored hash.
func VerifyToken(token, storedHash string) bool {
	parts := splitHashParts(storedHash)
	if len(parts) != 2 {
		// Legacy unsalted hash - compare directly
		h := sha256.Sum256([]byte(token))
		return storedHash == hex.EncodeToString(h[:])
	}
	salt, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	expectedHash := HashTokenWithSalt(token, salt)
	return storedHash == expectedHash
}

func splitHashParts(hash string) []string {
	for i, c := range hash {
		if c == '$' {
			return []string{hash[:i], hash[i+1:]}
		}
	}
	return []string{hash}
}

// ErrNotFound is returned when a resource is not found.
var ErrNotFound = errors.New("not found")
