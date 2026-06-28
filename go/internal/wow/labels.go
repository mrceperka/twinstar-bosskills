// Package wow holds short, display-oriented labels for WoW domain concepts
// (classes, talent specs, difficulty modes). These were duplicated across
// most page packages; centralising them prevents drift.
package wow

import (
	"strconv"

	"github.com/mrceperka/twinstar-bosskills/go/internal/realm"
)

// Difficulty mirrors packages/core/src/wow.ts:difficultyToString. Returns a
// short label like "10HC". Falls back to the raw mode integer for unknown
// values.
func Difficulty(expansion, mode int) string {
	switch expansion {
	case realm.ExpansionMoP, realm.ExpansionCata:
		switch mode {
		case 3:
			return "10N"
		case 4:
			return "25N"
		case 5:
			return "10HC"
		case 6:
			return "25HC"
		case 7:
			return "LFR"
		case 14:
			return "Flex"
		}
	case realm.ExpansionVanilla:
		switch mode {
		case 0, 9:
			return "40"
		case 3:
			return "10"
		case 4:
			return "25"
		}
	}
	return strconv.Itoa(mode)
}

// Class returns the WoW player class name for the given numeric class ID.
// Returns empty string for unknown.
func Class(c int) string {
	switch c {
	case 1:
		return "Warrior"
	case 2:
		return "Paladin"
	case 3:
		return "Hunter"
	case 4:
		return "Rogue"
	case 5:
		return "Priest"
	case 6:
		return "Death Knight"
	case 7:
		return "Shaman"
	case 8:
		return "Mage"
	case 9:
		return "Warlock"
	case 10:
		return "Monk"
	case 11:
		return "Druid"
	}
	return ""
}

// SpecNames maps talent_spec ID to a short label. MoP-era values cover most
// of what shows on the boss/character pages.
var SpecNames = map[int]string{
	62:  "Arcane",
	63:  "Fire",
	64:  "Frost",
	65:  "Holy Pal",
	66:  "Prot Pal",
	70:  "Retri",
	71:  "Arms",
	72:  "Fury",
	73:  "Prot War",
	102: "Balance",
	103: "Feral",
	104: "Guardian",
	105: "Resto Druid",
	250: "Blood DK",
	251: "Frost DK",
	252: "Unholy",
	253: "Beast",
	254: "Marks",
	255: "Survival",
	256: "Disc",
	257: "Holy Pr",
	258: "Shadow",
	259: "Assa",
	260: "Combat",
	261: "Subtlety",
	262: "Elemental",
	263: "Enh",
	264: "Resto Sham",
	265: "Affli",
	266: "Demo",
	267: "Destro",
	268: "Brewmaster",
	269: "Windwalker",
	270: "Mistweaver",
}

// Spec returns the short label for a talent_spec ID, or "-" for spec 0,
// or the raw integer for an unknown spec.
func Spec(id int) string {
	if v, ok := SpecNames[id]; ok {
		return v
	}
	if id == 0 {
		return "-"
	}
	return strconv.Itoa(id)
}

// ClassFromSpec returns the class ID for a known talent_spec ID. Useful when
// the raid_lock_rankings MV doesn't carry class but does carry spec — the
// mapping is fixed by WoW design.
func ClassFromSpec(spec int) int {
	switch spec {
	case 62, 63, 64:
		return 8 // Mage
	case 65, 66, 70:
		return 2 // Paladin
	case 71, 72, 73:
		return 1 // Warrior
	case 102, 103, 104, 105:
		return 11 // Druid
	case 250, 251, 252:
		return 6 // DK
	case 253, 254, 255:
		return 3 // Hunter
	case 256, 257, 258:
		return 5 // Priest
	case 259, 260, 261:
		return 4 // Rogue
	case 262, 263, 264:
		return 7 // Shaman
	case 265, 266, 267:
		return 9 // Warlock
	case 268, 269, 270:
		return 10 // Monk
	}
	return 0
}
