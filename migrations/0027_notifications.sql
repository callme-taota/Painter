-- Reference-only inbox: no cached comment, title or actor profile can outlive
-- the source's current visibility. Existing comments are not backfilled.
CREATE TABLE notifications (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 recipient_user_id BIGINT NOT NULL REFERENCES users(id),
 comment_id BIGINT NOT NULL REFERENCES comments(id),
 kind TEXT NOT NULL CHECK (kind IN ('comment','reply')),
 created_at_ms BIGINT NOT NULL,
 read_at_ms BIGINT,
 UNIQUE(recipient_user_id,comment_id)
);
CREATE INDEX notifications_recipient_page ON notifications(recipient_user_id,created_at_ms DESC,id DESC);
CREATE INDEX notifications_unread ON notifications(recipient_user_id) WHERE read_at_ms IS NULL;
