package characterperf

import (
	"context"
	"database/sql"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"twinstar-bosskills/internal/collection"
	"twinstar-bosskills/internal/metric"
	"twinstar-bosskills/internal/realm"
	"twinstar-bosskills/internal/web/middleware"
	"twinstar-bosskills/internal/web/query"
	"twinstar-bosskills/internal/web/repository"
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

const sampleLimit = 200

func Handler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		realmName := middleware.Realm(r.Context())
		if middleware.GatePrivateRealm(w, r) {
			return
		}
		name := r.PathValue("name")
		if name == "" {
			http.NotFound(w, r)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
		defer cancel()

		expansion := realm.Expansion(realmName)

		guid, class, killCount, lastSeen, err := lookupCharacter(ctx, deps.DB, realmName, name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if guid == 0 {
			http.NotFound(w, r)
			return
		}

		q := r.URL.Query()
		filter := parseFilter(q)

		samples, err := loadSamples(ctx, deps.DB, realmName, guid, filter)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Group samples by (bossID, mode), preserving insertion order.
		type groupKey struct {
			BossID uint32
			Mode   int
		}
		byGroup := map[groupKey][]Sample{}
		var groupOrder []groupKey
		bossIDsSet := map[uint32]bool{}
		for _, s := range samples {
			k := groupKey{s.BossID, s.Mode}
			if _, ok := byGroup[k]; !ok {
				groupOrder = append(groupOrder, k)
			}
			byGroup[k] = append(byGroup[k], s)
			bossIDsSet[s.BossID] = true
		}
		// Drop groups with only one data point (no trend to show).
		for k, v := range byGroup {
			if len(v) <= 1 {
				delete(byGroup, k)
			}
		}

		medians, err := loadMedianByBoss(ctx, deps.DB, realmName, collection.Keys(bossIDsSet), filter)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		bossPositions, err := loadBossPositions(ctx, deps.DB, realmName, collection.Keys(bossIDsSet))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Sort groups by encounter order, then mode.
		sort.Slice(groupOrder, func(i, j int) bool {
			a, b := groupOrder[i], groupOrder[j]
			oa := bossPositions[a.BossID]
			ob := bossPositions[b.BossID]
			if oa != ob {
				return oa < ob
			}
			if a.BossID != b.BossID {
				return a.BossID < b.BossID
			}
			return a.Mode < b.Mode
		})

		var charts []BossChart
		for _, k := range groupOrder {
			samps, ok := byGroup[k]
			if !ok {
				continue
			}
			var dpsMedian, hpsMedian int64
			if mp, ok := medians[k.BossID][k.Mode]; ok {
				dpsMedian = mp.DPS
				hpsMedian = mp.HPS
			}
			chartJSON, _ := buildBossChart(realmName, samps, dpsMedian, hpsMedian)
			charts = append(charts, BossChart{
				BossID:    k.BossID,
				BossName:  samps[0].BossName,
				Mode:      k.Mode,
				ModeLabel: wow.Difficulty(expansion, k.Mode),
				ChartJSON: chartJSON,
			})
		}

		bossOpts, err := loadBossOptions(ctx, deps.DB, realmName, guid)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		markBossSelected(bossOpts, filter.Bosses)
		raidOpts, err := loadRaidOptions(ctx, deps.DB, realmName, guid)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		markStringSelected(raidOpts, filter.Raids)
		specOpts, err := loadSpecOptions(ctx, deps.DB, realmName, guid, expansion)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		markIntSelected(specOpts, filter.Specs)
		modeOpts := buildModeOptions(filter.Modes, expansion)

		vm := ViewModel{
			Meta: layouts.PageMeta{
				Title:      name + " performance - " + realmName,
				Realm:      realmName,
				CSSHash:    deps.CSSHash,
				JSHash:     deps.JSHash,
				NeedsChart: true,
			},
			Realm:    realmName,
			CharName: name,
			Char: CharacterInfo{
				Class:      class,
				ClassLabel: wow.Class(class),
				KillCount:  killCount,
				LastSeen:   lastSeen.Format("2006-01-02 15:04"),
			},
			Filter:      filter,
			BossOptions: bossOpts,
			RaidOptions: raidOpts,
			SpecOptions: specOpts,
			ModeOptions: modeOpts,
			Charts:      charts,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = Page(vm).Render(r.Context(), w)
	}
}

func parseFilter(q map[string][]string) FilterValues {
	f := FilterValues{}
	for _, v := range q["boss"] {
		for _, part := range strings.Split(v, ",") {
			if n, err := strconv.ParseUint(strings.TrimSpace(part), 10, 32); err == nil {
				f.Bosses = append(f.Bosses, uint32(n))
			}
		}
	}
	for _, v := range q["raid"] {
		for _, part := range strings.Split(v, ",") {
			if part = strings.TrimSpace(part); part != "" {
				f.Raids = append(f.Raids, part)
			}
		}
	}
	for _, v := range append(q["difficulty"], q["mode"]...) {
		for _, part := range strings.Split(v, ",") {
			if n, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
				f.Modes = append(f.Modes, n)
			}
		}
	}
	for _, v := range q["spec"] {
		for _, part := range strings.Split(v, ",") {
			if n, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
				f.Specs = append(f.Specs, n)
			}
		}
	}
	f.IlvlMin = query.IntOr(query.First(q, "ilvl_min"), 0)
	f.IlvlMax = query.IntOr(query.First(q, "ilvl_max"), 0)
	return f
}

func lookupCharacter(ctx context.Context, db *sql.DB, realmName, name string) (
	guid uint64, class int, killCount int, lastSeen time.Time, err error,
) {
	character, err := repository.CharacterByName(ctx, db, realmName, name)
	if err != nil {
		return 0, 0, 0, time.Time{}, err
	}
	return character.GUID, character.Class, character.KillCount, character.LastSeen, nil
}

func loadSamples(ctx context.Context, db *sql.DB, realmName string, guid uint64, f FilterValues) ([]Sample, error) {
	var whereParts []string
	var args []any
	args = append(args, guid, realmName, guid)
	whereParts = append(whereParts, "realm = ?", "has(players.guid, ?)")
	if len(f.Bosses) > 0 {
		ph := sqlutil.Placeholders(len(f.Bosses))
		whereParts = append(whereParts, "boss_remote_id IN ("+ph+")")
		for _, b := range f.Bosses {
			args = append(args, b)
		}
	}
	if len(f.Raids) > 0 {
		ph := sqlutil.Placeholders(len(f.Raids))
		whereParts = append(whereParts, "raid_name IN ("+ph+")")
		for _, r := range f.Raids {
			args = append(args, r)
		}
	}
	if len(f.Modes) > 0 {
		ph := sqlutil.Placeholders(len(f.Modes))
		whereParts = append(whereParts, "mode IN ("+ph+")")
		for _, m := range f.Modes {
			args = append(args, uint8(m))
		}
	}
	if len(f.Specs) > 0 {
		ph := sqlutil.Placeholders(len(f.Specs))
		whereParts = append(whereParts, "players.talent_spec[idx] IN ("+ph+")")
		for _, s := range f.Specs {
			args = append(args, uint16(s))
		}
	}
	if f.IlvlMin > 0 {
		whereParts = append(whereParts, "toFloat32(players.avg_item_lvl[idx]) >= ?")
		args = append(args, float32(f.IlvlMin))
	}
	if f.IlvlMax > 0 {
		whereParts = append(whereParts, "toFloat32(players.avg_item_lvl[idx]) <= ?")
		args = append(args, float32(f.IlvlMax))
	}
	args = append(args, sampleLimit)

	q := "WITH indexOf(players.guid, ?) AS idx " +
		"SELECT kill_time, remote_id, boss_remote_id, boss_name, mode, " +
		"players.talent_spec[idx] AS spec, " +
		"toFloat32(players.avg_item_lvl[idx]) AS avg_item_lvl, " +
		metric.SQLUInt64(metric.DmgDoneIndexed) + " AS dps, " +
		metric.SQLUInt64(metric.HealAbsorbIndexed) + " AS hps " +
		"FROM boss_kill " +
		"WHERE " + strings.Join(whereParts, " AND ") + " " +
		"ORDER BY kill_time ASC " +
		"LIMIT ?"
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Sample
	for rows.Next() {
		var (
			t          time.Time
			remoteID   string
			bossID     uint32
			bossName   string
			mode       uint8
			spec       uint16
			avgItemLvl float32
			dps        uint64
			hps        uint64
		)
		if err := rows.Scan(&t, &remoteID, &bossID, &bossName, &mode, &spec, &avgItemLvl, &dps, &hps); err != nil {
			return nil, err
		}
		out = append(out, Sample{
			Time:       t,
			RemoteID:   remoteID,
			BossID:     bossID,
			BossName:   bossName,
			Mode:       int(mode),
			Spec:       int(spec),
			AvgItemLvl: avgItemLvl,
			DPS:        int64(dps),
			HPS:        int64(hps),
		})
	}
	return out, rows.Err()
}

func loadBossOptions(ctx context.Context, db *sql.DB, realmName string, guid uint64) ([]Option, error) {
	const q = `
		SELECT boss_remote_id, any(boss_name) AS name
		FROM boss_kill
		WHERE realm = ? AND has(players.guid, ?)
		GROUP BY boss_remote_id
		ORDER BY name
	`
	rows, err := db.QueryContext(ctx, q, realmName, guid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Option
	for rows.Next() {
		var id uint32
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out = append(out, Option{Value: strconv.FormatUint(uint64(id), 10), Label: name})
	}
	return out, rows.Err()
}

func loadRaidOptions(ctx context.Context, db *sql.DB, realmName string, guid uint64) ([]Option, error) {
	const q = `
		SELECT raid_name
		FROM boss_kill
		WHERE realm = ? AND has(players.guid, ?)
		GROUP BY raid_name
		ORDER BY raid_name
	`
	rows, err := db.QueryContext(ctx, q, realmName, guid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Option
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, Option{Value: name, Label: name})
	}
	return out, rows.Err()
}

func loadSpecOptions(ctx context.Context, db *sql.DB, realmName string, guid uint64, expansion int) ([]Option, error) {
	const q = `
		WITH indexOf(players.guid, ?) AS idx
		SELECT players.talent_spec[idx] AS spec
		FROM boss_kill
		WHERE realm = ?
		  AND has(players.guid, ?)
		  AND players.talent_spec[idx] > 0
		GROUP BY spec
		ORDER BY spec
	`
	rows, err := db.QueryContext(ctx, q, guid, realmName, guid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Option
	for rows.Next() {
		var spec uint16
		if err := rows.Scan(&spec); err != nil {
			return nil, err
		}
		out = append(out, Option{
			Value: strconv.Itoa(int(spec)),
			Label: wow.SpecForExpansion(expansion, int(spec)),
		})
	}
	return out, rows.Err()
}

func loadBossPositions(ctx context.Context, db *sql.DB, realmName string, ids []uint32) (map[uint32]uint16, error) {
	out := map[uint32]uint16{}
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(ids)+1)
	args = append(args, realmName)
	ph := sqlutil.Placeholders(len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	q := "SELECT remote_id, position FROM boss FINAL WHERE realm = ? AND remote_id IN (" + ph + ")"
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uint32
		var position uint16
		if err := rows.Scan(&id, &position); err != nil {
			return nil, err
		}
		out[id] = position
	}
	return out, rows.Err()
}

func markBossSelected(opts []Option, selected []uint32) {
	set := map[string]bool{}
	for _, b := range selected {
		set[strconv.FormatUint(uint64(b), 10)] = true
	}
	for i := range opts {
		opts[i].Selected = set[opts[i].Value]
	}
}

func markStringSelected(opts []Option, selected []string) {
	set := map[string]bool{}
	for _, s := range selected {
		set[s] = true
	}
	for i := range opts {
		opts[i].Selected = set[opts[i].Value]
	}
}

func markIntSelected(opts []Option, selected []int) {
	set := map[string]bool{}
	for _, n := range selected {
		set[strconv.Itoa(n)] = true
	}
	for i := range opts {
		opts[i].Selected = set[opts[i].Value]
	}
}

func buildModeOptions(selectedModes []int, expansion int) []Option {
	modes := wow.RaidDifficulties(expansion)
	sort.Ints(modes)
	sel := map[int]bool{}
	for _, m := range selectedModes {
		sel[m] = true
	}
	out := make([]Option, 0, len(modes))
	for _, m := range modes {
		out = append(out, Option{
			Value:    strconv.Itoa(m),
			Label:    wow.Difficulty(expansion, m),
			Selected: sel[m],
		})
	}
	return out
}

func Mount(mux *http.ServeMux, deps Deps) {
	h := middleware.RequireRealm(Handler(deps))
	router.ForEachRealmPrefix(func(prefix string) {
		mux.Handle("GET "+prefix+"/character/{name}/performance", h)
	})
}
