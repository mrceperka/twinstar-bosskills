package boss

import "testing"

func TestBossFilterHrefsDoNotForceRaidLockForOverallView(t *testing.T) {
	vm := ViewModel{
		Realm:          "Helios",
		Boss:           BossInfo{RemoteID: 71543},
		SelectedMode:   6,
		SelectedPctile: 85,
		SelectedSpec:   102,
	}

	if got, want := string(tabHref(vm, 7)), "/Helios/boss/71543?difficulty=7&p=85&spec=102"; got != want {
		t.Fatalf("tabHref = %q, want %q", got, want)
	}
	if got, want := string(siblingHref(vm, 71544)), "/Helios/boss/71544?difficulty=6&p=85&spec=102"; got != want {
		t.Fatalf("siblingHref = %q, want %q", got, want)
	}
}

func TestBossFilterHrefsPreserveExplicitRaidLock(t *testing.T) {
	vm := ViewModel{
		Realm:          "Helios",
		Boss:           BossInfo{RemoteID: 71543},
		SelectedMode:   6,
		SelectedPctile: 85,
		SelectedSpec:   102,
		LockScoped:     true,
		LockOffset:     2,
	}

	if got, want := string(tabHref(vm, 7)), "/Helios/boss/71543?difficulty=7&raidlock=2&p=85&spec=102"; got != want {
		t.Fatalf("tabHref = %q, want %q", got, want)
	}
	if got, want := string(previousLockHref(vm)), "/Helios/boss/71543?difficulty=6&raidlock=3&p=85&spec=102"; got != want {
		t.Fatalf("previousLockHref = %q, want %q", got, want)
	}
}
