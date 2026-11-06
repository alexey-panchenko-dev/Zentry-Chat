ALTER TABLE profile_photos ADD COLUMN revision UUID NOT NULL DEFAULT gen_random_uuid();
