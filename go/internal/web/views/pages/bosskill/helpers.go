package bosskill

import (
	"net/url"
	"strconv"

	"twinstar-bosskills/internal/links"

	"github.com/a-h/templ"
)

// wowheadHref points to the Wowhead Cata classic DB which carries MoP items.
// External link - opens in a new tab so users can read the full tooltip.
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
