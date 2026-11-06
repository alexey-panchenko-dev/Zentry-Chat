ALTER TABLE messages DROP CONSTRAINT messages_content_check;
ALTER TABLE messages ADD CONSTRAINT messages_content_check CHECK (
(kind='text' AND char_length(btrim(body)) BETWEEN 1 AND 1000 AND audio IS NULL AND audio_mime IS NULL AND duration_ms IS NULL)
OR (kind='voice' AND body='' AND audio IS NOT NULL AND audio_mime IS NOT NULL AND duration_ms IS NOT NULL
AND octet_length(audio) BETWEEN 1 AND 3145728 AND audio_mime IN ('audio/webm','audio/ogg','audio/mp4') AND duration_ms BETWEEN 1 AND 120000));
