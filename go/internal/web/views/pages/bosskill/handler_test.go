package bosskill

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestBuildTimelineJSONUsesCategoryAxis(t *testing.T) {
	got, err := buildTimelineJSON(
		[]int32{2, 0, 1},
		[]uint64{20, 0, 10},
		[]uint64{21, 1, 11},
		[]uint64{22, 2, 12},
		[]uint64{23, 3, 13},
		nil,
	)
	if err != nil {
		t.Fatalf("buildTimelineJSON() error = %v", err)
	}

	var opt map[string]any
	if err := json.Unmarshal(got, &opt); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	xAxis := opt["xAxis"].(map[string]any)
	if xAxis["type"] != "category" {
		t.Fatalf("xAxis.type = %v, want category", xAxis["type"])
	}
	if xAxis["boundaryGap"] != false {
		t.Fatalf("xAxis.boundaryGap = %v, want false", xAxis["boundaryGap"])
	}
	if gotData := numbers(xAxis["data"]); !reflect.DeepEqual(gotData, []float64{0, 1, 2}) {
		t.Fatalf("xAxis.data = %v, want [0 1 2]", gotData)
	}

	enemyDamage := seriesData(opt, "Enemy Damage")
	if gotData := numbers(enemyDamage); !reflect.DeepEqual(gotData, []float64{0, 10, 20}) {
		t.Fatalf("Enemy Damage data = %v, want [0 10 20]", gotData)
	}
	for _, v := range enemyDamage.([]any) {
		if _, ok := v.([]any); ok {
			t.Fatalf("Enemy Damage data item = %v, want scalar category-aligned value", v)
		}
	}
}

func seriesData(timeline map[string]any, name string) any {
	for _, raw := range timeline["series"].([]any) {
		series := raw.(map[string]any)
		if series["name"] == name {
			return series["data"]
		}
	}
	return nil
}

func numbers(raw any) []float64 {
	values := raw.([]any)
	out := make([]float64, 0, len(values))
	for _, v := range values {
		out = append(out, v.(float64))
	}
	return out
}
