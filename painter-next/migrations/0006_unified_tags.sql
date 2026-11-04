-- Taxonomy is a single tag vocabulary. Historical category IDs and immutable
-- revision rows remain intact; these projections retain every old association.
CREATE TABLE tag_aliases (
 tag_id BIGINT PRIMARY KEY REFERENCES tags(id) ON DELETE RESTRICT,
 canonical_tag_id BIGINT NOT NULL REFERENCES tags(id) ON DELETE RESTRICT
);
CREATE TABLE category_tag_map (
 category_id BIGINT PRIMARY KEY REFERENCES categories(id) ON DELETE RESTRICT,
 tag_id BIGINT NOT NULL REFERENCES tags(id) ON DELETE RESTRICT
);
CREATE FUNCTION painter_taxonomy_key(label text) RETURNS text
 LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE AS $$
 SELECT trim(regexp_replace(normalize(casefold(normalize(label,NFKC) COLLATE pg_c_utf8),NFKC),'[[:space:]]+',' ','g'))
$$;

-- Also used once, inside the offline legacy import transaction. No category
-- writes are exposed by the running application.
CREATE FUNCTION painter_sync_unified_tags() RETURNS void LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO tags(name,description,archived_at_ms,created_at_ms,updated_at_ms)
 SELECT DISTINCT ON (painter_taxonomy_key(c.name)) c.name,c.description,c.archived_at_ms,c.created_at_ms,c.updated_at_ms
 FROM categories c
 WHERE NOT EXISTS(SELECT 1 FROM tags t WHERE painter_taxonomy_key(t.name)=painter_taxonomy_key(c.name))
 ORDER BY painter_taxonomy_key(c.name),c.id
 ON CONFLICT(name) DO NOTHING;
 INSERT INTO tag_aliases(tag_id,canonical_tag_id)
 SELECT id,min(id) OVER(PARTITION BY painter_taxonomy_key(name)) FROM tags
 ON CONFLICT(tag_id) DO UPDATE SET canonical_tag_id=EXCLUDED.canonical_tag_id;
 INSERT INTO category_tag_map(category_id,tag_id)
 SELECT c.id,min(a.canonical_tag_id) FROM categories c
 JOIN tags t ON painter_taxonomy_key(t.name)=painter_taxonomy_key(c.name)
 JOIN tag_aliases a ON a.tag_id=t.id GROUP BY c.id
 ON CONFLICT(category_id) DO UPDATE SET tag_id=EXCLUDED.tag_id;
 -- Merging an archived spelling with an active spelling must keep the visible
 -- tag selectable. All archived spellings still yield an archived tag.
 UPDATE tags t SET archived_at_ms=NULL
 WHERE t.archived_at_ms IS NOT NULL
 AND EXISTS(SELECT 1 FROM tag_aliases a WHERE a.canonical_tag_id=t.id)
 AND (EXISTS(SELECT 1 FROM tags old JOIN tag_aliases a ON a.tag_id=old.id WHERE a.canonical_tag_id=t.id AND old.archived_at_ms IS NULL)
 OR EXISTS(SELECT 1 FROM categories c JOIN category_tag_map m ON m.category_id=c.id WHERE m.tag_id=t.id AND c.archived_at_ms IS NULL));
END $$;

SELECT painter_sync_unified_tags();

CREATE VIEW effective_draft_tags AS
 SELECT d.post_id,COALESCE(a.canonical_tag_id,d.tag_id) AS tag_id
 FROM post_draft_tags d LEFT JOIN tag_aliases a ON a.tag_id=d.tag_id
 UNION
 SELECT d.post_id,m.tag_id FROM post_drafts d JOIN category_tag_map m ON m.category_id=d.category_id;
CREATE VIEW effective_revision_tags AS
 SELECT links.revision_id,links.tag_id,t.name AS tag_name FROM (
 SELECT r.revision_id,COALESCE(a.canonical_tag_id,r.tag_id) AS tag_id
 FROM post_revision_tags r LEFT JOIN tag_aliases a ON a.tag_id=r.tag_id
 UNION
 SELECT r.id,m.tag_id FROM post_revisions r JOIN category_tag_map m ON m.category_id=r.category_id
 ) links JOIN tags t ON t.id=links.tag_id;

-- New snapshots contain canonical tags, including a saved legacy category.
-- Keep historical category columns readable without creating new categories.
CREATE OR REPLACE FUNCTION painter_revision_snapshot() RETURNS trigger LANGUAGE plpgsql VOLATILE AS $$
DECLARE d post_drafts%ROWTYPE;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtextextended('painter:revision:'||NEW.id,0));
 PERFORM 1 FROM posts WHERE id=NEW.post_id FOR UPDATE;
 SELECT * INTO d FROM post_drafts WHERE post_id=NEW.post_id FOR UPDATE;
 IF NOT FOUND OR d.draft_version<>NEW.source_draft_version OR d.title<>NEW.title OR d.summary<>NEW.summary OR d.markdown<>NEW.markdown THEN
  RAISE EXCEPTION 'revision must snapshot current saved draft' USING ERRCODE='23514';
 END IF;
 IF NEW.category_id IS NOT NULL OR NEW.category_name IS NOT NULL THEN
  RAISE EXCEPTION 'use canonical tags for new revisions' USING ERRCODE='23514';
 END IF;
 PERFORM t.id FROM tags t JOIN effective_draft_tags dt ON dt.tag_id=t.id WHERE dt.post_id=NEW.post_id ORDER BY t.id FOR SHARE OF t;
 PERFORM m.id FROM media_assets m JOIN post_draft_media_refs dr ON dr.media_id=m.id WHERE dr.post_id=NEW.post_id ORDER BY m.id FOR UPDATE OF m;
 IF EXISTS(SELECT tag_id,tag_name FROM post_revision_tags WHERE revision_id=NEW.id
           EXCEPT SELECT dt.tag_id,t.name FROM effective_draft_tags dt JOIN tags t ON t.id=dt.tag_id WHERE dt.post_id=NEW.post_id)
 OR EXISTS(SELECT dt.tag_id,t.name FROM effective_draft_tags dt JOIN tags t ON t.id=dt.tag_id WHERE dt.post_id=NEW.post_id
           EXCEPT SELECT tag_id,tag_name FROM post_revision_tags WHERE revision_id=NEW.id) THEN
  RAISE EXCEPTION 'tag snapshot mismatch' USING ERRCODE='23514'; END IF;
 IF EXISTS(SELECT media_id FROM post_revision_media_refs WHERE revision_id=NEW.id EXCEPT SELECT media_id FROM post_draft_media_refs WHERE post_id=NEW.post_id)
 OR EXISTS(SELECT media_id FROM post_draft_media_refs WHERE post_id=NEW.post_id EXCEPT SELECT media_id FROM post_revision_media_refs WHERE revision_id=NEW.id) THEN
  RAISE EXCEPTION 'media snapshot mismatch' USING ERRCODE='23514'; END IF;
 RETURN NEW;
END $$;

DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname='painter_app') THEN
  -- Runtime reads these projections; the offline legacy importer reconciles
  -- mapping rows with INSERT/UPDATE. Neither path deletes historical aliases.
  GRANT SELECT,INSERT,UPDATE ON tag_aliases,category_tag_map TO painter_app;
  REVOKE DELETE ON tag_aliases,category_tag_map FROM painter_app;
  GRANT SELECT ON effective_draft_tags,effective_revision_tags TO painter_app;
 END IF;
END $$;
