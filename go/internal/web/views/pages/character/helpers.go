package character

import (
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

func pagedKillsURL(vm ViewModel, page int) string {
	return links.Character(vm.Realm, vm.Char.Name) + "?page=" + strconv.Itoa(page)
}

func rankLabel(rank int) string {
	return viewhelpers.RankLabel(rank)
}

var (
	bossHref                 = viewhelpers.BossHref
	bosskillHref             = viewhelpers.BossKillHref
	characterPerformanceHref = viewhelpers.CharacterPerformanceHref
	fmtIlvl                  = viewhelpers.Ilvl
)
