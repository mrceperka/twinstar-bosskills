package stats

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"twinstar-bosskills/internal/domain"
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

type statsWhere struct {
	sql  string
	args []any
}

func Handler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		realmName := middleware.Realm(r.Context())
		if middleware.GatePrivateRealm(w, r) {
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()

		expansion := realm.Expansion(realmName)
		offset := 0
		if v, ok := query.RaidLock(r.URL.Query()); ok {
			offset = v
		}
		selectedModes := parseDifficulties(r.URL.Query())
		canFilterDifficulty := expansion != realm.ExpansionVanilla
		if !canFilterDifficulty {
			selectedModes = nil
		}

		now := time.Now().UTC()
		win := domain.RaidLock(now, offset)
		guildFilter := middleware.PrivateRealmGuildFilter(r)

		summary, err := loadSummary(ctx, deps.DB, realmName, guildFilter, win.Start, win.End, selectedModes)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		classRows, err := loadClassBreakdown(ctx, deps.DB, realmName, guildFilter, win.Start, win.End, selectedModes)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		specRows, err := loadSpecBreakdown(ctx, deps.DB, realmName, guildFilter, win.Start, win.End, selectedModes, expansion)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var difficultyRows []DifficultyRow
		if canFilterDifficulty {
			difficultyRows, err = loadDifficultyBreakdown(ctx, deps.DB, realmName, guildFilter, win.Start, win.End, expansion)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		guildRows, err := loadGuildBreakdown(ctx, deps.DB, realmName, guildFilter, win.Start, win.End, selectedModes)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		vm := ViewModel{
			Meta: layouts.PageMeta{
				Title:      realmName + " / Stats",
				Realm:      realmName,
				ActivePath: "/stats",
				CSSHash:    deps.CSSHash,
				JSHash:     deps.JSHash,
			},
			Realm:               realmName,
			LockLabel:           win.Start.Format("Jan 2 15:04 UTC") + " -> " + win.End.Format("Jan 2 15:04 UTC"),
			LockOffset:          offset,
			SelectedModes:       selectedModes,
			SelectedModeLabel:   selectedModeLabel(expansion, selectedModes),
			CanFilterDifficulty: canFilterDifficulty,
			Difficulties:        buildDifficulties(expansion, selectedModes),
			Summary:             summary,
			ClassRows:           classRows,
			SpecRows:            specRows,
			DifficultyRows:      markSelectedDifficulty(difficultyRows, selectedModes),
			GuildRows:           guildRows,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if middleware.IsHTMX(r) {
			_ = StatsShell(vm).Render(r.Context(), w)
			return
		}
		_ = Page(vm).Render(r.Context(), w)
	}
}

func parseDifficulties(q map[string][]string) []int {
	seen := map[int]bool{}
	var out []int
	for _, v := range append(q["difficulty"], q["mode"]...) {
		for _, part := range strings.Split(v, ",") {
			n, err := strconv.Atoi(strings.TrimSpace(part))
			if err == nil && n >= 0 && !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
	}
	return out
}

func selectedModeLabel(expansion int, modes []int) string {
	if len(modes) == 0 {
		if expansion == realm.ExpansionVanilla {
			return "All kills"
		}
		return "All difficulties"
	}
	if len(modes) == 1 {
		return wow.Difficulty(expansion, modes[0])
	}
	return strconv.Itoa(len(modes)) + " difficulties"
}

func buildDifficulties(expansion int, selected []int) []DifficultyChoice {
	modes := wow.RaidDifficulties(expansion)
	sel := map[int]bool{}
	for _, m := range selected {
		sel[m] = true
	}
	out := make([]DifficultyChoice, 0, len(modes))
	for _, m := range modes {
		out = append(out, DifficultyChoice{
			Mode:     m,
			Label:    wow.Difficulty(expansion, m),
			Selected: sel[m],
		})
	}
	return out
}

func markSelectedDifficulty(rows []DifficultyRow, selected []int) []DifficultyRow {
	sel := map[int]bool{}
	for _, m := range selected {
		sel[m] = true
	}
	for i := range rows {
		rows[i].Selected = sel[rows[i].Mode]
	}
	return rows
}

func filteredWhere(realmName, guildFilter string, start, end time.Time, modes []int) statsWhere {
	where := statsWhere{
		sql:  "realm = ? AND kill_time >= ? AND kill_time < ?",
		args: []any{realmName, start, end},
	}
	if len(modes) > 0 {
		where.sql += " AND mode IN (" + sqlutil.Placeholders(len(modes)) + ")"
		for _, mode := range modes {
			where.args = append(where.args, uint8(mode))
		}
	}
	if guildFilter != "" {
		where.sql += " AND guild = ?"
		where.args = append(where.args, guildFilter)
	}
	return where
}

func loadSummary(ctx context.Context, db *sql.DB, realmName, guildFilter string, start, end time.Time, modes []int) (Summary, error) {
	where := filteredWhere(realmName, guildFilter, start, end, modes)
	q := `
		SELECT
			uniqExact(guid) AS characters,
			uniqExact(remote_id) AS kills,
			uniqExact(guild) AS guilds,
			uniqExact(boss_remote_id) AS bosses
		FROM (
			SELECT remote_id, guild, boss_remote_id, players.guid AS player_guid
			FROM boss_kill
			WHERE ` + where.sql + `
		)
		ARRAY JOIN player_guid AS guid
	`
	var (
		characters, kills, guilds, bosses uint64
	)
	if err := db.QueryRowContext(ctx, q, where.args...).Scan(&characters, &kills, &guilds, &bosses); err != nil {
		return Summary{}, err
	}
	return Summary{
		Characters: int(characters),
		Kills:      int(kills),
		Guilds:     int(guilds),
		Bosses:     int(bosses),
	}, nil
}

func loadClassBreakdown(ctx context.Context, db *sql.DB, realmName, guildFilter string, start, end time.Time, modes []int) ([]ClassRow, error) {
	where := filteredWhere(realmName, guildFilter, start, end, modes)
	q := `
		SELECT
			class,
			uniqExact(guid) AS characters,
			uniqExact(remote_id) AS kills
		FROM (
			SELECT remote_id, players.guid AS player_guid, players.class AS player_class
			FROM boss_kill
			WHERE ` + where.sql + `
		)
		ARRAY JOIN player_guid AS guid, player_class AS class
		GROUP BY class
		ORDER BY characters DESC, class ASC
	`
	rows, err := db.QueryContext(ctx, q, where.args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ClassRow
	for rows.Next() {
		var class uint8
		var characters, kills uint64
		if err := rows.Scan(&class, &characters, &kills); err != nil {
			return nil, err
		}
		out = append(out, ClassRow{
			Class:      int(class),
			ClassLabel: wow.Class(int(class)),
			Characters: int(characters),
			Kills:      int(kills),
		})
	}
	return out, rows.Err()
}

func loadSpecBreakdown(ctx context.Context, db *sql.DB, realmName, guildFilter string, start, end time.Time, modes []int, expansion int) ([]SpecRow, error) {
	where := filteredWhere(realmName, guildFilter, start, end, modes)
	q := `
		SELECT
			spec,
			uniqExact(guid) AS characters,
			uniqExact(remote_id) AS kills
		FROM (
			SELECT remote_id, players.guid AS player_guid, players.talent_spec AS player_spec
			FROM boss_kill
			WHERE ` + where.sql + `
		)
		ARRAY JOIN player_guid AS guid, player_spec AS spec
		GROUP BY spec
		ORDER BY characters DESC, spec ASC
	`
	rows, err := db.QueryContext(ctx, q, where.args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SpecRow
	for rows.Next() {
		var spec uint16
		var characters, kills uint64
		if err := rows.Scan(&spec, &characters, &kills); err != nil {
			return nil, err
		}
		specID := int(spec)
		out = append(out, SpecRow{
			Spec:       specID,
			SpecLabel:  wow.SpecForExpansion(expansion, specID),
			Class:      wow.ClassFromSpecForExpansion(expansion, specID),
			Characters: int(characters),
			Kills:      int(kills),
		})
	}
	return out, rows.Err()
}

func loadDifficultyBreakdown(ctx context.Context, db *sql.DB, realmName, guildFilter string, start, end time.Time, expansion int) ([]DifficultyRow, error) {
	where := filteredWhere(realmName, guildFilter, start, end, nil)
	q := `
		SELECT
			mode,
			uniqExact(guid) AS characters,
			uniqExact(remote_id) AS kills
		FROM (
			SELECT remote_id, mode, players.guid AS player_guid
			FROM boss_kill
			WHERE ` + where.sql + `
		)
		ARRAY JOIN player_guid AS guid
		GROUP BY mode
		ORDER BY mode ASC
	`
	rows, err := db.QueryContext(ctx, q, where.args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DifficultyRow
	for rows.Next() {
		var mode uint8
		var characters, kills uint64
		if err := rows.Scan(&mode, &characters, &kills); err != nil {
			return nil, err
		}
		out = append(out, DifficultyRow{
			Mode:       int(mode),
			Label:      wow.Difficulty(expansion, int(mode)),
			Characters: int(characters),
			Kills:      int(kills),
		})
	}
	return out, rows.Err()
}

func loadGuildBreakdown(ctx context.Context, db *sql.DB, realmName, guildFilter string, start, end time.Time, modes []int) ([]GuildRow, error) {
	where := filteredWhere(realmName, guildFilter, start, end, modes)
	q := `
		SELECT
			guild,
			uniqExact(guid) AS characters,
			uniqExact(remote_id) AS kills
		FROM (
			SELECT remote_id, guild, players.guid AS player_guid
			FROM boss_kill
			WHERE ` + where.sql + `
		)
		ARRAY JOIN player_guid AS guid
		GROUP BY guild
		ORDER BY characters DESC, kills DESC, guild ASC
		LIMIT 20
	`
	rows, err := db.QueryContext(ctx, q, where.args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GuildRow
	for rows.Next() {
		var guild string
		var characters, kills uint64
		if err := rows.Scan(&guild, &characters, &kills); err != nil {
			return nil, err
		}
		out = append(out, GuildRow{
			Guild:      guild,
			Characters: int(characters),
			Kills:      int(kills),
		})
	}
	return out, rows.Err()
}

func Mount(mux *http.ServeMux, deps Deps) {
	h := middleware.RequireRealm(Handler(deps))
	router.ForEachRealmPrefix(func(prefix string) {
		mux.Handle("GET "+prefix+"/stats", h)
		mux.Handle("GET "+prefix+"/stats/{$}", h)
	})
}
