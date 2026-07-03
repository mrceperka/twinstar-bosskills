-- 007_character_stats_rating_components.sql
--
-- Compatibility migration for databases where 006_character_stats.sql was
-- applied before rating-style stats were expanded from percent-only columns to
-- explicit base/percent/rating triples.

ALTER TABLE character_stats ADD COLUMN IF NOT EXISTS hit_base UInt32 AFTER spell_power;
ALTER TABLE character_stats ADD COLUMN IF NOT EXISTS hit_rating Float32 AFTER hit_percent;
ALTER TABLE character_stats ADD COLUMN IF NOT EXISTS crit_base UInt32 AFTER hit_rating;
ALTER TABLE character_stats ADD COLUMN IF NOT EXISTS crit_rating Float32 AFTER crit_percent;
ALTER TABLE character_stats ADD COLUMN IF NOT EXISTS haste_base UInt32 AFTER crit_rating;
ALTER TABLE character_stats ADD COLUMN IF NOT EXISTS haste_rating Float32 AFTER haste_percent;
ALTER TABLE character_stats ADD COLUMN IF NOT EXISTS mastery_base UInt32 AFTER haste_rating;
ALTER TABLE character_stats ADD COLUMN IF NOT EXISTS mastery_rating Float32 AFTER mastery_percent;
ALTER TABLE character_stats ADD COLUMN IF NOT EXISTS armor_base UInt32 AFTER mastery_rating;
ALTER TABLE character_stats ADD COLUMN IF NOT EXISTS armor_percent Float32 AFTER armor_base;
ALTER TABLE character_stats ADD COLUMN IF NOT EXISTS armor_rating Float32 AFTER armor_percent;
ALTER TABLE character_stats ADD COLUMN IF NOT EXISTS dodge_base UInt32 AFTER armor_rating;
ALTER TABLE character_stats ADD COLUMN IF NOT EXISTS dodge_rating Float32 AFTER dodge_percent;
ALTER TABLE character_stats ADD COLUMN IF NOT EXISTS parry_base UInt32 AFTER dodge_rating;
ALTER TABLE character_stats ADD COLUMN IF NOT EXISTS parry_rating Float32 AFTER parry_percent;
