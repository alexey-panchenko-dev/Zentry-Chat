ALTER TABLE messages DROP CONSTRAINT messages_body_check;
ALTER TABLE messages ADD COLUMN kind TEXT NOT NULL DEFAULT 'text' CHECK(kind IN ('text','voice'));
ALTER TABLE messages ADD COLUMN audio BYTEA;
ALTER TABLE messages ADD COLUMN audio_mime TEXT;
ALTER TABLE messages ADD COLUMN duration_ms INTEGER;
ALTER TABLE messages ADD CONSTRAINT messages_content_check CHECK (
(kind='text' AND char_length(btrim(body)) BETWEEN 1 AND 1000 AND audio IS NULL AND duration_ms IS NULL)
OR (kind='voice' AND body='' AND octet_length(audio) BETWEEN 1 AND 3145728 AND audio_mime IN ('audio/webm','audio/ogg','audio/mp4') AND duration_ms BETWEEN 1 AND 120000));
CREATE TABLE profile_photos(user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE, data BYTEA NOT NULL CHECK(octet_length(data)<=2097152), mime TEXT NOT NULL CHECK(mime IN ('image/png','image/jpeg')));
