ALTER TABLE user_profiles ADD COLUMN birth_date DATE CHECK (birth_date >= DATE '1900-01-01');
