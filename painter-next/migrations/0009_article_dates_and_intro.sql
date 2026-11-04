ALTER TABLE site_profile ADD COLUMN intro_name TEXT NOT NULL DEFAULT '';
ALTER TABLE site_profile ADD COLUMN intro_roles_json JSONB NOT NULL DEFAULT '[]'::jsonb
 CHECK(jsonb_typeof(intro_roles_json)='array' AND jsonb_array_length(intro_roles_json)<=12);

CREATE FUNCTION painter_date_display_valid(c jsonb) RETURNS boolean LANGUAGE sql IMMUTABLE AS $$
 SELECT jsonb_typeof(c)='object'
 AND jsonb_typeof(c->'created')='boolean'
 AND jsonb_typeof(c->'updated')='boolean'
 AND jsonb_typeof(c->'published')='boolean'
 AND (c - 'created' - 'updated' - 'published')='{}'::jsonb
$$;
ALTER TABLE post_drafts ADD COLUMN date_display JSONB NOT NULL
 DEFAULT '{"created":true,"updated":true,"published":true}'::jsonb
 CHECK(painter_date_display_valid(date_display) IS TRUE);
ALTER TABLE post_revisions ADD COLUMN date_display JSONB NOT NULL
 DEFAULT '{"created":true,"updated":true,"published":true}'::jsonb
 CHECK(painter_date_display_valid(date_display) IS TRUE);
ALTER TABLE post_revisions ADD COLUMN content_updated_at_ms BIGINT
 CHECK(content_updated_at_ms IS NULL OR content_updated_at_ms>0);

-- Backfill only timestamps proven by the exact surviving source draft version.
-- No old content is rewritten and no guessed publication time is used as edit time.
-- The migration owns the table; the immutability guard is re-enabled in this
-- same transaction before ordinary application traffic can observe the schema.
ALTER TABLE post_revisions DISABLE TRIGGER revisions_immutable;
UPDATE post_revisions r SET content_updated_at_ms=d.updated_at_ms
 FROM post_drafts d WHERE d.post_id=r.post_id
 AND d.draft_version=r.source_draft_version AND d.updated_at_ms<=r.created_at_ms
 AND (r.reason<>'import' OR EXISTS(SELECT 1 FROM post_publications pub
  WHERE pub.revision_id=r.id AND d.updated_at_ms<=pub.published_at_ms));
ALTER TABLE post_revisions ENABLE TRIGGER revisions_immutable;

CREATE FUNCTION painter_revision_dates_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE d post_drafts%ROWTYPE;
BEGIN
 PERFORM 1 FROM posts WHERE id=NEW.post_id FOR UPDATE;
 SELECT * INTO d FROM post_drafts WHERE post_id=NEW.post_id FOR UPDATE;
 IF NEW.date_display IS DISTINCT FROM d.date_display
 OR ((NEW.reason<>'import' OR NEW.content_updated_at_ms IS NOT NULL)
 AND NEW.content_updated_at_ms IS DISTINCT FROM d.updated_at_ms) THEN
  RAISE EXCEPTION 'revision dates must snapshot saved draft' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER revision_dates_snapshot BEFORE INSERT ON post_revisions
 FOR EACH ROW EXECUTE FUNCTION painter_revision_dates_snapshot();

CREATE FUNCTION painter_post_created_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.created_at_ms IS DISTINCT FROM OLD.created_at_ms THEN
  RAISE EXCEPTION 'post creation time is immutable' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER post_created_guard BEFORE UPDATE ON posts
 FOR EACH ROW EXECUTE FUNCTION painter_post_created_guard();
