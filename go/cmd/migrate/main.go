// migrate runs pending ClickHouse migrations.
//
// Usage:
//
//	migrate up      apply every pending migration
//	migrate status  list applied vs pending
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"twinstar-bosskills/internal/ch"
	"twinstar-bosskills/internal/config"
	"twinstar-bosskills/migrations"
)

func main() {
	cfg := config.FromEnv()
	dsn := flag.String("dsn", cfg.ClickHouse.DSN, "ClickHouse DSN (defaults to $"+config.EnvClickHouseDSN+")")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		args = []string{"up"}
	}
	cmd := args[0]

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if *dsn == "" {
		logger.Error("missing DSN", "flag", "--dsn", "env", config.EnvClickHouseDSN)
		os.Exit(2)
	}

	db, err := ch.Open(ch.Options{DSN: *dsn})
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

	loaded, err := ch.LoadMigrations(migrations.FS, ".")
	if err != nil {
		logger.Error("load migrations", "err", err)
		os.Exit(1)
	}
	if len(loaded) == 0 {
		logger.Error("no migrations found in embedded FS")
		os.Exit(1)
	}

	switch cmd {
	case "up":
		ran, err := ch.Apply(ctx, db, loaded)
		if err != nil {
			logger.Error("apply", "err", err)
			os.Exit(1)
		}
		if len(ran) == 0 {
			fmt.Println("already up to date")
			return
		}
		for _, m := range ran {
			fmt.Printf("applied %03d_%s\n", m.Version, m.Name)
		}
	case "status":
		if err := ch.EnsureMigrationTable(ctx, db); err != nil {
			logger.Error("ensure _migrations", "err", err)
			os.Exit(1)
		}
		applied, err := ch.AppliedVersions(ctx, db)
		if err != nil {
			logger.Error("read applied", "err", err)
			os.Exit(1)
		}
		for _, m := range loaded {
			mark := "pending"
			if applied[m.Version] {
				mark = "applied"
			}
			fmt.Printf("%-8s %03d_%s\n", mark, m.Version, m.Name)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q (use: up | status)\n", cmd)
		os.Exit(2)
	}
}
