// server is the HTTP entry point for the Go rewrite of twinstar-bosskills.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"twinstar-bosskills/internal/api"
	"twinstar-bosskills/internal/cache"
	"twinstar-bosskills/internal/ch"
	"twinstar-bosskills/internal/config"
	"twinstar-bosskills/internal/realm"
	"twinstar-bosskills/internal/web/server"
)

var version = "dev"

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	logger = logger.With("version", version)
	cfg := config.FromEnv()

	if err := cfg.RequireClickHouseDSN(); err != nil {
		logger.Error("missing config", "err", err)
		os.Exit(2)
	}

	db, err := ch.Open(ch.Options{DSN: cfg.ClickHouse.DSN})
	if err != nil {
		logger.Error("ch.Open", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := ch.Ping(ctx, db); err != nil {
		logger.Error("ch.Ping", "err", err)
		os.Exit(1)
	}

	icons, err := cache.NewIconDisk(cfg.Cache.IconDir)
	if err != nil {
		logger.Error("icon cache init", "err", err)
		os.Exit(1)
	}

	apiClient := api.NewClient(cfg.Twinstar.APIURL)
	items, err := cache.NewItemDisk(cfg.Cache.ItemDir, apiClient)
	if err != nil {
		logger.Error("item cache init", "err", err)
		os.Exit(1)
	}
	// Tooltip API requires an expansion; default to MoP (matches the seed
	// data). Items previously cached without a tooltip stay valid - the
	// rendered popover just doesn't appear for them until the cache entry
	// gets re-created.
	items.Expansion = realm.ExpansionMoP

	handler := server.New(server.Config{
		DB:          db,
		Logger:      logger,
		Icons:       icons,
		Items:       items,
		APIBase:     cfg.Twinstar.APIURL,
		SecretGuild: cfg.Secrets.GuildToken,
		SecretAdmin: cfg.Secrets.AdminToken,
	})

	srv := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server listening", "addr", cfg.HTTP.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown requested")
	case err := <-errCh:
		if err != nil {
			logger.Error("listen", "err", err)
			os.Exit(1)
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown", "err", err)
	}
}
