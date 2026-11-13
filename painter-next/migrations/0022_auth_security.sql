-- One account policy for local, OAuth and federated clients. Owners retain the
-- ability to sign in while public account access is closed.
ALTER TABLE site_settings ADD COLUMN external_login_enabled BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE sessions ADD COLUMN authentication_method TEXT NOT NULL DEFAULT 'legacy'
 CHECK(authentication_method IN('legacy','password','github','password+mfa','github+mfa'));
ALTER TABLE sessions ADD COLUMN mfa_authenticated_at_ms BIGINT;
ALTER TABLE sessions ADD COLUMN authenticated_at_ms BIGINT;

CREATE TABLE user_mfa (
 user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 secret_ciphertext BYTEA,
 enabled_at_ms BIGINT,
 last_totp_step BIGINT NOT NULL DEFAULT -1,
 pending_secret_ciphertext BYTEA,
 pending_expires_at_ms BIGINT,
 pending_attempts INTEGER NOT NULL DEFAULT 0 CHECK(pending_attempts>=0),
 failure_window_start_ms BIGINT NOT NULL DEFAULT 0,
 failed_attempts INTEGER NOT NULL DEFAULT 0 CHECK(failed_attempts>=0),
 locked_until_ms BIGINT NOT NULL DEFAULT 0,
 updated_at_ms BIGINT NOT NULL,
 CHECK((secret_ciphertext IS NULL)=(enabled_at_ms IS NULL)),
 CHECK((pending_secret_ciphertext IS NULL)=(pending_expires_at_ms IS NULL))
);
CREATE TABLE mfa_recovery_codes (
 user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 code_hash TEXT NOT NULL,
 created_at_ms BIGINT NOT NULL,
 consumed_at_ms BIGINT,
 PRIMARY KEY(user_id,code_hash)
);
CREATE TABLE mfa_login_challenges (
 id TEXT PRIMARY KEY,
 token_hash TEXT NOT NULL UNIQUE,
 user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 authentication_method TEXT NOT NULL CHECK(authentication_method IN('password','github')),
 user_version BIGINT NOT NULL,
 created_at_ms BIGINT NOT NULL,
 expires_at_ms BIGINT NOT NULL,
 attempts INTEGER NOT NULL DEFAULT 0 CHECK(attempts>=0),
 consumed_at_ms BIGINT
);
CREATE INDEX mfa_login_challenges_expiry ON mfa_login_challenges(expires_at_ms);
CREATE INDEX mfa_login_challenges_user ON mfa_login_challenges(user_id);
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname='painter_app') THEN
  GRANT SELECT,INSERT,UPDATE,DELETE ON user_mfa,mfa_recovery_codes,mfa_login_challenges TO painter_app;
 END IF;
END $$;
