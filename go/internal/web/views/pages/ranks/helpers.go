package ranks

import (
	"hash/crc32"
	"net/url"
	"strconv"

	"github.com/a-h/templ"
	"twinstar-bosskills/internal/links"
	"twinstar-bosskills/internal/web/views/viewhelpers"
)

func previousLockHref(vm ViewModel) templ.SafeURL {
	href := links.Ranks(vm.Realm) + "?raidlock=" + strconv.Itoa(vm.LockOffset+1)
	if !vm.ClassMode {
		href += "&difficulty=" + strconv.Itoa(vm.SelectedMode)
	}
	return templ.SafeURL(href)
}

func rankContentBossHref(content RaidRankContent, bossID uint32) templ.SafeURL {
	if content.ClassMode {
		return viewhelpers.BossHref(content.Realm, bossID)
	}
	return viewhelpers.BossWithDifficultyHref(content.Realm, bossID, content.SelectedMode)
}

func rankRaidFragmentHref(vm ViewModel, raidName string) templ.SafeURL {
	q := url.Values{}
	q.Set("partial", "raid")
	q.Set("raid", raidName)
	q.Set("raidlock", strconv.Itoa(vm.LockOffset))
	if !vm.ClassMode {
		q.Set("difficulty", strconv.Itoa(vm.SelectedMode))
	}
	return templ.SafeURL(links.Ranks(vm.Realm) + "?" + q.Encode())
}

func raidContentID(raidName string) string {
	return "rank-raid-" + strconv.FormatUint(uint64(crc32.ChecksumIEEE([]byte(raidName))), 36)
}

func totalBosses(raids []RaidGroup) int {
	total := 0
	for _, raid := range raids {
		total += raidTotalBosses(raid)
	}
	return total
}

func raidTotalBosses(raid RaidGroup) int {
	if raid.TotalBosses > 0 {
		return raid.TotalBosses
	}
	return len(raid.Bosses)
}

func raidProgressStyle(raid RaidGroup) string {
	total := raidTotalBosses(raid)
	if total <= 0 {
		return "width: 0%"
	}
	pct := len(raid.Bosses) * 100 / total
	if pct > 100 {
		pct = 100
	}
	return "width: " + strconv.Itoa(pct) + "%"
}

func rankTableContext(content RaidRankContent) ViewModel {
	return ViewModel{
		Realm:        content.Realm,
		ClassMode:    content.ClassMode,
		SelectedMode: content.SelectedMode,
	}
}

func rankGroupLabel(vm ViewModel) string {
	if vm.ClassMode {
		return "Class"
	}
	return "Spec"
}
