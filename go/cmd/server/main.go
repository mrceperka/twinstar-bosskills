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
	"twinstar-bosskills/internal/realm"
	"twinstar-bosskills/internal/web/server"
)

var version = "dev"

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	logger = logger.With("version", version)

	dsn := os.Getenv("BK_CH_DSN")
	if dsn == "" {
		logger.Error("missing BK_CH_DSN")
		os.Exit(2)
	}
	addr := os.Getenv("BK_HTTP_ADDR")
	if addr == "" {
		addr = ":3000"
	}

	db, err := ch.Open(ch.Options{DSN: dsn})
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

	iconDir := os.Getenv("BK_ICON_DIR")
	if iconDir == "" {
		iconDir = "./var/icons"
	}
	icons, err := cache.NewIconDisk(iconDir)
	if err != nil {
		logger.Error("icon cache init", "err", err)
		os.Exit(1)
	}

	apiBase := os.Getenv("BK_TWINSTAR_API_URL")
	apiClient := api.NewClient(apiBase)
	itemDir := os.Getenv("BK_ITEM_DIR")
	if itemDir == "" {
		itemDir = "./var/items"
	}
	items, err := cache.NewItemDisk(itemDir, apiClient)
	if err != nil {
		logger.Error("item cache init", "err", err)
		os.Exit(1)
	}
	// Tooltip API requires an expansion; default to MoP (matches the seed
	// data). Items previously cached without a tooltip stay valid — the
	// rendered popover just doesn't appear for them until the cache entry
	// gets re-created.
	items.Expansion = realm.ExpansionMoP

	handler := server.New(server.Config{
		DB:          db,
		Logger:      logger,
		Icons:       icons,
		Items:       items,
		APIBase:     apiBase,
		SecretGuild: os.Getenv("SECRET_TOKEN_GUILD"),
		SecretAdmin: os.Getenv("SECRET_TOKEN_ADMIN"),
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server listening", "addr", addr)
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
