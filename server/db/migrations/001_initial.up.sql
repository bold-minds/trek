-- Trek Control Plane Schema

-- Organizations
CREATE TABLE orgs (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Environments (dev/stage/prod)
CREATE TABLE envs (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, name)
);

CREATE INDEX envs_org_id ON envs(org_id);

-- Users (OIDC subjects)
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    oidc_subject TEXT NOT NULL,
    email TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, oidc_subject)
);

CREATE INDEX users_org_id ON users(org_id);

-- Roles
CREATE TABLE roles (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    permissions JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, name)
);

CREATE INDEX roles_org_id ON roles(org_id);

-- User role assignments
CREATE TABLE user_roles (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id TEXT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    env_id TEXT REFERENCES envs(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(user_id, role_id, env_id)
);

-- Service tokens (for SDK polling)
CREATE TABLE service_tokens (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    env_id TEXT NOT NULL REFERENCES envs(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    service_allowlist JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

CREATE INDEX service_tokens_org_env ON service_tokens(org_id, env_id);
CREATE INDEX service_tokens_hash ON service_tokens(token_hash) WHERE revoked_at IS NULL;

-- Debug sessions
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    env_id TEXT NOT NULL REFERENCES envs(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked', 'expired')),
    selector JSONB NOT NULL,
    level TEXT NOT NULL CHECK (level IN ('debug', 'trace')),
    expires_at TIMESTAMPTZ NOT NULL,
    caps JSONB NOT NULL DEFAULT '{}',
    labels JSONB NOT NULL DEFAULT '{}',
    service_scope JSONB,
    created_by TEXT REFERENCES users(id),
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

CREATE INDEX sessions_active_poll ON sessions(org_id, env_id, expires_at) WHERE status = 'active';
CREATE INDEX sessions_org_env_status ON sessions(org_id, env_id, status);

-- Policies (org-level or env-level)
CREATE TABLE policies (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    env_id TEXT REFERENCES envs(id) ON DELETE CASCADE,
    max_ttl_seconds INT NOT NULL DEFAULT 1800,
    allow_empty_selector BOOLEAN NOT NULL DEFAULT FALSE,
    allowed_selector_keys JSONB,
    default_caps JSONB NOT NULL DEFAULT '{"max_debug_events_per_request": 200, "max_debug_events_per_session": 5000}',
    require_reason BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, env_id)
);

CREATE INDEX policies_org_id ON policies(org_id);

-- Audit events
CREATE TABLE audit_events (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    env_id TEXT REFERENCES envs(id) ON DELETE CASCADE,
    actor_user_id TEXT REFERENCES users(id),
    actor_token_id TEXT REFERENCES service_tokens(id),
    action TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX audit_events_org_env_time ON audit_events(org_id, env_id, created_at DESC);

-- Notification configs
CREATE TABLE notification_configs (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    env_id TEXT REFERENCES envs(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('slack', 'webhook')),
    config JSONB NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX notification_configs_org_id ON notification_configs(org_id);
