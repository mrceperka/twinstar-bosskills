package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/mrceperka/twinstar-bosskills/go/internal/domain"
	"github.com/mrceperka/twinstar-bosskills/go/internal/realm"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/middleware"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/router"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/views/layouts"
	"github.com/mrceperka/twinstar-bosskills/go/internal/wow"
)

type Deps struct {
	DB      *sql.DB
	CSSHash string
	JSHash  string
}

const topBossLimit = 14

func Handler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		realmName := middleware.Realm(r.Context())

		ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
		defer cancel()

		expansion := realm.Expansion(realmName)
		now := time.Now().UTC()
		curWin := domain.RaidLock(now, 0)
		prevWin := domain.RaidLock(now, 1)

		curr, err := loadLockSummary(ctx, deps.DB, realmName, curWin, expansion)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		prev, err := loadLockSummary(ctx, deps.DB, realmName, prevWin, expansion)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		vm := ViewModel{
			Meta: layouts.PageMeta{
				Title:      realmName,
				Realm:      realmName,
				CSSHash:    deps.CSSHash,
				JSHash:     deps.JSHash,
				NeedsChart: true,
			},
			Realm:        realmName,
			CurrentLock:  curr,
			PreviousLock: prev,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = Page(vm).Render(r.Context(), w)
	}
}

// loadLockSummary builds the per-lockout block: totals, top kills, top wipes,
// and the bar-chart configs for day-of-week + hour-of-day.
func loadLockSummary(ctx context.Context, db *sql.DB, realmName string, win domain.RaidLockWindow, expansion int) (LockSummary, error) {
	s := LockSummary{
		StartLabel: win.Start.Format("01/02/2006, 3:04 PM"),
		EndLabel:   win.End.Format("01/02/2006, 3:04 PM"),
	}

	// Totals.
	const totalsQ = `
		SELECT count() AS kills, sum(wipes) AS wipes
		FROM boss_kill
		WHERE realm = ? AND kill_time >= ? AND kill_time < ?
	`
	var kills, wipes uint64
	if err := db.QueryRowContext(ctx, totalsQ, realmName, win.Start, win.End).Scan(&kills, &wipes); err != nil {
		return s, err
	}
	s.TotalKills = int(kills)
	s.TotalWipes = int(wipes)
	if kills+wipes > 0 {
		s.WipeChance = 100 * float64(wipes) / float64(kills+wipes)
	}

	// Most kills (top boss/mode combos).
	rows, err := db.QueryContext(ctx, `
		SELECT count() AS c, boss_remote_id, any(boss_name), mode
		FROM boss_kill
		WHERE realm = ? AND kill_time >= ? AND kill_time < ?
		GROUP BY boss_remote_id, mode
		ORDER BY c DESC
		LIMIT ?
	`, realmName, win.Start, win.End, topBossLimit)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var (
			c        uint64
			bossID   uint32
			bossName string
			mode     uint8
		)
		if err := rows.Scan(&c, &bossID, &bossName, &mode); err != nil {
			rows.Close()
			return s, err
		}
		s.TopKills = append(s.TopKills, BossCount{
			Count: int(c), BossID: bossID, BossName: bossName,
			ModeLabel: wow.Difficulty(expansion, int(mode)),
		})
	}
	rows.Close()

	// Most wipes.
	rows, err = db.QueryContext(ctx, `
		SELECT sum(wipes) AS w, boss_remote_id, any(boss_name), mode
		FROM boss_kill
		WHERE realm = ? AND kill_time >= ? AND kill_time < ? AND wipes > 0
		GROUP BY boss_remote_id, mode
		ORDER BY w DESC
		LIMIT ?
	`, realmName, win.Start, win.End, topBossLimit)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var (
			w        uint64
			bossID   uint32
			bossName string
			mode     uint8
		)
		if err := rows.Scan(&w, &bossID, &bossName, &mode); err != nil {
			rows.Close()
			return s, err
		}
		s.TopWipes = append(s.TopWipes, BossCount{
			Count: int(w), BossID: bossID, BossName: bossName,
			ModeLabel: wow.Difficulty(expansion, int(mode)),
		})
	}
	rows.Close()

	// By day of week. CH: toDayOfWeek returns Mon=1..Sun=7.
	dayCounts := [7]int{}
	rows, err = db.QueryContext(ctx, `
		SELECT toDayOfWeek(kill_time) AS d, count()
		FROM boss_kill
		WHERE realm = ? AND kill_time >= ? AND kill_time < ?
		GROUP BY d
	`, realmName, win.Start, win.End)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var d uint8
		var c uint64
		if err := rows.Scan(&d, &c); err != nil {
			rows.Close()
			return s, err
		}
		if d >= 1 && d <= 7 {
			dayCounts[d-1] = int(c)
		}
	}
	rows.Close()

	// Reorder: existing app starts the bar chart on Wednesday (raid reset day).
	dayLabels := []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
	order := []int{2, 3, 4, 5, 6, 0, 1} // Wed, Thu, Fri, Sat, Sun, Mon, Tue
	dayValues := make([]int, 7)
	orderedLabels := make([]string, 7)
	maxDayIdx, maxDayVal := 0, -1
	for i, idx := range order {
		dayValues[i] = dayCounts[idx]
		orderedLabels[i] = dayLabels[idx]
		if dayValues[i] > maxDayVal {
			maxDayVal = dayValues[i]
			maxDayIdx = i
		}
	}
	if maxDayVal > 0 {
		s.TopDayName = orderedLabels[maxDayIdx]
	}
	s.ByDayJSON, _ = buildBarChartJSON(orderedLabels, dayValues)

	// By hour 00..23.
	hourCounts := [24]int{}
	rows, err = db.QueryContext(ctx, `
		SELECT toHour(kill_time) AS h, count()
		FROM boss_kill
		WHERE realm = ? AND kill_time >= ? AND kill_time < ?
		GROUP BY h
	`, realmName, win.Start, win.End)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var h uint8
		var c uint64
		if err := rows.Scan(&h, &c); err != nil {
			rows.Close()
			return s, err
		}
		if h < 24 {
			hourCounts[h] = int(c)
		}
	}
	rows.Close()

	hourLabels := make([]string, 24)
	hourValues := make([]int, 24)
	maxHourIdx, maxHourVal := 0, -1
	for h := 0; h < 24; h++ {
		hourLabels[h] = leftPad2(h) + ":00"
		hourValues[h] = hourCounts[h]
		if hourCounts[h] > maxHourVal {
			maxHourVal = hourCounts[h]
			maxHourIdx = h
		}
	}
	if maxHourVal > 0 {
		s.TopHourName = hourLabels[maxHourIdx]
	}
	s.ByHourJSON, _ = buildBarChartJSON(hourLabels, hourValues)

	return s, nil
}

func leftPad2(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

// buildBarChartJSON renders a simple categorical bar chart (gold bars on
// dark background) used for the day-of-week / hour-of-day breakdowns.
func buildBarChartJSON(categories []string, values []int) ([]byte, error) {
	opt := map[string]any{
		"backgroundColor": "transparent",
		"tooltip":         map[string]any{"trigger": "axis"},
		"grid": map[string]any{
			"left":         "3%",
			"right":        "1%",
			"top":          "5%",
			"bottom":       "10%",
			"containLabel": true,
		},
		"xAxis": map[string]any{
			"type":      "category",
			"data":      categories,
			"axisLabel": map[string]any{"color": "#c5c5c5"},
			"splitLine": map[string]any{"show": false},
		},
		"yAxis": map[string]any{
			"type":      "value",
			"axisLabel": map[string]any{"color": "#c5c5c5"},
			"splitLine": map[string]any{"lineStyle": map[string]any{"color": "rgba(255,255,255,0.07)"}},
		},
		"series": []any{
			map[string]any{
				"type":           "bar",
				"data":           values,
				"itemStyle":      map[string]any{"color": "#daa520"},
				"barCategoryGap": "20%",
			},
		},
	}
	return json.Marshal(opt)
}

func Mount(mux *http.ServeMux, deps Deps) {
	h := middleware.RequireRealm(Handler(deps))
	router.ForEachRealmPrefix(func(prefix string) {
		mux.Handle("GET "+prefix+"/{$}", h)
		mux.Handle("GET "+prefix, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, prefix+"/", http.StatusFound)
		}))
	})
}
