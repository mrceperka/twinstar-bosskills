package boss

import (
	"context"
	"strings"
	"testing"
)

// The client-side sorter in static/bk-ui.js reads td[data-sort] for any cell
// whose text is locale-formatted ("1,234,567" / "5:05" / "3 days ago"). If a
// numeric column loses its data-sort, that column silently sorts as text.
func TestTopTableRendersSortKeys(t *testing.T) {
	vm := ViewModel{Realm: "Helios"}
	rows := []Ranking{{
		Rank: 1, Name: "Foo", RemoteID: "18_1",
		Spec: 62, SpecLabel: "Arcane", Class: 8, ClassLabel: "Mage",
		DPS: 1234567, HPS: 7654321,
		DmgDone: 98765432, HealDone: 23456789,
		LengthSec: 305, KilledAt: "3 days ago", KilledAtUnix: 1700000000,
		ItemLevel: 540,
	}}

	var sb strings.Builder
	if err := topTable(vm, rows, "DPS").Render(context.Background(), &sb); err != nil {
		t.Fatal(err)
	}
	html := sb.String()
	for _, want := range []string{
		"data-sortable",           // sorter attaches to this table
		`data-sort="Mage Arcane"`, // class/spec column renders icons only
		`data-sort="1234567"`,     // DPS, rendered as 1,234,567
		`data-sort="98765432"`,    // Dmg Done
		`data-sort="305"`,         // fight length, rendered as "5 minutes 5 seconds"
		`data-sort="1700000000"`,  // kill time, rendered as "3 days ago"
		"data-no-sort",            // Details column
		"1,234,567",               // the formatted cell text survives
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in DPS table HTML:\n%s", want, html)
		}
	}

	sb.Reset()
	if err := topTable(vm, rows, "HPS").Render(context.Background(), &sb); err != nil {
		t.Fatal(err)
	}
	html = sb.String()
	for _, want := range []string{`data-sort="7654321"`, `data-sort="23456789"`, "7,654,321"} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in HPS table HTML:\n%s", want, html)
		}
	}
	if strings.Contains(html, `data-sort="1234567"`) {
		t.Fatalf("HPS table must not carry DPS sort keys:\n%s", html)
	}
}
