CREATE TABLE abuse_reports (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 reporter_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 subject_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 scope text NOT NULL CHECK(scope IN ('user','message','group_message','discovery')),
 message_id bigint,
 category text NOT NULL CHECK(category IN ('spam','harassment','fraud','danger','other')),
 details text NOT NULL DEFAULT '' CHECK(length(details)<=1000),
 created_at timestamptz NOT NULL DEFAULT now(),
 CHECK(reporter_id<>subject_id)
);
CREATE UNIQUE INDEX abuse_reports_dedup ON abuse_reports(reporter_id,subject_id,scope,COALESCE(message_id,0));
CREATE INDEX abuse_reports_budget ON abuse_reports(reporter_id,created_at DESC);
