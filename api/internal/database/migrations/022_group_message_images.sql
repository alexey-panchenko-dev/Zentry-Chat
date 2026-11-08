ALTER TABLE group_messages
    ADD COLUMN kind text NOT NULL DEFAULT 'text' CHECK (kind IN ('text','image')),
    ADD COLUMN image bytea,
    ADD COLUMN image_mime text,
    ADD CONSTRAINT group_message_image_fields_consistent CHECK (
        (kind = 'image' AND image IS NOT NULL AND image_mime = 'image/jpeg') OR
        (kind = 'text' AND image IS NULL AND image_mime IS NULL)
    );

ALTER TABLE group_messages DROP CONSTRAINT group_messages_body_check;
ALTER TABLE group_messages ADD CONSTRAINT group_message_body_valid CHECK (
    (kind = 'text' AND char_length(btrim(body)) BETWEEN 1 AND 1000) OR
    (kind = 'image' AND char_length(btrim(body)) BETWEEN 0 AND 1000)
);
