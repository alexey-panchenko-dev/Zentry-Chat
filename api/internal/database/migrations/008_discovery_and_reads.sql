CREATE TABLE user_profiles (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    age INTEGER CHECK (age >= 0),
    bio TEXT NOT NULL DEFAULT '' CHECK (char_length(bio) <= 100),
    city TEXT NOT NULL DEFAULT '' CHECK (char_length(city) <= 100)
);
CREATE TABLE discovery_votes (
    actor_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    liked BOOLEAN NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (actor_id, target_id),
    CHECK (actor_id <> target_id)
);
CREATE INDEX discovery_incoming_idx ON discovery_votes(target_id, actor_id) WHERE liked;
ALTER TABLE conversation_members ADD COLUMN last_read_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE messages ALTER COLUMN sender_id DROP NOT NULL;
ALTER TABLE messages DROP CONSTRAINT messages_kind_check;
ALTER TABLE messages ADD CONSTRAINT messages_kind_check CHECK (kind IN ('text', 'voice', 'system'));
ALTER TABLE messages DROP CONSTRAINT messages_content_check;
ALTER TABLE messages ADD CONSTRAINT messages_content_check CHECK (
    (kind='text' AND sender_id IS NOT NULL AND char_length(btrim(body)) BETWEEN 1 AND 1000 AND audio IS NULL AND audio_mime IS NULL AND duration_ms IS NULL)
    OR (kind='voice' AND sender_id IS NOT NULL AND body='' AND audio IS NOT NULL AND audio_mime IS NOT NULL AND duration_ms IS NOT NULL
        AND octet_length(audio) BETWEEN 1 AND 3145728 AND audio_mime IN ('audio/webm','audio/ogg','audio/mp4') AND duration_ms BETWEEN 1 AND 120000)
    OR (kind='system' AND sender_id IS NULL AND body='Вы стали друзьями!' AND audio IS NULL AND audio_mime IS NULL AND duration_ms IS NULL)
);
CREATE TABLE discovery_matches (
    user_low UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_high UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    system_message_id BIGINT NOT NULL UNIQUE REFERENCES messages(id) ON DELETE CASCADE,
    PRIMARY KEY (user_low, user_high),
    CHECK (user_low < user_high)
);
CREATE INDEX messages_unread_idx ON messages(conversation_id, id) INCLUDE (sender_id);
