-- Keep the existing pagination cursor, but advance it on every new message,
-- including photos, voice and files, regardless of who sent it.
CREATE FUNCTION advance_conversation_position() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    UPDATE conversations SET position=DEFAULT WHERE id=NEW.conversation_id;
    RETURN NEW;
END;
$$;

CREATE TRIGGER conversation_message_activity
AFTER INSERT ON messages FOR EACH ROW
EXECUTE FUNCTION advance_conversation_position();

-- Reorder existing conversations using their latest message, oldest first.
DO $$
DECLARE item record;
BEGIN
    FOR item IN
        SELECT c.id FROM conversations c
        LEFT JOIN LATERAL (SELECT max(id) AS last_id FROM messages WHERE conversation_id=c.id) m ON true
        ORDER BY m.last_id ASC NULLS FIRST,c.position ASC
    LOOP
        UPDATE conversations SET position=DEFAULT WHERE id=item.id;
    END LOOP;
END;
$$;
