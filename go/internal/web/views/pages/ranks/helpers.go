package ranks

import (
	"strconv"

	"github.com/a-h/templ"
	"github.com/mrceperka/twinstar-bosskills/go/internal/links"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/views/viewhelpers"
)

func previousLockHref(vm ViewModel) templ.SafeURL {
	href := links.Ranks(vm.Realm) + "?raidlock=" + strconv.Itoa(vm.LockOffset+1)
	if !vm.ClassMode {
		href += "&difficulty=" + strconv.Itoa(vm.SelectedMode)
	}
	return templ.SafeURL(href)
}

func rankBossHref(vm ViewModel, bossID uint32) templ.SafeURL {
	if vm.ClassMode {
		return viewhelpers.BossHref(vm.Realm, bossID)
	}
	return viewhelpers.BossWithDifficultyHref(vm.Realm, bossID, vm.SelectedMode)
}

func rankGroupLabel(vm ViewModel) string {
	if vm.ClassMode {
		return "Class"
	}
	return "Spec"
}
