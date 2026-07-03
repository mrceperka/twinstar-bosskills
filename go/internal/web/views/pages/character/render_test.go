package character

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mrceperka/twinstar-bosskills/go/internal/api"
	"github.com/mrceperka/twinstar-bosskills/go/internal/wow"
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

func TestPageRenderSpecSummaryHighlightsMostPlayed(t *testing.T) {
	vm := ViewModel{
		Realm: "Helios",
		Char: CharacterInfo{
			Name:       "Plaguis",
			Class:      9,
			ClassLabel: "Warlock",
			FirstSeen:  "2026-01-01",
			LastSeen:   "2026-07-03 12:00",
			KillCount:  15,
		},
		SpecSummary: []SpecSummary{
			{Spec: 265, Class: 9, SpecLabel: "Affliction", KillCount: 10, IsMostPlayed: true},
			{Spec: 266, Class: 9, SpecLabel: "Demonology", KillCount: 5},
		},
	}

	var sb strings.Builder
	if err := Page(vm).Render(context.Background(), &sb); err != nil {
		t.Fatal(err)
	}
	html := sb.String()
	for _, want := range []string{
		"Specs",
		"Affliction",
		"10",
		"Most played",
		"border-bk-accent",
		"Demonology",
		"5",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in HTML:\n%s", want, html)
		}
	}
}

func TestBuildSpecSummaryRowsSortsAndHighlightsMostPlayed(t *testing.T) {
	got := buildSpecSummaryRows(5, map[int]uint64{105: 3, 103: 12, 104: 12})

	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0].Spec != 103 || got[0].KillCount != 12 || !got[0].IsMostPlayed {
		t.Fatalf("first row = %#v, want spec 103 highlighted with 12 kills", got[0])
	}
	if got[1].Spec != 104 || got[1].IsMostPlayed {
		t.Fatalf("second row = %#v, want spec 104 not highlighted", got[1])
	}
	if got[2].Spec != 105 || got[2].KillCount != 3 {
		t.Fatalf("third row = %#v, want spec 105 with 3 kills", got[2])
	}
}

func TestActivityRefreshMetaIsStale(t *testing.T) {
	now := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)

	if !activityIsStale(activityRefreshMeta{}, now) {
		t.Fatal("missing metadata should be stale")
	}
	if activityIsStale(activityRefreshMeta{Found: true, LastCheckedAt: now.Add(-30 * time.Minute)}, now) {
		t.Fatal("metadata checked 30 minutes ago should be fresh")
	}
	if !activityIsStale(activityRefreshMeta{Found: true, LastCheckedAt: now.Add(-61 * time.Minute)}, now) {
		t.Fatal("metadata checked over 1 hour ago should be stale")
	}
}

func TestBuildActivityRefreshMarker(t *testing.T) {
	checkedAt := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)

	row := buildActivityRefreshMarker("Helios", "Narmolock", checkedAt, "upstream failed")

	if row.EventType != activityTypeRefreshMarker {
		t.Fatalf("EventType = %d, want %d", row.EventType, activityTypeRefreshMarker)
	}
	if row.EventTime != checkedAt {
		t.Fatalf("EventTime = %s, want %s", row.EventTime, checkedAt)
	}
	if row.RefreshError != "upstream failed" {
		t.Fatalf("RefreshError = %q, want upstream failed", row.RefreshError)
	}
}

func TestStatsInsertQueryHasPlaceholders(t *testing.T) {
	if !strings.Contains(strings.ToUpper(insertStatsSQL), "VALUES") {
		t.Fatalf("stats insert query must include VALUES for ClickHouse Exec args:\n%s", insertStatsSQL)
	}
	if got, want := strings.Count(insertStatsSQL, "?"), 47; got != want {
		t.Fatalf("stats insert placeholders = %d, want %d:\n%s", got, want, insertStatsSQL)
	}
}

func TestBuildActivityRowDerivesBossKillRemoteID(t *testing.T) {
	rows := buildActivityStorageRows("Helios", "Narmolock", 1, []api.CharacterActivityEvent{{
		ID:         0,
		Type:       activityTypeBossKill,
		Data:       71515,
		Data2:      2232691,
		Date:       1783028979,
		Difficulty: 7,
		Icon:       "/img/character-feed/feed_icon_bosskill.png",
		Boss:       &api.CharacterActivityBoss{Name: "General Nazgrim"},
	}})

	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	if rows[0].BossKillRemoteID != "18_2232691" {
		t.Fatalf("BossKillRemoteID = %q, want 18_2232691", rows[0].BossKillRemoteID)
	}
	if rows[0].Title != "General Nazgrim" {
		t.Fatalf("Title = %q, want General Nazgrim", rows[0].Title)
	}
	if rows[0].SourcePage != 1 {
		t.Fatalf("SourcePage = %d, want 1", rows[0].SourcePage)
	}
}

func TestActivityFragmentRenderNoticeAndRows(t *testing.T) {
	vm := ActivityViewModel{
		Realm:  "Helios",
		Notice: "Activity feed is cached and may be up to 1 hour stale.",
		Rows: []ActivityRow{{
			EventKind:        "Boss kill",
			EventTime:        "2026-07-03 12:00",
			Title:            "General Nazgrim",
			DifficultyLabel:  "LFR",
			BossKillRemoteID: "18_2232691",
		}},
	}

	var sb strings.Builder
	if err := ActivityFragment(vm).Render(context.Background(), &sb); err != nil {
		t.Fatal(err)
	}
	html := sb.String()
	for _, want := range []string{
		"Activity feed is cached and may be up to 1 hour stale.",
		"General Nazgrim",
		"/Helios/boss-kills/18_2232691",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in HTML:\n%s", want, html)
		}
	}
}

func TestActivityFragmentRenderLoadMoreAndLootTooltip(t *testing.T) {
	vm := ActivityViewModel{
		Realm:    "Helios",
		Notice:   "Activity feed is cached and may be up to 1 hour stale.",
		HasMore:  true,
		NextHref: "/Helios/character/Narmolock/activity?page=1",
		Rows: []ActivityRow{{
			EventKind:        "Loot",
			EventTime:        "2026-07-03 12:00",
			Title:            "Kardris' Toxic Totem",
			ItemID:           105042,
			ItemQuality:      4,
			ItemQualityColor: "#a335ee",
		}},
	}

	var sb strings.Builder
	if err := ActivityFragment(vm).Render(context.Background(), &sb); err != nil {
		t.Fatal(err)
	}
	html := sb.String()
	for _, want := range []string{
		`class="bk-tip-row`,
		`data-tooltip-url="/img/tooltip?id=105042&amp;realm=Helios"`,
		`/img/icon?type=item&amp;id=105042`,
		`style="color: #a335ee;"`,
		`hx-get="/Helios/character/Narmolock/activity?page=1"`,
		`Load more`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in HTML:\n%s", want, html)
		}
	}
}

func TestBuildStatsStorageRowExtractsBasicFields(t *testing.T) {
	now := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	payload := api.CharacterStatsPayload{
		Raw: []byte(`{"ok":true}`),
		Stats: api.CharacterStats{
			Attributes: api.CharacterStatsAttributes{
				Stamina:   api.CharacterAttributeStat{Effective: 34812, Health: 487108, PosBuf: 34698},
				Strength:  api.CharacterAttributeStat{Effective: 175, PosBuf: 80},
				Agility:   api.CharacterAttributeStat{Effective: 27334, PosBuf: 27223},
				Intellect: api.CharacterAttributeStat{Effective: 248, Mana: 3440, PosBuf: 80},
				Spirit:    api.CharacterAttributeStat{Effective: 272, PosBuf: 80, NegBuf: 5},
			},
			Melee: api.CharacterStatsMelee{
				AttackPower: api.CharacterPowerStat{Effective: 54993},
				EnergyRegen: 1261.405029296875,
				HitChance:   api.CharacterRatingStat{Base: 2563, Percent: 7.538235294117647, Rating: 7.538235294117647},
				CritChance:  api.CharacterRatingStat{Base: 1900, Percent: 19.416666666666668, Rating: 12.416666666666668},
				Haste:       api.CharacterRatingStat{Base: 4961, Percent: 29.16941176470588, Rating: 19.16941176470588},
				Mastery:     api.CharacterRatingStat{Base: 712, Percent: 4.746666666666667, Rating: 4.746666666666667},
			},
			Spell: api.CharacterStatsSpell{SpellPower: 238},
			Defense: api.CharacterStatsDefense{
				Armor: api.CharacterRatingStat{Base: 21954, Percent: 32.18525314331055},
				Dodge: api.CharacterRatingStat{Percent: 22.460695266723633, Rating: 1743},
				Parry: api.CharacterRatingStat{Percent: 8.015125274658203, Rating: 475},
			},
		},
	}

	row := buildStatsStorageRow("Helios", "Hottik", payload, now, "")

	if row.Health != 487108 || row.Mana != 3440 {
		t.Fatalf("health/mana = %d/%d, want 487108/3440", row.Health, row.Mana)
	}
	if row.AttackPower != 54993 || row.SpellPower != 238 {
		t.Fatalf("attack/spell power = %d/%d, want 54993/238", row.AttackPower, row.SpellPower)
	}
	if row.StaminaPosBuf != 34698 || row.SpiritNegBuf != 5 {
		t.Fatalf("attribute deltas = stamina +%d spirit -%d, want +34698/-5", row.StaminaPosBuf, row.SpiritNegBuf)
	}
	if row.EnergyRegen != 1261.405029296875 {
		t.Fatalf("energy regen = %f, want 1261.405029", row.EnergyRegen)
	}
	if row.HitBase != 2563 || row.HitPercent != 7.538235294117647 || row.HitRating != 7.538235294117647 {
		t.Fatalf("hit = %d/%f/%f, want 2563/7.538235/7.538235", row.HitBase, row.HitPercent, row.HitRating)
	}
	if row.ArmorBase != 21954 || row.ArmorPercent != 32.18525314331055 || row.ArmorRating != 0 {
		t.Fatalf("armor = %d/%f/%f, want 21954/32.185253/0", row.ArmorBase, row.ArmorPercent, row.ArmorRating)
	}
	if row.DodgeBase != 0 || row.DodgePercent != 22.460695266723633 || row.DodgeRating != 1743 {
		t.Fatalf("dodge = %d/%f/%f, want 0/22.460695/1743", row.DodgeBase, row.DodgePercent, row.DodgeRating)
	}
	if row.RawJSON != `{"ok":true}` {
		t.Fatalf("RawJSON = %q, want raw payload", row.RawJSON)
	}
}

func TestBuildStatsStorageRowCoercesSlimSentinels(t *testing.T) {
	now := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	payload := api.CharacterStatsPayload{
		Raw: []byte(`{"character":"Slim"}`),
		Stats: api.CharacterStats{
			Attributes: api.CharacterStatsAttributes{
				Stamina:   api.CharacterAttributeStat{Effective: 36563, Health: 511622},
				Intellect: api.CharacterAttributeStat{Effective: 128, Mana: -1, SpellPower: -1},
			},
			Melee: api.CharacterStatsMelee{
				AttackPower: api.CharacterPowerStat{Effective: 76789.9986922741},
			},
			Spell: api.CharacterStatsSpell{SpellPower: 118},
		},
	}

	row := buildStatsStorageRow("Helios", "Slim", payload, now, "")

	if row.Mana != 0 {
		t.Fatalf("Mana = %d, want 0 for upstream -1 sentinel", row.Mana)
	}
	if row.AttackPower != 76790 {
		t.Fatalf("AttackPower = %d, want rounded 76790", row.AttackPower)
	}
	if row.SpellPower != 118 {
		t.Fatalf("SpellPower = %d, want 118", row.SpellPower)
	}
}

func TestBuildStatsViewModelShowsPrimaryClassResource(t *testing.T) {
	cases := []struct {
		name      string
		class     int
		wantLabel string
		wantValue string
		wantClass string
	}{
		{name: "warrior", class: wow.ClassWarrior, wantLabel: "Rage", wantValue: "100", wantClass: "text-orange-300"},
		{name: "hunter", class: wow.ClassHunter, wantLabel: "Focus", wantValue: "100", wantClass: "text-orange-300"},
		{name: "rogue", class: wow.ClassRogue, wantLabel: "Energy", wantValue: "100", wantClass: "text-yellow-300"},
		{name: "monk", class: wow.ClassMonk, wantLabel: "Energy", wantValue: "100", wantClass: "text-yellow-300"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			vm := buildStatsViewModel(context.Background(), statsStorageRow{
				Found:          true,
				CharacterClass: tt.class,
				Health:         1000,
				Mana:           0,
			}, nil)

			var got StatsMetric
			for _, group := range vm.Groups {
				if group.Title != "Core" {
					continue
				}
				for _, metric := range group.Metrics {
					if metric.Label == tt.wantLabel {
						got = metric
					}
				}
			}
			if got.Label == "" {
				t.Fatalf("missing %s resource in core metrics: %#v", tt.wantLabel, vm.Groups[0].Metrics)
			}
			if got.Value != tt.wantValue || got.ValueClass != tt.wantClass {
				t.Fatalf("%s resource = %q/%q, want %q/%q", tt.wantLabel, got.Value, got.ValueClass, tt.wantValue, tt.wantClass)
			}
		})
	}
}

func TestBuildStatsViewModelKeepsRatingComponents(t *testing.T) {
	row := statsStorageRow{
		Found:         true,
		Realm:         "Helios",
		CharacterName: "Hottik",
		HitBase:       2563,
		HitPercent:    7.538235294117647,
		HitRating:     7.538235294117647,
		ArmorBase:     21954,
		ArmorPercent:  32.18525314331055,
		ArmorRating:   0,
		EnergyRegen:   1261.405029296875,
	}

	vm := buildStatsViewModel(context.Background(), row, nil)

	var hitValue, armorValue, energyValue, energyClass string
	for _, group := range vm.Groups {
		for _, metric := range group.Metrics {
			switch metric.Label {
			case "Hit":
				hitValue = metric.Value
			case "Armor":
				armorValue = metric.Value
			case "Energy regen":
				energyValue = metric.Value
				energyClass = metric.ValueClass
			}
		}
	}
	if hitValue != "Base 2,563 | 7.5% | Rating 7.5" {
		t.Fatalf("Hit value = %q", hitValue)
	}
	if armorValue != "Base 21,954 | 32.2% | Rating 0" {
		t.Fatalf("Armor value = %q", armorValue)
	}
	if energyValue != "1261.4" || energyClass != "text-yellow-300" {
		t.Fatalf("Energy regen value/class = %q/%q", energyValue, energyClass)
	}
}

func TestStatsFragmentRenderBasicStats(t *testing.T) {
	vm := StatsViewModel{
		Notice: "Stats are cached and may be up to 1 hour stale.",
		Groups: []StatsGroup{{
			Title: "Core",
			Metrics: []StatsMetric{
				{Label: "Health", Value: "487,108", ValueClass: "text-red-300"},
				{Label: "Mana", Value: "3,440", ValueClass: "text-blue-300"},
				{Label: "Stamina", Value: "34,812", PosDelta: "+34,698", NegDelta: "-5"},
			},
		}},
	}

	var sb strings.Builder
	if err := StatsFragment(vm).Render(context.Background(), &sb); err != nil {
		t.Fatal(err)
	}
	html := sb.String()
	for _, want := range []string{
		"Stats are cached and may be up to 1 hour stale.",
		"Health",
		"487,108",
		"text-red-300",
		"Mana",
		"3,440",
		"text-blue-300",
		"+34,698",
		"text-emerald-300",
		"-5",
		"text-red-300",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in HTML:\n%s", want, html)
		}
	}
}
