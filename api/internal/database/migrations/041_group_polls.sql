CREATE TABLE group_polls (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 group_id uuid NOT NULL REFERENCES group_chats(id) ON DELETE CASCADE,
 message_id bigint UNIQUE NOT NULL REFERENCES group_messages(id) ON DELETE CASCADE,
 creator_id uuid NOT NULL REFERENCES users(id),
 question text NOT NULL CHECK(char_length(btrim(question)) BETWEEN 1 AND 300),
 multiple boolean NOT NULL DEFAULT false,
 closes_at timestamptz,
 closed_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX group_polls_group ON group_polls(group_id,created_at);
CREATE TABLE group_poll_options (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 poll_id uuid NOT NULL REFERENCES group_polls(id) ON DELETE CASCADE,
 label text NOT NULL CHECK(char_length(btrim(label)) BETWEEN 1 AND 100),
 position integer NOT NULL,
 UNIQUE(poll_id,id),UNIQUE(poll_id,position)
);
CREATE TABLE group_poll_votes (
 poll_id uuid NOT NULL,
 option_id uuid NOT NULL,
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 PRIMARY KEY(poll_id,option_id,user_id),
 FOREIGN KEY(poll_id,option_id) REFERENCES group_poll_options(poll_id,id) ON DELETE CASCADE
);
CREATE INDEX group_poll_votes_user ON group_poll_votes(poll_id,user_id);
