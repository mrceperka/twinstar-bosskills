package bosskill

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mrceperka/twinstar-bosskills/go/internal/cache"
	"github.com/mrceperka/twinstar-bosskills/go/internal/metric"
	"github.com/mrceperka/twinstar-bosskills/go/internal/realm"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/middleware"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/router"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/sqlutil"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/views/layouts"
	"github.com/mrceperka/twinstar-bosskills/go/internal/wow"
)

const (
	defaultStatsSort = "dps"
	defaultStatsDir  = "desc"
)

type Deps struct {
	DB      *sql.DB
	Items   *cache.ItemDisk
	CSSHash string
	JSHash  string
}

func Handler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		realmName := middleware.Realm(r.Context())
		if middleware.GatePrivateRealm(w, r) {
			return
		}
		remoteID := r.PathValue("id")
		if remoteID == "" {
			http.NotFound(w, r)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		expansion := realm.Expansion(realmName)

		info, players, loot, deaths, timelineJSON, found, err := loadKill(ctx, deps.DB, realmName, remoteID, expansion)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !found {
			http.NotFound(w, r)
			return
		}

		// Hydrate loot rows with item metadata.
		if deps.Items != nil && len(loot) > 0 {
			ids := make([]int, 0, len(loot))
			for _, l := range loot {
				ids = append(ids, int(l.ItemID))
			}
			ictx, icancel := context.WithTimeout(r.Context(), 4*time.Second)
			items := deps.Items.GetMany(ictx, ids)
			icancel()
			for i := range loot {
				if it, ok := items[int(loot[i].ItemID)]; ok {
					loot[i].Item = it
				}
			}
		}

		sortBy, sortDir := parseStatsSort(r.URL.Query())
		sortPlayers(players, sortBy, sortDir)

		vm := ViewModel{
			Meta: layouts.PageMeta{
				Title:      info.BossName + " kill — " + realmName,
				Realm:      realmName,
				CSSHash:    deps.CSSHash,
				JSHash:     deps.JSHash,
				NeedsChart: true,
			},
			Realm:        realmName,
			Kill:         info,
			Players:      players,
			Loot:         loot,
			Deaths:       deaths,
			TimelineJSON: timelineJSON,
			SortBy:       sortBy,
			SortDir:      sortDir,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if middleware.IsHTMX(r) {
			_ = StatsTableFragment(vm).Render(r.Context(), w)
			return
		}
		_ = Page(vm).Render(r.Context(), w)
	}
}

// parseStatsSort extracts ?sort= and ?dir= for the stats table.
func parseStatsSort(q map[string][]string) (string, string) {
	sortBy := defaultStatsSort
	if v, ok := q["sort"]; ok && len(v) > 0 {
		if _, valid := statsSortAccessors[v[0]]; valid {
			sortBy = v[0]
		}
	}
	dir := defaultStatsDir
	if v, ok := q["dir"]; ok && len(v) > 0 {
		if d := strings.ToLower(v[0]); d == "asc" || d == "desc" {
			dir = d
		}
	}
	return sortBy, dir
}

// statsSortAccessors maps a URL sort key to a comparator on PlayerRow.
// Returns the "greater" ordering for two rows (DESC direction). Callers
// invert for ASC.
var statsSortAccessors = map[string]func(a, b PlayerRow) bool{
	"name":         func(a, b PlayerRow) bool { return a.Name > b.Name },
	"dps":          func(a, b PlayerRow) bool { return a.DPS > b.DPS },
	"hps":          func(a, b PlayerRow) bool { return a.HPS > b.HPS },
	"dmg_done":     func(a, b PlayerRow) bool { return a.DmgDone > b.DmgDone },
	"dmg_taken":    func(a, b PlayerRow) bool { return a.DmgTaken > b.DmgTaken },
	"dmg_absorbed": func(a, b PlayerRow) bool { return a.DmgAbsorbed > b.DmgAbsorbed },
	"heal_done":    func(a, b PlayerRow) bool { return a.HealDone > b.HealDone },
	"abs_done":     func(a, b PlayerRow) bool { return a.AbsDone > b.AbsDone },
	"heal_taken":   func(a, b PlayerRow) bool { return a.HealTaken > b.HealTaken },
	"interrupts":   func(a, b PlayerRow) bool { return a.Interrupts > b.Interrupts },
	"dispels":      func(a, b PlayerRow) bool { return a.Dispels > b.Dispels },
	"ilvl":         func(a, b PlayerRow) bool { return a.ItemLvl > b.ItemLvl },
}

// sortPlayers reorders players in place. Rank stays tied to DPS (the "#"
// column already prints the DPS rank), so we don't renumber here.
func sortPlayers(players []PlayerRow, sortBy, dir string) {
	cmp := statsSortAccessors[sortBy]
	if cmp == nil {
		return
	}
	if dir == "asc" {
		less := cmp
		sort.SliceStable(players, func(i, j int) bool { return less(players[j], players[i]) })
	} else {
		sort.SliceStable(players, func(i, j int) bool { return cmp(players[i], players[j]) })
	}
}

// loadKill unpacks one row of boss_kill with all four Nested arrays.
func loadKill(ctx context.Context, db *sql.DB, realmName, remoteID string, expansion int) (
	info KillInfo, players []PlayerRow, loot []LootRow, deaths []DeathRow, timelineJSON []byte, found bool, err error,
) {
	const q = `
		SELECT
			boss_remote_id, boss_name, raid_name, mode, guild, kill_time,
			length, wipes, deaths, ress_used,

			players.guid, players.talent_spec, players.avg_item_lvl,
			players.dmg_done, players.healing_done, players.overhealing_done, players.absorb_done,
			players.dmg_taken, players.dmg_absorbed, players.healing_taken,
			players.dispels, players.interrupts,
			players.name, players.race, players.class, players.gender, players.level,

			loot.item_id, loot.count,
			deaths_detail.guid, deaths_detail.time,
			timeline.time, timeline.encounter_damage, timeline.encounter_heal,
			timeline.raid_damage, timeline.raid_heal
		FROM boss_kill
		WHERE realm = ? AND remote_id = ?
		ORDER BY version DESC
		LIMIT 1
	`
	row := db.QueryRowContext(ctx, q, realmName, remoteID)

	var (
		bossID   uint32
		bossName string
		raidName string
		mode     uint8
		guild    string
		killTime time.Time
		length   uint32
		wipes    uint32
		nDeaths  uint32
		ressUsed uint32

		pGuid, pDmgDone, pHealDone, pOverDone, pAbsDone, pDmgTaken, pDmgAbs, pHealTaken []uint64
		pSpec                                                                           []uint16
		pIlvl                                                                           []float32
		pDispels, pInterrupts                                                           []uint32
		pName                                                                           []string
		pRace, pClass, pGender, pLevel                                                  []uint8

		lootItemID []uint32
		lootCount  []uint8

		deathGuid  []uint64
		deathTime  []int32
		tlTime     []int32
		tlEncDmg   []uint64
		tlEncHeal  []uint64
		tlRaidDmg  []uint64
		tlRaidHeal []uint64
	)
	if err = row.Scan(
		&bossID, &bossName, &raidName, &mode, &guild, &killTime,
		&length, &wipes, &nDeaths, &ressUsed,

		&pGuid, &pSpec, &pIlvl,
		&pDmgDone, &pHealDone, &pOverDone, &pAbsDone,
		&pDmgTaken, &pDmgAbs, &pHealTaken,
		&pDispels, &pInterrupts,
		&pName, &pRace, &pClass, &pGender, &pLevel,

		&lootItemID, &lootCount,
		&deathGuid, &deathTime,
		&tlTime, &tlEncDmg, &tlEncHeal, &tlRaidDmg, &tlRaidHeal,
	); err != nil {
		if err == sql.ErrNoRows {
			return KillInfo{}, nil, nil, nil, nil, false, nil
		}
		return KillInfo{}, nil, nil, nil, nil, false, err
	}

	avgIlvl := 0.0
	if len(pIlvl) > 0 {
		var sum float64
		for _, v := range pIlvl {
			sum += float64(v)
		}
		avgIlvl = sum / float64(len(pIlvl))
	}

	var totalDmg, totalHeal, totalDmgTaken int64
	for _, v := range pDmgDone {
		totalDmg += int64(v)
	}
	var totalAbs int64
	for _, v := range pHealDone {
		totalHeal += int64(v)
	}
	for _, v := range pAbsDone {
		totalAbs += int64(v)
	}
	for _, v := range pDmgTaken {
		totalDmgTaken += int64(v)
	}

	// Count actual ressurects (time < 0 in deaths_detail.time) vs deaths
	var ressCount int
	for _, t := range deathTime {
		if t < 0 {
			ressCount++
		}
	}
	actualDeaths := len(deathTime) - ressCount

	info = KillInfo{
		RemoteID:      remoteID,
		BossID:        bossID,
		BossName:      bossName,
		RaidName:      raidName,
		Mode:          int(mode),
		ModeLabel:     wow.Difficulty(expansion, int(mode)),
		Guild:         guild,
		KillTime:      killTime.Format("01/02/2006, 3:04 PM"),
		LengthSec:     int(length) / 1000,
		Wipes:         int(wipes),
		Deaths:        actualDeaths,
		Ressurects:    ressCount,
		RessUsed:      int(ressUsed),
		AvgIlvl:       avgIlvl,
		TotalDmgDone:  totalDmg,
		TotalHealDone: totalHeal + totalAbs,
		TotalDmgTaken: totalDmgTaken,
		RaidDPS:       metric.DPS(totalDmg, int64(length)),
		RaidHPS:       metric.HPS(totalHeal, totalAbs, int64(length)),
	}
	_ = nDeaths // upstream's `deaths` field may include ress; we use the more accurate split

	// Dead-marker for player rows.
	deadGuids := map[uint64]bool{}
	for i, g := range deathGuid {
		if deathTime[i] >= 0 {
			deadGuids[g] = true
		}
	}

	players = make([]PlayerRow, 0, len(pGuid))
	for i := range pGuid {
		dps := metric.DPS(int64(pDmgDone[i]), int64(length))
		hps := metric.HPS(int64(pHealDone[i]), int64(pAbsDone[i]), int64(length))
		players = append(players, PlayerRow{
			Name:          pName[i],
			Guid:          pGuid[i],
			Class:         int(pClass[i]),
			ClassLabel:    wow.Class(int(pClass[i])),
			Race:          int(pRace[i]),
			Spec:          int(pSpec[i]),
			SpecLabel:     wow.SpecForExpansion(expansion, int(pSpec[i])),
			ItemLvl:       float64(pIlvl[i]),
			DPS:           dps,
			HPS:           hps,
			DPSPercentile: -1,
			HPSPercentile: -1,
			DmgDone:       int64(pDmgDone[i]),
			DmgTaken:      int64(pDmgTaken[i]),
			DmgAbsorbed:   int64(pDmgAbs[i]),
			HealDone:      int64(pHealDone[i]),
			AbsDone:       int64(pAbsDone[i]),
			HealTaken:     int64(pHealTaken[i]),
			Overheal:      int64(pOverDone[i]),
			Interrupts:    int(pInterrupts[i]),
			Dispels:       int(pDispels[i]),
			IsDead:        deadGuids[pGuid[i]],
		})
	}
	sort.Slice(players, func(i, j int) bool {
		if players[i].DPS != players[j].DPS {
			return players[i].DPS > players[j].DPS
		}
		return players[i].HPS > players[j].HPS
	})
	for i := range players {
		players[i].Rank = i + 1
	}

	// Compute per-player DPS / HPS percentile rank within the same boss + mode
	// + spec population. Mirrors SvelteKit's getBossPercentilesPerPlayer but
	// done synchronously inside this handler since the page already aggregates
	// all the per-player data on render.
	if err := fillPercentiles(ctx, db, realmName, bossID, mode, players); err != nil {
		// Non-fatal: log via err return only when query setup fails. For
		// transient row errors we leave Percentile == -1 which the templ
		// renders as "N/A".
		return KillInfo{}, nil, nil, nil, nil, false, err
	}

	itemIDs := make([]uint32, 0, len(lootItemID))
	for _, id := range lootItemID {
		itemIDs = append(itemIDs, id)
	}
	lootChances, err := loadLootChances(ctx, db, realmName, bossID, mode, itemIDs)
	if err != nil {
		return KillInfo{}, nil, nil, nil, nil, false, err
	}

	loot = make([]LootRow, 0, len(lootItemID))
	for i, id := range lootItemID {
		count := 1
		if i < len(lootCount) && lootCount[i] > 0 {
			count = int(lootCount[i])
		}
		row := LootRow{ItemID: id, Count: count, DropChanceLabel: lootChances[id]}
		loot = append(loot, row)
	}

	// Deaths timeline.
	nameByGuid := map[uint64]string{}
	for i, g := range pGuid {
		nameByGuid[g] = pName[i]
	}
	deaths = make([]DeathRow, 0, len(deathGuid))
	for i, g := range deathGuid {
		t := int(deathTime[i])
		// deaths_detail.time is stored in seconds (same unit as timeline.time).
		// Negative values indicate a resurrection.
		deaths = append(deaths, DeathRow{
			TimeSec: t,
			Name:    nameByGuid[g],
			IsRess:  t < 0,
		})
	}
	sort.Slice(deaths, func(i, j int) bool { return absInt(deaths[i].TimeSec) < absInt(deaths[j].TimeSec) })

	// Fight timeline echarts JSON.
	timelineJSON, _ = buildTimelineJSON(tlTime, tlEncDmg, tlEncHeal, tlRaidDmg, tlRaidHeal, deaths)
	return info, players, loot, deaths, timelineJSON, true, nil
}

func loadLootChances(ctx context.Context, db *sql.DB, realmName string, bossID uint32, mode uint8, itemIDs []uint32) (map[uint32]string, error) {
	out := map[uint32]string{}
	if len(itemIDs) == 0 {
		return out, nil
	}

	const totalQ = `
		SELECT count()
		FROM boss_kill
		WHERE realm = ?
		  AND boss_remote_id = ?
		  AND mode = ?
	`
	var total uint64
	if err := db.QueryRowContext(ctx, totalQ, realmName, bossID, mode).Scan(&total); err != nil {
		return nil, err
	}
	if total == 0 {
		return out, nil
	}

	seen := map[uint32]bool{}
	unique := make([]uint32, 0, len(itemIDs))
	for _, id := range itemIDs {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}

	ph := sqlutil.Placeholders(len(unique))
	args := make([]any, 0, len(unique)+3)
	args = append(args, realmName, bossID, mode)
	for _, id := range unique {
		args = append(args, id)
	}
	q := `
		SELECT loot.item_id, uniqExact(remote_id)
		FROM boss_kill
		ARRAY JOIN loot
		WHERE realm = ?
		  AND boss_remote_id = ?
		  AND mode = ?
		  AND loot.item_id IN (` + ph + `)
		GROUP BY loot.item_id
	`
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var itemID uint32
		var count uint64
		if err := rows.Scan(&itemID, &count); err != nil {
			return nil, err
		}
		pct := float64(count) * 100.0 / float64(total)
		out[itemID] = strconv.FormatUint(count, 10) + " of " +
			strconv.FormatUint(total, 10) + " ~ " +
			strconv.FormatFloat(pct, 'f', 2, 64) + "%"
	}
	return out, rows.Err()
}

// fillPercentiles fills players[i].DPSPercentile / HPSPercentile with the
// per-spec percentile rank computed against the full sample population for
// this boss + mode. Players with no spec data or no peer samples keep -1
// (rendered as "N/A").
//
// We pull only the rows whose spec appears in the current kill so the query
// stays bounded; for the typical 25-player roster that's at most ~25 IN
// values, and ClickHouse scans the ARRAY JOIN efficiently with a single pass.
func fillPercentiles(ctx context.Context, db *sql.DB, realmName string, bossID uint32, mode uint8, players []PlayerRow) error {
	specSet := map[int]bool{}
	for _, p := range players {
		if p.Spec > 0 {
			specSet[p.Spec] = true
		}
	}
	if len(specSet) == 0 {
		return nil
	}
	specs := make([]int, 0, len(specSet))
	for s := range specSet {
		specs = append(specs, s)
	}

	args := []any{realmName, bossID, mode}
	placeholders := ""
	for i, s := range specs {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, uint16(s))
	}
	q := `
		SELECT
			players.talent_spec AS spec,
			` + metric.SQLUInt64(metric.DmgDoneArrayJoin) + ` AS dps,
			` + metric.SQLUInt64(metric.HealAbsorbArrayJoin) + ` AS hps
		FROM boss_kill ARRAY JOIN players
		WHERE realm = ? AND boss_remote_id = ? AND mode = ? AND length > 0
		  AND players.talent_spec IN (` + placeholders + `)
	`
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	dpsBySpec := map[int][]uint64{}
	hpsBySpec := map[int][]uint64{}
	for rows.Next() {
		var spec uint16
		var dps, hps uint64
		if err := rows.Scan(&spec, &dps, &hps); err != nil {
			return err
		}
		dpsBySpec[int(spec)] = append(dpsBySpec[int(spec)], dps)
		hpsBySpec[int(spec)] = append(hpsBySpec[int(spec)], hps)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for spec := range dpsBySpec {
		sort.Slice(dpsBySpec[spec], func(i, j int) bool { return dpsBySpec[spec][i] < dpsBySpec[spec][j] })
		sort.Slice(hpsBySpec[spec], func(i, j int) bool { return hpsBySpec[spec][i] < hpsBySpec[spec][j] })
	}

	for i := range players {
		if players[i].Spec == 0 {
			continue
		}
		dpsArr := dpsBySpec[players[i].Spec]
		hpsArr := hpsBySpec[players[i].Spec]
		if len(dpsArr) > 0 {
			players[i].DPSPercentile = percentileRank(dpsArr, uint64(players[i].DPS))
		}
		if len(hpsArr) > 0 {
			players[i].HPSPercentile = percentileRank(hpsArr, uint64(players[i].HPS))
		}
	}
	return nil
}

// percentileRank returns the percent of samples in `sorted` that are strictly
// less than `v` (so a player tied with the top sample gets 99-ish, not 100).
// Sorted must be ascending.
func percentileRank(sorted []uint64, v uint64) float64 {
	if len(sorted) == 0 {
		return -1
	}
	n := sort.Search(len(sorted), func(i int) bool { return sorted[i] >= v })
	return float64(n) * 100.0 / float64(len(sorted))
}

// buildTimelineJSON renders a multi-series line chart matching the existing
// SvelteKit "Fight timeline" panel.
//
// Series:
//   - Raid Damage     (yellow)
//   - Enemy Damage    (red)
//   - Raid Healing    (green)
//   - Enemy Healing   (blue)
//   - Deaths          (markers from the deaths slice)
//   - Ressurects      (markers from the deaths slice, time < 0)
//
// X axis: timeline categories in seconds since pull start. Y axis: per-second damage / healing.
func buildTimelineJSON(tlTime []int32, encDmg, encHeal, raidDmg, raidHeal []uint64, deaths []DeathRow) ([]byte, error) {
	n := min(len(tlTime), len(encDmg), len(encHeal), len(raidDmg), len(raidHeal))
	if n == 0 {
		return []byte("{}"), nil
	}

	type timelineSample struct {
		time     int
		encDmg   uint64
		encHeal  uint64
		raidDmg  uint64
		raidHeal uint64
	}

	samples := make([]timelineSample, n)
	for i := 0; i < n; i++ {
		samples[i] = timelineSample{
			time:     int(tlTime[i]),
			encDmg:   encDmg[i],
			encHeal:  encHeal[i],
			raidDmg:  raidDmg[i],
			raidHeal: raidHeal[i],
		}
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i].time < samples[j].time })

	xAxisData := make([]int, n)
	encDmgData := make([]uint64, n)
	encHealData := make([]uint64, n)
	raidDmgData := make([]uint64, n)
	raidHealData := make([]uint64, n)
	for i, sample := range samples {
		xAxisData[i] = sample.time
		encDmgData[i] = sample.encDmg
		encHealData[i] = sample.encHeal
		raidDmgData[i] = sample.raidDmg
		raidHealData[i] = sample.raidHeal
	}

	// Deaths/ress scatter on secondary (hidden) y-axes so they appear at the
	// top of the chart regardless of the main y-axis scale. Each point carries
	// the player name so ECharts can render it as a label.
	type namedPt struct {
		Value [2]any `json:"value"`
		Name  string `json:"name"`
	}
	deathPts := make([]namedPt, 0)
	ressPts := make([]namedPt, 0)
	for _, d := range deaths {
		// Ress entries are stored with negative time in the DB; use abs so the
		// marker appears at the correct position on the x-axis.
		t := absInt(d.TimeSec)
		pt := namedPt{Value: [2]any{t, 1}, Name: d.Name}
		if d.IsRess {
			ressPts = append(ressPts, pt)
		} else {
			deathPts = append(deathPts, pt)
		}
	}

	axisStyle := map[string]any{
		"axisLabel": map[string]any{"color": "#c5c5c5"},
		"axisLine":  map[string]any{"lineStyle": map[string]any{"color": "#c5c5c5"}},
		"axisTick":  map[string]any{"lineStyle": map[string]any{"color": "#c5c5c5"}},
		"splitLine": map[string]any{"lineStyle": map[string]any{"color": "rgba(255,255,255,0.07)"}},
	}

	labelStyle := func(color string) map[string]any {
		return map[string]any{
			"show":      true,
			"formatter": "{b}",
			"color":     color,
			"position":  "top",
			"rotate":    60,
		}
	}

	opt := map[string]any{
		"backgroundColor": "transparent",
		"animation":       false,
		"tooltip":         map[string]any{"trigger": "axis"},
		"legend": map[string]any{
			"top":       "top",
			"textStyle": map[string]any{"color": "#c5c5c5"},
			"data":      []string{"Enemy Healing", "Enemy Damage", "Raid Damage", "Raid Healing", "Deaths", "Ressurects"},
		},
		"grid": map[string]any{
			"left":         "1%",
			"right":        "0%",
			"bottom":       "2%",
			"containLabel": true,
		},
		"xAxis": map[string]any{
			"type":        "category",
			"boundaryGap": false,
			"data":        xAxisData,
			"axisLabel":   map[string]any{"color": "#c5c5c5"},
			"axisLine":    map[string]any{"lineStyle": map[string]any{"color": "#c5c5c5"}},
			"axisTick":    map[string]any{"lineStyle": map[string]any{"color": "#c5c5c5"}},
			"splitLine":   map[string]any{"lineStyle": map[string]any{"color": "rgba(255,255,255,0.07)"}},
		},
		// Three y-axes: [0] main data, [1] deaths (hidden 0–1), [2] ress (hidden 0–1)
		"yAxis": []any{
			map[string]any{
				"type":      "value",
				"axisLabel": axisStyle["axisLabel"],
				"axisLine":  axisStyle["axisLine"],
				"axisTick":  axisStyle["axisTick"],
				"splitLine": axisStyle["splitLine"],
			},
			map[string]any{"type": "value", "show": false, "min": 0, "max": 1},
			map[string]any{"type": "value", "show": false, "min": 0, "max": 1},
		},
		"series": []any{
			map[string]any{"name": "Enemy Healing", "type": "line", "data": encHealData, "smooth": true, "showSymbol": false, "lineStyle": map[string]any{"color": "#68ccef"}, "color": "#68ccef"},
			map[string]any{"name": "Enemy Damage", "type": "line", "data": encDmgData, "smooth": true, "showSymbol": false, "lineStyle": map[string]any{"color": "#ff4040"}, "color": "#ff4040"},
			map[string]any{"name": "Raid Damage", "type": "line", "data": raidDmgData, "smooth": true, "showSymbol": false, "lineStyle": map[string]any{"color": "#ffd100"}, "color": "#ffd100"},
			map[string]any{"name": "Raid Healing", "type": "line", "data": raidHealData, "smooth": true, "showSymbol": false, "lineStyle": map[string]any{"color": "#1eff00"}, "color": "#1eff00"},
			map[string]any{
				"name": "Deaths", "type": "scatter", "yAxisIndex": 1,
				"data": deathPts, "symbolSize": 10,
				"itemStyle": map[string]any{"color": "#ffffff"},
				"label":     labelStyle("#ffffff"),
				"color":     "#ffffff",
			},
			map[string]any{
				"name": "Ressurects", "type": "scatter", "yAxisIndex": 2,
				"data": ressPts, "symbolSize": 10,
				"itemStyle": map[string]any{"color": "#ff69b4"},
				"label":     labelStyle("#ff69b4"),
				"color":     "#ff69b4",
			},
		},
	}
	return json.Marshal(opt)
}

func Mount(mux *http.ServeMux, deps Deps) {
	h := middleware.RequireRealm(Handler(deps))
	router.ForEachRealmPrefix(func(prefix string) {
		mux.Handle("GET "+prefix+"/boss-kills/{id}", h)
	})
}
