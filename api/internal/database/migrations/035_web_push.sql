CREATE TABLE push_subscriptions (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 session_hash bytea NOT NULL REFERENCES sessions(token_hash) ON DELETE CASCADE,
 endpoint text UNIQUE NOT NULL,
 p256dh text NOT NULL,
 auth text NOT NULL,
 preview boolean NOT NULL DEFAULT false,
 groups boolean NOT NULL DEFAULT true,
 muted_chats uuid[] NOT NULL DEFAULT '{}',
 active_chat uuid,
 active_until timestamptz,
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX push_subscription_user ON push_subscriptions(user_id);
CREATE TABLE push_jobs (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 subscription_id bigint NOT NULL REFERENCES push_subscriptions(id) ON DELETE CASCADE,
 scope text NOT NULL,
 chat_id uuid NOT NULL,
 message_id bigint NOT NULL,
 attempts int NOT NULL DEFAULT 0,
 available_at timestamptz NOT NULL DEFAULT now(),
 locked_until timestamptz,
 UNIQUE(subscription_id,scope,message_id)
);
CREATE INDEX push_jobs_ready ON push_jobs(available_at);
CREATE FUNCTION queue_message_push() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.sender_id IS NULL THEN RETURN NEW; END IF;
 IF TG_TABLE_NAME='messages' THEN
  INSERT INTO push_jobs(subscription_id,scope,chat_id,message_id)
  SELECT p.id,'personal',NEW.conversation_id,NEW.id FROM push_subscriptions p JOIN conversation_members m ON m.user_id=p.user_id AND m.conversation_id=NEW.conversation_id
  WHERE p.user_id<>NEW.sender_id AND NOT (NEW.conversation_id=ANY(p.muted_chats)) ON CONFLICT DO NOTHING;
 ELSE
  INSERT INTO push_jobs(subscription_id,scope,chat_id,message_id)
  SELECT p.id,'group',NEW.group_id,NEW.id FROM push_subscriptions p JOIN group_members m ON m.user_id=p.user_id AND m.group_id=NEW.group_id
  WHERE p.user_id<>NEW.sender_id AND p.groups AND NOT (NEW.group_id=ANY(p.muted_chats)) ON CONFLICT DO NOTHING;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER queue_personal_push AFTER INSERT ON messages FOR EACH ROW EXECUTE FUNCTION queue_message_push();
CREATE TRIGGER queue_group_push AFTER INSERT ON group_messages FOR EACH ROW EXECUTE FUNCTION queue_message_push();
