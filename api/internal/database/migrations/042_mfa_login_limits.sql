CREATE TABLE mfa_login_limits (
 user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 attempts integer NOT NULL,
 resets_at timestamptz NOT NULL
);
