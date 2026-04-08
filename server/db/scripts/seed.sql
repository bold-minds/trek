-- Seed data for local development

-- Create demo organization
INSERT INTO orgs (id, name, created_at) VALUES
    ('org_demo', 'Demo Organization', NOW())
ON CONFLICT (id) DO NOTHING;

-- Create environments
INSERT INTO envs (id, org_id, name, created_at) VALUES
    ('env_dev', 'org_demo', 'dev', NOW()),
    ('env_stage', 'org_demo', 'stage', NOW()),
    ('env_prod', 'org_demo', 'prod', NOW())
ON CONFLICT (id) DO NOTHING;

-- Create demo service tokens (different tokens per env due to unique constraint on hash)
-- In production, generate tokens securely!
-- dev: demo-token-dev  -> sha256
-- prod: demo-token-prod -> sha256
INSERT INTO service_tokens (id, org_id, env_id, token_hash, name, created_at) VALUES
    ('tok_demo_dev', 'org_demo', 'env_dev', 
     'e44f41ad57b11c239e4cd6f6a67cd95d2ac78152c3755b924d000d2d9d96f377', -- sha256 of 'demo-token-dev'
     'Demo Dev Token', NOW()),
    ('tok_demo_prod', 'org_demo', 'env_prod',
     '90c7c0b89f399b6cfe81f8cc65fcce3f473fe6b44413e935c109f301251430e9', -- sha256 of 'demo-token-prod'
     'Demo Prod Token', NOW())
ON CONFLICT (id) DO NOTHING;

-- Create default policies
INSERT INTO policies (id, org_id, env_id, max_ttl_seconds, allow_empty_selector, require_reason, created_at, updated_at) VALUES
    ('pol_demo_dev', 'org_demo', 'env_dev', 3600, false, false, NOW(), NOW()),
    ('pol_demo_prod', 'org_demo', 'env_prod', 1800, false, true, NOW(), NOW())
ON CONFLICT (org_id, env_id) DO NOTHING;

-- Create a demo user
INSERT INTO users (id, org_id, oidc_subject, email, created_at) VALUES
    ('usr_demo', 'org_demo', 'demo|12345', 'demo@example.com', NOW())
ON CONFLICT (id) DO NOTHING;

-- Create roles
INSERT INTO roles (id, org_id, name, permissions, created_at) VALUES
    ('role_admin', 'org_demo', 'admin', '["sessions:create", "sessions:revoke", "policies:write", "audit:read"]', NOW()),
    ('role_oncall', 'org_demo', 'oncall', '["sessions:create", "sessions:revoke", "audit:read"]', NOW()),
    ('role_readonly', 'org_demo', 'readonly', '["sessions:read", "audit:read"]', NOW())
ON CONFLICT (id) DO NOTHING;

-- Assign demo user to admin role
INSERT INTO user_roles (user_id, role_id, env_id, created_at) VALUES
    ('usr_demo', 'role_admin', NULL, NOW())
ON CONFLICT DO NOTHING;

SELECT 'Seed data loaded successfully!' as status;
