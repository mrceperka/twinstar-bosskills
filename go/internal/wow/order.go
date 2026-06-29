package wow

import "strings"

// RaidPosition returns the chronological release position for a raid name.
// Higher value = more recent raid. Returns 0 for unknown raid names.
// Falls back to a normalized (lowercase, apostrophe-stripped) lookup so that
// minor API variants (e.g. curly vs straight apostrophe) still match.
func RaidPosition(name string) int {
	if pos, ok := raidPositions[name]; ok {
		return pos
	}
	return raidPositionsNorm[normalizeRaidName(name)]
}

func normalizeRaidName(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "’", "") // right single quotation mark '
	s = strings.ReplaceAll(s, "‘", "") // left single quotation mark '
	s = strings.ReplaceAll(s, "'", "") // ASCII apostrophe
	return strings.Join(strings.Fields(s), " ")
}

// raidPositionsNorm is a pre-built normalized version of raidPositions.
var raidPositionsNorm = func() map[string]int {
	m := make(map[string]int, len(raidPositions))
	for k, v := range raidPositions {
		m[normalizeRaidName(k)] = v
	}
	return m
}()

// BossSortKey returns a global sort key for a boss by its remote_id (NPC ID).
// Key encodes both the raid release position and the encounter order within
// the raid: (raidPosition * 100 + bossPositionWithinRaid).
// Returns (100_000 + remoteID) for unknown bosses so they sort stably at the end.
func BossSortKey(remoteID uint32) int {
	if pos, ok := bossPositions[remoteID]; ok {
		return pos
	}
	return 100_000 + int(remoteID)
}

// BossPosition returns the encounter order within the boss's raid.
// Returns 0 for unknown bosses.
func BossPosition(remoteID uint32) int {
	if pos, ok := bossPositions[remoteID]; ok {
		return pos % 100
	}
	return 0
}

// raidPositions maps raid name → chronological release position (higher = newer).
var raidPositions = map[string]int{
	// MoP
	"Mogu'shan Vaults":          1,
	"Heart of Fear":             2,
	"Terrace of Endless Spring": 3,
	"Throne of Thunder":         4,
	"Siege of Orgrimmar":        5,

	// Cataclysm
	"Baradin Hold":             6,
	"Blackwing Descent":        7,
	"The Bastion of Twilight":  8,
	"Throne of the Four Winds": 9,
	"Firelands":                10,
	"Dragon Soul":              11,

	// Vanilla (Kronos)
	"Molten Core":        12,
	"Onyxia's Lair":      13,
	"Blackwing Lair":     14,
	"Zul'Gurub":          15,
	"Ruins of Ahn'Qiraj": 16,
	"Ahn'Qiraj Temple":   17,
	"Naxxramas":          18,
}

// bossPositions maps boss NPC ID → global sort key (raidPosition*100 + encounterIndex).
// Source: MySQL migration (raid_id assignments) + WoW game NPC IDs.
var bossPositions = map[uint32]int{
	// ── Mogu'shan Vaults (MoP T14, raidPos=1) ──────────────────────
	59915: 101, // Stone Guard
	60009: 102, // Feng the Accursed
	60143: 103, // Gara'jal the Spiritbinder
	60701: 104, // Spirit Kings
	60410: 105, // Elegon
	60399: 106, // Will of the Emperor

	// ── Heart of Fear (MoP T14, raidPos=2) ─────────────────────────
	62980: 201, // Imperial Vizier Zor'lok
	62543: 202, // Blade Lord Ta'yak
	62164: 203, // Garalon
	62397: 204, // Wind Lord Mel'jarak
	62511: 205, // Amber-Shaper Un'sok
	62837: 206, // Grand Empress Shek'zeer

	// ── Terrace of Endless Spring (MoP T14, raidPos=3) ─────────────
	60583: 301, // Protectors of the Endless
	62442: 302, // Tsulong
	62983: 303, // Lei Shi
	60999: 304, // Sha of Fear

	// ── Throne of Thunder (MoP T15, raidPos=4) ─────────────────────
	60375: 401, // Jin'rokh the Breaker
	68476: 402, // Horridon
	69132: 403, // Council of Elders
	67977: 404, // Tortos
	70212: 405, // Megaera
	69712: 406, // Ji-Kun
	68036: 407, // Durumu the Forgotten
	69017: 408, // Primordius
	69427: 409, // Dark Animus
	68994: 410, // Iron Qon
	68905: 411, // Twin Consorts
	68397: 412, // Lei Shen
	69473: 413, // Ra-den

	// ── Siege of Orgrimmar (MoP T16, raidPos=5) ────────────────────
	71543: 501, // Immerseus
	71480: 502, // The Fallen Protectors
	72276: 503, // Norushen
	71734: 504, // Sha of Pride
	72249: 505, // Galakras
	71466: 506, // Iron Juggernaut
	71859: 507, // Kor'kron Dark Shaman
	71515: 508, // General Nazgrim
	71454: 509, // Malkorok
	71921: 510, // Spoils of Pandaria
	71529: 511, // Thok the Bloodthirsty
	71504: 512, // Siegecrafter Blackfuse
	72057: 513, // Paragons of the Klaxxi
	71865: 514, // Garrosh Hellscream

	// ── Baradin Hold (Cata, raidPos=6) ─────────────────────────────
	47120: 601, // Argaloth
	52363: 602, // Occu'thar
	55869: 603, // Alizabal

	// ── Blackwing Descent (Cata T11, raidPos=7) ────────────────────
	41570: 701, // Magmaw
	42179: 702, // Omnotron Defense System
	41378: 703, // Maloriak
	43296: 704, // Chimaeron
	41442: 705, // Atramedes
	41376: 706, // Nefarian

	// ── The Bastion of Twilight (Cata T11, raidPos=8) ──────────────
	44600: 801, // Halfus Wyrmbreaker
	45992: 802, // Theralion and Valiona
	43735: 803, // Ascendant Council
	43324: 804, // Cho'gall
	45213: 805, // Sinestra

	// ── Throne of the Four Winds (Cata T11, raidPos=9) ─────────────
	45871: 901, // The Conclave of Wind
	46753: 902, // Al'Akir

	// ── Firelands (Cata T12, raidPos=10) ───────────────────────────
	53494: 1001, // Shannox
	52498: 1002, // Beth'tilac
	52558: 1003, // Lord Rhyolith
	52530: 1004, // Alysrazor
	53691: 1005, // Baleroc
	52409: 1006, // Majordomo Staghelm
	52571: 1007, // Ragnaros

	// ── Dragon Soul (Cata T13, raidPos=11) ─────────────────────────
	55265: 1101, // Morchok
	55308: 1102, // Warlord Zon'ozz
	55312: 1103, // Yor'sahj the Unsleeping
	55689: 1104, // Hagara the Stormbinder
	55294: 1105, // Ultraxion
	56427: 1106, // Warmaster Blackhorn
	53879: 1107, // Spine of Deathwing
	56173: 1108, // Madness of Deathwing
}
