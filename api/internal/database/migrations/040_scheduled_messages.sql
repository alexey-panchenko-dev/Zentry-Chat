CREATE TABLE scheduled_messages (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 sender_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 conversation_id uuid REFERENCES conversations(id) ON DELETE CASCADE,
 group_id uuid REFERENCES group_chats(id) ON DELETE CASCADE,
 client_message_id uuid NOT NULL,
 body text NOT NULL CHECK(char_length(btrim(body)) BETWEEN 1 AND 1000),
 send_at timestamptz NOT NULL,
 retry_at timestamptz NOT NULL DEFAULT now(),
 status text NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','sent','cancelled','failed')),
 attempts integer NOT NULL DEFAULT 0,
 message_id bigint,
 failure text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 CHECK((conversation_id IS NOT NULL)::int+(group_id IS NOT NULL)::int=1),
 UNIQUE(sender_id,client_message_id)
);
CREATE INDEX scheduled_due_idx ON scheduled_messages(send_at,retry_at) WHERE status='pending';
CREATE INDEX scheduled_owner_idx ON scheduled_messages(sender_id,send_at);
