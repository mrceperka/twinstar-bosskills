-- Materialized views process every inserted block before ReplacingMergeTree
-- merges duplicate source rows. Rebuild the derived character table so kill
-- counts use the exact set of upstream kill IDs instead of summing rows.
--
-- This table is fully derived from boss_kill, so dropping and backfilling it is
-- safer and retryable compared with trying to convert incompatible aggregate
-- state bytes in place.

DROP VIEW IF EXISTS mv_character;
DROP TABLE IF EXISTS character;

CREATE TABLE character (
    realm            LowCardinality(String),
    guid             UInt64,
    name_state       AggregateFunction(argMax, String, DateTime),
    race_state       AggregateFunction(argMax, UInt8,  DateTime),
    class_state      AggregateFunction(argMax, UInt8,  DateTime),
    gender_state     AggregateFunction(argMax, UInt8,  DateTime),
    level_state      AggregateFunction(argMax, UInt8,  DateTime),
    first_seen_state AggregateFunction(min, DateTime),
    last_seen_state  AggregateFunction(max, DateTime),
    kill_count_state AggregateFunction(uniqExact, String)
)
ENGINE = AggregatingMergeTree
ORDER BY (realm, guid);

INSERT INTO character
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
    uniqExactState(remote_id)              AS kill_count_state
FROM boss_kill FINAL
ARRAY JOIN players
GROUP BY realm, players.guid;

CREATE MATERIALIZED VIEW mv_character
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
    uniqExactState(remote_id)              AS kill_count_state
FROM boss_kill
ARRAY JOIN players
GROUP BY realm, players.guid;
