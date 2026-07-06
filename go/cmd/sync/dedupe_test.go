package main

import (
	"testing"

	"github.com/mrceperka/twinstar-bosskills/go/internal/api"
	"github.com/mrceperka/twinstar-bosskills/go/internal/wow"
)

func TestFilterDuplicateDarkShamanLFRBossKills(t *testing.T) {
	kills := []api.BossKill{
		darkShamanLFRKill("18_200", "2026-07-07T18:00:00Z"),
		darkShamanLFRKill("18_199", "2026-07-07T18:00:00Z"),
		darkShamanLFRKill("18_201", "2026-07-07T18:10:00Z"),
		{ID: "18_300", Entry: 71859, Mode: wow.DifficultyMoP25Heroic, Guild: "LFR Heroes", Time: "2026-07-07T18:00:00Z", Length: 300},
		{ID: "18_400", Entry: 71865, Mode: wow.DifficultyMoPLFR, Guild: "LFR Heroes", Time: "2026-07-07T18:00:00Z", Length: 300},
	}

	got, skipped := filterDuplicateDarkShamanLFRBossKills(kills, nil)

	if skipped != 1 {
		t.Fatalf("skipped = %d, want 1", skipped)
	}
	wantIDs := []string{"18_199", "18_201", "18_300", "18_400"}
	if len(got) != len(wantIDs) {
		t.Fatalf("len(got) = %d, want %d: %#v", len(got), len(wantIDs), got)
	}
	for i, want := range wantIDs {
		if got[i].ID != want {
			t.Fatalf("got[%d].ID = %q, want %q", i, got[i].ID, want)
		}
	}
}

func TestFilterDuplicateDarkShamanLFRBossKillsSkipsExistingSignature(t *testing.T) {
	kill := darkShamanLFRKill("18_200", "2026-07-07T18:00:00Z")
	sig, ok := makeDarkShamanLFRSignature(kill)
	if !ok {
		t.Fatal("expected Dark Shaman LFR signature")
	}

	got, skipped := filterDuplicateDarkShamanLFRBossKills([]api.BossKill{kill}, map[darkShamanLFRSignature]bool{sig: true})

	if skipped != 1 {
		t.Fatalf("skipped = %d, want 1", skipped)
	}
	if len(got) != 0 {
		t.Fatalf("len(got) = %d, want 0", len(got))
	}
}

func TestBossKillIDLessComparesNumericSuffix(t *testing.T) {
	if !bossKillIDLess("18_99", "18_100") {
		t.Fatal("expected 18_99 to sort before 18_100")
	}
	if bossKillIDLess("18_100", "18_99") {
		t.Fatal("did not expect 18_100 to sort before 18_99")
	}
}

func darkShamanLFRKill(id, killTime string) api.BossKill {
	return api.BossKill{
		ID:       id,
		Entry:    korKronDarkShamanEntry,
		Mode:     wow.DifficultyMoPLFR,
		Guild:    "LFR Heroes",
		Time:     killTime,
		Length:   300,
		Wipes:    1,
		Deaths:   2,
		RessUsed: 3,
	}
}
