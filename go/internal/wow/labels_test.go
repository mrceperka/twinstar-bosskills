package wow

import (
	"reflect"
	"testing"

	"github.com/mrceperka/twinstar-bosskills/go/internal/realm"
)

func TestDifficultyByExpansion(t *testing.T) {
	tests := []struct {
		name      string
		expansion int
		mode      int
		want      string
	}{
		{"mop 10 normal", realm.ExpansionMoP, DifficultyMoP10Normal, "10 N"},
		{"mop 10 heroic", realm.ExpansionMoP, DifficultyMoP10Heroic, "10 HC"},
		{"cata 10 normal", realm.ExpansionCata, DifficultyCata10Normal, "10 N"},
		{"cata 10 heroic", realm.ExpansionCata, DifficultyCata10Heroic, "10 HC"},
		{"vanilla forty", realm.ExpansionVanilla, 9, "40"},
		{"unknown", realm.ExpansionMoP, 99, "99"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Difficulty(tt.expansion, tt.mode); got != tt.want {
				t.Fatalf("Difficulty(%d, %d) = %q, want %q", tt.expansion, tt.mode, got, tt.want)
			}
		})
	}
}

func TestDifficultyLists(t *testing.T) {
	if got, want := RaidDifficulties(realm.ExpansionCata), []int{DifficultyCata10Normal, DifficultyCata25Normal, DifficultyCata10Heroic, DifficultyCata25Heroic}; !reflect.DeepEqual(got, want) {
		t.Fatalf("RaidDifficulties(Cata) = %v, want %v", got, want)
	}
	if got, want := RaidDifficulties(realm.ExpansionMoP), []int{DifficultyMoP10Normal, DifficultyMoP25Normal, DifficultyMoP10Heroic, DifficultyMoP25Heroic, DifficultyMoPLFR, DifficultyMoPFlex}; !reflect.DeepEqual(got, want) {
		t.Fatalf("RaidDifficulties(MoP) = %v, want %v", got, want)
	}
}

func TestIsRaidDifficultyWithLoot(t *testing.T) {
	tests := []struct {
		name      string
		expansion int
		mode      int
		want      bool
	}{
		{"mop lfr", realm.ExpansionMoP, DifficultyMoPLFR, false},
		{"mop flex", realm.ExpansionMoP, DifficultyMoPFlex, true},
		{"mop challenge", realm.ExpansionMoP, DifficultyMoPChallenge, false},
		{"cata heroic", realm.ExpansionCata, DifficultyCata10Heroic, true},
		{"vanilla", realm.ExpansionVanilla, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRaidDifficultyWithLoot(tt.expansion, tt.mode); got != tt.want {
				t.Fatalf("IsRaidDifficultyWithLoot(%d, %d) = %v, want %v", tt.expansion, tt.mode, got, tt.want)
			}
		})
	}
}

func TestSpecAndClassByExpansion(t *testing.T) {
	tests := []struct {
		name      string
		expansion int
		spec      int
		wantSpec  string
		wantClass int
	}{
		{"mop elemental", realm.ExpansionMoP, 262, "Elemental", ClassShaman},
		{"mop retribution", realm.ExpansionMoP, 70, "Retribution", ClassPaladin},
		{"mop beast mastery", realm.ExpansionMoP, 253, "Beast Mastery", ClassHunter},
		{"mop marksmanship", realm.ExpansionMoP, 254, "Marksmanship", ClassHunter},
		{"mop discipline", realm.ExpansionMoP, 256, "Discipline", ClassPriest},
		{"mop assassination", realm.ExpansionMoP, 259, "Assassination", ClassRogue},
		{"mop affliction", realm.ExpansionMoP, 265, "Affliction", ClassWarlock},
		{"mop demonology", realm.ExpansionMoP, 266, "Demonology", ClassWarlock},
		{"mop destruction", realm.ExpansionMoP, 267, "Destruction", ClassWarlock},
		{"cata elemental collision", realm.ExpansionCata, 261, "Elemental", ClassShaman},
		{"cata restoration shaman collision", realm.ExpansionCata, 262, "Restoration (Shaman)", ClassShaman},
		{"vanilla fire", realm.ExpansionVanilla, 41, "Fire", ClassMage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SpecForExpansion(tt.expansion, tt.spec); got != tt.wantSpec {
				t.Fatalf("SpecForExpansion(%d, %d) = %q, want %q", tt.expansion, tt.spec, got, tt.wantSpec)
			}
			if got := ClassFromSpecForExpansion(tt.expansion, tt.spec); got != tt.wantClass {
				t.Fatalf("ClassFromSpecForExpansion(%d, %d) = %d, want %d", tt.expansion, tt.spec, got, tt.wantClass)
			}
		})
	}
}
