CREATE TABLE personal_message_favorites (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    message_id BIGINT REFERENCES messages(id) ON DELETE CASCADE,
    group_message_id BIGINT REFERENCES group_messages(id) ON DELETE CASCADE,
    saved_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (num_nonnulls(message_id, group_message_id) = 1),
    UNIQUE (user_id, message_id),
    UNIQUE (user_id, group_message_id)
);
CREATE INDEX personal_favorites_recent ON personal_message_favorites(user_id, saved_at DESC);
