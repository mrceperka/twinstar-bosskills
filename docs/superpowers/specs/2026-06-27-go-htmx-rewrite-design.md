# Go + htmx Rewrite of twinstar-bosskills

Date: 2026-06-27
Status: Approved (pending user review of this document)

## Goal

Rewrite the existing SvelteKit application as a single-binary Go server using htmx for interactivity, templ for typed templates, Tailwind for styling, and ClickHouse as the only data store. The existing SvelteKit code stays in the repo for reference until the new app reaches parity.

## Non-goals

- Pixel-perfect visual parity. Visual parity to a "close enough" standard.
- Feature additions or scope changes during the rewrite. Behavior is mirrored, then iterated on after cutover.
- Multi-instance deployment. Single-process, single-host is the target.
- Ported test suite. The existing app has none; we add targeted tests only where logic is non-obvious (raid-lock dates, HMAC tokens).

## Stack

| Layer | Choice |
|---|---|
| Language | Go 1.22+ |
| HTTP | stdlib `net/http`, `http.ServeMux` |
| Templates | `a-h/templ` |
| Styling | Tailwind CSS via standalone CLI binary (no Node) |
| Interactivity | htmx (self-hosted, version-pinned) |
| Charts | echarts kept; wrapped in a plain custom element (no Lit) |
| Tables | server-rendered + htmx for sort/filter; ~50 LOC vanilla JS for client-side sort of already-loaded pages |
| Data | ClickHouse only (`ClickHouse/clickhouse-go/v2` via `database/sql`) |
| Cache | In-process LRU (memory) for query results + on-disk for icon blobs |
| Config | `os.Getenv` + tiny `.env` parser |
| Logging | `log/slog` |
| Routing convention | Custom ~150 LOC file-router on top of `http.ServeMux` |
| CLI jobs | `flag` package, subcommands of one binary |
| Dev loop | `wgo` (or `air`) for live reload — dev dep only |

Non-stdlib dependencies (full list):
- `github.com/a-h/templ`
- `github.com/ClickHouse/clickhouse-go/v2`

Everything else is stdlib.

## Project layout

```
go/
  bin/tailwindcss              # standalone binary, gitignored, fetched by `make setup`
  tailwind.config.js
  input.css                    # @tailwind directives
  Makefile
  cmd/
    server/main.go             # HTTP server
    sync/main.go               # synchronize-with-api (only remaining CLI job)
    migrate/main.go            # migration runner
  internal/
    api/                       # twinstar API client (port of packages/api)
    cache/
      lru.go                   # in-process LRU + TTL
      icons.go                 # on-disk blob cache for /img/icon
    ch/                        # ClickHouse: pool, queries, helpers
    domain/
      raidlock.go              # time-zone-aware reset calc (TESTED)
      hmac.go                  # guild-token HMAC (TESTED, byte-compatible with existing format)
      metrics.go               # DPS/HPS calc helpers
      realm.go                 # realm enums, expansion mapping, merge redirects
    lookup/                    # in-memory registry for boss/raid/realm/boss_prop, refreshed on a timer
    web/
      router/                  # file-router convention (~150 LOC)
      views/
        layouts/               # root.templ, error.templ
        components/            # table.templ, pagination.templ, chart.templ, filter_form.templ, icon.templ, link.templ
        pages/                 # convention: directory = URL path; file = handler
          +layout.templ
          index.templ
          changelog/
            index.templ
          (realm)/
            +layout.templ
            index.templ
            boss-kills/
              index.templ
              [id].templ
            boss/
              [id]/
                +layout.templ
                index.templ
                history/index.templ
            character/
              [name]/
                +layout.templ
                index.templ
                performance/index.templ
            characters/index.templ
            guild-token/
              index.templ
              generate/index.templ
            raids/index.templ
            ranks/index.templ
          admin/gc/index.templ
          img/icon/index.templ
      handlers/                # one file per route group; corresponds to a pages/ subtree
      middleware/              # recover, requestid, slog, realm, guildtoken, headers
      static/                  # built: app.css, htmx.min.js, echarts.min.js, components.js
  migrations/                  # numbered ClickHouse .sql files
  queries/                     # named .sql files for app queries (embed.FS)
```

## File-based routing convention

A ~150 LOC router walks `internal/web/views/pages/**` at startup and registers handlers on `http.ServeMux`.

Conventions:

| File / directory | Effect |
|---|---|
| `pages/foo/index.templ` | `GET /foo` |
| `pages/foo/bar.templ` | `GET /foo/bar` (rare — prefer index.templ subdirs) |
| `pages/foo/[id].templ` | `GET /foo/{id}` with path parameter `id` |
| `pages/foo/+layout.templ` | wraps every `index.templ` at or below `foo/` |
| `pages/(realm)/` | optional realm prefix: matches both `/...` and `/{realm}/...` (Next-style route group with special semantics for the realm param) |

Each `index.templ` exports two things via a Go sibling file (`index.go`):
- A `Component(...)` returning `templ.Component` — the rendered HTML.
- A `Handler(w, r)` that loads data, then renders either:
  - the full page (`layouts.Root(Component(data))`) on a normal request, or
  - the fragment (`Component(data)`) when `HX-Request: true`.

Layouts compose top-down via templ children: `@layouts.Realm() { @pages.BossKills(data) }`.

Form actions: handlers expose explicit POST routes (e.g., `pages/(realm)/guild-token/index.go` registers `POST /{realm}/guild-token`), returning either a 303 redirect or an htmx-targeted fragment depending on `HX-Request`.

## ClickHouse schema strategy

Schema is redesigned freely; not a port. Decisions:

- **Lookup tables** (`boss`, `raid`, `realm`, `boss_prop`, `realm_x_raid`) — `MergeTree` with small datasets, loaded into Go `lookup.Registry` at startup, refreshed every 5 min.
- **Events** — `boss_kill` and its children. Two options:
  - **A (preferred):** single wide `boss_kill` table with `Nested` columns for `players`, `deaths`, `loot`, `timeline`. Engine `ReplacingMergeTree(version)` ORDER BY `(realm, remote_id)` PARTITION BY `toYYYYMM(kill_time)`.
  - **B (fallback):** separate `boss_kill_player`/`boss_kill_loot`/etc. tables if query patterns favor them. Decided in Phase 0.
- **Reads** use `FINAL` or `argMax()` per query to see the latest version after dedup. We pick one and standardize.
- **TTL** — optional `kill_time + INTERVAL 2 YEAR DELETE` if retention becomes an issue. Not enabled at launch.

## Materialized views (replaces ranking CLI jobs)

All three ranking CLI jobs are deleted and replaced by MVs that maintain target `AggregatingMergeTree` tables on insert:

| MV | Replaces | Target shape |
|---|---|---|
| `mv_character_boss_rankings` | `cache-character-boss-rankings.ts` | `(realm, boss_id, difficulty, spec_id, metric, character_id) → max(value)` via `argMaxState` |
| `mv_character_boss_percentiles` | `cache-character-boss-kill-percentiles.ts` | `(realm, boss_id, difficulty, spec_id, metric) → quantilesTDigestState(value)` |
| `mv_raid_lock_rankings` | `cache-raid-lock-rankings.ts` | `(realm, raid_lock_start, difficulty, spec_id, metric, character_id) → argMaxState(value)` |

Reads merge `*State` columns via `argMaxMerge` / `quantilesTDigestMerge`. No application-side caching of rankings.

Trade-offs:
- MVs add no latency to sync inserts (CH MV updates run inline with insert).
- Schema migrations on the target tables require care — we deploy them with the `mv` defined, and rebuild from scratch during dev cycles.

## Sync workflow

`cmd/sync` is the only writer. Pulls from the Twinstar API (`twinstar-api.twinstar-wow.com`), validates with Go structs + `encoding/json`, batches into `INSERT INTO boss_kill VALUES (...)`. MVs handle the downstream aggregations.

CLI flags mirror the existing `synchronize-with-api.ts`: `--realms`, `--from-date`, `--to-date`, `--bosskill-ids`, `--page-size`, `--offset`. Same scheduler config the user has today (cron / systemd timer) keeps working.

## Auth (guild token)

HMAC port: `SHA256(realm + guild + secret)` byte-for-byte compatible with the existing TS code in `packages/sveltekit/src/lib/server/guild-token.service.ts`. Existing tokens in user cookies remain valid after cutover.

Routes:
- `GET /{realm}/guild-token` — display token + form
- `POST /{realm}/guild-token` — verify + set cookies (`guild-token`, `guild-name`, 1-year, httpOnly)
- `GET /{realm}/guild-token/generate` — admin form (gated by `SECRET_TOKEN_ADMIN`)
- `POST /{realm}/guild-token/generate` — verify admin + return generated guild token

Middleware `guildtoken` reads cookies, attaches `realmIsPrivate`, `guild`, `tokenVerified` to request context. Routes that require it (boss-kills, character pages on private realms) check and 404/403 as appropriate. Public realms bypass.

## Caching strategy

Two caches, both in-process:

| Cache | Backend | Sizing | Eviction |
|---|---|---|---|
| Query results (lookup data, API responses) | LRU (`container/list` + `sync.Mutex`) | `BK_CACHE_MAX_ENTRIES` env, default 10k | LRU + TTL via `time.AfterFunc` |
| Icon blobs (`/img/icon`) | Disk (`BK_ICON_DIR`, default `./var/icons`) | unbounded (icons are static) | manual purge via `cmd/sync purge-icons` if ever needed |

Single-instance assumption. If the deployment ever scales to >1 replica, both caches need to move out (and the icon dir needs to be shared).

## Charts

echarts is kept as-is, served from `embed.FS`. A plain web component reads its config:

```html
<bk-chart>
  <script type="application/json">{"type":"box","series":[...]}</script>
</bk-chart>
```

The custom element (`internal/web/static/components.js`, ~40 LOC) constructs an echarts instance from the JSON on `connectedCallback` and handles resize. The Go template emits the JSON via templ's safe-string interpolation.

Same `bk-chart` element handles all chart variants — type is selected from the JSON. Five chart types from the old app: box, bar, line, line+bands, scatter. Each is just a different echarts config; no Go code knows about chart internals.

## Tables

Server-rendered HTML tables, sort/filter via:
- htmx form submissions for filter changes (`hx-get`, `hx-target=#table`, `hx-push-url=true`) — URL stays bookmarkable
- For columns sortable without round-trip: a small vanilla JS sorter (~50 LOC) reads `data-sort` attributes and reorders rows. Activated by `<table data-sortable>`.

No `@tanstack/svelte-table` replacement. Pagination is a server-rendered `components.Pagination` partial.

## Icon proxy (`/img/icon`)

GET handler:
1. Validates `type` and `id` against an allowlist (port from existing route — verify SSRF protection).
2. Computes cache key: `sha256(type + id + realm)`.
3. Reads `BK_ICON_DIR/<key>.bin` if present (with content-type from sibling `.meta` file).
4. On miss: fetches remote URL, writes both files, returns blob.
5. Sets `Cache-Control: public, max-age=1209600` (14 days, matches current).

## Middleware

Wired in order:
1. `recover` — panic to slog + 500
2. `requestid` — `X-Request-Id` or new UUID, added to context
3. `logger` — slog with method, path, status, duration, request-id
4. `headers` — security headers (CSP, X-Content-Type-Options, etc.)
5. `realm` — resolves realm from path param, handles realm-merge redirects, attaches to context
6. `guildtoken` — reads cookies, attaches verification result to context

Per-route auth checks live in handlers (not middleware) so we can return route-specific errors.

## Build & dev pipeline

Makefile targets:
- `make setup` — downloads `bin/tailwindcss`, runs `go mod download`, installs `templ` and `wgo`
- `make dev` — runs templ generate watch + tailwind watch + wgo (3 background processes)
- `make build` — templ generate, tailwind build (minified), `go build -trimpath -ldflags="-s -w -X main.version=$(git rev-parse HEAD)"`
- `make test` — `go test ./...`
- `make migrate` — `go run ./cmd/migrate up`
- `make sync` — `go run ./cmd/sync` (passthrough for local testing)

CI: GitHub Actions adding `templ generate`, `go vet`, `go test`, and a binary build job. Out of scope for the rewrite itself — propose as a follow-up.

## Migrations

`migrations/` contains files named `001_init.sql`, `002_xxx.sql`, etc. Each is a single ClickHouse SQL script that may contain multiple statements.

`cmd/migrate` reads from `embed.FS`, tracks applied versions in `_migrations` table:

```sql
CREATE TABLE IF NOT EXISTS _migrations (
  version UInt32,
  applied_at DateTime DEFAULT now()
) ENGINE = MergeTree ORDER BY version
```

Commands: `up`, `down` (best-effort; CH doesn't always have a clean inverse), `status`.

No external migration tool. ~100 LOC, stdlib.

## Time zones

Raid lockout boundaries depend on server-reset times (per-realm/per-region). Port the existing `date-fns-tz`-based logic to Go's `time.Location`. Write tests covering:
- DST transitions
- The Tuesday reset (US) vs Wednesday reset (EU) split
- Realm-specific overrides (current code uses a hardcoded TZ — verify)

This is the single most landmine-prone piece of code. It gets tests.

## Parity verification

Before cutover, run both apps against the same data and diff:
- A list of ~30 fixed URLs covering every route + a few sort/filter variations
- For each: fetch from SvelteKit + Go, parse with `goquery` (or stdlib `html`), compare key DOM nodes (tables, chart configs, headings, link hrefs)
- Manual visual diff for chart rendering

Tolerances: floating-point output rounded to 2 decimals; ordering matches (sort stability); identical HMAC tokens for the same input.

## Phasing

| Phase | Scope | Est. |
|---|---|---|
| 0 | CH schema spike: design 2–3 representative tables + the three MVs; verify they produce identical numbers to current rankings on a sample dataset | 1 day |
| 1 | Go module, file-router, templ + Tailwind + htmx wiring, base layout, slog/middleware, lookup registry | 1–2 days |
| 2 | Migrations + sync job — populate CH from API | 1–2 days |
| 3 | Charts pilot: `bk-chart` web component + `/boss/:id` end-to-end | 1–2 days |
| 4 | Remaining read routes: `/`, `/raids`, `/ranks`, `/boss-kills`, `/character/*` | 3–4 days |
| 5 | Auth flows: guild-token routes + middleware + character form | 1 day |
| 6 | Icon proxy + on-disk cache | ½ day |
| 7 | Parity verification + fixes | 1–2 days |
| 8 | Cutover | ½ day |

Total: ~12–17 working days.

## Risks

1. **ClickHouse MV correctness for percentiles.** `quantilesTDigestState` is approximate; the existing TS code is exact. Acceptable in this domain (rankings are inherently noisy), but verify in Phase 0.
2. **`Nested` columns vs separate child tables.** Decision deferred to Phase 0 based on real query shapes.
3. **No tests in source app.** Parity verification is our only safety net. Acceptable risk given the user is the maintainer and can sanity-check pages.
4. **echarts JSON payload size.** Chart configs can be big. Mitigate by emitting only the data points needed (already true), and gzip the response.
5. **In-process cache + multi-instance.** Documented constraint. If deployment changes, both caches need to move out — flagged in the cache section.
6. **Time-zone bugs in raid-lock calc.** Mitigated by tests in Phase 1.

## Out of scope

- Adding user accounts / sessions beyond the existing HMAC guild token
- Streaming / SSE / WebSockets (no current usage)
- Multi-instance / multi-region deployment
- Migrating prod MariaDB data — re-sync from the upstream API instead
- CI/CD pipeline setup (propose as follow-up)
- Containerisation / Dockerfile (propose as follow-up)

## Constraints from user instructions

- No commits during the rewrite. User reviews and commits themselves.
- Existing SvelteKit code is preserved in `packages/sveltekit/`. New code lives in `go/`.
- Tailwind via standalone CLI binary — no Node tooling introduced.
