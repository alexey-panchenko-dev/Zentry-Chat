-- Existing technical handles are replaced only when the chosen profile name
-- already is a valid, unique @username. Other accounts can choose one in profile.
WITH candidates AS (
    SELECT id, lower(name) AS handle
    FROM users
    WHERE username = 'u_' || replace(id::text, '-', '')
      AND lower(name) ~ '^[a-z][a-z0-9_]{2,39}$'
), unique_candidates AS (
    SELECT id, handle FROM candidates c
    WHERE (SELECT count(*) FROM candidates other WHERE other.handle = c.handle) = 1
      AND NOT EXISTS (SELECT 1 FROM users other WHERE other.id <> c.id AND other.username = c.handle)
)
UPDATE users u SET username = c.handle, name = c.handle
FROM unique_candidates c WHERE u.id = c.id;
