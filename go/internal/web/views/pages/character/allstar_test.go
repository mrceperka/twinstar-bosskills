package character

import (
	"testing"
	"time"
)

func TestAllStarPointsRankOne(t *testing.T) {
	// Matching rank-1 value: ratio=1, pct=100 => max(100,100)+20 = 120.
	got := allStarPoints(1000, 1000, 1, 10)
	if got != 120 {
		t.Fatalf("allStarPoints rank1 = %v, want 120", got)
	}
}

func TestAllStarPointsHalfValueUsesPercentileFloor(t *testing.T) {
	// my=200k, rank1=400k => ratio=0.5, 100*ratio=50.
	// rank=2 of 10 => pct = 100*(1-1/10) = 90 => max(50,90)=90, +20*0.5=10 => 100.
	got := allStarPoints(200000, 400000, 2, 10)
	if got != 100 {
		t.Fatalf("allStarPoints half = %v, want 100", got)
	}
}

func TestAllStarPointsRatioDominatesWhenFewPeers(t *testing.T) {
	// my=300, rank1=1000 => ratio=0.3, 100*ratio=30.
	// rank=2 of 2 => pct=100*(1-1/2)=50 => max(30,50)=50 +20*0.3=6 => 56.
	got := allStarPoints(300, 1000, 2, 2)
	if got != 56 {
		t.Fatalf("allStarPoints = %v, want 56", got)
	}
}

func TestAllStarPointsZeroValueScoresZero(t *testing.T) {
	if got := allStarPoints(0, 1000, 5, 10); got != 0 {
		t.Fatalf("allStarPoints zero value = %v, want 0", got)
	}
}

func TestAllStarPointsGuardsEmptyPool(t *testing.T) {
	if got := allStarPoints(1000, 0, 1, 0); got != 0 {
		t.Fatalf("allStarPoints empty pool = %v, want 0", got)
	}
}

func TestAllStarGradeTiers(t *testing.T) {
	cases := []struct {
		avg       float64
		wantLabel string
		wantClass string
	}{
		{110, "Gold", "text-yellow-400"},
		{95, "Gold", "text-yellow-400"},
		{94.9, "Silver", "text-slate-300"},
		{75, "Silver", "text-slate-300"},
		{74.9, "Bronze", "text-orange-300"},
		{0.1, "Bronze", "text-orange-300"},
		{0, "", ""},
	}
	for _, tt := range cases {
		gotLabel, gotClass := allStarGrade(tt.avg)
		if gotLabel != tt.wantLabel || gotClass != tt.wantClass {
			t.Fatalf("allStarGrade(%v) = %q/%q, want %q/%q", tt.avg, gotLabel, gotClass, tt.wantLabel, tt.wantClass)
		}
	}
}

func TestComputeAllStarSumsBestPerBoss(t *testing.T) {
	rows := []rankRow{
		// Boss 1: two specs, same difficulty. Best DPS points from rank-1 spec (120).
		{BossID: 1, Mode: 4, Spec: 62, DPS: 1000, DPSRank1: 1000, DPSRank: 1, DPSN: 5},
		{BossID: 1, Mode: 4, Spec: 63, DPS: 500, DPSRank1: 1000, DPSRank: 3, DPSN: 5},
		// Boss 2: rank-1 as well (120).
		{BossID: 2, Mode: 4, Spec: 62, DPS: 2000, DPSRank1: 2000, DPSRank: 1, DPSN: 8},
	}
	scores := computeAllStar(rows)
	got := scores[4]
	if got.DPSBosses != 2 {
		t.Fatalf("DPSBosses = %d, want 2", got.DPSBosses)
	}
	if got.DPSPoints != 240 {
		t.Fatalf("DPSPoints = %v, want 240 (120+120)", got.DPSPoints)
	}
}

func TestComputeAllStarSplitsByDifficulty(t *testing.T) {
	// Same boss on two difficulties must NOT merge — each mode scores separately.
	rows := []rankRow{
		{BossID: 1, Mode: 4, Spec: 62, DPS: 1000, DPSRank1: 1000, DPSRank: 1, DPSN: 5},
		{BossID: 1, Mode: 5, Spec: 62, DPS: 800, DPSRank1: 1000, DPSRank: 2, DPSN: 5},
	}
	scores := computeAllStar(rows)
	if len(scores) != 2 {
		t.Fatalf("len(scores) = %d, want 2 (one per difficulty)", len(scores))
	}
	if scores[4].DPSBosses != 1 || scores[4].DPSPoints != 120 {
		t.Fatalf("mode 4 = %d bosses / %v pts, want 1/120", scores[4].DPSBosses, scores[4].DPSPoints)
	}
	// mode 5: ratio=0.8 => 80; pct=100*(1-1/5)=80 => max(80,80)+16 = 96.
	if scores[5].DPSBosses != 1 || scores[5].DPSPoints != 96 {
		t.Fatalf("mode 5 = %d bosses / %v pts, want 1/96", scores[5].DPSBosses, scores[5].DPSPoints)
	}
}

func TestComputeAllStarSeparatesDPSandHPS(t *testing.T) {
	rows := []rankRow{
		{BossID: 1, Mode: 4, Spec: 256, DPS: 0, HPS: 900, HPSRank1: 900, HPSRank: 1, HPSN: 4},
		{BossID: 2, Mode: 4, Spec: 71, DPS: 1500, DPSRank1: 1500, DPSRank: 1, DPSN: 6, HPS: 0},
	}
	got := computeAllStar(rows)[4]
	if got.DPSBosses != 1 || got.HPSBosses != 1 {
		t.Fatalf("bosses dps/hps = %d/%d, want 1/1", got.DPSBosses, got.HPSBosses)
	}
	if got.DPSPoints != 120 || got.HPSPoints != 120 {
		t.Fatalf("points dps/hps = %v/%v, want 120/120", got.DPSPoints, got.HPSPoints)
	}
}

func TestPickDefaultModePrefersMostBosses(t *testing.T) {
	scores := map[int]AllStarScore{
		4: {DPSBosses: 1},
		5: {DPSBosses: 3, HPSBosses: 1},
	}
	if got := pickDefaultMode(scores); got != 5 {
		t.Fatalf("pickDefaultMode = %d, want 5 (most bosses)", got)
	}
}

func TestPickDefaultModeTieBreaksHigherMode(t *testing.T) {
	scores := map[int]AllStarScore{
		4: {DPSBosses: 2},
		6: {DPSBosses: 2},
	}
	if got := pickDefaultMode(scores); got != 6 {
		t.Fatalf("pickDefaultMode = %d, want 6 (higher mode on tie)", got)
	}
}

func TestBuildAllStarViewModelSetsGradesAndDifficulty(t *testing.T) {
	scores := map[int]AllStarScore{
		4: {DPSPoints: 300, DPSBosses: 3}, // avg 100 => Gold
	}
	vm := buildAllStarViewModel("Helios", "Foo", 4, "Raid", []raidActivity{{Name: "Raid"}}, scores, 4)
	if vm.DPSGrade != "Gold" || vm.DPSGradeClass != "text-yellow-400" {
		t.Fatalf("dps grade = %q/%q, want Gold/text-yellow-400", vm.DPSGrade, vm.DPSGradeClass)
	}
	if vm.Mode != 4 || vm.DifficultyLabel == "" {
		t.Fatalf("difficulty = %d/%q, want mode 4 with a label", vm.Mode, vm.DifficultyLabel)
	}
	if len(vm.Difficulties) != 1 || !vm.Difficulties[0].Selected || vm.Difficulties[0].Href == "" {
		t.Fatalf("difficulty options = %#v, want one selected with href", vm.Difficulties)
	}
}

func TestBuildAllStarViewModelComputesAveragesAndFlags(t *testing.T) {
	raids := []raidActivity{{Name: "Throne of Thunder"}, {Name: "Terrace"}}
	scores := map[int]AllStarScore{
		4: {DPSPoints: 245.5, DPSBosses: 3},
		5: {DPSPoints: 100, DPSBosses: 1},
	}
	vm := buildAllStarViewModel("Helios", "Foo", 4, "Throne of Thunder", raids, scores, 4)
	if vm.RaidName != "Throne of Thunder" {
		t.Fatalf("RaidName = %q", vm.RaidName)
	}
	if !vm.HasDPS || vm.DPSPoints != 246 || vm.DPSBosses != 3 {
		t.Fatalf("dps = has:%v pts:%d bosses:%d, want true/246/3", vm.HasDPS, vm.DPSPoints, vm.DPSBosses)
	}
	if got := vm.DPSAvg; got < 81.8 || got > 81.9 {
		t.Fatalf("DPSAvg = %v, want ~81.83", got)
	}
	if vm.HasHPS {
		t.Fatal("HasHPS should be false when no HPS bosses")
	}
	// Raid selector: two options, the selected one flagged.
	if len(vm.Raids) != 2 {
		t.Fatalf("len(Raids) = %d, want 2", len(vm.Raids))
	}
	var selected int
	for _, r := range vm.Raids {
		if r.Selected {
			selected++
			if r.Name != "Throne of Thunder" {
				t.Fatalf("wrong raid selected: %q", r.Name)
			}
		}
		if r.Href == "" {
			t.Fatalf("raid option %q missing Href", r.Name)
		}
	}
	if selected != 1 {
		t.Fatalf("selected count = %d, want 1", selected)
	}
	// Two difficulties available for this raid.
	if len(vm.Difficulties) != 2 {
		t.Fatalf("len(Difficulties) = %d, want 2", len(vm.Difficulties))
	}
}

func TestBuildAllStarViewModelEmptyWhenNoRaids(t *testing.T) {
	vm := buildAllStarViewModel("Helios", "Foo", 4, "", nil, nil, -1)
	if vm.HasDPS || vm.HasHPS || len(vm.Raids) != 0 || len(vm.Difficulties) != 0 {
		t.Fatalf("empty should have no metrics, raids or difficulties: %#v", vm)
	}
}

func TestRaidByLastKillPicksMostRecent(t *testing.T) {
	raids := []raidActivity{
		{Name: "Old", KillCount: 50, LastKill: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Name: "Recent", KillCount: 2, LastKill: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)},
	}
	if got := raidByLastKill(raids); got != "Recent" {
		t.Fatalf("raidByLastKill = %q, want Recent", got)
	}
}

func TestRaidByKillCountPicksMostActive(t *testing.T) {
	raids := []raidActivity{
		{Name: "Old", KillCount: 50, LastKill: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Name: "Recent", KillCount: 2, LastKill: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)},
	}
	if got := raidByKillCount(raids); got != "Old" {
		t.Fatalf("raidByKillCount = %q, want Old", got)
	}
}
