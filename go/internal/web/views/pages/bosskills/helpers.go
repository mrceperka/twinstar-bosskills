package bosskills

import (
	"net/url"
	"strconv"

	"github.com/a-h/templ"
	"github.com/mrceperka/twinstar-bosskills/go/internal/links"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/views/viewhelpers"
)

func bosskillsURL(realmName string) templ.SafeURL {
	return templ.SafeURL(links.BossKills(realmName))
}

// pagedURL builds a pagination href preserving the current filters. The
// links package owns the base URL; we just append the encoded query.
func pagedURL(vm ViewModel, page int) string {
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
	v.Set("page", strconv.Itoa(page))
	return links.BossKills(vm.Realm) + "?" + v.Encode()
}

var (
	bossHref          = viewhelpers.BossHref
	selectFilterClass = viewhelpers.SelectFilterClass
)
