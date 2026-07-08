// Package server wires the HTTP mux, middleware, and route handlers.
//
// Convention: each page package under internal/web/views/pages exposes a
// Mount(mux, Deps) function. server.New calls them all. To add a new route,
// create a new page package with its own page.templ + handler.go + Mount(),
// then add one line below.
package server

import (
	"database/sql"
	"log/slog"
	"net/http"

	"twinstar-bosskills/internal/api"
	"twinstar-bosskills/internal/cache"
	"twinstar-bosskills/internal/web/middleware"
	"twinstar-bosskills/internal/web/static"
	"twinstar-bosskills/internal/web/views/pages/admingc"
	"twinstar-bosskills/internal/web/views/pages/boss"
	"twinstar-bosskills/internal/web/views/pages/bosskill"
	"twinstar-bosskills/internal/web/views/pages/bosskills"
	"twinstar-bosskills/internal/web/views/pages/changelog"
	"twinstar-bosskills/internal/web/views/pages/character"
	"twinstar-bosskills/internal/web/views/pages/characterperf"
	"twinstar-bosskills/internal/web/views/pages/characters"
	"twinstar-bosskills/internal/web/views/pages/dashboard"
	"twinstar-bosskills/internal/web/views/pages/guildtoken"
	"twinstar-bosskills/internal/web/views/pages/home"
	"twinstar-bosskills/internal/web/views/pages/icon"
	"twinstar-bosskills/internal/web/views/pages/raids"
	"twinstar-bosskills/internal/web/views/pages/ranks"
	"twinstar-bosskills/internal/web/views/pages/stats"
)

type Config struct {
	DB          *sql.DB
	Logger      *slog.Logger
	Icons       *cache.IconDisk
	Items       *cache.ItemDisk
	APIBase     string
	SecretGuild string
	SecretAdmin string
}

// New returns an http.Handler with the full middleware chain + routes.
func New(cfg Config) http.Handler {
	mux := http.NewServeMux()

	cssHash := static.Hash("app.css")
	jsHash := static.Hash("htmx.min.js")

	// Static assets — content-hashed in URLs, cached aggressively.
	mux.Handle("GET /static/", cacheStatic(http.StripPrefix("/static", static.Handler())))
	mux.Handle("GET /favicon.ico", cacheStatic(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, static.FS(), "favicon.ico")
	})))

	// Health endpoint (cheap, no DB).
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// /admin/gc — runtime memory stats + manual GC trigger.
	admingc.Mount(mux)

	// Pages — one Mount() per route group.
	home.Mount(mux, home.Deps{DB: cfg.DB, CSSHash: cssHash, JSHash: jsHash})
	changelog.Mount(mux, changelog.Deps{CSSHash: cssHash, JSHash: jsHash})
	dashboard.Mount(mux, dashboard.Deps{DB: cfg.DB, CSSHash: cssHash, JSHash: jsHash})
	raids.Mount(mux, raids.Deps{DB: cfg.DB, CSSHash: cssHash, JSHash: jsHash})
	boss.Mount(mux, boss.Deps{DB: cfg.DB, CSSHash: cssHash, JSHash: jsHash})
	bosskills.Mount(mux, bosskills.Deps{DB: cfg.DB, CSSHash: cssHash, JSHash: jsHash})
	bosskill.Mount(mux, bosskill.Deps{DB: cfg.DB, Items: cfg.Items, CSSHash: cssHash, JSHash: jsHash})
	ranks.Mount(mux, ranks.Deps{DB: cfg.DB, CSSHash: cssHash, JSHash: jsHash})
	stats.Mount(mux, stats.Deps{DB: cfg.DB, CSSHash: cssHash, JSHash: jsHash})
	character.Mount(mux, character.Deps{DB: cfg.DB, API: api.NewClient(cfg.APIBase), CSSHash: cssHash, JSHash: jsHash})
	characterperf.Mount(mux, characterperf.Deps{DB: cfg.DB, CSSHash: cssHash, JSHash: jsHash})
	characters.Mount(mux, characters.Deps{DB: cfg.DB, CSSHash: cssHash, JSHash: jsHash})
	if cfg.Icons != nil {
		icon.Mount(mux, icon.Deps{APIBase: cfg.APIBase, Icons: cfg.Icons})
	}
	icon.MountTooltip(mux, cfg.APIBase)
	guildtoken.Mount(mux, guildtoken.Deps{
		SecretGuild: cfg.SecretGuild,
		SecretAdmin: cfg.SecretAdmin,
		CSSHash:     cssHash,
		JSHash:      jsHash,
	})

	return middleware.Chain(cfg.Logger, middleware.AttachGuildAuth(cfg.SecretGuild)(mux))
}

// cacheStatic adds a 30-day Cache-Control header to assets served under /static.
// Cache-busting is via query-string content hash, so it's safe to be greedy.
func cacheStatic(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=2592000, immutable")
		h.ServeHTTP(w, r)
	})
}
