CREATE TABLE user_totp (
 user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 secret_cipher bytea NOT NULL,
 enabled boolean NOT NULL DEFAULT false,
 setup_verified boolean NOT NULL DEFAULT false,
 pending_until timestamptz NOT NULL,
 last_step bigint NOT NULL DEFAULT -1
);
CREATE TABLE totp_backup_codes (
 user_id uuid NOT NULL REFERENCES user_totp(user_id) ON DELETE CASCADE,
 code_hash bytea NOT NULL CHECK(octet_length(code_hash)=32),
 PRIMARY KEY(user_id,code_hash)
);
CREATE TABLE mfa_challenges (
 token_hash bytea PRIMARY KEY CHECK(octet_length(token_hash)=32),
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 expires_at timestamptz NOT NULL,
 attempts integer NOT NULL DEFAULT 0,
 user_agent text NOT NULL DEFAULT ''
);
CREATE INDEX mfa_challenges_user ON mfa_challenges(user_id,expires_at);
CREATE TABLE security_action_limits (
 user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 attempts integer NOT NULL,
 resets_at timestamptz NOT NULL
);
CREATE TABLE security_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 kind text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX security_events_user_time ON security_events(user_id,created_at DESC);
