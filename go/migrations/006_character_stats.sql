-- 006_character_stats.sql
--
-- Cached Twinstar character stats. The character page lazy-loads this snapshot
-- through an HTMX fragment, and refreshes at most once per hour per character.

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
