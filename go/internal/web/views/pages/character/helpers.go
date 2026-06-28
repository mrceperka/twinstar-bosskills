package character

import (
	"strconv"

	"github.com/a-h/templ"
	"github.com/mrceperka/twinstar-bosskills/go/internal/links"
)

func bossHref(realmName string, id uint32) templ.SafeURL {
	return templ.SafeURL(links.Boss(realmName, id))
}

func bossWithFiltersHref(realmName string, id uint32, mode, class, spec int) templ.SafeURL {
	q := "?mode=" + strconv.Itoa(mode)
	if spec > 0 {
		q += "&spec=" + strconv.Itoa(spec)
	}
	if class > 0 {
		q += "&class=" + strconv.Itoa(class)
	}
	return templ.SafeURL(links.Boss(realmName, id) + q)
}

func bosskillHref(realmName, id string) templ.SafeURL {
	return templ.SafeURL(links.BossKill(realmName, id))
}

func characterPerformanceHref(realmName, name string) templ.SafeURL {
	return templ.SafeURL(links.CharacterPerformance(realmName, name))
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
	return "#" + strconv.Itoa(rank)
}

func fmtIlvl(v float32) string {
	if v <= 0 {
		return ""
	}
	return strconv.Itoa(int(v))
}
