-- A restore point must preserve an unfinished working copy, including an empty
-- title. Publishing/manual checkpoints continue requiring a title in the app.
ALTER TABLE post_revisions DROP CONSTRAINT post_revisions_title_check;
ALTER TABLE post_revisions ADD CONSTRAINT post_revisions_title_check
 CHECK(length(trim(title))>0 OR reason='before_restore');
