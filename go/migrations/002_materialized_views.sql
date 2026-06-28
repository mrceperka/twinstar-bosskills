-- 002_materialized_views.sql
--
-- Replaces packages/model CLI ranking jobs with ClickHouse materialized views.
-- Each MV writes into an AggregatingMergeTree target. Reads merge the *State
-- columns via the matching *Merge function (e.g. maxMerge, quantilesTDigestMerge).
--
-- Idempotency: the sync job MUST guarantee one INSERT per upstream boss_kill.
-- If the same kill is re-inserted, the percentile MV would double-count.
-- ReplacingMergeTree on boss_kill collapses duplicates on background merge,
-- but MVs see raw INSERTs.  Read packages/go/cmd/sync/README for details.

-- --------------------------------------------------------------------------
-- character — one row per (realm, guid) with most-recent identity + counts
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
-- character_boss_rankings — best DPS / HPS per (realm, boss, mode, spec, guid)
--
-- `length` from the upstream API is in MILLISECONDS (see
-- packages/core/src/metrics.ts:valuePerSecond, which divides by 1000).
--   DPS = dmg_done * 1000 / GREATEST(length, 1)
--   HPS = (healing_done + absorb_done) * 1000 / GREATEST(length, 1)
--
-- max is idempotent under duplicate inserts, so re-sync noise is tolerable.
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
-- character_boss_percentiles — quantile distributions per (realm, boss, mode, spec)
-- --------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS character_boss_percentiles (
    realm                LowCardinality(String),
    boss_remote_id       UInt32,
    mode                 UInt8,
    talent_spec          UInt16,
    dps_quantiles_state  AggregateFunction(quantilesTDigest(0.25, 0.5, 0.75, 0.9, 0.95, 0.99), Float64),
    hps_quantiles_state  AggregateFunction(quantilesTDigest(0.25, 0.5, 0.75, 0.9, 0.95, 0.99), Float64)
)
ENGINE = AggregatingMergeTree
ORDER BY (realm, boss_remote_id, mode, talent_spec);

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_character_boss_percentiles
TO character_boss_percentiles AS
SELECT
    realm,
    boss_remote_id,
    mode,
    players.talent_spec AS talent_spec,
    quantilesTDigestState(0.25, 0.5, 0.75, 0.9, 0.95, 0.99)(
        toFloat64(players.dmg_done) * 1000 / greatest(length, 1)
    ) AS dps_quantiles_state,
    quantilesTDigestState(0.25, 0.5, 0.75, 0.9, 0.95, 0.99)(
        toFloat64(players.healing_done + players.absorb_done) * 1000 / greatest(length, 1)
    ) AS hps_quantiles_state
FROM boss_kill
ARRAY JOIN players
GROUP BY realm, boss_remote_id, mode, players.talent_spec;

-- --------------------------------------------------------------------------
-- raid_lock_rankings — same as character_boss_rankings but bucketed by week.
--
-- A raid lock starts on Wednesday 06:00 UTC and runs 7 days. The bucket key
-- is the Date of the lock's Wednesday. CH expression breakdown:
--   adjusted = kill_time - INTERVAL 6 HOUR
--   weekday  = toDayOfWeek(adjusted)        -- Mon=1..Sun=7, Wed=3
--   offset   = (weekday - 3 + 7) % 7        -- days back to previous Wednesday
--   bucket   = toDate(adjusted) - offset    -- the Wednesday's date
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
