package bosskills

import (
	"net/url"
	"strconv"

	"github.com/mrceperka/twinstar-bosskills/go/internal/links"
)

// filterValues encodes the currently applied filters + sort state so
// pagination + sort-header links preserve them.
func filterValues(vm ViewModel) url.Values {
	v := url.Values{}
	for _, b := range vm.Filter.Bosses {
		v.Add("boss", strconv.FormatUint(uint64(b), 10))
	}
	for _, r := range vm.Filter.Raids {
		v.Add("raid", r)
	}
	for _, m := range vm.Filter.Difficulties {
		v.Add("difficulty", strconv.Itoa(m))
	}
	for _, c := range vm.Filter.Classes {
		v.Add("class", strconv.Itoa(c))
	}
	for _, s := range vm.Filter.Specs {
		v.Add("spec", strconv.Itoa(s))
	}
	if vm.Filter.SortBy != "" && vm.Filter.SortBy != defaultBKSort {
		v.Set("sort", vm.Filter.SortBy)
	}
	if vm.Filter.SortDir != "" && vm.Filter.SortDir != defaultBKDir {
		v.Set("dir", vm.Filter.SortDir)
	}
	return v
}

// pagedURL builds a pagination href preserving the current filters. The
// links package owns the base URL; we just append the encoded query.
func pagedURL(vm ViewModel, page int) string {
	v := filterValues(vm)
	v.Set("page", strconv.Itoa(page))
	return links.BossKills(vm.Realm) + "?" + v.Encode()
}

// sortHref toggles direction on repeat clicks, otherwise switches column.
func sortHref(vm ViewModel, col string) string {
	dir := "desc"
	if vm.Filter.SortBy == col && vm.Filter.SortDir == "desc" {
		dir = "asc"
	}
	v := filterValues(vm)
	if col == defaultBKSort && dir == defaultBKDir {
		v.Del("sort")
		v.Del("dir")
	} else {
		v.Set("sort", col)
		v.Set("dir", dir)
	}
	base := links.BossKills(vm.Realm)
	if enc := v.Encode(); enc != "" {
		return base + "?" + enc
	}
	return base
}
