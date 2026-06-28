# Table Unification Design

**Date:** 2026-06-28
**Scope:** `go/` directory — templ pages and CSS only

## Problem

Two rendering patterns exist for tables across 10 page packages:

- **4 pages** use `@components.Table()` — clean, no per-cell overrides, relies on CSS for all styling.
- **6 pages** use raw inline `<div class="bk-table-wrap"><table class="w-full text-sm">` — adds dead Tailwind classes (`py-1`, `text-left`, `text-bk-muted` on `<thead><tr>`) that are overridden by the CSS, plus `font-mono` on `<tbody>` and `border-t border-bk-border` on `<tr>`.

The one real visual difference is `font-mono` on `<tbody>`: present in 6 inline-table pages, absent in the 4 component-based pages.

Additionally, the "no data" empty state div is duplicated 8+ times with inconsistent padding (`py-2`, `py-4`, `py-6`, `py-8`).

## Goals

1. All tables render identically — same font, same padding, same header color.
2. All tables go through `@components.Table()` — no inline `bk-table-wrap` markup in page templates.
3. Empty-state messages use a single shared component.

## Non-goals

- Extracting shared viewmodel types (`DifficultyChoice`, `Option`) — too much handler churn for little gain.
- Changing table structure or column layout on any page.
- Touching the `SortableTable` or `TablePanel` variants — they're already component-based.

## Changes

### 1. `go/input.css` — add mono to table bodies

Add one rule inside the existing `.bk-table-wrap` block:

```css
.bk-table-wrap tbody {
  font-family: monospace;
}
```

This makes all tables (both existing component-based and newly converted) monospace in the body. No per-template changes needed for font.

### 2. `go/internal/web/views/components/panel.templ` — add `EmptyState`

Add after `StatRow`:

```templ
templ EmptyState(msg string) {
    <div class="py-6 text-center text-sm text-bk-muted">{ msg }</div>
}
```

Standardizes padding to `py-6`.

### 3. Six pages: replace inline tables with `@components.Table()`

Pages to convert: `bosshistory`, `character`, `characters`, `home`, `raids`, `ranks`.

For each inline table:
- Replace `<div class="bk-table-wrap"><table class="w-full text-sm">...</table></div>` with `@components.Table() { ... }`.
- Remove dead per-cell classes: `py-1` on `<th>`/`<td>`, `text-left text-bk-muted` on `<thead><tr>`, `font-mono` on `<tbody>`, `border-t border-bk-border` on `<tr>`.
- Keep meaningful alignment overrides: `text-right` on specific `<th>`/`<td>` cells.

### 4. Replace inline empty-state divs with `@components.EmptyState()`

Replace all instances of:
```html
<div class="py-N text-center text-sm text-bk-muted">message</div>
```
with:
```templ
@components.EmptyState("message")
```

**Exceptions** (kept as-is — different structure):
- `bosskills/page.templ` standalone bordered panel (has `rounded-md border border-bk-border bg-bk-panel p-6`)
- `boss/page.templ` `emptyChart()` full-height flex container (`flex h-72 items-center justify-center`)
- `bosskill/page.templ` inline `py-2` loot message (non-centered inline text)

## Affected Files

| File | Change |
|------|--------|
| `go/input.css` | Add mono rule to `.bk-table-wrap tbody` |
| `go/internal/web/views/components/panel.templ` | Add `EmptyState` component |
| `go/internal/web/views/pages/bosshistory/page.templ` | Convert table, use EmptyState |
| `go/internal/web/views/pages/character/page.templ` | Convert 2 tables, use EmptyState ×2 |
| `go/internal/web/views/pages/characters/page.templ` | Convert table, use EmptyState |
| `go/internal/web/views/pages/home/page.templ` | Convert table |
| `go/internal/web/views/pages/raids/page.templ` | Convert table |
| `go/internal/web/views/pages/ranks/page.templ` | Convert table, use EmptyState |
| `go/internal/web/views/pages/boss/page.templ` | Use EmptyState (table already component-based) |
| `go/internal/web/views/pages/bosskill/page.templ` | Use EmptyState for timeline |
| `go/internal/web/views/pages/characterperf/page.templ` | Use EmptyState ×2 |

## Regeneration

After editing `.templ` files, run `templ generate` (or `make templ`) to regenerate the `*_templ.go` files.
