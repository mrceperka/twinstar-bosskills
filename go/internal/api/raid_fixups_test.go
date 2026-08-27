package api

import "testing"

func TestFilterVanillaRaids(t *testing.T) {
	upstream := []Raid{
		{Map: "Molten Core"},
		{Map: "Blackrock Spire"},
		{Map: "Deeprun Tram"},
		{Map: "Kalimdor"},
		{Map: "Naxxramas"},
	}
	names := func(rs []Raid) []string {
		out := make([]string, 0, len(rs))
		for _, r := range rs {
			out = append(out, r.Map)
		}
		return out
	}
	eq := func(got, want []string) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}

	// Both vanilla realms: world zones dropped, Blackrock Spire kept.
	want := []string{"Molten Core", "Blackrock Spire", "Naxxramas"}
	for _, r := range []string{"Kronos", "KronosV"} {
		in := append([]Raid(nil), upstream...)
		if got := names(filterVanillaRaids(r, in)); !eq(got, want) {
			t.Errorf("%s: got %v want %v", r, got, want)
		}
	}

	// MoP realms untouched.
	in := append([]Raid(nil), upstream...)
	if got := len(filterVanillaRaids("Perses", in)); got != len(upstream) {
		t.Errorf("Perses: got %d raids, want %d", got, len(upstream))
	}
}
