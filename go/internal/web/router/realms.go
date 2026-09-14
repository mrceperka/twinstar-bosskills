// Package router holds small route-composition helpers used by every page
// package. The full registration is still explicit (see internal/web/server),
// these helpers just avoid the boilerplate of registering each pattern once
// per realm.
package router

import (
	"strings"

	"twinstar-bosskills/internal/realm"
)

// ForEachRealmPrefix calls register once for the canonical form and once for
// the lowercase form of every known realm.
//
// Why: Go 1.22 ServeMux can't disambiguate a `/{realm}/...` wildcard from a
// concrete `/static/...` pattern at the root. Listing realms explicitly
// sidesteps the conflict and lets us redirect lowercase URLs to canonical
// case via the realm middleware.
func ForEachRealmPrefix(register func(prefix string)) {
	for _, r := range realm.All() {
		register("/" + r)
		if lower := strings.ToLower(r); lower != r {
			register("/" + lower)
		}
	}
}
