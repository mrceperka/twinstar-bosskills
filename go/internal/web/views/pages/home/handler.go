package home

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/mrceperka/twinstar-bosskills/go/internal/realm"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/views/layouts"
)

// Deps is what the handler factory needs from the server.
type Deps struct {
	DB      *sql.DB
	CSSHash string
	JSHash  string
}

// Handler returns the HTTP handler for the home page.
func Handler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		realms, err := loadRealmStats(ctx, deps.DB)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		vm := ViewModel{
			Meta: layouts.PageMeta{
				Title:   "Home",
				CSSHash: deps.CSSHash,
				JSHash:  deps.JSHash,
			},
			Realms: realms,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = Page(vm).Render(r.Context(), w)
	}
}

func loadRealmStats(ctx context.Context, db *sql.DB) ([]RealmStat, error) {
	// Aggregate per-realm boss/kill counts directly from the events table.
	const q = `
		SELECT realm,
		       count(DISTINCT boss_remote_id) AS bosses,
		       count() AS kills,
		       max(kill_time) AS last_kill
		FROM boss_kill
		GROUP BY realm
	`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := map[string]RealmStat{}
	for rows.Next() {
		var (
			name      string
			bosses    uint64
			kills     uint64
			lastKill  time.Time
		)
		if err := rows.Scan(&name, &bosses, &kills, &lastKill); err != nil {
			return nil, err
		}
		stats[name] = RealmStat{
			Name:       name,
			Expansion:  realm.Expansion(name),
			Bosses:     int(bosses),
			BossKills:  int(kills),
			IsPrivate:  !realm.IsPublic(name),
			LastKillAt: lastKill.Format(time.RFC3339),
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Always include every known realm, even if no kills have been synced yet.
	out := make([]RealmStat, 0, len(realm.All()))
	for _, name := range realm.All() {
		if r, ok := stats[name]; ok {
			out = append(out, r)
			continue
		}
		out = append(out, RealmStat{
			Name:       name,
			Expansion:  realm.Expansion(name),
			IsPrivate:  !realm.IsPublic(name),
			LastKillAt: "-",
		})
	}
	return out, nil
}
