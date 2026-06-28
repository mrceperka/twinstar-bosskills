// Package viewhelpers holds small templ helper functions shared by every
// page package. Per-page helpers.go files should keep ONLY page-specific
// values (e.g. a Reset URL that needs the page's filter state).
//
// URL building lives in internal/links — use that for any kind of href.
// Number / duration / locale formatting lives in internal/format.
// WoW domain labels (class, spec, difficulty) live in internal/wow.
package viewhelpers

import "strconv"

// Itoa is a small templ-friendly wrapper around strconv.Itoa so the templ
// expression syntax stays clean (`{ Itoa(x) }`).
func Itoa(v int) string { return strconv.Itoa(v) }

// FormatFloat formats a float64 with the requested number of decimal places.
// Locale-neutral — for thousand-separated numbers see format.IntCtx.
func FormatFloat(v float64, decimals int) string {
	return strconv.FormatFloat(v, 'f', decimals, 64)
}

// LongDuration formats seconds as "X minutes Y seconds" — matches the
// narrative phrasing the SvelteKit app uses (boss-page headlines, etc.).
func LongDuration(seconds int) string {
	if seconds <= 0 {
		return "-"
	}
	m := seconds / 60
	s := seconds % 60
	switch {
	case m == 0:
		return strconv.Itoa(s) + " seconds"
	case s == 0:
		return strconv.Itoa(m) + " minutes"
	default:
		return strconv.Itoa(m) + " minutes " + strconv.Itoa(s) + " seconds"
	}
}

// AbsInt returns |v|.
func AbsInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// Uitoa is the uint64 counterpart of Itoa.
func Uitoa(v uint64) string { return strconv.FormatUint(v, 10) }

// TabActiveClass returns the Tailwind classes that mark a tab as active or
// inactive. Used by boss / boss-history / ranks pages — same look for all.
func TabActiveClass(active bool) string {
	if active {
		return "text-bk-fg font-semibold underline"
	}
	return "text-bk-muted hover:text-bk-fg"
}
