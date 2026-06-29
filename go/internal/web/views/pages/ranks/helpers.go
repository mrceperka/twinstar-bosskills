package ranks

import (
	"strconv"

	"github.com/a-h/templ"
	"github.com/mrceperka/twinstar-bosskills/go/internal/links"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/views/viewhelpers"
)

func tabHref(realmName string, mode, offset int) templ.SafeURL {
	return templ.SafeURL(links.Ranks(realmName) + "?difficulty=" + strconv.Itoa(mode) + "&raidlock=" + strconv.Itoa(offset))
}

var (
	itoa             = viewhelpers.Itoa
	bossWithModeHref = viewhelpers.BossWithDifficultyHref
	charHref         = viewhelpers.CharacterHref
	killHref         = viewhelpers.BossKillHref
	fmtLength        = viewhelpers.ShortDuration
	fmtIlvl          = viewhelpers.Ilvl
	tabActiveClass   = viewhelpers.TabActiveClass
)
