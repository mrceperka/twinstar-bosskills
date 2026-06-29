-- 001_init.sql — initial ClickHouse schema for twinstar-bosskills.
--
-- Layout:
--   raid, boss            — small lookup tables populated by the sync job from
--                           the upstream /bosskills/raids endpoint
--   boss_kill             — wide events table; per-kill detail (players, loot,
--                           deaths, timeline) is stored as Nested columns to
--                           avoid join overhead on read paths
--
-- All event tables use ReplacingMergeTree on a `version` column so re-syncs
-- update existing rows on the next merge. Reads MUST use FINAL or argMax-style
-- aggregates to see the latest version before merge has happened.

-- --------------------------------------------------------------------------
-- Lookup tables
-- --------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS raid (
    realm   LowCardinality(String),
    name    String,
    position UInt16 DEFAULT 1,
    version DateTime64(3) DEFAULT now64()
)
ENGINE = ReplacingMergeTree(version)
ORDER BY (realm, name);

CREATE TABLE IF NOT EXISTS boss (
    realm       LowCardinality(String),
    raid_name   String,
    remote_id   UInt32,
    name        String,
    position    UInt16 DEFAULT 1,
    version     DateTime64(3) DEFAULT now64()
)
ENGINE = ReplacingMergeTree(version)
ORDER BY (realm, remote_id);

-- --------------------------------------------------------------------------
-- Events: boss_kill
--
-- ORDER BY (realm, remote_id) is the dedup key. PARTITION BY kill_time month
-- keeps partitions roughly aligned with WoW expansion content cadence so
-- queries that scan one raid-lock touch one or two partitions only.
-- --------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS boss_kill (
    -- upstream id format is "<realm_id>_<bosskill_id>"
    remote_id       String,
    realm           LowCardinality(String),
    raid_name       String,
    boss_remote_id  UInt32,
    boss_name       String,
    mode            UInt8,
    guild           String,
    kill_time       DateTime,
    length          UInt32,
    wipes           UInt32,
    deaths          UInt32,
    ress_used       UInt32,

    -- ---- Nested per-player rows ----
    players Nested (
        guid             UInt64,
        talent_spec      UInt16,
        avg_item_lvl     Float32,
        dmg_done         UInt64,
        healing_done     UInt64,
        overhealing_done UInt64,
        absorb_done      UInt64,
        dmg_taken        UInt64,
        dmg_absorbed     UInt64,
        healing_taken    UInt64,
        dispels          UInt32,
        interrupts       UInt32,
        name             String,
        race             UInt8,
        class            UInt8,
        gender           UInt8,
        level            UInt8
    ),

    -- ---- Nested per-death/res events ----
    -- time < 0 means a resurrection.
    deaths_detail Nested (
        remote_id UInt32,
        guid      UInt64,
        time      Int32
    ),

    -- ---- Nested loot drops ----
    loot Nested (
        remote_id UInt32,
        item_id   UInt32,
        count     UInt8
    ),

    -- ---- Nested fight timeline ----
    timeline Nested (
        time             Int32,
        encounter_damage UInt64,
        encounter_heal   UInt64,
        raid_damage      UInt64,
        raid_heal        UInt64
    ),

    inserted_at DateTime64(3) DEFAULT now64(),
    version     DateTime64(3) DEFAULT now64()
)
ENGINE = ReplacingMergeTree(version)
PARTITION BY toYYYYMM(kill_time)
ORDER BY (realm, remote_id)
SETTINGS index_granularity = 8192;

-- Skip-index for common filter patterns.
ALTER TABLE boss_kill
    ADD INDEX IF NOT EXISTS bk_boss_idx       boss_remote_id TYPE bloom_filter GRANULARITY 4,
    ADD INDEX IF NOT EXISTS bk_guild_idx      guild          TYPE bloom_filter GRANULARITY 4,
    ADD INDEX IF NOT EXISTS bk_mode_idx       mode           TYPE set(64)      GRANULARITY 4;
