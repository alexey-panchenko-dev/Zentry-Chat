ALTER TABLE messages
    ADD COLUMN reply_to_id BIGINT REFERENCES messages(id) ON DELETE SET NULL,
    ADD COLUMN forwarded_from_id BIGINT REFERENCES messages(id) ON DELETE SET NULL,
    ADD COLUMN edited_at TIMESTAMPTZ,
    ADD COLUMN attachment BYTEA,
    ADD COLUMN attachment_mime TEXT,
    ADD COLUMN attachment_name TEXT;

ALTER TABLE messages DROP CONSTRAINT messages_kind_check;
ALTER TABLE messages ADD CONSTRAINT messages_kind_check
    CHECK (kind IN ('text','voice','system','image','file'));
ALTER TABLE messages DROP CONSTRAINT messages_content_check;
ALTER TABLE messages ADD CONSTRAINT messages_content_check CHECK (
    (kind='text' AND sender_id IS NOT NULL AND char_length(btrim(body)) BETWEEN 1 AND 1000 AND audio IS NULL AND audio_mime IS NULL AND duration_ms IS NULL AND image IS NULL AND image_mime IS NULL AND attachment IS NULL AND attachment_mime IS NULL AND attachment_name IS NULL)
 OR (kind='voice' AND sender_id IS NOT NULL AND body='' AND audio IS NOT NULL AND audio_mime IS NOT NULL AND duration_ms IS NOT NULL AND octet_length(audio) BETWEEN 1 AND 3145728 AND audio_mime IN ('audio/webm','audio/ogg','audio/mp4') AND duration_ms BETWEEN 1 AND 120000 AND image IS NULL AND image_mime IS NULL AND attachment IS NULL AND attachment_mime IS NULL AND attachment_name IS NULL)
 OR (kind='system' AND sender_id IS NULL AND body='Вы стали друзьями!' AND audio IS NULL AND audio_mime IS NULL AND duration_ms IS NULL AND image IS NULL AND image_mime IS NULL AND attachment IS NULL AND attachment_mime IS NULL AND attachment_name IS NULL)
 OR (kind='image' AND sender_id IS NOT NULL AND char_length(btrim(body)) <= 1000 AND audio IS NULL AND audio_mime IS NULL AND duration_ms IS NULL AND image IS NOT NULL AND octet_length(image) BETWEEN 1 AND 4194304 AND image_mime='image/jpeg' AND attachment IS NULL AND attachment_mime IS NULL AND attachment_name IS NULL)
 OR (kind='file' AND sender_id IS NOT NULL AND char_length(btrim(body)) <= 1000 AND audio IS NULL AND audio_mime IS NULL AND duration_ms IS NULL AND image IS NULL AND image_mime IS NULL AND attachment IS NOT NULL AND octet_length(attachment) BETWEEN 1 AND 15728640 AND attachment_mime IS NOT NULL AND char_length(attachment_name) BETWEEN 1 AND 180)
);

CREATE TABLE message_reactions (
    message_id BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    emoji TEXT NOT NULL CHECK (emoji IN ('❤️','👍','😂','😮','😢','🔥')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id, user_id)
);
CREATE INDEX message_reactions_message_idx ON message_reactions(message_id);

CREATE TABLE pinned_messages (
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    message_id BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    pinned_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (conversation_id, message_id)
);
CREATE INDEX pinned_messages_recent_idx ON pinned_messages(conversation_id, created_at DESC);

ALTER TABLE group_messages
    ADD COLUMN reply_to_id BIGINT REFERENCES group_messages(id) ON DELETE SET NULL,
    ADD COLUMN forwarded_from_id BIGINT REFERENCES group_messages(id) ON DELETE SET NULL,
    ADD COLUMN edited_at TIMESTAMPTZ,
    ADD COLUMN attachment BYTEA,
    ADD COLUMN attachment_mime TEXT,
    ADD COLUMN attachment_name TEXT;
ALTER TABLE group_messages DROP CONSTRAINT group_messages_kind_check;
ALTER TABLE group_messages ADD CONSTRAINT group_messages_kind_check
    CHECK (kind IN ('text','image','file'));
ALTER TABLE group_messages DROP CONSTRAINT group_message_image_fields_consistent;
ALTER TABLE group_messages ADD CONSTRAINT group_message_image_fields_consistent CHECK (
    (kind = 'image' AND image IS NOT NULL AND image_mime = 'image/jpeg') OR
    (kind IN ('text','file') AND image IS NULL AND image_mime IS NULL)
);
ALTER TABLE group_messages DROP CONSTRAINT group_message_body_valid;
ALTER TABLE group_messages ADD CONSTRAINT group_message_body_valid CHECK (
    (kind = 'text' AND char_length(btrim(body)) BETWEEN 1 AND 1000 AND attachment IS NULL AND attachment_mime IS NULL AND attachment_name IS NULL) OR
    (kind = 'image' AND char_length(btrim(body)) BETWEEN 0 AND 1000 AND attachment IS NULL AND attachment_mime IS NULL AND attachment_name IS NULL) OR
    (kind = 'file' AND char_length(btrim(body)) BETWEEN 0 AND 1000 AND attachment IS NOT NULL AND octet_length(attachment) BETWEEN 1 AND 15728640 AND attachment_mime IS NOT NULL AND char_length(attachment_name) BETWEEN 1 AND 180)
);
CREATE TABLE group_message_reactions (
    message_id BIGINT NOT NULL REFERENCES group_messages(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    emoji TEXT NOT NULL CHECK (emoji IN ('❤️','👍','😂','😮','😢','🔥')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id, user_id)
);
CREATE INDEX group_message_reactions_message_idx ON group_message_reactions(message_id);
CREATE TABLE pinned_group_messages (
    group_id UUID NOT NULL REFERENCES group_chats(id) ON DELETE CASCADE,
    message_id BIGINT NOT NULL REFERENCES group_messages(id) ON DELETE CASCADE,
    pinned_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, message_id)
);
CREATE INDEX pinned_group_messages_recent_idx ON pinned_group_messages(group_id, created_at DESC);
