// Package viewhelpers holds small templ helper functions shared by every
// page package. Per-page helpers.go files should keep ONLY page-specific
// values (e.g. a Reset URL that needs the page's filter state).
//
// URL building lives in internal/links — use that for any kind of href.
// Number / duration / locale formatting lives in internal/format.
// WoW domain labels (class, spec, difficulty) live in internal/wow.
package viewhelpers

import (
	"net/url"
	"strconv"

	"github.com/a-h/templ"
	"github.com/mrceperka/twinstar-bosskills/go/internal/links"
)

// Itoa is a small templ-friendly wrapper around strconv.Itoa so the templ
// expression syntax stays clean (`{ Itoa(x) }`).
func Itoa(v int) string { return strconv.Itoa(v) }

// FormatFloat formats a float64 with the requested number of decimal places.
// Locale-neutral — for thousand-separated numbers see format.IntCtx.
func FormatFloat(v float64, decimals int) string {
	return strconv.FormatFloat(v, 'f', decimals, 64)
}

// LongDuration formats seconds as "X minutes Y seconds" — matches the
// narrative phrasing the SvelteKit app uses (boss-page headlines, etc.).
func LongDuration(seconds int) string {
	if seconds <= 0 {
		return "-"
	}
	m := seconds / 60
	s := seconds % 60
	switch {
	case m == 0:
		return strconv.Itoa(s) + " seconds"
	case s == 0:
		return strconv.Itoa(m) + " minutes"
	default:
		return strconv.Itoa(m) + " minutes " + strconv.Itoa(s) + " seconds"
	}
}

// AbsInt returns |v|.
func AbsInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// Uitoa is the uint64 counterpart of Itoa.
func Uitoa(v uint64) string { return strconv.FormatUint(v, 10) }

// Int64 formats an int64 without grouping. Use format.Int64Ctx for
// locale-aware table cells.
func Int64(v int64) string { return strconv.FormatInt(v, 10) }

// EmptyItoa returns an empty string for zero/negative values. Useful for
// optional numeric form fields where "0" should mean unset.
func EmptyItoa(v int) string {
	if v <= 0 {
		return ""
	}
	return strconv.Itoa(v)
}

func RankLabel(rank int) string { return "#" + strconv.Itoa(rank) }

func Ilvl(v float32) string {
	if v <= 0 {
		return ""
	}
	return strconv.Itoa(int(v))
}

func ShortDuration(seconds int) string {
	if seconds <= 0 {
		return ""
	}
	m := seconds / 60
	s := seconds % 60
	if m == 0 {
		return strconv.Itoa(s) + "s"
	}
	return strconv.Itoa(m) + "m" + strconv.Itoa(s) + "s"
}

func BossHref(realmName string, id uint32) templ.SafeURL {
	return templ.SafeURL(links.Boss(realmName, id))
}

func BossWithDifficultyHref(realmName string, id uint32, difficulty int) templ.SafeURL {
	return templ.SafeURL(links.Boss(realmName, id) + "?difficulty=" + strconv.Itoa(difficulty))
}

func BossKillHref(realmName, id string) templ.SafeURL {
	return templ.SafeURL(links.BossKill(realmName, id))
}

func CharacterHref(realmName, name string) templ.SafeURL {
	return templ.SafeURL(links.Character(realmName, name))
}

func CharacterPerformanceHref(realmName, name string) templ.SafeURL {
	return templ.SafeURL(links.CharacterPerformance(realmName, name))
}

func RealmPathHref(realmName, suffix string) templ.SafeURL {
	return templ.SafeURL("/" + realmName + suffix)
}

func RaidIconHref(name string) templ.SafeURL {
	return templ.SafeURL(links.RaidIcon(name))
}

func ItemIconHref(itemID uint32) templ.SafeURL {
	return templ.SafeURL(links.ItemIcon(itemID))
}

func ClassIconHref(class int) templ.SafeURL {
	return templ.SafeURL(links.ClassIcon(class))
}

func SpecIconHref(realmName string, spec int) templ.SafeURL {
	return templ.SafeURL(links.TalentIcon(realmName, spec))
}

func ItemTooltipURL(realmName string, itemID uint32) string {
	return "/img/tooltip?id=" + strconv.FormatUint(uint64(itemID), 10) + "&realm=" + url.QueryEscape(realmName)
}

// TabActiveClass returns the Tailwind classes that mark a tab as active or
// inactive. Used by boss / boss-history / ranks pages — same look for all.
func TabActiveClass(active bool) string {
	if active {
		return "rounded border border-bk-accent bg-bk-accent/20 px-2 py-1 text-bk-accent font-semibold"
	}
	return "rounded border border-bk-border px-2 py-1 text-bk-muted hover:text-bk-fg"
}

func SelectFilterClass(active bool) string {
	if active {
		return "rounded border border-bk-accent bg-bk-bg p-2"
	}
	return "rounded border border-bk-border bg-bk-bg p-2"
}
