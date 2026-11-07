ALTER TABLE users ADD COLUMN username text;
UPDATE users SET username='u_'||replace(id::text,'-','');
ALTER TABLE users ALTER COLUMN username SET NOT NULL;
ALTER TABLE users ADD CONSTRAINT users_username_format CHECK (username ~ '^[a-z][a-z0-9_]{2,39}$' AND username=lower(username));
CREATE UNIQUE INDEX users_username_unique ON users(lower(username));
CREATE FUNCTION set_default_username() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.username IS NULL OR NEW.username='' THEN
        NEW.username='u_'||replace(NEW.id::text,'-','');
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER users_default_username BEFORE INSERT ON users FOR EACH ROW EXECUTE FUNCTION set_default_username();
