-- 001_init.sql - ClickHouse schema for twinstar-bosskills.
--
-- This migration is intentionally squashed: it represents the complete schema
-- expected by the current Go application for fresh databases.
--
-- All mutable/event tables use ReplacingMergeTree on a `version` column so
-- re-syncs update existing rows on the next merge. Reads that need immediate
-- replacement visibility must use FINAL or argMax-style aggregation.

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
-- keeps partitions bounded while aligning with time-based lifecycle needs.
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

-- Skip-indexes for common filters that are not part of the ORDER BY key.
ALTER TABLE boss_kill
    ADD INDEX IF NOT EXISTS bk_boss_idx       boss_remote_id TYPE bloom_filter GRANULARITY 4,
    ADD INDEX IF NOT EXISTS bk_guild_idx      guild          TYPE bloom_filter GRANULARITY 4,
    ADD INDEX IF NOT EXISTS bk_mode_idx       mode           TYPE set(64)      GRANULARITY 4;

-- --------------------------------------------------------------------------
-- character - one row per (realm, guid) with most-recent identity + counts
-- --------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS character (
    realm            LowCardinality(String),
    guid             UInt64,
    name_state       AggregateFunction(argMax, String, DateTime),
    race_state       AggregateFunction(argMax, UInt8,  DateTime),
    class_state      AggregateFunction(argMax, UInt8,  DateTime),
    gender_state     AggregateFunction(argMax, UInt8,  DateTime),
    level_state      AggregateFunction(argMax, UInt8,  DateTime),
    first_seen_state AggregateFunction(min, DateTime),
    last_seen_state  AggregateFunction(max, DateTime),
    kill_count_state AggregateFunction(sum, UInt64)
)
ENGINE = AggregatingMergeTree
ORDER BY (realm, guid);

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_character
TO character AS
SELECT
    realm,
    players.guid AS guid,
    argMaxState(players.name,   kill_time) AS name_state,
    argMaxState(players.race,   kill_time) AS race_state,
    argMaxState(players.class,  kill_time) AS class_state,
    argMaxState(players.gender, kill_time) AS gender_state,
    argMaxState(players.level,  kill_time) AS level_state,
    minState(kill_time)                    AS first_seen_state,
    maxState(kill_time)                    AS last_seen_state,
    sumState(toUInt64(1))                  AS kill_count_state
FROM boss_kill
ARRAY JOIN players
GROUP BY realm, players.guid;

-- --------------------------------------------------------------------------
-- character_boss_rankings - best DPS / HPS per (realm, boss, mode, spec, guid)
-- --------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS character_boss_rankings (
    realm          LowCardinality(String),
    boss_remote_id UInt32,
    mode           UInt8,
    talent_spec    UInt16,
    guid           UInt64,
    dps_state      AggregateFunction(max, UInt64),
    hps_state      AggregateFunction(max, UInt64)
)
ENGINE = AggregatingMergeTree
ORDER BY (realm, boss_remote_id, mode, talent_spec, guid);

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_character_boss_rankings
TO character_boss_rankings AS
SELECT
    realm,
    boss_remote_id,
    mode,
    players.talent_spec AS talent_spec,
    players.guid        AS guid,
    maxState(toUInt64(players.dmg_done                             * 1000 / greatest(length, 1))) AS dps_state,
    maxState(toUInt64((players.healing_done + players.absorb_done) * 1000 / greatest(length, 1))) AS hps_state
FROM boss_kill
ARRAY JOIN players
GROUP BY realm, boss_remote_id, mode, players.talent_spec, players.guid;

-- --------------------------------------------------------------------------
-- raid_lock_rankings - character boss rankings bucketed by raid lock.
--
-- A raid lock starts on Wednesday 06:00 UTC and runs 7 days. The bucket key is
-- the Date of the lock's Wednesday.
-- --------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS raid_lock_rankings (
    realm          LowCardinality(String),
    raid_lock      Date,
    boss_remote_id UInt32,
    mode           UInt8,
    talent_spec    UInt16,
    guid           UInt64,
    dps_state      AggregateFunction(max, UInt64),
    hps_state      AggregateFunction(max, UInt64)
)
ENGINE = AggregatingMergeTree
ORDER BY (realm, raid_lock, boss_remote_id, mode, talent_spec, guid);

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_raid_lock_rankings
TO raid_lock_rankings AS
SELECT
    realm,
    toDate(kill_time - INTERVAL 6 HOUR)
        - toIntervalDay(modulo(toDayOfWeek(kill_time - INTERVAL 6 HOUR) - 3 + 7, 7)) AS raid_lock,
    boss_remote_id,
    mode,
    players.talent_spec AS talent_spec,
    players.guid        AS guid,
    maxState(toUInt64(players.dmg_done                             * 1000 / greatest(length, 1))) AS dps_state,
    maxState(toUInt64((players.healing_done + players.absorb_done) * 1000 / greatest(length, 1))) AS hps_state
FROM boss_kill
ARRAY JOIN players
GROUP BY realm, raid_lock, boss_remote_id, mode, players.talent_spec, players.guid;

-- --------------------------------------------------------------------------
-- Cached Twinstar character activity feed
-- --------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS character_activity_feed (
    realm LowCardinality(String),
    character_name LowCardinality(String),
    event_type UInt8,
    event_time DateTime,
    api_id UInt16,
    data UInt32,
    data2 UInt64,
    difficulty UInt8,
    item_guid UInt64,
    item_quality UInt8,
    icon LowCardinality(String),
    title LowCardinality(String),
    boss_kill_remote_id String,
    achievement_points UInt16,
    achievement_realm_first Bool,
    refresh_error String,
    source_page UInt8,
    inserted_at DateTime64(3) DEFAULT now64(),
    version DateTime64(3) DEFAULT now64()
)
ENGINE = ReplacingMergeTree(version)
ORDER BY (
    realm,
    character_name,
    event_time,
    event_type,
    data,
    data2,
    item_guid
);

-- --------------------------------------------------------------------------
-- Cached Twinstar character stats
-- --------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS character_stats (
    realm LowCardinality(String),
    character_name LowCardinality(String),
    raw_json String,

    health UInt32,
    mana UInt32,
    stamina UInt32,
    strength UInt32,
    agility UInt32,
    intellect UInt32,
    spirit UInt32,
    stamina_pos_buf UInt32,
    stamina_neg_buf UInt32,
    strength_pos_buf UInt32,
    strength_neg_buf UInt32,
    agility_pos_buf UInt32,
    agility_neg_buf UInt32,
    intellect_pos_buf UInt32,
    intellect_neg_buf UInt32,
    spirit_pos_buf UInt32,
    spirit_neg_buf UInt32,
    energy_regen Float64,
    attack_power UInt32,
    spell_power UInt32,
    hit_base UInt32,
    hit_percent Float32,
    hit_rating Float32,
    crit_base UInt32,
    crit_percent Float32,
    crit_rating Float32,
    haste_base UInt32,
    haste_percent Float32,
    haste_rating Float32,
    mastery_base UInt32,
    mastery_percent Float32,
    mastery_rating Float32,
    armor_base UInt32,
    armor_percent Float32,
    armor_rating Float32,
    dodge_base UInt32,
    dodge_percent Float32,
    dodge_rating Float32,
    parry_base UInt32,
    parry_percent Float32,
    parry_rating Float32,

    last_checked_at DateTime64(3),
    last_success_at DateTime64(3),
    last_error String,
    version DateTime64(3) DEFAULT now64()
)
ENGINE = ReplacingMergeTree(version)
ORDER BY (realm, character_name);
