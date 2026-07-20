package boss

import (
	"context"
	"database/sql"
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
	"twinstar-bosskills/internal/web/views/layouts"
	"twinstar-bosskills/internal/wow"
)

type Deps struct {
	DB      *sql.DB
	CSSHash string
	JSHash  string
}

const (
	topRowLimit = 25
)

type lockFilter struct {
	Enabled bool
	Start   time.Time
	End     time.Time
}

func Handler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		realmName := middleware.Realm(r.Context())

		idStr := r.PathValue("id")
		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		bossID := uint32(id)

		ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
		defer cancel()

		expansion := realm.Expansion(realmName)

		bossInfo, err := loadBossInfo(ctx, deps.DB, realmName, bossID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if bossInfo.Name == "" {
			http.NotFound(w, r)
			return
		}

		avail, err := loadAvailableModes(ctx, deps.DB, realmName, bossID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		requestedMode, hasRequestedMode := query.Difficulty(r.URL.Query())
		mode := selectMode(requestedMode, hasRequestedMode, avail, expansion)
		lockOffset := 0
		lock := lockFilter{}
		if v, ok := query.RaidLock(r.URL.Query()); ok {
			lockOffset = v
			window := domain.RaidLock(time.Now().UTC(), lockOffset)
			lock = lockFilter{Enabled: true, Start: window.Start, End: window.End}
		}

		// Optional spec / class filters (?spec=N, ?class=N). Zero means
		// "no filter".
		spec, _ := strconv.Atoi(r.URL.Query().Get("spec"))
		class, _ := strconv.Atoi(r.URL.Query().Get("class"))
		if expansionIsClassMode(expansion) {
			spec = 0
		} else {
			class = 0
		}

		stats, err := loadHeaderStats(ctx, deps.DB, realmName, bossID, mode, expansion, lock)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		siblings, err := loadSiblings(ctx, deps.DB, realmName, bossInfo.RaidName, bossID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		dpsCurves, hpsCurves, err := loadSpecCurves(ctx, deps.DB, realmName, bossID, mode, spec, class, expansion, lock)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		selectedP := 50
		if v, err := strconv.Atoi(r.URL.Query().Get("p")); err == nil && v >= 1 && v <= 99 {
			selectedP = v
		}
		groupLabel := "spec"
		if expansionIsClassMode(expansion) {
			groupLabel = "class"
		}
		dpsCurveJSON, _ := buildCurveJSON("DPS by "+groupLabel, expansion, dpsCurves, selectedP)
		hpsCurveJSON, _ := buildCurveJSON("HPS by "+groupLabel, expansion, hpsCurves, selectedP)
		dpsBoxJSON, _ := buildBoxPlotJSON("DPS", dpsCurves, realmName)
		hpsBoxJSON, _ := buildBoxPlotJSON("HPS", hpsCurves, realmName)
		atP := extractAtPercentile(dpsCurves, hpsCurves, selectedP, expansion)

		topDPS, topHPS, err := loadRankings(ctx, deps.DB, realmName, bossID, mode, spec, class, expansion, lock)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		diffChoices := make([]DifficultyChoice, 0, len(avail))
		for _, m := range avail {
			diffChoices = append(diffChoices, DifficultyChoice{Mode: m, Label: wow.Difficulty(expansion, m)})
		}

		// Collect available spec / class IDs for the filter row.
		availSpecs, availClasses, err := loadAvailableSpecsClasses(ctx, deps.DB, realmName, bossID, mode, expansion, lock)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		vm := ViewModel{
			Meta: layouts.PageMeta{
				Title:      bossInfo.Name + " - " + realmName,
				Realm:      realmName,
				CSSHash:    deps.CSSHash,
				JSHash:     deps.JSHash,
				NeedsChart: true,
			},
			Realm:            realmName,
			Boss:             bossInfo,
			Siblings:         siblings,
			Stats:            stats,
			Difficulties:     diffChoices,
			SelectedMode:     mode,
			LockLabel:        lockLabel(lock, lockOffset),
			LockOffset:       lockOffset,
			LockScoped:       lock.Enabled,
			SelectedSpec:     spec,
			SelectedClass:    class,
			ClassMode:        expansionIsClassMode(expansion),
			AvailableSpecs:   availSpecs,
			AvailableClasses: availClasses,
			SelectedPctile:   selectedP,
			DPSCurveJSON:     dpsCurveJSON,
			HPSCurveJSON:     hpsCurveJSON,
			DPSBoxJSON:       dpsBoxJSON,
			HPSBoxJSON:       hpsBoxJSON,
			AtPercentile:     atP,
			TopDPS:           topDPS,
			TopHPS:           topHPS,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if middleware.IsHTMX(r) {
			_ = Fragment(vm).Render(r.Context(), w)
			return
		}
		_ = Page(vm).Render(r.Context(), w)
	}
}

func expansionIsClassMode(expansion int) bool {
	return expansion == realm.ExpansionVanilla
}

func loadBossInfo(ctx context.Context, db *sql.DB, realmName string, id uint32) (BossInfo, error) {
	const lookupQ = `
		SELECT name, raid_name
		FROM boss FINAL
		WHERE realm = ? AND remote_id = ?
		LIMIT 1
	`
	var name, raidName string
	if err := db.QueryRowContext(ctx, lookupQ, realmName, id).Scan(&name, &raidName); err == nil {
		return BossInfo{RemoteID: id, Name: name, RaidName: raidName}, nil
	} else if err != sql.ErrNoRows {
		return BossInfo{}, err
	}

	const eventQ = `
		SELECT any(boss_name) AS boss_name, any(raid_name) AS raid_name
		FROM boss_kill
		WHERE realm = ? AND boss_remote_id = ?
	`
	if err := db.QueryRowContext(ctx, eventQ, realmName, id).Scan(&name, &raidName); err != nil {
		if err == sql.ErrNoRows {
			return BossInfo{}, nil
		}
		return BossInfo{}, err
	}
	return BossInfo{RemoteID: id, Name: name, RaidName: raidName}, nil
}

func loadAvailableModes(ctx context.Context, db *sql.DB, realmName string, id uint32) ([]int, error) {
	const q = `
		SELECT DISTINCT mode FROM boss_kill
		WHERE realm = ? AND boss_remote_id = ?
		ORDER BY mode
	`
	rows, err := db.QueryContext(ctx, q, realmName, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int
	for rows.Next() {
		var m uint8
		if err := rows.Scan(&m); err != nil {
			return nil, err
		}
		out = append(out, int(m))
	}
	return out, rows.Err()
}

func selectMode(requested int, hasRequested bool, available []int, expansion int) int {
	if hasRequested {
		for _, m := range available {
			if m == requested {
				return requested
			}
		}
	}
	def := wow.DefaultDifficulty(expansion)
	for _, m := range available {
		if m == def {
			return def
		}
	}
	if len(available) > 0 {
		return available[0]
	}
	return def
}

func lockLabel(lock lockFilter, _ int) string {
	if !lock.Enabled {
		return "Overall"
	}
	return lock.Start.Format("Jan 2 15:04 UTC") + " → " + lock.End.Format("Jan 2 15:04 UTC")
}

func applyLockFilter(q string, args []any, lock lockFilter) (string, []any) {
	if !lock.Enabled {
		return q, args
	}
	q += " AND kill_time >= ? AND kill_time < ?"
	args = append(args, lock.Start, lock.End)
	return q, args
}

// loadAvailableSpecsClasses returns the distinct spec / class IDs that have
// data for the requested boss + mode, so the filter row can render only
// icons that actually do something.
func loadAvailableSpecsClasses(ctx context.Context, db *sql.DB, realmName string, id uint32, mode, expansion int, lock lockFilter) (specs, classes []int, err error) {
	if expansionIsClassMode(expansion) {
		q := `
			SELECT DISTINCT players.class AS c
			FROM boss_kill ARRAY JOIN players
			WHERE realm = ? AND boss_remote_id = ? AND mode = ?
			  AND players.class > 0
		`
		args := []any{realmName, id, uint8(mode)}
		q, args = applyLockFilter(q, args, lock)
		rows, err := db.QueryContext(ctx, q, args...)
		if err != nil {
			return nil, nil, err
		}
		defer rows.Close()
		classSet := map[int]bool{}
		for rows.Next() {
			var c uint8
			if err := rows.Scan(&c); err != nil {
				return nil, nil, err
			}
			classSet[int(c)] = true
		}
		for c := range classSet {
			classes = append(classes, c)
		}
		sort.Ints(classes)
		return nil, classes, rows.Err()
	}

	q := `
		SELECT DISTINCT players.talent_spec AS s, players.class AS c
		FROM boss_kill ARRAY JOIN players
		WHERE realm = ? AND boss_remote_id = ? AND mode = ?
		  AND players.talent_spec > 0
	`
	args := []any{realmName, id, uint8(mode)}
	q, args = applyLockFilter(q, args, lock)
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	specSet := map[int]bool{}
	for rows.Next() {
		var s uint16
		var c uint8
		if err := rows.Scan(&s, &c); err != nil {
			return nil, nil, err
		}
		specSet[int(s)] = true
	}
	for s := range specSet {
		specs = append(specs, s)
	}
	sort.Ints(specs)
	return specs, nil, rows.Err()
}

// loadHeaderStats computes the narrative numbers shown above the tabs:
// total kills, wipes, kill/wipe chance, and fight length percentiles.
func loadHeaderStats(ctx context.Context, db *sql.DB, realmName string, id uint32, mode, expansion int, lock lockFilter) (HeaderStats, error) {
	stats := HeaderStats{ModeLabel: wow.Difficulty(expansion, mode)}

	q := `
		SELECT count() AS kills,
		       sum(wipes) AS wipes,
		       minIf(length, length > 0) AS min_len,
		       avg(length) AS avg_len,
		       max(length) AS max_len
		FROM boss_kill
		WHERE realm = ? AND boss_remote_id = ? AND mode = ?
	`
	args := []any{realmName, id, uint8(mode)}
	q, args = applyLockFilter(q, args, lock)
	var (
		kills, wipes uint64
		minLen       uint64
		avgLen       float64
		maxLen       uint64
	)
	if err := db.QueryRowContext(ctx, q, args...).
		Scan(&kills, &wipes, &minLen, &avgLen, &maxLen); err != nil {
		if err == sql.ErrNoRows {
			return stats, nil
		}
		return stats, err
	}
	stats.Kills = int(kills)
	stats.Wipes = int(wipes)
	if kills > 0 {
		stats.AvgWipes = float64(wipes) / float64(kills)
	}
	if kills+wipes > 0 {
		stats.KillChance = 100 * float64(kills) / float64(kills+wipes)
		stats.WipeChance = 100 - stats.KillChance
	}
	stats.FastestSec = int(minLen) / 1000
	stats.AverageSec = int(avgLen) / 1000
	stats.SlowestSec = int(maxLen) / 1000
	return stats, nil
}

// loadSiblings returns the other bosses in the same raid for the inline nav,
// sorted by encounter order.
func loadSiblings(ctx context.Context, db *sql.DB, realmName, raidName string, currentBossID uint32) ([]SiblingBoss, error) {
	if raidName == "" {
		return nil, nil
	}
	const q = `
		SELECT remote_id, name
		FROM boss FINAL
		WHERE realm = ? AND raid_name = ?
		ORDER BY position ASC, remote_id ASC
	`
	rows, err := db.QueryContext(ctx, q, realmName, raidName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SiblingBoss
	for rows.Next() {
		var id uint32
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out = append(out, SiblingBoss{RemoteID: id, Name: name, Current: id == currentBossID})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// loadRankings returns the top DPS / top HPS rows enriched with the same
// columns the SvelteKit page shows: dmg/heal totals, fight length, kill
// time, ilvl, and the per-kill remote_id for the Detail link.
//
// `specFilter` and `classFilter` are 0 for "no filter" - non-zero values
// are applied as additional predicates on players.talent_spec / players.class.
// Vanilla ranks by class, later expansions rank by spec.
func loadRankings(ctx context.Context, db *sql.DB, realmName string, id uint32, mode, specFilter, classFilter, expansion int, lock lockFilter) (dps, hps []Ranking, err error) {
	q := `
		SELECT
			players.guid                                    AS guid,
			players.talent_spec                             AS spec,
			players.name                                    AS name,
			players.class                                   AS class,
			` + metric.SQLUInt64(metric.DmgDoneArrayJoin) + ` AS dps,
			` + metric.SQLUInt64(metric.HealAbsorbArrayJoin) + ` AS hps,
			toUInt64(players.dmg_done)                       AS dmg_done,
			toUInt64(players.healing_done + players.absorb_done) AS heal_done,
			length                                           AS len,
			kill_time                                        AS kill_time,
			players.avg_item_lvl                             AS ilvl,
			remote_id                                        AS remote_id
		FROM boss_kill ARRAY JOIN players
		WHERE realm = ? AND boss_remote_id = ? AND mode = ? AND length > 0
	`
	args := []any{realmName, id, uint8(mode)}
	q, args = applyLockFilter(q, args, lock)
	if !expansionIsClassMode(expansion) && specFilter > 0 {
		q += " AND players.talent_spec = ?"
		args = append(args, uint16(specFilter))
	}
	if expansionIsClassMode(expansion) && classFilter > 0 {
		q += " AND players.class = ?"
		args = append(args, uint8(classFilter))
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	type sample struct {
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
	var all []sample
	for rows.Next() {
		var s sample
		if err := rows.Scan(&s.Guid, &s.Spec, &s.Name, &s.Class, &s.DPS, &s.HPS,
			&s.DmgDone, &s.HealDone, &s.Length, &s.KillTime, &s.ILvl, &s.RemoteID); err != nil {
			return nil, nil, err
		}
		all = append(all, s)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	// Pick best DPS / best HPS per character group.
	type key struct {
		Guid  uint64
		Spec  uint16
		Class uint8
	}
	bestDPS := map[key]sample{}
	bestHPS := map[key]sample{}
	for _, s := range all {
		k := key{Guid: s.Guid, Spec: s.Spec}
		if expansionIsClassMode(expansion) {
			k = key{Guid: s.Guid, Class: s.Class}
		}
		if prev, ok := bestDPS[k]; !ok || s.DPS > prev.DPS {
			bestDPS[k] = s
		}
		if prev, ok := bestHPS[k]; !ok || s.HPS > prev.HPS {
			bestHPS[k] = s
		}
	}

	now := time.Now().UTC()
	to := func(s sample, isDPS bool) Ranking {
		return Ranking{
			Name:       s.Name,
			RemoteID:   s.RemoteID,
			Spec:       int(s.Spec),
			SpecLabel:  wow.SpecForExpansion(expansion, int(s.Spec)),
			Class:      int(s.Class),
			ClassLabel: wow.Class(int(s.Class)),
			DPS:        int64(s.DPS),
			HPS:        int64(s.HPS),
			DmgDone:    int64(s.DmgDone),
			HealDone:   int64(s.HealDone),
			LengthSec:  int(s.Length) / 1000,
			KilledAt:   humanizeAgo(now, s.KillTime),
			ItemLevel:  float64(s.ILvl),
		}
	}

	// DPS leaderboard
	for _, s := range bestDPS {
		if s.DPS > 0 {
			dps = append(dps, to(s, true))
		}
	}
	sort.Slice(dps, func(i, j int) bool { return dps[i].DPS > dps[j].DPS })
	for i := range dps {
		dps[i].Rank = i + 1
	}
	if len(dps) > topRowLimit {
		dps = dps[:topRowLimit]
	}

	// HPS leaderboard
	for _, s := range bestHPS {
		if s.HPS > 0 {
			hps = append(hps, to(s, false))
		}
	}
	sort.Slice(hps, func(i, j int) bool { return hps[i].HPS > hps[j].HPS })
	for i := range hps {
		hps[i].Rank = i + 1
	}
	if len(hps) > topRowLimit {
		hps = hps[:topRowLimit]
	}
	return dps, hps, nil
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

func Mount(mux *http.ServeMux, deps Deps) {
	h := middleware.RequireRealm(Handler(deps))
	router.ForEachRealmPrefix(func(prefix string) {
		mux.Handle("GET "+prefix+"/boss/{id}", h)
		mux.Handle("GET "+prefix+"/boss/{id}/history", h)
	})
}
