ALTER TABLE profile_photos
    ADD COLUMN profile_data BYTEA
    CHECK (profile_data IS NULL OR octet_length(profile_data) BETWEEN 1 AND 2097152),
    ADD COLUMN thumbnail_data BYTEA
    CHECK (thumbnail_data IS NULL OR octet_length(thumbnail_data) BETWEEN 1 AND 524288);
