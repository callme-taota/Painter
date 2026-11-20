-- Replies keep one visible level under a root while retaining the actual
-- comment they answer. Soft deletion never removes either relationship.
ALTER TABLE comments
 ADD COLUMN parent_id BIGINT,
 ADD COLUMN reply_to_id BIGINT,
 ADD CONSTRAINT comments_post_id_id_key UNIQUE(post_id,id),
 ADD CONSTRAINT comments_reply_shape CHECK (
  (parent_id IS NULL AND reply_to_id IS NULL) OR
  (parent_id IS NOT NULL AND reply_to_id IS NOT NULL AND parent_id<>id AND reply_to_id<>id)
 ),
 ADD CONSTRAINT comments_parent_same_post FOREIGN KEY(post_id,parent_id) REFERENCES comments(post_id,id) ON DELETE RESTRICT,
 ADD CONSTRAINT comments_reply_to_same_post FOREIGN KEY(post_id,reply_to_id) REFERENCES comments(post_id,id) ON DELETE RESTRICT;

CREATE INDEX comments_roots_public ON comments(post_id,created_at_ms,id) WHERE parent_id IS NULL AND state='visible';
CREATE INDEX comments_replies_public ON comments(parent_id,created_at_ms,id) WHERE state='visible';
CREATE INDEX comments_reply_to ON comments(reply_to_id) WHERE reply_to_id IS NOT NULL;

CREATE FUNCTION painter_comment_thread_guard() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE root_parent bigint; target_parent bigint;
BEGIN
 IF TG_OP='UPDATE' THEN
  IF NEW.id IS DISTINCT FROM OLD.id OR NEW.post_id IS DISTINCT FROM OLD.post_id OR
     NEW.user_id IS DISTINCT FROM OLD.user_id OR NEW.created_at_ms IS DISTINCT FROM OLD.created_at_ms OR
     NEW.parent_id IS DISTINCT FROM OLD.parent_id OR NEW.reply_to_id IS DISTINCT FROM OLD.reply_to_id THEN
   RAISE EXCEPTION 'comment identity and reply relationships are immutable' USING ERRCODE='23514';
  END IF;
 ELSIF NEW.parent_id IS NOT NULL THEN
  SELECT parent_id INTO root_parent FROM comments WHERE id=NEW.parent_id AND post_id=NEW.post_id;
  IF NOT FOUND OR root_parent IS NOT NULL THEN
   RAISE EXCEPTION 'comment parent must be a root in the same article' USING ERRCODE='23514';
  END IF;
  SELECT parent_id INTO target_parent FROM comments WHERE id=NEW.reply_to_id AND post_id=NEW.post_id;
  IF NOT FOUND OR (NEW.reply_to_id<>NEW.parent_id AND target_parent IS DISTINCT FROM NEW.parent_id) THEN
   RAISE EXCEPTION 'comment reply target must belong to the same root' USING ERRCODE='23514';
  END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER comments_thread_guard BEFORE INSERT OR UPDATE ON comments FOR EACH ROW EXECUTE FUNCTION painter_comment_thread_guard();
