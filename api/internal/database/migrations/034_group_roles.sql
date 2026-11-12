ALTER TABLE group_members ADD COLUMN role text NOT NULL DEFAULT 'member' CHECK(role IN ('member','admin'));
