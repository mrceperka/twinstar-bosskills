package bosskill

import (
	"net/url"
	"strconv"

	"github.com/a-h/templ"
)

func bossHref(realmName string, id uint32) templ.SafeURL {
	return templ.SafeURL("/" + realmName + "/boss/" + strconv.FormatUint(uint64(id), 10))
}

func bossWithModeHref(realmName string, id uint32, mode int) templ.SafeURL {
	return templ.SafeURL("/" + realmName + "/boss/" + strconv.FormatUint(uint64(id), 10) + "?mode=" + strconv.Itoa(mode))
}

func characterHref(realmName, name string) templ.SafeURL {
	return templ.SafeURL("/" + realmName + "/character/" + url.PathEscape(name))
}

func itemIconHref(itemID uint32) templ.SafeURL {
	return templ.SafeURL("/img/icon?type=item&id=" + strconv.FormatUint(uint64(itemID), 10))
}

func itemTooltipURL(realmName string, itemID uint32) string {
	return "/img/tooltip?id=" + strconv.FormatUint(uint64(itemID), 10) + "&realm=" + url.QueryEscape(realmName)
}

func raidIconHref(name string) templ.SafeURL {
	return templ.SafeURL("/img/icon?type=raid&id=" + url.QueryEscape(name))
}

// wowheadHref points to the Wowhead Cata classic DB which carries MoP items.
// External link — opens in a new tab so users can read the full tooltip.
func wowheadHref(itemID uint32) templ.SafeURL {
	return templ.SafeURL("https://www.wowhead.com/mop-classic/item=" + strconv.FormatUint(uint64(itemID), 10))
}

func playerRowClass(p PlayerRow) string {
	if p.IsDead {
		return "text-red-400/80"
	}
	return ""
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func uitoa(v uint64) string {
	return strconv.FormatUint(v, 10)
}

func formatFloat(v float64, decimals int) string {
	return strconv.FormatFloat(v, 'f', decimals, 64)
}

// longDuration formats seconds as "X minutes Y seconds".
func longDuration(seconds int) string {
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

func classIconHref(class int) templ.SafeURL {
	return templ.SafeURL("/img/icon?type=class&id=" + strconv.Itoa(class))
}

func specIconHref(realmName string, spec int) templ.SafeURL {
	return templ.SafeURL("/img/icon?type=talent&id=" + strconv.Itoa(spec) + "&realm=" + url.QueryEscape(realmName))
}
