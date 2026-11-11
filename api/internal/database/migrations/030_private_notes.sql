CREATE TABLE private_note_chats (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    group_id uuid UNIQUE NOT NULL REFERENCES group_chats(id) ON DELETE CASCADE
);
