ALTER TABLE conversation_members ADD COLUMN cleared_through BIGINT NOT NULL DEFAULT 0 CHECK(cleared_through>=0);
CREATE TABLE contact_blocks (
 blocker_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 blocked_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(blocker_id,blocked_id),
 CHECK(blocker_id<>blocked_id)
);
CREATE INDEX contact_blocks_reverse_idx ON contact_blocks(blocked_id,blocker_id);
