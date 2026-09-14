package bosskills

import (
	"context"
	"strings"
	"testing"
)

// The kills grid is server-paginated, so sorting must round-trip through
// ?sort=/?dir= (sorting one page client-side would only reorder 20 of N rows).
// Every sortable header therefore needs a whitelisted column key.
func TestParseFilterAcceptsEverySortHeaderColumn(t *testing.T) {
	for _, col := range []string{"boss_name", "raid_name", "guild", "length", "kill_time"} {
		f := parseFilter(map[string][]string{"sort": {col}, "dir": {"asc"}})
		if f.SortBy != col {
			t.Errorf("sort=%s rejected, SortBy = %q", col, f.SortBy)
		}
		if f.SortDir != "asc" {
			t.Errorf("sort=%s: SortDir = %q, want asc", col, f.SortDir)
		}
		if validBKSortCols[col] == "" {
			t.Errorf("sort=%s has no ORDER BY column", col)
		}
	}
	for _, rejected := range []string{"length); DROP", "mode"} {
		if f := parseFilter(map[string][]string{"sort": {rejected}}); f.SortBy != defaultBKSort {
			t.Errorf("sort=%s accepted, SortBy = %q", rejected, f.SortBy)
		}
	}
}

func TestTableFragmentRendersSortHeaders(t *testing.T) {
	vm := ViewModel{
		Realm:  "Helios",
		Filter: FilterValues{SortBy: defaultBKSort, SortDir: defaultBKDir},
		Rows: []KillRow{{
			RemoteID: "18_1", KillTime: "2026-07-03 12:00", BossName: "B", BossID: 1,
			RaidName: "R", Mode: 3, ModeLabel: "10 N", Guild: "G", LengthSec: 305,
		}},
		PageSize: defaultPageSize,
		Total:    1,
	}

	var sb strings.Builder
	if err := TableFragment(vm).Render(context.Background(), &sb); err != nil {
		t.Fatal(err)
	}
	html := sb.String()
	for _, want := range []string{
		`hx-get="/Helios/boss-kills?dir=desc&amp;sort=boss_name"`,
		`hx-get="/Helios/boss-kills?dir=desc&amp;sort=raid_name"`,
		`hx-get="/Helios/boss-kills?dir=desc&amp;sort=guild"`,
		`hx-get="/Helios/boss-kills?dir=desc&amp;sort=length"`,
		`hx-target="#kills-table"`,
		// Default sort is kill_time desc, so only that header shows an arrow.
		`>Killed <span class="text-bk-accent">↓</span>`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in HTML:\n%s", want, html)
		}
	}
	// Server-paginated grid: the client-side sorter must not touch it.
	if strings.Contains(html, "data-sortable") {
		t.Fatalf("kills grid must not be client-sortable:\n%s", html)
	}
}
