-- 009_character_stats_energy_regen.sql
--
-- Store melee energy regeneration for classes that use energy. The upstream
-- stats API returns this as melee.energyRegen and omits it as null/zero for
-- mana-only characters.

ALTER TABLE character_stats ADD COLUMN IF NOT EXISTS energy_regen Float64 AFTER spirit_neg_buf;
