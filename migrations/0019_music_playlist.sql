-- Explicit previews of public Apple Music pages; no MusicKit credentials.
ALTER TABLE homepage_items DROP CONSTRAINT homepage_items_source_check;
ALTER TABLE homepage_items ADD CONSTRAINT homepage_items_source_check CHECK(source IN('manual','steam_sync','music_playlist'));
ALTER TABLE homepage_items ADD CONSTRAINT homepage_items_music_identity CHECK(source<>'music_playlist' OR (kind='music' AND provider_id ~ '^[1-9][0-9]{0,19}$'));
CREATE UNIQUE INDEX homepage_items_music_identity ON homepage_items(kind,source,provider_id) WHERE source='music_playlist';

CREATE TABLE homepage_music_playlist (
 id BIGINT PRIMARY KEY CHECK(id=1),
 version BIGINT NOT NULL DEFAULT 1 CHECK(version>0),
 source_url TEXT NOT NULL DEFAULT '' CHECK(char_length(source_url)<=2048),
 resolved_url TEXT NOT NULL DEFAULT '' CHECK(char_length(resolved_url)<=2048),
 playlist_id TEXT NOT NULL DEFAULT '' CHECK(char_length(playlist_id)<=64),
 title TEXT NOT NULL DEFAULT '' CHECK(char_length(title)<=200),
 track_count INTEGER NOT NULL DEFAULT 0 CHECK(track_count BETWEEN 0 AND 300),
 album_count INTEGER NOT NULL DEFAULT 0 CHECK(album_count BETWEEN 0 AND 300),
 partial BOOLEAN NOT NULL DEFAULT false,
 last_success_at_ms BIGINT,
 last_attempt_at_ms BIGINT,
 last_attempt_source_url TEXT NOT NULL DEFAULT '' CHECK(char_length(last_attempt_source_url)<=2048),
 last_error TEXT NOT NULL DEFAULT '' CHECK(char_length(last_error)<=240),
 attempt_token TEXT
);
INSERT INTO homepage_music_playlist(id) VALUES(1);

CREATE TABLE homepage_music_previews (
 id TEXT PRIMARY KEY CHECK(id ~ '^[0-9a-f]{48}$'),
 owner_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 snapshot JSONB NOT NULL CHECK(jsonb_typeof(snapshot)='object'),
 created_at_ms BIGINT NOT NULL,
 expires_at_ms BIGINT NOT NULL CHECK(expires_at_ms>created_at_ms)
);
CREATE INDEX homepage_music_previews_expiry ON homepage_music_previews(expires_at_ms);
CREATE INDEX homepage_music_previews_owner ON homepage_music_previews(owner_id,created_at_ms DESC,id);

-- Extend the existing audit allowlist without discarding earlier module actions.
DO $$ DECLARE existing text; BEGIN
 SELECT pg_get_expr(conbin,conrelid) INTO existing FROM pg_constraint WHERE conrelid='audit_logs'::regclass AND conname='audit_logs_action_check';
 IF existing IS NULL THEN RAISE EXCEPTION 'audit allowlist missing'; END IF;
 ALTER TABLE audit_logs DROP CONSTRAINT audit_logs_action_check;
 EXECUTE 'ALTER TABLE audit_logs ADD CONSTRAINT audit_logs_action_check CHECK (('||existing||') OR action=''home.music_import'')';
 IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname='painter_app') THEN
  REVOKE ALL ON homepage_music_playlist,homepage_music_previews FROM painter_app;
  GRANT SELECT,UPDATE ON homepage_music_playlist TO painter_app;
  GRANT SELECT,INSERT,DELETE ON homepage_music_previews TO painter_app;
 END IF;
END $$;
