CREATE TABLE group_message_mentions (
 group_id uuid NOT NULL REFERENCES group_chats(id) ON DELETE CASCADE,
 message_id bigint NOT NULL REFERENCES group_messages(id) ON DELETE CASCADE,
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 token text NOT NULL CHECK(char_length(token) BETWEEN 2 AND 33),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(message_id,user_id)
);
CREATE INDEX group_message_mentions_user ON group_message_mentions(user_id,message_id);
