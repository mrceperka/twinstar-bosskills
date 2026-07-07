package characters

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/mrceperka/twinstar-bosskills/go/internal/web/middleware"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/router"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/views/layouts"
	"github.com/mrceperka/twinstar-bosskills/go/internal/wow"
)

type Deps struct {
	DB      *sql.DB
	CSSHash string
	JSHash  string
}

const searchLimit = 25

func Handler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		realmName := middleware.Realm(r.Context())
		if middleware.GatePrivateRealm(w, r) {
			return
		}
		guildFilter := middleware.PrivateRealmGuildFilter(r)
		q := strings.TrimSpace(r.URL.Query().Get("q"))

		ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
		defer cancel()

		var matches []Match
		var err error
		if q != "" && len(q) >= 2 {
			matches, err = search(ctx, deps.DB, realmName, guildFilter, q)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		vm := ViewModel{
			Meta: layouts.PageMeta{
				Title:      realmName + " / Characters",
				Realm:      realmName,
				ActivePath: "/characters",
				CSSHash:    deps.CSSHash,
				JSHash:     deps.JSHash,
			},
			Realm:   realmName,
			Query:   q,
			Matches: matches,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if middleware.IsHTMX(r) {
			_ = Matches(vm).Render(r.Context(), w)
			return
		}
		_ = Page(vm).Render(r.Context(), w)
	}
}

// search runs a case-insensitive prefix-match across the `character` MV.
//
// Prefix search uses startsWith because we want "Sk" -> "Skrassham", not
// substring matches across the whole identity. If you want broader matches,
// swap for positionCaseInsensitive(name, q) > 0.
func search(ctx context.Context, db *sql.DB, realmName, guildFilter, q string) ([]Match, error) {
	args := []any{realmName}
	guildFilterSQL := ""
	if guildFilter != "" {
		guildFilterSQL = `
		  AND guid IN (
			  SELECT players.guid
			  FROM boss_kill ARRAY JOIN players
			  WHERE realm = ? AND guild = ?
			  GROUP BY players.guid
		  )`
		args = append(args, realmName, guildFilter)
	}
	args = append(args, q, searchLimit)
	sqlText := `
		SELECT guid,
		       argMaxMerge(name_state)   AS name,
		       argMaxMerge(class_state)  AS class,
		       argMaxMerge(level_state)  AS level,
		       sumMerge(kill_count_state) AS kills,
		       maxMerge(last_seen_state)  AS last_seen
		FROM character
		WHERE realm = ?` + guildFilterSQL + `
		GROUP BY realm, guid
		HAVING startsWith(lower(name), lower(?))
		ORDER BY kills DESC
		LIMIT ?
	`
	rows, err := db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Match
	for rows.Next() {
		var (
			guid     uint64
			name     string
			class    uint8
			level    uint8
			kills    uint64
			lastSeen time.Time
		)
		if err := rows.Scan(&guid, &name, &class, &level, &kills, &lastSeen); err != nil {
			return nil, err
		}
		out = append(out, Match{
			Name:       name,
			Class:      int(class),
			ClassLabel: wow.Class(int(class)),
			Level:      int(level),
			KillCount:  int(kills),
			LastSeen:   lastSeen.Format("2006-01-02"),
		})
	}
	return out, rows.Err()
}

func Mount(mux *http.ServeMux, deps Deps) {
	h := middleware.RequireRealm(Handler(deps))
	router.ForEachRealmPrefix(func(prefix string) {
		mux.Handle("GET "+prefix+"/characters", h)
		mux.Handle("GET "+prefix+"/characters/{$}", h)
	})
}
