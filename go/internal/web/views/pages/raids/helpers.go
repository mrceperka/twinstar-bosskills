package raids

import (
	"github.com/a-h/templ"
	"github.com/mrceperka/twinstar-bosskills/go/internal/links"
)

func raidIconHref(name string) templ.SafeURL {
	return templ.SafeURL(links.RaidIcon(name))
}
