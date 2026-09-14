package characterperf

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBuildBossChartIncludesTooltipMetadata(t *testing.T) {
	samples := []Sample{{
		Time:       time.Unix(1700000000, 123000000),
		RemoteID:   "18_12345",
		BossID:     71865,
		BossName:   "Garrosh Hellscream",
		Mode:       6,
		DPS:        123456,
		HPS:        7890,
		Spec:       71,
		AvgItemLvl: 567.8,
	}}

	got, err := buildBossChart("Helios", samples, 100000, 5000)
	if err != nil {
		t.Fatalf("buildBossChart() error = %v", err)
	}

	var opt map[string]any
	if err := json.Unmarshal(got, &opt); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	tooltip := opt["tooltip"].(map[string]any)
	if _, ok := tooltip["formatter"]; ok {
		t.Fatalf("tooltip.formatter = %v, want no literal formatter in JSON", tooltip["formatter"])
	}
	if opt["bkTooltip"] != "characterPerf" {
		t.Fatalf("bkTooltip = %v, want characterPerf", opt["bkTooltip"])
	}

	dpsPoint := seriesData(opt, "DPS").([]any)[0].(map[string]any)
	if dpsPoint["avgItemLvl"] != 567.8 {
		t.Fatalf("DPS point avgItemLvl = %v, want 567.8", dpsPoint["avgItemLvl"])
	}
	if dpsPoint["detailUrl"] != "/Helios/boss-kills/18_12345" {
		t.Fatalf("DPS point detailUrl = %v, want /Helios/boss-kills/18_12345", dpsPoint["detailUrl"])
	}
	values := dpsPoint["value"].([]any)
	if values[1] != float64(123456) {
		t.Fatalf("DPS point value[1] = %v, want 123456", values[1])
	}

	hpsPoint := seriesData(opt, "HPS").([]any)[0].(map[string]any)
	if hpsPoint["avgItemLvl"] != 567.8 {
		t.Fatalf("HPS point avgItemLvl = %v, want 567.8", hpsPoint["avgItemLvl"])
	}
	if hpsPoint["detailUrl"] != "/Helios/boss-kills/18_12345" {
		t.Fatalf("HPS point detailUrl = %v, want /Helios/boss-kills/18_12345", hpsPoint["detailUrl"])
	}
	values = hpsPoint["value"].([]any)
	if values[1] != float64(7890) {
		t.Fatalf("HPS point value[1] = %v, want 7890", values[1])
	}
}

func seriesData(chart map[string]any, name string) any {
	for _, raw := range chart["series"].([]any) {
		series := raw.(map[string]any)
		if series["name"] == name {
			return series["data"]
		}
	}
	return nil
}
