CREATE TABLE profile_contacts (
 user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 phone TEXT NOT NULL DEFAULT '' CHECK(phone='' OR phone ~ '^\+[0-9]{8,15}$'),
 show_phone BOOLEAN NOT NULL DEFAULT false,
 show_email BOOLEAN NOT NULL DEFAULT false,
 CHECK(phone<>'' OR NOT show_phone)
);
