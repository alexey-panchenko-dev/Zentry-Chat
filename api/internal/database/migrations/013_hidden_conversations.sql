ALTER TABLE conversation_members ADD COLUMN hidden BOOLEAN NOT NULL DEFAULT false;
-- Only a newly inserted message reopens chats. Idempotent ON CONFLICT retries
-- and read/update operations never change the member's private hidden state.
CREATE FUNCTION reopen_conversation_on_message() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 UPDATE conversation_members SET hidden=false WHERE conversation_id=NEW.conversation_id AND hidden;
 RETURN NEW;
END;
$$;
CREATE TRIGGER messages_reopen_conversation AFTER INSERT ON messages
FOR EACH ROW EXECUTE FUNCTION reopen_conversation_on_message();
