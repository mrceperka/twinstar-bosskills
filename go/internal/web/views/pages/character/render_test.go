package character

import (
	"context"
	"strings"
	"testing"
)

func TestKillsTableFragmentRenderSortHeaders(t *testing.T) {
	vm := ViewModel{
		Realm: "Helios",
		Char:  CharacterInfo{Name: "Foo"},
		Filter: KillsFilter{
			SortBy:  "kill_time",
			SortDir: "desc",
		},
		RecentKills: []KillRow{{
			RemoteID: "1", KillTime: "now", BossName: "B", BossID: 1,
			Mode: 3, ModeLabel: "10 N", Spec: 62, SpecLabel: "Arcane",
			DPS: 1000, HPS: 0, LengthSec: 300, AvgItemLvl: 500,
		}},
		KillsPageSize: 20,
		KillsTotal:    1,
	}
	var sb strings.Builder
	if err := KillsTableFragment(vm).Render(context.Background(), &sb); err != nil {
		t.Fatal(err)
	}
	html := sb.String()
	// Should have an hx-get anchor for DPS sort.
	if !strings.Contains(html, `hx-get="/Helios/character/Foo?dir=desc&amp;sort=dps"`) {
		t.Errorf("missing DPS sort hx-get. HTML:\n%s", html)
	}
	// Target must be the outer kills-table div.
	if !strings.Contains(html, `hx-target="#kills-table"`) {
		t.Errorf("missing hx-target. HTML:\n%s", html)
	}
	// Default sort (kill_time desc) should render arrow next to Time header.
	if !strings.Contains(html, `>Time <span class="text-bk-accent">↓</span>`) {
		t.Errorf("missing Time arrow. HTML:\n%s", html)
	}
}
