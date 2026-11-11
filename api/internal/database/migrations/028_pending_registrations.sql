CREATE TABLE pending_registrations (
    email TEXT PRIMARY KEY,
    name TEXT NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 100),
    username TEXT NOT NULL UNIQUE CHECK (username ~ '^[a-z][a-z0-9_]{2,39}$' AND username=lower(username)),
    password_hash TEXT NOT NULL,
    birth_date DATE CHECK (birth_date >= DATE '1900-01-01'),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT pending_registrations_email_normalized CHECK (
        email=lower(btrim(email)) AND char_length(email) BETWEEN 3 AND 254
    )
);

INSERT INTO pending_registrations(email,name,username,password_hash,birth_date,expires_at)
SELECT u.email,u.name,u.username,u.password_hash,p.birth_date,now()+interval '10 minutes'
FROM users u
LEFT JOIN user_profiles p ON p.user_id=u.id
WHERE u.email_verified=false;

DELETE FROM users WHERE email_verified=false;
