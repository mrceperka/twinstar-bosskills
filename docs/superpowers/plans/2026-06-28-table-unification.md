# Table Unification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Unify all table rendering across 10 page templates so every table uses `@components.Table()`, has monospace body text, and empty states use a shared `EmptyState` component.

**Architecture:** All changes are in `.templ` source files plus `input.css`. Six pages that use raw inline `bk-table-wrap` markup are converted to `@components.Table()`. A new `EmptyState` component replaces 8+ duplicated "no data" divs. Monospace body text is added once to CSS so it applies everywhere without per-template changes. After every `.templ` edit, run `make gen-templ` from `go/` to regenerate `*_templ.go` files.

**Tech Stack:** Go `templ` (`.templ` → `*_templ.go` via `templ generate`), Tailwind v4 (`input.css` → `internal/web/static/app.css`)

**All commands run from `go/` directory.**

---

## File Map

| File | What changes |
|------|-------------|
| `go/input.css` | Add `font-family: monospace` to `.bk-table-wrap tbody` |
| `go/internal/web/views/components/panel.templ` | Add `EmptyState(msg string)` component |
| `go/internal/web/views/pages/bosshistory/page.templ` | Convert table, use EmptyState |
| `go/internal/web/views/pages/character/page.templ` | Convert 2 tables, use EmptyState ×2 |
| `go/internal/web/views/pages/characters/page.templ` | Convert table, use EmptyState ×2 |
| `go/internal/web/views/pages/home/page.templ` | Convert table |
| `go/internal/web/views/pages/raids/page.templ` | Convert table, add `components` import |
| `go/internal/web/views/pages/ranks/page.templ` | Convert table, use EmptyState |
| `go/internal/web/views/pages/boss/page.templ` | Use EmptyState in `topTable` + `percentileTable` |
| `go/internal/web/views/pages/bosskill/page.templ` | Use EmptyState for timeline empty state |
| `go/internal/web/views/pages/characterperf/page.templ` | Use EmptyState ×2 for chart empty states |

---

## Task 1: Add mono CSS rule + `EmptyState` component

**Files:**
- Modify: `go/input.css`
- Modify: `go/internal/web/views/components/panel.templ`

- [ ] **Step 1: Add monospace rule to `input.css`**

In `go/input.css`, find the `.bk-table-wrap tbody tr:nth-child(even)` block (around line 113) and add the new rule directly before it:

```css
.bk-table-wrap tbody {
  font-family: monospace;
}
.bk-table-wrap tbody tr:nth-child(even) {
  background: rgba(0, 0, 0, 0.5);
}
```

- [ ] **Step 2: Add `EmptyState` to `panel.templ`**

In `go/internal/web/views/components/panel.templ`, add after the closing brace of `StatRow`:

```templ
templ EmptyState(msg string) {
	<div class="py-6 text-center text-sm text-bk-muted">{ msg }</div>
}
```

- [ ] **Step 3: Regenerate and verify**

```bash
cd go && make gen-templ && go build ./...
```

Expected: no errors. The `components` package now exports `EmptyState`.

---

## Task 2: Convert `bosshistory/page.templ`

**Files:**
- Modify: `go/internal/web/views/pages/bosshistory/page.templ`

- [ ] **Step 1: Replace `topTable` function**

Replace the entire `topTable` function with:

```templ
templ topTable(realmName string, rows []Row, metric string) {
	if len(rows) == 0 {
		@components.EmptyState("No data for this lock + difficulty.")
	} else {
		@components.Table() {
			<thead>
				<tr>
					<th class="w-8">#</th>
					<th>Name</th>
					<th>Spec</th>
					<th class="text-right">{ metric }</th>
				</tr>
			</thead>
			<tbody>
				for _, r := range rows {
					<tr>
						<td class="text-bk-muted">{ intStr(r.Rank) }</td>
						<td>{ r.Name }</td>
						<td class="text-xs">
							<span class="inline-flex items-center gap-1">
								@components.ClassSpecIcons(realmName, r.Class, r.Spec)
								<span class="text-bk-muted">{ r.SpecLabel }</span>
							</span>
						</td>
						<td class="text-right">
							if metric == "DPS" {
								{ format.Int64Ctx(ctx, r.DPS) }
							} else {
								{ format.Int64Ctx(ctx, r.HPS) }
							}
						</td>
					</tr>
				}
			</tbody>
		}
	}
}
```

- [ ] **Step 2: Regenerate and verify**

```bash
cd go && make gen-templ && go build ./...
```

Expected: no errors.

---

## Task 3: Convert `character/page.templ`

**Files:**
- Modify: `go/internal/web/views/pages/character/page.templ`

- [ ] **Step 1: Replace the two inline panel bodies in `Page`**

Replace the entire content of the two `@components.Panel` blocks (lines 72–166). The full updated section (from the `<div class="grid ...">` onward):

```templ
			<div class="grid gap-6 lg:grid-cols-2">
				@components.Panel("Recent kills") {
					if len(vm.RecentKills) == 0 {
						@components.EmptyState("No kills yet.")
					} else {
						@components.Table() {
							<thead>
								<tr>
									<th>Time</th>
									<th>Boss</th>
									<th>Diff</th>
									<th>Spec</th>
									<th class="text-right">DPS</th>
									<th class="text-right">HPS</th>
								</tr>
							</thead>
							<tbody>
								for _, k := range vm.RecentKills {
									<tr>
										<td class="text-bk-muted text-xs">{ k.KillTime }</td>
										<td>
											<a href={ bossHref(vm.Realm, k.BossID) } class="hover:text-bk-accent">{ k.BossName }</a>
										</td>
										<td>{ k.ModeLabel }</td>
										<td class="text-xs">
											<span class="inline-flex items-center gap-1">
												@components.ClassSpecIcons(vm.Realm, k.Class, k.Spec)
												<span class="text-bk-muted">{ k.SpecLabel }</span>
											</span>
										</td>
										<td class="text-right">
											if k.DPS > 0 {
												{ format.Int64Ctx(ctx, k.DPS) }
											} else {
												<span class="text-bk-muted">—</span>
											}
										</td>
										<td class="text-right">
											if k.HPS > 0 {
												{ format.Int64Ctx(ctx, k.HPS) }
											} else {
												<span class="text-bk-muted">—</span>
											}
										</td>
									</tr>
								}
							</tbody>
						}
					}
				}
				@components.Panel("Best by boss") {
					if len(vm.BestPerBoss) == 0 {
						@components.EmptyState("No rankings yet.")
					} else {
						@components.Table() {
							<thead>
								<tr>
									<th>Boss</th>
									<th>Diff</th>
									<th>Spec</th>
									<th class="text-right">DPS</th>
									<th class="text-right">HPS</th>
								</tr>
							</thead>
							<tbody>
								for _, b := range vm.BestPerBoss {
									<tr>
										<td>
											<a href={ bossHref(vm.Realm, b.BossID) } class="hover:text-bk-accent">{ b.BossName }</a>
										</td>
										<td>{ b.ModeLabel }</td>
										<td class="text-xs">
											<span class="inline-flex items-center gap-1">
												@components.ClassSpecIcons(vm.Realm, b.Class, b.Spec)
												<span class="text-bk-muted">{ b.SpecLabel }</span>
											</span>
										</td>
										<td class="text-right">
											if b.DPS > 0 {
												{ format.Int64Ctx(ctx, b.DPS) }
											} else {
												<span class="text-bk-muted">—</span>
											}
										</td>
										<td class="text-right">
											if b.HPS > 0 {
												{ format.Int64Ctx(ctx, b.HPS) }
											} else {
												<span class="text-bk-muted">—</span>
											}
										</td>
									</tr>
								}
							</tbody>
						}
					}
				}
			</div>
```

- [ ] **Step 2: Regenerate and verify**

```bash
cd go && make gen-templ && go build ./...
```

Expected: no errors.

---

## Task 4: Convert `characters/page.templ`

**Files:**
- Modify: `go/internal/web/views/pages/characters/page.templ`

- [ ] **Step 1: Replace `Matches` function**

Replace the entire `Matches` function:

```templ
templ Matches(vm ViewModel) {
	if vm.Query == "" {
		@components.EmptyState("Start typing to search.")
	} else if len(vm.Matches) == 0 {
		@components.EmptyState(`No characters match "` + vm.Query + `".`)
	} else {
		@components.Table() {
			<thead>
				<tr>
					<th>Name</th>
					<th>Class</th>
					<th class="text-right">Kills</th>
					<th class="text-right">Last seen</th>
				</tr>
			</thead>
			<tbody>
				for _, m := range vm.Matches {
					<tr>
						<td>
							<a href={ characterHref(vm.Realm, m.Name) } class="hover:text-bk-accent">{ m.Name }</a>
						</td>
						<td class="text-bk-muted">
							<span class="inline-flex items-center gap-1">
								@components.ClassSpecIcons(vm.Realm, m.Class, 0)
								<span>{ m.ClassLabel }</span>
							</span>
						</td>
						<td class="text-right">{ format.IntCtx(ctx, m.KillCount) }</td>
						<td class="text-right text-bk-muted text-xs">{ m.LastSeen }</td>
					</tr>
				}
			</tbody>
		}
	}
}
```

- [ ] **Step 2: Regenerate and verify**

```bash
cd go && make gen-templ && go build ./...
```

Expected: no errors.

---

## Task 5: Convert `home/page.templ`

**Files:**
- Modify: `go/internal/web/views/pages/home/page.templ`

- [ ] **Step 1: Replace the Realms panel body**

Replace the `@components.Panel("Realms")` block content — the inner `<div class="bk-table-wrap">...<table>...</table></div>`:

```templ
			@components.Panel("Realms") {
				@components.Table() {
					<thead>
						<tr>
							<th>Realm</th>
							<th class="text-right">Expansion</th>
							<th class="text-right">Bosses</th>
							<th class="text-right">Kills</th>
							<th class="text-right">Last seen</th>
						</tr>
					</thead>
					<tbody>
						for _, r := range vm.Realms {
							<tr>
								<td>
									<a href={ realmHref(r.Name) } class="hover:text-bk-accent">{ r.Name }</a>
									if r.IsPrivate {
										<span class="ml-1 text-xs text-bk-muted">(private)</span>
									}
								</td>
								<td class="text-right">{ expansionLabel(r.Expansion) }</td>
								<td class="text-right">{ format.IntCtx(ctx, r.Bosses) }</td>
								<td class="text-right">{ format.IntCtx(ctx, r.BossKills) }</td>
								<td class="text-right text-bk-muted">{ r.LastKillAt }</td>
							</tr>
						}
					</tbody>
				}
			}
```

- [ ] **Step 2: Regenerate and verify**

```bash
cd go && make gen-templ && go build ./...
```

Expected: no errors.

---

## Task 6: Convert `raids/page.templ`

**Files:**
- Modify: `go/internal/web/views/pages/raids/page.templ`

- [ ] **Step 1: Add `components` import**

The file currently imports only `format` and `layouts`. Add the `components` import:

```templ
import (
	"github.com/mrceperka/twinstar-bosskills/go/internal/format"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/views/components"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/views/layouts"
)
```

- [ ] **Step 2: Replace `raidTable` function**

Replace the entire `raidTable` function:

```templ
templ raidTable(r Raid, difficulties []string) {
	@components.Table() {
		<thead>
			<tr>
				<th>Boss</th>
				for _, d := range difficulties {
					<th class="text-right">{ d }</th>
				}
				<th class="text-right">Total</th>
			</tr>
		</thead>
		<tbody>
			for _, b := range r.Bosses {
				<tr>
					<td>{ b.Name }</td>
					for _, c := range b.KillsByDifficulty {
						<td class="text-right">
							if c > 0 {
								{ format.IntCtx(ctx, c) }
							} else {
								<span class="text-bk-muted">—</span>
							}
						</td>
					}
					<td class="text-right font-medium">{ format.IntCtx(ctx, b.Total) }</td>
				</tr>
			}
		</tbody>
	}
}
```

- [ ] **Step 3: Regenerate and verify**

```bash
cd go && make gen-templ && go build ./...
```

Expected: no errors.

---

## Task 7: Convert `ranks/page.templ`

**Files:**
- Modify: `go/internal/web/views/pages/ranks/page.templ`

- [ ] **Step 1: Replace `rankTable` function**

Replace the entire `rankTable` function:

```templ
templ rankTable(realmName string, rows []Rank, metric string) {
	if len(rows) == 0 {
		@components.EmptyState("No data yet.")
	} else {
		@components.Table() {
			<thead>
				<tr>
					<th class="w-8">#</th>
					<th>Name</th>
					<th>Spec</th>
					<th>Boss</th>
					<th class="text-right">{ metric }</th>
				</tr>
			</thead>
			<tbody>
				for _, r := range rows {
					<tr>
						<td class="text-bk-muted">{ format.IntCtx(ctx, r.Rank) }</td>
						<td>{ r.Name }</td>
						<td class="text-xs">
							<span class="inline-flex items-center gap-1">
								@components.ClassSpecIcons(realmName, r.Class, r.Spec)
								<span class="text-bk-muted">{ r.SpecLabel }</span>
							</span>
						</td>
						<td class="text-bk-muted">
							<a href={ bossHref(realmName, r.BossID) } class="hover:text-bk-accent">{ r.BossName }</a>
						</td>
						<td class="text-right">
							if metric == "DPS" {
								{ format.Int64Ctx(ctx, r.DPS) }
							} else {
								{ format.Int64Ctx(ctx, r.HPS) }
							}
						</td>
					</tr>
				}
			</tbody>
		}
	}
}
```

- [ ] **Step 2: Regenerate and verify**

```bash
cd go && make gen-templ && go build ./...
```

Expected: no errors.

---

## Task 8: Use `EmptyState` in `boss`, `bosskill`, `characterperf`

These pages already use `@components.Table()` — only the empty-state divs need updating.

**Files:**
- Modify: `go/internal/web/views/pages/boss/page.templ`
- Modify: `go/internal/web/views/pages/bosskill/page.templ`
- Modify: `go/internal/web/views/pages/characterperf/page.templ`

- [ ] **Step 1: `boss/page.templ` — update `topTable` empty state**

In `topTable`, replace:
```templ
		<div class="py-4 text-center text-sm text-bk-muted">No data for this difficulty yet.</div>
```
with:
```templ
		@components.EmptyState("No data for this difficulty yet.")
```

- [ ] **Step 2: `boss/page.templ` — update `percentileTable` empty state**

In `percentileTable`, replace:
```templ
		<div class="py-4 text-center text-sm text-bk-muted">No samples.</div>
```
with:
```templ
		@components.EmptyState("No samples.")
```

- [ ] **Step 3: `bosskill/page.templ` — update timeline empty state**

In `Page`, in the `"Fight timeline"` section, replace:
```templ
				<div class="py-8 text-center text-sm text-bk-muted">No timeline data.</div>
```
with:
```templ
				@components.EmptyState("No timeline data.")
```

- [ ] **Step 4: `characterperf/page.templ` — update both chart empty states**

In `Page`, replace:
```templ
					<div class="py-8 text-center text-sm text-bk-muted">No DPS samples.</div>
```
with:
```templ
					@components.EmptyState("No DPS samples.")
```

And replace:
```templ
					<div class="py-8 text-center text-sm text-bk-muted">No HPS samples.</div>
```
with:
```templ
					@components.EmptyState("No HPS samples.")
```

- [ ] **Step 5: Regenerate and run full check**

```bash
cd go && make check
```

Expected: `templ generate` succeeds, `go vet ./...` reports nothing, `go test ./...` passes all existing tests.

---

## Task 9: Rebuild CSS and verify visually

- [ ] **Step 1: Rebuild CSS**

```bash
cd go && make gen-css
```

Expected: `internal/web/static/app.css` is updated with the new `.bk-table-wrap tbody { font-family: monospace; }` rule.

- [ ] **Step 2: Start the server and spot-check**

```bash
cd go && make server
```

Visit these pages and confirm tables all look the same — gold headers, monospace body text, uniform cell padding:
- `/` (home — realms table)
- `/<realm>/boss-kills` (bosskills — kills table)
- `/<realm>/raids` (raids — boss kill counts)
- `/<realm>/ranks` (ranks — top DPS/HPS)
- `/<realm>/characters?q=a` (characters — search results)
- `/<realm>/boss/<id>` (boss — top DPS/HPS tables, percentile table)
- `/<realm>/boss/<id>/history` (bosshistory — top DPS/HPS)
- `/<realm>/character/<name>` (character — recent kills, best by boss)
