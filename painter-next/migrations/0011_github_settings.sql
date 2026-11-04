-- A missing row preserves deployment-environment credentials. An explicit row,
-- including a disabled row, is authoritative and never falls back implicitly.
CREATE TABLE github_auth_settings (
 id BIGINT PRIMARY KEY CHECK(id=1),
 version BIGINT NOT NULL CHECK(version>0),
 enabled BOOLEAN NOT NULL,
 client_id TEXT NOT NULL DEFAULT '',
 client_secret_ciphertext BYTEA,
 updated_at_ms BIGINT NOT NULL,
 CHECK(NOT enabled OR (client_id<>'' AND client_secret_ciphertext IS NOT NULL))
);
ALTER TABLE oauth_states ADD COLUMN provider_config_hash TEXT NOT NULL DEFAULT '';
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname='painter_app') THEN
  GRANT SELECT,INSERT,UPDATE,DELETE ON github_auth_settings TO painter_app;
 END IF;
END $$;
