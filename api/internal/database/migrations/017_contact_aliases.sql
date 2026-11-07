CREATE TABLE contact_aliases (
    owner_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    peer_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    alias text NOT NULL CHECK (char_length(btrim(alias)) BETWEEN 1 AND 80),
    PRIMARY KEY(owner_id,peer_id),
    CHECK(owner_id<>peer_id)
);
