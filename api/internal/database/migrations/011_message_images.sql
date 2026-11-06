ALTER TABLE messages ADD COLUMN image BYTEA;
ALTER TABLE messages ADD COLUMN image_mime TEXT;
ALTER TABLE messages DROP CONSTRAINT messages_kind_check;
ALTER TABLE messages ADD CONSTRAINT messages_kind_check CHECK (kind IN ('text','voice','system','image'));
ALTER TABLE messages DROP CONSTRAINT messages_content_check;
ALTER TABLE messages ADD CONSTRAINT messages_content_check CHECK (
 (kind='text' AND sender_id IS NOT NULL AND char_length(btrim(body)) BETWEEN 1 AND 1000 AND audio IS NULL AND audio_mime IS NULL AND duration_ms IS NULL AND image IS NULL AND image_mime IS NULL)
 OR (kind='voice' AND sender_id IS NOT NULL AND body='' AND audio IS NOT NULL AND audio_mime IS NOT NULL AND duration_ms IS NOT NULL AND octet_length(audio) BETWEEN 1 AND 3145728 AND audio_mime IN ('audio/webm','audio/ogg','audio/mp4') AND duration_ms BETWEEN 1 AND 120000 AND image IS NULL AND image_mime IS NULL)
 OR (kind='system' AND sender_id IS NULL AND body='Вы стали друзьями!' AND audio IS NULL AND audio_mime IS NULL AND duration_ms IS NULL AND image IS NULL AND image_mime IS NULL)
 OR (kind='image' AND sender_id IS NOT NULL AND body='' AND audio IS NULL AND audio_mime IS NULL AND duration_ms IS NULL AND image IS NOT NULL AND octet_length(image) BETWEEN 1 AND 4194304 AND image_mime IS NOT NULL AND image_mime='image/jpeg')
);

