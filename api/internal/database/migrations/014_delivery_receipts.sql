ALTER TABLE conversation_members ADD COLUMN received_through bigint NOT NULL DEFAULT 0;
ALTER TABLE conversation_members ADD COLUMN viewed_through bigint NOT NULL DEFAULT 0;
UPDATE conversation_members SET viewed_through=CASE WHEN last_read_id>cleared_through THEN last_read_id ELSE 0 END,received_through=last_read_id;
