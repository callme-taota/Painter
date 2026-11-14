-- Local credentials are opt-in per existing user; OAuth identities stay intact.
-- The users_username_ci unique index remains the canonical case-folded identity.
CREATE TABLE local_accounts (
 user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 password_hash TEXT NOT NULL CHECK(password_hash LIKE '$argon2id$v=19$m=19456,t=2,p=1$%'),
 password_changed_at_ms BIGINT NOT NULL,
 created_at_ms BIGINT NOT NULL
);
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname='painter_app') THEN
  GRANT SELECT,INSERT,UPDATE,DELETE ON local_accounts TO painter_app;
 END IF;
END $$;
