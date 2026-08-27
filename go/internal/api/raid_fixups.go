package api

import "twinstar-bosskills/internal/realm"

// Boss filter/rename rules ported from packages/api/src/raid.ts.
// The upstream API exposes some redundant or oddly-named bosses;
// the existing SvelteKit app filters them out and applies display names.

// Bosses removed from the raid roster (duplicates / sub-mobs we don't want
// shown as standalone entries).
var bossesToDrop = map[int]bool{
	// MoP
	71858: true, // Wavebinder Kardris (duplicate; Kor'kron Dark Shaman is kept)
	71475: true, // Rook Stonetoe   - kept Sun Tenderheart from Fallen Protectors group
	71479: true, // He Softfoot
	68904: true, // Lu'lin           - kept Suen from Twin Consorts
	69131: true, // Sul the Sandcrawler - Council of Elders group
	69134: true, // Frost King Malakk
	69078: true, // Kazra'jin
	60586: true, // Elder Asani      - Protectors of the Endless

	// Cata
	57773: true, // DS - Kohcrom (duplicate of Yor'sahj)
	54199: true, // FL - Rhyolith duplicate
	45993: true, // BoT - Theralion
	42166: true, // BWD - Arcanotron
	42180: true, // BWD - Toxitron
	41270: true, // BWD - Onyxia
	45870: true, // TotFW - Anshal
	45872: true, // TotFW - Rohash
}

// Display-name overrides for bosses kept in the roster.
var bossRenames = map[int]string{
	// MoP - SoO
	71858: "Kor'kron Dark Shaman",
	71859: "Kor'kron Dark Shaman",
	72276: "Norushen",
	71480: "The Fallen Protectors",
	// (71475 / 71479 also map but are dropped above)

	// MoP - ToT
	68905: "Twin Consorts",
	69132: "Council of Elders",
	// MoP - ToES
	60583: "Protectors of the Endless",
	// MoP - MSV
	59915: "Stone Guard",
	60701: "Spirit Kings",
	60399: "Will of the Emperor",
	// Cata - DS
	53879: "Spine of Deathwing",
	56173: "Madness of Deathwing",
	// Cata - BoT
	45992: "Theralion and Valiona",
	43735: "Ascendant Council",
	// Cata - BWD
	42179: "Omnotron Defense System",
	// Cata - TotFW
	45871: "The Conclave of Wind",
}

func sanitizeBosses(in []Boss) []Boss {
	out := in[:0]
	for _, b := range in {
		if bossesToDrop[b.Entry] {
			continue
		}
		out = append(out, b)
	}
	return out
}

func renameBosses(in []Boss) []Boss {
	for i, b := range in {
		if newName, ok := bossRenames[b.Entry]; ok {
			in[i].Name = newName
		}
	}
	return in
}

// vanillaRaids is the raid whitelist shared by every vanilla realm. Both
// Kronos and KronosV run all of these; Blackrock Spire is a busy raid on
// both (it was the 4th most-killed map on Kronos while it was excluded).
var vanillaRaids = map[string]bool{
	"Blackrock Spire":    true,
	"Molten Core":        true,
	"Onyxia's Lair":      true,
	"Blackwing Lair":     true,
	"Zul'Gurub":          true,
	"Ahn'Qiraj Temple":   true,
	"Ruins of Ahn'Qiraj": true,
	"Naxxramas":          true,
}

// filterVanillaRaids drops the world zones upstream returns for
// expansion=0 (Kalimdor, Eastern Kingdoms, Deeprun Tram, Stratholme,
// Dire Maul, Alterac Valley) so only real raids survive. Non-vanilla
// realms pass through untouched.
func filterVanillaRaids(realmName string, in []Raid) []Raid {
	switch realmName {
	case realm.Kronos, realm.KronosV:
	default:
		return in
	}
	out := in[:0]
	for _, r := range in {
		if vanillaRaids[r.Map] {
			out = append(out, r)
		}
	}
	return out
}
