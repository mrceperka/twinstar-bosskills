// Package api ports packages/api — the upstream twinstar-api client and its
// JSON schemas. Validation is done with struct tags + targeted strconv where
// the upstream uses string-coerced numbers.
package api

import (
	"encoding/json"
	"strconv"
)

// FlexInt is a JSON number that may arrive quoted as a string ("123") or as
// a real number (123). The TS schemas use z.coerce.number() for the
// per-player damage/healing fields — same idea here.
type FlexInt int64

func (f *FlexInt) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		if s == "" {
			return nil
		}
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			// Some fields are coerced from floats embedded in strings; try float.
			fv, ferr := strconv.ParseFloat(s, 64)
			if ferr != nil {
				return err
			}
			*f = FlexInt(int64(fv))
			return nil
		}
		*f = FlexInt(v)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	v, err := n.Int64()
	if err != nil {
		fv, ferr := n.Float64()
		if ferr != nil {
			return err
		}
		*f = FlexInt(int64(fv))
		return nil
	}
	*f = FlexInt(v)
	return nil
}

// Boss matches the `entry+name` shape returned by /bosskills/raids.
type Boss struct {
	Entry int    `json:"entry"`
	Name  string `json:"name"`
}

// Raid matches the `map+bosses` shape from /bosskills/raids.
type Raid struct {
	Map    string `json:"map"`
	Bosses []Boss `json:"bosses"`
}

// BossKill is the row returned by /bosskills (without per-player detail).
// The API's `id` field is a string (e.g. "18_1234567"); we keep it as-is.
type BossKill struct {
	ID           string `json:"id"`
	Entry        int    `json:"entry"`
	Map          string `json:"map"`
	Mode         int    `json:"mode"`
	Guild        string `json:"guild"`
	Time         string `json:"time"` // RFC3339-ish; parsed downstream
	Realm        string `json:"realm"`
	Length       int    `json:"length"`
	Wipes        int    `json:"wipes"`
	Deaths       int    `json:"deaths"`
	RessUsed     int    `json:"ressUsed"`
	CreatureName string `json:"creature_name"`
}

// BossKillPlayer matches boss_kills_players[*] in /bosskills/:id.
type BossKillPlayer struct {
	GUID            int64   `json:"guid"`
	TalentSpec      int     `json:"talent_spec"`
	AvgItemLvl      float64 `json:"avg_item_lvl"`
	DmgDone         FlexInt `json:"dmgDone"`
	HealingDone     FlexInt `json:"healingDone"`
	OverhealingDone FlexInt `json:"overhealingDone"`
	AbsorbDone      FlexInt `json:"absorbDone"`
	DmgTaken        FlexInt `json:"dmgTaken"`
	DmgAbsorbed     FlexInt `json:"dmgAbsorbed"`
	HealingTaken    FlexInt `json:"healingTaken"`
	Dispels         FlexInt `json:"dispels"`
	Interrupts      FlexInt `json:"interrupts"`
	Name            string  `json:"name"`
	Race            int     `json:"race"`
	Class           int     `json:"class"`
	Gender          int     `json:"gender"`
	Level           int     `json:"level"`
}

type BossKillLoot struct {
	ID     int    `json:"id"`
	ItemID string `json:"itemId"` // upstream sends a string
	Count  int    `json:"count"`
}

type BossKillDeath struct {
	ID   int   `json:"id"`
	GUID int64 `json:"guid"`
	Time int   `json:"time"` // negative = ress
}

type BossKillTimeline struct {
	Time            int     `json:"time"`
	EncounterDamage FlexInt `json:"encounterDamage"`
	EncounterHeal   FlexInt `json:"encounterHeal"`
	RaidDamage      FlexInt `json:"raidDamage"`
	RaidHeal        FlexInt `json:"raidHeal"`
}

type BossKillDetail struct {
	BossKill
	Players  []BossKillPlayer   `json:"boss_kills_players"`
	Loot     []BossKillLoot     `json:"boss_kills_loot"`
	Deaths   []BossKillDeath    `json:"boss_kills_deaths"`
	Timeline []BossKillTimeline `json:"boss_kills_maps"`
}

// PaginatedBossKills is the envelope used by /bosskills.
type PaginatedBossKills struct {
	Data  []BossKill `json:"data"`
	Total int        `json:"total"`
}

type CharacterActivityAchievement struct {
	Name       string `json:"name"`
	Points     int    `json:"points"`
	RealmFirst bool   `json:"realmFirst"`
}

type CharacterActivityLoot struct {
	Name string `json:"name"`
}

type CharacterActivityBoss struct {
	Name string `json:"name"`
}

type CharacterActivityEvent struct {
	ID          int                           `json:"id"`
	Type        int                           `json:"type"`
	Data        uint32                        `json:"data"`
	Data2       uint64                        `json:"data2"`
	Date        int64                         `json:"date"`
	Difficulty  int                           `json:"difficulty"`
	ItemGUID    uint64                        `json:"item_guid"`
	ItemQuality int                           `json:"item_quality"`
	Icon        string                        `json:"icon"`
	Achievement *CharacterActivityAchievement `json:"achievement,omitempty"`
	Loot        *CharacterActivityLoot        `json:"loot,omitempty"`
	Boss        *CharacterActivityBoss        `json:"boss,omitempty"`
}

type PaginatedCharacterActivityFeed struct {
	Data  []CharacterActivityEvent `json:"data"`
	Total int                      `json:"total"`
}

type CharacterStatsPayload struct {
	Raw   json.RawMessage
	Stats CharacterStats
}

type CharacterStats struct {
	Attributes CharacterStatsAttributes `json:"attributes"`
	Melee      CharacterStatsMelee      `json:"melee"`
	Ranged     CharacterStatsRanged     `json:"ranged"`
	Spell      CharacterStatsSpell      `json:"spell"`
	Defense    CharacterStatsDefense    `json:"defense"`
}

type CharacterStatsAttributes struct {
	Stamina   CharacterAttributeStat `json:"stamina"`
	Strength  CharacterAttributeStat `json:"strength"`
	Agility   CharacterAttributeStat `json:"agility"`
	Intellect CharacterAttributeStat `json:"intellect"`
	Spirit    CharacterAttributeStat `json:"spirit"`
}

type CharacterAttributeStat struct {
	Effective      uint32  `json:"effective"`
	Base           uint32  `json:"base"`
	Health         uint32  `json:"health"`
	Mana           int32   `json:"mana"`
	Attack         int32   `json:"attack"`
	Armor          uint32  `json:"armor"`
	SpellPower     int32   `json:"spellPower"`
	HealthRegen    float64 `json:"healthRegen"`
	ManaRegen      float64 `json:"manaRegen"`
	CritHitPercent float64 `json:"critHitPercent"`
	PosBuf         uint32  `json:"posBuf"`
	NegBuf         uint32  `json:"negBuf"`
}

type CharacterStatsMelee struct {
	AttackPower CharacterPowerStat  `json:"attackPower"`
	Haste       CharacterRatingStat `json:"haste"`
	HitChance   CharacterRatingStat `json:"hitChance"`
	CritChance  CharacterRatingStat `json:"critChance"`
	Mastery     CharacterRatingStat `json:"mastery"`
	PVPPower    CharacterRatingStat `json:"pvpPower"`
	EnergyRegen float64             `json:"energyRegen"`
}

type CharacterStatsRanged struct {
	AttackPower CharacterPowerStat  `json:"attackPower"`
	Haste       CharacterRatingStat `json:"haste"`
	HitChance   CharacterRatingStat `json:"hitChance"`
	CritChance  CharacterRatingStat `json:"critChance"`
	Mastery     CharacterRatingStat `json:"mastery"`
	PVPPower    CharacterRatingStat `json:"pvpPower"`
}

type CharacterStatsSpell struct {
	SpellPower  uint32              `json:"spellPower"`
	Haste       CharacterRatingStat `json:"haste"`
	HitChance   CharacterRatingStat `json:"hitChance"`
	ManaRegen   float64             `json:"manaRegen"`
	CombatRegen float64             `json:"combatRegen"`
	CritChance  CharacterRatingStat `json:"critChance"`
	Mastery     CharacterRatingStat `json:"mastery"`
	PVPPower    CharacterRatingStat `json:"pvpPower"`
}

type CharacterStatsDefense struct {
	Armor         CharacterRatingStat `json:"armor"`
	Dodge         CharacterRatingStat `json:"dodge"`
	Parry         CharacterRatingStat `json:"parry"`
	Block         CharacterRatingStat `json:"block"`
	PVPResilience CharacterRatingStat `json:"pvpResilience"`
	PVPPower      CharacterRatingStat `json:"pvpPower"`
}

type CharacterPowerStat struct {
	Base      uint32  `json:"base"`
	DPS       float64 `json:"dps"`
	PosBuf    uint32  `json:"posBuf"`
	NegBuf    uint32  `json:"negBuf"`
	Effective float64 `json:"effective"`
}

type CharacterRatingStat struct {
	Base    uint32  `json:"base"`
	Percent float64 `json:"percent"`
	Rating  float64 `json:"rating"`
}
