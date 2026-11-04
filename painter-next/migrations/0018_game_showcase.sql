-- Additive only: existing cover, text, configuration and account data remain.
-- Unknown lifetime playtime is NULL; old rounded subtitles are not statistics.
ALTER TABLE homepage_items
 ADD COLUMN hero_url TEXT NOT NULL DEFAULT '' CHECK(char_length(hero_url)<=2048),
 ADD COLUMN logo_url TEXT NOT NULL DEFAULT '' CHECK(char_length(logo_url)<=2048),
 ADD COLUMN playtime_minutes BIGINT CHECK(playtime_minutes IS NULL OR playtime_minutes BETWEEN 0 AND 52704000),
 ADD CONSTRAINT homepage_items_game_showcase CHECK(kind='steam' OR (hero_url='' AND logo_url='' AND playtime_minutes IS NULL));
CREATE INDEX homepage_games_playtime ON homepage_items(playtime_minutes DESC NULLS LAST,id)
 WHERE kind='steam' AND visible;
CREATE INDEX homepage_games_title ON homepage_items(title COLLATE "C",id)
 WHERE kind='steam' AND visible;
-- V22 stored the unrounded provider minutes in this exact subtitle format.
-- Its value-typed decoder also turned a missing value into zero: old zero is
-- therefore unknown until a fresh pointer-based sync confirms the statistic.
-- A locally edited subtitle is never treated as a provider fact. CASE protects
-- conversion even if the optimizer changes predicate evaluation order.
UPDATE homepage_items SET playtime_minutes=CASE WHEN subtitle ~ '^[0-9]{1,8} 分钟$'
 THEN split_part(subtitle,' ',1)::bigint ELSE NULL END
 WHERE source='steam_sync' AND NOT local_override
 AND CASE WHEN subtitle ~ '^[0-9]{1,8} 分钟$' THEN split_part(subtitle,' ',1)::bigint BETWEEN 1 AND 52704000 ELSE false END;
-- Existing table-level runtime SELECT/INSERT/UPDATE grants cover these columns.
-- No existing fields, visibility or display limits are rewritten.
