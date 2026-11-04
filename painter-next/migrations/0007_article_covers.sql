ALTER TABLE post_drafts ADD COLUMN cover JSONB NOT NULL DEFAULT '{"mode":"auto","template":"editorial","color":"ink"}'::jsonb;
ALTER TABLE post_revisions ADD COLUMN cover JSONB NOT NULL DEFAULT '{"mode":"auto","template":"editorial","color":"ink"}'::jsonb;
CREATE FUNCTION painter_cover_valid(c jsonb) RETURNS boolean LANGUAGE sql IMMUTABLE AS $$
 SELECT jsonb_typeof(c)='object'
 AND c->>'mode' IN('auto','upload','none')
 AND c->>'template' IN('editorial','geometric','minimal')
 AND c->>'color' IN('ink','moss','clay','plum')
 AND (c->>'mode'<>'upload' OR (jsonb_typeof(c->'media_id')='string' AND c->>'media_id' ~ '^[a-f0-9]{48}$'))
$$;
ALTER TABLE post_drafts ADD CONSTRAINT draft_cover_valid CHECK(painter_cover_valid(cover) IS TRUE);
ALTER TABLE post_revisions ADD CONSTRAINT revision_cover_valid CHECK(painter_cover_valid(cover) IS TRUE);
CREATE FUNCTION painter_cover_reference() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE c jsonb;
BEGIN
 IF TG_TABLE_NAME='post_drafts' THEN
  -- Deferred checks read the final saved value when a transaction updates twice.
  SELECT cover INTO c FROM post_drafts WHERE post_id=NEW.post_id;
  IF c->>'mode'='upload' AND NOT EXISTS(SELECT 1 FROM post_draft_media_refs WHERE post_id=NEW.post_id AND media_id=c->>'media_id') THEN
   RAISE EXCEPTION 'draft cover media reference missing' USING ERRCODE='23514'; END IF;
 ELSE
  SELECT cover INTO c FROM post_drafts WHERE post_id=NEW.post_id;
  IF NEW.cover IS DISTINCT FROM c THEN RAISE EXCEPTION 'cover snapshot mismatch' USING ERRCODE='23514'; END IF;
  IF NEW.cover->>'mode'='upload' AND NOT EXISTS(SELECT 1 FROM post_revision_media_refs WHERE revision_id=NEW.id AND media_id=NEW.cover->>'media_id') THEN
   RAISE EXCEPTION 'revision cover media reference missing' USING ERRCODE='23514'; END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE CONSTRAINT TRIGGER draft_cover_reference AFTER INSERT OR UPDATE ON post_drafts DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION painter_cover_reference();
CREATE TRIGGER revision_cover_snapshot BEFORE INSERT ON post_revisions FOR EACH ROW EXECUTE FUNCTION painter_cover_reference();
