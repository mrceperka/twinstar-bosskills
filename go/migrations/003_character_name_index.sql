-- character_name_index - point lookup for "find guid by (realm, name)".
--
-- CharacterByName used to GROUP BY the entire character table per realm and
-- filter the result in HAVING, aggregating every guid's argMax/min/max states
-- just to resolve one name (seen in prod: ~1.2s, 189MB, per request). Query
-- cache didn't help - each distinct name is a distinct cache key, so the
-- long tail of character lookups mostly missed anyway.
--
-- This table is refreshed from `character` (not boss_kill) at the end of
-- every sync run, same lifecycle as the query cache flush in cmd/sync: reads
-- stay correct because nothing else writes between syncs. Multiple guids can
-- share a name (existing ORDER BY kills DESC LIMIT 1 in the Go query handles
-- that), so the sort key keeps guid to disambiguate rather than collapsing
-- rows by (realm, name) alone.

CREATE TABLE IF NOT EXISTS character_name_index (
    realm      LowCardinality(String),
    name       String,
    guid       UInt64,
    class      UInt8,
    first_seen DateTime,
    last_seen  DateTime,
    kills      UInt64,
    version    DateTime64(3) DEFAULT now64()
)
ENGINE = ReplacingMergeTree(version)
ORDER BY (realm, name, guid);
