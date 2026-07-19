package character

import (
	"context"
	"database/sql"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"twinstar-bosskills/internal/api"
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

const (
	defaultSortBy  = "kill_time"
	defaultSortDir = "desc"
)

// validSortCols keeps the sort column safe against injection since it is
// concatenated into the ORDER BY clause.
var validSortCols = map[string]string{
	"kill_time": "kill_time",
	"dps":       "dps",
	"hps":       "hps",
	"length":    "length",
	"ilvl":      "avg_item_lvl",
}

type Deps struct {
	DB      *sql.DB
	API     *api.Client
	CSSHash string
	JSHash  string
}

const killsPageSize = 20

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

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		expansion := realm.Expansion(realmName)
		isPartial := middleware.IsHTMX(r) && r.URL.Query().Get("full") == ""

		guid, class, firstSeen, lastSeen, killCount, err := lookupCharacter(ctx, deps.DB, realmName, name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if guid == 0 {
			http.NotFound(w, r)
			return
		}

		killsPage := query.IntOr(r.URL.Query().Get("page"), 0)
		if killsPage < 0 {
			killsPage = 0
		}

		filter := parseKillsFilter(r.URL.Query())

		recent, killsTotal, err := loadRecentKills(ctx, deps.DB, realmName, guid, expansion, filter, killsPage, killsPageSize)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		specSummary, err := loadSpecSummary(ctx, deps.DB, realmName, guid, expansion)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		bossOpts, raidOpts, modeOpts, specOpts, err := loadKillsFilterOptions(ctx, deps.DB, realmName, guid, expansion, filter)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		vm := ViewModel{
			Meta: layouts.PageMeta{
				Title:   name + " - " + realmName,
				Realm:   realmName,
				CSSHash: deps.CSSHash,
				JSHash:  deps.JSHash,
			},
			Realm: realmName,
			Char: CharacterInfo{
				Name:       name,
				Class:      class,
				ClassLabel: wow.Class(class),
				FirstSeen:  firstSeen.Format("2006-01-02"),
				LastSeen:   lastSeen.Format("2006-01-02 15:04"),
				KillCount:  killCount,
			},
			KillsPage:     killsPage,
			KillsPageSize: killsPageSize,
			KillsTotal:    killsTotal,
			SpecSummary:   specSummary,
			RecentKills:   recent,
			Filter:        filter,
			BossOptions:   bossOpts,
			RaidOptions:   raidOpts,
			ModeOptions:   modeOpts,
			SpecOptions:   specOpts,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		if isPartial {
			_ = KillsTableFragment(vm).Render(r.Context(), w)
			return
		}

		_ = Page(vm).Render(r.Context(), w)
	}
}

// RankingsHandler handles GET /{realm}/character/{name}/rankings and returns
// the lazy-loaded rankings fragment inserted into the <details> element.
func RankingsHandler(deps Deps) http.HandlerFunc {
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

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		expansion := realm.Expansion(realmName)
		currentSpec := query.IntOr(r.URL.Query().Get("spec"), 0)

		guid, _, _, _, _, err := lookupCharacter(ctx, deps.DB, realmName, name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if guid == 0 {
			http.NotFound(w, r)
			return
		}

		rankRows, err := loadRankings(ctx, deps.DB, realmName, guid, expansion)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		killDetails, err := loadBestKillDetails(ctx, deps.DB, realmName, guid)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		bossIDsSet := map[uint32]bool{}
		for _, rr := range rankRows {
			bossIDsSet[rr.BossID] = true
		}
		bossMeta, err := loadBossMeta(ctx, deps.DB, realmName, collection.Keys(bossIDsSet))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		dpsGroups, hpsGroups := buildBossGroups(rankRows, killDetails, bossMeta, expansion, currentSpec)
		specButtons := buildSpecButtons(rankRows, currentSpec, realmName, name)

		vm := RankingsViewModel{
			Realm:       realmName,
			CharName:    name,
			DPSGroups:   dpsGroups,
			HPSGroups:   hpsGroups,
			SpecButtons: specButtons,
			CurrentSpec: currentSpec,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = RankingsFragment(vm).Render(r.Context(), w)
	}
}

func ActivityHandler(deps Deps) http.HandlerFunc {
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

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		guid, _, _, _, _, err := lookupCharacter(ctx, deps.DB, realmName, name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if guid == 0 {
			http.NotFound(w, r)
			return
		}
		page := query.IntOr(r.URL.Query().Get("page"), 0)
		if page < 0 {
			page = 0
		}

		cli := deps.API
		if cli == nil {
			cli = api.NewClient("")
		}

		now := time.Now().UTC()
		meta, refreshErr := refreshActivityIfStale(ctx, deps.DB, cli, realmName, name, now)

		visibleLimit := (page + 1) * activityDisplayPageSize
		rows, err := loadActivityRows(ctx, deps.DB, realmName, name, realm.Expansion(realmName), visibleLimit+1)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		hasMore := len(rows) > visibleLimit
		if hasMore {
			rows = rows[:visibleLimit]
		}

		vm := ActivityViewModel{
			Realm:    realmName,
			CharName: name,
			Rows:     rows,
			Notice:   "Activity feed is cached and may be up to 1 hour stale.",
			Page:     page,
			HasMore:  hasMore,
		}
		if hasMore {
			vm.NextHref = activityPagedHref(realmName, name, page+1)
		}
		if !meta.LastSuccessAt.IsZero() && meta.LastSuccessAt.After(time.Unix(0, 0)) {
			vm.LastSuccessAt = meta.LastSuccessAt.Format("2006-01-02 15:04")
		}
		if refreshErr != nil {
			if len(rows) > 0 {
				vm.Warning = "Latest refresh failed; showing cached activity."
			} else {
				vm.Error = "Activity is unavailable right now."
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = ActivityFragment(vm).Render(r.Context(), w)
	}
}

func StatsHandler(deps Deps) http.HandlerFunc {
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

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		guid, class, _, _, _, err := lookupCharacter(ctx, deps.DB, realmName, name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if guid == 0 {
			http.NotFound(w, r)
			return
		}

		cli := deps.API
		if cli == nil {
			cli = api.NewClient("")
		}

		row, refreshErr := refreshStatsIfStale(ctx, deps.DB, cli, realmName, name, time.Now().UTC())
		row.CharacterClass = class
		vm := buildStatsViewModel(r.Context(), row, refreshErr)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = StatsFragment(vm).Render(r.Context(), w)
	}
}

// rankRow is one character's best-per-(boss, mode, spec) with pre-computed ranks.
type rankRow struct {
	BossID    uint32
	Mode      int
	Spec      int
	SpecLabel string
	Class     int
	DPS       int64
	HPS       int64
	DPSRank   int
	HPSRank   int
}

type bossKey struct {
	BossID uint32
	Mode   int
	Spec   int
}

type killDetail struct {
	DPSKillID string
	DPSilvl   float32
	HPSKillID string
	HPSilvl   float32
}

func loadRankings(ctx context.Context, db *sql.DB, realmName string, guid uint64, expansion int) ([]rankRow, error) {
	// my_bests: character's best DPS/HPS per (boss, mode, spec)
	// peers: all players' best on the same (boss, mode, spec) combos
	// Final: count peers with higher value to compute rank (1-indexed)
	const q = `
	WITH
	  my_bests AS (
	    SELECT boss_remote_id, mode, talent_spec,
	           maxMerge(dps_state) AS dps, maxMerge(hps_state) AS hps
	    FROM character_boss_rankings
	    WHERE realm = ? AND guid = ?
	    GROUP BY realm, boss_remote_id, mode, talent_spec, guid
	    HAVING dps > 0 OR hps > 0
	  ),
	  peers AS (
	    SELECT boss_remote_id, mode, talent_spec,
	           maxMerge(dps_state) AS dps, maxMerge(hps_state) AS hps
	    FROM character_boss_rankings
	    WHERE realm = ?
	      AND (boss_remote_id, mode, talent_spec) IN (
	        SELECT boss_remote_id, mode, talent_spec FROM my_bests
	      )
	    GROUP BY realm, boss_remote_id, mode, talent_spec, guid
	  )
	SELECT
	  m.boss_remote_id,
	  m.mode,
	  m.talent_spec,
	  m.dps,
	  m.hps,
	  countIf(p.dps > m.dps) + 1 AS dps_rank,
	  countIf(p.hps > m.hps) + 1 AS hps_rank
	FROM my_bests m
	JOIN peers p USING (boss_remote_id, mode, talent_spec)
	GROUP BY m.boss_remote_id, m.mode, m.talent_spec, m.dps, m.hps
	`
	rows, err := db.QueryContext(ctx, q, realmName, guid, realmName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []rankRow
	for rows.Next() {
		var (
			bossID           uint32
			mode             uint8
			spec             uint16
			dps, hps         uint64
			dpsRank, hpsRank uint64
		)
		if err := rows.Scan(&bossID, &mode, &spec, &dps, &hps, &dpsRank, &hpsRank); err != nil {
			return nil, err
		}
		s := int(spec)
		out = append(out, rankRow{
			BossID:    bossID,
			Mode:      int(mode),
			Spec:      s,
			SpecLabel: wow.SpecForExpansion(expansion, s),
			Class:     wow.ClassFromSpecForExpansion(expansion, s),
			DPS:       int64(dps),
			HPS:       int64(hps),
			DPSRank:   int(dpsRank),
			HPSRank:   int(hpsRank),
		})
	}
	return out, rows.Err()
}

func loadBestKillDetails(ctx context.Context, db *sql.DB, realmName string, guid uint64) (map[bossKey]killDetail, error) {
	q := `
	WITH indexOf(players.guid, ?) AS idx
	SELECT
	  boss_remote_id,
	  mode,
	  players.talent_spec[idx] AS spec,
	  argMax(remote_id, ` + metric.SQLUInt64(metric.DmgDoneIndexed) + `) AS dps_kill_id,
	  toFloat32(argMax(players.avg_item_lvl[idx], ` + metric.SQLUInt64(metric.DmgDoneIndexed) + `)) AS dps_ilvl,
	  argMax(remote_id, ` + metric.SQLUInt64(metric.HealAbsorbIndexed) + `) AS hps_kill_id,
	  toFloat32(argMax(players.avg_item_lvl[idx], ` + metric.SQLUInt64(metric.HealAbsorbIndexed) + `)) AS hps_ilvl
	FROM boss_kill
	WHERE realm = ? AND has(players.guid, ?)
	GROUP BY boss_remote_id, mode, spec
	`
	rows, err := db.QueryContext(ctx, q, guid, realmName, guid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[bossKey]killDetail{}
	for rows.Next() {
		var (
			bossID    uint32
			mode      uint8
			spec      uint16
			dpsKillID string
			dpsIlvl   float32
			hpsKillID string
			hpsIlvl   float32
		)
		if err := rows.Scan(&bossID, &mode, &spec, &dpsKillID, &dpsIlvl, &hpsKillID, &hpsIlvl); err != nil {
			return nil, err
		}
		out[bossKey{bossID, int(mode), int(spec)}] = killDetail{
			DPSKillID: dpsKillID,
			DPSilvl:   dpsIlvl,
			HPSKillID: hpsKillID,
			HPSilvl:   hpsIlvl,
		}
	}
	return out, rows.Err()
}

type bossMeta struct {
	Name     string
	Position uint16
}

func buildBossGroups(rows []rankRow, details map[bossKey]killDetail, meta map[uint32]bossMeta, expansion int, specFilter int) (dpsGroups, hpsGroups []BossGroup) {
	type bossMode struct {
		BossID uint32
		Mode   int
	}

	dpsMap := map[bossMode]rankRow{}
	hpsMap := map[bossMode]rankRow{}

	for _, r := range rows {
		if specFilter != 0 && r.Spec != specFilter {
			continue
		}
		k := bossMode{r.BossID, r.Mode}
		if existing, ok := dpsMap[k]; !ok || r.DPS > existing.DPS {
			dpsMap[k] = r
		}
		if existing, ok := hpsMap[k]; !ok || r.HPS > existing.HPS {
			hpsMap[k] = r
		}
	}

	bossName := func(id uint32) string {
		if m := meta[id]; m.Name != "" {
			return m.Name
		}
		return strconv.FormatUint(uint64(id), 10)
	}
	bossPosition := func(id uint32) uint16 {
		return meta[id].Position
	}

	dpsByBoss := map[uint32][]RankingEntry{}
	for k, r := range dpsMap {
		if r.DPS <= 0 {
			continue
		}
		d := details[bossKey{k.BossID, k.Mode, r.Spec}]
		dpsByBoss[k.BossID] = append(dpsByBoss[k.BossID], RankingEntry{
			ModeLabel: wow.Difficulty(expansion, k.Mode),
			Spec:      r.Spec,
			SpecLabel: r.SpecLabel,
			Class:     r.Class,
			Value:     r.DPS,
			Rank:      r.DPSRank,
			KillID:    d.DPSKillID,
			Ilvl:      d.DPSilvl,
		})
	}
	for bossID, entries := range dpsByBoss {
		sort.Slice(entries, func(i, j int) bool { return entries[i].ModeLabel < entries[j].ModeLabel })
		dpsGroups = append(dpsGroups, BossGroup{BossID: bossID, Name: bossName(bossID), Entries: entries})
	}
	sort.Slice(dpsGroups, func(i, j int) bool {
		oi := bossPosition(dpsGroups[i].BossID)
		oj := bossPosition(dpsGroups[j].BossID)
		if oi != oj {
			return oi < oj
		}
		return dpsGroups[i].Name < dpsGroups[j].Name
	})

	hpsByBoss := map[uint32][]RankingEntry{}
	for k, r := range hpsMap {
		if r.HPS <= 0 {
			continue
		}
		d := details[bossKey{k.BossID, k.Mode, r.Spec}]
		hpsByBoss[k.BossID] = append(hpsByBoss[k.BossID], RankingEntry{
			ModeLabel: wow.Difficulty(expansion, k.Mode),
			Spec:      r.Spec,
			SpecLabel: r.SpecLabel,
			Class:     r.Class,
			Value:     r.HPS,
			Rank:      r.HPSRank,
			KillID:    d.HPSKillID,
			Ilvl:      d.HPSilvl,
		})
	}
	for bossID, entries := range hpsByBoss {
		sort.Slice(entries, func(i, j int) bool { return entries[i].ModeLabel < entries[j].ModeLabel })
		hpsGroups = append(hpsGroups, BossGroup{BossID: bossID, Name: bossName(bossID), Entries: entries})
	}
	sort.Slice(hpsGroups, func(i, j int) bool {
		oi := bossPosition(hpsGroups[i].BossID)
		oj := bossPosition(hpsGroups[j].BossID)
		if oi != oj {
			return oi < oj
		}
		return hpsGroups[i].Name < hpsGroups[j].Name
	})

	return dpsGroups, hpsGroups
}

func buildSpecButtons(rows []rankRow, currentSpec int, realmName, charName string) []SpecButton {
	seen := map[int]bool{}
	var buttons []SpecButton
	for _, r := range rows {
		if seen[r.Spec] {
			continue
		}
		seen[r.Spec] = true
		// Clicking the active spec deselects (goes to spec=0); otherwise selects.
		targetSpec := r.Spec
		if r.Spec == currentSpec {
			targetSpec = 0
		}
		buttons = append(buttons, SpecButton{
			Spec:     r.Spec,
			Class:    r.Class,
			IsActive: r.Spec == currentSpec,
			Href:     rankingsHref(realmName, charName, targetSpec),
		})
	}
	sort.Slice(buttons, func(i, j int) bool { return buttons[i].Spec < buttons[j].Spec })
	return buttons
}

func lookupCharacter(ctx context.Context, db *sql.DB, realmName, name string) (
	guid uint64, class int, firstSeen, lastSeen time.Time, killCount int, err error,
) {
	character, err := repository.CharacterByName(ctx, db, realmName, name)
	if err != nil {
		return 0, 0, time.Time{}, time.Time{}, 0, err
	}
	return character.GUID, character.Class, character.FirstSeen, character.LastSeen, character.KillCount, nil
}

func loadSpecSummary(ctx context.Context, db *sql.DB, realmName string, guid uint64, expansion int) ([]SpecSummary, error) {
	const q = `
		WITH indexOf(players.guid, ?) AS idx
		SELECT players.talent_spec[idx] AS spec, count() AS kills
		FROM boss_kill
		WHERE realm = ? AND has(players.guid, ?) AND players.talent_spec[idx] > 0
		GROUP BY spec
	`
	rows, err := db.QueryContext(ctx, q, guid, realmName, guid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[int]uint64{}
	for rows.Next() {
		var (
			spec  uint16
			kills uint64
		)
		if err := rows.Scan(&spec, &kills); err != nil {
			return nil, err
		}
		counts[int(spec)] = kills
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return buildSpecSummaryRows(expansion, counts), nil
}

func buildSpecSummaryRows(expansion int, counts map[int]uint64) []SpecSummary {
	rows := make([]SpecSummary, 0, len(counts))
	for spec, kills := range counts {
		if spec <= 0 || kills == 0 {
			continue
		}
		rows = append(rows, SpecSummary{
			Spec:      spec,
			Class:     wow.ClassFromSpecForExpansion(expansion, spec),
			SpecLabel: wow.SpecForExpansion(expansion, spec),
			KillCount: int(kills),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].KillCount != rows[j].KillCount {
			return rows[i].KillCount > rows[j].KillCount
		}
		return rows[i].Spec < rows[j].Spec
	})
	if len(rows) > 0 {
		rows[0].IsMostPlayed = true
	}
	return rows
}

func loadRecentKills(ctx context.Context, db *sql.DB, realmName string, guid uint64, expansion int, f KillsFilter, page, pageSize int) ([]KillRow, int, error) {
	whereParts := []string{"realm = ?", "has(players.guid, ?)"}
	whereArgs := []any{realmName, guid}

	if len(f.Bosses) > 0 {
		whereParts = append(whereParts, "boss_remote_id IN ("+sqlutil.Placeholders(len(f.Bosses))+")")
		for _, b := range f.Bosses {
			whereArgs = append(whereArgs, b)
		}
	}
	if len(f.Raids) > 0 {
		whereParts = append(whereParts, "raid_name IN ("+sqlutil.Placeholders(len(f.Raids))+")")
		for _, r := range f.Raids {
			whereArgs = append(whereArgs, r)
		}
	}
	if len(f.Difficulties) > 0 {
		whereParts = append(whereParts, "mode IN ("+sqlutil.Placeholders(len(f.Difficulties))+")")
		for _, m := range f.Difficulties {
			whereArgs = append(whereArgs, uint8(m))
		}
	}
	if len(f.Specs) > 0 {
		whereParts = append(whereParts, "players.talent_spec[indexOf(players.guid, ?)] IN ("+sqlutil.Placeholders(len(f.Specs))+")")
		whereArgs = append(whereArgs, guid)
		for _, s := range f.Specs {
			whereArgs = append(whereArgs, uint16(s))
		}
	}
	where := strings.Join(whereParts, " AND ")

	sortCol := validSortCols[f.SortBy]
	if sortCol == "" {
		sortCol = "kill_time"
	}
	dir := "DESC"
	if strings.EqualFold(f.SortDir, "asc") {
		dir = "ASC"
	}

	rowsQ := `
	WITH indexOf(players.guid, ?) AS idx
	SELECT remote_id, kill_time, boss_name, boss_remote_id, mode, length,
	       ` + metric.SQLUInt64(metric.DmgDoneIndexed) + ` AS dps,
	       ` + metric.SQLUInt64(metric.HealAbsorbIndexed) + ` AS hps,
	       ` + metric.SQLFloat64(metric.DmgDoneIndexed) + ` AS dps_rate,
	       ` + metric.SQLFloat64(metric.HealAbsorbIndexed) + ` AS hps_rate,
	       players.talent_spec[idx] AS spec,
	       toFloat32(players.avg_item_lvl[idx]) AS avg_item_lvl
	FROM boss_kill
	WHERE ` + where + `
	ORDER BY ` + sortCol + ` ` + dir + `, kill_time DESC
	LIMIT ? OFFSET ?
	`
	rowsArgs := append([]any{guid}, whereArgs...)
	rowsArgs = append(rowsArgs, pageSize, page*pageSize)
	rows, err := db.QueryContext(ctx, rowsQ, rowsArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []KillRow
	for rows.Next() {
		var (
			remoteID   string
			t          time.Time
			bossName   string
			bossID     uint32
			mode       uint8
			length     uint32
			dps        uint64
			hps        uint64
			dpsRate    float64
			hpsRate    float64
			spec       uint16
			avgItemLvl float32
		)
		if err := rows.Scan(&remoteID, &t, &bossName, &bossID, &mode, &length, &dps, &hps, &dpsRate, &hpsRate, &spec, &avgItemLvl); err != nil {
			return nil, 0, err
		}
		s := int(spec)
		out = append(out, KillRow{
			RemoteID:   remoteID,
			KillAt:     t,
			KillTime:   t.Format("2006-01-02 15:04"),
			BossName:   bossName,
			BossID:     bossID,
			Mode:       int(mode),
			ModeLabel:  wow.Difficulty(expansion, int(mode)),
			Class:      wow.ClassFromSpecForExpansion(expansion, s),
			Spec:       s,
			SpecLabel:  wow.SpecForExpansion(expansion, s),
			DPS:        int64(dps),
			HPS:        int64(hps),
			DPSRate:    dpsRate,
			HPSRate:    hpsRate,
			LengthSec:  int(length) / 1000,
			AvgItemLvl: avgItemLvl,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var total uint64
	countQ := "SELECT count() FROM boss_kill WHERE " + where
	if err := db.QueryRowContext(ctx, countQ, whereArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if err := attachPerformanceTrends(ctx, db, realmName, guid, expansion, out); err != nil {
		return nil, 0, err
	}
	return out, int(total), nil
}

func attachPerformanceTrends(ctx context.Context, db *sql.DB, realmName string, guid uint64, expansion int, rows []KillRow) error {
	difficulties := wow.PerformanceDifficulties(expansion)
	if len(difficulties) == 0 || len(rows) == 0 {
		return nil
	}
	performanceMode := make(map[int]bool, len(difficulties))
	for _, difficulty := range difficulties {
		performanceMode[difficulty] = true
	}
	for i := range rows {
		if !performanceMode[rows[i].Mode] || rows[i].Spec <= 0 || rows[i].KillAt.IsZero() {
			continue
		}
		prevDPS, prevHPS, ok, err := loadPreviousPerformanceRates(ctx, db, realmName, guid, rows[i])
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		rows[i].HasTrend = true
		rows[i].DPSDelta = trendPercent(rows[i].DPSRate, prevDPS)
		rows[i].HPSDelta = trendPercent(rows[i].HPSRate, prevHPS)
	}
	return nil
}

func loadPreviousPerformanceRates(ctx context.Context, db *sql.DB, realmName string, guid uint64, current KillRow) (float64, float64, bool, error) {
	q := `
	WITH indexOf(players.guid, ?) AS idx
	SELECT
	       ` + metric.SQLFloat64(metric.DmgDoneIndexed) + ` AS dps_rate,
	       ` + metric.SQLFloat64(metric.HealAbsorbIndexed) + ` AS hps_rate
	FROM boss_kill
	WHERE realm = ?
	  AND has(players.guid, ?)
	  AND boss_remote_id = ?
	  AND mode = ?
	  AND kill_time < ?
	  AND players.talent_spec[idx] = ?
	ORDER BY kill_time DESC
	LIMIT 1
	`
	var dpsRate, hpsRate float64
	err := db.QueryRowContext(
		ctx,
		q,
		guid,
		realmName,
		guid,
		current.BossID,
		uint8(current.Mode),
		current.KillAt,
		uint16(current.Spec),
	).Scan(&dpsRate, &hpsRate)
	if err == sql.ErrNoRows {
		return 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, false, err
	}
	return dpsRate, hpsRate, true, nil
}

func trendPercent(current, previous float64) float64 {
	if previous <= 0 {
		return 0
	}
	v := math.Round((10000*(current-previous))/previous) / 100
	if v == 0 {
		return 0
	}
	return v
}

// parseKillsFilter reads the URL params for the kills table.
func parseKillsFilter(q map[string][]string) KillsFilter {
	f := KillsFilter{SortBy: defaultSortBy, SortDir: defaultSortDir}
	for _, v := range q["boss"] {
		for _, part := range strings.Split(v, ",") {
			if n, err := strconv.ParseUint(strings.TrimSpace(part), 10, 32); err == nil {
				f.Bosses = append(f.Bosses, uint32(n))
			}
		}
	}
	for _, v := range q["raid"] {
		if v = strings.TrimSpace(v); v != "" {
			f.Raids = append(f.Raids, v)
		}
	}
	for _, v := range append(q["difficulty"], q["mode"]...) {
		for _, part := range strings.Split(v, ",") {
			if n, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
				f.Difficulties = append(f.Difficulties, n)
			}
		}
	}
	for _, v := range q["spec"] {
		for _, part := range strings.Split(v, ",") {
			if n, err := strconv.Atoi(strings.TrimSpace(part)); err == nil && n > 0 {
				f.Specs = append(f.Specs, n)
			}
		}
	}
	if sortRaw := query.First(q, "sort"); sortRaw != "" {
		if _, ok := validSortCols[sortRaw]; ok {
			f.SortBy = sortRaw
		}
	}
	if dirRaw := strings.ToLower(query.First(q, "dir")); dirRaw == "asc" || dirRaw == "desc" {
		f.SortDir = dirRaw
	}
	return f
}

// loadKillsFilterOptions returns the filter dropdown options limited to
// combinations that actually exist for this character.
func loadKillsFilterOptions(ctx context.Context, db *sql.DB, realmName string, guid uint64, expansion int, f KillsFilter) (bossOpts, raidOpts, modeOpts, specOpts []Option, err error) {
	// Bosses played by this character.
	const bossQ = `
		SELECT boss_remote_id, any(boss_name) AS name
		FROM boss_kill
		WHERE realm = ? AND has(players.guid, ?)
		GROUP BY boss_remote_id
		ORDER BY name
	`
	brows, err := db.QueryContext(ctx, bossQ, realmName, guid)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	bossSel := map[uint32]bool{}
	for _, b := range f.Bosses {
		bossSel[b] = true
	}
	for brows.Next() {
		var id uint32
		var name string
		if err := brows.Scan(&id, &name); err != nil {
			brows.Close()
			return nil, nil, nil, nil, err
		}
		bossOpts = append(bossOpts, Option{
			Value:    strconv.FormatUint(uint64(id), 10),
			Label:    name,
			Selected: bossSel[id],
		})
	}
	brows.Close()

	// Raids + modes + specs the character has appeared in.
	const rmsQ = `
		SELECT DISTINCT raid_name, mode, players.talent_spec[indexOf(players.guid, ?)] AS spec
		FROM boss_kill
		WHERE realm = ? AND has(players.guid, ?)
	`
	rmrows, err := db.QueryContext(ctx, rmsQ, guid, realmName, guid)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	defer rmrows.Close()
	raidSet := map[string]bool{}
	modeSet := map[int]bool{}
	specSet := map[int]bool{}
	for rmrows.Next() {
		var raid string
		var mode uint8
		var spec uint16
		if err := rmrows.Scan(&raid, &mode, &spec); err != nil {
			return nil, nil, nil, nil, err
		}
		if raid != "" {
			raidSet[raid] = true
		}
		modeSet[int(mode)] = true
		if spec > 0 {
			specSet[int(spec)] = true
		}
	}
	if err := rmrows.Err(); err != nil {
		return nil, nil, nil, nil, err
	}

	raidSel := map[string]bool{}
	for _, r := range f.Raids {
		raidSel[r] = true
	}
	raids := make([]string, 0, len(raidSet))
	for r := range raidSet {
		raids = append(raids, r)
	}
	sort.Strings(raids)
	for _, r := range raids {
		raidOpts = append(raidOpts, Option{Value: r, Label: r, Selected: raidSel[r]})
	}

	modeSel := map[int]bool{}
	for _, m := range f.Difficulties {
		modeSel[m] = true
	}
	modes := make([]int, 0, len(modeSet))
	for m := range modeSet {
		modes = append(modes, m)
	}
	sort.Ints(modes)
	for _, m := range modes {
		modeOpts = append(modeOpts, Option{
			Value:    strconv.Itoa(m),
			Label:    wow.Difficulty(expansion, m),
			Selected: modeSel[m],
		})
	}

	specSel := map[int]bool{}
	for _, s := range f.Specs {
		specSel[s] = true
	}
	specs := make([]int, 0, len(specSet))
	for s := range specSet {
		specs = append(specs, s)
	}
	sort.Ints(specs)
	for _, s := range specs {
		specOpts = append(specOpts, Option{
			Value:    strconv.Itoa(s),
			Label:    wow.SpecForExpansion(expansion, s),
			Selected: specSel[s],
		})
	}
	return bossOpts, raidOpts, modeOpts, specOpts, nil
}

func loadBossMeta(ctx context.Context, db *sql.DB, realmName string, ids []uint32) (map[uint32]bossMeta, error) {
	out := map[uint32]bossMeta{}
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(ids)+1)
	args = append(args, realmName)
	ph := make([]byte, 0, len(ids)*2)
	for i, id := range ids {
		if i > 0 {
			ph = append(ph, ',')
		}
		ph = append(ph, '?')
		args = append(args, id)
	}
	q := "SELECT remote_id, name, position FROM boss FINAL " +
		"WHERE realm = ? AND remote_id IN (" + string(ph) + ")"
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uint32
		var name string
		var position uint16
		if err := rows.Scan(&id, &name, &position); err != nil {
			return nil, err
		}
		out[id] = bossMeta{Name: name, Position: position}
	}
	return out, rows.Err()
}

func Mount(mux *http.ServeMux, deps Deps) {
	h := middleware.RequireRealm(Handler(deps))
	rh := middleware.RequireRealm(RankingsHandler(deps))
	ah := middleware.RequireRealm(ActivityHandler(deps))
	sh := middleware.RequireRealm(StatsHandler(deps))
	router.ForEachRealmPrefix(func(prefix string) {
		mux.Handle("GET "+prefix+"/character/{name}", h)
		mux.Handle("GET "+prefix+"/character/{name}/rankings", rh)
		mux.Handle("GET "+prefix+"/character/{name}/activity", ah)
		mux.Handle("GET "+prefix+"/character/{name}/stats", sh)
	})
}
