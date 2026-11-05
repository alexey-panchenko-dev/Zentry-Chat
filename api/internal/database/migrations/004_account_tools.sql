CREATE TABLE password_resets (
 user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 token_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(token_hash)=32),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 expires_at TIMESTAMPTZ NOT NULL,
 CHECK (expires_at > created_at)
);

-- Self-pairs are private Saved Messages; the existing unique pair and membership
-- foreign keys continue to enforce one dialog and private message access.
ALTER TABLE conversations DROP CONSTRAINT conversations_check;
ALTER TABLE conversations ADD CONSTRAINT conversations_pair_order CHECK (user_low <= user_high);
