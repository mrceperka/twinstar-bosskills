package characterperf

import (
	"encoding/json"
	"time"
)

// Sample is one kill-time point for a character's performance on a specific boss+mode.
type Sample struct {
	Time       time.Time
	BossID     uint32
	BossName   string
	Mode       int
	DPS        int64
	HPS        int64
	Spec       int
	AvgItemLvl float32
}

// BossChart holds the echarts config for a single (boss, mode) combination.
type BossChart struct {
	BossID    uint32
	BossName  string
	Mode      int
	ModeLabel string
	ChartJSON []byte
}

// buildBossChart builds an echarts config showing DPS (gold) and HPS (green) over
// time for one (boss, mode). dpsMedian/hpsMedian are shown as dashed p50 lines
// when > 0.
func buildBossChart(samples []Sample, dpsMedian, hpsMedian int64) ([]byte, error) {
	if len(samples) == 0 {
		return []byte("{}"), nil
	}

	dpsData := make([][2]any, len(samples))
	hpsData := make([][2]any, len(samples))
	for i, s := range samples {
		ts := s.Time.UnixMilli()
		dpsData[i] = [2]any{ts, s.DPS}
		hpsData[i] = [2]any{ts, s.HPS}
	}

	dpsSeries := map[string]any{
		"name":       "DPS",
		"type":       "line",
		"color":      "#d4af37",
		"data":       dpsData,
		"smooth":     false,
		"showSymbol": true,
		"symbolSize": 4,
	}
	if dpsMedian > 0 {
		dpsSeries["markLine"] = medianMarkLine(dpsMedian, "p50 DPS", "#d4af37")
	}

	hpsSeries := map[string]any{
		"name":       "HPS",
		"type":       "line",
		"color":      "#22c55e",
		"data":       hpsData,
		"smooth":     false,
		"showSymbol": true,
		"symbolSize": 4,
	}
	if hpsMedian > 0 {
		hpsSeries["markLine"] = medianMarkLine(hpsMedian, "p50 HPS", "#22c55e")
	}

	opt := map[string]any{
		"tooltip": map[string]any{
			"trigger": "axis",
		},
		"legend": map[string]any{
			"data":      []string{"DPS", "HPS"},
			"textStyle": map[string]any{"fontSize": 10, "color": "#d8dde6"},
		},
		"grid": map[string]any{
			"left":         "8%",
			"right":        "4%",
			"top":          "18%",
			"bottom":       "10%",
			"containLabel": true,
		},
		"xAxis": map[string]any{
			"type":      "time",
			"axisLabel": map[string]any{"color": "#7a8294", "fontSize": 10},
			"splitLine": map[string]any{"lineStyle": map[string]any{"color": "#232936"}},
		},
		"yAxis": map[string]any{
			"type":      "value",
			"axisLabel": map[string]any{"color": "#7a8294"},
			"splitLine": map[string]any{"lineStyle": map[string]any{"color": "#232936"}},
		},
		"series": []any{dpsSeries, hpsSeries},
	}
	return json.Marshal(opt)
}

func medianMarkLine(value int64, label, color string) map[string]any {
	return map[string]any{
		"silent": true,
		"symbol": []string{"none", "none"},
		"label": map[string]any{
			"formatter": label,
			"position":  "insideEndTop",
			"color":     color,
			"fontSize":  9,
		},
		"lineStyle": map[string]any{
			"type":    "dashed",
			"color":   color,
			"opacity": 0.6,
		},
		"data": []any{
			map[string]any{"yAxis": value},
		},
	}
}
