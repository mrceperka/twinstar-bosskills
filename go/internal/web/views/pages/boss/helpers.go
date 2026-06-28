package boss

import (
	"strconv"

	"github.com/a-h/templ"
	"github.com/mrceperka/twinstar-bosskills/go/internal/links"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/views/viewhelpers"
	"github.com/mrceperka/twinstar-bosskills/go/internal/wow"
)

// Page-specific URL builders. Generic helpers (formatFloat, longDuration,
// itoa, tabActiveClass) come from internal/web/views/viewhelpers and
// internal/links via the var aliases below.

func contentBaseURL(vm ViewModel) templ.SafeURL {
	return templ.SafeURL(links.Boss(vm.Realm, vm.Boss.RemoteID))
}

func tabHref(vm ViewModel, mode int) templ.SafeURL {
	q := "mode=" + strconv.Itoa(mode) + "&p=" + strconv.Itoa(vm.SelectedPctile)
	if vm.SelectedSpec > 0 {
		q += "&spec=" + strconv.Itoa(vm.SelectedSpec)
	}
	if vm.SelectedClass > 0 {
		q += "&class=" + strconv.Itoa(vm.SelectedClass)
	}
	return templ.SafeURL(links.Boss(vm.Realm, vm.Boss.RemoteID) + "?" + q)
}

// siblingHref links to a sibling boss while preserving the current mode/spec/class.
func siblingHref(vm ViewModel, remoteID uint32) templ.SafeURL {
	q := "mode=" + strconv.Itoa(vm.SelectedMode)
	if vm.SelectedSpec > 0 {
		q += "&spec=" + strconv.Itoa(vm.SelectedSpec)
	}
	if vm.SelectedClass > 0 {
		q += "&class=" + strconv.Itoa(vm.SelectedClass)
	}
	return templ.SafeURL(links.Boss(vm.Realm, remoteID) + "?" + q)
}

func historyHref(vm ViewModel) templ.SafeURL {
	return templ.SafeURL(links.BossHistory(vm.Realm, vm.Boss.RemoteID) +
		"?mode=" + strconv.Itoa(vm.SelectedMode))
}

func raidIconHref(name string) templ.SafeURL {
	return templ.SafeURL(links.RaidIcon(name))
}

var (
	itoa            = viewhelpers.Itoa
	formatFloat     = viewhelpers.FormatFloat
	longDuration    = viewhelpers.LongDuration
	tabActiveClass  = viewhelpers.TabActiveClass
)

// filterHref builds a /{realm}/boss/{id} URL with the requested
// spec/class/mode/percentile combination. Used by the spec/class filter
// row and by Reset.
func filterHref(vm ViewModel, spec, class int) string {
	base := links.Boss(vm.Realm, vm.Boss.RemoteID)
	q := "mode=" + strconv.Itoa(vm.SelectedMode) +
		"&p=" + strconv.Itoa(vm.SelectedPctile)
	if spec > 0 {
		q += "&spec=" + strconv.Itoa(spec)
	}
	if class > 0 {
		q += "&class=" + strconv.Itoa(class)
	}
	return base + "?" + q
}

func filterCellClass(selected bool) string {
	if selected {
		return "ring-2 ring-bk-accent rounded"
	}
	return "opacity-70 hover:opacity-100"
}

// Thin wrappers used by the templ filter row.
func wowClass(c int) string         { return wow.Class(c) }
func wowSpecLabel(spec int) string  { return wow.Spec(spec) }
