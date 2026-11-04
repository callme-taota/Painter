CREATE TABLE installation_state (
 id SMALLINT PRIMARY KEY CHECK(id=1),
 completed_at_ms BIGINT CHECK(completed_at_ms>0),
 completed_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
 CHECK(completed_at_ms IS NOT NULL OR completed_by IS NULL)
);
-- Existing identities are an installed site, even if every owner is disabled.
-- Upgrading must never expose an anonymous route to replace its administrators.
INSERT INTO installation_state(id,completed_at_ms,completed_by)
 SELECT 1,CASE WHEN EXISTS(SELECT 1 FROM users WHERE role='owner')
  OR EXISTS(SELECT 1 FROM local_accounts) OR EXISTS(SELECT 1 FROM oauth_accounts)
  THEN (extract(epoch FROM clock_timestamp())*1000)::bigint ELSE NULL END,
  (SELECT id FROM users WHERE role='owner' ORDER BY id LIMIT 1);
CREATE FUNCTION painter_installation_sealed() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.id<>OLD.id OR (OLD.completed_at_ms IS NOT NULL AND
  (NEW.completed_at_ms IS DISTINCT FROM OLD.completed_at_ms OR NEW.completed_by IS DISTINCT FROM OLD.completed_by)) THEN
  RAISE EXCEPTION 'installation cannot be reopened' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER installation_sealed BEFORE UPDATE ON installation_state
 FOR EACH ROW EXECUTE FUNCTION painter_installation_sealed();
CREATE TRIGGER installation_no_delete BEFORE DELETE ON installation_state
 FOR EACH ROW EXECUTE FUNCTION painter_immutable();
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname='painter_app') THEN
  GRANT SELECT,UPDATE ON installation_state TO painter_app;
 END IF;
END $$;
