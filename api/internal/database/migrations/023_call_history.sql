ALTER TABLE calls ADD COLUMN accepted_at TIMESTAMPTZ;

-- Preserve a best-effort start time for calls already in progress during migration.
UPDATE calls SET accepted_at=created_at WHERE status='accepted';
