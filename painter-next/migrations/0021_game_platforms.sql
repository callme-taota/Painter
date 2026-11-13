-- The existing steam collection key remains the one mixed-platform game wall.
-- Preserve every saved field and Steam connection; only label its old rows.
ALTER TABLE homepage_items
 ADD COLUMN platform TEXT NOT NULL DEFAULT '',
 ADD COLUMN platforms TEXT[] NOT NULL DEFAULT '{}';
UPDATE homepage_items SET platform='steam' WHERE kind='steam';

ALTER TABLE homepage_items DROP CONSTRAINT homepage_items_source_check;
ALTER TABLE homepage_items ADD CONSTRAINT homepage_items_source_check
 CHECK(source IN('manual','steam_sync','psn_sync','music_playlist'));
ALTER TABLE homepage_items ADD CONSTRAINT homepage_items_platform_check CHECK(
 (kind='steam' AND platform IN('steam','psn')) OR (kind<>'steam' AND platform='')
);
ALTER TABLE homepage_items ADD CONSTRAINT homepage_items_platforms_check CHECK(
 (platform='psn' OR platforms='{}'::text[]) AND platforms IN(
  '{}'::text[],ARRAY['PS4'],ARRAY['PS5'],ARRAY['PS4','PS5'],ARRAY['PS5','PS4']
 )
);
ALTER TABLE homepage_items ADD CONSTRAINT homepage_items_steam_platform_check
 CHECK(source<>'steam_sync' OR platform='steam');
ALTER TABLE homepage_items ADD CONSTRAINT homepage_items_psn_provider_check
 CHECK(platform<>'psn' OR provider_id ~ '^[A-Za-z0-9:_-]{1,64}$');
ALTER TABLE homepage_items ADD CONSTRAINT homepage_items_psn_identity_check
 CHECK(source<>'psn_sync' OR (kind='steam' AND platform='psn'));
-- Keep the released Steam partial index and its existing ON CONFLICT target.
CREATE UNIQUE INDEX homepage_items_psn_identity
 ON homepage_items(kind,source,provider_id) WHERE source='psn_sync';

-- Migration 17 generated an unnamed two-column image-presence CHECK. Find it
-- by the columns it protects, preserving the separate photo/media-ready guards.
DO $$ DECLARE old_check text; image_column smallint; media_column smallint; BEGIN
 SELECT attnum INTO image_column FROM pg_attribute
  WHERE attrelid='homepage_items'::regclass AND attname='image_url';
 SELECT attnum INTO media_column FROM pg_attribute
  WHERE attrelid='homepage_items'::regclass AND attname='media_id';
 SELECT conname INTO STRICT old_check FROM pg_constraint
  WHERE conrelid='homepage_items'::regclass AND contype='c'
   AND cardinality(conkey)=2 AND conkey @> ARRAY[image_column,media_column];
 EXECUTE format('ALTER TABLE homepage_items DROP CONSTRAINT %I',old_check);
END $$;
ALTER TABLE homepage_items ADD CONSTRAINT homepage_items_image_present CHECK(
 media_id IS NOT NULL OR image_url<>'' OR (kind='steam' AND platform='psn')
);

CREATE OR REPLACE FUNCTION painter_home_media_ready() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE s text; mime text;
BEGIN
 -- Existing Steam sync INSERTs intentionally remain compatible without a new
 -- explicit column. Only new legacy game rows receive this default.
 IF TG_OP='INSERT' AND NEW.kind='steam' AND NEW.platform='' THEN
  NEW.platform:='steam';
 END IF;
 IF NEW.media_id IS NOT NULL THEN
  SELECT state,mime_type INTO s,mime FROM media_assets WHERE id=NEW.media_id FOR UPDATE;
  IF NOT FOUND OR s<>'ready' OR mime NOT IN('image/png','image/jpeg','image/gif','image/webp') THEN
   RAISE EXCEPTION 'homepage cover must be ready image' USING ERRCODE='23514';
  END IF;
 END IF;
 IF TG_OP='UPDATE' AND (NEW.id<>OLD.id OR NEW.kind<>OLD.kind OR NEW.source<>OLD.source OR NEW.platform<>OLD.platform) THEN
  RAISE EXCEPTION 'homepage item identity immutable' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;

CREATE TABLE homepage_game_connections (
 provider TEXT PRIMARY KEY CHECK(provider='psn'),
 version BIGINT NOT NULL DEFAULT 1 CHECK(version>0),
 credential_version BIGINT NOT NULL DEFAULT 1 CHECK(credential_version>0),
 online_id TEXT NOT NULL DEFAULT '' CHECK(char_length(online_id)<=80),
 account_id TEXT NOT NULL DEFAULT '' CHECK(char_length(account_id)<=32),
 token_ciphertext BYTEA,
 auto_sync BOOLEAN NOT NULL DEFAULT false,
 sync_requested BOOLEAN NOT NULL DEFAULT false,
 last_sync_at_ms BIGINT,
 last_error TEXT NOT NULL DEFAULT '' CHECK(char_length(last_error)<=240),
 next_sync_at_ms BIGINT NOT NULL DEFAULT 0,
 lease_token TEXT,
 lease_until_ms BIGINT,
 sync_failures INTEGER NOT NULL DEFAULT 0 CHECK(sync_failures BETWEEN 0 AND 20),
 CHECK(NOT auto_sync OR (token_ciphertext IS NOT NULL AND account_id<>''))
);
INSERT INTO homepage_game_connections(provider) VALUES('psn');

ALTER TABLE homepage_provider_rates DROP CONSTRAINT homepage_provider_rates_provider_check;
ALTER TABLE homepage_provider_rates ADD CONSTRAINT homepage_provider_rates_provider_check
 CHECK(provider IN('steam','music','psn'));

-- Inherit all released actions, including music and homepage project events.
DO $$ DECLARE existing text; BEGIN
 SELECT pg_get_expr(conbin,conrelid) INTO existing FROM pg_constraint
  WHERE conrelid='audit_logs'::regclass AND conname='audit_logs_action_check';
 IF existing IS NULL THEN RAISE EXCEPTION 'audit allowlist missing'; END IF;
 ALTER TABLE audit_logs DROP CONSTRAINT audit_logs_action_check;
 EXECUTE 'ALTER TABLE audit_logs ADD CONSTRAINT audit_logs_action_check CHECK (('||existing||') OR action IN(''home.psn_connect'',''home.psn_update'',''home.psn_disconnect'',''home.psn_sync''))';
 IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname='painter_app') THEN
  REVOKE ALL ON homepage_game_connections FROM painter_app;
  GRANT SELECT,UPDATE ON homepage_game_connections TO painter_app;
 END IF;
END $$;
