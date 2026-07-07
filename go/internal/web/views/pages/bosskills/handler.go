package bosskills

import (
	"context"
	"database/sql"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mrceperka/twinstar-bosskills/go/internal/links"
	"github.com/mrceperka/twinstar-bosskills/go/internal/realm"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/middleware"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/router"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/sqlutil"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/views/layouts"
	"github.com/mrceperka/twinstar-bosskills/go/internal/wow"
)

type Deps struct {
	DB      *sql.DB
	CSSHash string
	JSHash  string
}

const (
	defaultPageSize = 20
	defaultBKSort   = "kill_time"
	defaultBKDir    = "desc"
)

// validBKSortCols is used both to whitelist ?sort= values and to translate
// them into safe ORDER BY column names.
var validBKSortCols = map[string]string{
	"kill_time": "kill_time",
	"length":    "length",
	"wipes":     "wipes",
	"deaths":    "deaths",
}

func Handler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		realmName := middleware.Realm(r.Context())
		if middleware.GatePrivateRealm(w, r) {
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
		defer cancel()

		expansion := realm.Expansion(realmName)
		q := r.URL.Query()
		filter := parseFilter(q)
		if expansion == realm.ExpansionVanilla {
			filter.Specs = nil
		} else {
			filter.Classes = nil
		}
		// Force guild filter on private realms even if user hasn't selected
		// one — the SvelteKit app applies the same constraint.
		if guild := middleware.PrivateRealmGuildFilter(r); guild != "" {
			filter.GuildOverride = guild
		}
		page := atoiOr(q.Get("page"), 0)
		if page < 0 {
			page = 0
		}
		pageSize := defaultPageSize

		rows, total, err := loadKills(ctx, deps.DB, realmName, filter, page, pageSize, expansion)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		bossOpts, raidOpts, err := loadFilterOptions(ctx, deps.DB, realmName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		markSelected(bossOpts, filter.Bosses, raidOpts, filter.Raids)

		modeOpts := buildModeOptions(rows, filter.Difficulties, expansion)
		var classOpts []Option
		var specOpts []Option
		if expansion == realm.ExpansionVanilla {
			classOpts = buildClassOptions(filter.Classes, expansion)
		} else {
			specOpts = buildSpecOptions(realmName, filter.Specs, expansion)
		}

		vm := ViewModel{
			Meta: layouts.PageMeta{
				Title:      realmName + " / Boss kills",
				Realm:      realmName,
				ActivePath: "/boss-kills",
				CSSHash:    deps.CSSHash,
				JSHash:     deps.JSHash,
			},
			Realm:        realmName,
			Filter:       filter,
			BossOptions:  bossOpts,
			RaidOptions:  raidOpts,
			ModeOptions:  modeOpts,
			ClassOptions: classOpts,
			SpecOptions:  specOpts,
			Rows:         rows,
			Page:         page,
			PageSize:     pageSize,
			Total:        total,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// htmx swap: render only the table partial.
		if middleware.IsHTMX(r) && q.Get("full") == "" {
			_ = TableFragment(vm).Render(r.Context(), w)
			return
		}
		_ = Page(vm).Render(r.Context(), w)
	}
}

func parseFilter(q map[string][]string) FilterValues {
	f := FilterValues{SortBy: defaultBKSort, SortDir: defaultBKDir}
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
	for _, v := range q["class"] {
		for _, part := range strings.Split(v, ",") {
			if n, err := strconv.Atoi(strings.TrimSpace(part)); err == nil && n > 0 && n <= 255 {
				f.Classes = append(f.Classes, n)
			}
		}
	}
	for _, v := range q["spec"] {
		for _, part := range strings.Split(v, ",") {
			if n, err := strconv.Atoi(strings.TrimSpace(part)); err == nil && n > 0 && n <= 65535 {
				f.Specs = append(f.Specs, n)
			}
		}
	}
	if v, ok := q["sort"]; ok && len(v) > 0 {
		if _, valid := validBKSortCols[v[0]]; valid {
			f.SortBy = v[0]
		}
	}
	if v, ok := q["dir"]; ok && len(v) > 0 {
		if d := strings.ToLower(v[0]); d == "asc" || d == "desc" {
			f.SortDir = d
		}
	}
	return f
}

func atoiOr(s string, fallback int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return fallback
}

// loadKills runs two queries: one for the page rows, one for total count.
func loadKills(ctx context.Context, db *sql.DB, realmName string, f FilterValues, page, pageSize, expansion int) ([]KillRow, int, error) {
	var whereParts []string
	var args []any
	whereParts = append(whereParts, "realm = ?")
	args = append(args, realmName)
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
	if len(f.Difficulties) > 0 {
		ph := sqlutil.Placeholders(len(f.Difficulties))
		whereParts = append(whereParts, "mode IN ("+ph+")")
		for _, m := range f.Difficulties {
			args = append(args, uint8(m))
		}
	}
	if expansion == realm.ExpansionVanilla && len(f.Classes) > 0 {
		whereParts = append(whereParts, hasAnyClause("players.class", len(f.Classes)))
		for _, c := range f.Classes {
			args = append(args, uint8(c))
		}
	}
	if expansion != realm.ExpansionVanilla && len(f.Specs) > 0 {
		whereParts = append(whereParts, hasAnyClause("players.talent_spec", len(f.Specs)))
		for _, s := range f.Specs {
			args = append(args, uint16(s))
		}
	}
	if f.GuildOverride != "" {
		whereParts = append(whereParts, "guild = ?")
		args = append(args, f.GuildOverride)
	}
	where := strings.Join(whereParts, " AND ")

	sortCol := validBKSortCols[f.SortBy]
	if sortCol == "" {
		sortCol = "kill_time"
	}
	dir := "DESC"
	if strings.EqualFold(f.SortDir, "asc") {
		dir = "ASC"
	}

	rowsQ := "SELECT remote_id, kill_time, boss_name, boss_remote_id, raid_name, mode, guild, length, wipes, deaths " +
		"FROM boss_kill WHERE " + where + " ORDER BY " + sortCol + " " + dir + ", kill_time DESC LIMIT ? OFFSET ?"
	rowsArgs := append(append([]any{}, args...), pageSize, page*pageSize)
	rows, err := db.QueryContext(ctx, rowsQ, rowsArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []KillRow
	for rows.Next() {
		var (
			remoteID, bossName, raidName, guild string
			killTime                            time.Time
			bossID                              uint32
			mode                                uint8
			length, wipes, deaths               uint32
		)
		if err := rows.Scan(&remoteID, &killTime, &bossName, &bossID, &raidName, &mode, &guild, &length, &wipes, &deaths); err != nil {
			return nil, 0, err
		}
		out = append(out, KillRow{
			RemoteID:  remoteID,
			KillTime:  killTime.Format("2006-01-02 15:04"),
			BossName:  bossName,
			BossID:    bossID,
			RaidName:  raidName,
			Mode:      int(mode),
			ModeLabel: wow.Difficulty(expansion, int(mode)),
			Guild:     guild,
			LengthSec: int(length) / 1000,
			Wipes:     int(wipes),
			Deaths:    int(deaths),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	countQ := "SELECT count() FROM boss_kill WHERE " + where
	var total uint64
	if err := db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	return out, int(total), nil
}

func hasAnyClause(column string, count int) string {
	parts := make([]string, count)
	for i := range parts {
		parts[i] = "has(" + column + ", ?)"
	}
	return "(" + strings.Join(parts, " OR ") + ")"
}

func loadFilterOptions(ctx context.Context, db *sql.DB, realmName string) ([]Option, []Option, error) {
	// Bosses: take distinct (boss_remote_id, boss_name) from the events table,
	// which always reflects the freshest names — even if a future rename hasn't
	// flushed into the `boss` lookup.
	const bossQ = `
		SELECT boss_remote_id, any(boss_name) AS name
		FROM boss_kill
		WHERE realm = ?
		GROUP BY boss_remote_id
		ORDER BY name
	`
	rows, err := db.QueryContext(ctx, bossQ, realmName)
	if err != nil {
		return nil, nil, err
	}
	var bossOpts []Option
	for rows.Next() {
		var id uint32
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			rows.Close()
			return nil, nil, err
		}
		bossOpts = append(bossOpts, Option{
			Value: strconv.FormatUint(uint64(id), 10),
			Label: name,
		})
	}
	rows.Close()

	const raidQ = `
		SELECT raid_name, count() AS c FROM boss_kill
		WHERE realm = ?
		GROUP BY raid_name
		ORDER BY c DESC
	`
	rows2, err := db.QueryContext(ctx, raidQ, realmName)
	if err != nil {
		return nil, nil, err
	}
	defer rows2.Close()
	var raidOpts []Option
	for rows2.Next() {
		var name string
		var c uint64
		if err := rows2.Scan(&name, &c); err != nil {
			return nil, nil, err
		}
		raidOpts = append(raidOpts, Option{Value: name, Label: name})
	}
	return bossOpts, raidOpts, rows2.Err()
}

func markSelected(bossOpts []Option, bosses []uint32, raidOpts []Option, raids []string) {
	bossSet := map[string]bool{}
	for _, b := range bosses {
		bossSet[strconv.FormatUint(uint64(b), 10)] = true
	}
	for i := range bossOpts {
		bossOpts[i].Selected = bossSet[bossOpts[i].Value]
	}
	raidSet := map[string]bool{}
	for _, r := range raids {
		raidSet[r] = true
	}
	for i := range raidOpts {
		raidOpts[i].Selected = raidSet[raidOpts[i].Value]
	}
}

func buildModeOptions(rows []KillRow, selectedModes []int, expansion int) []Option {
	// Just expose the common difficulties for this expansion. Skip "empty"
	// selectors; we'd rather offer all options than only those with data.
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

func buildClassOptions(selectedClasses []int, expansion int) []Option {
	sel := intSet(selectedClasses)
	classes := wow.ClassesForExpansion(expansion)
	out := make([]Option, 0, len(classes))
	for _, class := range classes {
		label := wow.Class(class)
		out = append(out, Option{
			Value:    strconv.Itoa(class),
			Label:    label,
			Selected: sel[class],
			IconURL:  links.ClassIcon(class),
			IconAlt:  label,
		})
	}
	return out
}

func buildSpecOptions(realmName string, selectedSpecs []int, expansion int) []Option {
	sel := intSet(selectedSpecs)
	specs := wow.SpecsForExpansion(expansion)
	out := make([]Option, 0, len(specs))
	for _, spec := range specs {
		specLabel := wow.SpecForExpansion(expansion, spec)
		class := wow.ClassFromSpecForExpansion(expansion, spec)
		classLabel := wow.Class(class)
		label := specLabel
		iconURL := links.TalentIcon(realmName, spec)
		iconAlt := specLabel
		iconURL2 := ""
		iconAlt2 := ""
		if classLabel != "" {
			label = classLabel + " / " + specLabel
			iconURL = links.ClassIcon(class)
			iconAlt = classLabel
			iconURL2 = links.TalentIcon(realmName, spec)
			iconAlt2 = specLabel
		}
		out = append(out, Option{
			Value:    strconv.Itoa(spec),
			Label:    label,
			Selected: sel[spec],
			IconURL:  iconURL,
			IconAlt:  iconAlt,
			IconURL2: iconURL2,
			IconAlt2: iconAlt2,
		})
	}
	return out
}

func intSet(values []int) map[int]bool {
	out := make(map[int]bool, len(values))
	for _, v := range values {
		out[v] = true
	}
	return out
}

func Mount(mux *http.ServeMux, deps Deps) {
	h := middleware.RequireRealm(Handler(deps))
	router.ForEachRealmPrefix(func(prefix string) {
		mux.Handle("GET "+prefix+"/boss-kills", h)
		mux.Handle("GET "+prefix+"/boss-kills/{$}", h)
	})
}
