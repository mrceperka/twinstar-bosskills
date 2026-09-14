package raids

import (
	"context"
	"strings"
	"testing"
)

func TestPageRendersRaidTotalKills(t *testing.T) {
	vm := ViewModel{
		Realm: "Helios",
		Lock: LockLabel{
			StartLabel: "Jul 15 06:00 UTC",
			EndLabel:   "Jul 22 06:00 UTC",
		},
		Raids: []Raid{{
			Name:       "Siege of Orgrimmar",
			TotalKills: 42,
			Bosses: []BossRow{{
				Name:     "Immerseus",
				RemoteID: 71543,
				Total:    42,
			}},
		}},
	}

	var sb strings.Builder
	if err := Page(vm).Render(context.Background(), &sb); err != nil {
		t.Fatal(err)
	}
	html := sb.String()
	for _, want := range []string{
		"Siege of Orgrimmar",
		`total <span class="font-mono text-bk-accent">42</span> kills`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in HTML:\n%s", want, html)
		}
	}
}
