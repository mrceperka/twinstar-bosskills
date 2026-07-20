// sync ports packages/model/src/cli/synchronize-with-api.ts.
//
// Flags mirror the existing CLI:
//
//	--realm     <name>      single realm
//	--realms    Helios,Athena   multiple realms (sync runs in parallel)
//	--from-date 2026-06-01  starting date (RFC3339 also OK)
//	--offset    <int>       raid-lock offset (0 = current). Sets startsAt/endsAt
//	                        from the current raid lock window
//	--bosskill-ids 1,2,3    only these bosskill remote_ids (passed straight to upstream filter)
//	--boss-ids 71358        only these boss entries
//	--page      <int>       0-indexed page; combined with --page-size
//	--page-size <int>       upstream page size
//	--batch-size 10000        CH insert batch size
//	--concurrency 4         API/CH concurrency per realm
//
// If --from-date is set, --offset is ignored (matches existing TS behavior).
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"twinstar-bosskills/internal/api"
	"twinstar-bosskills/internal/ch"
	"twinstar-bosskills/internal/config"
	"twinstar-bosskills/internal/domain"
	"twinstar-bosskills/internal/realm"
)

func main() {
	cfg := config.FromEnv()
	var (
		dsn         = flag.String("dsn", cfg.ClickHouse.DSN, "ClickHouse DSN (defaults to $"+config.EnvClickHouseDSN+")")
		baseURL     = flag.String("api-url", cfg.Twinstar.APIURL, "Twinstar API base URL")
		realmFlag   = flag.String("realm", "", "single realm name")
		realmsFlag  = flag.String("realms", "", "comma-separated realms")
		fromDate    = flag.String("from-date", "", "starting date (YYYY-MM-DD or RFC3339)")
		offset      = flag.Int("offset", -1, "raid-lock offset; -1 means unset")
		bosskillIDs = flag.String("bosskill-ids", "", "comma-separated bosskill remote_ids")
		bossIDs     = flag.String("boss-ids", "", "comma-separated boss entry IDs")
		page        = flag.Int("page", -1, "page (0-indexed); -1 means unset")
		pageSize    = flag.Int("page-size", -1, "page size; -1 means unset")
		batchSize   = flag.Int("batch-size", 10000, "ClickHouse insert batch size")
		concurrency = flag.Int("concurrency", 4, "per-realm worker concurrency")
	)
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if *dsn == "" {
		logger.Error("missing DSN", "flag", "--dsn", "env", config.EnvClickHouseDSN)
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

	var startsAt, endsAt *time.Time
	if *offset >= 0 && *fromDate == "" {
		w := domain.RaidLock(time.Now().UTC(), *offset)
		startsAt = &w.Start
		endsAt = &w.End
	}
	if *fromDate != "" {
		if *offset >= 0 {
			logger.Warn("--from-date overrides --offset")
		}
		t, err := parseDateFlag(*fromDate)
		if err != nil {
			logger.Error("parse --from-date", "err", err)
			os.Exit(2)
		}
		startsAt = &t
	}

	opts := syncOptions{
		BatchSize:   *batchSize,
		Concurrency: *concurrency,
		StartsAt:    startsAt,
		EndsAt:      endsAt,
		BosskillIDs: splitNonEmpty(*bosskillIDs),
		BossIDs:     parseUInt32List(*bossIDs),
	}
	if *page >= 0 {
		v := *page
		opts.Page = &v
	}
	if *pageSize > 0 {
		v := *pageSize
		opts.PageSize = &v
	}

	db, err := ch.Open(ch.Options{DSN: *dsn, MaxOpen: *concurrency * len(realms)})
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

	cli := api.NewClient(*baseURL)

	logger.Info("starting sync",
		"realms", realms,
		"startsAt", fmtTime(startsAt),
		"endsAt", fmtTime(endsAt),
		"boss-ids", opts.BossIDs,
		"bosskill-ids", opts.BosskillIDs,
	)

	var wg sync.WaitGroup
	var anyErr error
	var errMu sync.Mutex
	for _, r := range realms {
		r := r
		ro := opts
		ro.Realm = r
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := syncRealm(ctx, logger, db, cli, ro); err != nil {
				if cerr := syncContextError(ctx, err); cerr != nil {
					logger.Warn("realm sync canceled", "realm", r, "reason", cerr, "err", err)
				} else {
					logger.Error("realm failed", "realm", r, "err", err)
				}
				errMu.Lock()
				if anyErr == nil {
					anyErr = err
				}
				errMu.Unlock()
			}
		}()
	}
	wg.Wait()

	if anyErr != nil {
		if cerr := syncContextError(ctx, anyErr); cerr != nil {
			logger.Warn("sync stopped before completion", "reason", cerr)
		}
		os.Exit(1)
	}
	logger.Info("sync done")
}

func pickRealms(single, multi string) []string {
	if multi != "" {
		out := splitNonEmpty(multi)
		for i, r := range out {
			out[i] = realm.Normalize(r)
		}
		return out
	}
	if single != "" {
		return []string{realm.Normalize(single)}
	}
	return nil
}

func splitNonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := parts[:0]
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseUInt32List(s string) []uint32 {
	parts := splitNonEmpty(s)
	out := make([]uint32, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.ParseUint(p, 10, 32)
		if err != nil {
			continue
		}
		out = append(out, uint32(v))
	}
	return out
}

func parseDateFlag(s string) (time.Time, error) {
	for _, f := range []string{
		"2006-01-02",
		time.RFC3339,
		"2006-01-02T15:04:05",
	} {
		if t, err := time.Parse(f, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date %q", s)
}

func fmtTime(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.Format(time.RFC3339)
}
