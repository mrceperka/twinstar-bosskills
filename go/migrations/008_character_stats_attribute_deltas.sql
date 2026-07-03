-- 008_character_stats_attribute_deltas.sql
--
-- Store attribute positive and negative buff deltas as explicit display
-- columns so the stats fragment can render them without parsing raw JSON.

ALTER TABLE character_stats ADD COLUMN IF NOT EXISTS stamina_pos_buf UInt32 AFTER spirit;
ALTER TABLE character_stats ADD COLUMN IF NOT EXISTS stamina_neg_buf UInt32 AFTER stamina_pos_buf;
ALTER TABLE character_stats ADD COLUMN IF NOT EXISTS strength_pos_buf UInt32 AFTER stamina_neg_buf;
ALTER TABLE character_stats ADD COLUMN IF NOT EXISTS strength_neg_buf UInt32 AFTER strength_pos_buf;
ALTER TABLE character_stats ADD COLUMN IF NOT EXISTS agility_pos_buf UInt32 AFTER strength_neg_buf;
ALTER TABLE character_stats ADD COLUMN IF NOT EXISTS agility_neg_buf UInt32 AFTER agility_pos_buf;
ALTER TABLE character_stats ADD COLUMN IF NOT EXISTS intellect_pos_buf UInt32 AFTER agility_neg_buf;
ALTER TABLE character_stats ADD COLUMN IF NOT EXISTS intellect_neg_buf UInt32 AFTER intellect_pos_buf;
ALTER TABLE character_stats ADD COLUMN IF NOT EXISTS spirit_pos_buf UInt32 AFTER intellect_neg_buf;
ALTER TABLE character_stats ADD COLUMN IF NOT EXISTS spirit_neg_buf UInt32 AFTER spirit_pos_buf;
