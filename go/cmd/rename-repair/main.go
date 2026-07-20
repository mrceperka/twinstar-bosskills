// rename-repair fixes historical boss-kill player names after character
// renames.
//
// It detects GUIDs that appear with more than one name in boss_kill, chooses
// the latest name by kill_time, and mutates only the affected Nested
// players.name entries. This is intentionally a maintenance command instead of
// part of the normal sync path: ClickHouse mutations are expensive enough that
// they should stay explicit and bounded.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"twinstar-bosskills/internal/ch"
	"twinstar-bosskills/internal/config"
	"twinstar-bosskills/internal/realm"
)

type renameCandidate struct {
	Realm        string
	GUID         uint64
	NameNext     string
	Names        []string
	AffectedRows uint64
}

func main() {
	cfg := config.FromEnv()
	var (
		dsn           = flag.String("dsn", cfg.ClickHouse.DSN, "ClickHouse DSN (defaults to $"+config.EnvClickHouseDSN+")")
		realmFlag     = flag.String("realm", "", "single realm name")
		realmsFlag    = flag.String("realms", "", "comma-separated realms")
		dryRun        = flag.Bool("dry-run", false, "print planned repairs without mutating boss_kill")
		limit         = flag.Int("limit", 0, "max GUIDs to repair per realm; 0 means no limit")
		mutationsSync = flag.Int("mutations-sync", 1, "ClickHouse mutations_sync setting: 0 async, 1 wait local, 2 wait replicas")
	)
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if *dsn == "" {
		logger.Error("missing DSN", "flag", "--dsn", "env", config.EnvClickHouseDSN)
		os.Exit(2)
	}
	if *limit < 0 {
		logger.Error("--limit must be >= 0")
		os.Exit(2)
	}
	if *mutationsSync < 0 || *mutationsSync > 2 {
		logger.Error("--mutations-sync must be 0, 1, or 2")
		os.Exit(2)
	}

	realms := pickRealms(*realmFlag, *realmsFlag)
	if len(realms) == 0 {
		logger.Error("no realm given; use --realm or --realms")
		os.Exit(2)
	}
	for _, r := range realms {
		if !realm.IsKnown(r) {
			logger.Error("unknown realm", "realm", r)
			os.Exit(2)
		}
	}

	db, err := ch.Open(ch.Options{DSN: *dsn, MaxOpen: 2})
	if err != nil {
		logger.Error("connect", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := ch.Ping(ctx, db); err != nil {
		logger.Error("ping", "err", err)
		os.Exit(1)
	}

	totalCandidates := 0
	totalMutatedRows := uint64(0)
	for _, realmName := range realms {
		candidates, err := findRenameCandidates(ctx, db, realmName, *limit)
		if err != nil {
			logger.Error("find rename candidates", "realm", realmName, "err", err)
			os.Exit(1)
		}
		if len(candidates) == 0 {
			logger.Info("no renames found", "realm", realmName)
			continue
		}

		for _, c := range candidates {
			totalCandidates++
			totalMutatedRows += c.AffectedRows
			logger.Info("rename candidate",
				"realm", c.Realm,
				"guid", c.GUID,
				"names", strings.Join(c.Names, ","),
				"name_next", c.NameNext,
				"affected_rows", c.AffectedRows,
				"dry_run", *dryRun,
			)
			if *dryRun {
				continue
			}
			if err := repairRename(ctx, db, c, *mutationsSync); err != nil {
				logger.Error("repair rename", "realm", c.Realm, "guid", c.GUID, "err", err)
				os.Exit(1)
			}
		}
	}

	if *dryRun {
		logger.Info("dry run complete", "candidates", totalCandidates, "affected_rows", totalMutatedRows)
		return
	}
	logger.Info("rename repair complete", "candidates", totalCandidates, "mutated_rows", totalMutatedRows)
}

func findRenameCandidates(ctx context.Context, db *sql.DB, realmName string, limit int) ([]renameCandidate, error) {
	q := `
		WITH latest AS (
			SELECT
				guid,
				argMax(name, last_seen) AS name_next,
				groupUniqArray(name) AS names
			FROM (
				SELECT
					players.guid AS guid,
					players.name AS name,
					max(kill_time) AS last_seen
				FROM boss_kill
				ARRAY JOIN players
				WHERE realm = ?
				GROUP BY guid, name
			)
			GROUP BY guid
			HAVING length(names) > 1
		)
		SELECT
			b.guid,
			any(l.name_next) AS name_next,
			any(l.names) AS names,
			count() AS affected_rows
		FROM (
			SELECT
				players.guid AS guid,
				players.name AS name
			FROM boss_kill
			ARRAY JOIN players
			WHERE realm = ?
		) AS b
		INNER JOIN latest AS l ON l.guid = b.guid
		WHERE b.name != l.name_next
		GROUP BY b.guid
		ORDER BY affected_rows DESC, b.guid ASC
	`
	args := []any{realmName, realmName}
	if limit > 0 {
		q += fmt.Sprintf("\n\t\tLIMIT %d", limit)
	}

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []renameCandidate
	for rows.Next() {
		c := renameCandidate{Realm: realmName}
		if err := rows.Scan(&c.GUID, &c.NameNext, &c.Names, &c.AffectedRows); err != nil {
			return nil, err
		}
		sort.Strings(c.Names)
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func repairRename(ctx context.Context, db *sql.DB, c renameCandidate, mutationsSync int) error {
	q := fmt.Sprintf(`
		ALTER TABLE boss_kill
		UPDATE `+"`players.name`"+` = arrayMap(
			(g, n) -> if(g = toUInt64(?), ?, n),
			`+"`players.guid`"+`,
			`+"`players.name`"+`
		)
		WHERE realm = ?
		  AND arrayExists(
			  (g, n) -> g = toUInt64(?) AND n != ?,
			  `+"`players.guid`"+`,
			  `+"`players.name`"+`
		  )
		SETTINGS mutations_sync = %d
	`, mutationsSync)
	_, err := db.ExecContext(ctx, q, c.GUID, c.NameNext, c.Realm, c.GUID, c.NameNext)
	return err
}

func pickRealms(realmFlag, realmsFlag string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		v = realm.Normalize(v)
		if seen[v] {
			return
		}
		seen[v] = true
		out = append(out, v)
	}
	add(realmFlag)
	for _, part := range strings.Split(realmsFlag, ",") {
		add(part)
	}
	return out
}
