-- The official deployment grants runtime DML on business tables by default.
-- The migration ledger is readable for readiness/checksums but not runtime-writable.
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname='painter_app') THEN
  REVOKE INSERT,UPDATE,DELETE ON schema_migrations FROM painter_app;
 END IF;
END $$;
