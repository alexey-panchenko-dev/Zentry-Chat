CREATE TABLE user_presence (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    last_seen timestamptz NOT NULL DEFAULT now()
);
