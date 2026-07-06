package boss

import (
	"strconv"

	"github.com/a-h/templ"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/views/viewhelpers"
)

func tabHref(vm ViewModel, mode int) templ.SafeURL {
	href := string(viewhelpers.BossWithDifficultyHref(vm.Realm, vm.Boss.RemoteID, mode)) +
		"&raidlock=" + strconv.Itoa(vm.LockOffset) +
		"&p=" + strconv.Itoa(vm.SelectedPctile)
	if vm.SelectedSpec > 0 {
		href += "&spec=" + strconv.Itoa(vm.SelectedSpec)
	}
	if vm.SelectedClass > 0 {
		href += "&class=" + strconv.Itoa(vm.SelectedClass)
	}
	return templ.SafeURL(href)
}

// previousLockHref builds a Boss URL for the previous raid lockout while
// preserving the current mode/spec/class/percentile filters.
func previousLockHref(vm ViewModel) templ.SafeURL {
	href := string(viewhelpers.BossWithDifficultyHref(vm.Realm, vm.Boss.RemoteID, vm.SelectedMode)) +
		"&raidlock=" + strconv.Itoa(vm.LockOffset+1) +
		"&p=" + strconv.Itoa(vm.SelectedPctile)
	if vm.SelectedSpec > 0 {
		href += "&spec=" + strconv.Itoa(vm.SelectedSpec)
	}
	if vm.SelectedClass > 0 {
		href += "&class=" + strconv.Itoa(vm.SelectedClass)
	}
	return templ.SafeURL(href)
}

// siblingHref links to a sibling boss while preserving the current mode/spec/class.
func siblingHref(vm ViewModel, remoteID uint32) templ.SafeURL {
	href := string(viewhelpers.BossWithDifficultyHref(vm.Realm, remoteID, vm.SelectedMode)) +
		"&raidlock=" + strconv.Itoa(vm.LockOffset)
	if vm.SelectedSpec > 0 {
		href += "&spec=" + strconv.Itoa(vm.SelectedSpec)
	}
	if vm.SelectedClass > 0 {
		href += "&class=" + strconv.Itoa(vm.SelectedClass)
	}
	return templ.SafeURL(href)
}

// filterHref builds a /{realm}/boss/{id} URL with the requested
// spec/class/mode/percentile combination. Used by the spec/class filter
// row and by Reset.
func filterHref(vm ViewModel, spec, class int) string {
	href := string(viewhelpers.BossWithDifficultyHref(vm.Realm, vm.Boss.RemoteID, vm.SelectedMode)) +
		"&raidlock=" + strconv.Itoa(vm.LockOffset) +
		"&p=" + strconv.Itoa(vm.SelectedPctile)
	if spec > 0 {
		href += "&spec=" + strconv.Itoa(spec)
	}
	if class > 0 {
		href += "&class=" + strconv.Itoa(class)
	}
	return href
}

func filterCellClass(selected bool) string {
	if selected {
		return "ring-2 ring-bk-accent rounded"
	}
	return "opacity-70 hover:opacity-100"
}
