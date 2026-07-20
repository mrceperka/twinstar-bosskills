package boss

import (
	"strconv"

	"twinstar-bosskills/internal/web/views/viewhelpers"

	"github.com/a-h/templ"
)

func tabHref(vm ViewModel, mode int) templ.SafeURL {
	return templ.SafeURL(bossHrefWithFilters(vm, vm.Boss.RemoteID, mode, vm.SelectedSpec, vm.SelectedClass, vm.LockScoped, vm.LockOffset))
}

// previousLockHref builds a Boss URL for the previous raid lockout while
// preserving the current mode/spec/class/percentile filters.
func previousLockHref(vm ViewModel) templ.SafeURL {
	return templ.SafeURL(bossHrefWithFilters(vm, vm.Boss.RemoteID, vm.SelectedMode, vm.SelectedSpec, vm.SelectedClass, true, vm.LockOffset+1))
}

// currentLockHref switches the overall view to the current raid lockout.
func currentLockHref(vm ViewModel) templ.SafeURL {
	return templ.SafeURL(bossHrefWithFilters(vm, vm.Boss.RemoteID, vm.SelectedMode, vm.SelectedSpec, vm.SelectedClass, true, 0))
}

// siblingHref links to a sibling boss while preserving the current mode/spec/class/percentile.
func siblingHref(vm ViewModel, remoteID uint32) templ.SafeURL {
	return templ.SafeURL(bossHrefWithFilters(vm, remoteID, vm.SelectedMode, vm.SelectedSpec, vm.SelectedClass, vm.LockScoped, vm.LockOffset))
}

// filterHref builds a /{realm}/boss/{id} URL with the requested
// spec/class/mode/percentile combination. Used by the spec/class filter
// row and by Reset.
func filterHref(vm ViewModel, spec, class int) string {
	return bossHrefWithFilters(vm, vm.Boss.RemoteID, vm.SelectedMode, spec, class, vm.LockScoped, vm.LockOffset)
}

func bossHrefWithFilters(vm ViewModel, remoteID uint32, mode, spec, class int, includeLock bool, lockOffset int) string {
	href := string(viewhelpers.BossWithDifficultyHref(vm.Realm, remoteID, mode))
	if includeLock {
		href += "&raidlock=" + strconv.Itoa(lockOffset)
	}
	href += "&p=" + strconv.Itoa(vm.SelectedPctile)
	if !vm.ClassMode && spec > 0 {
		href += "&spec=" + strconv.Itoa(spec)
	}
	if vm.ClassMode && class > 0 {
		href += "&class=" + strconv.Itoa(class)
	}
	return href
}

func groupLabel(vm ViewModel) string {
	if vm.ClassMode {
		return "class"
	}
	return "spec"
}

func groupTitle(vm ViewModel) string {
	if vm.ClassMode {
		return "Class"
	}
	return "Spec"
}

func filterCellClass(selected bool) string {
	if selected {
		return "ring-2 ring-bk-accent rounded"
	}
	return "opacity-70 hover:opacity-100"
}
