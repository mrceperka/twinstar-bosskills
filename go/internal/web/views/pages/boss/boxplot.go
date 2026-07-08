package boss

import (
	"encoding/json"
	"sort"
	"strconv"

	"twinstar-bosskills/internal/links"
	"twinstar-bosskills/internal/wow"
)

// buildBoxPlotJSON renders a horizontal echarts boxplot config from per-spec
// curves. Mirrors the existing SvelteKit "DPS by Talent Spec" chart: one row
// per spec, X axis = metric value, Y axis = spec label (categorical).
//
// Y-axis labels embed class + spec icons via ECharts rich-text syntax so that
// each row shows "[class-icon][spec-icon] Spec Name" — matching the SvelteKit
// BossPerformanceBoxChart component.
//
// Each box uses 5 points pulled from the curve:
//
//	[min, Q1, median, Q3, max]  at p1, p25, p50, p75, p99
//
// Specs are sorted by median desc so the top performer appears at the top.
func buildBoxPlotJSON(title string, curves []SpecCurve, realmName string) ([]byte, error) {
	if len(curves) == 0 {
		return []byte("{}"), nil
	}

	sorted := append([]SpecCurve{}, curves...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Values[49] > sorted[j].Values[49]
	})

	// Build ECharts rich-text style map: one entry per unique spec icon and
	// one per unique class icon. Keys must be valid identifiers, so we prefix
	// with "s" (spec) and "c" (class).
	rich := map[string]any{}
	iconStyle := func(url string) map[string]any {
		return map[string]any{
			"width":  20,
			"height": 20,
			"align":  "center",
			"backgroundColor": map[string]any{
				"image": url,
			},
		}
	}

	categories := make([]string, 0, len(sorted))
	boxes := make([]any, 0, len(sorted))
	for _, c := range sorted {
		q1 := c.Values[24]
		med := c.Values[49]
		q3 := c.Values[74]
		mn := c.Values[0]  // p1
		mx := c.Values[98] // p99

		classMode := c.Spec == 0 && c.Class > 0
		class := wow.ClassFromSpecForRealm(realmName, c.Spec)
		specLabel := wow.SpecForRealm(realmName, c.Spec)
		if classMode {
			class = c.Class
			specLabel = wow.Class(c.Class)
		}
		if specLabel == "" {
			specLabel = "Group " + strconv.Itoa(c.Spec)
			if classMode {
				specLabel = "Class " + strconv.Itoa(c.Class)
			}
		}
		specKey := "s" + strconv.Itoa(c.Spec)
		classKey := "c" + strconv.Itoa(class)
		specIcon := links.TalentIcon(realmName, c.Spec)
		classIcon := ""
		if class > 0 {
			classIcon = links.ClassIcon(class)
		}

		if _, ok := rich[specKey]; !ok && !classMode {
			rich[specKey] = iconStyle(specIcon)
		}
		if _, ok := rich[classKey]; !ok && class > 0 {
			rich[classKey] = iconStyle(classIcon)
		}

		// "{classKey|}{specKey|} Spec Name" — ECharts parses rich-text tokens
		// after substituting {value} so the icons render inline with the label.
		label := "{" + classKey + "|}{" + specKey + "|} " + specLabel
		if classMode {
			label = "{" + classKey + "|} " + specLabel
			specIcon = ""
		}
		categories = append(categories, label)
		boxes = append(boxes, map[string]any{
			"name":      specLabel,
			"value":     [5]float64{mn, q1, med, q3, mx},
			"spec":      c.Spec,
			"specLabel": specLabel,
			"specIcon":  specIcon,
			"class":     class,
			"classIcon": classIcon,
			"stats": map[string]float64{
				"min":    mn,
				"q1":     q1,
				"median": med,
				"q3":     q3,
				"max":    mx,
			},
		})
	}

	opt := map[string]any{
		"backgroundColor": "transparent",
		"animation":       false,
		"bkTooltip":       "bossBoxplot",
		"tooltip": map[string]any{
			"trigger": "item",
		},
		"grid": map[string]any{
			"left":         "1%",
			"right":        "4%",
			"top":          "1%",
			"bottom":       "2%",
			"containLabel": true,
		},
		"xAxis": map[string]any{
			"type":      "value",
			"scale":     true,
			"axisLabel": map[string]any{"color": "#c5c5c5"},
			"axisLine":  map[string]any{"lineStyle": map[string]any{"color": "#c5c5c5"}},
			"axisTick":  map[string]any{"lineStyle": map[string]any{"color": "#c5c5c5"}},
			"splitLine": map[string]any{"lineStyle": map[string]any{"color": "rgba(255,255,255,0.07)"}},
		},
		"yAxis": map[string]any{
			"type":      "category",
			"data":      categories,
			"inverse":   true,
			"splitArea": map[string]any{"show": false},
			"axisLabel": map[string]any{
				"color":    "#c5c5c5",
				"fontSize": 11,
				"rich":     rich,
			},
		},
		"series": []any{
			map[string]any{
				"name":     title,
				"type":     "boxplot",
				"data":     boxes,
				"boxWidth": []string{"38%", "62%"},
				"itemStyle": map[string]any{
					"color":       "rgba(218,165,32,0.25)",
					"borderColor": "#daa520",
				},
			},
		},
	}
	return json.Marshal(opt)
}
