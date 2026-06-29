package raids

import (
	"strconv"

	"github.com/a-h/templ"
	"github.com/mrceperka/twinstar-bosskills/go/internal/links"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/views/viewhelpers"
)

func previousLockHref(realmName string, offset int) templ.SafeURL {
	return templ.SafeURL(links.Raids(realmName) + "?raidlock=" + strconv.Itoa(offset))
}

var raidIconHref = viewhelpers.RaidIconHref
