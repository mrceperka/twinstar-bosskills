package ranks

import (
	"context"
	"database/sql"
	"net/http"
	"sort"
	"strconv"
	"time"

	"twinstar-bosskills/internal/collection"
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

type rawRank struct {
	BossID uint32
	Spec   uint16
	Class  uint8
	GUID   uint64
	DPS    uint64
	HPS    uint64
}

type bossInfoRec struct {
	Name         string
	RaidName     string
	BossPosition uint16
	RaidPosition uint16
}

type killDetailKey struct {
	BossID uint32
	GUID   uint64
}

type killDetail struct {
	KillID    string
	LengthSec int
	Ilvl      float32
}

func Handler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		realmName := middleware.Realm(r.Context())
		if !realm.IsPublic(realmName) {
			http.Error(w, "ranks are not exposed for private realms", http.StatusForbidden)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		expansion := realm.Expansion(realmName)
		classMode := realm.IsVanilla(expansion)
		mode := wow.DefaultDifficulty(expansion)
		if !classMode {
			if v, ok := query.Difficulty(r.URL.Query()); ok {
				mode = v
			}
		}
		offset, _ := middleware.RaidLock(r.Context())

		now := time.Now().UTC()
		win := domain.RaidLock(now, offset)

		if r.URL.Query().Get("partial") == "raid" {
			raidName := r.URL.Query().Get("raid")
			if raidName == "" {
				http.Error(w, "missing raid", http.StatusBadRequest)
				return
			}
			content, err := loadRaidRankContent(ctx, deps.DB, realmName, raidName, win, mode, expansion, classMode)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_ = RaidRanksFragment(content).Render(r.Context(), w)
			return
		}

		bossIDs, err := loadRankBossIDs(ctx, deps.DB, realmName, win.Start, win.End, mode, expansion)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		bossInfos, err := loadBossInfos(ctx, deps.DB, realmName, bossIDs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		raidCounts, err := loadRaidBossCounts(ctx, deps.DB, realmName, rankRaidNames(bossIDs, bossInfos))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		raids := buildRaidSummaries(bossIDs, bossInfos, raidCounts)

		vm := ViewModel{
			Meta: layouts.PageMeta{
				Title:      realmName + " / Ranks",
				Realm:      realmName,
				ActivePath: "/ranks",
				CSSHash:    deps.CSSHash,
				JSHash:     deps.JSHash,
			},
			Realm:        realmName,
			LockLabel:    win.Start.Format("Jan 2 15:04 UTC") + " → " + win.End.Format("Jan 2 15:04 UTC"),
			LockOffset:   offset,
			ClassMode:    classMode,
			Difficulties: buildDifficulties(expansion, classMode),
			SelectedMode: mode,
			ModeLabel:    modeLabel(expansion, mode, classMode),
			Raids:        raids,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = Page(vm).Render(r.Context(), w)
	}
}

func buildDifficulties(expansion int, classMode bool) []DifficultyChoice {
	if classMode {
		return nil
	}
	modes := wow.RaidDifficulties(expansion)
	out := make([]DifficultyChoice, 0, len(modes))
	for _, m := range modes {
		out = append(out, DifficultyChoice{Mode: m, Label: wow.Difficulty(expansion, m)})
	}
	return out
}

func modeLabel(expansion, mode int, classMode bool) string {
	if classMode {
		return "All kills"
	}
	return wow.Difficulty(expansion, mode)
}

func loadRankBossIDs(ctx context.Context, db *sql.DB, realmName string, lockStart, lockEnd time.Time, mode, expansion int) ([]uint32, error) {
	if realm.IsVanilla(expansion) {
		q := `
			SELECT boss_remote_id
			FROM boss_kill ARRAY JOIN players
			WHERE realm = ?
			  AND kill_time >= ? AND kill_time < ?
			  AND length > 0
			  AND players.class > 0
			GROUP BY boss_remote_id
		`
		rows, err := db.QueryContext(ctx, q, realmName, lockStart, lockEnd)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []uint32
		for rows.Next() {
			var id uint32
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			out = append(out, id)
		}
		return out, rows.Err()
	}

	const q = `
		SELECT boss_remote_id
		FROM raid_lock_rankings
		WHERE realm = ?
		  AND raid_lock = toDate(?)
		  AND mode = ?
		GROUP BY boss_remote_id
	`
	rows, err := db.QueryContext(ctx, q, realmName, lockStart, uint8(mode))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []uint32
	for rows.Next() {
		var id uint32
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func loadRankRowsForBosses(ctx context.Context, db *sql.DB, realmName string, lockStart, lockEnd time.Time, mode, expansion int, bossIDs []uint32) ([]rawRank, error) {
	if len(bossIDs) == 0 {
		return nil, nil
	}
	ph := sqlutil.Placeholders(len(bossIDs))
	if realm.IsVanilla(expansion) {
		args := make([]any, 0, len(bossIDs)+3)
		args = append(args, realmName, lockStart, lockEnd)
		for _, id := range bossIDs {
			args = append(args, id)
		}
		q := `
			SELECT boss_remote_id,
			       players.class,
			       players.guid,
			       max(` + metric.SQLUInt64(metric.DmgDoneArrayJoin) + `) AS dps,
			       max(` + metric.SQLUInt64(metric.HealAbsorbArrayJoin) + `) AS hps
			FROM boss_kill ARRAY JOIN players
			WHERE realm = ?
			  AND kill_time >= ? AND kill_time < ?
			  AND length > 0
			  AND players.class > 0
			  AND boss_remote_id IN (` + ph + `)
			GROUP BY boss_remote_id, players.class, players.guid
		`
		rows, err := db.QueryContext(ctx, q, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []rawRank
		for rows.Next() {
			var r rawRank
			if err := rows.Scan(&r.BossID, &r.Class, &r.GUID, &r.DPS, &r.HPS); err != nil {
				return nil, err
			}
			out = append(out, r)
		}
		return out, rows.Err()
	}

	args := make([]any, 0, len(bossIDs)+3)
	args = append(args, realmName, lockStart, uint8(mode))
	for _, id := range bossIDs {
		args = append(args, id)
	}
	q := `
		SELECT boss_remote_id, talent_spec, guid,
		       maxMerge(dps_state) AS dps,
		       maxMerge(hps_state) AS hps
		FROM raid_lock_rankings
		WHERE realm = ?
		  AND raid_lock = toDate(?)
		  AND mode = ?
		  AND boss_remote_id IN (` + ph + `)
		GROUP BY realm, raid_lock, boss_remote_id, mode, talent_spec, guid
	`
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []rawRank
	for rows.Next() {
		var r rawRank
		if err := rows.Scan(&r.BossID, &r.Spec, &r.GUID, &r.DPS, &r.HPS); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func loadBossInfos(ctx context.Context, db *sql.DB, realmName string, ids []uint32) (map[uint32]bossInfoRec, error) {
	out := map[uint32]bossInfoRec{}
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(ids)+2)
	args = append(args, realmName)
	ph := sqlutil.Placeholders(len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	args = append(args, realmName)
	q := `
		SELECT b.remote_id, b.name, b.raid_name, b.position, ifNull(r.position, 0)
		FROM (
			SELECT remote_id, name, raid_name, position
			FROM boss FINAL
			WHERE realm = ? AND remote_id IN (` + ph + `)
		) AS b
		LEFT ANY JOIN (
			SELECT name, position
			FROM raid FINAL
			WHERE realm = ?
		) AS r ON r.name = b.raid_name
	`
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uint32
		var name, raidName string
		var bossPos, raidPos uint16
		if err := rows.Scan(&id, &name, &raidName, &bossPos, &raidPos); err != nil {
			return nil, err
		}
		out[id] = bossInfoRec{
			Name:         name,
			RaidName:     raidName,
			BossPosition: bossPos,
			RaidPosition: raidPos,
		}
	}
	return out, rows.Err()
}

func loadRaidBossCounts(ctx context.Context, db *sql.DB, realmName string, raidNames []string) (map[string]int, error) {
	out := map[string]int{}
	if len(raidNames) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(raidNames)+1)
	args = append(args, realmName)
	ph := sqlutil.Placeholders(len(raidNames))
	for _, name := range raidNames {
		args = append(args, name)
	}
	q := `
		SELECT raid_name, count()
		FROM boss FINAL
		WHERE realm = ? AND raid_name IN (` + ph + `)
		GROUP BY raid_name
	`
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var count uint64
		if err := rows.Scan(&name, &count); err != nil {
			return nil, err
		}
		out[name] = int(count)
	}
	return out, rows.Err()
}

func loadNames(ctx context.Context, db *sql.DB, realmName string, guids []uint64) (map[uint64]string, error) {
	out := map[uint64]string{}
	if len(guids) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(guids)+1)
	args = append(args, realmName)
	ph := sqlutil.Placeholders(len(guids))
	for _, g := range guids {
		args = append(args, g)
	}
	q := "SELECT guid, argMaxMerge(name_state) FROM character " +
		"WHERE realm = ? AND guid IN (" + ph + ") " +
		"GROUP BY realm, guid"
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var g uint64
		var name string
		if err := rows.Scan(&g, &name); err != nil {
			return nil, err
		}
		out[g] = name
	}
	return out, rows.Err()
}

// loadKillDetails finds the best kill (by DPS) per (boss, guid) within the
// lock window, returning fight length and item level for the rank table.
func loadKillDetails(ctx context.Context, db *sql.DB, realmName string, lockStart, lockEnd time.Time, mode, expansion int, guids []uint64) (map[killDetailKey]killDetail, error) {
	out := map[killDetailKey]killDetail{}
	if len(guids) == 0 {
		return out, nil
	}
	ph := sqlutil.Placeholders(len(guids))
	args := make([]any, 0, len(guids)+4)
	args = append(args, realmName, lockStart, lockEnd)
	for _, g := range guids {
		args = append(args, g)
	}
	q := `SELECT
		boss_remote_id,
		players.guid,
		argMax(remote_id, ` + metric.SQLUInt64(metric.DmgDoneArrayJoin) + `) AS best_kill_id,
		argMax(length, ` + metric.SQLUInt64(metric.DmgDoneArrayJoin) + `) AS best_length,
		argMax(toFloat32(players.avg_item_lvl), ` + metric.SQLUInt64(metric.DmgDoneArrayJoin) + `) AS best_ilvl
	FROM boss_kill
	ARRAY JOIN players
	WHERE realm = ?
	  AND kill_time >= ? AND kill_time < ?
	  AND players.guid IN (` + ph + `)
	`
	if !realm.IsVanilla(expansion) {
		q += " AND mode = ?"
		args = append(args, uint8(mode))
	}
	q += " GROUP BY boss_remote_id, players.guid"
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var bossID uint32
		var guid uint64
		var killID string
		var length uint64
		var ilvl float32
		if err := rows.Scan(&bossID, &guid, &killID, &length, &ilvl); err != nil {
			return nil, err
		}
		out[killDetailKey{BossID: bossID, GUID: guid}] = killDetail{
			KillID:    killID,
			LengthSec: int(length) / 1000,
			Ilvl:      ilvl,
		}
	}
	return out, rows.Err()
}

func loadRaidRankContent(ctx context.Context, db *sql.DB, realmName, raidName string, win domain.RaidLockWindow, mode, expansion int, classMode bool) (RaidRankContent, error) {
	content := RaidRankContent{
		Realm:        realmName,
		ClassMode:    classMode,
		SelectedMode: mode,
		RaidName:     raidName,
	}

	bossIDs, err := loadRankBossIDs(ctx, db, realmName, win.Start, win.End, mode, expansion)
	if err != nil {
		return content, err
	}
	bossInfos, err := loadBossInfos(ctx, db, realmName, bossIDs)
	if err != nil {
		return content, err
	}
	raidCounts, err := loadRaidBossCounts(ctx, db, realmName, rankRaidNames(bossIDs, bossInfos))
	if err != nil {
		return content, err
	}
	var summary RaidGroup
	for _, raid := range buildRaidSummaries(bossIDs, bossInfos, raidCounts) {
		if raid.Name == raidName {
			summary = raid
			break
		}
	}
	if len(summary.Bosses) == 0 {
		return content, nil
	}

	raidBossIDs := make([]uint32, 0, len(summary.Bosses))
	for _, boss := range summary.Bosses {
		raidBossIDs = append(raidBossIDs, boss.BossID)
	}
	allRows, err := loadRankRowsForBosses(ctx, db, realmName, win.Start, win.End, mode, expansion, raidBossIDs)
	if err != nil {
		return content, err
	}

	guidSet := map[uint64]bool{}
	for _, r := range allRows {
		guidSet[r.GUID] = true
	}
	names, err := loadNames(ctx, db, realmName, collection.Keys(guidSet))
	if err != nil {
		return content, err
	}
	killDetails, err := loadKillDetails(ctx, db, realmName, win.Start, win.End, mode, expansion, collection.Keys(guidSet))
	if err != nil {
		return content, err
	}

	dpsRaids := buildRaidGroups(allRows, bossInfos, killDetails, names, "dps", expansion)
	hpsRaids := buildRaidGroups(allRows, bossInfos, killDetails, names, "hps", expansion)
	return buildRaidRankContent(content, summary, dpsRaids, hpsRaids), nil
}

func rankRaidNames(bossIDs []uint32, bossInfos map[uint32]bossInfoRec) []string {
	seen := map[string]bool{}
	for _, bossID := range bossIDs {
		name := bossInfos[bossID].RaidName
		if name != "" {
			seen[name] = true
		}
	}
	return collection.Keys(seen)
}

func buildRaidSummaries(bossIDs []uint32, bossInfos map[uint32]bossInfoRec, raidCounts map[string]int) []RaidGroup {
	type raidSummary struct {
		Name     string
		Position uint16
		Bosses   []BossRankGroup
	}
	raidMap := map[string]*raidSummary{}
	for _, bossID := range bossIDs {
		info := bossInfos[bossID]
		raidName := info.RaidName
		if raidName == "" {
			continue
		}
		bossName := info.Name
		if bossName == "" {
			bossName = "Boss " + strconv.FormatUint(uint64(bossID), 10)
		}
		raid, ok := raidMap[raidName]
		if !ok {
			raid = &raidSummary{Name: raidName, Position: info.RaidPosition}
			raidMap[raidName] = raid
		}
		if raid.Position == 0 {
			raid.Position = info.RaidPosition
		}
		raid.Bosses = append(raid.Bosses, BossRankGroup{
			BossID:   bossID,
			BossName: bossName,
		})
	}

	out := make([]RaidGroup, 0, len(raidMap))
	for _, raid := range raidMap {
		sort.Slice(raid.Bosses, func(i, j int) bool {
			oi := bossInfos[raid.Bosses[i].BossID].BossPosition
			oj := bossInfos[raid.Bosses[j].BossID].BossPosition
			if oi != oj {
				return oi < oj
			}
			return raid.Bosses[i].BossID < raid.Bosses[j].BossID
		})
		out = append(out, RaidGroup{Name: raid.Name, Bosses: raid.Bosses, TotalBosses: raidCounts[raid.Name]})
	}

	sort.Slice(out, func(i, j int) bool {
		pi := raidMap[out[i].Name].Position
		pj := raidMap[out[j].Name].Position
		if pi != pj {
			return pi > pj
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func buildRaidRankContent(base RaidRankContent, summary RaidGroup, dpsRaids, hpsRaids []RaidGroup) RaidRankContent {
	dpsByBoss := bossRankGroupsByID(dpsRaids)
	hpsByBoss := bossRankGroupsByID(hpsRaids)
	base.Bosses = make([]BossRankContent, 0, len(summary.Bosses))
	for _, boss := range summary.Bosses {
		base.Bosses = append(base.Bosses, BossRankContent{
			BossID:     boss.BossID,
			BossName:   boss.BossName,
			DPSEntries: dpsByBoss[boss.BossID].Entries,
			HPSEntries: hpsByBoss[boss.BossID].Entries,
		})
	}
	return base
}

func bossRankGroupsByID(raids []RaidGroup) map[uint32]BossRankGroup {
	out := map[uint32]BossRankGroup{}
	for _, raid := range raids {
		for _, boss := range raid.Bosses {
			out[boss.BossID] = boss
		}
	}
	return out
}

func buildRaidGroups(allRows []rawRank, bossInfos map[uint32]bossInfoRec, killDetails map[killDetailKey]killDetail, names map[uint64]string, metric string, expansion int) []RaidGroup {
	// Group raw rows by boss.
	type bossRows struct {
		BossID  uint32
		Entries []rawRank
	}
	bossByID := map[uint32]*bossRows{}
	for _, r := range allRows {
		b, ok := bossByID[r.BossID]
		if !ok {
			b = &bossRows{BossID: r.BossID}
			bossByID[r.BossID] = b
		}
		b.Entries = append(b.Entries, r)
	}

	// Group bosses by raid name.
	type raidBosses struct {
		Name     string
		Position uint16
		Bosses   []*bossRows
	}
	raidMap := map[string]*raidBosses{}
	for _, b := range bossByID {
		info := bossInfos[b.BossID]
		r, ok := raidMap[info.RaidName]
		if !ok {
			r = &raidBosses{Name: info.RaidName, Position: info.RaidPosition}
			raidMap[info.RaidName] = r
		}
		if r.Position == 0 {
			r.Position = info.RaidPosition
		}
		r.Bosses = append(r.Bosses, b)
	}

	out := make([]RaidGroup, 0, len(raidMap))
	for _, r := range raidMap {
		// Sort bosses by encounter order.
		sort.Slice(r.Bosses, func(i, j int) bool {
			oi := bossInfos[r.Bosses[i].BossID].BossPosition
			oj := bossInfos[r.Bosses[j].BossID].BossPosition
			if oi != oj {
				return oi < oj
			}
			return r.Bosses[i].BossID < r.Bosses[j].BossID
		})

		bossList := make([]BossRankGroup, 0, len(r.Bosses))
		for _, b := range r.Bosses {
			info := bossInfos[b.BossID]
			entries := buildRankEntries(b.Entries, killDetails, names, metric, expansion)
			if len(entries) > 0 {
				bossList = append(bossList, BossRankGroup{
					BossID:   b.BossID,
					BossName: info.Name,
					Entries:  entries,
				})
			}
		}
		if len(bossList) > 0 {
			out = append(out, RaidGroup{Name: r.Name, Bosses: bossList})
		}
	}

	// Sort raids: newest first.
	sort.Slice(out, func(i, j int) bool {
		pi := raidMap[out[i].Name].Position
		pj := raidMap[out[j].Name].Position
		if pi != pj {
			return pi > pj
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func buildRankEntries(rows []rawRank, killDetails map[killDetailKey]killDetail, names map[uint64]string, metric string, expansion int) []Rank {
	type best struct {
		rawRank
		val uint64
	}
	byGroup := map[int]best{}
	classMode := realm.IsVanilla(expansion)
	for _, r := range rows {
		var val uint64
		if metric == "dps" {
			val = r.DPS
		} else {
			val = r.HPS
		}
		if val == 0 {
			continue
		}
		key := int(r.Spec)
		if classMode {
			key = int(r.Class)
		}
		if prev, ok := byGroup[key]; !ok || val > prev.val {
			byGroup[key] = best{r, val}
		}
	}

	sorted := make([]best, 0, len(byGroup))
	for _, b := range byGroup {
		sorted = append(sorted, b)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].val > sorted[j].val })

	out := make([]Rank, 0, len(sorted))
	for i, b := range sorted {
		name := names[b.GUID]
		if name == "" {
			name = "Unknown"
		}
		dk := killDetailKey{BossID: b.BossID, GUID: b.GUID}
		detail := killDetails[dk]
		class := wow.ClassFromSpecForExpansion(expansion, int(b.Spec))
		specLabel := wow.SpecForExpansion(expansion, int(b.Spec))
		if classMode {
			class = int(b.Class)
			specLabel = ""
		}
		out = append(out, Rank{
			Rank:       i + 1,
			Name:       name,
			Class:      class,
			ClassLabel: wow.Class(class),
			Spec:       int(b.Spec),
			SpecLabel:  specLabel,
			DPS:        int64(b.DPS),
			HPS:        int64(b.HPS),
			LengthSec:  detail.LengthSec,
			Ilvl:       detail.Ilvl,
			KillID:     detail.KillID,
		})
	}
	return out
}

func Mount(mux *http.ServeMux, deps Deps) {
	h := middleware.RequireRealm(Handler(deps))
	router.ForEachRealmPrefix(func(prefix string) {
		mux.Handle("GET "+prefix+"/ranks", h)
	})
}
