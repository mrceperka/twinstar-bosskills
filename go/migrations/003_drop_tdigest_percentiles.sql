-- 003_drop_tdigest_percentiles.sql
--
-- Drops the t-digest percentile MV and target table. At the workload's
-- current data volume (max ~35 samples per spec/boss/mode cell), exact
-- percentile computation via `boss_kill ARRAY JOIN players` is both faster
-- to write against and *more accurate* (t-digest was off by up to 4% on the
-- median for small-sample specs, exact on the tails).
--
-- The other three MVs (character, character_boss_rankings, raid_lock_rankings)
-- stay because their aggregates (argMax identity, max DPS/HPS) are idempotent
-- and benefit from incremental maintenance regardless of volume.

DROP VIEW  IF EXISTS mv_character_boss_percentiles;
DROP TABLE IF EXISTS character_boss_percentiles;
