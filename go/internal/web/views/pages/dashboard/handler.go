package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"time"

	"twinstar-bosskills/internal/domain"
	"twinstar-bosskills/internal/metric"
	"twinstar-bosskills/internal/realm"
	"twinstar-bosskills/internal/web/middleware"
	"twinstar-bosskills/internal/web/query"
	"twinstar-bosskills/internal/web/router"
	"twinstar-bosskills/internal/web/sqlutil"
	"twinstar-bosskills/internal/web/views/layouts"
	"twinstar-bosskills/internal/wow"
)

type Deps struct {
	DB      *sql.DB
	CSSHash string
	JSHash  string
}

const (
	topBossLimit               = 14
	topPerformerDefaultLimit   = 5
	topPerformerMaxLimit       = 25
	topPerformerLimitIncrement = 5
)

func Handler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		realmName := middleware.Realm(r.Context())
		if middleware.GatePrivateRealm(w, r) {
			return
		}
		guildFilter := middleware.PrivateRealmGuildFilter(r)

		ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
		defer cancel()

		expansion := realm.Expansion(realmName)
		now := time.Now().UTC()

		var selectedPerformerBoss uint32
		if v, err := strconv.ParseUint(r.URL.Query().Get("perf_boss"), 10, 32); err == nil {
			selectedPerformerBoss = uint32(v)
		}
		selectedPerformerLockOffset, _ := middleware.RaidLock(r.Context())
		performerWin := domain.RaidLock(now, selectedPerformerLockOffset)
		performerLimit := topPerformerDefaultLimit
		if requestedPerformerLimit, ok := query.Int(r.URL.Query(), 1, "perf_limit"); ok && requestedPerformerLimit > performerLimit {
			performerLimit = requestedPerformerLimit
			if performerLimit > topPerformerMaxLimit {
				performerLimit = topPerformerMaxLimit
			}
		}
		requestedPerformerMode, hasRequestedPerformerMode := query.Difficulty(r.URL.Query())
		performerRaid, performerDifficulties, selectedPerformerMode, performers, err := loadCurrentLockPerformers(ctx, deps.DB, realmName, guildFilter, performerWin, expansion, selectedPerformerBoss, requestedPerformerMode, hasRequestedPerformerMode, performerLimit)
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
			Realm:                 realmName,
			PerformerLock:         lockSummaryLabels(performerWin),
			PerformerLockOffset:   selectedPerformerLockOffset,
			CurrentPerformerRaid:  performerRaid,
			PerformerDifficulties: performerDifficulties,
			SelectedPerformerMode: selectedPerformerMode,
			PerformerLimit:        performerLimit,
			CurrentPerformers:     performers,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// Performer navigation is driven by htmx: swap just the #perf-shell
		// fragment instead of re-rendering the whole dashboard (and re-running
		// the two lockout-summary query sets below).
		if middleware.IsHTMX(r) {
			_ = PerformersShell(vm).Render(r.Context(), w)
			return
		}

		// Full page also needs the current + previous lockout summaries.
		curr, err := loadLockSummary(ctx, deps.DB, realmName, guildFilter, domain.RaidLock(now, 0), expansion)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		prev, err := loadLockSummary(ctx, deps.DB, realmName, guildFilter, domain.RaidLock(now, 1), expansion)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		vm.CurrentLock = curr
		vm.PreviousLock = prev
		_ = Page(vm).Render(r.Context(), w)
	}
}

func lockSummaryLabels(win domain.RaidLockWindow) LockSummary {
	return LockSummary{
		StartLabel: win.Start.Format("01/02/2006, 3:04 PM"),
		EndLabel:   win.End.Format("01/02/2006, 3:04 PM"),
	}
}

// loadLockSummary builds the per-lockout block: totals, top kills, top wipes,
// and the bar-chart configs for day-of-week + hour-of-day.
func loadLockSummary(ctx context.Context, db *sql.DB, realmName, guildFilter string, win domain.RaidLockWindow, expansion int) (LockSummary, error) {
	s := lockSummaryLabels(win)
	guildWhere, guildArgs := sqlutil.GuildFilter(guildFilter)

	// Totals.
	const totalsQ = `
		SELECT count() AS kills, sum(wipes) AS wipes
		FROM boss_kill
		WHERE realm = ? AND kill_time >= ? AND kill_time < ?
	`
	var kills, wipes uint64
	totalsArgs := append([]any{realmName, win.Start, win.End}, guildArgs...)
	if err := db.QueryRowContext(ctx, totalsQ+guildWhere, totalsArgs...).Scan(&kills, &wipes); err != nil {
		return s, err
	}
	s.TotalKills = int(kills)
	s.TotalWipes = int(wipes)
	if kills+wipes > 0 {
		s.WipeChance = 100 * float64(wipes) / float64(kills+wipes)
	}

	// Most kills (top boss/mode combos).
	topKillsArgs := append([]any{realmName, win.Start, win.End}, guildArgs...)
	topKillsArgs = append(topKillsArgs, topBossLimit)
	rows, err := db.QueryContext(ctx, `
		SELECT count() AS c, boss_remote_id, any(boss_name), mode
		FROM boss_kill
		WHERE realm = ? AND kill_time >= ? AND kill_time < ?`+guildWhere+`
		GROUP BY boss_remote_id, mode
		ORDER BY c DESC
		LIMIT ?
	`, topKillsArgs...)
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
			Count:     int(c),
			BossID:    bossID,
			BossName:  bossName,
			Mode:      mode,
			ModeLabel: wow.Difficulty(expansion, int(mode)),
		})
	}
	rows.Close()

	// Most wipes.
	topWipesArgs := append([]any{realmName, win.Start, win.End}, guildArgs...)
	topWipesArgs = append(topWipesArgs, topBossLimit)
	rows, err = db.QueryContext(ctx, `
		SELECT sum(wipes) AS w, boss_remote_id, any(boss_name), mode
		FROM boss_kill
		WHERE realm = ? AND kill_time >= ? AND kill_time < ? AND wipes > 0`+guildWhere+`
		GROUP BY boss_remote_id, mode
		ORDER BY w DESC
		LIMIT ?
	`, topWipesArgs...)
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
			Count:     int(w),
			BossID:    bossID,
			BossName:  bossName,
			Mode:      mode,
			ModeLabel: wow.Difficulty(expansion, int(mode)),
		})
	}
	rows.Close()

	// By day of week. CH: toDayOfWeek returns Mon=1..Sun=7.
	dayCounts := [7]int{}
	dayArgs := append([]any{realmName, win.Start, win.End}, guildArgs...)
	rows, err = db.QueryContext(ctx, `
		SELECT toDayOfWeek(kill_time) AS d, count()
		FROM boss_kill
		WHERE realm = ? AND kill_time >= ? AND kill_time < ?`+guildWhere+`
		GROUP BY d
	`, dayArgs...)
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
	hourArgs := append([]any{realmName, win.Start, win.End}, guildArgs...)
	rows, err = db.QueryContext(ctx, `
		SELECT toHour(kill_time) AS h, count()
		FROM boss_kill
		WHERE realm = ? AND kill_time >= ? AND kill_time < ?`+guildWhere+`
		GROUP BY h
	`, hourArgs...)
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

type latestRaidBoss struct {
	RaidName string
	RemoteID uint32
	Name     string
	Position uint16
}

func loadLatestRaidBosses(ctx context.Context, db *sql.DB, realmName string) (string, map[uint32]latestRaidBoss, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT b.raid_name, b.remote_id, b.name, b.position, ifNull(r.position, 0)
		FROM (
			SELECT raid_name, remote_id, name, position
			FROM boss FINAL
			WHERE realm = ?
		) AS b
		LEFT ANY JOIN (
			SELECT name, position
			FROM raid FINAL
			WHERE realm = ?
		) AS r ON r.name = b.raid_name
	`, realmName, realmName)
	if err != nil {
		return "", nil, err
	}
	defer rows.Close()

	type row struct {
		boss         latestRaidBoss
		raidPosition uint16
	}
	var all []row
	raidPositions := map[string]uint16{}
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.boss.RaidName, &r.boss.RemoteID, &r.boss.Name, &r.boss.Position, &r.raidPosition); err != nil {
			return "", nil, err
		}
		if _, ok := raidPositions[r.boss.RaidName]; !ok || raidPositions[r.boss.RaidName] == 0 {
			raidPositions[r.boss.RaidName] = r.raidPosition
		}
		all = append(all, r)
	}
	if err := rows.Err(); err != nil {
		return "", nil, err
	}
	raidNames := make([]string, 0, len(raidPositions))
	for name := range raidPositions {
		raidNames = append(raidNames, name)
	}
	sort.Slice(raidNames, func(i, j int) bool {
		pi := raidPositions[raidNames[i]]
		pj := raidPositions[raidNames[j]]
		if pi == 0 && pj == 0 {
			return raidNames[i] < raidNames[j]
		}
		if pi == 0 {
			return false
		}
		if pj == 0 {
			return true
		}
		if pi != pj {
			return pi < pj
		}
		return raidNames[i] < raidNames[j]
	})
	if len(raidNames) == 0 {
		return "", nil, nil
	}
	bestRaidName := raidNames[len(raidNames)-1]

	bosses := map[uint32]latestRaidBoss{}
	for _, r := range all {
		if r.boss.RaidName == bestRaidName {
			bosses[r.boss.RemoteID] = r.boss
		}
	}
	return bestRaidName, bosses, nil
}

func loadCurrentLockPerformerModes(ctx context.Context, db *sql.DB, realmName, guildFilter, raidName string, win domain.RaidLockWindow, expansion int) ([]DifficultyChoice, error) {
	guildWhere, guildArgs := sqlutil.GuildFilter(guildFilter)
	args := append([]any{realmName, raidName, win.Start, win.End}, guildArgs...)
	rows, err := db.QueryContext(ctx, `
		SELECT DISTINCT mode
		FROM boss_kill
		WHERE realm = ? AND raid_name = ? AND kill_time >= ? AND kill_time < ?`+guildWhere+`
		ORDER BY mode
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DifficultyChoice
	for rows.Next() {
		var mode uint8
		if err := rows.Scan(&mode); err != nil {
			return nil, err
		}
		out = append(out, DifficultyChoice{Mode: int(mode), Label: wow.Difficulty(expansion, int(mode))})
	}
	return out, rows.Err()
}

func selectPerformerMode(requested int, hasRequested bool, available []DifficultyChoice, expansion int) int {
	if hasRequested {
		for _, d := range available {
			if d.Mode == requested {
				return requested
			}
		}
	}
	def := wow.DefaultDifficulty(expansion)
	for _, d := range available {
		if d.Mode == def {
			return def
		}
	}
	if len(available) > 0 {
		return available[0].Mode
	}
	return def
}

func loadCurrentLockPerformers(ctx context.Context, db *sql.DB, realmName, guildFilter string, win domain.RaidLockWindow, expansion int, selectedBossID uint32, requestedMode int, hasRequestedMode bool, performerLimit int) (string, []DifficultyChoice, int, []BossPerformers, error) {
	raidName, latestBosses, err := loadLatestRaidBosses(ctx, db, realmName)
	if err != nil {
		return "", nil, 0, nil, err
	}
	if raidName == "" {
		return "", nil, 0, nil, nil
	}
	difficulties, err := loadCurrentLockPerformerModes(ctx, db, realmName, guildFilter, raidName, win, expansion)
	if err != nil {
		return "", nil, 0, nil, err
	}
	selectedMode := selectPerformerMode(requestedMode, hasRequestedMode, difficulties, expansion)

	guildWhere, guildArgs := sqlutil.GuildFilter(guildFilter)
	args := append([]any{realmName, raidName, uint8(selectedMode), win.Start, win.End}, guildArgs...)
	rows, err := db.QueryContext(ctx, `
		SELECT
			boss_remote_id,
			boss_name,
			mode,
			players.guid                                    AS guid,
			players.talent_spec                             AS spec,
			players.name                                    AS name,
			players.class                                   AS class,
			`+metric.SQLUInt64(metric.DmgDoneArrayJoin)+` AS dps,
			`+metric.SQLUInt64(metric.HealAbsorbArrayJoin)+` AS hps,
			toUInt64(players.dmg_done)                       AS dmg_done,
			toUInt64(players.healing_done + players.absorb_done) AS heal_done,
			length                                           AS len,
			kill_time                                        AS kill_time,
			players.avg_item_lvl                             AS ilvl,
			remote_id                                        AS remote_id
		FROM boss_kill ARRAY JOIN players
		WHERE realm = ? AND raid_name = ? AND mode = ? AND kill_time >= ? AND kill_time < ? AND length > 0`+guildWhere+`
	`, args...)
	if err != nil {
		return "", nil, 0, nil, err
	}
	defer rows.Close()

	type sample struct {
		BossID   uint32
		BossName string
		Mode     uint8
		Guid     uint64
		Spec     uint16
		Name     string
		Class    uint8
		DPS, HPS uint64
		DmgDone  uint64
		HealDone uint64
		Length   uint32
		KillTime time.Time
		ILvl     float32
		RemoteID string
	}
	type performerKey struct {
		BossID uint32
		Guid   uint64
		Spec   uint16
	}

	bestDPS := map[performerKey]sample{}
	bestHPS := map[performerKey]sample{}
	for rows.Next() {
		var s sample
		if err := rows.Scan(&s.BossID, &s.BossName, &s.Mode, &s.Guid, &s.Spec, &s.Name, &s.Class, &s.DPS, &s.HPS,
			&s.DmgDone, &s.HealDone, &s.Length, &s.KillTime, &s.ILvl, &s.RemoteID); err != nil {
			return "", nil, 0, nil, err
		}
		if _, ok := latestBosses[s.BossID]; !ok {
			continue
		}
		k := performerKey{BossID: s.BossID, Guid: s.Guid, Spec: s.Spec}
		if prev, ok := bestDPS[k]; !ok || s.DPS > prev.DPS {
			bestDPS[k] = s
		}
		if prev, ok := bestHPS[k]; !ok || s.HPS > prev.HPS {
			bestHPS[k] = s
		}
	}
	if err := rows.Err(); err != nil {
		return "", nil, 0, nil, err
	}

	byBoss := map[uint32]*BossPerformers{}
	getBoss := func(s sample) *BossPerformers {
		if p := byBoss[s.BossID]; p != nil {
			return p
		}
		meta, ok := latestBosses[s.BossID]
		if !ok {
			return nil
		}
		p := &BossPerformers{
			BossID:   s.BossID,
			BossName: meta.Name,
			RaidName: raidName,
			Position: int(meta.Position),
		}
		if p.BossName == "" {
			p.BossName = s.BossName
		}
		byBoss[s.BossID] = p
		return p
	}

	now := time.Now().UTC()
	toPerformer := func(s sample) Performer {
		return Performer{
			Name:      s.Name,
			RemoteID:  s.RemoteID,
			Spec:      int(s.Spec),
			SpecLabel: wow.SpecForExpansion(expansion, int(s.Spec)),
			Class:     int(s.Class),
			Mode:      int(s.Mode),
			ModeLabel: wow.Difficulty(expansion, int(s.Mode)),
			DPS:       int64(s.DPS),
			HPS:       int64(s.HPS),
			DmgDone:   int64(s.DmgDone),
			HealDone:  int64(s.HealDone),
			LengthSec: int(s.Length) / 1000,
			KilledAt:  humanizeAgo(now, s.KillTime),
			ItemLevel: float64(s.ILvl),
		}
	}

	for _, s := range bestDPS {
		if s.DPS > 0 {
			p := getBoss(s)
			if p != nil {
				p.TopDPS = append(p.TopDPS, toPerformer(s))
			}
		}
	}
	for _, s := range bestHPS {
		if s.HPS > 0 {
			p := getBoss(s)
			if p != nil {
				p.TopHPS = append(p.TopHPS, toPerformer(s))
			}
		}
	}

	out := make([]BossPerformers, 0, len(byBoss))
	for _, p := range byBoss {
		sort.Slice(p.TopDPS, func(i, j int) bool { return p.TopDPS[i].DPS > p.TopDPS[j].DPS })
		if len(p.TopDPS) > performerLimit {
			p.HasMore = true
			p.TopDPS = p.TopDPS[:performerLimit]
		}
		for i := range p.TopDPS {
			p.TopDPS[i].Rank = i + 1
		}
		sort.Slice(p.TopHPS, func(i, j int) bool { return p.TopHPS[i].HPS > p.TopHPS[j].HPS })
		if len(p.TopHPS) > performerLimit {
			p.HasMore = true
			p.TopHPS = p.TopHPS[:performerLimit]
		}
		for i := range p.TopHPS {
			p.TopHPS[i].Rank = i + 1
		}
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Position != out[j].Position {
			return out[i].Position < out[j].Position
		}
		if out[i].BossID != out[j].BossID {
			return out[i].BossID < out[j].BossID
		}
		return out[i].BossName < out[j].BossName
	})
	if len(out) > 0 {
		active := -1
		for i := range out {
			if out[i].BossID == selectedBossID {
				active = i
				break
			}
		}
		if active < 0 {
			active = 0
		}
		out[active].Active = true
	}
	return raidName, difficulties, selectedMode, out, nil
}

func leftPad2(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

func humanizeAgo(now, t time.Time) string {
	d := now.Sub(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		m := int(d / time.Minute)
		return strconv.Itoa(m) + " minutes ago"
	}
	if d < 24*time.Hour {
		h := int(d / time.Hour)
		if h == 1 {
			return "1 hour ago"
		}
		return strconv.Itoa(h) + " hours ago"
	}
	if d < 30*24*time.Hour {
		days := int(d / (24 * time.Hour))
		if days == 1 {
			return "1 day ago"
		}
		return strconv.Itoa(days) + " days ago"
	}
	if d < 365*24*time.Hour {
		months := int(d / (30 * 24 * time.Hour))
		if months == 1 {
			return "1 month ago"
		}
		return strconv.Itoa(months) + " months ago"
	}
	years := int(d / (365 * 24 * time.Hour))
	if years == 1 {
		return "1 year ago"
	}
	return strconv.Itoa(years) + " years ago"
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
