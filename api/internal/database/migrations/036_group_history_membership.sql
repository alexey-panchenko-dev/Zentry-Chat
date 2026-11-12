-- Preserve message authorship after a member leaves; send authorization stays transactional in the API.
ALTER TABLE group_messages DROP CONSTRAINT group_messages_group_id_sender_id_fkey;
ALTER TABLE group_messages ADD CONSTRAINT group_messages_sender_user_fkey FOREIGN KEY(sender_id) REFERENCES users(id) ON DELETE CASCADE;
