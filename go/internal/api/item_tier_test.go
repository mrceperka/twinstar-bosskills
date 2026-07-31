package api

import "testing"

// Real tooltip bodies captured from the Twinstar item/tooltip endpoint.
const (
	tipWarforged = `<div><b style="color: #a335ee;"> Blood Rage Bracers </b><br /><span style="color: #1eff00;">Warforged</span><br/><span style="color: #ffd100;">Item Level 559</span><br /><span>Binds when picked up</span><span>2907 Armor</span> <br /><span>+1226 Strength </span><br /><span style="color: #00FF00;">+717 Parry </span><br /><span>Required Level 90</span> </div>`

	tipNormal = `<div><b style="color: #a335ee;"> Bottle of Infinite Stars </b><br/><span style="color: #ffd100;">Item Level 489</span><br /><span>Binds when picked up</span><span style="color: #00FF00;">Equip: Your attacks have a chance to grant you 963 Agility for 20 sec.</span><br /><span>Required Level 90</span> </div>`

	tipHeroic = `<div><b style="color: #a335ee;"> Seal of Sullen Fury </b><br /><span style="color: #1eff00;">Heroic</span><br/><span style="color: #ffd100;">Item Level 553</span><br /><span>Binds when picked up</span> </div>`
)

func TestItemTier(t *testing.T) {
	cases := []struct {
		name string
		html string
		want string
	}{
		{"warforged", tipWarforged, "Warforged"},
		{"heroic", tipHeroic, "Heroic"},
		{"normal", tipNormal, ""},
		{"empty", "", ""},
	}
	for _, c := range cases {
		if got := ItemTier(c.html); got != c.want {
			t.Errorf("%s: ItemTier = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestItemLevelFromTooltip(t *testing.T) {
	cases := []struct {
		name string
		html string
		want int
	}{
		{"warforged", tipWarforged, 559},
		{"normal", tipNormal, 489},
		{"heroic", tipHeroic, 553},
		{"empty", "", 0},
	}
	for _, c := range cases {
		if got := ItemLevelFromTooltip(c.html); got != c.want {
			t.Errorf("%s: ItemLevelFromTooltip = %d, want %d", c.name, got, c.want)
		}
	}
}
