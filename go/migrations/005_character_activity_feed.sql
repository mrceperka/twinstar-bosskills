-- 005_character_activity_feed.sql
--
-- Cached Twinstar character activity feed. The main character page renders a
-- shell and lazy-loads this data through an HTMX fragment so page load is not
-- blocked by upstream activity refreshes.

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
