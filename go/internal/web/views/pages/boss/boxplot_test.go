package boss

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildBoxPlotJSONIncludesTooltipMetadata(t *testing.T) {
	var values [99]float64
	for i := range values {
		values[i] = float64((i + 1) * 1000)
	}

	raw, err := buildBoxPlotJSON("DPS", []SpecCurve{{Spec: 267, Values: values}}, "Helios")
	if err != nil {
		t.Fatal(err)
	}

	var opt map[string]any
	if err := json.Unmarshal(raw, &opt); err != nil {
		t.Fatal(err)
	}
	if got := opt["bkTooltip"]; got != "bossBoxplot" {
		t.Fatalf("bkTooltip = %v, want bossBoxplot", got)
	}

	series := opt["series"].([]any)[0].(map[string]any)
	data := series["data"].([]any)[0].(map[string]any)
	if got := data["name"]; got != "Destruction" {
		t.Fatalf("data.name = %v, want Destruction", got)
	}
	for _, key := range []string{"specIcon", "classIcon", "stats"} {
		if data[key] == nil {
			t.Fatalf("missing data.%s in %#v", key, data)
		}
	}
	if !strings.Contains(data["specIcon"].(string), "type=talent") {
		t.Fatalf("specIcon = %q, want talent icon URL", data["specIcon"])
	}
	if strings.Contains(data["name"].(string), "{s267|}") {
		t.Fatalf("tooltip name contains rich label token: %q", data["name"])
	}
}

func TestBuildBoxPlotJSONSupportsClassCurves(t *testing.T) {
	var values [99]float64
	for i := range values {
		values[i] = float64((i + 1) * 1000)
	}

	raw, err := buildBoxPlotJSON("DPS", []SpecCurve{{Class: 8, Values: values}}, "Kronos")
	if err != nil {
		t.Fatal(err)
	}

	var opt map[string]any
	if err := json.Unmarshal(raw, &opt); err != nil {
		t.Fatal(err)
	}
	series := opt["series"].([]any)[0].(map[string]any)
	data := series["data"].([]any)[0].(map[string]any)
	if got := data["name"]; got != "Mage" {
		t.Fatalf("data.name = %v, want Mage", got)
	}
	if data["specIcon"] != "" {
		t.Fatalf("specIcon = %v, want empty for class curve", data["specIcon"])
	}
	if !strings.Contains(data["classIcon"].(string), "type=class") {
		t.Fatalf("classIcon = %q, want class icon URL", data["classIcon"])
	}
}
