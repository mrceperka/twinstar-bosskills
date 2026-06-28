package bosshistory

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

const (
	topLimit       = 50
	lockOptionsLen = 8
)

func Handler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		realmName := middleware.Realm(r.Context())
		if !realm.IsPublic(realmName) {
			http.Error(w, "raid-lock history is not exposed for private realms", http.StatusForbidden)
			return
		}

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
		mode := pickDefaultMode(expansion)
		if v, err := strconv.Atoi(r.URL.Query().Get("mode")); err == nil && v >= 0 {
			mode = v
		}
		offset := 0
		if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && v >= 0 {
			offset = v
		}

		info, err := loadBossInfo(ctx, deps.DB, realmName, bossID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if info.Name == "" {
			http.NotFound(w, r)
			return
		}

		now := time.Now().UTC()
		win := domain.RaidLock(now, offset)

		topDPS, topHPS, err := loadRanks(ctx, deps.DB, realmName, bossID, mode, win.Start)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		vm := ViewModel{
			Meta: layouts.PageMeta{
				Title:   info.Name + " history — " + realmName,
				Realm:   realmName,
				CSSHash: deps.CSSHash,
				JSHash:  deps.JSHash,
			},
			Realm:        realmName,
			Boss:         info,
			Difficulties: buildDifficulties(expansion),
			SelectedMode: mode,
			Lock:         lockLabel(now, offset, win),
			LockOptions:  buildLockOptions(now, lockOptionsLen),
			TopDPS:       topDPS,
			TopHPS:       topHPS,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = Page(vm).Render(r.Context(), w)
	}
}

func pickDefaultMode(expansion int) int {
	switch expansion {
	case realm.ExpansionVanilla:
		return 0
	default:
		return 5
	}
}

func buildDifficulties(expansion int) []DifficultyChoice {
	var modes []int
	switch expansion {
	case realm.ExpansionVanilla:
		modes = []int{0, 3, 4, 9}
	default:
		modes = []int{3, 4, 5, 6, 7}
	}
	out := make([]DifficultyChoice, 0, len(modes))
	for _, m := range modes {
		out = append(out, DifficultyChoice{Mode: m, Label: wow.Difficulty(expansion, m)})
	}
	return out
}

func buildLockOptions(now time.Time, n int) []LockChoice {
	out := make([]LockChoice, 0, n)
	for i := 0; i < n; i++ {
		w := domain.RaidLock(now, i)
		label := w.Start.Format("Jan 2") + " — " + w.End.Format("Jan 2")
		if i == 0 {
			label = "Current (" + label + ")"
		}
		out = append(out, LockChoice{Offset: i, Label: label})
	}
	return out
}

func lockLabel(now time.Time, offset int, win domain.RaidLockWindow) LockChoice {
	label := win.Start.Format("Jan 2 06:00 UTC") + " → " + win.End.Format("Jan 2 06:00 UTC")
	return LockChoice{Offset: offset, Label: label}
}

func loadBossInfo(ctx context.Context, db *sql.DB, realmName string, id uint32) (BossInfo, error) {
	const q = `
		SELECT any(boss_name), any(raid_name)
		FROM boss_kill
		WHERE realm = ? AND boss_remote_id = ?
	`
	var name, raidName string
	if err := db.QueryRowContext(ctx, q, realmName, id).Scan(&name, &raidName); err != nil {
		if err == sql.ErrNoRows {
			return BossInfo{}, nil
		}
		return BossInfo{}, err
	}
	return BossInfo{RemoteID: id, Name: name, RaidName: raidName}, nil
}

// loadRanks aggregates raid_lock_rankings for one boss + lock + mode and
// joins character identities in a follow-up query.
func loadRanks(ctx context.Context, db *sql.DB, realmName string, id uint32, mode int, lockStart time.Time) (
	[]Row, []Row, error,
) {
	const q = `
		SELECT talent_spec, guid,
		       maxMerge(dps_state) AS dps,
		       maxMerge(hps_state) AS hps
		FROM raid_lock_rankings
		WHERE realm = ?
		  AND raid_lock = toDate(?)
		  AND boss_remote_id = ?
		  AND mode = ?
		GROUP BY realm, raid_lock, boss_remote_id, mode, talent_spec, guid
	`
	rows, err := db.QueryContext(ctx, q, realmName, lockStart, id, uint8(mode))
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	type raw struct {
		Spec uint16
		GUID uint64
		DPS  uint64
		HPS  uint64
	}
	var all []raw
	guidSet := map[uint64]bool{}
	for rows.Next() {
		var r raw
		if err := rows.Scan(&r.Spec, &r.GUID, &r.DPS, &r.HPS); err != nil {
			return nil, nil, err
		}
		all = append(all, r)
		guidSet[r.GUID] = true
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	names, err := loadNames(ctx, db, realmName, mapKeys(guidSet))
	if err != nil {
		return nil, nil, err
	}

	make := func(metric string) []Row {
		sorted := append([]raw{}, all...)
		sort.Slice(sorted, func(i, j int) bool {
			if metric == "dps" {
				return sorted[i].DPS > sorted[j].DPS
			}
			return sorted[i].HPS > sorted[j].HPS
		})
		seen := map[uint64]bool{}
		out := make([]Row, 0, topLimit)
		for _, r := range sorted {
			if metric == "dps" && r.DPS == 0 {
				continue
			}
			if metric == "hps" && r.HPS == 0 {
				continue
			}
			if seen[r.GUID] {
				continue
			}
			seen[r.GUID] = true
			name := names[r.GUID]
			if name == "" {
				name = "Unknown"
			}
			out = append(out, Row{
				Rank:      len(out) + 1,
				Name:      name,
				Class:     wow.ClassFromSpec(int(r.Spec)),
				Spec:      int(r.Spec),
				SpecLabel: wow.Spec(int(r.Spec)),
				DPS:       int64(r.DPS),
				HPS:       int64(r.HPS),
			})
			if len(out) >= topLimit {
				break
			}
		}
		return out
	}
	return make("dps"), make("hps"), nil
}

func loadNames(ctx context.Context, db *sql.DB, realmName string, guids []uint64) (map[uint64]string, error) {
	out := map[uint64]string{}
	if len(guids) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(guids)+1)
	args = append(args, realmName)
	ph := make([]byte, 0, len(guids)*2)
	for i, g := range guids {
		if i > 0 {
			ph = append(ph, ',')
		}
		ph = append(ph, '?')
		args = append(args, g)
	}
	q := "SELECT guid, argMaxMerge(name_state) FROM character " +
		"WHERE realm = ? AND guid IN (" + string(ph) + ") " +
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

func mapKeys(m map[uint64]bool) []uint64 {
	out := make([]uint64, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func Mount(mux *http.ServeMux, deps Deps) {
	h := middleware.RequireRealm(Handler(deps))
	router.ForEachRealmPrefix(func(prefix string) {
		mux.Handle("GET "+prefix+"/boss/{id}/history", h)
	})
}
