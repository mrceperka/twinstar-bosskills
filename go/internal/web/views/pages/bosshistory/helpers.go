package bosshistory

import (
	"strconv"

	"github.com/a-h/templ"
	"github.com/mrceperka/twinstar-bosskills/go/internal/links"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/views/viewhelpers"
)

func bossOverviewHref(realmName string, id uint32) templ.SafeURL {
	return templ.SafeURL(links.Boss(realmName, id))
}

func tabHref(realmName string, id uint32, mode, offset int) templ.SafeURL {
	return templ.SafeURL(links.BossHistory(realmName, id) +
		"?mode=" + strconv.Itoa(mode) + "&offset=" + strconv.Itoa(offset))
}

var (
	intStr         = viewhelpers.Itoa
	formatInt64    = func(v int64) string { return strconv.FormatInt(v, 10) }
	tabActiveClass = viewhelpers.TabActiveClass
)
