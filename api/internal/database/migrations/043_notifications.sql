CREATE TABLE notification_preferences (
 user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 sound_messages boolean NOT NULL DEFAULT true,
 push_enabled boolean NOT NULL DEFAULT true,
 sound_calls boolean NOT NULL DEFAULT true,
 mentions_enabled boolean NOT NULL DEFAULT true,
 dnd_until timestamptz,
 dnd_mute_message_sound boolean NOT NULL DEFAULT true,
 dnd_suppress_push boolean NOT NULL DEFAULT true,
 dnd_mute_call_sound boolean NOT NULL DEFAULT true,
 dnd_suppress_mentions boolean NOT NULL DEFAULT false,
 quiet_enabled boolean NOT NULL DEFAULT false,
 quiet_days smallint NOT NULL DEFAULT 127 CHECK (quiet_days BETWEEN 0 AND 127),
 quiet_start time NOT NULL DEFAULT '23:00',
 quiet_end time NOT NULL DEFAULT '09:00',
 timezone text NOT NULL DEFAULT 'UTC',
 updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE chat_notification_preferences (
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 scope text NOT NULL CHECK(scope IN ('personal','group')),
 chat_id uuid NOT NULL,
 mode text NOT NULL DEFAULT 'all' CHECK(mode IN ('all','silent','mentions','off')),
 muted_until timestamptz,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(user_id,scope,chat_id)
);
CREATE INDEX chat_notification_preferences_chat ON chat_notification_preferences(scope,chat_id);

CREATE TABLE notification_exceptions (
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 contact_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 allow_dnd_push boolean NOT NULL DEFAULT false,
 allow_call_sound boolean NOT NULL DEFAULT false,
 important boolean NOT NULL DEFAULT false,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(user_id,contact_id),
 CHECK(user_id<>contact_id)
);

CREATE OR REPLACE FUNCTION queue_message_push() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.sender_id IS NULL THEN RETURN NEW; END IF;
 IF TG_TABLE_NAME='messages' THEN
  INSERT INTO push_jobs(subscription_id,scope,chat_id,message_id)
  SELECT p.id,'personal',NEW.conversation_id,NEW.id FROM push_subscriptions p
   JOIN conversation_members m ON m.user_id=p.user_id AND m.conversation_id=NEW.conversation_id
  WHERE p.user_id<>NEW.sender_id AND (SELECT count(*) FROM push_jobs j WHERE j.subscription_id=p.id)<100
   AND EXISTS(SELECT 1 FROM sessions s WHERE s.token_hash=p.session_hash AND s.expires_at>now()) ON CONFLICT DO NOTHING;
 ELSE
  INSERT INTO push_jobs(subscription_id,scope,chat_id,message_id)
  SELECT p.id,'group',NEW.group_id,NEW.id FROM push_subscriptions p
   JOIN group_members m ON m.user_id=p.user_id AND m.group_id=NEW.group_id
  WHERE p.user_id<>NEW.sender_id AND (SELECT count(*) FROM push_jobs j WHERE j.subscription_id=p.id)<100
   AND EXISTS(SELECT 1 FROM sessions s WHERE s.token_hash=p.session_hash AND s.expires_at>now()) ON CONFLICT DO NOTHING;
 END IF;
 RETURN NEW;
END $$;
