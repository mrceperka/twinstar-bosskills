package wow

import "twinstar-bosskills/internal/realm"

const (
	DifficultyMoPNone       = 0
	DifficultyMoPNormal     = 1
	DifficultyMoPHeroic     = 2
	DifficultyMoP10Normal   = 3
	DifficultyMoP25Normal   = 4
	DifficultyMoP10Heroic   = 5
	DifficultyMoP25Heroic   = 6
	DifficultyMoPLFR        = 7
	DifficultyMoPChallenge  = 8
	DifficultyMoP40         = 9
	DifficultyMoPHCScenario = 11
	DifficultyMoPNScenario  = 12
	DifficultyMoPFlex       = 14
	DifficultyMoPMax        = 15
)

const (
	DifficultyCata10Normal = 0
	DifficultyCata25Normal = 1
	DifficultyCata10Heroic = 2
	DifficultyCata25Heroic = 3
)

func IsRaidDifficulty(expansion, diff int) bool {
	switch expansion {
	case realm.ExpansionMoP:
		return isRaidDifficultyMoP(diff)
	case realm.ExpansionCata:
		return isRaidDifficultyCata(diff)
	default:
		return false
	}
}

func IsRaidDifficultyWithLoot(expansion, diff int) bool {
	switch expansion {
	case realm.ExpansionMoP:
		return isRaidDifficultyMoP(diff) && diff != DifficultyMoPLFR
	case realm.ExpansionCata:
		return isRaidDifficultyCata(diff)
	case realm.ExpansionVanilla:
		return true
	default:
		return false
	}
}

func isRaidDifficultyMoP(diff int) bool {
	switch diff {
	case DifficultyMoP10Normal, DifficultyMoP25Normal, DifficultyMoP10Heroic, DifficultyMoP25Heroic, DifficultyMoPLFR, DifficultyMoPFlex:
		return true
	default:
		return false
	}
}

func isRaidDifficultyCata(diff int) bool {
	switch diff {
	case DifficultyCata10Normal, DifficultyCata25Normal, DifficultyCata10Heroic, DifficultyCata25Heroic:
		return true
	default:
		return false
	}
}
