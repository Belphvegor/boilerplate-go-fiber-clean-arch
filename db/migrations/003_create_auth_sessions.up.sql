CREATE TABLE IF NOT EXISTS auth_sessions (
    id VARCHAR(36) PRIMARY KEY,
    session_id VARCHAR(36) NOT NULL,
    user_id VARCHAR(36) NOT NULL,
    identity_id VARCHAR(36) NOT NULL,
    token_hash VARCHAR(64) NOT NULL,
    id_token TEXT NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    used_at TIMESTAMP NULL,
    revoked_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL,
    CONSTRAINT ux_auth_sessions_token_hash UNIQUE (token_hash)
);

CREATE INDEX idx_auth_sessions_session_id ON auth_sessions (session_id);
