package ranks

import (
	"context"
	"strings"
	"testing"
)

// Same contract as the boss top-stats table: cells with locale-formatted or
// humanized text must carry a raw data-sort value for static/bk-ui.js.
func TestBossRankTableRendersSortKeys(t *testing.T) {
	vm := ViewModel{Realm: "Helios"}
	rows := []Rank{{
		Rank: 1, Name: "Foo", KillID: "18_1",
		Class: 8, ClassLabel: "Mage", Spec: 62, SpecLabel: "Arcane",
		DPS: 1234567, HPS: 7654321, LengthSec: 305, Ilvl: 540,
	}}

	var sb strings.Builder
	if err := bossRankTable(vm, rows, "DPS").Render(context.Background(), &sb); err != nil {
		t.Fatal(err)
	}
	html := sb.String()
	for _, want := range []string{
		"data-sortable",
		`data-sort="Mage Arcane"`,
		`data-sort="1234567"`,
		`data-sort="305"`,
		"data-no-sort",
		"1,234,567",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in DPS rank table HTML:\n%s", want, html)
		}
	}

	sb.Reset()
	if err := bossRankTable(vm, rows, "HPS").Render(context.Background(), &sb); err != nil {
		t.Fatal(err)
	}
	if html = sb.String(); !strings.Contains(html, `data-sort="7654321"`) {
		t.Fatalf("HPS rank table must sort by HPS:\n%s", html)
	}
}

// ClassMode realms rank by class, so the icon column's sort key is the class
// label alone - no spec suffix to sort by.
func TestBossRankTableClassModeSortKey(t *testing.T) {
	vm := ViewModel{Realm: "Lordaeron", ClassMode: true}
	rows := []Rank{{Rank: 1, Name: "Foo", Class: 8, ClassLabel: "Mage", DPS: 100}}

	var sb strings.Builder
	if err := bossRankTable(vm, rows, "DPS").Render(context.Background(), &sb); err != nil {
		t.Fatal(err)
	}
	if html := sb.String(); !strings.Contains(html, `data-sort="Mage"`) {
		t.Fatalf(`missing data-sort="Mage" in class-mode HTML:`+"\n%s", html)
	}
}
