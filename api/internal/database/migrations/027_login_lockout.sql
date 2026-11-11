CREATE TABLE auth_login_limits (
    email TEXT PRIMARY KEY,
    attempts SMALLINT NOT NULL CHECK (attempts BETWEEN 1 AND 3),
    locked_until TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT auth_login_limits_normalized CHECK (email = lower(btrim(email)))
);

CREATE INDEX auth_login_limits_expiry_idx ON auth_login_limits(locked_until);
