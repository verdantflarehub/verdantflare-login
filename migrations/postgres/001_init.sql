-- VerdantFlare Login PostgreSQL schema.

BEGIN;

CREATE TABLE IF NOT EXISTS auth_users (
    id UUID PRIMARY KEY,
    email VARCHAR(254) NOT NULL,
    normalized_email VARCHAR(254) NOT NULL,
    status VARCHAR(24) NOT NULL CHECK (status IN ('active', 'disabled', 'deleted')),
    email_verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ,
    CONSTRAINT auth_users_normalized_email_unique UNIQUE (normalized_email)
);

CREATE TABLE IF NOT EXISTS auth_password_credentials (
    user_id UUID PRIMARY KEY REFERENCES auth_users(id) ON DELETE CASCADE,
    password_hash TEXT NOT NULL,
    password_changed_at TIMESTAMPTZ NOT NULL,
    failed_attempts INTEGER NOT NULL DEFAULT 0 CHECK (failed_attempts >= 0),
    locked_until TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS auth_identities (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    provider VARCHAR(64) NOT NULL,
    provider_subject VARCHAR(255) NOT NULL,
    email_at_provider VARCHAR(254),
    claims JSONB NOT NULL DEFAULT '{}'::JSONB,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT auth_identities_provider_subject_unique UNIQUE (provider, provider_subject)
);

CREATE INDEX IF NOT EXISTS auth_identities_user_id_idx
    ON auth_identities (user_id);

CREATE TABLE IF NOT EXISTS auth_email_challenges (
    id UUID PRIMARY KEY,
    normalized_email VARCHAR(254) NOT NULL,
    purpose VARCHAR(32) NOT NULL CHECK (purpose IN ('sign_up', 'verify_email')),
    secret_hash TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    max_attempts INTEGER NOT NULL CHECK (max_attempts > 0),
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT auth_email_challenges_active_key UNIQUE (normalized_email, purpose)
);

CREATE INDEX IF NOT EXISTS auth_email_challenges_expiry_idx
    ON auth_email_challenges (expires_at);

CREATE TABLE IF NOT EXISTS auth_password_reset_tokens (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS auth_password_reset_tokens_user_idx
    ON auth_password_reset_tokens (user_id);
CREATE INDEX IF NOT EXISTS auth_password_reset_tokens_expiry_idx
    ON auth_password_reset_tokens (expires_at);

CREATE TABLE IF NOT EXISTS auth_sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    ip_address INET,
    user_agent VARCHAR(512),
    created_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS auth_sessions_user_active_idx
    ON auth_sessions (user_id, expires_at)
    WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS auth_sessions_expiry_idx
    ON auth_sessions (expires_at);

CREATE TABLE IF NOT EXISTS auth_oidc_states (
    id UUID PRIMARY KEY,
    provider VARCHAR(64) NOT NULL,
    state_hash BYTEA NOT NULL UNIQUE,
    nonce_hash BYTEA NOT NULL,
    pkce_verifier_ciphertext BYTEA NOT NULL,
    return_to VARCHAR(1024) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS auth_oidc_states_expiry_idx
    ON auth_oidc_states (expires_at);

CREATE TABLE IF NOT EXISTS auth_audit_events (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES auth_users(id) ON DELETE SET NULL,
    event_type VARCHAR(64) NOT NULL,
    outcome VARCHAR(24) NOT NULL CHECK (outcome IN ('success', 'failure', 'blocked')),
    ip_address INET,
    user_agent VARCHAR(512),
    request_id VARCHAR(128),
    metadata JSONB NOT NULL DEFAULT '{}'::JSONB,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS auth_audit_events_user_time_idx
    ON auth_audit_events (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS auth_audit_events_type_time_idx
    ON auth_audit_events (event_type, created_at DESC);

COMMENT ON COLUMN auth_password_credentials.password_hash IS
    'Encoded Argon2id hash. Never stores a password or reversible secret.';
COMMENT ON COLUMN auth_email_challenges.secret_hash IS
    'HMAC digest only. Never stores a verification code.';
COMMENT ON COLUMN auth_password_reset_tokens.token_hash IS
    'HMAC digest only. Never stores a raw password reset token.';
COMMENT ON COLUMN auth_sessions.token_hash IS
    'SHA-256 digest only. Never stores a raw browser session token.';
COMMENT ON COLUMN auth_oidc_states.pkce_verifier_ciphertext IS
    'Encrypted at application layer using a separately managed key.';

COMMIT;
