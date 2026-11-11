ALTER TABLE users ADD COLUMN email_verified BOOLEAN NOT NULL DEFAULT TRUE;

CREATE TABLE auth_email_codes (
    email TEXT NOT NULL,
    purpose TEXT NOT NULL CHECK (purpose IN ('verify_email', 'reset_password')),
    code_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    resend_after TIMESTAMPTZ NOT NULL,
    attempts SMALLINT NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 5),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (email, purpose),
    CONSTRAINT auth_email_codes_normalized CHECK (email = lower(btrim(email)))
);

CREATE INDEX auth_email_codes_expiry_idx ON auth_email_codes(expires_at);
