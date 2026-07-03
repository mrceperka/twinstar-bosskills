// Package wow holds short, display-oriented labels for WoW domain concepts
// (classes, talent specs, difficulty modes). These were duplicated across
// most page packages; centralising them prevents drift.
package wow

import (
	"strconv"

	"github.com/mrceperka/twinstar-bosskills/go/internal/realm"
)

// Difficulty mirrors packages/core/src/wow.ts:difficultyToString. Returns a
// short label like "10 HC". Falls back to the raw mode integer for unknown
// values.
func Difficulty(expansion, mode int) string {
	switch expansion {
	case realm.ExpansionMoP:
		switch mode {
		case 0:
			return "None"
		case 1:
			return "N"
		case 2:
			return "HC"
		case 3:
			return "10 N"
		case 4:
			return "25 N"
		case 5:
			return "10 HC"
		case 6:
			return "25 HC"
		case 7:
			return "LFR"
		case 8:
			return "Challenge"
		case 9:
			return "40"
		case 11:
			return "Scenario HC"
		case 12:
			return "Scenario N"
		case 14:
			return "Flex"
		}
	case realm.ExpansionCata:
		switch mode {
		case 0:
			return "10 N"
		case 1:
			return "25 N"
		case 2:
			return "10 HC"
		case 3:
			return "25 HC"
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

func DefaultDifficulty(expansion int) int {
	switch expansion {
	case realm.ExpansionMoP:
		return 3
	case realm.ExpansionCata:
		return 0
	default:
		return 0
	}
}

func RaidDifficulties(expansion int) []int {
	switch expansion {
	case realm.ExpansionMoP:
		return []int{3, 4, 5, 6, 7, 14}
	case realm.ExpansionCata:
		return []int{0, 1, 2, 3}
	case realm.ExpansionVanilla:
		return []int{0, 3, 4, 9}
	default:
		return nil
	}
}

func PerformanceDifficulties(expansion int) []int {
	switch expansion {
	case realm.ExpansionMoP:
		return []int{3, 5, 4, 6}
	case realm.ExpansionCata:
		return []int{0, 2, 1, 3}
	default:
		return nil
	}
}

// Class constants are the numeric WoW player class IDs used by upstream data.
const (
	ClassWarrior     = 1
	ClassPaladin     = 2
	ClassHunter      = 3
	ClassRogue       = 4
	ClassPriest      = 5
	ClassDeathKnight = 6
	ClassShaman      = 7
	ClassMage        = 8
	ClassWarlock     = 9
	ClassMonk        = 10
	ClassDruid       = 11
)

// Class returns the WoW player class name for the given numeric class ID.
// Returns empty string for unknown.
func Class(c int) string {
	switch c {
	case ClassWarrior:
		return "Warrior"
	case ClassPaladin:
		return "Paladin"
	case ClassHunter:
		return "Hunter"
	case ClassRogue:
		return "Rogue"
	case ClassPriest:
		return "Priest"
	case ClassDeathKnight:
		return "Death Knight"
	case ClassShaman:
		return "Shaman"
	case ClassMage:
		return "Mage"
	case ClassWarlock:
		return "Warlock"
	case ClassMonk:
		return "Monk"
	case ClassDruid:
		return "Druid"
	}
	return ""
}

var specNamesMoP = map[int]string{
	62:  "Arcane",
	63:  "Fire",
	64:  "Frost (Mage)",
	65:  "Holy (Paladin)",
	66:  "Protection (Paladin)",
	70:  "Retri",
	71:  "Arms",
	72:  "Fury",
	73:  "Protection (Warrior)",
	102: "Balance",
	103: "Feral",
	104: "Guardian",
	105: "Restoration (Druid)",
	250: "Blood DK",
	251: "Frost DK",
	252: "Unholy",
	253: "Beast",
	254: "Marks",
	255: "Survival",
	256: "Disc",
	257: "Holy (Priest)",
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

var specNamesCata = map[int]string{
	799: "Arcane",
	851: "Fire",
	823: "Frost (Mage)",
	831: "Holy (Paladin)",
	839: "Protection (Paladin)",
	855: "Retribution",
	746: "Arms",
	815: "Fury",
	845: "Protection (Warrior)",
	752: "Balance",
	750: "Feral",
	748: "Restoration (Druid)",
	398: "Blood DK",
	399: "Frost DK",
	400: "Unholy",
	811: "Beast Mastery",
	807: "Marksmanship",
	809: "Survival",
	760: "Discipline",
	813: "Holy (Priest)",
	795: "Shadow",
	181: "Assassination",
	182: "Combat",
	183: "Subtlety",
	261: "Elemental",
	263: "Enhancement",
	262: "Restoration (Shaman)",
	871: "Affliction",
	867: "Demonology",
	865: "Destruction",
}

var specNamesVanilla = map[int]string{
	81:  "Arcane",
	41:  "Fire",
	61:  "Frost (Mage)",
	161: "Arms",
	164: "Fury",
	163: "Protection (Warrior)",
	382: "Holy (Paladin)",
	383: "Protection (Paladin)",
	381: "Retribution",
	283: "Balance",
	281: "Feral",
	282: "Restoration (Druid)",
	361: "Beast Mastery",
	363: "Marksmanship",
	362: "Survival",
	201: "Discipline",
	202: "Holy (Priest)",
	203: "Shadow",
	182: "Assassination",
	181: "Combat",
	183: "Subtlety",
	261: "Elemental",
	263: "Enhancement",
	262: "Restoration (Shaman)",
	302: "Affliction",
	303: "Demonology",
	301: "Destruction",
}

// Spec returns the MoP-era short label for a talent_spec ID. Prefer
// SpecForExpansion when the page knows the active expansion.
func Spec(id int) string {
	return SpecForExpansion(realm.ExpansionMoP, id)
}

func SpecForRealm(realmName string, id int) string {
	return SpecForExpansion(realm.Expansion(realmName), id)
}

func SpecForExpansion(expansion, id int) string {
	if id == 0 {
		return "-"
	}
	var names map[int]string
	switch expansion {
	case realm.ExpansionCata:
		names = specNamesCata
	case realm.ExpansionVanilla:
		names = specNamesVanilla
	default:
		names = specNamesMoP
	}
	if v, ok := names[id]; ok {
		return v
	}
	return strconv.Itoa(id)
}

// ClassFromSpec returns the MoP-era class ID for a known talent_spec ID.
// Prefer ClassFromSpecForExpansion when the page knows the active expansion.
func ClassFromSpec(spec int) int {
	return ClassFromSpecForExpansion(realm.ExpansionMoP, spec)
}

func ClassFromSpecForRealm(realmName string, spec int) int {
	return ClassFromSpecForExpansion(realm.Expansion(realmName), spec)
}

func ClassFromSpecForExpansion(expansion, spec int) int {
	switch expansion {
	case realm.ExpansionCata:
		return classFromSpecMap(spec, specClassCata)
	case realm.ExpansionVanilla:
		return classFromSpecMap(spec, specClassVanilla)
	default:
		return classFromSpecMap(spec, specClassMoP)
	}
}

func classFromSpecMap(spec int, m map[int]int) int {
	if cls, ok := m[spec]; ok {
		return cls
	}
	return 0
}

var specClassMoP = map[int]int{
	62: 8, 63: 8, 64: 8,
	65: 2, 66: 2, 70: 2,
	71: 1, 72: 1, 73: 1,
	102: 11, 103: 11, 104: 11, 105: 11,
	250: 6, 251: 6, 252: 6,
	253: 3, 254: 3, 255: 3,
	256: 5, 257: 5, 258: 5,
	259: 4, 260: 4, 261: 4,
	262: 7, 263: 7, 264: 7,
	265: 9, 266: 9, 267: 9,
	268: 10, 269: 10, 270: 10,
}

var specClassCata = map[int]int{
	799: 8, 851: 8, 823: 8,
	831: 2, 839: 2, 855: 2,
	746: 1, 815: 1, 845: 1,
	752: 11, 750: 11, 748: 11,
	398: 6, 399: 6, 400: 6,
	811: 3, 807: 3, 809: 3,
	760: 5, 813: 5, 795: 5,
	181: 4, 182: 4, 183: 4,
	261: 7, 263: 7, 262: 7,
	871: 9, 867: 9, 865: 9,
}

var specClassVanilla = map[int]int{
	81: 8, 41: 8, 61: 8,
	161: 1, 164: 1, 163: 1,
	382: 2, 383: 2, 381: 2,
	283: 11, 281: 11, 282: 11,
	361: 3, 363: 3, 362: 3,
	201: 5, 202: 5, 203: 5,
	182: 4, 181: 4, 183: 4,
	261: 7, 263: 7, 262: 7,
	302: 9, 303: 9, 301: 9,
}
