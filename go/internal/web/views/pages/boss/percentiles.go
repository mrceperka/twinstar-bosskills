package boss

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mrceperka/twinstar-bosskills/go/internal/metric"
	"github.com/mrceperka/twinstar-bosskills/go/internal/wow"
)

// SpecCurve is a per-spec percentile curve: Values[i] = value at percentile i+1
// (so Values[0]=p1, Values[98]=p99).
type SpecCurve struct {
	Spec   int
	Values [99]float64
}

// curveLevelsCSV builds "0.01, 0.02, ..., 0.99" once at init.
var curveLevelsCSV = func() string {
	var b strings.Builder
	for i := 1; i <= 99; i++ {
		if i > 1 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%g", float64(i)/100)
	}
	return b.String()
}()

// loadSpecCurves fetches a 99-point quantile curve per spec for DPS and HPS
// in a single round trip. Reads from boss_kill directly (no MV) so values
// are always exact — this is feasible because the per-realm/boss/mode cell
// rarely exceeds a few thousand rows. See migration 003 for the rationale.
func loadSpecCurves(ctx context.Context, db *sql.DB, realmName string, id uint32, mode, specFilter, classFilter int, start, end time.Time) (dps, hps []SpecCurve, err error) {
	q := `
		SELECT
			players.talent_spec AS spec,
			quantilesExact(` + curveLevelsCSV + `)(` + metric.SQLFloat64(metric.DmgDoneArrayJoin) + `) AS dps_curve,
			quantilesExact(` + curveLevelsCSV + `)(` + metric.SQLFloat64(metric.HealAbsorbArrayJoin) + `) AS hps_curve
		FROM boss_kill ARRAY JOIN players
		WHERE realm = ? AND boss_remote_id = ? AND mode = ? AND length > 0
		  AND kill_time >= ? AND kill_time < ?`
	args := []any{realmName, id, uint8(mode), start, end}
	if specFilter > 0 {
		q += " AND players.talent_spec = ?"
		args = append(args, uint16(specFilter))
	}
	if classFilter > 0 {
		q += " AND players.class = ?"
		args = append(args, uint8(classFilter))
	}
	q += " GROUP BY spec"
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			spec               uint16
			dpsCurve, hpsCurve []float64
		)
		if err := rows.Scan(&spec, &dpsCurve, &hpsCurve); err != nil {
			return nil, nil, err
		}
		if len(dpsCurve) == 99 && anyPositive(dpsCurve) {
			dps = append(dps, SpecCurve{Spec: int(spec), Values: arrayOf99(dpsCurve)})
		}
		if len(hpsCurve) == 99 && anyPositive(hpsCurve) {
			hps = append(hps, SpecCurve{Spec: int(spec), Values: arrayOf99(hpsCurve)})
		}
	}
	return dps, hps, rows.Err()
}

func arrayOf99(in []float64) [99]float64 {
	var out [99]float64
	copy(out[:], in)
	return out
}

func anyPositive(in []float64) bool {
	for _, v := range in {
		if v > 0 {
			return true
		}
	}
	return false
}

// SpecAtPercentile is one row of "at percentile P, this spec hits this value".
type SpecAtPercentile struct {
	Spec      int
	SpecLabel string
	DPS       int64
	HPS       int64
}

// extractAtPercentile pulls Values[p-1] from each curve and returns rows sorted
// by DPS desc.
func extractAtPercentile(dps, hps []SpecCurve, p int, expansion int) []SpecAtPercentile {
	if p < 1 {
		p = 1
	}
	if p > 99 {
		p = 99
	}
	idx := p - 1
	byKey := map[int]*SpecAtPercentile{}
	for _, c := range dps {
		row := byKey[c.Spec]
		if row == nil {
			row = &SpecAtPercentile{Spec: c.Spec, SpecLabel: wow.SpecForExpansion(expansion, c.Spec)}
			byKey[c.Spec] = row
		}
		row.DPS = int64(c.Values[idx])
	}
	for _, c := range hps {
		row := byKey[c.Spec]
		if row == nil {
			row = &SpecAtPercentile{Spec: c.Spec, SpecLabel: wow.SpecForExpansion(expansion, c.Spec)}
			byKey[c.Spec] = row
		}
		row.HPS = int64(c.Values[idx])
	}
	out := make([]SpecAtPercentile, 0, len(byKey))
	for _, r := range byKey {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].DPS != out[j].DPS {
			return out[i].DPS > out[j].DPS
		}
		return out[i].HPS > out[j].HPS
	})
	return out
}

// buildCurveJSON renders the 99-point curve as a multi-line echarts chart.
// X axis is percentile (1..99), Y is the metric. A vertical markLine
// highlights the selected percentile.
func buildCurveJSON(title string, expansion int, curves []SpecCurve, selectedP int) ([]byte, error) {
	if len(curves) == 0 {
		return []byte("{}"), nil
	}

	// Sort specs by p50 (the curve's middle value) so the legend is ordered.
	sort.Slice(curves, func(i, j int) bool {
		return curves[i].Values[49] > curves[j].Values[49]
	})

	series := make([]any, 0, len(curves))
	legend := make([]string, 0, len(curves))
	for _, c := range curves {
		label := wow.SpecForExpansion(expansion, c.Spec)
		if label == "" {
			label = "Spec " + strconv.Itoa(c.Spec)
		}
		data := make([][2]float64, 99)
		for i, v := range c.Values {
			data[i] = [2]float64{float64(i + 1), v}
		}
		series = append(series, map[string]any{
			"name":       label,
			"type":       "line",
			"data":       data,
			"showSymbol": false,
			"smooth":     true,
			"lineStyle":  map[string]any{"width": 2},
		})
		legend = append(legend, label)
	}

	// Vertical reference line at the selected percentile, attached to the
	// first series.
	if len(series) > 0 {
		series[0] = withMarkLine(series[0].(map[string]any), selectedP)
	}

	opt := map[string]any{
		"title": map[string]any{
			"text":      title,
			"left":      "center",
			"textStyle": map[string]any{"fontSize": 12, "color": "#7a8294"},
		},
		"tooltip": map[string]any{
			"trigger": "axis",
			"axisPointer": map[string]any{
				"type": "cross",
			},
		},
		"legend": map[string]any{
			"data":      legend,
			"top":       "8%",
			"textStyle": map[string]any{"fontSize": 10, "color": "#d8dde6"},
			"type":      "scroll",
		},
		"grid": map[string]any{
			"left":         "8%",
			"right":        "4%",
			"top":          "22%",
			"bottom":       "10%",
			"containLabel": true,
		},
		"xAxis": map[string]any{
			"type":         "value",
			"name":         "Percentile",
			"nameLocation": "middle",
			"nameGap":      24,
			"min":          1,
			"max":          99,
			"axisLabel":    map[string]any{"color": "#7a8294", "formatter": "p{value}"},
			"splitLine":    map[string]any{"lineStyle": map[string]any{"color": "#232936"}},
		},
		"yAxis": map[string]any{
			"type":      "value",
			"axisLabel": map[string]any{"color": "#7a8294"},
			"splitLine": map[string]any{"lineStyle": map[string]any{"color": "#232936"}},
		},
		"series": series,
	}
	return json.Marshal(opt)
}

func withMarkLine(series map[string]any, p int) map[string]any {
	series["markLine"] = map[string]any{
		"silent": true,
		"symbol": []string{"none", "none"},
		"label": map[string]any{
			"formatter": "p" + strconv.Itoa(p),
			"color":     "#d4a017",
		},
		"lineStyle": map[string]any{
			"color": "#d4a017",
			"type":  "dashed",
			"width": 1,
		},
		"data": []any{
			map[string]any{"xAxis": p},
		},
	}
	return series
}
