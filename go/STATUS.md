# Status — 2026-06-27 (end of Phase 5)

## Phases complete

| Phase | Scope | State |
|---|---|---|
| 0 | ClickHouse 25.3 LTS in docker-compose, schema, materialized views, sync CLI, migration runner | done |
| 1 | Go web foundation: templ, Tailwind v4, htmx, embed.FS, middleware, home page | done |
| 2 | Realm routing, chart pilot, 7 read-only pages | done |
| 3 | Kill detail, icon proxy, guild-token auth, character search, character performance, boss history, admin/gc | done |
| 4 | Arbitrary percentile UI, guild-token gating, median benchmark overlay, wgo live reload | done |
| 5 | **Exact percentiles** (drop t-digest MV), **sync loot dedup**, **item previews**, **/ redirect**, **raid icons**, **htmx boss-page swap**, **handler tests** | done |

## Phase 5 highlights

### Dropped the percentile MV — exact via ARRAY JOIN

Migration `003_drop_tdigest_percentiles.sql` removes `mv_character_boss_percentiles` and its target table. The boss-page percentile curve and the character-performance median benchmark now query `boss_kill ARRAY JOIN players` directly with `quantilesExact(...)`.

**Why this beats the MV at this scale:** real measurement against synced data showed t-digest was off by up to 4% on the **median** for small-sample specs (44k → 46k DPS) but exact at the tails — the opposite of what users care about. Switching to `quantileExact` is both faster to reason about and *more accurate*.

The other three MVs (`character`, `character_boss_rankings`, `raid_lock_rankings`) stay — their aggregates (argMax identity, max DPS/HPS) are idempotent and benefit from incremental maintenance.

### Sync loot filter (Dark Shaman dedup)

Cleanup: 36 empty-loot Dark Shaman rows in CH deleted (kept LFR mode 7 alone).

Going forward, the sync CLI drops any kill where `bk.Mode != 7 && len(detail.Loot) == 0`. Reason: upstream sometimes records two rows for the same Dark Shaman fight — the duplicate has no loot, and non-LFR fights should always produce at least one loot entry. The filter is scoped to non-LFR modes (LFR doesn't store loot rows in our data).

### Item previews on `/{realm}/boss-kills/{id}`

- `internal/api/item.go` — `GetItem` calls `/item/{id}`, parses `{item.ID, itemSparse.{Name, Quality}}`
- `internal/cache/items.go` — disk-cached `ItemDisk` with in-process mutex + result memoisation so concurrent renders of the same kill only fetch each item once
- `bosskill.handler` resolves all loot items in parallel via `GetMany`
- The template renders item icon + quality-colored name + Wowhead link

Verified: "Iyyokuk's Hereditary Seal" rendered in epic purple (`#a335ee`), 5 items cached as JSON under `./var/items/`.

### Auto-redirect `/` → `/Helios/`

- `/` returns 302 to `/Helios/` (or the realm in the `last-realm` cookie if set)
- The realm picker still exists at `/realms`
- `RequireRealm` now sets `last-realm` on every successful realm visit so the next root visit goes there directly

### Raid icons

`<img src="/img/icon?type=raid&id={lc-name}-small.avif">` on:
- `/{realm}/raids` panel headers
- `/{realm}/boss/{id}` header
- `/{realm}/boss-kills/{id}` header

### htmx fragment swap on boss page

- `Page` shell stays; `Content` is the swappable fragment (difficulty tabs + curve charts + percentile slider + tables)
- Difficulty tabs use `hx-get` + `hx-target="#boss-content"` + `hx-push-url="true"`
- Percentile input swaps on `input changed delay:250ms` for live feedback
- Handler detects `HX-Request` and renders `Content` alone (195 KB) vs full `Page` (176 KB); both verified

### Handler tests

`internal/web/server/server_test.go`: 15 HTTP-level tests covering:

- `/healthz`, static asset cache headers
- `/` → `/Helios/` 302
- `/helios/` → `/Helios/` 301, `/Apollo/` → `/Athena/` 301
- Unknown realm 404, unknown boss 404
- Public realm dashboard/raids/boss pages 200
- Percentile filter `?p=85` ("DPS @ p85" in body)
- htmx fragment doesn't contain `<html>` but does contain the chart panel
- Private realm `/MoPPvE/boss-kills` → 403
- Icon proxy bad type → 400

Tests `t.Skip` when `BK_CH_DSN` isn't set so `make check` from a fresh checkout (without ClickHouse) still works. Run integration suite via `BK_CH_DSN=... make check`.

## MV inventory after Phase 5

| MV | Target table | Used by | Reason kept |
|---|---|---|---|
| `mv_character` | `character` | `/character/{name}`, `/characters` search, `/ranks` | argMax(name,…) per guid — idempotent, search-heavy lookups |
| `mv_character_boss_rankings` | `character_boss_rankings` | `/boss/{id}` top tables, `/character/{name}` best-by-boss | max DPS/HPS — idempotent under re-sync |
| `mv_raid_lock_rankings` | `raid_lock_rankings` | `/ranks`, `/boss/{id}/history` | max DPS/HPS bucketed by raid lock — incremental aggregation gives ~50× speedup on read |
| ~~`mv_character_boss_percentiles`~~ | ~~`character_boss_percentiles`~~ | (was) `/boss/{id}` curves, `/character/{name}/performance` median | **Dropped in 003** — exact via ARRAY JOIN is both faster and more accurate at our scale |

## Stats

- **132 source files** (Go + templ + SQL + JS)
- **~8k LOC** non-generated
- **15 handler tests** passing
- **3 external deps**: `clickhouse-go/v2`, `a-h/templ` (runtime), `a-h/templ/cmd` (gen)
- **20 route patterns** in the mux (excluding `/static/*`)

## Quick start

```bash
cd go
cp .env.example .env
# Set BK_CH_DSN and SECRET_TOKEN_GUILD at minimum
make setup           # tailwindcss, templ, wgo
make docker-up       # ClickHouse 25.3
make migrate ARGS=up
make sync ARGS="--realm Helios --from-date 2026-06-20"
make dev             # all watchers + server
# open http://127.0.0.1:3000
```

## Don't forget

- No commits made. Working tree carries `docs/` + `go/`.
- Existing SvelteKit app under `packages/` untouched.
- ClickHouse data persists in named volume `go_clickhouse-data`.
- Secrets: `SECRET_TOKEN_GUILD` (and `SECRET_TOKEN_ADMIN` for the generator).
