BEGIN;

ALTER TABLE site_settings
  ADD COLUMN rss_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN floating_controls JSONB NOT NULL DEFAULT '{"home":["theme","top","account"],"article":["theme","top","bottom","like","account"],"other":["theme","top","account"]}'::jsonb,
  ADD CONSTRAINT site_floating_controls_object CHECK (jsonb_typeof(floating_controls) = 'object');

COMMIT;
