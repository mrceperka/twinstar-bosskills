package bosskill

import (
	"net/url"
	"strconv"

	"github.com/a-h/templ"
	"github.com/mrceperka/twinstar-bosskills/go/internal/links"
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

// statsSortURL toggles sort direction on repeat clicks and switches column
// otherwise. Server-side sorting keeps things simple: the whole page URL
// carries ?sort=col&dir=asc|desc and htmx swaps the stats-table fragment.
func statsSortURL(vm ViewModel, col string) string {
	dir := "desc"
	if vm.SortBy == col && vm.SortDir == "desc" {
		dir = "asc"
	}
	q := url.Values{}
	if !(col == defaultStatsSort && dir == defaultStatsDir) {
		q.Set("sort", col)
		q.Set("dir", dir)
	}
	base := links.BossKill(vm.Realm, vm.Kill.RemoteID)
	if enc := q.Encode(); enc != "" {
		return base + "?" + enc
	}
	return base
}
