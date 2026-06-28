package home

import (
	"strconv"

	"github.com/a-h/templ"
	"github.com/mrceperka/twinstar-bosskills/go/internal/links"
	"github.com/mrceperka/twinstar-bosskills/go/internal/realm"
)

func realmHref(name string) templ.SafeURL {
	return templ.SafeURL(links.RealmHome(name))
}

func expansionLabel(exp int) string {
	switch exp {
	case realm.ExpansionVanilla:
		return "Vanilla"
	case realm.ExpansionCata:
		return "Cata"
	case realm.ExpansionMoP:
		return "MoP"
	}
	return strconv.Itoa(exp)
}
