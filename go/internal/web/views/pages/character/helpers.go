package character

import (
	"net/url"
	"strconv"

	"github.com/a-h/templ"
	"github.com/mrceperka/twinstar-bosskills/go/internal/links"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/views/viewhelpers"
)

func bossWithFiltersHref(realmName string, id uint32, mode, class, spec int) templ.SafeURL {
	q := "?difficulty=" + strconv.Itoa(mode)
	if spec > 0 {
		q += "&spec=" + strconv.Itoa(spec)
	}
	if class > 0 {
		q += "&class=" + strconv.Itoa(class)
	}
	return templ.SafeURL(links.Boss(realmName, id) + q)
}

// rankingsHref returns the URL for the lazy rankings fragment.
// spec=0 means "all specs".
func rankingsHref(realmName, charName string, spec int) string {
	base := links.Character(realmName, charName) + "/rankings"
	if spec != 0 {
		return base + "?spec=" + strconv.Itoa(spec)
	}
	return base
}

// killsBaseURL is the character page URL used as the hx-get target for the
// filter form and Reset link. It has no query params; the form submits its
// own values.
func killsBaseURL(vm ViewModel) string {
	return links.Character(vm.Realm, vm.Char.Name)
}

// filterQuery encodes just the applied filters + sort — used to preserve
// state across pagination + sort-header clicks.
func filterQuery(vm ViewModel) url.Values {
	q := url.Values{}
	for _, b := range vm.Filter.Bosses {
		q.Add("boss", strconv.FormatUint(uint64(b), 10))
	}
	for _, r := range vm.Filter.Raids {
		q.Add("raid", r)
	}
	for _, d := range vm.Filter.Difficulties {
		q.Add("difficulty", strconv.Itoa(d))
	}
	for _, s := range vm.Filter.Specs {
		q.Add("spec", strconv.Itoa(s))
	}
	if vm.Filter.SortBy != "" && vm.Filter.SortBy != defaultSortBy {
		q.Set("sort", vm.Filter.SortBy)
	}
	if vm.Filter.SortDir != "" && vm.Filter.SortDir != defaultSortDir {
		q.Set("dir", vm.Filter.SortDir)
	}
	return q
}

func pagedKillsURL(vm ViewModel, page int) string {
	q := filterQuery(vm)
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}
	base := links.Character(vm.Realm, vm.Char.Name)
	if enc := q.Encode(); enc != "" {
		return base + "?" + enc
	}
	return base
}

// sortURL toggles sort direction if the same column is clicked, otherwise
// switches to that column with a default direction (desc for numeric metrics,
// desc for kill_time).
func sortURL(vm ViewModel, col string) string {
	q := filterQuery(vm)
	// Toggle direction on the same column, otherwise default to desc.
	dir := "desc"
	if vm.Filter.SortBy == col && vm.Filter.SortDir == "desc" {
		dir = "asc"
	}
	if col == defaultSortBy && dir == defaultSortDir {
		q.Del("sort")
		q.Del("dir")
	} else {
		q.Set("sort", col)
		q.Set("dir", dir)
	}
	base := links.Character(vm.Realm, vm.Char.Name)
	if enc := q.Encode(); enc != "" {
		return base + "?" + enc
	}
	return base
}

func rankLabel(rank int) string {
	return viewhelpers.RankLabel(rank)
}

var (
	bossHref                 = viewhelpers.BossHref
	bosskillHref             = viewhelpers.BossKillHref
	characterPerformanceHref = viewhelpers.CharacterPerformanceHref
	fmtIlvl                  = viewhelpers.Ilvl
	armoryHref               = links.TwinstarArmory
)
