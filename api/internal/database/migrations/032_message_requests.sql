ALTER TABLE user_privacy DROP CONSTRAINT user_privacy_messages_check;
ALTER TABLE user_privacy ADD CONSTRAINT user_privacy_messages_check CHECK(messages IN ('everyone','contacts','nobody','requests'));
CREATE TABLE message_requests (
 conversation_id uuid PRIMARY KEY REFERENCES conversations(id) ON DELETE CASCADE,
 sender_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 recipient_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 status text NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','accepted','declined')),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 CHECK(sender_id<>recipient_id)
);
CREATE INDEX message_requests_recipient ON message_requests(recipient_id,status,created_at DESC);
