// ch-health prints ClickHouse health diagnostics: part counts, live and slow
// queries, merge backlog, query cache occupancy, and system log growth.
//
// Read-only. Meant to be run by hand periodically, or from cron with --only to
// track one number over time:
//
//	make ch-health
//	make ch-health ARGS="--only cache,cache-hits --hours 24"
//
// Deliberately opens its own pool WITHOUT the query cache the web server uses -
// cached diagnostics would report the state of the past, not of now.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"

	"twinstar-bosskills/internal/ch"
	"twinstar-bosskills/internal/config"
)

var version = "dev"

// section is one diagnostic query. Every column must come back as String -
// toString/formatReadableSize in the SQL - so scanning stays type-agnostic and
// a new ClickHouse version can't break it by widening an integer.
type section struct {
	name  string
	title string
	// sql may contain two %d verbs, filled with --hours then --limit, in that
	// order. Declare how many it uses so the formatting stays honest.
	sql   string
	hours bool
	limit bool
}

var sections = []section{
	{
		name:  "parts",
		title: "Parts per application table (high part count = fragmented inserts)",
		// Aliases must not collide with system.parts column names: aliasing
		// toString(sum(rows)) AS rows makes the next sum(rows) resolve to the
		// String alias, and ClickHouse rejects it with "Illegal type String".
		sql: "SELECT `table`, toString(count()) AS parts, toString(sum(rows)) AS total_rows," +
			" formatReadableSize(sum(bytes_on_disk)) AS disk," +
			" toString(round(sum(rows) / count(), 0)) AS rows_per_part" +
			" FROM system.parts WHERE active AND database = currentDatabase()" +
			" GROUP BY `table` ORDER BY count() DESC",
	},
	{
		name:  "system-logs",
		title: "System log tables (should stay flat after config.d/logs.xml)",
		sql: "SELECT `table`, toString(sum(rows)) AS total_rows," +
			" formatReadableSize(sum(bytes_on_disk)) AS disk" +
			" FROM system.parts WHERE active AND database = 'system'" +
			" GROUP BY `table` ORDER BY sum(rows) DESC",
	},
	{
		name:  "load",
		title: "Live server counters",
		sql: "SELECT metric AS name," +
			" if(metric LIKE '%Memory%', formatReadableSize(value), toString(value)) AS value" +
			" FROM system.metrics WHERE metric IN (" +
			"'Query','Merge','PartMutation','PartsActive','TCPConnection','HTTPConnection'," +
			"'MemoryTracking','BackgroundMergesAndMutationsPoolTask','DelayedInserts'," +
			"'QueryPreempted','GlobalThreadActive') ORDER BY metric",
	},
	{
		name:  "running",
		title: "Queries running right now",
		sql: "SELECT toString(round(elapsed, 1)) AS elapsed_s, toString(read_rows) AS read_rows," +
			" formatReadableSize(memory_usage) AS memory," +
			" toString(length(thread_ids)) AS threads," +
			" substring(replaceRegexpAll(query, '\\\\s+', ' '), 1, 80) AS query" +
			" FROM system.processes WHERE query != '' ORDER BY elapsed DESC",
	},
	{
		name:  "merges",
		title: "Merges in flight (never empty = merge backlog starving queries)",
		sql: "SELECT `table`, toString(round(elapsed, 1)) AS elapsed_s," +
			" toString(round(progress, 2)) AS progress, toString(num_parts) AS num_parts," +
			" formatReadableSize(memory_usage) AS memory FROM system.merges ORDER BY elapsed DESC",
	},
	{
		name:  "slow",
		title: "Slowest queries",
		sql: "SELECT toString(query_duration_ms) AS ms, toString(type) AS type," +
			" formatReadableSize(memory_usage) AS memory, toString(read_rows) AS read_rows," +
			" substring(replaceRegexpAll(query, '\\\\s+', ' '), 1, 80) AS query" +
			" FROM system.query_log" +
			" WHERE event_time > now() - toIntervalHour(%d) AND type != 'QueryStart'" +
			" ORDER BY query_duration_ms DESC LIMIT %d",
		hours: true,
		limit: true,
	},
	{
		name:  "cache",
		title: "Query cache occupancy (entries pinned at max_entries = still evicting)",
		sql: "SELECT toString(count()) AS entries, formatReadableSize(sum(result_size)) AS total," +
			" formatReadableSize(round(avgOrDefault(result_size))) AS avg_entry," +
			" formatReadableSize(max(result_size)) AS biggest," +
			" toString(countIf(stale)) AS stale FROM system.query_cache",
	},
	{
		name:  "cache-hits",
		title: "Query cache hit ratio",
		sql: "SELECT toString(sum(ProfileEvents['QueryCacheHits'])) AS hits," +
			" toString(sum(ProfileEvents['QueryCacheMisses'])) AS misses," +
			" toString(round(100 * sum(ProfileEvents['QueryCacheHits'])" +
			" / greatest(sum(ProfileEvents['QueryCacheHits'])" +
			" + sum(ProfileEvents['QueryCacheMisses']), 1), 1)) AS hit_pct" +
			" FROM system.query_log" +
			" WHERE event_time > now() - toIntervalHour(%d) AND type = 'QueryFinish'",
		hours: true,
	},
	{
		name:  "errors",
		title: "Server errors since startup",
		sql: "SELECT name, toString(value) AS count, toString(last_error_time) AS last_seen," +
			" substring(replaceRegexpAll(last_error_message, '\\\\s+', ' '), 1, 70) AS message" +
			" FROM system.errors WHERE value > 0 ORDER BY value DESC LIMIT %d",
		limit: true,
	},
}

func main() {
	cfg := config.FromEnv()
	var (
		dsn   = flag.String("dsn", cfg.ClickHouse.DSN, "ClickHouse DSN (defaults to $"+config.EnvClickHouseDSN+")")
		only  = flag.String("only", "", "comma-separated sections to run (default all); see --list")
		hours = flag.Int("hours", 1, "lookback window in hours for query_log sections")
		limit = flag.Int("limit", 15, "row limit for the slow/errors sections")
		list  = flag.Bool("list", false, "print section names and exit")
	)
	flag.Parse()

	if *list {
		for _, s := range sections {
			fmt.Printf("%-12s %s\n", s.name, s.title)
		}
		return
	}
	// Flag validation before the DSN check, so a typo in --only reports the typo
	// rather than complaining about configuration.
	if *hours < 1 {
		fmt.Fprintln(os.Stderr, "--hours must be >= 1")
		os.Exit(2)
	}
	if *limit < 1 {
		fmt.Fprintln(os.Stderr, "--limit must be >= 1")
		os.Exit(2)
	}
	selected, err := pickSections(*only)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *dsn == "" {
		fmt.Fprintf(os.Stderr, "missing DSN: pass --dsn or set $%s\n", config.EnvClickHouseDSN)
		os.Exit(2)
	}

	db, err := ch.Open(ch.Options{DSN: *dsn, MaxOpen: 1})
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := ch.Ping(ctx, db); err != nil {
		fmt.Fprintf(os.Stderr, "ping: %v\n", err)
		os.Exit(1)
	}

	// A failing section is reported and skipped, not fatal: system table columns
	// differ between ClickHouse versions, and one missing column must not hide
	// the other nine sections.
	failed := 0
	for _, s := range selected {
		if err := runSection(ctx, db, s, *hours, *limit); err != nil {
			fmt.Printf("\n== %s ==\n  FAILED: %v\n", s.title, err)
			failed++
		}
	}
	if failed > 0 {
		fmt.Fprintf(os.Stderr, "\n%d/%d sections failed\n", failed, len(selected))
		os.Exit(1)
	}
}

func pickSections(only string) ([]section, error) {
	if strings.TrimSpace(only) == "" {
		return sections, nil
	}
	byName := make(map[string]section, len(sections))
	for _, s := range sections {
		byName[s.name] = s
	}
	var out []section
	for _, raw := range strings.Split(only, ",") {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		s, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("unknown section %q; see --list", name)
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("--only matched no sections; see --list")
	}
	return out, nil
}

// sectionSQL fills the section's %d verbs. hours comes first, then limit -
// matching the order the verbs appear in every section's SQL.
func sectionSQL(s section, hours, limit int) string {
	var args []any
	if s.hours {
		args = append(args, hours)
	}
	if s.limit {
		args = append(args, limit)
	}
	if len(args) == 0 {
		return s.sql
	}
	return fmt.Sprintf(s.sql, args...)
}

func runSection(ctx context.Context, db *sql.DB, s section, hours, limit int) error {
	rows, err := db.QueryContext(ctx, sectionSQL(s, hours, limit))
	if err != nil {
		return err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return err
	}

	fmt.Printf("\n== %s ==\n", s.title)
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "  "+strings.Join(cols, "\t"))

	vals := make([]string, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	n := 0
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			w.Flush()
			return err
		}
		fmt.Fprintln(w, "  "+strings.Join(vals, "\t"))
		n++
	}
	if err := rows.Err(); err != nil {
		w.Flush()
		return err
	}
	if n == 0 {
		fmt.Fprintln(w, "  (none)")
	}
	return w.Flush()
}
