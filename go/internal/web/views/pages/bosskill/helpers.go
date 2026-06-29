package bosskill

import (
	"strconv"

	"github.com/a-h/templ"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/views/viewhelpers"
)

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

var (
	bossHref         = viewhelpers.BossHref
	bossWithModeHref = viewhelpers.BossWithDifficultyHref
	characterHref    = viewhelpers.CharacterHref
	itemIconHref     = viewhelpers.ItemIconHref
	itemTooltipURL   = viewhelpers.ItemTooltipURL
	raidIconHref     = viewhelpers.RaidIconHref
	absInt           = viewhelpers.AbsInt
	uitoa            = viewhelpers.Uitoa
	formatFloat      = viewhelpers.FormatFloat
	longDuration     = viewhelpers.LongDuration
	classIconHref    = viewhelpers.ClassIconHref
	specIconHref     = viewhelpers.SpecIconHref
)
