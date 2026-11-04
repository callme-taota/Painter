-- Durable operator observations. Directory is private and only exposed by owner APIs.
-- A row means a complete, fsynced artifact was observed; it does not claim the
-- artifact still exists, that remote storage succeeded, or that production was restored.
CREATE TABLE backup_records (
 id TEXT PRIMARY KEY,
 directory TEXT NOT NULL CHECK(length(directory)>0),
 database_sha256 TEXT NOT NULL CHECK(database_sha256 ~ '^[0-9a-f]{64}$'),
 manifest_sha256 TEXT NOT NULL UNIQUE CHECK(manifest_sha256 ~ '^[0-9a-f]{64}$'),
 media_objects BIGINT NOT NULL CHECK(media_objects>=0),
 media_bytes BIGINT NOT NULL CHECK(media_bytes>=0),
 created_at_ms BIGINT NOT NULL CHECK(created_at_ms>0),
 restore_verified_at_ms BIGINT CHECK(restore_verified_at_ms>0),
 origin TEXT NOT NULL DEFAULT 'local' CHECK(origin IN('local','imported'))
);
CREATE INDEX backup_records_latest ON backup_records(created_at_ms DESC,id);
CREATE FUNCTION painter_backup_history() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'backup observation history cannot be deleted' USING ERRCODE='23514'; END IF;
 IF NEW.id IS DISTINCT FROM OLD.id OR NEW.directory IS DISTINCT FROM OLD.directory
 OR NEW.database_sha256 IS DISTINCT FROM OLD.database_sha256
 OR NEW.manifest_sha256 IS DISTINCT FROM OLD.manifest_sha256
 OR NEW.media_objects IS DISTINCT FROM OLD.media_objects OR NEW.media_bytes IS DISTINCT FROM OLD.media_bytes
 OR NEW.created_at_ms IS DISTINCT FROM OLD.created_at_ms OR NEW.origin IS DISTINCT FROM OLD.origin THEN
  RAISE EXCEPTION 'backup observation metadata immutable' USING ERRCODE='23514';
 END IF;
 IF OLD.restore_verified_at_ms IS NOT NULL AND
   (NEW.restore_verified_at_ms IS NULL OR NEW.restore_verified_at_ms<OLD.restore_verified_at_ms) THEN
  RAISE EXCEPTION 'restore observation cannot move backward' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER backup_records_history BEFORE UPDATE OR DELETE ON backup_records FOR EACH ROW EXECUTE FUNCTION painter_backup_history();
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname='painter_app') THEN
  GRANT SELECT,INSERT,UPDATE ON backup_records TO painter_app;
  REVOKE DELETE ON backup_records FROM painter_app;
 END IF;
END $$;
