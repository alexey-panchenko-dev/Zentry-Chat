CREATE EXTENSION IF NOT EXISTS pg_trgm;

ALTER TABLE user_profiles DROP CONSTRAINT user_profiles_bio_check;
ALTER TABLE user_profiles ADD CONSTRAINT user_profiles_bio_check CHECK (char_length(bio) <= 200);

-- One random key per profile: stable shuffle and indexed keyset pagination.
ALTER TABLE user_profiles ADD COLUMN discovery_key UUID NOT NULL DEFAULT gen_random_uuid();
CREATE INDEX user_profiles_discovery_key_idx ON user_profiles(discovery_key);

CREATE FUNCTION discovery_normalize(value text) RETURNS text
LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE AS $$
 SELECT btrim(regexp_replace(translate(lower(value), 'ё', 'е'), '[^[:alnum:]]+', ' ', 'g'))
$$;
ALTER TABLE user_profiles ADD COLUMN discovery_city text
 GENERATED ALWAYS AS (discovery_normalize(city)) STORED;
CREATE INDEX user_profiles_discovery_city_key_idx ON user_profiles(discovery_city, discovery_key);

CREATE FUNCTION discovery_terms(value text) RETURNS text[]
LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE AS $$
 SELECT COALESCE(array_agg(DISTINCT word ORDER BY word), ARRAY[]::text[])
 FROM regexp_split_to_table(discovery_normalize(value), ' +') AS word
 WHERE char_length(word) >= 4 AND NOT (word = ANY(ARRAY[
 'люблю','любить','обожаю','нравится','ненавижу','лучшее','лучший','лучшая',
 'очень','просто','хочу','ищу','себя','себе','меня','мной','тебя','тебе',
 'тоже','есть','быть','бывает','будет','буду','всем','всех','всего','всегда',
 'человек','людей','людьми','общение','общаться','поговорить','привет',
 'like','love','hate','enjoy','best','very','just','want','with','about',
 'this','that','have','your','myself','hello','people','chat','talk'
 ]))
$$;

-- Indexed vocabulary avoids comparing every pair of complete descriptions.
CREATE TABLE discovery_profile_terms (
 user_id UUID NOT NULL REFERENCES user_profiles(user_id) ON DELETE CASCADE,
 term TEXT NOT NULL,
 PRIMARY KEY(user_id, term)
);
CREATE INDEX discovery_profile_terms_exact_idx ON discovery_profile_terms(term, user_id);
CREATE INDEX discovery_profile_terms_trgm_idx ON discovery_profile_terms USING gin(term gin_trgm_ops);

CREATE FUNCTION discovery_refresh_terms() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 DELETE FROM discovery_profile_terms WHERE user_id=NEW.user_id;
 INSERT INTO discovery_profile_terms(user_id,term)
 SELECT NEW.user_id, unnest(discovery_terms(NEW.bio));
 RETURN NEW;
END
$$;
CREATE TRIGGER discovery_profile_terms_refresh AFTER INSERT OR UPDATE OF bio ON user_profiles
 FOR EACH ROW EXECUTE FUNCTION discovery_refresh_terms();
INSERT INTO discovery_profile_terms(user_id,term)
 SELECT user_id, unnest(discovery_terms(bio)) FROM user_profiles;
