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
		{"mop 10 normal", realm.ExpansionMoP, 3, "10 N"},
		{"mop 10 heroic", realm.ExpansionMoP, 5, "10 HC"},
		{"cata 10 normal", realm.ExpansionCata, 0, "10 N"},
		{"cata 10 heroic", realm.ExpansionCata, 2, "10 HC"},
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
	if got, want := RaidDifficulties(realm.ExpansionCata), []int{0, 1, 2, 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("RaidDifficulties(Cata) = %v, want %v", got, want)
	}
	if got, want := RaidDifficulties(realm.ExpansionMoP), []int{3, 4, 5, 6, 7, 14}; !reflect.DeepEqual(got, want) {
		t.Fatalf("RaidDifficulties(MoP) = %v, want %v", got, want)
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
		{"mop elemental", realm.ExpansionMoP, 262, "Elemental", 7},
		{"cata elemental collision", realm.ExpansionCata, 261, "Elemental", 7},
		{"cata restoration shaman collision", realm.ExpansionCata, 262, "Restoration (Shaman)", 7},
		{"vanilla fire", realm.ExpansionVanilla, 41, "Fire", 8},
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
