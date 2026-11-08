ALTER TABLE group_chats
    ADD COLUMN avatar bytea,
    ADD COLUMN avatar_mime text,
    ADD COLUMN avatar_revision uuid NOT NULL DEFAULT gen_random_uuid();

ALTER TABLE group_chats
    ADD CONSTRAINT group_avatar_fields_consistent CHECK ((avatar IS NULL) = (avatar_mime IS NULL));
