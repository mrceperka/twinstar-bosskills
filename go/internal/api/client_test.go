package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestGetCharacterActivityFeedEncodesQuery(t *testing.T) {
	var gotPath string
	var gotQuery url.Values

	cli := NewClient("https://example.test")
	cli.HTTP = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		gotQuery = req.URL.Query()
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"data":[],"total":0}`)),
			Request:    req,
		}, nil
	})}

	if _, err := cli.GetCharacterActivityFeed(context.Background(), "Helios", "Narmolock", 1, 25); err != nil {
		t.Fatal(err)
	}

	if gotPath != "/character/activity-feed" {
		t.Fatalf("path = %q, want /character/activity-feed", gotPath)
	}
	for key, want := range map[string]string{
		"realm":    "Helios",
		"name":     "Narmolock",
		"page":     "1",
		"pageSize": "25",
	} {
		if got := gotQuery.Get(key); got != want {
			t.Fatalf("query %s = %q, want %q; full query = %#v", key, got, want, gotQuery)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestCharacterActivityFeedDecodeEventTypes(t *testing.T) {
	raw := []byte(`{
		"data": [
			{
				"id": 5,
				"type": 1,
				"data": 8453,
				"data2": 0,
				"date": 1783021869,
				"difficulty": 0,
				"item_guid": 0,
				"item_quality": 0,
				"icon": "/img/character-feed/feed_icon_achievement.png",
				"achievement": {"name": "Rescue Raiders", "points": 10, "realmFirst": false}
			},
			{
				"id": 1,
				"type": 2,
				"data": 105042,
				"data2": 0,
				"date": 1783028207,
				"difficulty": 0,
				"item_guid": 805916928,
				"item_quality": 4,
				"icon": "/img/character-feed/feed_icon_loot.png",
				"loot": {"name": "Kardris' Toxic Totem"}
			},
			{
				"id": 0,
				"type": 3,
				"data": 71515,
				"data2": 2232691,
				"date": 1783028979,
				"difficulty": 7,
				"item_guid": 0,
				"item_quality": 0,
				"icon": "/img/character-feed/feed_icon_bosskill.png",
				"boss": {"name": "General Nazgrim"}
			}
		],
		"total": 818
	}`)

	var got PaginatedCharacterActivityFeed
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Total != 818 || len(got.Data) != 3 {
		t.Fatalf("decoded total=%d len=%d, want total=818 len=3", got.Total, len(got.Data))
	}
	if got.Data[0].Achievement == nil || got.Data[0].Achievement.Name != "Rescue Raiders" || got.Data[0].Achievement.Points != 10 {
		t.Fatalf("achievement event decoded incorrectly: %#v", got.Data[0])
	}
	if got.Data[1].Loot == nil || got.Data[1].Loot.Name != "Kardris' Toxic Totem" || got.Data[1].ItemQuality != 4 {
		t.Fatalf("loot event decoded incorrectly: %#v", got.Data[1])
	}
	if got.Data[2].Boss == nil || got.Data[2].Boss.Name != "General Nazgrim" || got.Data[2].Data2 != 2232691 {
		t.Fatalf("boss event decoded incorrectly: %#v", got.Data[2])
	}
}

func TestGetCharacterStatsEncodesQuery(t *testing.T) {
	var gotPath string
	var gotQuery url.Values

	cli := NewClient("https://example.test")
	cli.HTTP = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		gotQuery = req.URL.Query()
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"attributes":{"stamina":{"effective":10,"health":100},"intellect":{"effective":20,"mana":200}}}`)),
			Request:    req,
		}, nil
	})}

	got, err := cli.GetCharacterStats(context.Background(), "Helios", "Hottik")
	if err != nil {
		t.Fatal(err)
	}

	if gotPath != "/character/stats" {
		t.Fatalf("path = %q, want /character/stats", gotPath)
	}
	for key, want := range map[string]string{
		"realm": "Helios",
		"name":  "Hottik",
	} {
		if got := gotQuery.Get(key); got != want {
			t.Fatalf("query %s = %q, want %q; full query = %#v", key, got, want, gotQuery)
		}
	}
	if len(got.Raw) == 0 {
		t.Fatal("Raw is empty")
	}
	if got.Stats.Attributes.Stamina.Health != 100 {
		t.Fatalf("health = %d, want 100", got.Stats.Attributes.Stamina.Health)
	}
}

func TestCharacterStatsDecodeBasicFields(t *testing.T) {
	raw := []byte(`{
		"attributes": {
			"stamina": {"effective": 34812, "base": 114, "health": 487108, "posBuf": 34698, "negBuf": 0},
			"strength": {"effective": 175, "base": 95, "attack": 165, "posBuf": 80, "negBuf": 0},
			"agility": {"armor": 54668, "attack": 54648, "base": 111, "critHitPercent": 3442771972.9999995, "effective": 27334, "posBuf": 27223, "negBuf": 0},
			"intellect": {"base": 168, "critHitPercent": 62834767.99999999, "effective": 248, "mana": 3440, "spellPower": 238, "posBuf": 80, "negBuf": 0},
			"spirit": {"base": 192, "effective": 272, "healthRegen": 0, "manaRegen": 4835.120204242622, "posBuf": 80, "negBuf": 0}
		},
		"melee": {
			"attackPower": {"base": 54993, "dps": 3928.0714285714284, "posBuf": 0, "negBuf": 0, "effective": 54993},
			"haste": {"base": 12397, "percent": 29.16941176470588, "rating": 29.16941176470588},
			"hitChance": {"base": 2563, "percent": 7.538235294117647, "rating": 7.538235294117647},
			"critChance": {"base": 11650, "percent": 19.416666666666668, "rating": 19.416666666666668},
			"mastery": {"base": 2848, "percent": 4.746666666666667, "rating": 4.746666666666667}
		},
		"spell": {
			"spellPower": 238,
			"hitChance": {"base": 2563, "percent": 7.538235294117647, "rating": 7.538235294117647}
		},
		"defense": {
			"armor": {"base": 21954, "percent": 32.18518871451295, "rating": 0},
			"dodge": {"base": 0, "percent": 22.460695266723633, "rating": 0},
			"parry": {"base": 0, "percent": 8.015125274658203, "rating": 0}
		}
	}`)

	var got CharacterStats
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}

	if got.Attributes.Stamina.Health != 487108 {
		t.Fatalf("health = %d, want 487108", got.Attributes.Stamina.Health)
	}
	if got.Attributes.Intellect.Mana != 3440 {
		t.Fatalf("mana = %d, want 3440", got.Attributes.Intellect.Mana)
	}
	if got.Melee.AttackPower.Effective != 54993 {
		t.Fatalf("attack power = %f, want 54993", got.Melee.AttackPower.Effective)
	}
	if got.Spell.SpellPower != 238 {
		t.Fatalf("spell power = %d, want 238", got.Spell.SpellPower)
	}
	if got.Defense.Dodge.Percent != 22.460695266723633 {
		t.Fatalf("dodge = %f, want 22.460695", got.Defense.Dodge.Percent)
	}
}

func TestCharacterStatsDecodeAllowsNegativeAttributeAttack(t *testing.T) {
	raw := []byte(`{
		"attributes": {
			"agility": {"armor": 332, "attack": -1, "base": 79, "effective": 166}
		}
	}`)

	var got CharacterStats
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Attributes.Agility.Attack != -1 {
		t.Fatalf("agility attack = %d, want -1", got.Attributes.Agility.Attack)
	}
}

func TestCharacterStatsDecodeSlimSentinelAndFractionalValues(t *testing.T) {
	raw := []byte(`{
		"attributes": {
			"intellect": {"base": 48, "critHitPercent": -1, "effective": 128, "mana": -1, "spellPower": -1, "posBuf": 80, "negBuf": 0},
			"spirit": {"base": 77, "effective": 157, "healthRegen": 0, "manaRegen": -1, "posBuf": 80, "negBuf": 0}
		},
		"melee": {
			"attackPower": {"base": 54850, "dps": 5484.999906591007, "posBuf": 0, "negBuf": 0, "effective": 76789.9986922741}
		},
		"spell": {
			"spellPower": 118,
			"manaRegen": null,
			"combatRegen": null
		}
	}`)

	var got CharacterStats
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Attributes.Intellect.Mana != -1 {
		t.Fatalf("intellect mana = %d, want -1", got.Attributes.Intellect.Mana)
	}
	if got.Attributes.Intellect.SpellPower != -1 {
		t.Fatalf("intellect spell power = %d, want -1", got.Attributes.Intellect.SpellPower)
	}
	if got.Melee.AttackPower.Effective != 76789.9986922741 {
		t.Fatalf("attack power effective = %f, want 76789.998692", got.Melee.AttackPower.Effective)
	}
}
