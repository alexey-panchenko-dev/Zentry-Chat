CREATE TABLE user_privacy (
 user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 messages text NOT NULL DEFAULT 'everyone' CHECK(messages IN ('everyone','contacts','nobody')),
 calls text NOT NULL DEFAULT 'everyone' CHECK(calls IN ('everyone','contacts','nobody')),
 presence text NOT NULL DEFAULT 'everyone' CHECK(presence IN ('everyone','contacts','nobody')),
 search text NOT NULL DEFAULT 'everyone' CHECK(search IN ('everyone','contacts','nobody')),
 birthday text NOT NULL DEFAULT 'everyone' CHECK(birthday IN ('everyone','contacts','nobody')),
 updated_at timestamptz NOT NULL DEFAULT now()
);
