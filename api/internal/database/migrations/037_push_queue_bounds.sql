CREATE OR REPLACE FUNCTION queue_message_push() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.sender_id IS NULL THEN RETURN NEW; END IF;
 IF TG_TABLE_NAME='messages' THEN
  INSERT INTO push_jobs(subscription_id,scope,chat_id,message_id)
  SELECT p.id,'personal',NEW.conversation_id,NEW.id FROM push_subscriptions p JOIN conversation_members m ON m.user_id=p.user_id AND m.conversation_id=NEW.conversation_id
  WHERE (SELECT count(*) FROM push_jobs j WHERE j.subscription_id=p.id)<100 AND EXISTS(SELECT 1 FROM sessions s WHERE s.token_hash=p.session_hash AND s.expires_at>now()) AND p.user_id<>NEW.sender_id AND NOT (NEW.conversation_id=ANY(p.muted_chats)) ON CONFLICT DO NOTHING;
 ELSE
  INSERT INTO push_jobs(subscription_id,scope,chat_id,message_id)
  SELECT p.id,'group',NEW.group_id,NEW.id FROM push_subscriptions p JOIN group_members m ON m.user_id=p.user_id AND m.group_id=NEW.group_id
  WHERE p.user_id<>NEW.sender_id AND p.groups AND NOT (NEW.group_id=ANY(p.muted_chats)) ON CONFLICT DO NOTHING;
 END IF;
 RETURN NEW;
END $$;
