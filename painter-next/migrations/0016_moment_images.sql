-- Ordered draft/public image sets; publishing copies the saved draft set.
CREATE TABLE moment_images (
 moment_id BIGINT NOT NULL REFERENCES moments(id) ON DELETE CASCADE,
 scope TEXT NOT NULL CHECK(scope IN('draft','public')),
 position SMALLINT NOT NULL CHECK(position BETWEEN 0 AND 8),
 media_id TEXT NOT NULL REFERENCES media_assets(id) ON DELETE RESTRICT,
 alt TEXT NOT NULL DEFAULT '' CHECK(char_length(alt)<=200),
 PRIMARY KEY(moment_id,scope,position),
 UNIQUE(moment_id,scope,media_id)
);
CREATE INDEX moment_images_media_ref ON moment_images(media_id);

ALTER TABLE moments DROP CONSTRAINT moments_draft_text_check;
ALTER TABLE moments ADD CONSTRAINT moments_draft_text_check CHECK(char_length(draft_text)<=500);
ALTER TABLE moments DROP CONSTRAINT moments_public_text_check;
ALTER TABLE moments ADD CONSTRAINT moments_public_text_check CHECK(public_text IS NULL OR char_length(public_text)<=500);

-- Match Go strings.TrimSpace, including ideographic and non-breaking spaces.
CREATE FUNCTION painter_moment_has_text(value TEXT) RETURNS boolean LANGUAGE sql IMMUTABLE STRICT AS $$
 SELECT btrim(value,U&'\0009\000A\000B\000C\000D\0020\0085\00A0\1680\2000\2001\2002\2003\2004\2005\2006\2007\2008\2009\200A\2028\2029\202F\205F\3000')<>''
$$;
CREATE FUNCTION painter_moment_content_guard() RETURNS trigger LANGUAGE plpgsql VOLATILE AS $$
DECLARE target bigint; draft_value text; public_value text; deleted_ms bigint;
BEGIN
 IF TG_TABLE_NAME='moments' THEN
  IF TG_OP='DELETE' THEN target:=OLD.id; ELSE target:=NEW.id; END IF;
 ELSE
  IF TG_OP='DELETE' THEN target:=OLD.moment_id; ELSE target:=NEW.moment_id; END IF;
 END IF;
 SELECT draft_text,public_text,deleted_at_ms INTO draft_value,public_value,deleted_ms
 FROM moments WHERE id=target FOR UPDATE;
 IF NOT FOUND THEN RETURN NULL; END IF;
 IF NOT painter_moment_has_text(draft_value) AND NOT EXISTS(SELECT 1 FROM moment_images WHERE moment_id=target AND scope='draft') THEN
  RAISE EXCEPTION 'moment draft needs text or images' USING ERRCODE='23514';
 END IF;
 IF public_value IS NULL OR deleted_ms IS NOT NULL THEN
  IF EXISTS(SELECT 1 FROM moment_images WHERE moment_id=target AND scope='public') THEN
   RAISE EXCEPTION 'unpublished moment cannot have public images' USING ERRCODE='23514';
  END IF;
 ELSIF NOT painter_moment_has_text(public_value) AND NOT EXISTS(SELECT 1 FROM moment_images WHERE moment_id=target AND scope='public') THEN
  RAISE EXCEPTION 'moment publication needs text or images' USING ERRCODE='23514';
 END IF;
 RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER moments_content_guard AFTER INSERT OR UPDATE ON moments
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION painter_moment_content_guard();
CREATE CONSTRAINT TRIGGER moment_images_content_guard AFTER INSERT OR UPDATE OR DELETE ON moment_images
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION painter_moment_content_guard();

CREATE FUNCTION painter_moment_image_ready() RETURNS trigger LANGUAGE plpgsql VOLATILE AS $$
DECLARE current_state text; mime text;
BEGIN
 SELECT state,mime_type INTO current_state,mime FROM media_assets WHERE id=NEW.media_id FOR UPDATE;
 IF NOT FOUND OR current_state<>'ready' OR mime NOT IN('image/png','image/jpeg','image/gif','image/webp') THEN
  RAISE EXCEPTION 'moment image must be a ready supported image' USING ERRCODE='23514';
 END IF;
 IF TG_OP='UPDATE' AND (NEW.moment_id<>OLD.moment_id OR NEW.scope<>OLD.scope) THEN
  RAISE EXCEPTION 'moment image set identity immutable' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER moment_images_ready BEFORE INSERT OR UPDATE ON moment_images
 FOR EACH ROW EXECUTE FUNCTION painter_moment_image_ready();

-- The existing media-state guard still protects every older reference type.
-- This additional guard protects both moment snapshots, including soft deletes.
CREATE FUNCTION painter_moment_media_state_guard() RETURNS trigger LANGUAGE plpgsql VOLATILE AS $$
BEGIN
 IF NEW.state IN('deleting','deleted','failed') AND EXISTS(SELECT 1 FROM moment_images WHERE media_id=NEW.id) THEN
  RAISE EXCEPTION 'media referenced by moment' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER media_moment_reference_guard BEFORE UPDATE OF state ON media_assets
 FOR EACH ROW EXECUTE FUNCTION painter_moment_media_state_guard();

DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname='painter_app') THEN
  REVOKE ALL ON moment_images FROM painter_app;
  GRANT SELECT,INSERT,DELETE ON moment_images TO painter_app;
 END IF;
END $$;
