package raids

import (
	"context"
	"database/sql"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/mrceperka/twinstar-bosskills/go/internal/domain"
	"github.com/mrceperka/twinstar-bosskills/go/internal/realm"
	"github.com/mrceperka/twinstar-bosskills/go/internal/wow"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/middleware"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/router"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/views/layouts"
)

type Deps struct {
	DB      *sql.DB
	CSSHash string
	JSHash  string
}

func Handler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		realmName := middleware.Realm(r.Context())

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		// Default to current raid lock; allow ?offset=N for previous locks.
		offset := 0
		if v := r.URL.Query().Get("offset"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				offset = n
			}
		}
		win := domain.RaidLock(time.Now().UTC(), offset)

		raidList, difficulties, err := loadRaids(ctx, deps.DB, realmName, win)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		vm := ViewModel{
			Meta: layouts.PageMeta{
				Title:   realmName + " / Raids",
				Realm:   realmName,
				CSSHash: deps.CSSHash,
				JSHash:  deps.JSHash,
			},
			Realm: realmName,
			Lock: LockLabel{
				StartLabel: win.Start.Format("Jan 2 06:00 UTC"),
				EndLabel:   win.End.Format("Jan 2 06:00 UTC"),
			},
			Raids:        raidList,
			Difficulties: difficulties,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = Page(vm).Render(r.Context(), w)
	}
}

// loadRaids returns one row per (raid_name, boss_remote_id) with a kill count
// per difficulty (mode). One round trip into CH.
func loadRaids(ctx context.Context, db *sql.DB, realmName string, win domain.RaidLockWindow) ([]Raid, []string, error) {
	const q = `
		SELECT raid_name,
		       boss_remote_id,
		       any(boss_name) AS boss_name,
		       mode,
		       count() AS kills
		FROM boss_kill
		WHERE realm = ? AND kill_time >= ? AND kill_time < ?
		GROUP BY raid_name, boss_remote_id, mode
	`
	rows, err := db.QueryContext(ctx, q, realmName, win.Start, win.End)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	type bossKey struct {
		Raid     string
		RemoteID uint32
	}
	type bossAgg struct {
		Name        string
		KillsByMode map[int]int
	}

	bosses := map[bossKey]*bossAgg{}
	modeSet := map[int]struct{}{}

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
		modeSet[int(mode)] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	modes := make([]int, 0, len(modeSet))
	for m := range modeSet {
		modes = append(modes, m)
	}
	sort.Ints(modes)
	exp := realm.Expansion(realmName)
	difficulties := make([]string, len(modes))
	for i, m := range modes {
		difficulties[i] = wow.Difficulty(exp, m)
	}

	// Group bosses by raid; sort raids by total kills desc; bosses by remote_id.
	type raidAgg struct {
		Name       string
		Bosses     []BossRow
		TotalKills int
	}
	raidMap := map[string]*raidAgg{}
	for k, agg := range bosses {
		ra, ok := raidMap[k.Raid]
		if !ok {
			ra = &raidAgg{Name: k.Raid}
			raidMap[k.Raid] = ra
		}
		row := BossRow{
			Name:              agg.Name,
			RemoteID:          k.RemoteID,
			KillsByDifficulty: make([]int, len(modes)),
		}
		for i, m := range modes {
			c := agg.KillsByMode[m]
			row.KillsByDifficulty[i] = c
			row.Total += c
		}
		ra.Bosses = append(ra.Bosses, row)
		ra.TotalKills += row.Total
	}

	out := make([]Raid, 0, len(raidMap))
	for _, ra := range raidMap {
		sort.Slice(ra.Bosses, func(i, j int) bool {
			oi := wow.BossOrder(ra.Bosses[i].RemoteID)
			oj := wow.BossOrder(ra.Bosses[j].RemoteID)
			if oi != oj {
				return oi < oj
			}
			return ra.Bosses[i].RemoteID < ra.Bosses[j].RemoteID
		})
		out = append(out, Raid{Name: ra.Name, Bosses: ra.Bosses})
	}
	sort.Slice(out, func(i, j int) bool {
		pi := wow.RaidOrder(out[i].Name)
		pj := wow.RaidOrder(out[j].Name)
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

func Mount(mux *http.ServeMux, deps Deps) {
	h := middleware.RequireRealm(Handler(deps))
	router.ForEachRealmPrefix(func(prefix string) {
		mux.Handle("GET "+prefix+"/raids", h)
	})
}
