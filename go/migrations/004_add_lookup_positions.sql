-- 004_add_lookup_positions.sql
--
-- Adds display ordering columns matching the SvelteKit schema:
--   raid.position = raid release/display order
--   boss.position = encounter order within its raid
--
-- Existing rows read as 1 until the next sync inserts replacement lookup rows
-- with concrete positions. Avoid ALTER TABLE UPDATE mutations on these
-- ReplacingMergeTree tables.

ALTER TABLE raid
    ADD COLUMN IF NOT EXISTS position UInt16 DEFAULT 1;

ALTER TABLE boss
    ADD COLUMN IF NOT EXISTS position UInt16 DEFAULT 1;
