CREATE TABLE IF NOT EXISTS password_accounts (
    user_id UUID PRIMARY KEY REFERENCES access_credentials(id) ON DELETE CASCADE,
    email VARCHAR(254) NOT NULL UNIQUE CHECK (email = lower(btrim(email))),
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS browser_sessions (
    token_hash VARCHAR(64) PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES password_accounts(user_id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS browser_sessions_expiry ON browser_sessions(expires_at);
CREATE INDEX IF NOT EXISTS browser_sessions_user ON browser_sessions(user_id);

CREATE TABLE IF NOT EXISTS password_login_attempts (
    bucket VARCHAR(64) PRIMARY KEY,
    attempts INTEGER NOT NULL CHECK (attempts > 0),
    window_started TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS password_login_attempts_window ON password_login_attempts(window_started);
