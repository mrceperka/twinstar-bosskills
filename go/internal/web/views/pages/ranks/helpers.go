package ranks

import (
	"strconv"

	"github.com/a-h/templ"
	"github.com/mrceperka/twinstar-bosskills/go/internal/links"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/views/viewhelpers"
)

func tabHref(realmName string, mode int) templ.SafeURL {
	return templ.SafeURL(links.Ranks(realmName) + "?mode=" + strconv.Itoa(mode))
}

func bossHref(realmName string, id uint32) templ.SafeURL {
	return templ.SafeURL(links.Boss(realmName, id))
}

func bossWithModeHref(realmName string, id uint32, mode int) templ.SafeURL {
	return templ.SafeURL(links.Boss(realmName, id) + "?mode=" + strconv.Itoa(mode))
}

func charHref(realmName, name string) templ.SafeURL {
	return templ.SafeURL(links.Character(realmName, name))
}

func killHref(realmName, id string) templ.SafeURL {
	return templ.SafeURL(links.BossKill(realmName, id))
}

func fmtLength(sec int) string {
	if sec <= 0 {
		return ""
	}
	m := sec / 60
	s := sec % 60
	if m == 0 {
		return strconv.Itoa(s) + "s"
	}
	return strconv.Itoa(m) + "m" + strconv.Itoa(s) + "s"
}

func fmtIlvl(v float32) string {
	if v <= 0 {
		return ""
	}
	return strconv.Itoa(int(v))
}

var (
	itoa           = viewhelpers.Itoa
	tabActiveClass = viewhelpers.TabActiveClass
)
