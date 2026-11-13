-- Embedded OIDC provider. Credentials are hashed; only the signing key is
-- encrypted with the deployment encryption key, which must accompany backups.
CREATE TABLE identity_clients (
 id TEXT PRIMARY KEY,
 name TEXT NOT NULL CHECK(length(name) BETWEEN 1 AND 100),
 kind TEXT NOT NULL CHECK(kind IN ('web','spa','native')),
 secret_hash TEXT NOT NULL DEFAULT '',
 redirect_uris JSONB NOT NULL CHECK(jsonb_typeof(redirect_uris)='array'),
 disabled_at_ms BIGINT,
 created_by BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 created_at_ms BIGINT NOT NULL,
 updated_at_ms BIGINT NOT NULL
);
CREATE TABLE identity_signing_keys (
 id TEXT PRIMARY KEY,
 private_key_ciphertext BYTEA NOT NULL,
 created_at_ms BIGINT NOT NULL
);
CREATE TABLE identity_auth_requests (
 id TEXT PRIMARY KEY,
 client_id TEXT NOT NULL REFERENCES identity_clients(id) ON DELETE CASCADE,
 request_json JSONB NOT NULL,
 user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
 session_id TEXT,
 code_hash TEXT UNIQUE,
 code_used_at_ms BIGINT,
 created_at_ms BIGINT NOT NULL,
 expires_at_ms BIGINT NOT NULL
);
CREATE INDEX identity_auth_expiration ON identity_auth_requests(expires_at_ms);
CREATE TABLE identity_grants (
 id TEXT PRIMARY KEY,
 client_id TEXT NOT NULL REFERENCES identity_clients(id) ON DELETE CASCADE,
 user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 session_id TEXT NOT NULL,
 created_at_ms BIGINT NOT NULL,
 expires_at_ms BIGINT NOT NULL,
 revoked_at_ms BIGINT
);
CREATE INDEX identity_grants_user ON identity_grants(user_id,client_id);
CREATE TABLE identity_tokens (
 id TEXT PRIMARY KEY,
 grant_id TEXT NOT NULL REFERENCES identity_grants(id) ON DELETE CASCADE,
 kind TEXT NOT NULL CHECK(kind IN ('access','refresh')),
 request_json JSONB NOT NULL,
 created_at_ms BIGINT NOT NULL,
 expires_at_ms BIGINT NOT NULL,
 used_at_ms BIGINT,
 revoked_at_ms BIGINT
);
CREATE INDEX identity_tokens_grant ON identity_tokens(grant_id);
CREATE INDEX identity_tokens_expiration ON identity_tokens(expires_at_ms);
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname='painter_app') THEN
  GRANT SELECT,INSERT,UPDATE,DELETE ON identity_clients,identity_signing_keys,identity_auth_requests,identity_grants,identity_tokens TO painter_app;
 END IF;
END $$;
