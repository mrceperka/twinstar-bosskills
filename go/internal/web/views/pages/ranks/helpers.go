package ranks

import (
	"strconv"

	"github.com/a-h/templ"
	"github.com/mrceperka/twinstar-bosskills/go/internal/format"
	"github.com/mrceperka/twinstar-bosskills/go/internal/links"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/views/viewhelpers"
)

func tabHref(realmName string, mode, offset int) templ.SafeURL {
	return templ.SafeURL(links.Ranks(realmName) + "?difficulty=" + strconv.Itoa(mode) + "&raidlock=" + strconv.Itoa(offset))
}

func previousLockHref(vm ViewModel) templ.SafeURL {
	return tabHref(vm.Realm, vm.SelectedMode, vm.LockOffset+1)
}

var (
	itoa             = viewhelpers.Itoa
	bossWithModeHref = viewhelpers.BossWithDifficultyHref
	charHref         = viewhelpers.CharacterHref
	killHref         = viewhelpers.BossKillHref
	// Same "m:ss" style as bosskills and character pages so the column
	// stays a consistent width row-to-row and matches sibling tables.
	fmtLength = format.Duration
	fmtIlvl          = viewhelpers.Ilvl
	tabActiveClass   = viewhelpers.TabActiveClass
)
