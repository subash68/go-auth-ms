-- ============================================================
-- Auth Service — PostgreSQL Schema
-- ============================================================

-- Enable UUID generation
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================
-- users
-- ============================================================
CREATE TABLE IF NOT EXISTS users (
    id            UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    username      VARCHAR(50)   NOT NULL,                -- unique, alphanumeric
    first_name    VARCHAR(100)  NOT NULL,
    last_name     VARCHAR(100)  NOT NULL,
    email         VARCHAR(255),                          -- optional
    password_hash VARCHAR(255)  NOT NULL,                -- bcrypt hash
    is_active     BOOLEAN       NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ                            -- soft delete
);

-- username: unique among non-deleted users, alphanumeric + underscore only
CREATE UNIQUE INDEX idx_users_username
    ON users (username)
    WHERE deleted_at IS NULL;

ALTER TABLE users
    ADD CONSTRAINT chk_users_username_alphanumeric
    CHECK (username ~ '^[a-zA-Z0-9_]+$');

-- email: unique among non-deleted users (when provided)
CREATE UNIQUE INDEX idx_users_email
    ON users (email)
    WHERE deleted_at IS NULL AND email IS NOT NULL;

CREATE INDEX idx_users_deleted_at ON users (deleted_at);

-- ============================================================
-- roles
-- ============================================================
CREATE TABLE IF NOT EXISTS roles (
    id          BIGSERIAL     PRIMARY KEY,
    name        VARCHAR(100)  NOT NULL UNIQUE,
    description TEXT,
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

-- ============================================================
-- permissions
-- Naming convention: <resource>:<action>  e.g. auth:read
-- ============================================================
CREATE TABLE IF NOT EXISTS permissions (
    id          BIGSERIAL     PRIMARY KEY,
    name        VARCHAR(100)  NOT NULL UNIQUE,
    description TEXT,
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

-- ============================================================
-- role_permissions  (R2PM)
-- ============================================================
CREATE TABLE IF NOT EXISTS role_permissions (
    role_id       BIGINT  NOT NULL REFERENCES roles(id)       ON DELETE CASCADE,
    permission_id BIGINT  NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE INDEX idx_role_permissions_role_id       ON role_permissions (role_id);
CREATE INDEX idx_role_permissions_permission_id ON role_permissions (permission_id);

-- ============================================================
-- user_roles
-- ============================================================
CREATE TABLE IF NOT EXISTS user_roles (
    user_id     UUID    NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id     BIGINT  NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, role_id)
);

CREATE INDEX idx_user_roles_user_id ON user_roles (user_id);
CREATE INDEX idx_user_roles_role_id ON user_roles (role_id);

-- ============================================================
-- sessions  (refresh token store)
-- Access tokens are stateless JWTs — only refresh tokens are stored.
-- Logout = mark the session revoked.
-- ============================================================
CREATE TABLE IF NOT EXISTS sessions (
    id             UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token  TEXT         NOT NULL UNIQUE,  -- hashed refresh token
    user_agent     TEXT,
    ip_address     INET,
    is_revoked     BOOLEAN      NOT NULL DEFAULT FALSE,
    expires_at     TIMESTAMPTZ  NOT NULL,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    revoked_at     TIMESTAMPTZ
);

CREATE INDEX idx_sessions_user_id       ON sessions (user_id);
CREATE INDEX idx_sessions_refresh_token ON sessions (refresh_token);
CREATE INDEX idx_sessions_expires_at    ON sessions (expires_at);
CREATE INDEX idx_sessions_active
    ON sessions (user_id)
    WHERE is_revoked = FALSE;

-- ============================================================
-- updated_at trigger
-- ============================================================
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- Seed — roles
-- ============================================================
INSERT INTO roles (name, description) VALUES
    ('admin',  'Full system access'),
    ('user',   'Standard authenticated user'),
    ('viewer', 'Read-only access')
ON CONFLICT (name) DO NOTHING;

-- ============================================================
-- Seed — permissions  (<resource>:<action>)
-- ============================================================
INSERT INTO permissions (name, description) VALUES
    ('auth:register',  'Register new users'),
    ('auth:login',     'Authenticate and acquire tokens'),
    ('auth:logout',    'Revoke session'),
    ('auth:refresh',   'Refresh access token'),
    ('user:read',      'View user profiles'),
    ('user:write',     'Create or update users'),
    ('user:delete',    'Soft-delete users'),
    ('role:read',      'View roles and permissions'),
    ('role:write',     'Create or update roles'),
    ('role:assign',    'Assign roles to users')
ON CONFLICT (name) DO NOTHING;

-- ============================================================
-- Seed — R2PM
-- ============================================================

-- admin gets everything
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'admin'
ON CONFLICT DO NOTHING;

-- user: basic auth + own profile read
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'user'
  AND p.name IN ('auth:login', 'auth:logout', 'auth:refresh', 'user:read')
ON CONFLICT DO NOTHING;

-- viewer: read-only
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'viewer'
  AND p.name IN ('auth:login', 'auth:logout', 'auth:refresh', 'user:read', 'role:read')
ON CONFLICT DO NOTHING;
