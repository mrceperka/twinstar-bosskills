// Package links is the single source of truth for in-app and external URLs.
// Mirrors packages/sveltekit/src/lib/links.ts so paths stay consistent
// between the two apps.
//
// Internal links return string for use in templ (Go converts to templ.SafeURL
// at the call site). External links return string too, but they are
// always wrapped in safe contexts by callers.
package links

import (
	"net/url"
	"strconv"

	"twinstar-bosskills/internal/realm"
)

// twinheadPrefix returns the expansion-specific Twinhead subdomain prefix.
// MoP → "mop-", Cata → "cata-", Vanilla → "vanilla-", otherwise empty.
func twinheadPrefix(realmName string) string {
	switch realm.Expansion(realmName) {
	case realm.ExpansionMoP:
		return "mop-"
	case realm.ExpansionCata:
		return "cata-"
	case realm.ExpansionVanilla:
		return "vanilla-"
	}
	return ""
}

// ---- internal routes -----------------------------------------------------

func Changelog() string { return "/changelog" }

func Realms() string { return "/realms" }

func RealmHome(realmName string) string { return "/" + realmName + "/" }

func BossKills(realmName string) string { return "/" + realmName + "/boss-kills" }

func BossKill(realmName, id string) string {
	return "/" + realmName + "/boss-kills/" + id
}

func Boss(realmName string, id uint32) string {
	return "/" + realmName + "/boss/" + strconv.FormatUint(uint64(id), 10)
}

func Raids(realmName string) string { return "/" + realmName + "/raids" }

func Ranks(realmName string) string { return "/" + realmName + "/ranks" }

func Stats(realmName string) string { return "/" + realmName + "/stats" }

func Characters(realmName string) string { return "/" + realmName + "/characters" }

func Character(realmName, name string) string {
	return "/" + realmName + "/character/" + url.PathEscape(name)
}

func CharacterPerformance(realmName, name string) string {
	return "/" + realmName + "/character/" + url.PathEscape(name) + "/performance"
}

func GuildToken(realmName string) string { return "/" + realmName + "/guild-token" }

func GuildTokenGenerate(realmName string) string {
	return "/" + realmName + "/guild-token/generate"
}

// ---- icon proxy ----------------------------------------------------------

func RaidIcon(name string) string {
	return "/img/icon?type=raid&id=" + url.QueryEscape(name)
}

func ItemIcon(itemID uint32) string {
	return "/img/icon?type=item&id=" + strconv.FormatUint(uint64(itemID), 10)
}

func ClassIcon(class int) string {
	return "/img/icon?type=class&id=" + strconv.Itoa(class)
}

func RaceIcon(raceGender string) string {
	return "/img/icon?type=race&id=" + url.QueryEscape(raceGender)
}

func TalentIcon(realmName string, talentSpec int) string {
	return "/img/icon?type=talent&id=" + strconv.Itoa(talentSpec) + "&realm=" + url.QueryEscape(realmName)
}

// ---- external Twinhead / Armory / Wowhead --------------------------------

// TwinheadBossKill points at the per-kill page on the Twinstar Twinhead DB.
// The upstream `id` is "<realm_id>_<bkid>"; only the bkid suffix is used.
//
// Mirrors links.ts:twinstarBossKill.
func TwinheadBossKill(realmName, remoteID string) string {
	realmID := realm.ID(realmName)
	prefix := strconv.Itoa(realmID) + "_"
	bkid := remoteID
	if len(remoteID) > len(prefix) && remoteID[:len(prefix)] == prefix {
		bkid = remoteID[len(prefix):]
	}
	return "https://" + twinheadPrefix(realmName) + "twinhead.twinstar.cz/?boss-kill=" + bkid
}

func TwinheadNPC(realmName string, npcID uint32) string {
	return "https://" + twinheadPrefix(realmName) + "twinhead.twinstar.cz/?npc=" + strconv.FormatUint(uint64(npcID), 10)
}

func TwinheadGuild(realmName, guild string) string {
	return "https://" + twinheadPrefix(realmName) + "twinhead.twinstar.cz/?guild=" +
		url.QueryEscape(guild) + "&realm=" + url.QueryEscape(realmName)
}

func TwinstarArmory(realmName, characterName string) string {
	return "https://armory.twinstar-wow.com/character?name=" +
		url.QueryEscape(characterName) + "&realm=" + url.QueryEscape(realmName)
}

// TwinheadItem links to the Twinstar Twinhead item page - same DB the
// tooltip API serves from, so it stays in sync with the in-game item data.
// Wowhead is reserved as a fallback for non-Twinstar lookups.
func TwinheadItem(realmName string, id uint32) string {
	return "https://" + twinheadPrefix(realmName) + "twinhead.twinstar.cz/?item=" +
		strconv.FormatUint(uint64(id), 10)
}
