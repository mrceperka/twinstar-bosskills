package links

import "testing"

func TestTwinheadBossKill_StripsRealmPrefix(t *testing.T) {
	// Helios is realm ID 18; remote_id "18_2230832" should become bkid 2230832.
	got := TwinheadBossKill("Helios", "18_2230832")
	want := "https://mop-twinhead.twinstar.cz/?boss-kill=2230832"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTwinheadBossKill_VanillaPrefix(t *testing.T) {
	// Kronos is vanilla - realm ID 4.
	got := TwinheadBossKill("Kronos", "4_1234")
	want := "https://vanilla-twinhead.twinstar.cz/?boss-kill=1234"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTwinheadBossKill_NoPrefixIfMissing(t *testing.T) {
	got := TwinheadBossKill("Helios", "weird-id-no-prefix")
	want := "https://mop-twinhead.twinstar.cz/?boss-kill=weird-id-no-prefix"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBoss(t *testing.T) {
	if Boss("Helios", 71865) != "/Helios/boss/71865" {
		t.Errorf("Boss URL wrong")
	}
}

func TestCharacter_PathEscapes(t *testing.T) {
	got := Character("Helios", "Sk'ra Bear")
	want := "/Helios/character/Sk%27ra%20Bear"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
