-- Page presentation has its own revision so saving contact details cannot
-- overwrite a concurrently edited author background.
CREATE TABLE author_page_profiles (
 user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 background_preset TEXT NOT NULL DEFAULT 'paper'
  CHECK(background_preset IN('paper','mist','ink','rose')),
 background_media_id TEXT REFERENCES media_assets(id) ON DELETE RESTRICT,
 version BIGINT NOT NULL DEFAULT 1 CHECK(version>0),
 updated_at_ms BIGINT NOT NULL
);
CREATE INDEX author_page_profiles_background_ref ON author_page_profiles(background_media_id)
 WHERE background_media_id IS NOT NULL;

DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname='painter_app') THEN
  GRANT SELECT,INSERT,UPDATE,DELETE ON author_page_profiles TO painter_app;
 END IF;
END $$;
