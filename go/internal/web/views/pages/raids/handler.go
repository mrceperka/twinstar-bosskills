package raids

import (
	"context"
	"database/sql"
	"net/http"
	"sort"
	"time"

	"twinstar-bosskills/internal/domain"
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

func Handler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		realmName := middleware.Realm(r.Context())
		if middleware.GatePrivateRealm(w, r) {
			return
		}
		guildFilter := middleware.PrivateRealmGuildFilter(r)

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		// Default to current raid lock; accept Node's ?raidlock=N and the
		// older Go ?offset=N alias for previous locks.
		offset := 0
		if n, ok := query.RaidLock(r.URL.Query()); ok {
			offset = n
		}
		win := domain.RaidLock(time.Now().UTC(), offset)

		raidList, difficulties, err := loadRaids(ctx, deps.DB, realmName, guildFilter, win)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		vm := ViewModel{
			Meta: layouts.PageMeta{
				Title:      realmName + " / Raids",
				Realm:      realmName,
				ActivePath: "/raids",
				CSSHash:    deps.CSSHash,
				JSHash:     deps.JSHash,
			},
			Realm: realmName,
			Lock: LockLabel{
				StartLabel: win.Start.Format("Jan 2 06:00 UTC"),
				EndLabel:   win.End.Format("Jan 2 06:00 UTC"),
				Offset:     offset,
			},
			Raids:        raidList,
			Difficulties: difficulties,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = Page(vm).Render(r.Context(), w)
	}
}

// loadRaids returns every known boss with a kill count per difficulty for
// the selected lock. Bosses with no kills are kept visible.
func loadRaids(ctx context.Context, db *sql.DB, realmName, guildFilter string, win domain.RaidLockWindow) ([]Raid, []DifficultyChoice, error) {
	const bossQ = `
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
	`
	bossRows, err := db.QueryContext(ctx, bossQ, realmName, realmName)
	if err != nil {
		return nil, nil, err
	}
	defer bossRows.Close()

	type bossKey struct {
		Raid     string
		RemoteID uint32
	}
	type bossAgg struct {
		Name         string
		BossPosition uint16
		RaidPosition uint16
		KillsByMode  map[int]int
	}

	bosses := map[bossKey]*bossAgg{}

	for bossRows.Next() {
		var (
			raidName string
			remoteID uint32
			bossName string
			bossPos  uint16
			raidPos  uint16
		)
		if err := bossRows.Scan(&raidName, &remoteID, &bossName, &bossPos, &raidPos); err != nil {
			return nil, nil, err
		}
		k := bossKey{Raid: raidName, RemoteID: remoteID}
		bosses[k] = &bossAgg{
			Name:         bossName,
			BossPosition: bossPos,
			RaidPosition: raidPos,
			KillsByMode:  map[int]int{},
		}
	}
	if err := bossRows.Err(); err != nil {
		return nil, nil, err
	}

	guildWhere, guildArgs := privateGuildWhere(guildFilter)
	killQ := `
		SELECT raid_name,
		       boss_remote_id,
		       any(boss_name) AS boss_name,
		       mode,
		       count() AS kills
		FROM boss_kill
		WHERE realm = ? AND kill_time >= ? AND kill_time < ?` + guildWhere + `
		GROUP BY raid_name, boss_remote_id, mode
	`
	killArgs := append([]any{realmName, win.Start, win.End}, guildArgs...)
	rows, err := db.QueryContext(ctx, killQ, killArgs...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			raidName string
			remoteID uint32
			bossName string
			mode     uint8
			kills    uint64
		)
		if err := rows.Scan(&raidName, &remoteID, &bossName, &mode, &kills); err != nil {
			return nil, nil, err
		}
		k := bossKey{Raid: raidName, RemoteID: remoteID}
		agg, ok := bosses[k]
		if !ok {
			agg = &bossAgg{Name: bossName, KillsByMode: map[int]int{}}
			bosses[k] = agg
		}
		agg.KillsByMode[int(mode)] = int(kills)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	exp := realm.Expansion(realmName)
	var modes []int
	var difficulties []DifficultyChoice
	if !realm.IsVanilla(exp) {
		modes = wow.RaidDifficulties(exp)
		sort.Ints(modes)
		difficulties = make([]DifficultyChoice, len(modes))
		for i, m := range modes {
			difficulties[i] = DifficultyChoice{Mode: m, Label: wow.Difficulty(exp, m)}
		}
	}

	// Group bosses by raid; sort raids by total kills desc; bosses by remote_id.
	type raidAgg struct {
		Name       string
		Position   uint16
		Bosses     []BossRow
		TotalKills int
	}
	raidMap := map[string]*raidAgg{}
	for k, agg := range bosses {
		ra, ok := raidMap[k.Raid]
		if !ok {
			ra = &raidAgg{Name: k.Raid, Position: agg.RaidPosition}
			raidMap[k.Raid] = ra
		}
		if ra.Position == 0 {
			ra.Position = agg.RaidPosition
		}
		row := BossRow{
			Name:              agg.Name,
			RemoteID:          k.RemoteID,
			KillsByDifficulty: make(map[int]int, len(modes)),
		}
		if len(modes) == 0 {
			for _, c := range agg.KillsByMode {
				row.Total += c
			}
		} else {
			for _, m := range modes {
				c := agg.KillsByMode[m]
				row.KillsByDifficulty[m] = c
				row.Total += c
			}
		}
		ra.Bosses = append(ra.Bosses, row)
		ra.TotalKills += row.Total
	}

	out := make([]Raid, 0, len(raidMap))
	for _, ra := range raidMap {
		sort.Slice(ra.Bosses, func(i, j int) bool {
			oi := bosses[bossKey{Raid: ra.Name, RemoteID: ra.Bosses[i].RemoteID}].BossPosition
			oj := bosses[bossKey{Raid: ra.Name, RemoteID: ra.Bosses[j].RemoteID}].BossPosition
			if oi != oj {
				return oi < oj
			}
			return ra.Bosses[i].RemoteID < ra.Bosses[j].RemoteID
		})
		out = append(out, Raid{Name: ra.Name, Bosses: ra.Bosses})
	}
	sort.Slice(out, func(i, j int) bool {
		pi := raidMap[out[i].Name].Position
		pj := raidMap[out[j].Name].Position
		// Unknowns (position=0) sort after all known raids.
		if pi == 0 && pj == 0 {
			return out[i].Name < out[j].Name
		}
		if pi == 0 {
			return false
		}
		if pj == 0 {
			return true
		}
		if pi != pj {
			return pi < pj // oldest raid first (MSV → HoF → ToES → ToT → SoO)
		}
		return out[i].Name < out[j].Name
	})

	return out, difficulties, nil
}

func privateGuildWhere(guildFilter string) (string, []any) {
	if guildFilter == "" {
		return "", nil
	}
	return " AND guild = ?", []any{guildFilter}
}

func Mount(mux *http.ServeMux, deps Deps) {
	h := middleware.RequireRealm(Handler(deps))
	router.ForEachRealmPrefix(func(prefix string) {
		mux.Handle("GET "+prefix+"/raids", h)
	})
}
